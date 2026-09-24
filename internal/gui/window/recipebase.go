package window

import (
	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// The part of the batch screen that builds on a preset.
//
// A recipe may name a preset and add its own targets after the preset's -
// the extends and with keys, docs/RECIPE.md section 6 - and this is the same
// thing on the screen: a switch, a menu of presets, and the chosen preset's
// parameters drawn from its declaration the way the preset screen draws
// them. The batches below are then the recipe's own targets.
//
// A switch rather than a menu with a blank position, and the reason is what
// the screen has to be able to say. A menu that has been chosen from cannot
// be un-chosen, so a screen with only the menu could turn a preset on and
// never off again - and "off" is the ordinary state of this screen. The
// switch is that state, and the menu appears only while it is on.
//
// Its own type with its own methods rather than more of the screen's: the
// screen stood at its ceiling of methods, and what is here answers one
// question - what the recipe builds on - which the rest of the screen only
// asks.

// base is the preset the batch screen builds on, as the screen holds it.
type base struct {
	on   *parts.Toggle
	pick *parts.Chooser

	// declared is what the chosen preset takes - its parameters and the
	// globals it reads - and params are the controls drawn from it. Both are
	// replaced when the preset changes and kept across a rebuild, for the
	// reason the batch's own props are: a rebuilt box has forgotten what was
	// typed into it.
	declared []format.Property
	params   []parts.PropertyField
}

// newBase builds the controls, without placing them. The switch starts off,
// which is the recipe with no extends key.
func newBase(r *Recipe) *base {
	b := &base{}
	b.on = parts.NewToggle(func(on bool) {
		// Off with no batch left is a form that can produce nothing, so a
		// batch comes back - the one the screen opened with. What the form
		// then comes to is said by the switch's own check, as for any switch.
		if !on && len(r.batches) == 0 {
			newBatchAtTheEnd(r)
		}
		r.rebuild()
	})
	ids := preset.IDs()
	b.pick = parts.NewChooser(ids, func(id string) {
		if err := b.choose(id); err != nil {
			// The registry filled the list, so a press cannot land here. A
			// build where the two have come apart can, and saying so beats a
			// section with no settings and no reason given.
			r.refuse(err)
			return
		}
		// Chosen while the screen is still being built, before the screen
		// holds the base and before rebuild has anything to lay out.
		if r.base != nil {
			r.rebuild()
		}
	})
	// Chosen here rather than left empty, so a switch turned on shows a
	// preset with its parameters at once rather than a menu asking to be
	// opened first.
	//
	// The one the preset declares, not the first in the list. The list is in
	// alphabetical order, so "the first" moved the day a preset sorting
	// earlier was written - see preset.Landing.
	if landing := preset.Landing(); landing != "" {
		b.pick.SetSelected(landing)
	}
	return b
}

// choose replaces the chosen preset's settings.
func (b *base) choose(id string) error {
	chosen, err := preset.Get(id)
	if err != nil {
		return err
	}
	// What the preset declares, then the globals it supplies a value for -
	// the same order "tfg preset show" prints and the preset screen draws.
	settings := make([]format.Property, 0, len(chosen.Parameters)+len(chosen.Reads))
	settings = append(settings, chosen.Parameters...)
	settings = append(settings, chosen.Globals()...)
	b.declared = settings
	b.params = make([]parts.PropertyField, 0, len(settings))
	for _, p := range settings {
		b.params = append(b.params, parts.FromProperty(p))
	}
	return nil
}

// carriesTheRun says whether the preset's files are part of the run, which
// is when the switch is on. While it is, the screen may stand with no batch
// at all - a recipe of extends alone is legal, a preset run kept in a
// repository - and an outside review of #119 pointed out that the screen
// could not produce one, because the last batch had no Remove button.
func (b *base) carriesTheRun() bool { return b.on.Checked }

// rows are the part as it appears on the screen, registered under the keys a
// refusal about it arrives with: extends for the preset itself, and
// with.<name> for each parameter, which is the line in the recipe the value
// would be written on. The screen puts them in a section that folds - see
// sections.go.
func (b *base) rows(fields *parts.Fields, tips *parts.Tips) []fyne.CanvasObject {
	rows := []fyne.CanvasObject{
		parts.Note(text.NoteBase()),
		fields.AddToggle(settingBuildOnPreset, text.FieldBuildOnPreset(), "",
			tips.Say(text.DetailBuildOnPreset()), b.on),
	}
	if b.on.Checked {
		rows = append(rows, fields.Add(recipe.KeyExtends, text.FieldBasePreset(), text.HintBasePreset(),
			tips.Say(text.DetailBasePreset()), parts.Menu(b.pick)))
		rows = append(rows, b.parameterRows(fields, tips)...)
	}
	return rows
}

// parameterRows draws the chosen preset's parameters.
func (b *base) parameterRows(fields *parts.Fields, tips *parts.Tips) []fyne.CanvasObject {
	rows := make([]fyne.CanvasObject, 0, len(b.params))
	for i, f := range b.params {
		d := b.declared[i]
		if d.Kind == format.PropertySize {
			fields.InBytes(recipe.KeyWith + "." + f.Name)
		}
		rows = append(rows, fields.Add(recipe.KeyWith+"."+f.Name, text.SettingLabel(f.Name),
			parts.PropertyDetail(d), tips.Say(text.SettingKey(f.Name)), parts.ShapedFor(d, f.Control)))
	}
	return rows
}

// settingBuildOnPreset is the key the switch goes under.
//
// Not a recipe key, because a recipe has no such setting - a recipe either
// carries extends or it does not. It is here so that the switch has a key
// like every other control rather than being the one exception, the same
// reason the preset screen gives its own menu one.
const settingBuildOnPreset = "start_from_preset"

// draft is what the section says, ready to be composed: the preset's id when
// the switch is on, and every parameter somebody typed. A field left empty
// is left out, so the declared default stands in and the manifest records
// that it did.
func (b *base) draft() (extends string, with map[string]string) {
	if !b.on.Checked {
		return "", nil
	}
	with = map[string]string{}
	for _, f := range b.params {
		if v := f.Value(); v != "" {
			with[f.Name] = v
		}
	}
	if len(with) == 0 {
		with = nil
	}
	return b.pick.Selected, with
}
