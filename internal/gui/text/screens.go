package text

import (
	"strings"
)

// The sentence under each work screen's title, saying what the screen DOES
// for the person reading it - files of what shape, and what they are for.
//
// The titles said this until 2026-09-15 ("Generate files", "Build a set for a
// question"), when the word on the tab became the title so that a screen has
// one name rather than two. The old titles moved under the new ones first,
// word for word, and the owner allowed them to be rewritten the same day:
// "Generate files" under "Single batch" explained nothing the tab had not.
//
// SubtitleGenerate names the manifest as of 2026-09-23, and that is the one
// sentence a first start has in which to say what this tool is. It read
// "Files of one format and one size, as many as you need", which describes
// the mechanism and is true of every other generator of test files - so the
// screen the window opens on looked exactly like the tools this one is not.
// The thing it does that they do not is the manifest, and the manifest was
// named on no work screen at all: not in the form, not after a run. Named
// here it costs a clause on a line that was already there.
func SubtitleGenerate() string {
	return say("SubtitleGenerate", "Files of one format and one size, with a manifest that says how the system under test should react to them.")
}

func SubtitlePreset() string {
	return say("SubtitlePreset", "Ready-made sets of files, each built to answer one question about the system under test.")
}

// HeadingAbout carries the version, so the screen somebody is told to look at
// when reporting a problem says which build they are on.
func HeadingAbout(version string) string {
	return "Testing Files Generator " + version
}

// AboutTagline is the one sentence saying what this tool is. It is the product
// thesis in a line: files, and what should happen to them.
func AboutTagline() string {
	return say("AboutTagline", "Generate test files, and know how the system under test should react to them.")
}

// SectionHowToUse heads the three steps under the tagline, and it is there
// because of what the screen was made of without it.
//
// Counted on 2026-09-22: About gave the thesis one sentence and then ran
// straight into the licence notice and the list of what the binary carries,
// so four fifths of the screen answered a question about redistribution -
// which is a real question and not the first one anybody has. Somebody who
// has just opened this program is asking what to do with it, and the answer
// was on no screen in the window.
func SectionHowToUse() string { return say("SectionHowToUse", "How to use it") }

// HowToUseSteps is that answer, in the order somebody does it: choose what to
// make, make it, and then use what came out.
//
// The third step is the one the other two are for. It is also the only place
// in the window that says the manifest may decline to have an opinion - a run
// records outcome: unspecified wherever the right answer belongs to the
// application's own policy (manifest rule MF5), and a person writing a test
// against the file would otherwise meet that for the first time in the JSON.
//
// It names the outcomes with the SPELLING THE MANIFEST USES, and both halves
// of that were wrong when this was written on 2026-09-23. It said "turn it
// away" for an outcome the document calls reject, which is a second word for
// a contract value somebody is about to read - and it left "sanitize" out
// altogether, so the list promised three answers where the closed set has
// four (internal/recipe: accept, reject, sanitize, unspecified). The first
// half came from an outside review of #125, the second was found while
// checking it. American spelling because that is the value, not a preference.
//
// A list of sentences rather than one paragraph per step: the window draws
// these as the bullets the preset screen already uses for what a set
// typically finds, so the three steps are read at a glance rather than read.
func HowToUseSteps() []string {
	return []string{
		say("HowToUseChoose", "Choose a preset, or fill in one batch on the first screen."),
		say("HowToUsePress", "Press Generate. The files and a manifest land in the output folder."),
		say("HowToUseRead", "Point your test at the manifest. For every file it says what the system under test should do with it - accept it, reject it or sanitize it - or records the outcome as unspecified, where the right answer belongs to the application's own policy."),
	}
}

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
func TabOneTarget() string { return say("TabOneTarget", "Single batch") }
func TabPresets() string   { return say("TabPresets", "Presets") }
func TabAbout() string     { return say("TabAbout", "About") }

// Buttons that do something on the screen they are on.
func ButtonChoose() string { return say("ButtonChoose", "Choose...") }

// The sections a screen is grouped into. A form of eight settings in one column
// reads as eight unrelated things, and these are the questions they answer.
// SectionFormat is gone as of 2026-08-12. It headed a section holding one
// field, which groups nothing and cost a title, a surface and two gaps on
// a screen that did not have the room. The format moved into the section
// below it, where the rest of "what should this file be" already lived.
// Not "Preset". The card was called that and so was the first field
// inside it, so the same word stood twice within 40 px in two different
// ranks of type - which reads as unfinished naming rather than as a
// grouping. It names what the card is for instead, which is the one thing
// the field label cannot say.
func SectionConfiguration() string { return say("SectionConfiguration", "File configuration") }
func SectionOutput() string        { return say("SectionOutput", "Output") }
func SectionPreset() string        { return say("SectionPreset", "The question") }
func SectionSettings() string      { return say("SectionSettings", "Settings") }
func SectionLicence() string       { return say("SectionLicence", "Licence") }
func SectionSupport() string       { return say("SectionSupport", "Support") }

// The three headings over what this binary carries that somebody else wrote.
//
// They are three rather than one because the difference matters to whoever is
// reading: a library and a font are not the same kind of thing, they arrive
// under different licences, and a person checking whether they may ship this
// is asking about both. Seven fonts and ninety-seven drawings sat unnamed
// until 2026-08-28 precisely because everything only ever counted libraries.
// The third, since 2026-09-17, is what ships NEXT to the binary rather than
// in it - the software renderer in the Windows archive - which no list of
// what was compiled in could ever name.
//
// The values under them - names, versions, licence identifiers - are not
// translated. They are the same in every language, and the program reads them
// out of its own build rather than out of this catalogue.
func SectionCarriedCode() string { return say("SectionCarriedCode", "Third party code compiled in") }
func SectionCarriedFiles() string {
	return say("SectionCarriedFiles", "Files compiled in that are not code")
}
func SectionCarriedBeside() string {
	return say("SectionCarriedBeside", "Shipped beside the program on Windows")
}

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
//
// Two more were read again on 2026-08-25, once the line under each field had
// moved behind its button and the label became the only word left standing.
//
// "How many" is now "How many files". How many of what was answerable from the
// two fields beside it and from nothing on the field itself, and the line that
// used to answer it is behind a button somebody has to press.
//
// "Group name" is now "Batch name", which is the word already on the screen:
// the tab says Single batch and each block on the other screen is headed
// Batch 1. Group named nothing a person could point at, and it was chosen in
// the first place to avoid colliding with the setting below - see FieldGroup,
// where that collision is now gone.
func FieldFormat() string       { return say("FieldFormat", "Format") }
func FieldSize() string         { return say("FieldSize", "Size") }
func FieldCount() string        { return say("FieldCount", "How many files") }
func FieldTargetID() string     { return say("FieldTargetID", "Batch name") }
func FieldNameTemplate() string { return say("FieldNameTemplate", "File names") }
func FieldOutputDir() string    { return say("FieldOutputDir", "Output directory") }
func FieldSeed() string         { return say("FieldSeed", "Seed") }
func FieldLabel() string        { return say("FieldLabel", "Label in each file") }
func FieldPreset() string       { return say("FieldPreset", "Preset") }

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
func HintFormat() string { return say("HintFormat", "What kind of file to produce.") }
func HintSize() string   { return say("HintSize", "Exact size of every file.") }
func HintTargetID() string {
	return say("HintTargetID", "A short name for this batch.")
}
func HintNameTemplate() string {
	return say("HintNameTemplate", "What every file in this batch is called.")
}
func HintOutputDir() string { return say("HintOutputDir", "Where the files and the manifest go.") }
func HintSeed() string      { return say("HintSeed", "The same seed gives the same bytes.") }
func HintPreset() string    { return say("HintPreset", "What you are testing.") }

// The longer explanation behind the button beside a field name.
//
// What somebody needs once rather than every time: the units, the example, the
// consequence. Kept because each of these says something that cannot be worked
// out from the label - that changing the batch name changes the bytes, that
// 10mb is not ten million, that a seed carries across machines.
//
// DetailLabel holds the whole explanation because the switch says what it does
// on its own face, so there is no short line left to put under it.
//
// The format menu has none, since 2026-08-27 and by the owner's decision. It
// said "Run out of the list and the tool has no other", which is a sentence
// about a list somebody is looking at - and every other one here says something
// the field cannot show. The button is still there, carrying the line that says
// what the setting does.
func DetailSize() string {
	return say("DetailSize", "Units count in 1024s, so 10mb is 10485760 bytes. A plain number is a count of bytes.")
}
func DetailTargetID() string {
	return say("DetailTargetID", "It reaches the manifest and the file names, so a test can tell these files from the rest of the run. The seeds are derived from it, so changing it changes the bytes.")
}
func DetailNameTemplate() string {
	return say("DetailNameTemplate", "Left empty, the files are named after the batch and numbered, like invoices_0001.pdf. Fill it in and the name is used exactly as written, extension and all. The one placeholder is {index:04}, which becomes 0001, 0002 and so on, and a batch of more than one file needs it.")
}
func DetailOutputDir() string { return say("DetailOutputDir", "It is created if it is not there.") }
func DetailSeed() string {
	return say("DetailSeed", "On any machine and in any build of this tool. 0 is the seed a run uses when nobody asks for another one. It is a seed like any other rather than a request for random files, which this tool never produces.")
}
func DetailLabel() string {
	return say("DetailLabel", "Writes into the file what it is and how big it was meant to be. Turn it off for a file that has to hold nothing but its content.")
}
func DetailPreset() string { return say("DetailPreset", "The set is worked out from the answer.") }

// PlaceholderNameTemplate stands in the empty name box. It shows the answer
// rather than describing it, because a name template is easier recognised than
// explained.
//
// The words "left empty" came off it on 2026-08-12. A placeholder is what the
// box will hold, and a placeholder explaining its own absence is documentation
// in the one place that disappears the moment somebody types.
const PlaceholderNameTemplate = "files_0001"

// PresetCatchesHeading introduces what a preset typically finds.
func PresetCatchesHeading() string { return say("PresetCatchesHeading", "Typically finds:") }

// PresetCatchesItem is gone as of 2026-08-12. It put the list marker into the
// string - "   - " in front of the words - so the marker sat on the text
// baseline and wrapped along with it, and a wrapped item carried on underneath
// its own marker. The marker is a column of its own now, drawn by parts.Bullets,
// which leaves nothing here to word.

// PlaceholderWorkedOut stands in a format setting the format decides for
// itself when nobody states it. A declaration with no default means the answer
// comes from the size that was asked for.
func PlaceholderWorkedOut() string { return say("PlaceholderWorkedOut", "worked out from the size") }

// PlaceholderFilter stands in the box at the top of an open list of formats,
// where typing narrows the list to the formats whose identifier holds what was
// typed, or whose name or kind has a word starting with it.
func PlaceholderFilter() string { return say("PlaceholderFilter", "type to filter") }

// ListNothingMatches stands in an open list whose filter left no value. It is
// a row nobody can choose, so the list does not look broken while it is
// empty, and it says what to do as well as what happened: "Nothing matches"
// said only the second, so the way back to the whole list was left to be
// guessed (review of #127, decided by the owner on 2026-09-23). "The box" is
// the filter box the person is typing in, the one control the list has.
func ListNothingMatches() string {
	return say("ListNothingMatches", "No format matches - clear the box to see all")
}

// The headings inside an open list of formats, one over each kind of file.
// The list puts the kinds in the order of these words, so a translation
// reorders the headings with it rather than leaving them in English order.
func ListKindArchives() string  { return say("ListKindArchives", "Archives") }
func ListKindDocuments() string { return say("ListKindDocuments", "Documents") }
func ListKindPictures() string  { return say("ListKindPictures", "Pictures") }
func ListKindSound() string     { return say("ListKindSound", "Sound") }
func ListKindText() string      { return say("ListKindText", "Text and data") }
func ListKindVideo() string     { return say("ListKindVideo", "Video") }

// ListHeadingCount is one of those headings with how many formats stand under
// it in the list as it is drawn - after the filter, so the number is what the
// eye can count below it. The separator is the one the line at the foot of a
// screen uses between the parts of what a run comes to.
func ListHeadingCount(kind string, count int) string {
	return sayf("ListHeadingCount", "{{.Kind}} · {{.Count}}", map[string]any{"Kind": kind, "Count": count})
}

// UseSmallestSize is the button under a refusal about a size below what the
// format can make. It puts the smallest size that works into the box, so the
// count of bytes in the refusal does not have to be copied by hand.
func UseSmallestSize(size string) string {
	return sayf("UseSmallestSize", "Use the smallest size, {{.Size}}", map[string]any{"Size": size})
}

// PlaceholderLeftEmpty stands in a format setting that has a declared default,
// showing what happens if the field is not touched.
//
// The field is deliberately NOT filled in with that default. A window whose
// every field arrives carrying a value cannot say "I did not state this", and
// untouchable rule 5 needs that difference - it is what tells a limit somebody
// chose from a placeholder we invented.
//
// The value alone from 2026-08-12, where it used to read "left empty: 10mb",
// on the reason that the sentence under the field said "default 10mb" in
// words. That sentence moved behind the field's explanation button on
// 2026-08-25, so the box became the only place on the screen saying it - and
// the owner's review of the prototype of 2026-09-23 found a grey "60" or "1"
// read as a value somebody had already typed. So the word is back, once.
func PlaceholderLeftEmpty(declared string) string {
	return sayf("PlaceholderLeftEmpty", "default: {{.Value}}", map[string]any{"Value": declared})
}

// ChoiceLeftAlone is gone, and the reason it existed is worth keeping because
// the colour measurement in it is still true.
//
// It made the first entry of a menu read "not stated - pdf", so that a default
// nobody picked could be told apart from a value somebody chose. The toolkit
// draws a menu's placeholder in the ordinary foreground colour - measured on
// 2026-08-19 at RGB(230,230,230), against RGB(157,163,168) for a text box's
// placeholder, which the toolkit does dim - so colour could not carry that
// difference and words had to.
//
// What was never checked is whether anything downstream WANTED the difference.
// Measured on 2026-08-27 at both ends: a format setting reaches the manifest
// expanded either way, and a preset's defaulted is built from declared
// parameters, which no menu on that screen is. Nothing read it, so five menus
// carried a third entry that did nothing. Removed on 2026-08-27, by the owner's
// decision, with the measurement in parts.choiceField.
//
// PlaceholderNotStated stays. It is a different thing: a menu with no declared
// default, where nothing chosen really does mean nothing stated, and the
// manifest really does record it as unspecified.
// The damage field. A file broken on purpose is the one thing this tool makes
// that is not well formed, so the words around it say what will happen rather
// than what it is called.
func FieldDamage() string { return say("FieldDamage", "Damage") }

func HintDamage() string {
	return say("HintDamage", "Break the files on purpose.")
}

func DetailDamage() string {
	return say("DetailDamage", "The files come out the size you asked for and no reader will accept them, which is what a validator has to reject. The manifest records what was broken and says the file is expected to be rejected.")
}

// DamageNone is the entry that means no damage, and it is what the menu opens
// on. A menu cannot be empty, so "not damaged" has to be one of its values.
func DamageNone() string { return say("DamageNone", "none") }

// DamageSettingsFor heads the block of fields a chosen damage declares.
//
// A function rather than a constant with the name glued on, for the reason
// SettingsFor gives below.
func DamageSettingsFor(damageID string) string {
	return sayf("DamageSettingsFor", "Settings for {{.Damage}}", map[string]any{"Damage": damageID})
}

// SettingsFor heads the block of fields a chosen format declares.
//
// A function rather than a constant with the id glued on: languages do not
// agree on where a name goes in a phrase, and this is the seam that lets one
// move it without finding every caller.
func SettingsFor(formatID string) string {
	return sayf("SettingsFor", "Settings for {{.Format}}", map[string]any{"Format": formatID})
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
	return sayf("SettingKey", "Written as {{.Key}} in a recipe.", map[string]any{"Key": key})
}

// TooManyFiles refuses a run before the list is built, because building it is
// the failure. The reason comes from core, so the window and the command line
// do not hold two opinions about how many files is too many.
func TooManyFiles(count int64, reason error) string {
	return sayf("TooManyFiles", "this run asks for {{.Count}} files - {{.Reason}}",
		map[string]any{"Count": count, "Reason": reason})
}

// The recipe screen: several batches in one run.
//
// It is the third work screen and the one the parity guard was waiting for.
// Everything the engine can be asked for that the single batch screen has no
// room for lives here - a second batch, a size range, a boundary, a class, a
// declared expectation, the files inside an archive.
// TabRecipe stands beside "Single batch", and the pair is deliberate: the
// difference between the two screens is how many batches, not how advanced
// the person is. "Advanced" would have said the other screen is for
// beginners, which is not true of anybody generating one batch of files.
func SubtitleRecipe() string {
	return say("SubtitleRecipe", "Batches of different formats and sizes, generated together in one run.")
}

func TabRecipe() string { return say("TabRecipe", "Several batches") }

// BatchHeading names one batch in the list, counted the way the refusals count.
//
// From one rather than from zero, because a refusal about the second batch says
// "target 2" and a heading saying "Batch 1" above it would send somebody to the
// wrong block. A function because a number in the middle of a phrase does not
// sit in the same place in every language.
func BatchHeading(n int) string {
	return sayf("BatchHeading", "Batch {{.Number}}", map[string]any{"Number": n})
}

// ContentsHeading introduces the files an archive is told to hold.
func ContentsHeading() string { return say("ContentsHeading", "Files inside each archive") }

// Field labels used only on the recipe screen. The rest are shared with the
// single batch screen, because the same setting keeps the same word.
//
// FieldGroup was "Class" until 2026-08-25, and "Class" was itself a way around
// a collision: a target's id was labelled "Group name", so two fields on one
// screen would have been called group. That collision went when the id became
// "Batch name", and what was left was a single word naming nothing a person
// could point at. It says what it does now.
//
// FieldReason was "Why", which reads as an invitation to write a sentence. It
// takes one value from a closed list and the list is of rules - a file a byte
// under a size limit is expected to be accepted, and the rule in play is still
// the size limit. The label says that now, so the menu under it is no longer a
// surprise.
func FieldSizeRange() string { return say("FieldSizeRange", "Size range") }
func FieldBoundary() string  { return say("FieldBoundary", "Limit to test") }

// The three ways of saying how big, as one control that allows one of them.
//
// They were three boxes side by side until 2026-08-25, with a sentence above
// them saying that only one may be filled in - which is O114, and it was a
// sentence because the screen let somebody fill in two and find out at the
// press of a button. A switch removes the state instead of describing it.
//
// The words differ from the labels below them on purpose. A switch reading
// "Size" above a box reading "Size" is the same word twice in 40 px, and these
// say HOW the size is stated where the label says WHAT the box holds.
func SizeWayExact() string    { return say("SizeWayExact", "One size") }
func SizeWayRange() string    { return say("SizeWayRange", "A range") }
func SizeWayBoundary() string { return say("SizeWayBoundary", "Around a limit") }

// FieldSizeWay names that switch, and until 2026-09-23 it had no name at all.
//
// It was the one control on the batch screen standing between two named
// fields with nothing over it, so what it was about came from where it sat -
// which is guessing, and which puts it outside the rule that every element
// belongs to something. The words say HOW the size is stated, where the
// label under it says WHAT the box holds, for the reason written above the
// three words themselves.
func FieldSizeWay() string { return say("FieldSizeWay", "How the size is given") }

// DetailSizeWay says what each of the three does, and it is behind the button
// because it is read once.
//
// All three in one sentence each, deliberately: the line under a box explains
// only the way already chosen, so before choosing there is nothing on the
// screen that compares them - and "Around a limit" is both the least obvious
// of the three and the one this tool is really for.
func DetailSizeWay() string {
	return say("DetailSizeWay", "One size gives every file the same size. A range draws a different size for each file. Around a limit makes three files: one byte under the limit, one on it, one over.")
}
func FieldGroup() string    { return say("FieldGroup", "Kind of case") }
func FieldPurpose() string  { return say("FieldPurpose", "Purpose") }
func FieldExpected() string { return say("FieldExpected", "Expected outcome") }
func FieldReason() string   { return say("FieldReason", "Rule being tested") }
func FieldManifest() string { return say("FieldManifest", "Manifest file name") }

// The line under each of the recipe screen's own fields.
func HintSizeRange() string { return say("HintSizeRange", "A different size for every file.") }
func HintBoundary() string {
	return say("HintBoundary", "Three files: one byte under the limit, one on it, one over.")
}
func HintGroup() string { return say("HintGroup", "Marks several batches as one kind of case.") }
func HintPurpose() string {
	return say("HintPurpose", "What these files are and why they are in the set.")
}
func HintExpected() string {
	return say("HintExpected", "What the system under test should do with these files.")
}
func HintReason() string   { return say("HintReason", "Which rule this is about.") }
func HintManifest() string { return say("HintManifest", "The record of what this run produced.") }

// The longer explanation behind the button beside each of them.
func DetailSizeRange() string {
	return say("DetailSizeRange", "Two sizes with a hyphen, as 1kb-8kb. Each file gets its own size, drawn from the seed, so the run repeats.")
}
func DetailBoundary() string {
	return say("DetailBoundary", "Give the limit your system declares, as 10mb. Units count in 1024s, and the run prints the number it used.")
}
func DetailPurpose() string {
	return say("DetailPurpose", "It is written into the manifest and into the instructions beside it, which say what every file of the run is for. It changes no byte of any file.")
}
func DetailGroup() string {
	return say("DetailGroup", "It reaches the manifest, so a test can assert about a whole class of case at once.")
}
func DetailExpected() string {
	return say("DetailExpected", "It reaches the manifest and nothing else reads it. Leave it alone where the right answer depends on the application's own policy.")
}
func DetailReason() string {
	return say("DetailReason", "From a closed list, so a report can group by reason. It names the rule in play whatever the outcome is - a file a byte under a size limit is expected to be accepted, and the rule in play is still the size limit. A reason needs an outcome beside it.")
}
func DetailManifest() string {
	return say("DetailManifest", "It goes in the output directory beside the files.")
}

// The three ways of saying how big, offered side by side.
//
// A mode chosen first, since 2026-08-25, which this comment used to argue
// against: one box each was called safe because the recipe reader refuses two,
// and a mode would be "a fourth rule for the window to keep in step". Both
// halves were true and neither was the point - a refusal somebody has to press
// a button to receive is a state the screen let them reach. The mode keeps in
// step by construction, because the box that is not shown is not sent.
func HintSizeExact() string { return say("HintSizeExact", "One size for every file.") }

// OneSizeSettingOnly is gone as of 2026-08-25, and this note is here rather
// than nothing because the sentence it held was a FIX and not clutter.
//
// It said "Fill in one of these three." over the three ways of stating a size.
// O114: the three sat side by side as equals, only one of them could be filled
// in, and nothing on the screen said so - somebody filled in two and found out
// at the press of a button.
//
// The three boxes became one switch and one box, so the state that sentence
// warned about cannot be reached. A rule nobody can break is a rule with
// nothing to say. The protection did not go with it: it moved from
// "the warning is on the screen, once, near all three" to
// "only the chosen way reaches the run", which is the property rather than the
// wording of it - see the guards named after it.

// BatchSummary is the one line a folded batch shows about itself.
//
// A fold with nothing to say is a column of titles somebody opens one by one to
// find the one they meant, which is a worse screen than a long one. What goes
// in it is what tells two batches apart at a glance: the name, the kind of
// file, how many and how big.
//
// Parts that are not stated are left out rather than shown empty. A batch with
// nothing filled in says nothing, which is honest - it has nothing to say yet.
func BatchSummary(name, format string, count int, size string) string {
	said := []string{name, format}
	if count > 0 {
		said = append(said, files(count))
	}
	return FoldedSummary(append(said, size)...)
}

// FoldedSummary is what any folded section says about itself, from the values
// that were actually stated.
//
// Empty ones are left out rather than shown blank, and a section where nothing
// was stated says nothing at all - which is honest, and which hides the line
// rather than leaving a gap where a sentence would be.
//
// It matters most where a section is folded from the start: a stated value
// inside one would otherwise be off the screen with nothing to say it is
// there, and a setting somebody typed and then cannot see is worse than one
// they never had.
func FoldedSummary(said ...string) string {
	kept := make([]string, 0, len(said))
	for _, one := range said {
		if one != "" {
			kept = append(kept, one)
		}
	}
	return strings.Join(kept, separator)
}

// SettingSaid is one stated format setting, as a folded section reports it.
func SettingSaid(label, value string) string { return label + " " + value }

// SectionManifestNotes heads the settings that describe the case rather than
// the files.
//
// Named for where they go, because that is the whole of what they have in
// common and it is what makes the section safe to fold away by default: not
// one of them changes a single byte of what is written. They are read back out
// of the manifest by whatever test suite the files were made for.
func SectionManifestNotes() string { return say("SectionManifestNotes", "Notes for the manifest") }

// NoteManifestOnly is the sentence inside that section.
func NoteManifestOnly() string {
	return say("NoteManifestOnly",
		"These describe the case. They go into the manifest and change nothing in the files.")
}

// SectionBase heads the part of the recipe screen that builds on a preset.
//
// A recipe may start from a preset's set and add its own batches after it -
// the recipe key is extends, and this is the same thing on the screen. It
// stands above the batches because that is the order the run takes them in:
// the preset's targets first, then the batches.
func SectionBase() string { return say("SectionBase", "Build on a preset") }

// NoteBase is the sentence inside that section. It describes the switch,
// because the switch is the state: a preset is always chosen once the menu
// is there, and "unchosen" - which the first wording said - was a state the
// screen does not have. Pointed out by an outside review of #119.
func NoteBase() string {
	return say("NoteBase",
		"With the box ticked, the chosen preset's files come first and the batches below are added after them. Without it, the batches are the whole recipe.")
}

// FieldBuildOnPreset names the switch that turns the section on. Off is the
// ordinary recipe, on is one that carries extends.
func FieldBuildOnPreset() string { return say("FieldBuildOnPreset", "Start from a preset") }
func DetailBuildOnPreset() string {
	return say("DetailBuildOnPreset",
		"Unticked, the batches below are the whole recipe. Ticked, the chosen preset's files come first and the batches are added after them.")
}

// FieldBasePreset names the box choosing the preset to build on. A different
// name from the preset screen's, because that one asks which set to make and
// this one asks what to make a set on top of.
func FieldBasePreset() string { return say("FieldBasePreset", "Preset to build on") }
func HintBasePreset() string  { return say("HintBasePreset", "Its files come first.") }
func DetailBasePreset() string {
	return say("DetailBasePreset",
		"The settings under it are the preset's own. One left empty takes its default, and the manifest records which ones did.")
}

// Buttons on the recipe screen.
func ButtonAddBatch() string       { return say("ButtonAddBatch", "Add a batch") }
func ButtonRemoveBatch() string    { return say("ButtonRemoveBatch", "Remove") }
func ButtonDuplicateBatch() string { return say("ButtonDuplicateBatch", "Duplicate") }
func ButtonAddContents() string    { return say("ButtonAddContents", "Add files inside") }
func ButtonRemoveContents() string { return say("ButtonRemoveContents", "Remove") }

// PlaceholderNotStated stands in a list nobody has chosen from.
//
// A list has no empty position to select, so the placeholder is what says the
// setting was left alone - and leaving it alone has to stay possible, because a
// window whose every field arrives carrying a value can never say "I did not
// state this". Untouchable rule 5.
func PlaceholderNotStated() string { return say("PlaceholderNotStated", "not stated") }

// RequiredMark stands beside the name of a field that has to be filled in.
//
// Here rather than in the widget that draws it, because everything a person
// reads is composed in this package - a star is a word in every language that
// uses this convention, and a language that marks a required field some other
// way needs one place to change it.
//
// A star rather than the word, because it sits on the same line as the field's
// name and a word there competes with the name. The same reasoning that made
// the longer explanation an icon.
const RequiredMark = "*"

// OneExplanation joins the line that says what a field does to the longer
// explanation of it, in that order.
//
// Here rather than where the two are put together, because joining two
// sentences is a decision about writing: what separates them, and which comes
// first. The guard for text outside this package caught the space doing that
// job in parts, and it was right to - a language that ends a sentence some
// other way needs one place to change it.
//
// What it does first, then the detail of it. Somebody opening this wants to
// know what the setting is before they read what it takes.
func OneExplanation(line, detail string) string {
	switch {
	case line == "":
		return detail
	case detail == "":
		return line
	}
	return line + " " + detail
}

// RefusedBeforeWriting is what the foot of the form says when a press was
// turned down and every reason for it went onto a box.
//
// Without it the press had no visible answer at all: the boxes were already
// marked, the marks may be well off the bottom of a form that does not fit its
// window, and nothing where the button is changed. It carries no count, because
// the boxes themselves say which ones and how many.
func RefusedBeforeWriting() string {
	return say("RefusedBeforeWriting", "Nothing was written. Check the settings marked above.")
}

// ButtonDonate asks for money towards the work, and it is a word somebody reads
// so it is translated like every other.
//
// "Donate" rather than "Support" or "Sponsor": it says what pressing it leads
// to. Support reads like a help desk, which is the one thing this button is
// not, and a person looking for help would press it and be asked for money.
func ButtonDonate() string { return say("ButtonDonate", "Donate") }

// DetailDonate is what the button leads to, for somebody who wants to know
// before pressing rather than after.
func DetailDonate() string {
	return say("DetailDonate", "Opens the support page in your browser. The tool is free and stays free - this pays for the time that goes into it.")
}

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
