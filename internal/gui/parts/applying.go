package parts

import (
	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// A setting that changes a file only while another setting has some values -
// the colours of a placeholder, while the content is not the test pattern - is
// on the screen only while it does, and sends nothing while it does not. The
// owner's decision of 2026-11-17 (docs/SIZE-FIXES-2026-11-16.md section 6.5):
// four boxes that could only lead to a refusal were four boxes too many.
//
// The declaration says which settings these are (format.Condition), so nothing
// here names a format. Hidden rather than disabled, for the reason the switch
// between the ways of giving a size gives: a disabled box holding a value
// still reads as something the run will use. A hidden box keeps what was typed
// in it, so choosing the placeholder again gives it back.

// applying keeps the rows of one declaration on the screen exactly while
// their settings change anything. One row for each field, in the same order.
type applying struct {
	fields []PropertyField
	rows   []applyingRow
}

// applyingRow is one field as the form holds it.
type applyingRow struct {
	// row is what the form holds for the field - the one the grid skips while
	// it is hidden, which for a long text is the cell round the field.
	row fyne.CanvasObject
	// heading is the name of the block the setting belongs to, or nil. It goes
	// when every row under it has gone, since a name over nothing is a
	// heading for a block that is not there.
	heading fyne.CanvasObject
	// field is where a refusal about the setting is shown, taken back when
	// the row goes: a refusal about a value that is no longer sent is a
	// refusal about nothing, and left on a hidden row it would come back with
	// the row and read as a complaint about the choice just made.
	field *Field
}

// wire shows the rows that apply now, and has every setting another one
// depends on show them again whenever it changes. Nothing at all for a
// declaration with no condition, which is every one but a picture's today.
//
// The setting depended on is a menu - format.Register refuses a condition on
// anything else - and the menu is told once where to report (see wiredOnce):
// the batch screen lays the same fields out again on every rebuild, and a
// menu wired at each one would show the rows of every layout it ever had.
func (a *applying) wire() {
	depended := map[string]bool{}
	for _, f := range a.fields {
		if c := f.Declared.OnlyWhen; !c.IsZero() {
			depended[c.Setting] = true
		}
	}
	if len(depended) == 0 {
		return
	}
	for _, f := range a.fields {
		if menu, ok := f.Control.(*Chooser); ok && depended[f.Name] {
			chainOnce(&menu.OnChanged, &menu.shows, a, showAgain)
		}
	}
	a.show()
}

// showAgain is what a menu other settings depend on does after a choice: the
// rows it is pointed at are shown afresh.
func showAgain(_ string, to *applying) { to.show() }

// show puts on the screen the rows whose settings apply with the values the
// fields hold now, and takes the others off it.
func (a *applying) show() {
	headings := map[fyne.CanvasObject]bool{}
	for i, r := range a.rows {
		on := appliesAt(a.fields, i)
		r.put(on)
		if r.heading != nil {
			headings[r.heading] = headings[r.heading] || on
		}
	}
	for heading, on := range headings {
		if on {
			heading.Show()
		} else {
			heading.Hide()
		}
	}
}

// put shows the row, or hides it and takes back what was said about it.
func (r applyingRow) put(on bool) {
	if on {
		r.row.Show()
		return
	}
	r.row.Hide()
	if r.field != nil {
		r.field.Clear()
	}
}

// appliesAt is whether the setting of fields[i] changes anything with the
// values the fields hold now. A menu always holds one, so a setting depended
// on is never read as left out.
func appliesAt(fields []PropertyField, i int) bool {
	p := fields[i].Declared
	if p.OnlyWhen.IsZero() {
		return true
	}
	declared := make([]format.Property, 0, len(fields))
	for _, f := range fields {
		declared = append(declared, f.Declared)
	}
	return p.Applies(declared, func(name string) string { return valueIn(fields, name) })
}

// valueIn is what the field of one setting holds, or nothing when no field
// is that setting's.
func valueIn(fields []PropertyField, name string) string {
	for _, f := range fields {
		if f.Name == name {
			return f.Value()
		}
	}
	return ""
}

// StatedValues is what a declaration's fields ask for, under the keys
// declared: each value somebody left in a field, except an empty one - a
// setting not stated, which a format works its own answer out for - and one
// whose setting changes nothing with the values beside it, which is off the
// screen. Nil when that leaves nothing.
//
// One function for the screens that send a declaration's values, which each
// had a loop of their own until 2026-11-17: the setting that applies only
// under another's value would have had to be remembered in every one.
func StatedValues(fields []PropertyField) map[string]string {
	out := map[string]string{}
	for i, f := range fields {
		if v := f.Value(); v != "" && appliesAt(fields, i) {
			out[f.Name] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// SettingsSaid is what a folded section of a declaration's fields says about
// itself: each setting somebody chose, by its label and its value as the field
// shows it - a value of a list by its name in the list, since the line names
// what the fold hides and the menu inside says test pattern, not test, and a
// switch as on or off, since the switch inside says neither true nor false.
//
// What was CHOSEN, not what a field started on. A menu opens on its default
// and so always has a value, and listing all of them turned this line into
// every setting the format declares - which for log, with seven, ran off the
// edge of the window. And only what applies, since a section folded from the
// start that named a value nobody can see would be a setting they cannot find.
func SettingsSaid(fields []PropertyField) string {
	said := make([]string, 0, len(fields))
	for i, f := range fields {
		if f.Chosen != nil && !f.Chosen() {
			continue
		}
		v := f.Value()
		if v == "" || !appliesAt(fields, i) {
			continue
		}
		switch f.Declared.Kind {
		case format.PropertyChoice:
			v = text.ChoiceName(f.Name, v)
		case format.PropertyBool:
			v = text.SwitchState(v == "true")
		default:
			// A number, a size, a duration or a text reads as it was written.
		}
		said = append(said, text.SettingSaid(text.SettingLabel(f.Name), v))
	}
	return text.FoldedSummary(said...)
}
