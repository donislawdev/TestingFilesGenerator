package window

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	// The formats register themselves when this package is pulled in, and
	// without it the registry the menu is built from is empty.
	//
	// Found by running the built window rather than by any test, on 2026-08-05.
	// The list said "(Select one)" and had nothing under it, while every guard
	// was green - because the guard package pulls the registrations in for its
	// own use, so the registry is full there and was empty in the binary
	// somebody would actually be handed. The command line has carried the same
	// line in internal/cli since the beginning.
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// Host is what a screen needs of the window it lives in.
//
// An interface with two methods rather than fyne.Window, and the reason is
// testable rather than tidy: a screen that took the whole window would need a
// real one to be exercised, and a real one needs a C compiler and a graphics
// environment. fyne.Window satisfies this, and so does a stand in that records
// what it was handed - which is what lets a guard close the window mid run on a
// machine with no screen.
type Host interface {
	SetContent(fyne.CanvasObject)
	SetCloseIntercept(func())
	Close()
}

// Generate is the screen that produces the files.
//
// One target: a format, a size, how many, where they go. Not because a window
// deserves less than the command line - D1 says the opposite - but because this
// is the first of the screens and the rest of a recipe arrives with the one
// that edits recipes.
type Generate struct {
	host Host

	formatPick *widget.Select
	size       *widget.Entry
	count      *widget.Entry
	id         *widget.Entry
	name       *widget.Entry
	outDir     *widget.Entry
	seed       *widget.Entry
	label      *widget.Check

	// props are the fields drawn from whatever the chosen format declares, and
	// propBox is where they are put. Rebuilt on every change of format, because
	// the settings of a PNG are not the settings of a WAV.
	props   []parts.PropertyField
	propBox *fyne.Container

	previewBtn  *widget.Button
	generateBtn *widget.Button
	cancelBtn   *widget.Button

	bar     *widget.ProgressBar
	status  *widget.Label
	problem *parts.ErrorArea

	body fyne.CanvasObject

	// stop ends the run in progress. Set when one starts, nil the rest of the
	// time, and only ever touched on the interface thread.
	stop func()
}

// NewGenerate builds the screen. about is how the licence notice is reached.
func NewGenerate(h Host, about func()) *Generate {
	g := &Generate{host: h}
	// Progress first, and the order is load bearing rather than tidy. Choosing a
	// format redraws the settings that format declares, and the toolkit fires
	// that the moment a selection is set - so the box those settings go in, and
	// the area a refusal would go in, have to exist before the first selection
	// rather than after it. Built the other way round this crashed on the way
	// up, which is how the order was found.
	g.buildProgress()
	g.buildFields()
	g.buildActions()

	g.body = container.NewVScroll(parts.Screen(
		"Generate files",
		g.settingsSection(),
		g.propBox,
		g.actionsSection(about),
		g.progressSection(),
		g.problem.Object(),
	))

	// The format decides which settings exist, so the first one has to be
	// applied rather than waited for.
	g.onFormatChosen(g.formatPick.Selected)

	// Closing the window during a run is a cancellation and not a kill, G7. The
	// invariant that the output directory never holds a half written file rests
	// on the signal handler in cmd/tfg, and closing a window is not a signal -
	// so without this the run would carry on with nobody watching it, or die in
	// the middle of a file.
	h.SetCloseIntercept(g.onClose)
	return g
}

// Object is the screen, to put in a window.
func (g *Generate) Object() fyne.CanvasObject { return g.body }

func (g *Generate) buildFields() {
	// Before the select below, which fills it as soon as a format is chosen.
	g.propBox = container.NewVBox()

	// Every format this build registered, asked of the registry rather than
	// listed here. A list here would be a second place to edit, and the one that
	// gets forgotten - so a fourteenth format appears in this menu on the day it
	// is registered.
	ids := format.IDs()
	g.formatPick = widget.NewSelect(ids, g.onFormatChosen)
	if len(ids) > 0 {
		// The first rather than a favourite. Something has to be chosen, because
		// a menu showing nothing turns the first press of Generate into a
		// refusal about an empty value, and picking one by name here would be a
		// preference nothing else in this tool holds.
		g.formatPick.SetSelected(ids[0])
	}

	g.size = entry("10mb", "")
	g.count = entry("1", "")
	g.id = entry("files", "")
	g.name = entry("", "left empty: files_0001 and so on")
	g.outDir = entry(".", "")
	g.seed = entry("0", "")

	g.label = widget.NewCheck("", nil)
	g.label.SetChecked(true)
}

func entry(text, placeholder string) *widget.Entry {
	e := widget.NewEntry()
	e.SetText(text)
	e.SetPlaceHolder(placeholder)
	return e
}

// settingsSection is the target, in the order somebody fills it in.
//
// Every sentence under a field says what the field does and, where it matters,
// the consequence - which is docs/CLAUDE.md on writing for a reader, not a
// description of how any of it works.
func (g *Generate) settingsSection() fyne.CanvasObject {
	return container.NewVBox(
		parts.Field("format", "What kind of file to produce. Run out of the list and the tool has no other.", g.formatPick),
		parts.Field("size", "Exact size of every file. Units count in 1024s, so 10mb is 10485760 bytes. A plain number is a count of bytes.", g.size),
		parts.Field("how many", "How many files to produce.", g.count),
		parts.Field("target id", "Names the group. The seeds are derived from it, so changing it changes the bytes.", g.id),
		parts.Field("name template", "What the files are called. {index:04} becomes 0001, 0002 and so on.", g.name),
		parts.Field("output directory", "Where the files and the manifest go. It is created if it is not there.", g.outDir),
		parts.Field("seed", "The same seed gives the same bytes, on any machine.", g.seed),
		parts.Field("self describing label", "Writes into the file what it is and how big it was meant to be. Turn it off for a file that has to hold nothing but its content.", g.label),
	)
}

// actionsSection puts Preview before Generate, and that order is G6.
//
// It is the one thing a window does better than a command line rather than
// merely as well: --dry-run has to be known about and remembered, and this is
// on the way. With presets running to several gigabytes and disks that are not
// always emptier than that, it is not decoration.
func (g *Generate) actionsSection(about func()) fyne.CanvasObject {
	return container.NewHBox(
		g.previewBtn,
		g.generateBtn,
		g.cancelBtn,
		widget.NewButton("About", about),
	)
}

func (g *Generate) buildActions() {
	g.previewBtn = widget.NewButton("Preview", g.onPreview)
	g.generateBtn = widget.NewButton("Generate", g.onGenerate)
	g.generateBtn.Importance = widget.HighImportance

	g.cancelBtn = widget.NewButton("Cancel", g.onCancel)
	g.cancelBtn.Disable()
}

func (g *Generate) buildProgress() {
	g.bar = widget.NewProgressBar()
	// Counted as a percentage rather than as bytes, so the arithmetic that keeps
	// a very large run inside the range of its own type is the one the command
	// line already uses.
	g.bar.Max = 100
	g.bar.Hide()

	g.status = widget.NewLabel("")
	g.status.Wrapping = fyne.TextWrapWord
	g.problem = parts.NewErrorArea()
}

func (g *Generate) progressSection() fyne.CanvasObject {
	return container.NewVBox(g.bar, g.status)
}

// onFormatChosen redraws the settings the chosen format declares.
//
// Nothing here knows what a PNG or a WAV takes. The registry declares a name, a
// kind, a range or a closed set, a default and a sentence for each setting, and
// that is enough to draw the field - so a format that gains a setting gains its
// field, its wording and its refusal without a line of window code.
func (g *Generate) onFormatChosen(id string) {
	g.propBox.RemoveAll()
	g.props = nil

	d, err := format.Get(id)
	if err != nil {
		// The registry filled this menu, so this cannot happen from a press. It
		// can happen from a build where the two have come apart, and saying so
		// beats a screen with no settings on it and no reason given.
		g.problem.Say(err.Error())
		return
	}

	fields, objects := parts.PropertyFields(d)
	g.props = fields
	if len(objects) == 0 {
		g.propBox.Refresh()
		return
	}
	g.propBox.Add(parts.Heading("settings for " + d.ID))
	for _, o := range objects {
		g.propBox.Add(o)
	}
	g.propBox.Refresh()
}

// properties is what the user put in the generated fields, under the keys the
// format declared. An empty one is left out, because the registry reads a
// missing key and an empty value as the same thing - not stated - and a format
// that works its own answer out has to be allowed to.
func (g *Generate) properties() map[string]string {
	out := map[string]string{}
	for _, f := range g.props {
		if v := f.Value(); v != "" {
			out[f.Name] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
