// This file gives every preset the program registers a page of its own, in
// every language, and holds what one of those pages is rendered against.
//
// These are the first pages of the site that come from a list rather than
// from site.json. The list is the registry, so a seventh preset gets its pages
// without anybody adding them - and without anybody writing its words, the
// render stops and names the preset and the language, which is the whole point
// of making the pages here rather than by hand. The reasoning, and what was
// left out on purpose, is in docs/PRESET-PAGES-2026-09-29.md.

package site

import (
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
)

// PresetFacts is one preset as the program describes it.
//
// Everything here is a name or a number. The sentences about a preset differ
// by language and live in PresetText, for the same reason a unit does.
type PresetFacts struct {
	ID       string
	Settings []Setting
	Reads    []Read
	Budget   Budget
	Outcomes []Outcome
}

// Setting is one parameter a preset declares.
type Setting struct {
	Property
	Default string
	// Placeholder marks a default that stands in for a number only the
	// system under test knows, such as the limit of an upload form. The
	// program says so out loud when that default is used, and the page says
	// it beside the value, so nobody copies our number and believes it is
	// theirs.
	Placeholder bool
}

// Read is a flag of the tool itself that a preset gives a default to, rather
// than declaring a parameter of the same meaning.
type Read struct {
	Name    string
	Default string
}

// Budget is what the set costs at its defaults, as tfg preset show prints it.
type Budget struct {
	Targets int
	Files   int
	Bytes   int64
	Formats []string
}

// Outcome is how many files of the set a system should meet with one
// reaction, as the manifest of a dry run declares them.
type Outcome struct {
	Name  string
	Count int
}

// PresetText is every word one preset needs, in one language.
//
// Title is what the preset is called and PageTitle is the title of its page
// for a search engine, which has to say more than a name in the same few
// words. Details is keyed by the parameter name. Catches and Details in
// English are copies of the registry, held to it by
// TestEveryLanguageDescribesEverythingTheProgramCanProduce.
type PresetText struct {
	Question    string            `json:"question"`
	Title       string            `json:"title"`
	PageTitle   string            `json:"pageTitle"`
	Description string            `json:"description"`
	Catches     []string          `json:"catches"`
	Details     map[string]string `json:"details"`
}

// presetParent is the key of the page every preset page sits under.
const presetParent = "presets"

// presetKey pairs the pages of one preset across languages.
func presetKey(id string) string { return "preset/" + id }

// addressable is what an id has to look like to become part of an address.
//
// The registry refuses only an empty id and a repeated one, and an id is
// written into a path on disk and a URL here. So this is the one place that
// asks, rather than trusting a rule nothing else holds.
var addressable = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// withPresetPages adds one page per preset to a language already filled in
// with the facts.
func (l Language) withPresetPages(f Facts) (Language, error) {
	if len(f.Presets) == 0 {
		return l, nil
	}
	parent, ok := find(l, presetParent)
	if !ok {
		return l, fmt.Errorf("the %s pages have no %q page, and every preset page sits under it", l.Code, presetParent)
	}
	pages := append([]Page(nil), l.Pages...)
	for _, p := range f.Presets {
		if !addressable.MatchString(p.ID) {
			return l, fmt.Errorf("the preset %q cannot be part of an address - an id of lower case letters, digits and single dashes can", p.ID)
		}
		text, ok := l.Presets[p.ID]
		if !ok || text.Title == "" || text.PageTitle == "" || text.Description == "" {
			return l, fmt.Errorf("the preset %q has no title, page title or description written in %s", p.ID, l.Code)
		}
		pages = append(pages, Page{
			Key:         presetKey(p.ID),
			Slug:        path.Join(parent.Slug, p.ID),
			Title:       text.PageTitle,
			Description: text.Description,
			Nav:         text.Title,
			Parent:      presetParent,
			Template:    "preset",
			Item:        p.ID,
		})
	}
	l.Pages = pages
	return l, nil
}

// expanded runs every word of one preset through the facts.
func (t PresetText) expanded(through func(string) string) PresetText {
	out := t
	out.Question = through(t.Question)
	out.Title = through(t.Title)
	out.PageTitle = through(t.PageTitle)
	out.Description = through(t.Description)
	out.Catches = make([]string, len(t.Catches))
	for i, c := range t.Catches {
		out.Catches[i] = through(c)
	}
	if t.Details != nil {
		out.Details = make(map[string]string, len(t.Details))
		for k, v := range t.Details {
			out.Details[k] = through(v)
		}
	}
	return out
}

// PresetList is every preset this build registers, described in the language
// being rendered, each with the address of its own page.
func (v view) PresetList() ([]Preset, error) {
	out := make([]Preset, 0, len(v.Facts.Presets))
	for _, p := range v.Facts.Presets {
		text, ok := v.Lang.Presets[p.ID]
		if !ok || text.Question == "" {
			return nil, fmt.Errorf("the preset %q has no question written in %s", p.ID, v.Lang.Code)
		}
		page, ok := find(v.Lang, presetKey(p.ID))
		if !ok {
			return nil, fmt.Errorf("the preset %q has no %s page to link to", p.ID, v.Lang.Code)
		}
		out = append(out, Preset{ID: p.ID, Title: text.Title, Question: text.Question, URL: pageURL(v.Lang, page)})
	}
	return out, nil
}

// PresetPage is what the page of one preset shows.
type PresetPage struct {
	ID       string
	Title    string
	Question string
	Catches  []string
	Settings []SettingRow
	Budget   Budget
	// Bytes is the total of the budget grouped in threes, the way tfg preset
	// show prints it, so the two can be read side by side.
	Bytes    string
	Outcomes []OutcomeRow
}

// SettingRow is one line of the settings table.
type SettingRow struct {
	Flag        string
	Takes       string
	Default     string
	Detail      string
	Placeholder bool
}

// OutcomeRow is one line of the reactions table.
type OutcomeRow struct {
	Name    string
	Meaning string
	Count   int
}

// Placeholders are the settings whose default is ours rather than the
// reader's, which is what a command on the page has to spell out.
func (p PresetPage) Placeholders() []SettingRow {
	var out []SettingRow
	for _, s := range p.Settings {
		if s.Placeholder {
			out = append(out, s)
		}
	}
	return out
}

// Preset is the preset the page being rendered is about.
func (v view) Preset() (PresetPage, error) {
	id := v.Page.Item
	var facts *PresetFacts
	for i := range v.Facts.Presets {
		if v.Facts.Presets[i].ID == id {
			facts = &v.Facts.Presets[i]
		}
	}
	text, ok := v.Lang.Presets[id]
	if facts == nil || !ok {
		return PresetPage{}, fmt.Errorf("the %s page %q is about a preset %q that the program or the language does not know", v.Lang.Code, v.Page.Key, id)
	}
	settings, err := v.settingRows(*facts, text)
	if err != nil {
		return PresetPage{}, err
	}
	outcomes, err := v.outcomeRows(*facts)
	if err != nil {
		return PresetPage{}, err
	}
	return PresetPage{
		ID:       id,
		Title:    text.Title,
		Question: text.Question,
		Catches:  text.Catches,
		Settings: settings,
		Budget:   facts.Budget,
		Bytes:    grouped(facts.Budget.Bytes),
		Outcomes: outcomes,
	}, nil
}

// settingRows describes every setting of a preset, its own parameters first
// and then the flags of the tool it gives a default to.
//
// A flag of the tool has no sentence in the registry of presets, because it is
// the tool's rather than the preset's. So its words come from the word list,
// keyed by the flag, and a preset that starts reading a second flag stops the
// render until somebody writes them.
func (v view) settingRows(f PresetFacts, text PresetText) ([]SettingRow, error) {
	out := make([]SettingRow, 0, len(f.Settings)+len(f.Reads))
	for _, s := range f.Settings {
		takes, err := v.AllowedOf(s.Property)
		if err != nil {
			return nil, err
		}
		detail, ok := text.Details[s.Name]
		if !ok {
			return nil, fmt.Errorf("the setting --%s of the preset %q has no sentence written in %s", s.Name, f.ID, v.Lang.Code)
		}
		out = append(out, SettingRow{Flag: s.Name, Takes: takes, Default: s.Default, Detail: detail, Placeholder: s.Placeholder})
	}
	for _, r := range f.Reads {
		takes, err := v.Word("readTakes." + r.Name)
		if err != nil {
			return nil, err
		}
		detail, err := v.Word("read." + r.Name)
		if err != nil {
			return nil, err
		}
		out = append(out, SettingRow{Flag: r.Name, Takes: takes, Default: r.Default, Detail: detail})
	}
	return out, nil
}

// outcomeRows says what each reaction in the set means, in the language being
// rendered.
func (v view) outcomeRows(f PresetFacts) ([]OutcomeRow, error) {
	out := make([]OutcomeRow, 0, len(f.Outcomes))
	for _, o := range f.Outcomes {
		meaning, ok := v.Lang.Outcomes[o.Name]
		if !ok {
			return nil, fmt.Errorf("the outcome %q has no meaning written in %s", o.Name, v.Lang.Code)
		}
		out = append(out, OutcomeRow{Name: o.Name, Meaning: meaning, Count: o.Count})
	}
	return out, nil
}

// grouped writes a byte count in threes separated by spaces, as the command
// line does. A plain space rather than a narrow one, because the English
// pages are held to ASCII.
func grouped(n int64) string {
	digits := strconv.FormatInt(n, 10)
	var b strings.Builder
	for i, d := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(d)
	}
	return b.String()
}
