package window

import (
	"strconv"
	"strings"

	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// The parts of a batch that can be put away, and what a refusal has to open
// before it can mark a box.
//
// Split out of recipe.go on 2026-08-25 rather than for tidiness: the file went
// past the mark this project sets at three quarters of its ceiling, and folding
// is one subject - what is away by default, what a folded section says about
// itself, and how a refusal reaches a box two floors down. The size switch was
// split off for the same reason and by the same rule.

// addField is the way a batch puts one control on the screen under the address
// a refusal names it by. Named so that a section built outside batchBlock can
// still register its fields against the batch they belong to.
type addField func(setting, label, hint string, detail parts.Detail,
	control fyne.CanvasObject) fyne.CanvasObject

// summary is what this batch says about itself while it is folded away.
func (b *batch) summary() string {
	count := 0
	if n, err := strconv.Atoi(strings.TrimSpace(b.count.Text)); err == nil {
		count = n
	}
	return text.BatchSummary(b.id.Text, b.formatPick.Selected, count,
		b.statedSize(b.chosenSizeKey()))
}

// declaredSettings draws the fields the chosen format declares for one batch.
//
// Nothing here knows what a PNG or a WAV takes. What differs from the single
// batch screen is only the address: two batches of PNG both have a width box, so
// the batch has to be part of the name or a refusal about the second would mark
// the first.
func (r *Recipe) declaredSettings(b *batch, at func(string) string) fyne.CanvasObject {
	if len(b.props) == 0 {
		b.settings = nil
		return nil
	}

	var out []fyne.CanvasObject
	// The same pairing the single batch screen uses, through the same code.
	pair := parts.PairNarrow(r.fields.Row)
	for i, f := range b.props {
		// Shaped here as well as on the single batch screen. The two draw the
		// same declarations through different code, and only one of them was
		// given the width on the first try - which is how a difference between
		// two surfaces starts.
		object := r.fields.Add(at(recipe.KeyProperties+"."+f.Name), text.SettingLabel(f.Name),
			parts.PropertyDetail(b.declared[i]), r.tips.Say(text.SettingKey(f.Name)),
			parts.ShapedFor(b.declared[i], f.Control))
		if parts.Narrow(b.declared[i]) {
			pair.Add(object)
			continue
		}
		out = append(out, pair.Rest()...)
		out = append(out, object)
	}
	out = append(out, pair.Rest()...)
	// A rule binding two settings belongs beside them. Two number boxes drawn
	// from a range alone would offer a pair the run then refuses.
	if d, err := format.Get(b.formatPick.Selected); err == nil {
		for _, j := range d.JointLimits {
			out = append(out, parts.Note(j.Describe()))
		}
	}

	b.settings = parts.NewInnerFolding(text.SettingsFor(b.formatPick.Selected), out...)
	r.wire(b.settings, &b.settingsFolded, func() string { return b.settingsSaid() })
	return b.settings.Object()
}

// manifestNotes is the section describing the case rather than the files.
//
// What holds these three together and nothing else is that not one of them
// changes a byte of what is written - they are carried into the manifest and
// read back by whatever suite the files were made for. That is the owner's
// boundary of 2026-08-25 and it is what makes the section safe to fold away by
// default. The seed is the setting that reads as if it belonged here and does
// not: it changes every byte, and letting it in would leave the section with no
// rule at all. TestNothingInTheManifestNotesChangesAByte holds the line against
// the engine rather than against this comment.
func (r *Recipe) manifestNotes(b *batch, add addField) fyne.CanvasObject {
	b.notes = parts.NewInnerFolding(text.SectionManifestNotes(),
		parts.Note(text.NoteManifestOnly()),
		add(recipe.KeyGroup, text.FieldGroup(), text.HintGroup(),
			r.tips.Say(text.DetailGroup()), b.group),
		r.fields.Row(
			add(recipe.KeyExpected, text.FieldExpected(), text.HintExpected(),
				r.tips.Say(text.DetailExpected()), b.expected),
			add(recipe.KeyExpectedReason, text.FieldReason(), text.HintReason(),
				r.tips.Say(text.DetailReason()), b.reason),
		),
	)
	r.wire(b.notes, &b.notesFolded, func() string { return b.notesSaid() })
	return b.notes.Object()
}

// wire puts a section into the state it was left in and keeps it there across
// the rebuilds.
//
// Both halves are needed and it is worth saying which defect each closes. The
// remembered flag survives a rebuild - this screen builds every panel again
// whenever a batch is added, removed or has its format changed, so a fold
// living only in the panel would spring open under somebody's hands. The
// summary is worked out at the moment of folding rather than when the panel is
// built, because at build time nobody has typed anything yet and a line taken
// then is the summary of an empty section forever. That one was found by a
// guard on its first run, on the batch fold, in 2026-08-25.
func (r *Recipe) wire(fold *parts.Folding, folded *bool, say func() string) {
	fold.OnChange = func(open bool) {
		*folded = !open
		if !open {
			fold.Say(say())
		}
	}
	fold.Say(say())
	fold.Set(!*folded)
}

// settingsSaid is what the settings section says while it is away.
//
// Only what was stated. A section folded from the start that swallowed a value
// somebody typed would be a setting they cannot see and did not remove, which
// is worse than the scrolling folding was meant to save.
func (b *batch) settingsSaid() string {
	said := make([]string, 0, len(b.props))
	for _, f := range b.props {
		// What was CHOSEN, not what the field started on. A menu opens on its
		// default and so always has a value, and listing all of them turned
		// this line into every setting the format declares - which for log,
		// with seven, ran off the edge of the window.
		if f.Chosen != nil && !f.Chosen() {
			continue
		}
		if v := f.Value(); v != "" {
			said = append(said, text.SettingSaid(text.SettingLabel(f.Name), v))
		}
	}
	return text.FoldedSummary(said...)
}

// notesSaid is what the manifest notes say while they are away.
func (b *batch) notesSaid() string {
	return text.FoldedSummary(b.group.Text, b.expected.Selected, b.reason.Selected)
}

// openFoldHolding opens everything a box has been put away inside.
//
// A refusal names the box it is about and the form scrolls to it, and neither
// of those can show a box that has been folded away. This is what makes folding
// safe to have on this screen at all: the objection recorded on 2026-08-18
// against putting batches away was that a batch off the screen has no box to
// mark, and the answer is that the screen opens it rather than that it is never
// away.
//
// EVERY fold, not the outermost one. Since 2026-08-25 a batch has sections
// inside it that are folded from the start, so a refusal about a format setting
// is two floors down - and opening the batch while leaving the section shut
// would refuse the run and mark nothing anybody can see, which is the very
// defect the paragraph above says was answered. This does not walk floors
// either: it asks each fold whether the control is anywhere inside it, and a
// fold containing another answers for both.
//
// Asked of the tree rather than of a list of addresses per section. A list
// would be a second place saying where a field lives, and it is the copy that
// drifts - see parts.Folding.Holds.
//
// A box it cannot find leaves everything as it was. Refusals about the output
// directory or the manifest are not inside anything.
func (r *Recipe) openFoldHolding(address string) {
	field := r.fields.Lookup(address)
	if field == nil {
		return
	}
	for _, b := range r.batches {
		for _, fold := range []*parts.Folding{b.fold, b.settings, b.notes} {
			if fold != nil && fold.Holds(field.Control) {
				fold.Set(true)
			}
		}
	}
}
