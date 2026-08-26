// This file holds what one page is rendered against, and the pass that
// fills language text with the facts.
//
// It is separate from the types because of one rule it enforces: a number
// comes from the program and the noun beside it from the language file. Every
// lookup here fails loudly rather than rendering a blank, so a unit or an exit
// code nobody has words for is a red guard instead of an English noun on a
// Polish page.

package site

import (
	"bytes"
	"fmt"
	"html/template"
	"strconv"
	"strings"

	texttemplate "text/template"
)

// expand runs every piece of language text through the facts.
//
// It exists because of one measured defect. A page title read "PDF, DOCX, PNG,
// ZIP and 16 more formats", and the sixteen was typed - it is twenty minus the
// four that are named. A twenty first format would have left that title untrue
// with nothing to catch it, because the guard compares tables and this was
// prose. Titles, descriptions, answers and every other string may now say
// {{ .Facts.FormatCount }} and mean it.
//
// Text rather than HTML templating on purpose: these values are inserted into
// pages as text and escaped there, so escaping them here as well would show a
// reader the escape rather than the character.
func (l Language) expand(f Facts) (Language, error) {
	var failure error
	through := func(s string) string {
		if failure != nil || !strings.Contains(s, "{{") {
			return s
		}
		t, err := texttemplate.New("value").Parse(s)
		if err != nil {
			failure = fmt.Errorf("reading a %s value: %w", l.Code, err)
			return s
		}
		var b bytes.Buffer
		if err := t.Execute(&b, struct{ Facts Facts }{f}); err != nil {
			failure = fmt.Errorf("filling in a %s value: %w", l.Code, err)
			return s
		}
		return b.String()
	}
	everyValue := func(in map[string]string) map[string]string {
		if in == nil {
			return nil
		}
		out := make(map[string]string, len(in))
		for k, v := range in {
			out[k] = through(v)
		}
		return out
	}

	out := l
	out.Words = everyValue(l.Words)
	out.Endings = everyValue(l.Endings)
	out.Terms = everyValue(l.Terms)
	out.Presets = everyValue(l.Presets)

	out.Pages = make([]Page, len(l.Pages))
	for i, p := range l.Pages {
		p.Title = through(p.Title)
		p.Description = through(p.Description)
		p.Nav = through(p.Nav)
		out.Pages[i] = p
	}
	out.Faq = make([]QA, len(l.Faq))
	for i, qa := range l.Faq {
		qa.Q = through(qa.Q)
		qa.A = through(qa.A)
		qa.Code = through(qa.Code)
		out.Faq[i] = qa
	}
	return out, failure
}

// view is what a template is executed against.
type view struct {
	Facts      Facts
	Page       Page
	Lang       Language
	W          map[string]string
	Nav        []NavItem
	Switches   []Switch
	Alternates []Alternate
	Canonical  string
	Path       string
	Body       template.HTML
	IsHome     bool
}

// Word looks up a piece of interface text.
//
// It fails loudly rather than rendering an empty string. A missing word on a
// page is the kind of defect that reads as a design choice, and the site has
// two languages to keep level with each other.
func (v view) Word(key string) (string, error) {
	if s, ok := v.W[key]; ok {
		return s, nil
	}
	return "", fmt.Errorf("the %s pages ask for the word %q and it is not in their word list", v.Lang.Code, key)
}

// Term looks up the noun that goes beside a number.
func (v view) Term(key string) (string, error) {
	if s, ok := v.Lang.Terms[key]; ok {
		return s, nil
	}
	return "", fmt.Errorf("the registry uses %q and the %s pages have no word for it", key, v.Lang.Code)
}

// Endings is the frozen exit code table, in the order the program declares,
// with each meaning taken from the language being rendered.
func (v view) Endings() ([]Ending, error) {
	out := make([]Ending, 0, len(v.Facts.ExitCodes))
	for _, code := range v.Facts.ExitCodes {
		meaning, ok := v.Lang.Endings[strconv.Itoa(code)]
		if !ok {
			return nil, fmt.Errorf("exit code %d has no meaning written in %s", code, v.Lang.Code)
		}
		out = append(out, Ending{Code: code, Meaning: meaning})
	}
	return out, nil
}

// PresetList is every preset this build registers, described in the language
// being rendered.
func (v view) PresetList() ([]Preset, error) {
	out := make([]Preset, 0, len(v.Facts.Presets))
	for _, id := range v.Facts.Presets {
		question, ok := v.Lang.Presets[id]
		if !ok {
			return nil, fmt.Errorf("the preset %q has no question written in %s", id, v.Lang.Code)
		}
		out = append(out, Preset{ID: id, Question: question})
	}
	return out, nil
}

// AllowedOf says what one setting accepts, in the language being rendered.
//
// The numbers come from the registry and the words from the language file. A
// kind or a unit the language has no word for is an error rather than a gap,
// which is what stops an English noun appearing on a Polish page.
func (v view) AllowedOf(p Property) (string, error) {
	switch p.Kind {
	case "choice":
		return strings.Join(p.Choices, ", "), nil
	case "int":
		if p.Min == 0 && p.Max == 0 {
			return v.Term("int")
		}
		span := fmt.Sprintf("%d - %d", p.Min, p.Max)
		if p.Unit == "" {
			return span, nil
		}
		unit, err := v.Term(p.Unit)
		if err != nil {
			return "", err
		}
		return span + " " + unit, nil
	default:
		return v.Term(p.Kind)
	}
}
