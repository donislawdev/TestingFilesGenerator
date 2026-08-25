package window

import (
	"errors"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// Preset is the screen that starts from a question rather than from numbers.
//
// It is the other way into the same run. The generate screen asks what to
// produce, this one asks what is being tested and works the rest out - which is
// the thesis of the whole tool, and the reason a preset carries the question it
// closes and the mistakes it typically finds.
//
// The parameters are drawn by the same code that draws a format's settings,
// because a preset parameter IS a format.Property - one declaration type, one
// set of controls, one wording for a refusal. That is what makes this screen
// small: nothing here knows what size-boundaries takes.
type Preset struct {
	*runner

	host Host

	pick   *parts.Chooser
	outDir *widget.Entry
	seed   *widget.Entry

	// params are the fields of the chosen preset, and paramBox is where they
	// go. Rebuilt whenever the preset changes.
	params   []parts.PropertyField
	paramBox *fyne.Container
	// about is the question this preset closes and what it typically catches.
	about *fyne.Container

	// tips is the sheet the field explanations are drawn on. See Generate.
	tips *parts.Tips
	// fixed is how many fields this screen has before a preset declares any.
	fixed int

	body fyne.CanvasObject
}

// NewPreset builds the screen. links are the buttons to the other screens.
func NewPreset(host Host, links ...fyne.CanvasObject) *Preset {
	p := &Preset{runner: newRunner(), host: host, tips: parts.NewTips()}
	p.runner.settle = p.settle
	// Before the fields, because Add reads it. Only these two: every parameter
	// a preset declares has a default and leaving it alone is the whole point
	// of them - it is what the manifest records as defaulted, which is
	// untouchable rule 5 and the one thing this screen must never take away.
	p.fields.Require(engine.SettingOutDir, engine.SettingSeed)

	p.paramBox = parts.FieldColumn()
	p.about = parts.FieldColumn()

	ids := preset.IDs()
	p.pick = parts.NewChooser(ids, p.onPresetChosen)
	p.outDir = entry(startingDirectory(), "")
	p.seed = entry("0", "")

	// The same shape as the other screen, so the window is one interface rather
	// than two. What the preset is and what it finds go in the card with the
	// chooser, because they are the answer to the question that card asks.
	p.body = p.tips.Over(container.NewBorder(
		nil,
		parts.ActionBar(rail(append([]fyne.CanvasObject{donateButton(host)}, links...)...),
			p.actions(), p.progress(), p.problem.Object()),
		nil, nil,
		(p.keepScroll(container.NewVScroll(parts.Screen(
			text.HeadingPreset(),
			parts.Section(text.SectionPreset(),
				p.fields.Add(settingPreset, text.FieldPreset(), text.HintPreset(),
					p.tips.Say(text.DetailPreset()), p.pick),
				p.about,
			),
			parts.Section(text.SectionSettings(), p.paramBox),
			parts.Section(text.SectionOutput(),
				p.fields.Add(engine.SettingOutDir, text.FieldOutputDir(), text.HintOutputDir(),
					p.tips.Say(text.DetailOutputDir()), chooserFor(p.host, p.outDir)),
				p.fields.Add(engine.SettingSeed, text.FieldSeed(), text.HintSeed(),
					p.tips.Say(text.DetailSeed()), parts.Numeric(p.seed)),
			),
		)))),
	))

	// Everything built above belongs to the screen whatever preset is chosen.
	// What a preset declares comes after this mark and is replaced with it.
	p.fixed = p.fields.Len()

	if len(ids) > 0 {
		p.pick.SetSelected(ids[0])
	}

	// Said last, once the box it reads exists.
	p.runner.destination = p.OutDir
	p.runner.sayDestination()
	return p
}

// settingPreset is the key the field holding the preset's own name goes under.
//
// Not a recipe key, because a recipe has no such setting - a preset is the
// thing that BECOMES a recipe. It is here so that the box has a key like every
// other box rather than being the one exception, which is the whole point of
// the registry.
const settingPreset = "preset"

// Object is the screen, to put in a window.
func (p *Preset) Object() fyne.CanvasObject { return p.body }

// OutDir and SetOutDir keep one answer to "where do the files go" across both
// screens. The window carries it over on the way between them - see Open.
func (p *Preset) OutDir() string { return p.outDir.Text }

// SetOutDir leaves an empty value alone, for the reason the generate screen
// gives: an emptied box is somebody about to type, not an answer to copy.
func (p *Preset) SetOutDir(dir string) {
	if dir != "" {
		p.outDir.SetText(dir)
	}
}

// onPresetChosen redraws what the chosen preset says and what it takes.
func (p *Preset) onPresetChosen(id string) {
	p.paramBox.RemoveAll()
	p.about.RemoveAll()
	p.params = nil

	chosen, err := preset.Get(id)
	if err != nil {
		p.refuse(err)
		return
	}

	// The question first, because it is the thing somebody chooses by. A list
	// of ids says nothing about which one answers what they came to ask.
	p.about.Add(parts.Prose(chosen.Question))

	// One line each rather than one sentence. Run together they are unreadable,
	// because the entries have commas inside them - "MB confused with MiB,
	// which is 4.8 per cent and enough to let a file through that should not
	// pass" is one item, and joined with commas and an "and" it reads as three.
	// Seen on screen on 2026-08-05, which is the only way this kind of thing is
	// ever seen.
	//
	// A real list since 2026-08-12. Separate lines was the right call and the
	// drawing was not: the dash was typed into the string, so it sat in the
	// text and wrapped with it, and each item carried a full label's spacing -
	// which left more room between the items than around the whole list.
	if len(chosen.Catches) > 0 {
		p.about.Add(parts.Heading(text.PresetCatchesHeading()))
		p.about.Add(parts.Bullets(chosen.Catches))
	}
	p.about.Refresh()

	// The previous preset's fields go with their boxes. Left in the registry
	// they would be places a refusal could still be put, under widgets no
	// longer on the screen, and nothing would ever clear them.
	p.fields.KeepFirst(p.fixed)
	// What the preset declares, then the global settings it supplies a value
	// for. The same order "tfg preset show" prints them in, which is the whole
	// of why it is this way round rather than the other: two surfaces listing
	// one preset's settings differently is D1 fraying in a place nobody
	// compares.
	settings := make([]format.Property, 0, len(chosen.Parameters)+len(chosen.Reads))
	settings = append(settings, chosen.Parameters...)
	settings = append(settings, chosen.Globals()...)
	// The same call the other two screens make, since 2026-08-24. A preset
	// declares its parameters as the same format.Property a format declares its
	// settings as, and this screen used to draw them with a loop of its own -
	// so it was the third place drawing one declaration and the one nobody
	// remembered. It had already fallen behind twice: pairing two narrow
	// settings onto a row never reached it, and the count of bytes beside a
	// size had to be written here as well as there on the day it went in.
	//
	// Nothing looks different today, because the one preset this build
	// registers declares a single narrow parameter and there is no pair to
	// make. What it buys is the next thing, arriving on three screens instead
	// of two.
	fields, objects := parts.DeclaredFields(settings, p.fields, p.tips)
	p.params = fields
	for _, o := range objects {
		p.paramBox.Add(o)
	}
	p.paramBox.Refresh()
}

// detailOfParameter is gone as of 2026-08-24, and its history is why the loop
// above went with it. It held its own copy of the sentence describing a setting
// until 2026-08-19 and that copy had already drifted, so it was reduced to
// calling parts.PropertyDetail - a wrapper around the shared thing, kept
// because this screen drew its parameters itself. The screen calls
// parts.DeclaredFields now, which composes that sentence on the way, so there
// is nothing left for a wrapper to wrap.

// given is what the user typed, by parameter name. A field left empty is left
// out rather than sent as emptiness, because that is what makes the declared
// default stand in - and standing in is the thing the manifest then records.
func (p *Preset) given() preset.Args {
	out := preset.Args{}
	for _, f := range p.params {
		if v := f.Value(); v != "" {
			out[f.Name] = v
		}
	}
	return out
}

// settle expands the preset into the recipe it stands for and reads that.
//
// Source rather than a structure, and the same parser a handwritten file goes
// through - PR5. So what this screen runs is what "tfg preset eject" prints and
// what "tfg generate --preset" consumes, down to the bytes, because there is
// only one expansion and all three call it.
func (p *Preset) settle() ([]engine.Target, engine.Options, error) {
	var none engine.Options

	// Collected rather than returned one at a time, for the reason given in
	// spread: a form that marks one box however many are wrong makes somebody
	// fix their settings one press per mistake.
	//
	// Only the two that do not depend on each other. What a preset expands into
	// cannot be parsed until it has expanded, so those stay in order - a second
	// message about a recipe nobody could build would name a cause that is not
	// the cause.
	var bad []error

	seed, err := wholeNumber(engine.SettingSeed, text.FieldSeed(), p.seed.Text)
	if err != nil {
		bad = append(bad, err)
	}

	expanded, err := preset.Expand(p.pick.Selected, p.given())
	if err != nil {
		bad = append(bad, err)
	}
	if len(bad) > 0 {
		return nil, none, errors.Join(bad...)
	}

	rec, err := recipe.Parse(expanded.Source, p.pick.Selected)
	if err != nil {
		return nil, none, err
	}
	hash, err := recipe.Hash(expanded.Source)
	if err != nil {
		return nil, none, err
	}

	// What the run has to say out loud about a value nobody gave it.
	p.notes = expanded.Notes()

	targets := make([]engine.Target, 0, len(rec.Targets))
	for _, t := range rec.Targets {
		targets = append(targets, engineTarget(t))
	}

	// Which numbers were ours rather than only which preset this came from.
	// Untouchable rule 5: the limit of somebody else's system, replaced by one
	// we invented, produces a set carrying expectations that read exactly like a
	// set built around the real one.
	return targets, engine.Options{
		OutDir:       p.outDir.Text,
		Seed:         seed,
		Command:      "tfg-gui",
		ManifestName: engine.DefaultManifestName,
		RecipeHash:   hash,
		Preset: &manifest.Preset{
			ID:         expanded.Preset.ID,
			Parameters: map[string]string(expanded.Settled),
			Defaulted:  expanded.Defaulted,
		},
	}, nil
}

// engineTarget turns one recipe target into one engine target.
//
// The command line has the same conversion and the two cannot be shared - they
// sit on the same layer - so a guard runs one preset from both surfaces and
// compares the files and the manifest rather than trusting that these two lists
// of fields stay in step. That drift is not hypothetical: validate and generate
// each had one of these, and the one in validate had lost BoundaryLimit.
func engineTarget(t recipe.Target) engine.Target {
	return engine.Target{
		ID:               t.ID,
		Format:           t.Format,
		Sizes:            t.Sizes,
		Contains:         contentsOf(t),
		SizeFromContents: t.SizeFromContents,
		SizeIsRange:      t.SizeIsRange,
		SizeMin:          t.SizeMin,
		SizeMax:          t.SizeMax,
		BoundaryLimit:    t.BoundaryLimit,
		NameTmpl:         t.Name,
		Label:            t.Label,
		Expected:         t.Expected,
		ExpectedReason:   t.ExpectedReason,
		Group:            t.Group,
		Properties:       t.Properties,
	}
}

func contentsOf(t recipe.Target) []format.Content {
	if t.Contains == nil {
		return nil
	}
	out := make([]format.Content, 0, len(t.Contains))
	for _, c := range t.Contains {
		out = append(out, format.Content{Format: c.Format, Count: c.Count, Bytes: c.Bytes})
	}
	return out
}
