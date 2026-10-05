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
// No entry, or a window in English, is the sentence exactly as the window
// showed it before any of this existed. That is not a courtesy: the stored
// pictures of every screen hold the English window to its old self, which is
// the proof this layer changed nothing there.

// SaidFolder is where the engine's sentences are kept, inside the folder of
// the window's catalogue. Its en.json is written for a translator by the guard
// that reads the sentences out of the code, and is never loaded.
const SaidFolder = "said"

// Refusal is an error in the window's language, naming the setting it is about
// by the label above that setting's box. An empty label names it by its recipe
// key, which is what a refusal at the foot of the form has always done.
func Refusal(err error, label string) string {
	if err == nil {
		return ""
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

// inEnglish is a refusal the way the window has always shown it.
func inEnglish(err error, label string) string {
	var reworded interface{ InTheWordsOf(string) string }
	if label != "" && errors.As(err, &reworded) {
		return reworded.InTheWordsOf(label)
	}
	return err.Error()
}

// Sentence is one of the engine's sentences that is not a refusal - a note a
// run left - in the window's language.
func Sentence(s core.Said) string {
	if out, ok := translated(s, ""); ok {
		return out
	}
	return s.String()
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
	if s.Stopless() {
		out = strings.TrimSuffix(out, ".")
	}
	return out, true
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
		return lookup(v.Key, v.Text)
	case core.Joined:
		return Joined(v.Items, v.And)
	case core.Sentences:
		parts := make([]string, len(v))
		for i, s := range v {
			parts[i] = valueIn(s, "%s", label)
		}
		return Formats(parts)
	case core.Choices:
		names := make([]string, len(v.Values))
		for i, value := range v.Values {
			names[i] = ChoiceName(v.Of, value)
		}
		return Formats(names)
	case error:
		return Refusal(v, label)
	case interface{ Said() core.Said }:
		return valueIn(v.Said(), directive, label)
	}
	return core.ShownText(fmt.Sprintf(strings.Replace(directive, "w", "v", 1), v))
}

// countOf is the number that chooses the form of a sentence.
func countOf(s core.Said) int64 {
	for _, a := range s.Args() {
		if a.Name == core.CountArg {
			if n, ok := a.Value.(int); ok {
				return int64(n)
			}
			if n, ok := a.Value.(int64); ok {
				return n
			}
		}
	}
	return 0
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
	}
	return SystemRefused(uint64(errno))
}
