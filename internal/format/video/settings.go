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
	SettingFrameRate        = "frame_rate"
	SettingKeyframeInterval = "keyframe_interval"
	SettingQuality          = "quality"
)

// The defaults are the owner's of 2026-10-06 (docs/WIDEO-2026-10-06.md
// section 12): ten seconds, and a key frame a minute, which leaves a short film
// with one key frame and an hour at about a megabyte and a half.
const (
	defaultDurationMs    = 10_000
	defaultKeyIntervalMs = 60_000
	defaultFPS           = 30

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

	// maxDimension is the longest side, the same as AVIF's - gav1d's ceiling.
	maxDimension = 16384
	// MaxPixels bounds the picture for the reason AVIF's is bounded: the whole
	// picture is held while it is coded.
	MaxPixels = 40_000_000
)

// frameRates are whole rates only. 23.976, 29.97 and 59.94 need a time scale
// that is not a whole number of milliseconds, which WebM does not have - left
// for later, section 11.5.
var frameRates = []string{"1", "5", "10", "12", "15", "24", "25", "30", "48", "50", "60"}

// Properties is what every video format declares, in the order both surfaces
// show it.
func Properties() []format.Property {
	return []format.Property{
		imagedim.Width(imagedim.Side{Largest: maxDimension,
			Detail: "How wide the picture is. Left out, a size is chosen that fits the bytes you asked for."}),
		imagedim.Height(imagedim.Side{Largest: maxDimension,
			Detail: "How tall the picture is. Left out, a size is chosen that fits the bytes you asked for."}),
		{
			Name: SettingDuration, Kind: format.PropertyDuration,
			Min: 1, Max: maxDurationMs, Default: core.FormatDuration(defaultDurationMs),
			Detail: "How long the film plays. It has to end on a frame, so at 30 frames a second it goes in steps of 100ms.",
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

// JointLimits is the bound on the picture, declared once for both formats.
func JointLimits() []format.JointLimit {
	return []format.JointLimit{{
		Of: imagedim.SettingWidth, By: imagedim.SettingHeight, Max: MaxPixels,
		Unit: "megapixels", Per: 1_000_000, Base: "pixels",
		Why: "the encoder holds the whole picture in memory while it works",
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
	t, err := NewTimeline(formatID, duration, interval, fps)
	if err != nil {
		return Settings{}, err
	}
	if t.Keys() > MaxKeyFrames {
		return Settings{}, tooManyKeyFrames(formatID, t, interval)
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
