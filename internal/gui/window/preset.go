package window

import (
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

	pick   *widget.Select
	outDir *widget.Entry
	seed   *widget.Entry

	// params are the fields of the chosen preset, and paramBox is where they
	// go. Rebuilt whenever the preset changes.
	params   []parts.PropertyField
	paramBox *fyne.Container
	// about is the question this preset closes and what it typically catches.
	about *fyne.Container

	body fyne.CanvasObject
}

// NewPreset builds the screen. links are the buttons to the other screens.
func NewPreset(host Host, links ...fyne.CanvasObject) *Preset {
	p := &Preset{runner: newRunner(), host: host}
	p.runner.settle = p.settle

	p.paramBox = container.NewVBox()
	p.about = container.NewVBox()

	ids := preset.IDs()
	p.pick = widget.NewSelect(ids, p.onPresetChosen)
	p.outDir = entry(startingDirectory(), "")
	p.seed = entry("0", "")

	p.body = container.NewVScroll(parts.Screen(
		text.HeadingPreset,
		parts.Field(text.FieldPreset, text.HintPreset, p.pick),
		p.about,
		p.paramBox,
		parts.Field(text.FieldOutputDir, text.HintOutputDir,
			chooserFor(p.host, p.outDir)),
		parts.Field(text.FieldSeed, text.HintSeed, p.seed),
		p.actions(links...),
		p.progress(),
		p.problem.Object(),
	))

	if len(ids) > 0 {
		p.pick.SetSelected(ids[0])
	}
	return p
}

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
	if len(chosen.Catches) > 0 {
		p.about.Add(parts.Note(text.PresetCatchesHeading))
		for _, catch := range chosen.Catches {
			p.about.Add(parts.Note(text.PresetCatchesItem(catch)))
		}
	}
	p.about.Refresh()

	for _, param := range chosen.Parameters {
		field := parts.FromProperty(param)
		p.params = append(p.params, field)
		p.paramBox.Add(parts.Field(param.Name, detailOfParameter(param), field.Control))
	}
	p.paramBox.Refresh()
}

// detailOfParameter is what a parameter takes and what it is for, in that
// order - the same sentence the command line prints, from the declaration.
func detailOfParameter(param format.Property) string {
	detail := param.Allowed()
	if param.Detail != "" {
		detail += ". " + param.Detail
	}
	return detail
}

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

	seed, err := wholeNumber("seed", p.seed.Text)
	if err != nil {
		return nil, none, err
	}

	expanded, err := preset.Expand(p.pick.Selected, p.given())
	if err != nil {
		return nil, none, err
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
