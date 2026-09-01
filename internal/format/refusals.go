package format

// What a format says when a setting is wrong.
//
// Split out of format.go on 2026-09-01, when the crowding gate went red:
// three files reached the band and the cap is two. The cut is by subject
// rather than by size. format.go is what a format DECLARES about itself -
// its kinds, its properties, the rules binding two of them - and this is
// what it SAYS when somebody asks for something it cannot give.
//
// Four refusals live here and they are four different mistakes, which is
// why they are four types rather than one with a code. A key nobody
// declares is a typo and lands on USAGE. A key this format cannot carry is
// a fact about the file format and lands on FORMAT. A declared key with a
// value outside its range is the caller's mistake and lands on FORMAT too,
// but says something different about what to do next.

import (
	"fmt"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// UnsupportedSetting is a setting a format deliberately cannot take, with the
// reason a person needs to hear.
//
// It exists because "targz does not have a property called password" is true
// and unhelpful. It reads as a gap in this build, and somebody would go looking
// for the version that has it. The truth is about the format: tar has no
// encryption at all, anywhere in its specification, and no version of this tool
// will give it any.
//
// Worth stating rather than leaving to the generic refusal because the
// alternative in the world is worse than silence. Measured on 2026-09-01: 7-Zip
// accepts -p on a tar and on a gzip, exits 0, says nothing, and writes a
// PLAINTEXT archive. Somebody asking for an encrypted fixture gets an
// unencrypted one and no warning - untouchable rule 6 broken in somebody else's
// tool. Refusing is the opposite of that, and refusing with the reason is what
// stops the person hunting for a flag that cannot exist.
//
// Declared per format rather than worked out, because only the format knows.
// A container arriving later has its own gaps - tar has no entry comment, zip
// has no owner or mode - and each is a sentence somebody has to write.
type UnsupportedSetting struct {
	// Name is the key a recipe would write.
	Name string
	// Why is the reason, in the words the refusal uses. It is about the file
	// format, not about this build.
	Why string
	// Instead is what to do about it, and it is allowed to be empty when there
	// is genuinely nothing else to do.
	Instead string
}

// UnsupportedSettingError is a declared setting this format cannot carry.
//
// Its own type rather than UnknownPropertyError because the mistake is
// different and so is the exit code. A key nobody declares is a typo, which is
// USAGE. A key this format cannot carry is a well formed request no format
// here can deliver for this file, which is FORMAT - the same class as an
// archive asked to nest inside itself.
type UnsupportedSettingError struct {
	Format string
	Key    string
	Reason string
	Remedy string
}

// AboutSetting is the key this refusal is about, so a form can mark the box.
func (e *UnsupportedSettingError) AboutSetting() string { return e.Key }

func (e *UnsupportedSettingError) What() string {
	return fmt.Sprintf("%s cannot take %q", e.Format, e.Key)
}

func (e *UnsupportedSettingError) Why() string { return e.Reason }

func (e *UnsupportedSettingError) Instead() string { return e.Remedy }

func (e *UnsupportedSettingError) Error() string {
	if e.Remedy == "" {
		return e.What() + " - " + e.Reason
	}
	return e.What() + " - " + e.Reason + ". " + e.Remedy
}

// cannotCarry says whether this format has declared the key as one it cannot
// take, and gives the refusal if it has.
func (d Descriptor) cannotCarry(key string) (*UnsupportedSettingError, bool) {
	for _, u := range d.Unsupported {
		if u.Name == key {
			return &UnsupportedSettingError{
				Format: d.ID, Key: key, Reason: u.Why, Remedy: u.Instead,
			}, true
		}
	}
	return nil, false
}

// UnknownPropertyError is a property key no format recognises.
type UnknownPropertyError struct {
	Format string
	Key    string
	Known  []string
}

// AboutSetting is the property this refusal is about, so a form can put the
// message under the box it came from. Its sibling below has carried this since
// 2026-08-12 and this one did not, which made a mistyped key the one refusal
// about a declared setting that still landed at the foot of the form.
func (e *UnknownPropertyError) AboutSetting() string { return e.Key }

// Why this is refused, for a report that keeps the four parts of D6 apart.
func (e *UnknownPropertyError) Why() string {
	return "a format takes only the settings it declares, and one it does not know would be dropped on the way"
}

// Instead is what to do about it, named from the declaration.
func (e *UnknownPropertyError) Instead() string {
	if len(e.Known) == 0 {
		return "remove the line"
	}
	return "use one of: " + strings.Join(e.Known, ", ")
}

// What happened, without the list of names. Kept apart from Error so a report
// with four parts does not print the names twice - once in the sentence and
// again in what to do instead.
func (e *UnknownPropertyError) What() string {
	if len(e.Known) == 0 {
		return fmt.Sprintf("%s takes no properties, so %q is not one of them", e.Format, e.Key)
	}
	return fmt.Sprintf("%s does not have a property called %q", e.Format, e.Key)
}

// Error is the whole thing in one sentence, unchanged to the character - it is
// what the one-target path from the command line flags prints.
func (e *UnknownPropertyError) Error() string {
	if len(e.Known) == 0 {
		return e.What()
	}
	return e.What() + ". It takes: " + strings.Join(e.Known, ", ")
}

// PropertyValueError is a declared key given a value the declaration forbids.
//
// Separate from UnknownPropertyError because the mistake is different and so
// is the fix: one is a key that does not exist, the other a value out of
// range. Both are the caller's doing rather than the tool's, which is the
// point - this used to surface as a plain error and end with the exit code
// that means the program itself failed, so CI could not tell "you typed that
// wrong" from "this build has a bug".
type PropertyValueError struct {
	Format string
	Key    string
	Value  string
	Reason string
	// Remedy is what to do about it, built from the declaration. It is carried
	// here rather than worked out by whoever reports this, because a refusal in
	// this tool has four parts - what happened, why, what is allowed, what to do
	// instead (D6) - and the fourth had nowhere to come from until 2026-08-25.
	// A reader that wants the whole thing in one sentence still gets it from
	// Error, which leaves this out: it is the part a form puts under the box.
	//
	// Named Remedy rather than Instead because the accessor below has to be
	// called Instead - that is the name the other refusals in this package use
	// for the same part, and the reader that asks for all three asks by name.
	Remedy string
}

// What happened, why the declaration forbids it, and what to do instead.
//
// Instead is the part Error leaves out on purpose - see the field - so this is
// the only way a report gets all four parts of D6 for the refusal a person hits
// most often, by typing a number.
func (e *PropertyValueError) What() string {
	return fmt.Sprintf("%s: %s cannot be %q", e.Format, e.Key, e.Value)
}

func (e *PropertyValueError) Why() string { return core.InTheWordsOf(e.Reason, e.Key) }

func (e *PropertyValueError) Instead() string { return e.Remedy }

func (e *PropertyValueError) Error() string {
	return e.InTheWordsOf(e.Key)
}

// InTheWordsOf is this refusal with the property named the way one surface
// names it - the declared key on the command line, the label above the box in
// a window. See core.SettingSlot.
//
// The name is a field here rather than a slot in a sentence, because this
// refusal is assembled rather than written out: the key already stands on its
// own in the format string. The reason a property gives can still hold slots,
// so a declaration that names itself twice needs no special case.
func (e *PropertyValueError) InTheWordsOf(name string) string {
	if name == "" {
		name = e.Key
	}
	return fmt.Sprintf("%s: %s cannot be %q - %s",
		e.Format, name, e.Value, core.InTheWordsOf(e.Reason, name))
}

// AboutSetting is the property this refusal is about, so a window can put the
// message under the box it came from.
//
// It was missing until 2026-08-12, and the gap is worth recording because it
// was invisible from either side. A window cannot place a message it is not
// told the subject of, so every refusal about a declared setting - width,
// height, pages, entries, a preset's own parameters - landed at the foot of the
// form however carefully the window was written. Two of the three interfaces
// beside this one had the method. This one is the one that fires most often,
// because it is the one a person hits by typing a number.
func (e *PropertyValueError) AboutSetting() string { return e.Key }
