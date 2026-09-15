package window

import (
	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
)

// The column of names is as wide as the widest name the window can ever
// show, and it is worked out rather than written down - GUI rule 14 of
// CLAUDE.md: a layout is computed, never measured, because a number picked to
// fit today's words stops fitting at the first translation.
//
// Every name is asked, not only the ones a screen draws at rest: the settings
// a chosen format declares, a chosen damage's parameters and a preset's
// arrive after the screen is built, and a column that widened for them would
// move every control on the screen the moment somebody chose a format with a
// long setting name. So the width is one number for the whole window, from
// every source a name can come from.
//
// What is asked is the same wording the screens draw, through the same
// functions, so a name renamed in the catalogue is measured under its new
// spelling without anything here changing. TestEveryNameOnAScreenFitsTheColumnOfNames
// is what says the list below is complete: it reads the names off the built
// screens and asks whether each fits.

// labelColumn is the width of the column of names, worked out for each
// screen as it is built. Worked out again rather than kept, on purpose: a
// value kept across screens would need a lock or a once, and concurrency in
// this package is a decision the guards ask about by name. Measuring eighty
// short names costs less than drawing one of them.
func labelColumn() float32 {
	return parts.WidestName(everyName()...)
}

// EveryFieldName is every field name the window can show, from every source
// - for the guard that reads the names off the built screens and asks
// whether each is in this list.
func EveryFieldName() []string { return everyName() }

// everyName is every field name the window can show, from every source.
func everyName() []string {
	names := []string{
		text.FieldFormat(), text.FieldSize(), text.FieldCount(), text.FieldTargetID(),
		text.FieldNameTemplate(), text.FieldOutputDir(), text.FieldSeed(),
		text.FieldPreset(), text.FieldDamage(), text.FieldSizeRange(),
		text.FieldBoundary(), text.FieldGroup(), text.FieldExpected(),
		text.FieldReason(), text.FieldManifest(),
		// The label switch stands under its name in the column since 2026-09-15,
		// so its name is a name on a screen the column has to be wide enough
		// for - it was not, while the switch carried its own words.
		text.FieldLabel(),
	}
	for _, id := range format.IDs() {
		d, err := format.Get(id)
		if err != nil {
			continue
		}
		for _, p := range d.Properties {
			names = append(names, text.SettingLabel(p.Name))
		}
	}
	for _, id := range damage.Names() {
		d, err := damage.Get(id)
		if err != nil {
			continue
		}
		for _, p := range d.Parameters {
			names = append(names, text.SettingLabel(p.Name))
		}
	}
	for _, p := range preset.All() {
		for _, param := range p.Parameters {
			names = append(names, text.SettingLabel(param.Name))
		}
	}
	return names
}
