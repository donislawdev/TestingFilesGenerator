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
// Ceiling is a tenth above the largest tile any picture of a film on the rung
// coded to, and it is the reserve of every one of them. Measured by
// tools/probes/videoladder at the default quality on 2026-10-06
// (docs/WIDEO-2026-10-06.md section 15), in three passes. First every seed
// offset there is with every length the label can have, either format's name
// in it, and the label absent, in both shapes of the clock. Then the eight
// heaviest of those at a hundred clocks, every step of the square and the
// longest times a day shows. Then 512 seeds far from zero, because the
// label's text moves with the whole seed and not only with the offset.
//
// The tenth is not caution for its own sake. On 640x360 the far seeds coded
// larger than the first pass by 1.9 percent (15194 B, then 15478 B), and
// before the pictures moved, on four rungs of eleven, by up to 3.7 percent
// (section 13), so a first pass alone would have been a ceiling a seed nobody
// tried could pass. A ceiling too high costs a smaller picture than there was
// room for and nothing else. One too low would let planning promise a size
// writing cannot keep, so writing checks every picture and says so rather
// than trusting this table.
var Ladder = []Rung{
	{640, 360, 17026}, {426, 240, 10905}, {320, 180, 8191}, {256, 144, 6754},
	{160, 90, 3951}, {80, 45, 1454}, {40, 23, 825}, {16, 9, 149},
	{4, 3, 46}, {2, 2, 31}, {1, 1, 6},
}

// Choice is the picture a film shows.
type Choice struct {
	Width, Height int
	Seed          uint64
	Label         string
	QIndex        int
	// First is the film's first picture when planning had to code it - a size
	// named by hand, or a quality the ceilings were not measured at. Nil means
	// the picture is a ladder rung and writing codes it.
	First *Coded
	// Ceiling is the most tile bytes the plan allowed each picture, which
	// writing holds every picture to.
	Ceiling int
	// Named is whether the request gave the picture's size. A picture chosen
	// to fit is the largest that does, so only a named one can be asked to be
	// smaller.
	Named bool
}

// Labelled is whether this picture carries its label.
func (c Choice) Labelled() bool { return Labelled(c.Width, c.Label) }

// ClockShown is whether this film's pictures show the clock.
func (c Choice) ClockShown(t Timeline) bool { return ClockShown(c.Width, c.Height, c.Label, t) }

// sampleReserve codes the sample of a film's pictures, SampleChanges, and
// gives back the first picture, which writing reuses, and the reserve every
// picture is held to: the largest of the sample when the sample is the whole
// film, and a tenth above it when it is not.
//
// The first picture alone is no base for it. The pictures of one film differ
// only in the clock and the square, and on a small picture those are much of
// it: the largest of a film came out up to 42 percent above its first (48x32)
// and 9 percent above the largest of its first ten (52x20), because the
// clock's other digits change later. Against the spread sample the largest of
// whole films of 240 to 3600 pictures, from 16x16 to 640x360, five seeds and
// both shapes of the clock, came out at most 4.0 percent above (48x32) -
// tools/probes/videomotion/spread, 2026-10-06, docs/WIDEO-2026-10-06.md
// section 15. A tenth is two and a half times that, the margin the ladder's
// ceilings keep over their own measurement. The gradient does not move between
// pictures, by the owner's decision of the same day, because moving it changed
// a picture's size by up to 1.9 times.
func sampleReserve(w, h int, seed uint64, label string, t Timeline, qindex int) (Coded, int, error) {
	p := newPainter(w, h, seed, label, t)
	sample := SampleChanges(t)
	var first Coded
	largest := 0
	for _, c := range sample {
		coded, err := Encode(p.paint(c), qindex)
		if err != nil {
			return Coded{}, 0, err
		}
		if c == 0 {
			first = coded
		}
		largest = max(largest, coded.Size())
	}
	if int64(len(sample)) == t.Changes() {
		return first, largest, nil
	}
	return first, largest + (largest+9)/10, nil
}

// SampleChanges is the pictures planning codes to settle the reserve of a film
// whose picture it cannot take from the ladder: every one of a film of ten
// pictures or fewer, and of a longer film ten, one at each step of the square,
// in walks spread from its start to its end, so the sample sees the square
// everywhere and the clock with the digits of the whole film.
func SampleChanges(t Timeline) []int64 {
	n := t.Changes()
	if n <= squareSteps {
		out := make([]int64, n)
		for c := range out {
			out[c] = int64(c)
		}
		return out
	}
	walks := n / squareSteps
	out := make([]int64, squareSteps)
	for k := range out {
		out[k] = int64(k)*(walks-1)/(squareSteps-1)*squareSteps + int64(k)
	}
	return out
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
// rung's ceiling for every picture, or - when the request named a quality the
// ceilings were not measured at - a reserve from a sample of the film's
// pictures, coded (sampleReserve). That is the slow road AVIF takes for the
// same reason, and only runs that asked for it pay.
func onRung(base Choice, rung Rung, s Settings) (Choice, Stream, error) {
	c := base
	c.Width, c.Height, c.Ceiling = rung.Width, rung.Height, rung.Ceiling
	if !s.QualityNamed {
		return c, NewStream(s.Timeline, rung.Ceiling, rung.Width, rung.Height), nil
	}
	first, reserve, err := sampleReserve(c.Width, c.Height, c.Seed, c.Label, s.Timeline, c.QIndex)
	if err != nil {
		return Choice{}, Stream{}, err
	}
	c.First, c.Ceiling = &first, reserve
	return c, NewStream(s.Timeline, c.Ceiling, c.Width, c.Height), nil
}

// named settles a film whose picture size the request gave by coding a sample
// of its pictures at planning, because nothing but coding them says how big
// they are (sampleReserve).
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
	first, reserve, err := sampleReserve(w, h, c.Seed, c.Label, s.Timeline, c.QIndex)
	if err != nil {
		return Choice{}, Stream{}, err
	}
	c.Width, c.Height, c.First, c.Ceiling, c.Named = w, h, &first, reserve, true
	return c, NewStream(s.Timeline, c.Ceiling, w, h), nil
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
		"change_count":               s.Changes(),
		"change_interval_ms":         s.ChangeMs(),
		"quality":                    s.Quality,
		"compression":                "av1",
		"audio":                      false,
		format.PropertyLabelEmbedded: c.Labelled(),
	}
}
