package text

import "fmt"

// Headings, one per screen. They name what the screen is for rather than what
// it contains, which is why the preset one is a question.
const (
	HeadingGenerate = "Generate files"
	HeadingPreset   = "Build a set for a question"
)

// HeadingAbout carries the version, so the screen somebody is told to look at
// when reporting a problem says which build they are on.
func HeadingAbout(version string) string {
	return "Testing Files Generator " + version
}

// AboutTagline is the one sentence saying what this tool is. It is the product
// thesis in a line: files, and what should happen to them.
const AboutTagline = "Generate test files, and know how the system under test should react to them."

// The tabs across the top, which are where moving between screens lives.
//
// They were buttons in the row of actions under the last field until
// 2026-08-11, so changing screen meant scrolling past the whole form to find
// the way out - reported from use. Moving between screens is not an action
// taken on the form, and sitting among Preview and Generate it read like one.
// TabOneTarget was "One target" until 2026-08-12. A target is a first class
// idea in the recipe, in the manifest and in every document, and it is not one
// anywhere else - so the first word somebody met on opening this tool was ours.
// The key keeps the old name because a key names the place rather than the
// wording.
const (
	TabOneTarget = "Single batch"
	TabPresets   = "Presets"
	TabAbout     = "About"
)

// Buttons that do something on the screen they are on.
const ButtonChoose = "Choose..."

// The sections a screen is grouped into. A form of eight settings in one column
// reads as eight unrelated things, and these are the questions they answer.
const (
	// SectionFormat is gone as of 2026-08-12. It headed a section holding one
	// field, which groups nothing and cost a title, a surface and two gaps on
	// a screen that did not have the room. The format moved into the section
	// below it, where the rest of "what should this file be" already lived.
	SectionConfiguration = "File configuration"
	SectionOutput        = "Output"
	SectionPreset        = "Preset"
	SectionSettings      = "Settings"
	SectionLicence       = "Licence"
)

// Field labels on the generate screen, in the order somebody fills them in.
//
// Sentence case since 2026-08-12. They were lower case throughout while the
// sections above them were capitalised, so one screen used two conventions and
// neither was a choice - the labels were the identifiers they came from.
//
// Two of them stopped being our vocabulary at the same time. "target id" named
// an idea that exists in the recipe and nowhere in anybody's head, and "self
// describing label" named the mechanism instead of saying what happens. The
// recipe keys, the flags and the manifest fields are all untouched: this is
// what the window shows, and those are a contract.
const (
	FieldFormat       = "Format"
	FieldSize         = "Size"
	FieldCount        = "How many"
	FieldTargetID     = "Group name"
	FieldNameTemplate = "File names"
	FieldOutputDir    = "Output directory"
	FieldSeed         = "Seed"
	FieldLabel        = "Write a label inside each file"
	FieldPreset       = "Preset"
)

// The line under each field: what it does, in one line, and nothing else.
//
// Split from the longer explanations below on 2026-08-12. Every field used to
// carry its whole sentence permanently, so the generate screen held eight grey
// paragraphs and they took more room than the controls did - the eye could not
// find the inputs. What is left here is the part somebody reads while filling
// the form in.
//
// HintCount is gone rather than shortened. It said "How many files to produce"
// under a field labelled "How many", which is the label again in a quieter
// colour.
const (
	HintFormat       = "What kind of file to produce."
	HintSize         = "Exact size of every file."
	HintTargetID     = "Names this group of files."
	HintNameTemplate = "What the files are called."
	HintOutputDir    = "Where the files and the manifest go."
	HintSeed         = "The same seed gives the same bytes."
	HintPreset       = "What you are testing."
)

// The longer explanation behind the button beside a field name.
//
// What somebody needs once rather than every time: the units, the example, the
// consequence. Kept because each of these says something that cannot be worked
// out from the label - that changing the group name changes the bytes, that
// 10mb is not ten million, that a seed carries across machines.
//
// DetailLabel holds the whole explanation because the switch says what it does
// on its own face, so there is no short line left to put under it.
const (
	DetailFormat       = "Run out of the list and the tool has no other."
	DetailSize         = "Units count in 1024s, so 10mb is 10485760 bytes. A plain number is a count of bytes."
	DetailTargetID     = "The seeds are derived from it, so changing it changes the bytes."
	DetailNameTemplate = "{index:04} becomes 0001, 0002 and so on."
	DetailOutputDir    = "It is created if it is not there."
	DetailSeed         = "On any machine and in any build of this tool."
	DetailLabel        = "Writes into the file what it is and how big it was meant to be. Turn it off for a file that has to hold nothing but its content."
	DetailPreset       = "The set is worked out from the answer."
)

// PlaceholderNameTemplate stands in the empty name box. It shows the answer
// rather than describing it, because a name template is easier recognised than
// explained.
//
// The words "left empty" came off it on 2026-08-12. A placeholder is what the
// box will hold, and a placeholder explaining its own absence is documentation
// in the one place that disappears the moment somebody types.
const PlaceholderNameTemplate = "files_0001"

// PresetCatchesHeading introduces what a preset typically finds.
const PresetCatchesHeading = "Typically finds:"

// PresetCatchesItem is gone as of 2026-08-12. It put the list marker into the
// string - "   - " in front of the words - so the marker sat on the text
// baseline and wrapped along with it, and a wrapped item carried on underneath
// its own marker. The marker is a column of its own now, drawn by parts.Bullets,
// which leaves nothing here to word.

// PlaceholderWorkedOut stands in a format setting the format decides for
// itself when nobody states it. A declaration with no default means the answer
// comes from the size that was asked for.
const PlaceholderWorkedOut = "worked out from the size"

// PlaceholderLeftEmpty stands in a format setting that has a declared default,
// showing what happens if the field is not touched.
//
// The field is deliberately NOT filled in with that default. A window whose
// every field arrives carrying a value cannot say "I did not state this", and
// untouchable rule 5 needs that difference - it is what tells a limit somebody
// chose from a placeholder we invented.
//
// The value alone since 2026-08-12, where it used to read "left empty: 10mb".
// That the value is what happens when nothing is typed is what a placeholder
// already means, and the sentence under the field says "default 10mb" in words
// - so the box was carrying a third copy in the one place that vanishes as soon
// as anybody uses it.
func PlaceholderLeftEmpty(declared string) string {
	return declared
}

// SettingsFor heads the block of fields a chosen format declares.
//
// A function rather than a constant with the id glued on: languages do not
// agree on where a name goes in a phrase, and this is the seam that lets one
// move it without finding every caller.
func SettingsFor(formatID string) string {
	return "settings for " + formatID
}

// TooManyFiles refuses a run before the list is built, because building it is
// the failure. The reason comes from core, so the window and the command line
// do not hold two opinions about how many files is too many.
func TooManyFiles(count int64, reason error) string {
	return fmt.Sprintf("this run asks for %d files - %s", count, reason)
}
