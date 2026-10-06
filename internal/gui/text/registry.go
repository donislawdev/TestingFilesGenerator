package text

import (
	"strconv"
	"strings"

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
func FormatOwner(id string) Owner { return Owner(core.FormatOwner(id)) }
func PresetOwner(id string) Owner { return Owner("preset/" + id) }
func DamageOwner(id string) Owner { return Owner("damage/" + id) }

// ToolOwner is the fourth: the settings of a tool on the Tools tab.
func ToolOwner(id string) Owner { return Owner(toolPrefix + id) }

const toolPrefix = "tool/"

// WrittenAs is what the button beside a declared setting says about writing
// it down: the key a recipe writes it under, or for a tool - which no recipe
// runs - the flag of tfg tool. A sentence about a recipe beside a setting
// nobody can put in one would send somebody looking for a file that cannot
// hold it.
func WrittenAs(o Owner, key string) string {
	if strings.HasPrefix(string(o), toolPrefix) {
		return ToolFlag(key)
	}
	return SettingKey(key)
}

// The keys, built here and nowhere else - exported for the guard that writes
// registry/en.json out of the registries, so the window and the guard cannot
// come to ask for two different things.
func LabelKey(name string) string            { return core.LabelKey(name) }
func DetailKey(o Owner, name string) string  { return "Detail." + string(o) + "." + name }
func GroupKey(o Owner, group string) string  { return "Group." + string(o) + "." + group }
func UnitKey(unit string) string             { return core.UnitKey(unit) }
func ShapeKey(shape string) string           { return "Shape." + shape }
func JointKey(o Owner, of, by string) string { return core.JointKey(string(o), of, by) }
func QuestionKey(preset string) string       { return "Question." + preset }
func CatchKey(preset string, n int) string   { return "Catch." + preset + "." + strconv.Itoa(n) }
func NoteKey(preset, about string) string    { return "Note." + preset + "." + about }

// ChoiceKey is the key of one value of a closed list, by the recipe key of the
// setting the list belongs to - the same value of two settings may need two
// words, and the same setting declared by two formats means one thing.
func ChoiceKey(of, value string) string { return core.ChoiceKey(of, value) }

// The words of a tool, under keys of their own rather than beside a preset's:
// a tool and a preset may one day share a name, and "Question.<id>" would then
// be one key for two sentences.
func ToolQuestionKey(id string) string   { return "Tool." + id + ".Question" }
func ToolDetailKey(id string) string     { return "Tool." + id + ".Detail" }
func InputKey(id, name string) string    { return "Input." + id + "." + name }
func ToolNoteKey(id, note string) string { return "Tool." + id + ".Note." + note }

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

// FormatNameKey, TitleKey and LicenceKey are the keys of a format's name, a
// preset's title and one paragraph of the licence notice.
func FormatNameKey(id string) string { return "Format." + id }
func TitleKey(preset string) string  { return "Title." + preset }
func LicenceKey(n int) string        { return "Licence." + strconv.Itoa(n) }

// FormatName is what a format is called, beside its id in a list of formats.
// Most are the names of standards and stay as they are in every language.
func FormatName(id, english string) string { return lookup(FormatNameKey(id), english) }

// PresetTitle is what a preset is called, beside its id in a list of presets.
func PresetTitle(id, english string) string { return lookup(TitleKey(id), english) }

// LicenceParagraphs is the licence notice split into the paragraphs a
// translation keys, in the order they stand. The first is the name and the
// copyright line, which no language translates.
func LicenceParagraphs(notice string) []string {
	return strings.Split(strings.TrimSpace(notice), "\n\n")
}

// Licence is the licence notice in the window's language, paragraph by
// paragraph, and a sentence after it saying that the English text binds where
// the window speaks another language. The sentence is empty in English, so an
// English window shows the notice as it always has.
func Licence(notice string) string {
	paras := LicenceParagraphs(notice)
	out := make([]string, len(paras))
	for i, p := range paras {
		out[i] = lookup(LicenceKey(i+1), p)
	}
	if binds := LicenceBinds(); binds != "" {
		out = append(out, binds)
	}
	return strings.Join(out, "\n\n")
}

// ChoiceName is what a list shows for one of its values. The value itself is
// what a recipe, the command line and the manifest write, and what the list
// hands back - only the words on the screen are the window's.
func ChoiceName(of, value string) string { return lookup(ChoiceKey(of, value), value) }

// ToolQuestion is the question a tool answers, which is its title.
func ToolQuestion(id, english string) string { return lookup(ToolQuestionKey(id), english) }

// ToolDetail is the sentence saying what a tool does.
func ToolDetail(id, english string) string { return lookup(ToolDetailKey(id), english) }

// ToolInput is the sentence beside what a tool works on.
func ToolInput(id, name, english string) string { return lookup(InputKey(id, name), english) }

// ToolNote is a line a tool's result may carry, before its items.
func ToolNote(id, note, english string) string { return lookup(ToolNoteKey(id, note), english) }

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
