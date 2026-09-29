package window

import (
	"context"
	"errors"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/tool"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/tool/all"
)

// Tools is the screen of the tools - docs/NARZEDZIA-SUMY-2026-09-29.md, the
// owner's decisions of 2026-09-29.
//
// Nothing on it is written for one tool. The list is the registry, the boxes
// are what the chosen tool declares - drawn by the same DeclaredFields the
// other three screens draw a format's settings with - and the result is the
// rows the tool hands back. So a tool added tomorrow is on this screen the
// day it is registered, the same as on the command line, which is D1 kept by
// construction rather than by remembering.
//
// The request is the only state (G3 read for a screen with no recipe): what
// is in the boxes when Run is pressed is what the tool is asked, the same
// request tfg tool builds from its flags.
type Tools struct {
	host     Host
	tips     *parts.Tips
	fields   *parts.Fields
	sections *sections
	// fixed is how many fields belong to the screen whatever tool is chosen;
	// the chosen tool's come after and are replaced with it.
	fixed int

	pick   *parts.Chooser
	titles map[string]string
	about  *fyne.Container
	work   *fyne.Container
	result *fyne.Container

	chosen   tool.Descriptor
	inputs   map[string]*parts.Entry
	settings []parts.PropertyField

	run    *parts.Button
	busy   *busy
	status *widget.Label
	fault  *parts.ErrorArea
	job    *toolJob

	body fyne.CanvasObject
}

// settingTool is the key the box choosing the tool goes under. Not a flag of
// anything - the tool is the word after "tfg tool" - but a key like every
// other box has, so the registry of fields has no exception in it.
const settingTool = "tool"

// NewTools builds the screen, with the first tool of the registry chosen.
func NewTools(host Host) *Tools {
	t := &Tools{host: host, tips: parts.NewTips(), fields: parts.NewFields(), sections: newSections()}
	t.titles = map[string]string{}
	questions := make([]string, 0, len(tool.Names()))
	for _, d := range tool.All() {
		q := text.ToolQuestion(d.ID, d.Question)
		t.titles[q] = d.ID
		questions = append(questions, q)
	}
	t.pick = parts.NewChooser(questions, t.onChosen)
	t.about = parts.FieldColumn()
	t.work = parts.Grid()
	t.result = parts.FieldColumn()

	t.run = parts.NewButton(parts.Primary, text.ButtonRunTool(), t.PressGenerate).InTheBar()
	cancel := parts.NewButton(parts.Secondary, text.ButtonCancel(), t.PressCancel).InTheBar()
	cancel.Disable()
	cancel.Hide()
	bar := parts.NewProgress()
	bar.Hide()
	t.busy = &busy{fields: t.fields, starters: []*parts.Button{t.run}, cancel: cancel, bar: bar, later: host.Later}
	t.busy.row = parts.ButtonRow(t.run, cancel)
	t.status = widget.NewLabel("")
	t.status.Wrapping = fyne.TextWrapWord
	t.fault = parts.NewErrorArea()

	t.body = t.tips.Over(container.NewBorder(
		nil,
		parts.ActionBar(rail(donateButton(host)), t.busy.row, roomToSpeak(bar, t.status, t.fault)),
		nil, nil,
		container.NewVScroll(parts.Screen(
			parts.Titled(text.TabTools(), text.SubtitleTools()),
			t.sections.section(sectionTool, text.SectionTool(),
				t.fields.Add(settingTool, text.FieldTool(), text.HintTool(), t.tips.Say(text.DetailTool()), t.pick),
				t.about,
			),
			t.sections.section(sectionToolInput, text.SectionToolInput(), t.work),
			t.sections.section(sectionToolResult, text.SectionToolResult(), t.result),
		)),
	))
	t.fixed = t.fields.Len()
	if len(questions) > 0 {
		t.pick.SetSelected(questions[0])
	}
	offerSettling(host, []interface{ Settled() }{t})
	return t
}

// Object is the screen, for the tab that holds it.
func (t *Tools) Object() fyne.CanvasObject { return t.body }

// FirstField is where the keyboard starts: which tool, because every box under
// it is drawn from that answer.
func (t *Tools) FirstField() fyne.Focusable { return t.pick }

// PressPreview does nothing: a tool reads and says, so there is no cost to
// work out before it runs. On the screen for the keyboard's sake - the window
// asks every screen the same four things.
func (t *Tools) PressPreview() {}

// PressCancel stops the tool that is running, if one is.
func (t *Tools) PressCancel() { t.Stop() }

// Stop ends the tool that is running and waits for it, or does nothing. Safe
// to call at any time, which is what lets closing the window ask every screen.
func (t *Tools) Stop() {
	if t.job != nil {
		t.job.stop()
	}
}

// Settled waits for the tool that is running to finish, for a guard.
func (t *Tools) Settled() {
	if t.job != nil {
		<-t.job.done
	}
}

// onChosen draws what the chosen tool works on and what it takes.
func (t *Tools) onChosen(question string) {
	d, err := tool.Get(t.titles[question])
	if err != nil {
		t.fault.Say(core.ShownText(err.Error()))
		return
	}
	t.chosen = d
	t.about.RemoveAll()
	t.about.Add(parts.Prose(text.ToolDetail(d.ID, d.Detail)))
	t.about.Refresh()

	t.fields.KeepFirst(t.fixed)
	t.work.RemoveAll()
	t.inputs = map[string]*parts.Entry{}
	for _, in := range d.Inputs {
		box := entry("", "")
		t.inputs[in.Name] = box
		t.work.Add(parts.Wide(t.fields.Add(in.Name, text.SettingLabel(in.Name), text.ToolInput(d.ID, in.Name, in.Detail),
			t.tips.Say(text.ToolInputWrittenAs(d.ID)), chooserFor(box, t.host.ChooseFile))))
	}
	settings, objects := parts.DeclaredFields(text.ToolOwner(d.ID), d.Settings, t.fields, t.tips)
	t.settings = settings
	for _, o := range objects {
		t.work.Add(o)
	}
	t.work.Refresh()
	t.showResult(nil)
}

// request is what the boxes say, as the tool is asked it - the same request
// tfg tool builds from its flags.
func (t *Tools) request() tool.Request {
	in := tool.Request{Inputs: map[string]string{}, Values: map[string]string{}}
	for name, box := range t.inputs {
		in.Inputs[name] = box.Text
	}
	for _, f := range t.settings {
		in.Values[f.Name] = f.Value()
	}
	return in
}

// PressGenerate runs the chosen tool - the name the window's keyboard asks
// every screen by.
func (t *Tools) PressGenerate() {
	if t.busy.occupied {
		return
	}
	t.fields.ClearAll()
	t.fault.Clear()
	showOn(t.status, "")
	t.job = startTool(t.chosen, t.request(), t)
}

// progressed and finished are what a running tool tells the screen, on the
// interface thread.
func (t *Tools) progressed(done, total int64) {
	t.busy.bar.SetValue(float64(core.Percent(done, total)))
	showOn(t.status, text.ToolReading(text.HumanBytes(done), text.HumanBytes(total)))
}

func (t *Tools) finished(r tool.Result, err error) {
	t.busy.set(false, busyFace{})
	showOn(t.status, "")
	if err != nil {
		t.refuse(err)
		return
	}
	t.showResult(&r)
}

// refuse puts a refusal under the box it is about, or at the foot of the
// screen when it is about none - the same two places every screen uses.
func (t *Tools) refuse(err error) {
	if errors.Is(err, context.Canceled) {
		showOn(t.status, text.ToolStopped())
		return
	}
	var about interface{ AboutSetting() string }
	if errors.As(err, &about) && t.fields.Mark(about.AboutSetting(), err) {
		t.sections.openHolding(t.fields.Lookup(about.AboutSetting()).Control)
		return
	}
	t.fault.Say(core.ShownText(err.Error()))
}

// showResult draws what a run found, or what will appear there before one.
func (t *Tools) showResult(r *tool.Result) {
	t.result.RemoveAll()
	defer t.result.Refresh()
	if r == nil {
		t.result.Add(parts.Prose(text.ToolNothingYet()))
		return
	}
	rows := parts.Grid()
	for _, row := range r.Rows {
		rows.Add(t.resultRow(row))
	}
	t.result.Add(rows)
	switch r.Verdict.Outcome {
	case tool.Match:
		t.result.Add(parts.Prose(text.ToolMatches(r.Verdict.About, r.Verdict.Got)))
	case tool.Mismatch:
		verdict := parts.NewErrorArea()
		verdict.Say(text.ToolDoesNotMatch(r.Verdict.About, r.Verdict.Got, r.Verdict.Wanted))
		t.result.Add(verdict.Object())
	case tool.Unasked:
	}
}

// resultRow is one row of a result: its first cell as the name in the column
// of names, the rest beside it, and a way to copy what is beside it. The name
// is data - an algorithm, a file - and is shown as it is (G8).
func (t *Tools) resultRow(row []string) fyne.CanvasObject {
	if len(row) == 0 {
		return parts.Prose("")
	}
	value := ""
	if len(row) > 1 {
		value = row[len(row)-1]
	}
	copyIt := parts.NewButton(parts.Secondary, text.ButtonCopy(), func() {
		t.host.Copy(value)
		showOn(t.status, text.ToolCopied(row[0]))
	})
	return parts.Wide(t.fields.Named(row[0], parts.NoDetail,
		container.NewBorder(nil, nil, nil, copyIt, parts.Prose(value))))
}

// toolJob is one run of a tool beside the window.
//
// Beside it because a file of gigabytes takes seconds to read and a window
// that waits for it is a window the desktop calls not responding - the same
// reason a run of the engine happens beside it. The stop is set before the
// goroutine starts, so closing the window at any moment of the run finds it
// and waits for the read to wind down (G7).
type toolJob struct {
	stop func()
	done chan struct{}
}

// startTool runs one tool and tells the screen how it went.
func startTool(d tool.Descriptor, in tool.Request, screen *Tools) *toolJob {
	ctx, cancel := context.WithCancel(context.Background())
	job := &toolJob{done: make(chan struct{})}
	job.stop = func() {
		cancel()
		<-job.done
	}
	screen.busy.set(true, busyFace{stoppable: true, progressing: true})
	screen.busy.bar.SetValue(0)
	limit := &throttle{}
	progress := func(done, total int64) {
		if !limit.allow(time.Now()) {
			return
		}
		fyne.Do(func() { screen.progressed(done, total) })
	}
	go func() {
		defer cancel()
		result, err := d.Start(ctx, in, progress)
		fyne.Do(func() { screen.finished(result, err) })
		close(job.done)
	}()
	return job
}
