package text

import (
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/tool"
)

// The engine's sentences are written for the command line, where a person has
// written a recipe or typed a command. Some of their words are the recipe's: a
// target, a line of the file, a key and its value, a command to run. The window
// has none of those - it has batches, boxes, lists and buttons - so the English
// window says those words as the screen does. docs/SIZE-FIXES-2026-11-16.md
// section 6.1, M3b.
//
// A table of phrases rather than a second English of every sentence: the
// window's word for a target is one rule, and a sentence the engine starts to
// say tomorrow is said in it without anybody writing it twice. A phrase only
// one sentence has is the whole of what that sentence says differently.
//
// A phrase is replaced in the layout, before the values go in, so a value
// somebody typed is never reworded - a file called target.txt stays target.txt.
// Every phrase keeps the values and the slots of the layout where they are,
// which TestTheWindowSaysNoWordOfTheRecipe holds for every sentence the code
// says.
//
// A translation says these things in its own words, and is held to the same
// rule by the same guard.

// phrase is one of the recipe's words and what the English window says
// instead. The screen's words are asked for when a sentence is said, because
// they name boxes and lists by the words the screen shows them under, and a
// label written here a second time would stay behind when the label changed.
type phrase struct {
	recipe string
	screen func() string
}

// as is words the screen says as they are.
func as(words string) func() string { return func() string { return words } }

// toolNamed is a tool as the list of tools names it - by the question it
// answers - in quotes, the way a sentence names a thing on the screen.
func toolNamed(id string) string {
	for _, d := range tool.All() {
		if d.ID == id {
			return `"` + ToolQuestion(d.ID, d.Question) + `"`
		}
	}
	return id
}

// setTo is a setting and a value of its list, as a sentence asks for them.
func setTo(key, value string) string { return SettingLabel(key) + " to " + ChoiceName(key, value) }

// inTheScreensWords is every phrase, in the order they are tried: a phrase
// comes before any shorter one it holds, so a sentence is said by its own
// phrase before the words of every sentence are.
var inTheScreensWords = []phrase{
	// A target is a batch on every screen. The id of one is its Batch name,
	// and a target without one is counted the way the screen heads it.
	{"target {setting} %q is used twice", as("{setting} %q is used twice")},
	{"target id", FieldTargetID},
	{"ids identify targets", func() string { return FieldTargetID() + "s identify batches" }},
	{"a target has no id", func() string { return "a batch has no " + FieldTargetID() }},
	{"every target needs a stable id", func() string { return "every batch needs a stable " + FieldTargetID() }},
	{", or leave the preset's target out by ejecting the preset and editing the recipe", as("")},
	{"the preset's target", as("the preset's batch")},
	{"target %d", as("Batch %d")},
	{"{a} {setting} is how many files this target produces, so it is read exactly as written", as("the number of files is read exactly as written")},
	{"use {a} {setting} of %d or less, or split the target across several recipes", as("ask for %d or fewer, or split the files across several runs")},
	{"or delete the target", as("or remove the batch")},
	{"a dry run", ButtonPreview},

	// What to type, said as what goes in a box rather than as a line of the
	// file.
	{"for example id: invoices", as("such as invoices")},
	{`add format: txt, or run "tfg formats" to see the whole list`, func() string { return "choose one under " + FieldFormat() }},
	{"add format: pdf", func() string { return "choose one under " + FieldFormat() }},
	{"add size: 2mb, size-range: 1kb-8kb, a boundary, contains, or a plain number of bytes", func() string {
		return "type a size such as 2mb, or choose another way under " + FieldSizeWay()
	}},
	{"add size: 2mb", as("type a size such as 2mb")},
	{"such as count: 10", as("such as 10")},
	{"such as seed: 20260802", as("such as 20260802")},
	{"use boundary: 10mb", as("use a limit such as 10mb")},
	{"use size-range: 1kb-8kb", as("use a range such as 1kb-8kb")},
	{"use count: 50", as("use a whole number such as 50")},
	{"use count: 0 for a group that contributes nothing, or drop the entry", as("ask for 0 or more, or remove the entry")},
	{"add outcome: accept, reject, sanitize or unspecified", func() string { return "choose one under " + FieldExpected() }},
	{"ask for compression: none", func() string { return "set " + setTo("compression", "none") }},
	{"or for compression: none", func() string { return "or set " + setTo("compression", "none") }},
	{"timestamps=fixed puts them", func() string {
		return SettingLabel("timestamps") + " set to " + ChoiceName("timestamps", "fixed") + " puts them"
	}},
	{"so this needs bom=true or encoding=%s", func() string {
		return "so this needs " + SettingLabel("bom") + " " + SwitchState(true) + ", or " + SettingLabel("encoding") + " set to %s"
	}},
	{"leave the line out", as("leave the box empty")},
	{"remove the line", as("empty the box")},

	// A command to run, said as the place on the screen that does the same.
	{"use tfg tool checksum for the one file", func() string { return "use the tool " + toolNamed("checksum") + " for the one file" }},
	{"with tfg tool checksum-check", func() string { return "with the tool " + toolNamed("checksum-check") }},
	{`run "tfg preset show %s" for what it takes`, func() string { return "choose %s on " + TabPresets() + " to see what it takes" }},
	{"Use the version of tfg that wrote it", as("Use the version of this program that wrote it")},
	{"A recipe is a document somebody writes, and reading time grows with its size, so an unbounded one is a way to hang a build. " +
		"Split it into several recipes, or generate the targets with a loop in your own script", as(
		"Reading time grows with the size of what is asked for, so the size is limited. Shorten what the boxes hold, or split the batches across several runs")},

	// Last, the word every sentence about a target has.
	{"targets", as("batches")},
	{"target", as("batch")},
}

// InTheScreensWords is a layout of the engine's with the recipe's words said as
// the English window says them. Only a whole word or phrase is replaced, never
// part of a longer word.
func InTheScreensWords(layout string) string {
	for _, p := range inTheScreensWords {
		if wholeIndex(layout, p.recipe) >= 0 {
			layout = replaceWhole(layout, p.recipe, p.screen())
		}
	}
	return layout
}

// replaceWhole replaces every old in s that stands as a whole: no letter or
// digit right before it or right after it.
func replaceWhole(s, old, with string) string {
	var b strings.Builder
	for i := wholeIndex(s, old); i >= 0; i = wholeIndex(s, old) {
		b.WriteString(s[:i])
		b.WriteString(with)
		s = s[i+len(old):]
	}
	b.WriteString(s)
	return b.String()
}

// wholeIndex is where old first stands as a whole in s, and -1 where it does
// not.
func wholeIndex(s, old string) int {
	from := 0
	for {
		i := strings.Index(s[from:], old)
		if i < 0 {
			return -1
		}
		i += from
		end := i + len(old)
		if (i == 0 || !wordByte(s[i-1])) && (end == len(s) || !wordByte(s[end])) {
			return i
		}
		from = i + 1
	}
}

// wordByte is a byte that belongs to a word: a letter or a digit of ASCII. A
// layout's own words are ASCII (D9), so a letter outside it never stands
// against a phrase.
func wordByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// RecipePhrases is every phrase of the recipe the English window rewords, for
// the guard that holds each to rewording some sentence.
func RecipePhrases() []string {
	out := make([]string, len(inTheScreensWords))
	for i, p := range inTheScreensWords {
		out[i] = p.recipe
	}
	return out
}
