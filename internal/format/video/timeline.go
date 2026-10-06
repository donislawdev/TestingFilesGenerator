package video

import (
	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// Timeline is how many frames a film has, how far apart they are, and which
// of them are key frames.
type Timeline struct {
	FPS        int
	Frames     int64
	DurationMs int64
	// KeyEvery is how many frames apart key frames are. The first frame is
	// always one.
	KeyEvery int64
}

// NewTimeline settles a length and a key frame interval against a frame rate.
//
// A length has to end on a frame, and so does the interval, or the film would
// last a different time from the one asked for - refused with the two nearest
// lengths that do, by the owner's decision of 2026-10-06
// (docs/WIDEO-2026-10-06.md section 12). Lengths are whole milliseconds, so at
// thirty frames a second they go in steps of 100ms, at twenty four in steps of
// 125ms: the step is the shortest whole number of milliseconds that is a whole
// number of frames.
func NewTimeline(formatID string, durationMs, keyIntervalMs int64, fps int) (Timeline, error) {
	frames, err := onFrames(formatID, SettingDuration, durationMs, fps)
	if err != nil {
		return Timeline{}, err
	}
	keyEvery, err := onFrames(formatID, SettingKeyframeInterval, keyIntervalMs, fps)
	if err != nil {
		return Timeline{}, err
	}
	return Timeline{FPS: fps, Frames: frames, DurationMs: durationMs, KeyEvery: keyEvery}, nil
}

// onFrames is how many frames a length is, or the refusal naming the two
// nearest lengths that are a whole number of frames.
func onFrames(formatID, key string, ms int64, fps int) (int64, error) {
	if ms*int64(fps)%1000 == 0 && ms > 0 {
		return ms * int64(fps) / 1000, nil
	}
	step := stepMs(fps)
	below := ms / step * step
	above := below + step
	// The two lengths to ask for are part of the reason rather than a remedy
	// beside it, because the command line prints the reason and not the
	// remedy, and without them the refusal names a rule and no way out.
	reason := core.Says("video.NotOnAFrame",
		"at %d frames a second a length has to end on a frame, and lengths at that rate go in steps of %s. Ask for %s or %s",
		core.A("FPS", fps), core.A("Step", core.FormatDuration(step)),
		core.A("Below", core.FormatDuration(below)), core.A("Above", core.FormatDuration(above)))
	if below == 0 {
		reason = core.Says("video.NotOnAFrameShortest",
			"at %d frames a second a length has to end on a frame, and lengths at that rate go in steps of %s. Ask for %s, the shortest film at this rate",
			core.A("FPS", fps), core.A("Step", core.FormatDuration(step)), core.A("Above", core.FormatDuration(above)))
	}
	return 0, &format.PropertyValueError{Format: formatID, Key: key, Value: core.FormatDuration(ms), Reason: reason}
}

// stepMs is the shortest whole number of milliseconds that is a whole number
// of frames at this rate: 1000 divided by the greatest common divisor of 1000
// and the rate.
func stepMs(fps int) int64 {
	a, b := int64(1000), int64(fps)
	for b != 0 {
		a, b = b, a%b
	}
	return 1000 / a
}

// IsKey is whether frame i starts at a key frame.
func (t Timeline) IsKey(i int64) bool { return i%t.KeyEvery == 0 }

// Keys is how many key frames the film has.
func (t Timeline) Keys() int64 { return (t.Frames + t.KeyEvery - 1) / t.KeyEvery }

// Copies is how many hidden copies the film carries: one after each key frame
// that has a frame after it to show it in. A film whose every frame is a key
// frame needs none.
func (t Timeline) Copies() int64 {
	if t.KeyEvery == 1 {
		return 0
	}
	n := t.Keys()
	if (t.Keys()-1)*t.KeyEvery+1 >= t.Frames {
		n-- // the last key frame is the film's last frame
	}
	return n
}

// StartMs is when frame i is shown, in whole milliseconds rounded to the
// nearest, the way a container that counts in milliseconds stores it.
func (t Timeline) StartMs(i int64) int64 {
	return (i*1000 + int64(t.FPS)/2) / int64(t.FPS)
}
