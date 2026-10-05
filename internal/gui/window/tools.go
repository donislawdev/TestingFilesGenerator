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

	menu   toolMenu
	about  *fyne.Container
	work   *fyne.Container
	result *fyne.Container

	chosen tool.Descriptor
	form   toolForm

	busy   *busy
	status *widget.Label
	fault  *parts.ErrorArea
	job    *toolJob

	view  toolsView
	offer folderOffer
}

// toolMenu is the menu at the head of the screen, and which tool each of its
// lines stands for.
type toolMenu struct {
	pick   *parts.Chooser
	titles map[string]string
}

// toolsView is what the screen is drawn as: the view over the column of
// sections, kept to lay the screen out again (see relay), and the whole with
// the bar under it.
type toolsView struct {
	scroll *container.Scroll
	body   fyne.CanvasObject
}

// folderOffer is Open folder in the bar: hidden until a run has finished, and
// then leading to the folder that run worked in - the owner's list of
// 2026-10-05, in the bar because that is where Single batch offers its
// folder.
type folderOffer struct {
	btn *parts.Button
	// folder is where the run worked, kept from the request rather than read
	// off the boxes, which can be edited afterwards - the reason
	// offers.wroteInto gives.
	folder string
}

// show puts the button up once a run has finished, when there is a folder.
func (o *folderOffer) show(relay func()) {
	if o.folder == "" {
		return
	}
	o.btn.Show()
	relay()
}

// withdraw takes it away, for a run starting or another tool chosen.
func (o *folderOffer) withdraw(relay func()) {
	o.folder = ""
	o.btn.Hide()
	relay()
}

// toolForm is the boxes of the chosen tool: what goes into a request, before
// it is asked. Held apart from the screen because the three are drawn, thrown
// away and read together, and never one without the others - which is also
// what kept Tools under the ceiling on the fields of a type (2026-09-30).
type toolForm struct {
	// fixed is how many fields belong to the screen whatever tool is chosen.
	// The chosen tool's come after them and are replaced with it.
	fixed    int
	inputs   map[string]*parts.Entry
	settings []parts.PropertyField
}

// request is what the boxes say, as the tool is asked it - the same request
// tfg tool builds from its flags.
func (f toolForm) request() tool.Request {
	in := tool.Request{Inputs: map[string]string{}, Values: map[string]string{}}
	for name, box := range f.inputs {
		in.Inputs[name] = box.Text
	}
	for _, field := range f.settings {
		in.Values[field.Name] = field.Value()
	}
	return in
}

// settingTool is the key the box choosing the tool goes under. Not a flag of
// anything - the tool is the word after "tfg tool" - but a key like every
// other box has, so the registry of fields has no exception in it.
const settingTool = "tool"

// NewTools builds the screen, with the first tool of the registry chosen.
func NewTools(host Host) *Tools {
	t := &Tools{host: host, tips: parts.NewTips(), fields: parts.NewFields(), sections: newSections()}
	t.menu.titles = map[string]string{}
	choices := make([]string, 0, len(tool.Names()))
	for _, d := range tool.All() {
		c := text.ToolChoice(d.ID, d.Question)
		t.menu.titles[c] = d.ID
		choices = append(choices, c)
	}
	t.menu.pick = parts.NewChooser(choices, t.onChosen)
	t.about = parts.FieldColumn()
	t.work = parts.Grid()
	t.result = parts.FieldColumn()

	run := parts.NewButton(parts.Primary, text.ButtonRunTool(), t.PressGenerate).InTheBar()
	cancel := parts.NewButton(parts.Secondary, text.ButtonCancel(), t.PressCancel).InTheBar()
	cancel.Disable()
	cancel.Hide()
	t.offer.btn = parts.NewButton(parts.Secondary, text.ButtonOpenFolder(), func() {
		if t.offer.folder != "" {
			host.OpenFolder(t.offer.folder)
		}
	})
	t.offer.btn.InTheBar().Hide()
	bar := parts.NewProgress()
	bar.Hide()
	t.busy = &busy{fields: t.fields, starters: []*parts.Button{run}, cancel: cancel, bar: bar, later: host.Later}
	t.busy.row = parts.ButtonRow(run, cancel, t.offer.btn)
	t.status = widget.NewLabel("")
	t.status.Wrapping = fyne.TextWrapWord
	t.fault = parts.NewErrorArea()

	t.view.scroll = container.NewVScroll(parts.Screen(
		parts.Titled(text.TabTools(), text.SubtitleTools()),
		t.sections.section(sectionTool, text.SectionTool(),
			t.fields.Add(settingTool, text.FieldTool(), text.HintTool(), t.tips.Say(text.DetailTool()), t.menu.pick),
			t.about,
		),
		t.sections.section(sectionToolInput, text.SectionToolInput(), t.work),
		t.sections.section(sectionToolResult, text.SectionToolResult(), t.result),
	))
	t.view.body = t.tips.Over(container.NewBorder(
		nil,
		parts.ActionBar(rail(donateButton(host)), t.busy.row, roomToSpeak(bar, t.status, t.fault)),
		nil, nil,
		t.view.scroll,
	))
	t.form.fixed = t.fields.Len()
	if len(choices) > 0 {
		t.menu.pick.SetSelected(choices[0])
	}
	offerSettling(host, []interface{ Settled() }{t})
	return t
}

// Object is the screen, for the tab that holds it.
func (t *Tools) Object() fyne.CanvasObject { return t.view.body }

// FirstField is where the keyboard starts: which tool, because every box under
// it is drawn from that answer.
func (t *Tools) FirstField() fyne.Focusable { return t.menu.pick }

// Fields is the boxes of the screen, for the window to hand its shortcuts to -
// so Ctrl+Enter pressed in the box naming the file runs the tool, the way it
// runs the work of every other screen.
func (t *Tools) Fields() *parts.Fields { return t.fields }

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
func (t *Tools) onChosen(choice string) {
	d, err := tool.Get(t.menu.titles[choice])
	if err != nil {
		t.fault.Say(core.ShownText(err.Error()))
		return
	}
	t.chosen = d
	t.offer.withdraw(t.busy.relay)
	t.about.RemoveAll()
	t.about.Add(parts.Prose(text.ToolDetail(d.ID, d.Detail)))
	t.about.Refresh()

	t.fields.KeepFirst(t.form.fixed)
	t.work.RemoveAll()
	t.form.inputs = map[string]*parts.Entry{}
	for _, in := range d.Inputs {
		box := entry("", "")
		t.form.inputs[in.Name] = box
		t.work.Add(parts.Wide(t.fields.Add(in.Name, text.SettingLabel(in.Name), text.ToolInput(d.ID, in.Name, in.Detail),
			t.tips.Say(text.ToolInputWrittenAs(d.ID)), chooserFor(box, t.pickerFor(in.Kind)))))
	}
	settings, objects := parts.DeclaredFields(text.ToolOwner(d.ID), d.Settings, t.fields, t.tips)
	t.form.settings = settings
	for _, o := range objects {
		t.work.Add(o)
	}
	t.work.Refresh()
	t.showResult(d, nil)
}

// pickerFor is the window's picker for what an input names: a folder for a
// folder, a file for everything else.
func (t *Tools) pickerFor(kind tool.InputKind) func(func(string)) {
	if kind == tool.Folder {
		return t.host.ChooseDirectory
	}
	return t.host.ChooseFile
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
	t.offer.withdraw(t.busy.relay)
	in := t.form.request()
	t.offer.folder = ranIn(t.chosen, in)
	t.job = startTool(t.chosen, in, t)
}

// progressed and finished are what a running tool tells the screen, on the
// interface thread.
func (t *Tools) progressed(done, total int64) {
	t.busy.bar.SetValue(float64(core.Percent(done, total)))
	showOn(t.status, text.ToolReading(text.HumanBytes(done), text.HumanBytes(total)))
}

func (t *Tools) finished(d tool.Descriptor, r tool.Result, err error) {
	t.busy.set(false, busyFace{})
	showOn(t.status, "")
	if err != nil {
		t.offer.withdraw(t.busy.relay)
		t.refuse(err)
		return
	}
	t.showResult(d, &r)
	t.offer.show(t.busy.relay)
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
		t.relay()
		return
	}
	t.fault.Say(core.ShownText(err.Error()))
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
		fyne.Do(func() { screen.finished(d, result, err) })
		close(job.done)
	}()
	return job
}
