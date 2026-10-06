package video

import (
	"fmt"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/imagedim"
)

// Rung is a picture size planning can choose without coding the picture, with
// the largest tile it has ever been seen to code to.
type Rung struct {
	Width, Height int
	Ceiling       int
}

// Ladder is tried from the largest down when the request names no picture
// size, so a small file gets a small picture instead of being refused.
//
// Sixteen by nine, because that is the shape of a film, and it stops at
// 640x360 for the reason AVIF stops at 640x480: a larger picture costs every
// file of a run its encoding time, and somebody who wants Full HD says so.
//
// Ceiling is a tenth above the largest tile the rung's picture coded to in two
// passes - every seed offset there is with every length the label can have,
// either format's name in it, and the label absent, then 512 seeds far from
// zero, because the label's text moves with the whole seed and not only with
// the offset. Measured by tools/probes/videoladder at the default quality on
// 2026-10-06 (docs/WIDEO-2026-10-06.md section 13).
//
// The tenth is not caution for its own sake. On four rungs of eleven the far
// seeds coded larger than the largest of the first pass, by up to 3.7 percent
// (256x144: 4185 B, then 4338 B), so the first pass alone would have been a
// ceiling a seed nobody tried could pass. A tenth is close to three times the
// worst of that. A ceiling too high costs a smaller picture than there was
// room for and nothing else. One too low would let planning promise a size
// writing cannot keep, so writing checks and says so rather than trusting
// this table.
var Ladder = []Rung{
	{640, 360, 14644}, {426, 240, 8500}, {320, 180, 6192}, {256, 144, 4772},
	{160, 90, 2953}, {80, 45, 385}, {40, 23, 218}, {16, 9, 83},
	{4, 3, 38}, {2, 2, 21}, {1, 1, 6},
}

// Choice is the picture a film shows.
type Choice struct {
	Width, Height int
	Seed          uint64
	Label         string
	QIndex        int
	// Coded is the picture when planning had to code it - a size named by hand,
	// or a quality the ceilings were not measured at. Nil means the picture is
	// a ladder rung and writing codes it.
	Coded *Coded
	// Ceiling is the largest tile the plan allowed for, which writing holds
	// the coded picture to.
	Ceiling int
	// Named is whether the request gave the picture's size. A picture chosen
	// to fit is the largest that does, so only a named one can be asked to be
	// smaller.
	Named bool
}

// Labelled is whether this picture carries its label.
func (c Choice) Labelled() bool { return Labelled(c.Width, c.Label) }

// Code is the picture's coded form, coding it when planning did not.
func (c Choice) Code() (Coded, error) {
	if c.Coded != nil {
		return *c.Coded, nil
	}
	coded, err := Encode(Picture(c.Width, c.Height, c.Seed, c.Label), c.QIndex)
	if err != nil {
		return Coded{}, err
	}
	if coded.Size() > c.Ceiling {
		return Coded{}, core.Defect(fmt.Errorf("video: a %dx%d picture coded to a %d B tile and its rung allows %d B, so the file planned around it cannot be kept",
			c.Width, c.Height, coded.Size(), c.Ceiling))
	}
	return coded, nil
}

// Choose settles the picture for a request, and the stream planning sizes the
// file by: exact when the picture was coded, the bound of its rung otherwise.
//
// fits says whether a stream leaves room in the requested size. When nothing
// fits, the smallest candidate comes back so the format can name the minimum.
func Choose(formatID string, r format.Request, s Settings, label string, fits func(Stream) bool) (Choice, Stream, error) {
	base := Choice{Seed: r.Seed, Label: label, QIndex: s.QIndex}
	_, wSet := r.Properties[imagedim.SettingWidth]
	_, hSet := r.Properties[imagedim.SettingHeight]
	if wSet || hSet {
		return named(formatID, r, s, base)
	}
	var last Choice
	var lastStream Stream
	for _, rung := range Ladder {
		c, st, err := onRung(base, rung, s)
		if err != nil {
			return Choice{}, Stream{}, err
		}
		if fits(st) {
			return c, st, nil
		}
		last, lastStream = c, st
	}
	return last, lastStream, nil
}

// onRung is the picture of one rung and the stream planning sizes it by: the
// rung's bound, or - when the request named a quality the ceilings were not
// measured at - the picture itself, coded. That is the slow road AVIF takes
// for the same reason, and only runs that asked for it pay.
func onRung(base Choice, rung Rung, s Settings) (Choice, Stream, error) {
	c := base
	c.Width, c.Height, c.Ceiling = rung.Width, rung.Height, rung.Ceiling
	if !s.QualityNamed {
		return c, Bound(s.Timeline, rung.Ceiling, rung.Width, rung.Height), nil
	}
	coded, err := Encode(Picture(c.Width, c.Height, c.Seed, c.Label), c.QIndex)
	if err != nil {
		return Choice{}, Stream{}, err
	}
	c.Coded, c.Ceiling = &coded, coded.Size()
	return c, NewStream(s.Timeline, coded, c.Width, c.Height), nil
}

// named codes a picture whose size the request gave, at planning, because
// nothing but coding it says how big it is.
func named(formatID string, r format.Request, s Settings, c Choice) (Choice, Stream, error) {
	w, err := imagedim.Value(formatID, imagedim.SettingWidth, r.Properties, maxWidth, Ladder[0].Width)
	if err != nil {
		return Choice{}, Stream{}, err
	}
	h, err := imagedim.Value(formatID, imagedim.SettingHeight, r.Properties, maxHeight, Ladder[0].Height)
	if err != nil {
		return Choice{}, Stream{}, err
	}
	if err := checkJointLimits(formatID, w, h); err != nil {
		return Choice{}, Stream{}, err
	}
	if err := checkOneTile(formatID, w, h); err != nil {
		return Choice{}, Stream{}, err
	}
	coded, err := Encode(Picture(w, h, c.Seed, c.Label), c.QIndex)
	if err != nil {
		return Choice{}, Stream{}, err
	}
	c.Width, c.Height, c.Coded, c.Ceiling, c.Named = w, h, &coded, coded.Size(), true
	return c, NewStream(s.Timeline, coded, w, h), nil
}

// checkJointLimits asks the registry's own declaration, so the refusal, the
// sentence tfg formats prints and the field a window draws come from one line.
func checkJointLimits(formatID string, w, h int) error {
	d, err := format.Get(formatID)
	if err != nil {
		return err
	}
	for _, j := range d.JointLimits {
		if bad := j.Allows(formatID, int64(w), int64(h)); !bad.IsZero() {
			return &format.PropertyValueError{
				Format: formatID, Key: j.Of + " and " + j.By, Subject: j.Subject(),
				Value:  fmt.Sprintf("%dx%d", w, h),
				Reason: core.Says("format.JointSmallerPair", "%s. Ask for a smaller pair", core.A("Why", bad)),
			}
		}
	}
	return nil
}

// checkOneTile refuses a picture that would need a second AV1 tile, before
// the encoder makes a frame nobody can decode out of it.
//
// The declared limit counts pixels and this counts blocks of 64 by 64, each
// side rounded up, which is what the encoder goes by - 4000x2359 is under the
// pixels and one block too many. Said as a setting the request can change,
// not as a fault of the program.
func checkOneTile(formatID string, w, h int) error {
	blocks := ((w + 63) / 64) * ((h + 63) / 64)
	if blocks <= tileBlocks {
		return nil
	}
	return &format.PropertyValueError{
		Format: formatID, Key: imagedim.SettingWidth + " and " + imagedim.SettingHeight,
		Subject: core.Says("format.TwoSettings", "%s and %s", core.A("Of", core.LabelTerm(imagedim.SettingWidth)), core.A("By", core.LabelTerm(imagedim.SettingHeight))),
		Value:   fmt.Sprintf("%dx%d", w, h),
		Reason: core.Says("video.MoreThanOneTile",
			"a %dx%d picture is %d blocks of 64 by 64 pixels once its sides are rounded up to whole blocks, and the encoder codes one AV1 tile, which holds %d. Ask for a smaller pair, such as 4096x2304 or 3840x2160",
			core.A("Width", w), core.A("Height", h), core.A("Blocks", blocks), core.A("Most", tileBlocks)),
	}
}

// Facts is what a film's plan tells the manifest. The keys an image and a
// sound already use mean the same here - width, height, frame_count,
// duration_ms, compression - because they are public names a test asserts
// on (untouchable rule 10).
func Facts(c Choice, s Settings) map[string]any {
	return map[string]any{
		"width":                      c.Width,
		"height":                     c.Height,
		"duration_ms":                s.DurationMs,
		"frame_count":                s.Frames,
		"frame_rate":                 s.FPS,
		"keyframe_count":             s.Keys(),
		"keyframe_interval_ms":       s.KeyEvery * 1000 / int64(s.FPS),
		"quality":                    s.Quality,
		"compression":                "av1",
		"audio":                      false,
		format.PropertyLabelEmbedded: c.Labelled(),
	}
}
