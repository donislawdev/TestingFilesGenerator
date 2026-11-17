package text

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"syscall"

	"github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// The engine's sentences - refusals and notes - in the language the window
// speaks. docs/OKNO-PO-POLSKU-2026-10-05.md section 3.
//
// The engine keeps each sentence as core.Said: an id, the English layout the
// command line prints, and named values. A window looks the id up in the
// folder below and fills the fields with the values, each written the way the
// English layout writes it - quoted where it quotes, a number where it prints
// a number - so a translation cannot change what a value looks like, only the
// words around it.
//
// No entry, or a window in English, is the command line's English in the
// screen's words (englishWindow): a value of a list by its name, a setting by
// its label, and the recipe's words in the layout by what the screen calls the
// same things (recipewords.go). The values are the command line's to the byte,
// which TestTheEnglishWindowRewordsOnlyNamesAndLabels holds across every kind
// of value.

// SaidFolder is where the engine's sentences are kept, inside the folder of
// the window's catalogue. Its en.json is written for a translator by the guard
// that reads the sentences out of the code, and is never loaded.
const SaidFolder = "said"

// Refusal is an error in the window's language, naming the setting it is about
// by the label above that setting's box. With no box - a refusal at the foot of
// the form - a setting of a format is named by its own label, and two refused
// together by both (settingWithoutABox). Anything else is named by its recipe
// key, which is what a refusal at the foot of the form has always done.
func Refusal(err error, label string) string {
	if err == nil {
		return ""
	}
	if label == "" {
		label = settingWithoutABox(err)
	}
	var said interface{ Said() core.Said }
	if errors.As(err, &said) {
		if out, ok := translated(said.Said(), label); ok {
			return out
		}
		return inEnglish(err, label)
	}
	if core.IsDefect(err) {
		return Defect(err.Error())
	}
	return systemIn(err)
}

// inEnglish is a refusal in English: the command line's sentence with the
// setting named by the label above its box, and every value of a list and
// every setting it names in the words the window shows them in - see
// core.Wording. A refusal that cannot be reworded is its own text.
//
// The label is asked of whatever the error wraps, the way the window always
// placed a refusal under its box. Without a label, what it wraps is reworded
// only where its text is the whole of the error's - the engine addresses a
// format's refusal to its batch without adding a word, and anything that does
// add one keeps it.
func inEnglish(err error, label string) string {
	var reworded interface {
		Reworded(string, core.Wording) string
	}
	if errors.As(err, &reworded) {
		if label != "" {
			return reworded.Reworded(label, englishWindow{})
		}
		if inner, isErr := reworded.(error); isErr && inner.Error() == err.Error() {
			return reworded.Reworded("", englishWindow{})
		}
	}
	var said interface{ Said() core.Said }
	if errors.As(err, &said) && said.Said().String() == err.Error() {
		return said.Said().Reworded("", englishWindow{})
	}
	return err.Error()
}

// settingWithoutABox is the setting a refusal of a format's setting is about,
// in the window's words, for a refusal no box on the screen stands for - two
// settings refused together are keyed "level_mix and entry_format", which no
// box is. Without it the slot takes the recipe key in English, and nothing at
// all in a translation, whose sentence has no key to fall back on. Empty for
// any other refusal, which keeps the key it always had.
func settingWithoutABox(err error) string {
	var named interface{ SettingSaid() core.Said }
	if !errors.As(err, &named) {
		return ""
	}
	return Sentence(named.SettingSaid())
}

// englishWindow is how the English window says the engine's sentences where it
// does not say them as the command line does - see core.Wording.
type englishWindow struct{}

// Layout is a sentence's layout with the recipe's words in the screen's - see
// recipewords.go.
func (englishWindow) Layout(layout string) string { return InTheScreensWords(layout) }

// Word is a value of a list by the name its menu shows, and a setting by the
// label above its box.
func (englishWindow) Word(v any) (string, bool) {
	switch v := v.(type) {
	case core.Choice:
		return ChoiceName(v.Of, v.Value), true
	case core.Term:
		if name, isLabel := v.Setting(); isLabel {
			return SettingLabel(name), true
		}
	}
	return "", false
}

// Sentence is one of the engine's sentences that is not a refusal - a note a
// run left - in the window's language.
func Sentence(s core.Said) string {
	if out, ok := translated(s, ""); ok {
		return out
	}
	return s.Reworded("", englishWindow{})
}

// Line is a note a run left about itself, as a line of its own on the screen.
//
// A line starts a sentence, so one that opens with a word of the program's
// opens with a capital here - the command line prints the same note after
// "note:", where it does not. One that opens with a name, the file a run wrote
// past its memory setting, keeps the name as it is spelt, because a file called
// files_0001.txt is not Files_0001.txt. Which of the two it is, is read from the
// English layout, and every translation fills the same names. Found by
// CodeRabbit on #246: the note about a run that took a number stood in the
// window starting with a small letter.
func Line(s core.Said) string {
	if !strings.HasPrefix(s.Layout(), "%") {
		s = s.Capitalized()
	}
	return Sentence(s)
}

// translated is the sentence in the window's language, and false when this
// window says it in English: the language is English, nothing is loaded, or
// the language has no entry for it.
func translated(s core.Said, label string) (string, bool) {
	if pseudo {
		return fill(pseudoOf(core.NamedLayout(s.Layout(), s.Names())), valuesOf(s, label)), true
	}
	if localiser == nil || Speaking() == English || s.ID() == "" {
		return "", false
	}
	asked := &i18n.LocalizeConfig{MessageID: s.ID(), TemplateData: valuesOf(s, label)}
	if s.Plural() {
		asked.PluralCount = countOf(s)
	}
	out, err := localiser.Localize(asked)
	if err != nil || out == "" {
		return "", false
	}
	return s.Shaped(out), true
}

// valuesOf is every value of a sentence as the text a translation puts in its
// field, with the setting under its reserved name.
func valuesOf(s core.Said, label string) map[string]any {
	values := map[string]any{core.SettingField: label}
	directives := s.Directives()
	for i, a := range s.Args() {
		directive := "%v"
		if i < len(directives) {
			directive = directives[i]
		}
		values[a.Name] = valueIn(a.Value, directive, label)
	}
	return values
}

// valueIn is one value as a field shows it.
//
// A value somebody typed goes through core.ShownText, because it is put on a
// screen and may carry a character that turns the text around it - the foot
// of the form did that before, and the line under a box did not.
func valueIn(v any, directive, label string) string {
	if out, several := severalIn(v, label); several {
		return out
	}
	switch v := v.(type) {
	case core.Said:
		if out, ok := translated(v, label); ok {
			return out
		}
		return core.InTheWordsOf(v.String(), label)
	case core.Bytes:
		return HumanBytes(int64(v))
	case core.Choice:
		return ChoiceName(v.Of, v.Value)
	case core.Term:
		if name, isLabel := v.Setting(); isLabel {
			return SettingLabel(name)
		}
		return lookup(v.Key, v.Text)
	case error:
		return Refusal(v, label)
	case interface{ Said() core.Said }:
		return valueIn(v.Said(), directive, label)
	}
	return core.ShownText(fmt.Sprintf(strings.Replace(directive, "w", "v", 1), v))
}

// severalIn is a value holding several things, joined the way this language
// joins them, and false for a value holding one.
func severalIn(v any, label string) (string, bool) {
	switch v := v.(type) {
	case core.Joined:
		return Joined(v.Items, v.And), true
	case core.Sentences:
		return Formats(eachIn(v, label)), true
	case core.Conjoined:
		return Joined(eachIn(v, label), true), true
	case core.Lines:
		return strings.Join(eachIn(v, label), "\n  "), true
	case core.Choices:
		names := make([]string, len(v.Values))
		for i, value := range v.Values {
			names[i] = ChoiceName(v.Of, value)
		}
		return Formats(names), true
	}
	return "", false
}

// eachIn is every sentence of a list as a field shows it.
func eachIn(list []core.Said, label string) []string {
	out := make([]string, len(list))
	for i, s := range list {
		out[i] = valueIn(s, "%s", label)
	}
	return out
}

// countOf is the number that chooses the form of a sentence.
func countOf(s core.Said) int64 {
	for _, a := range s.Args() {
		if n, whole := wholeCount(a.Value); whole && a.Name == core.CountArg {
			return n
		}
	}
	return 0
}

// wholeCount is a count given as either width of whole number.
func wholeCount(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int64:
		return n, true
	}
	return 0, false
}

// systemIn is an error from the system or from a library, with the system's
// own words about the cause replaced by ours - which is what the command line
// has done since 2026-09-25 (internal/cli/errors.go, systemReason) and the
// window did not (O251). The system answers in its own language, which may be
// neither the window's nor English.
func systemIn(err error) string {
	full := err.Error()
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return core.ShownText(full)
	}
	var path *fs.PathError
	if errors.As(err, &path) {
		return core.ShownText(SystemFailure(core.Shown(path.Path), systemReason(errno)))
	}
	return core.ShownText(strings.ReplaceAll(full, errno.Error(), systemReason(errno)))
}

// systemReason is our sentence for what the system refused, by its kind - the
// kinds the command line tells apart, from the same place.
func systemReason(errno syscall.Errno) string {
	switch core.SystemKindOf(errno) {
	case core.SystemNothingThere:
		return SystemNothingThere(uint64(errno))
	case core.SystemNoPermission:
		return SystemNoPermission(uint64(errno))
	case core.SystemAlreadyThere:
		return SystemAlreadyThere(uint64(errno))
	default:
		return SystemRefused(uint64(errno))
	}
}
