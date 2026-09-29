package text

import (
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// The words the engine's registries hold, in the language the window speaks -
// docs/JEZYKI-REJESTRY-2026-09-29.md.
//
// A format, a preset and a damage declare their settings with a sentence for a
// person, and the command line prints those sentences as they are: tfg formats
// --json and tfg preset show --json go to other people's scripts, so they stay
// English (D9). The window looks each one up under a key made of identifiers,
// and answers with the registry's English when its language has no entry - a
// sentence missing from a catalogue is an English sentence on screen, never an
// empty one, and a guard keeps every language this build carries complete.
//
// A folder of their own rather than entries in the window's catalogue, for two
// reasons read off the guards: en.json is written out of this package's code
// by tools/gen-locale.py and held to it, and every entry of another language
// has to be one en.json has. A registry sentence is neither - its English is
// in the registry, and registry/en.json is written out of the registries by the
// guard that holds it to them.
//
// Every entry in another language carries the hash of the English it
// translates, in the field go-i18n reserves for that and never shows. A key
// alone cannot tell that the English under it changed, and a Polish sentence
// describing last month's default is worse than an English one, because it
// looks translated.

// RegistryFolder is where the words of the registries are kept, inside the
// folder of the window's catalogue.
const RegistryFolder = "registry"

// Owner is whose declaration a setting is, which is half of the key its words
// are found under. Two formats both declare a width, and each says what it is
// for in a sentence of its own.
type Owner string

// FormatOwner, PresetOwner and DamageOwner are the three kinds of declaration.
//
// The global settings a preset reads - the format of the whole set - are the
// preset's here, because every screen draws them in one list with its
// parameters. The same English under two presets is one Polish sentence twice,
// and a guard holds the two to agreeing.
func FormatOwner(id string) Owner { return Owner("format/" + id) }
func PresetOwner(id string) Owner { return Owner("preset/" + id) }
func DamageOwner(id string) Owner { return Owner("damage/" + id) }

// The keys, built here and nowhere else - exported for the guard that writes
// registry/en.json out of the registries, so the window and the guard cannot
// come to ask for two different things.
func LabelKey(name string) string            { return "Label." + name }
func DetailKey(o Owner, name string) string  { return "Detail." + string(o) + "." + name }
func GroupKey(o Owner, group string) string  { return "Group." + string(o) + "." + group }
func UnitKey(unit string) string             { return "Unit." + unit }
func ShapeKey(shape string) string           { return "Shape." + shape }
func JointKey(o Owner, of, by string) string { return "Joint." + string(o) + "." + of + "." + by }
func QuestionKey(preset string) string       { return "Question." + preset }
func CatchKey(preset string, n int) string   { return "Catch." + preset + "." + strconv.Itoa(n) }
func NoteKey(preset, about string) string    { return "Note." + preset + "." + about }

// lookup is say for a sentence whose key is made of identifiers and whose
// English comes from a registry rather than from this package. Nothing to say
// stays nothing: a setting declared without a sentence has no entry to find.
func lookup(key, english string) string {
	if english == "" {
		return ""
	}
	return localise(key, english)
}

// SettingDetail is the sentence a declaration gives one of its settings.
func SettingDetail(o Owner, name, english string) string {
	return lookup(DetailKey(o, name), english)
}

// SettingGroup is the name of a block of settings - see format.Property.Group.
func SettingGroup(o Owner, group string) string { return lookup(GroupKey(o, group), group) }

// Unit is what a number counts, as a declaration names it: "pixels", "rows".
func Unit(unit string) string { return lookup(UnitKey(unit), unit) }

// Shape is what free text has to look like, as a declaration words it.
func Shape(shape string) string { return lookup(ShapeKey(shape), shape) }

// JointReason is why two settings multiplied together have a ceiling.
func JointReason(o Owner, of, by, english string) string {
	return lookup(JointKey(o, of, by), english)
}

// PresetQuestion is the question a preset answers.
func PresetQuestion(preset, english string) string { return lookup(QuestionKey(preset), english) }

// PresetCatches is what a preset typically finds, one line each, counted from
// one in the order the preset lists them.
func PresetCatches(preset string, english []string) []string {
	out := make([]string, len(english))
	for i, line := range english {
		out[i] = lookup(CatchKey(preset, i+1), line)
	}
	return out
}

// PresetNote is what a run says about a value of the preset nobody gave.
//
// A note about no value is one a preset put together from the values it was
// given, so there is no fixed sentence to have translated. It is said as it
// came, and the pseudo language leaves it undisguised on purpose: it IS English
// in every window, and that is what the pseudo language is for showing.
func PresetNote(preset, about, english string) string {
	if about == "" {
		return english
	}
	return lookup(NoteKey(preset, about), english)
}

// HumanBytes is core.HumanBytes with the decimal mark of the window's language.
//
// The pseudo language keeps the mark the number was written with: it disguises
// words, and a number is a value.
func HumanBytes(n int64) string {
	if pseudo {
		return core.HumanBytes(n)
	}
	return core.HumanBytesIn(n, DecimalMark())
}
