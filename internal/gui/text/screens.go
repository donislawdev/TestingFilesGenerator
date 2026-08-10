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

// Buttons that move between screens rather than doing anything.
const (
	ButtonPresets   = "Presets"
	ButtonAbout     = "About"
	ButtonOneTarget = "One target"
	ButtonBack      = "Back"
	ButtonChoose    = "Choose..."
)

// Field labels on the generate screen, in the order somebody fills them in.
const (
	FieldFormat       = "format"
	FieldSize         = "size"
	FieldCount        = "how many"
	FieldTargetID     = "target id"
	FieldNameTemplate = "name template"
	FieldOutputDir    = "output directory"
	FieldSeed         = "seed"
	FieldLabel        = "self describing label"
	FieldPreset       = "preset"
)

// The sentence under each field. Each says what the field does and, where it
// matters, the consequence - docs/CLAUDE.md on writing for a reader, rather
// than a description of how any of it works.
const (
	HintFormat       = "What kind of file to produce. Run out of the list and the tool has no other."
	HintSize         = "Exact size of every file. Units count in 1024s, so 10mb is 10485760 bytes. A plain number is a count of bytes."
	HintCount        = "How many files to produce."
	HintTargetID     = "Names the group. The seeds are derived from it, so changing it changes the bytes."
	HintNameTemplate = "What the files are called. {index:04} becomes 0001, 0002 and so on."
	HintOutputDir    = "Where the files and the manifest go. It is created if it is not there."
	HintSeed         = "The same seed gives the same bytes, on any machine."
	HintLabel        = "Writes into the file what it is and how big it was meant to be. Turn it off for a file that has to hold nothing but its content."
	HintPreset       = "What you are testing. The set is worked out from the answer."
)

// PlaceholderNameTemplate stands in the empty name box. It shows the answer
// rather than describing it, because a name template is easier recognised than
// explained.
const PlaceholderNameTemplate = "left empty: files_0001 and so on"

// PresetCatchesHeading introduces what a preset typically finds.
const PresetCatchesHeading = "Typically finds:"

// PresetCatchesItem is one of those, as its own line.
//
// A list rather than one sentence, and that came from looking: the items
// themselves contain commas, so joined with commas and an "and" a three item
// list read as five. Seen on screen on 2026-08-05.
func PresetCatchesItem(catch string) string {
	return "   - " + catch
}

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
func PlaceholderLeftEmpty(declared string) string {
	return "left empty: " + declared
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
