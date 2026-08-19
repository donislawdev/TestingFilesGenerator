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

	// OpenLink hands an address to whatever the desktop uses for the web.
	//
	// The program does not fetch it. It asks the system to, on a press somebody
	// made, and sends nothing - which is the line untouchable rule 8 draws and
	// the reason a Donate button does not break it. See the rule for the
	// wording, and TestTheDonateButtonOpensTheSupportPage for the guard.
	//
	// On the interface for the same reason as the picker above: only a real
	// window can do it, and a stand in records the address instead - so a guard
	// can press the button on a machine with no browser and still ask where it
	// was going to go.
	OpenLink(url string)
}

// Generate is the screen that produces files from settings somebody chose.
//
// One target: a format, a size, how many, where they go. The preset screen
// beside it is the other way in - a named question rather than a set of
// numbers - and the two share everything after "what should be produced".
type Generate struct {
	*runner

	host Host

	formatPick *parts.Chooser
	size       *widget.Entry
	count      *widget.Entry
	id         *widget.Entry
	name       *widget.Entry
	outDir     *widget.Entry
	seed       *widget.Entry
	label      *parts.Toggle

	// props are the fields drawn from whatever the chosen format declares, and
	// propBox is where they are put. Rebuilt on every change of format, because
	// the settings of a PNG are not the settings of a WAV.
	props   []parts.PropertyField
	propBox *fyne.Container
	// ready says the screen is far enough built for a change of format to
	// redraw anything. Setting the first selection fires the callback from
	// inside buildFields, before the mark below exists and before the fields
	// that never move have been registered - so without this the settings of
	// the opening format are registered twice and a refusal about one of them
	// lands under a box that is not the one it is about. It only showed when a
	// format declaring settings became the first in the menu.
	ready bool
	// fixed is how many fields this screen has before a format declares any.
	// What comes after is thrown away and drawn again on every change.
	fixed int

	// tips is the sheet the field explanations are drawn on. One per screen,
	// because a window holds every screen at once and an explanation has to
	// land on the one somebody is looking at.
	tips *parts.Tips

	body fyne.CanvasObject
}

// NewGenerate builds the screen. links are the buttons to the other screens.
func NewGenerate(host Host, links ...fyne.CanvasObject) *Generate {
	g := &Generate{runner: newRunner(), host: host, tips: parts.NewTips()}
	g.runner.settle = g.settle
	g.buildFields()

	// The actions sit outside the scrolling part, so they stay where they are
	// while the form moves. Inside it they were under the last field, which on
	// a form this tall meant scrolling to the end to press Generate - the same
	// complaint that moved the way between screens to the top.
	//
	// The progress and the refusal about the run as a whole belong with them:
	// what a run is doing is not a section of the form, and a message about a
	// press that just happened has to be where the press was.
	// The sheet goes over the whole screen rather than inside the scroll,
	// because a scroll clips what it draws - an explanation opened near the
	// foot of the form would be cut off at the edge of the viewport.
	g.body = g.tips.Over(container.NewBorder(
		nil,
		parts.ActionBar(g.actions(append([]fyne.CanvasObject{donateButton(host)}, links...)...),
			g.progress(), g.problem.Object()),
		nil, nil,
		container.NewVScroll(parts.Screen(
			text.HeadingGenerate,
			g.settingsSection(),
		)),
	))
	// Everything built above belongs to the screen whatever format is chosen.
	// What a format declares comes after this mark and is replaced with it.
	g.fixed = g.fields.Len()

	// The format decides which settings exist, so the first one has to be
	// applied rather than waited for.
	g.ready = true
	g.onFormatChosen(g.formatPick.Selected)

	// Said last, once the box it reads exists.
	g.runner.destination = g.OutDir
	g.runner.sayDestination()
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
	g.formatPick = parts.NewChooser(ids, g.onFormatChosen)
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

	g.label = parts.NewToggle("", nil)
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
	// Every field goes through the same call and every one of them can be told
	// it was the box that was refused - O73, and since 2026-08-12 without the
	// exceptions. The first argument is the key the engine names a setting by,
	// in recipe keys, and it is what a refusal is matched against.
	add := g.fields.Add
	return container.NewVBox(
		parts.Section(text.SectionConfiguration,
			// The format used to be a section of its own holding one field.
			// A grouping of one groups nothing, and it cost a title, a surface
			// and two gaps - measured at about 60 px on a screen that was 119
			// px too tall. It belongs here anyway: what kind of file, how big,
			// how many and what they are called are one set of questions.
			add(engine.SettingFormat, text.FieldFormat, text.HintFormat,
				g.tips.Say(text.DetailFormat), g.formatPick),
			// Side by side, because each pair is one thought: how big and how
			// many, then what the group is called and what the files are called.
			parts.Row(
				add(format.SettingSize, text.FieldSize, text.HintSize, g.tips.Say(text.DetailSize),
					parts.Numeric(g.size)),
				add(engine.SettingCount, text.FieldCount, "", parts.NoDetail, parts.Numeric(g.count)),
			),
			parts.Row(
				add(engine.SettingID, text.FieldTargetID, text.HintTargetID, g.tips.Say(text.DetailTargetID), g.id),
				add(engine.SettingName, text.FieldNameTemplate, text.HintNameTemplate,
					g.tips.Say(text.DetailNameTemplate), g.name),
			),
			// The settings the chosen format declares land here, under the ones
			// every format has.
			g.propBox,
		),
		parts.Section(text.SectionOutput,
			add(engine.SettingOutDir, text.FieldOutputDir, text.HintOutputDir, g.tips.Say(text.DetailOutputDir),
				chooserFor(g.host, g.outDir)),
			parts.Row(
				add(engine.SettingSeed, text.FieldSeed, text.HintSeed, g.tips.Say(text.DetailSeed),
					parts.Numeric(g.seed)),
				g.fields.AddToggle(engine.SettingLabel, text.FieldLabel, "", g.tips.Say(text.DetailLabel), g.label),
			),
		),
	)
}

// onFormatChosen redraws the settings the chosen format declares.
//
// Nothing here knows what a PNG or a WAV takes. The registry declares a name, a
// kind, a range or a closed set, a default and a sentence for each setting, and
// that is enough to draw the field - so a format that gains a setting gains its
// field, its wording and its refusal without a line of window code.
func (g *Generate) onFormatChosen(id string) {
	if !g.ready {
		return
	}
	g.propBox.RemoveAll()
	g.props = nil
	// The fields of the format that was chosen before this one go with their
	// boxes. Left in the registry they would be places a refusal could still be
	// put, under widgets no longer on the screen.
	g.fields.KeepFirst(g.fixed)

	d, err := format.Get(id)
	if err != nil {
		// The registry filled this menu, so this cannot happen from a press. It
		// can happen from a build where the two have come apart, and saying so
		// beats a screen with no settings on it and no reason given.
		g.refuse(err)
		return
	}

	fields, objects := parts.PropertyFields(d, g.fields)
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

	// Every box is read before any of them is given up on, so a screen with
	// three bad values marks three fields rather than the first one. See spread
	// in run.go for what carries them and why this is not the window inventing
	// a rule of its own.
	var bad []error

	bytesWanted, err := core.ParseSize(g.size.Text)
	if err != nil {
		bad = append(bad, saying(format.SettingSize, err))
	}
	count, err := wholeNumber(engine.SettingCount, text.FieldCount, g.count.Text)
	switch {
	case err != nil:
		bad = append(bad, err)
	// Asked before the list is built, because building it is the failure. The
	// same ceiling and the same sentence the command line uses, from core, so
	// there is no second opinion about how many files is too many. A count past
	// it used to reach make and panic with a stack trace.
	//
	// It names its box since 2026-08-18. It is a refusal about "how many" and
	// it went to the foot of the form with nothing marked, which is the same
	// defect as reading a number out of a box and losing the subject.
	case count > core.MaxFilesPerRun:
		bad = append(bad, saying(engine.SettingCount,
			errors.New(text.TooManyFiles(count, core.ErrTooManyFiles))))
	}
	seed, err := wholeNumber(engine.SettingSeed, text.FieldSeed, g.seed.Text)
	if err != nil {
		bad = append(bad, err)
	}

	if len(bad) > 0 {
		return nil, none, errors.Join(bad...)
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
