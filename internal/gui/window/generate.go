package window

import (
	"errors"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
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
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// Host is what a screen needs of the window it lives in.
//
// An interface with three methods rather than fyne.Window, and the reason is
// testable rather than tidy: a screen that took the whole window would need a
// real one to be exercised, and a real one needs a C compiler and a graphics
// environment. fyne.Window satisfies this, and so does a stand in that records
// what it was handed - which is what lets a guard close the window mid run on a
// machine with no screen.
type Host interface {
	SetContent(fyne.CanvasObject)
	SetCloseIntercept(func())
	Close()
	// ChooseDirectory asks the person where the files should go and calls back
	// with what they picked, or with nothing at all if they changed their mind.
	//
	// It always calls back, and that is the contract rather than an accident.
	// An implementation that stays silent on cancel leaves the caller unable to
	// tell "cancelled" from "still open", and it also leaves the caller's
	// handling of the empty answer as code no test can reach - which is how a
	// field ends up cleared by a cancelled picker with nothing to catch it.
	//
	// Here rather than in the screens because it is the one thing a screen
	// needs that only a real window can do. A stand in answers it with a path,
	// which is what lets a guard press the button on a machine with no screen.
	ChooseDirectory(func(string))
}

// Generate is the screen that produces files from settings somebody chose.
//
// One target: a format, a size, how many, where they go. The preset screen
// beside it is the other way in - a named question rather than a set of
// numbers - and the two share everything after "what should be produced".
type Generate struct {
	*runner

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

	body fyne.CanvasObject
}

// NewGenerate builds the screen. links are the buttons to the other screens.
func NewGenerate(host Host, links ...fyne.CanvasObject) *Generate {
	g := &Generate{runner: newRunner(), host: host}
	g.runner.settle = g.settle
	g.buildFields()

	g.body = container.NewVScroll(parts.Screen(
		text.HeadingGenerate,
		g.settingsSection(),
		g.propBox,
		g.actions(links...),
		g.progress(),
		g.problem.Object(),
	))

	// The format decides which settings exist, so the first one has to be
	// applied rather than waited for.
	g.onFormatChosen(g.formatPick.Selected)
	return g
}

// Object is the screen, to put in a window.
func (g *Generate) Object() fyne.CanvasObject { return g.body }

// OutDir and SetOutDir are how the screens keep one answer to "where do the
// files go" between them. The window carries it across on the way from one to
// the other - see Open.
func (g *Generate) OutDir() string { return g.outDir.Text }

// SetOutDir leaves an empty value alone. Emptying the box is somebody clearing
// it to type, and copying that over would wipe the other screen's answer.
func (g *Generate) SetOutDir(dir string) {
	if dir != "" {
		g.outDir.SetText(dir)
	}
}

func (g *Generate) buildFields() {
	// Before the select below, which fills it as soon as a format is chosen.
	// The order is load bearing rather than tidy: choosing a format redraws the
	// settings that format declares, and the toolkit fires that the moment a
	// selection is set. Built the other way round this crashed on the way up,
	// which is how the order was found.
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
	g.name = entry("", text.PlaceholderNameTemplate)
	g.outDir = entry(startingDirectory(), "")
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
	// The fields the engine can name in a refusal keep an area of their own, so
	// the message lands under the box that caused it rather than at the foot of
	// the form - O73. The engine says which setting, in recipe keys, and this
	// is the only place that knows which box that is.
	g.beside = map[string]*parts.ErrorArea{}
	withArea := func(setting, label, hint string, control fyne.CanvasObject) fyne.CanvasObject {
		field, area := parts.FieldSaying(label, hint, control)
		g.beside[setting] = area
		return field
	}
	return container.NewVBox(
		parts.Field(text.FieldFormat, text.HintFormat, g.formatPick),
		parts.Field(text.FieldSize, text.HintSize, g.size),
		withArea(engine.SettingCount, text.FieldCount, text.HintCount, g.count),
		withArea(engine.SettingID, text.FieldTargetID, text.HintTargetID, g.id),
		withArea(engine.SettingName, text.FieldNameTemplate, text.HintNameTemplate, g.name),
		withArea(engine.SettingOutDir, text.FieldOutputDir, text.HintOutputDir,
			chooserFor(g.host, g.outDir)),
		parts.Field(text.FieldSeed, text.HintSeed, g.seed),
		parts.Toggle(text.FieldLabel, text.HintLabel, g.label),
	)
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
		g.refuse(err)
		return
	}

	fields, objects := parts.PropertyFields(d)
	g.props = fields
	if len(objects) == 0 {
		g.propBox.Refresh()
		return
	}
	g.propBox.Add(parts.Heading(text.SettingsFor(d.ID)))
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

// settle turns what is on the screen into what the engine takes.
//
// It parses and it does not judge. Every rule about whether the numbers make
// sense - the minimum of the format, the ceiling on files, a name that is a
// path, two files heading for one name - belongs to the engine and is asked of
// it. That is G1, and it is why the window cannot come to accept something the
// command line refuses.
func (g *Generate) settle() ([]engine.Target, engine.Options, error) {
	var none engine.Options

	bytesWanted, err := core.ParseSize(g.size.Text)
	if err != nil {
		return nil, none, err
	}
	count, err := wholeNumber("how many", g.count.Text)
	if err != nil {
		return nil, none, err
	}
	seed, err := wholeNumber("seed", g.seed.Text)
	if err != nil {
		return nil, none, err
	}

	// Asked before the list is built, because building it is the failure. The
	// same ceiling and the same sentence the command line uses, from core, so
	// there is no second opinion about how many files is too many. A count past
	// it used to reach make and panic with a stack trace.
	if count > core.MaxFilesPerRun {
		return nil, none, errors.New(text.TooManyFiles(count, core.ErrTooManyFiles))
	}

	return []engine.Target{{
			ID:         g.id.Text,
			Format:     g.formatPick.Selected,
			Sizes:      engine.Uniform(int(count), bytesWanted),
			NameTmpl:   g.name.Text,
			Label:      g.label.Checked,
			Properties: g.properties(),
		}}, engine.Options{
			OutDir:       g.outDir.Text,
			Seed:         seed,
			Command:      "tfg-gui",
			ManifestName: engine.DefaultManifestName,
		}, nil
}
