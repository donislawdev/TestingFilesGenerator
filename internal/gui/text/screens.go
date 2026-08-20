package text

import (
	"fmt"
	"strings"
)

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

// ChoiceLeftAlone is the first entry of a menu whose setting has a declared
// default, and it means "I did not state this".
//
// A menu needs a real entry for it where a text box needs none, and the reason
// is measured rather than a preference: the toolkit draws a menu's placeholder
// in the ordinary foreground colour, exactly like a value somebody picked, so
// "pdf, because nobody said otherwise" and "pdf, because I chose it" looked
// identical - measured on 2026-08-19 at RGB(230,230,230) for both, against
// RGB(157,163,168) for a text box's placeholder, which the toolkit does dim.
// Colour cannot carry the difference here, so words do.
//
// It also gives the setting a way BACK. A menu with only real values in it can
// be moved off "not stated" and never returned to it, and being able to say
// nothing is what lets the manifest record the value as defaulted rather than
// chosen - which is a promise this window makes (O104).
func ChoiceLeftAlone(declared string) string {
	return PlaceholderNotStated + " - " + declared
}

// SettingsFor heads the block of fields a chosen format declares.
//
// A function rather than a constant with the id glued on: languages do not
// agree on where a name goes in a phrase, and this is the seam that lets one
// move it without finding every caller.
func SettingsFor(formatID string) string {
	return "Settings for " + formatID
}

// SettingLabel is what the name above a declared setting reads.
//
// A format declares its settings under the key a recipe writes - width,
// bit_depth, entry_size - and until 2026-08-20 the window put that key on the
// screen as the label. Two naming systems in one visual style: the names this
// window writes are capitalised and spaced, and the ones coming out of the
// registry were not, so half the labels on the screen looked written and half
// looked leaked.
//
// It changes only what is drawn. The key is what a refusal is matched against
// and what goes into a recipe, and neither of those goes through here - see
// SettingKey for where the key stays visible.
func SettingLabel(key string) string {
	if key == "" {
		return key
	}
	spaced := strings.ReplaceAll(key, "_", " ")
	return strings.ToUpper(spaced[:1]) + spaced[1:]
}

// SettingKey says what to write in a recipe for the field being looked at.
//
// The other half of SettingLabel, and the reason the change is not a loss.
// This is a tool whose window and whose recipe file are two ways into one
// engine, so a person who finds a setting on the screen has to be able to
// write it down - and the key is the only spelling that works there.
func SettingKey(key string) string {
	return "Written as " + key + " in a recipe."
}

// TooManyFiles refuses a run before the list is built, because building it is
// the failure. The reason comes from core, so the window and the command line
// do not hold two opinions about how many files is too many.
func TooManyFiles(count int64, reason error) string {
	return fmt.Sprintf("this run asks for %d files - %s", count, reason)
}

// The recipe screen: several batches in one run.
//
// It is the third work screen and the one the parity guard was waiting for.
// Everything the engine can be asked for that the single batch screen has no
// room for lives here - a second batch, a size range, a boundary, a class, a
// declared expectation, the files inside an archive.
const (
	HeadingRecipe = "Run several batches together"

	// TabRecipe stands beside "Single batch", and the pair is deliberate: the
	// difference between the two screens is how many batches, not how advanced
	// the person is. "Advanced" would have said the other screen is for
	// beginners, which is not true of anybody generating one batch of files.
	TabRecipe = "Several batches"

	SectionBatches = "Batches"
)

// BatchHeading names one batch in the list, counted the way the refusals count.
//
// From one rather than from zero, because a refusal about the second batch says
// "target 2" and a heading saying "Batch 1" above it would send somebody to the
// wrong block. A function because a number in the middle of a phrase does not
// sit in the same place in every language.
func BatchHeading(n int) string {
	return fmt.Sprintf("Batch %d", n)
}

// ContentsHeading introduces the files an archive is told to hold.
const ContentsHeading = "Files inside each archive"

// Field labels used only on the recipe screen. The rest are shared with the
// single batch screen, because the same setting keeps the same word.
//
// FieldGroup is "Class" rather than "Group", and the reason is a collision
// already in the window: the single batch screen labels a target's id "Group
// name", because that is what an id does for the person looking at it. Two
// fields called group on one screen would be worse than a word chosen for the
// idea, and a class of case is what this actually is.
const (
	FieldSizeRange = "Size range"
	FieldBoundary  = "Around a limit"
	FieldGroup     = "Class"
	FieldExpected  = "Expected outcome"
	FieldReason    = "Why"
	FieldManifest  = "Manifest file name"
)

// The line under each of the recipe screen's own fields.
const (
	HintSizeRange = "A different size for every file."
	HintBoundary  = "Three files: one byte under the limit, one on it, one over."
	HintGroup     = "Marks several batches as one kind of case."
	HintExpected  = "What the system under test should do with these files."
	HintReason    = "Which rule this is about."
	HintManifest  = "The record of what this run produced."
)

// The longer explanation behind the button beside each of them.
const (
	DetailSizeRange = "Two sizes with a hyphen, as 1kb-8kb. Each file gets its own size, drawn from the seed, so the run repeats."
	DetailBoundary  = "Give the limit your system declares, as 10mb. Units count in 1024s, and the run prints the number it used."
	DetailGroup     = "It reaches the manifest, so a test can assert about a whole class of case at once."
	DetailExpected  = "It reaches the manifest and nothing else reads it. Leave it alone where the right answer depends on the application's own policy."
	DetailReason    = "From a closed list, so a report can group by reason. It names the rule in play whatever the outcome is - a file a byte under a size limit is expected to be accepted, and the rule in play is still the size limit. A reason needs an outcome beside it."
	DetailManifest  = "It goes in the output directory beside the files."
)

// The three ways of saying how big, offered side by side.
//
// One box each rather than a mode to choose first, and the recipe reader is what
// makes that safe: stating two of them is a refusal it already words and
// addresses, so filling in two marks the box rather than being quietly resolved.
// A mode would have been a fourth rule for the window to keep in step.
const HintSizeExact = "One size for every file."

// OneSizeSettingOnly goes under EVERY one of the three ways of stating a size,
// not just the first.
//
// The three sit side by side as equals and only one of them may be filled in,
// which the engine refuses if you try. That rule was written under the first
// box alone, so somebody reading the third had nothing to learn it from - the
// two beside it simply described themselves and said nothing about excluding
// each other (O114).
//
// A sentence over the three rather than a control that shows one box at a time.
// The control would be the better answer and it is a change to the shape of
// this screen, which has a recorded decision and a rejected variant behind it -
// so it is not something to do on the way past. Written down in the
// observation rather than left as a preference.
//
// Said once rather than three times, from 2026-08-20. It was the tail of each
// of the three hints, so the row carried "Fill in one of these three." three
// times in one line of the screen - which is the rule turned into noise, and
// two lines of height on a form that does not fit. What stays under each box is
// what THAT box does, which is the only part of the three that differs.
const OneSizeSettingOnly = "Fill in one of these three."

// Buttons on the recipe screen.
const (
	ButtonAddBatch       = "Add a batch"
	ButtonRemoveBatch    = "Remove"
	ButtonAddContents    = "Add files inside"
	ButtonRemoveContents = "Remove"
)

// PlaceholderNotStated stands in a list nobody has chosen from.
//
// A list has no empty position to select, so the placeholder is what says the
// setting was left alone - and leaving it alone has to stay possible, because a
// window whose every field arrives carrying a value can never say "I did not
// state this". Untouchable rule 5.
const PlaceholderNotStated = "not stated"

// RefusedBeforeWriting is what the foot of the form says when a press was
// turned down and every reason for it went onto a box.
//
// Without it the press had no visible answer at all: the boxes were already
// marked, the marks may be well off the bottom of a form that does not fit its
// window, and nothing where the button is changed. It carries no count, because
// the boxes themselves say which ones and how many.
const RefusedBeforeWriting = "Nothing was written. Check the settings marked above."

// ButtonDonate asks for money towards the work, and it is a word somebody reads
// so it is translated like every other.
//
// "Donate" rather than "Support" or "Sponsor": it says what pressing it leads
// to. Support reads like a help desk, which is the one thing this button is
// not, and a person looking for help would press it and be asked for money.
const ButtonDonate = "Donate"

// DetailDonate is what the button leads to, for somebody who wants to know
// before pressing rather than after.
const DetailDonate = "Opens the support page in your browser. The tool is free and stays free - this pays for the time that goes into it."

// SupportURL is where the button goes.
//
// Here rather than in the window because everything the window says lives here,
// and a bare address in the middle of a screen file is the same defect as a bare
// sentence: something a person can end up reading, written where nobody looks
// for it.
//
// Deliberately NOT translated, and that is the difference between this and
// everything above it. An address is the same in every language, and a
// translated one would be a broken one.
const SupportURL = "https://donislawdev.com/support/"
