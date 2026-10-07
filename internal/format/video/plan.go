package video

import (
	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// Film is what planning settles for a film and writing makes it from: the
// picture and the settings. Both containers keep it as their plan's memo.
type Film struct {
	Choice   Choice
	Settings Settings
}

// Stream is the stream the film is written in, the one planning sized it by.
func (f Film) Stream() Stream {
	return NewStream(f.Settings.Timeline, f.Choice.Ceiling, f.Choice.Width, f.Choice.Height, f.Choice.Label)
}

// Plan settles a film for a request the way every container plans one, which
// differs between them only in what a stream costs and in the words of the
// refusal.
//
// need is the bytes a film of the stream takes in the container when every
// picture takes its whole reserve, with the smallest padding the container
// always writes - no film of that stream comes to more. refuse is the
// container's refusal when the smallest film it can make is larger than the
// request, naming what D6 asks for.
func Plan(formatID string, r format.Request, need func(Stream) int64, refuse func(requested, need int64, c Choice, s Settings) error) (format.Plan, error) {
	label := ""
	if r.Label {
		label = core.Label(formatID, r.Bytes, r.Seed)
	}
	s, err := Read(formatID, r.Properties)
	if err != nil {
		return format.Plan{}, err
	}
	fits := func(st Stream) bool { return need(st) <= r.Bytes }
	c, st, err := Choose(formatID, r, s, label, fits)
	if err != nil {
		return format.Plan{}, err
	}
	if n := need(st); n > r.Bytes {
		return format.Plan{}, refuse(r.Bytes, n, c, s)
	}

	p := format.Plan{
		Bytes:       r.Bytes,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties:  Facts(c, s),
		Memo:        Film{Choice: c, Settings: s},
		Work:        st.Work(),
	}
	if r.Label && !c.Labelled() {
		p.Notes = append(p.Notes, format.Note{
			Code:   "label_omitted",
			Detail: core.Says("format.ThePictureIsPxWideAnd", "The picture is %d px wide and the label needs more room, so this file carries no visible label. Its name and the manifest still identify it.", core.A("Width", c.Width)),
		})
	}
	if !c.ClockShown(s.Timeline) {
		p.Notes = append(p.Notes, format.Note{
			Code: "clock_omitted",
			Detail: core.Says("video.NoClock", "The picture is %dx%d and the clock needs more room, so this film shows no clock. The square still steps across it where it has room to move.",
				core.A("Width", c.Width), core.A("Height", c.Height)),
		})
	}
	return p, nil
}

// Hint is what a refusal below a film's minimum offers. A smaller picture only
// when the request named one: a picture chosen to fit is already the smallest
// that would, so offering a smaller one would send somebody to a setting that
// changes nothing. Every other knob is a setting, so each is named.
func Hint(need int64, c Choice) core.Said {
	if c.Named {
		return core.Says("video.AskForBOrMorePictures",
			"Ask for %d B or more, or a shorter film, fewer frames a second, key frames further apart, a new picture less often or a smaller picture",
			core.A("Floor", need))
	}
	return core.Says("video.AskForBOrMoreFilmPictures",
		"Ask for %d B or more, or a shorter film, fewer frames a second, key frames further apart or a new picture less often",
		core.A("Floor", need))
}
