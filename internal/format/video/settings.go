package video

import (
	"fmt"
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/imagedim"
)

// Setting names. Public names (untouchable rule 10), so they are spelled once
// and every video format declares them from here.
const (
	SettingDuration         = "duration"
	SettingChangeInterval   = "change_interval"
	SettingFrameRate        = "frame_rate"
	SettingKeyframeInterval = "keyframe_interval"
	SettingQuality          = "quality"
)

// The defaults are the owner's of 2026-10-06 (docs/WIDEO-2026-10-06.md
// sections 12 and 15): ten seconds, a key frame a minute, which leaves a short
// film with one key frame, and a new picture every second, so the clock in it
// ticks the way a clock does.
const (
	defaultDurationMs       = 10_000
	defaultKeyIntervalMs    = 60_000
	defaultChangeIntervalMs = 1_000
	defaultFPS              = 30

	// maxDurationMs is a day. A day at sixty frames a second is five million
	// frames, which a container writes in seconds and holds in no memory -
	// longer than that is not a test file anybody has asked for, and the bound
	// keeps the arithmetic far inside what the containers' fields carry.
	maxDurationMs = 86_400_000

	// The quality scale AVIF uses, and the same mapping onto the quantizer, so
	// one number means one thing in both.
	minQuality     = 1
	maxQuality     = 100
	defaultQuality = 60

	// The picture has to fit one AV1 tile, because gav1d codes one: a frame
	// wider than 4096 or larger than 2304 blocks of 64 by 64 pixels comes out
	// of its encoder with a header that announces several tiles over the data
	// of one, and both libaom and gav1d's own decoder refuse it. Measured on
	// 2026-10-06 (docs/REVIEW-165-2026-10-06.md): 4096x2304 decodes, 4096x2305
	// and 4097x64 do not. That is a limit of the encoder this tool carries, not
	// of AV1, so it is the number to raise if a later gav1d codes several tiles.
	//
	// maxWidth is the widest tile. maxHeight is gav1d's longest side - a tall
	// narrow picture is one tile as long as it stays within the area.
	maxWidth  = 4096
	maxHeight = 16384
	// tileBlocks is how many 64 by 64 blocks one tile holds, and
	// maxTilePixels the same area in pixels, which is what the registry can
	// declare: a product of the two sides. The blocks are what decides, because
	// a side is rounded up to a whole block, so a pair just under the pixels
	// can still be one block too many - checked before coding, see named.
	tileBlocks    = 2304
	maxTilePixels = 4096 * 2304
)

// frameRates are whole rates only. 23.976, 29.97 and 59.94 need a time scale
// that is not a whole number of milliseconds, which WebM does not have - left
// for later, section 11.5.
var frameRates = []string{"1", "5", "10", "12", "15", "24", "25", "30", "48", "50", "60"}

// Properties is what every video format declares, in the order both surfaces
// show it.
func Properties() []format.Property {
	return []format.Property{
		imagedim.Width(imagedim.Side{Largest: maxWidth,
			Detail: "How wide the picture is. Left out, a size is chosen that fits the bytes you asked for."}),
		imagedim.Height(imagedim.Side{Largest: maxHeight,
			Detail: "How tall the picture is. Left out, a size is chosen that fits the bytes you asked for."}),
		{
			Name: SettingDuration, Kind: format.PropertyDuration,
			Min: 1, Max: maxDurationMs, Default: core.FormatDuration(defaultDurationMs),
			Detail: "How long the film plays. It has to end on a frame, so at 30 frames a second it goes in steps of 100ms.",
		},
		{
			Name: SettingChangeInterval, Kind: format.PropertyDuration,
			Min: 1, Max: maxDurationMs, Default: core.FormatDuration(defaultChangeIntervalMs),
			Detail: "How often the picture changes: the clock in it moves on and the square takes a step. Each change costs a whole picture in bytes. An interval as long as the film or longer keeps one picture throughout.",
		},
		{
			Name: SettingFrameRate, Kind: format.PropertyChoice,
			Choices: frameRates, Default: strconv.Itoa(defaultFPS),
			Detail: "How many frames the film shows each second.",
		},
		{
			Name: SettingKeyframeInterval, Kind: format.PropertyDuration,
			Min: 1, Max: maxDurationMs, Default: core.FormatDuration(defaultKeyIntervalMs),
			Detail: "How far apart the frames are that a player can start from. Shorter makes skipping through a long film faster and the file bigger, because each of them carries the whole picture twice.",
		},
		{
			Name: SettingQuality, Kind: format.PropertyInt,
			Min: minQuality, Max: maxQuality, Default: strconv.Itoa(defaultQuality),
			Detail: "How much detail the picture keeps. Higher looks better and takes more of the file, leaving less room for padding.",
		},
	}
}

// JointLimits is the bound on the picture, declared once for both formats:
// the area of one AV1 tile. In pixels rather than megapixels, because 9 437 184
// read as "9 megapixels" would be a limit nobody can aim at.
//
// The memory bound AVIF declares is not here, and not by omission: one tile is
// far below it, so it could never be the limit a request meets.
func JointLimits() []format.JointLimit {
	return []format.JointLimit{{
		Of: imagedim.SettingWidth, By: imagedim.SettingHeight, Max: maxTilePixels,
		Unit: "pixels", Base: "pixels",
		Why: "the encoder codes a picture as one AV1 tile, which holds 4096 by 2304 pixels",
	}}
}

// Settings is what a request asked for, read and settled.
type Settings struct {
	Timeline
	Quality int
	// QIndex is the quantizer the quality maps to, the mapping gav1d's AVIF
	// encoder uses: quality 100 is 0, which is lossless.
	QIndex int
	// QualityNamed is whether the request named a quality, because the
	// ladder's ceilings were measured at the default and speak for no other.
	QualityNamed bool
}

// Read settles a request's video settings. The registry has already checked
// each value against its declaration, so a failure to read one here is a
// defect rather than a refusal - except where a length does not end on a
// frame, which no declaration can say on its own.
func Read(formatID string, props map[string]string) (Settings, error) {
	duration, err := durationOr(props, SettingDuration, defaultDurationMs)
	if err != nil {
		return Settings{}, err
	}
	interval, err := durationOr(props, SettingKeyframeInterval, defaultKeyIntervalMs)
	if err != nil {
		return Settings{}, err
	}
	change, err := durationOr(props, SettingChangeInterval, defaultChangeIntervalMs)
	if err != nil {
		return Settings{}, err
	}
	fps := defaultFPS
	if raw := props[SettingFrameRate]; raw != "" {
		if fps, err = strconv.Atoi(raw); err != nil {
			return Settings{}, core.Defect(fmt.Errorf("video: frame_rate %q passed the registry and is not a number", raw))
		}
	}
	quality := defaultQuality
	raw, named := props[SettingQuality]
	if named && raw != "" {
		if quality, err = strconv.Atoi(raw); err != nil {
			return Settings{}, core.Defect(fmt.Errorf("video: quality %q passed the registry and is not a number", raw))
		}
	}
	t, err := NewTimeline(formatID, duration, interval, change, fps)
	if err != nil {
		return Settings{}, err
	}
	if t.Keys() > MaxKeyFrames {
		return Settings{}, tooManyKeyFrames(formatID, t, interval)
	}
	if t.Changes() > MaxChanges {
		return Settings{}, tooManyChanges(formatID, t, change)
	}
	return Settings{
		Timeline: t, Quality: quality, QIndex: (100 - quality) * 255 / 100,
		QualityNamed: named && raw != "" && quality != defaultQuality,
	}, nil
}

// MaxKeyFrames bounds how many key frames one film has.
//
// A container lays a film out per key frame - a cluster and a cue point each -
// so the work of planning one grows with them, and every key frame carries the
// picture twice. A key frame on every frame of a short clip is a real test
// (every frame a place to start) and stays possible. The bound is what stops
// the same setting on a day long film from asking for five million of them:
// a hundred thousand is a key frame a second for more than a day.
const MaxKeyFrames = 100_000

func tooManyKeyFrames(formatID string, t Timeline, interval int64) error {
	step := stepMs(t.FPS)
	shortest := (t.DurationMs + MaxKeyFrames - 1) / MaxKeyFrames
	shortest = (shortest + step - 1) / step * step
	return &format.PropertyValueError{
		Format: formatID, Key: SettingKeyframeInterval, Value: core.FormatDuration(interval),
		Reason: core.Says("video.TooManyKeyFrames",
			"a film of %s with a key frame every %s has %d of them, and a film can have at most %d. Ask for %s or longer, or a shorter film",
			core.A("Duration", core.FormatDuration(t.DurationMs)), core.A("Interval", core.FormatDuration(interval)),
			core.A("Keys", t.Keys()), core.A("Most", MaxKeyFrames), core.A("Shortest", core.FormatDuration(shortest))),
	}
}

// MaxChanges bounds how many pictures one film shows.
//
// Every change is a picture of its own - gav1d codes stills, so nothing is
// carried over from the picture before - and a picture costs its bytes. Its
// time was 34 ms at 640x360 and about a second at 3840x2160 when every picture
// was coded whole (2026-10-06, docs/WIDEO-2026-10-06.md section 14). Cut into
// tiles, a change codes only the tiles nobody coded before, a few
// milliseconds (docs/WEBM-WYDAJNOSC-2026-10-06.md section 10). A change a
// second for a day is under the bound, a change every frame for an hour is
// not.
const MaxChanges = 100_000

func tooManyChanges(formatID string, t Timeline, interval int64) error {
	step := stepMs(t.FPS)
	shortest := (t.DurationMs + MaxChanges - 1) / MaxChanges
	shortest = (shortest + step - 1) / step * step
	return &format.PropertyValueError{
		Format: formatID, Key: SettingChangeInterval, Value: core.FormatDuration(interval),
		Reason: core.Says("video.TooManyChanges",
			"a film of %s with a new picture every %s shows %d pictures, and a film can show at most %d. Ask for %s or longer, or a shorter film",
			core.A("Duration", core.FormatDuration(t.DurationMs)), core.A("Interval", core.FormatDuration(interval)),
			core.A("Pictures", t.Changes()), core.A("Most", MaxChanges), core.A("Shortest", core.FormatDuration(shortest))),
	}
}

func durationOr(props map[string]string, key string, fallback int64) (int64, error) {
	raw := props[key]
	if raw == "" {
		return fallback, nil
	}
	ms, err := core.ParseDuration(raw)
	if err != nil {
		return 0, core.Defect(fmt.Errorf("video: %s %q passed the registry and does not read: %w", key, raw, err))
	}
	return ms, nil
}
