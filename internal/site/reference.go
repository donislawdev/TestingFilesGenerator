// This file holds what the reference pages of the documentation show: every
// command with every flag, every key of a recipe, every field of a manifest,
// every setting of every format, and the recipes to download.
//
// Every list here comes from the program, and the page only adds a sentence
// beside each item. Those sentences are English, in the root language file
// alone, because the pages are (docs.go), and each one is looked up by the item
// it describes, so an item nobody wrote a sentence for stops the render rather
// than leaving a blank row. The other direction - a sentence about an item the
// program no longer has - is asked by the guards in internal/guard.

package site

import (
	"fmt"
	"html/template"
	"path"
	"strings"
)

// RecipesDir is where the recipes of the gallery are published, beside the
// page that lists them, with the presets in a folder of their own.
const RecipesDir = "recipes"

// DocsFacts is what the documentation lists, read from the program.
//
// Commands is every command tfg --help prints, in the order it prints them,
// each as its own --help describes it. Taking the list from the help is the
// point: it is the text a visitor compares the page against, so agreeing
// with anything else would prove the wrong thing. RecipeKeys is every key
// recipe.Keys gives, Outcomes and Reasons the closed lists an expectation is
// written in, ManifestFields every field a manifest can carry by its dotted
// path, and Recipes every recipe of the gallery, the presets among them.
type DocsFacts struct {
	Commands       []CommandDoc
	RecipeKeys     []string
	Outcomes       []string
	Reasons        []string
	ManifestFields []string
	Recipes        []Recipe
}

// CommandNames is the names of the commands, in the order tfg --help prints
// them.
func (d DocsFacts) CommandNames() []string {
	out := make([]string, len(d.Commands))
	for i, c := range d.Commands {
		out[i] = c.Name
	}
	return out
}

// DocsText is the text of the documentation in one language.
//
// A translated language has Guides alone: a line about each page written in
// the root language only, keyed by the page, for the hub and its menu. The
// root language has the rest, the sentences beside the lists of the English
// pages: RecipeKeys keyed by the dotted key as recipe.Keys gives it,
// ManifestFields by the dotted path of the field and Recipes by the id of a
// recipe of the gallery.
type DocsText struct {
	Guides         map[string]GuideText  `json:"guides,omitempty"`
	RecipeKeys     map[string]string     `json:"recipeKeys,omitempty"`
	ManifestFields map[string]string     `json:"manifestFields,omitempty"`
	Recipes        map[string]RecipeText `json:"recipes,omitempty"`
}

// expanded runs the lines about the English pages through the facts, as every
// other text of a language is (view.go). The sentences of the reference are
// read as they are.
func (d *DocsText) expanded(through func(string) string) *DocsText {
	if d == nil {
		return nil
	}
	out := *d
	out.Guides = make(map[string]GuideText, len(d.Guides))
	for key, g := range d.Guides {
		out.Guides[key] = GuideText{Title: through(g.Title), Text: through(g.Text)}
	}
	return &out
}

// RecipeText says what one recipe of the gallery is for.
type RecipeText struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

// CommandDoc is one command as tfg <command> --help describes it.
//
// About is the paragraphs before the usage, Usage its lines as printed, and
// Notes the paragraphs after it. A command whose --help prints no help of its
// own, such as version, has a summary and nothing else.
type CommandDoc struct {
	Name    string
	Summary string
	About   []string
	Usage   []string
	Notes   []string
	Flags   []FlagDoc
}

// Anchor is the id of the command's section on the page.
func (c CommandDoc) Anchor() string { return "tfg-" + addressableName(c.Name) }

// FlagDoc is one flag as the help prints it. Takes is the word after the name,
// such as string or int, and empty for a flag that takes no value.
type FlagDoc struct {
	Name    string
	Takes   string
	Default string
	Text    string
}

// Recipe is one recipe the gallery offers, with what a dry run of it counts.
type Recipe struct {
	ID      string
	Preset  bool
	Body    []byte
	Files   int
	Bytes   int64
	Formats []string
}

// File is where the recipe is published under the root of the site.
func (r Recipe) File() string {
	if r.Preset {
		return path.Join(RecipesDir, "presets", r.ID+".yaml")
	}
	return path.Join(RecipesDir, r.ID+".yaml")
}

// KeyRow is one key of a recipe, or one field of a manifest, with what it
// does. Target is set for a key of a target rather than of the whole recipe.
type KeyRow struct {
	Key    string
	Text   template.HTML
	Target bool
}

// guide is the line about one English page, empty when there is none.
func (d *DocsText) guide(key string) GuideText {
	if d == nil {
		return GuideText{}
	}
	return d.Guides[key]
}

// docsText is the text of the documentation in the language, or an error
// naming what asked for it on a language that has none.
func docsText(v view) (*DocsText, error) {
	if v.Lang.Docs == nil {
		return nil, fmt.Errorf("the %s page %q shows reference text and the %s language file has none", v.Lang.Code, v.Page.Key, v.Lang.Code)
	}
	return v.Lang.Docs, nil
}

// RecipeKeyRows is every key a recipe accepts, in the order recipe.Keys gives
// them, with what each does.
func (v view) RecipeKeyRows() ([]KeyRow, error) {
	ref, err := docsText(v)
	if err != nil {
		return nil, err
	}
	out := make([]KeyRow, 0, len(v.Facts.Docs.RecipeKeys))
	for _, k := range v.Facts.Docs.RecipeKeys {
		text, ok := ref.RecipeKeys[k]
		if !ok || text == "" {
			return nil, fmt.Errorf("the recipe key %q has no sentence in the %s reference", k, v.Lang.Code)
		}
		out = append(out, KeyRow{Key: k, Text: withCode(text), Target: strings.HasPrefix(k, "targets.")})
	}
	return out, nil
}

// ManifestRows is every field a manifest can carry, with what it holds.
func (v view) ManifestRows() ([]KeyRow, error) {
	ref, err := docsText(v)
	if err != nil {
		return nil, err
	}
	out := make([]KeyRow, 0, len(v.Facts.Docs.ManifestFields))
	for _, f := range v.Facts.Docs.ManifestFields {
		text, ok := ref.ManifestFields[f]
		if !ok || text == "" {
			return nil, fmt.Errorf("the manifest field %q has no sentence in the %s reference", f, v.Lang.Code)
		}
		out = append(out, KeyRow{Key: f, Text: withCode(text)})
	}
	return out, nil
}

// SettingDoc is one setting of a format as the page of format settings lists
// it, in the registry's own words.
type SettingDoc struct {
	Name    string
	Takes   template.HTML
	Default template.HTML
	Detail  template.HTML
}

// entities writes every character above ASCII in an HTML string as a numeric
// reference, which the browser shows as the character.
//
// The English pages are held to ASCII (TestTranslatedTextStaysOnTheTranslatedPages),
// and the registry's own words carry a few such characters - the default text
// of a placeholder picture is "{width} x {height}" with a multiplication sign.
// They are the program's text, so they are shown as they are rather than
// rewritten.
func entities(h template.HTML) template.HTML {
	var b strings.Builder
	for _, r := range string(h) {
		if r > 127 {
			fmt.Fprintf(&b, "&#%d;", r)
			continue
		}
		b.WriteRune(r)
	}
	return template.HTML(b.String()) //nolint:gosec // escaped before, and a reference is no markup
}

// FormatSettings is one format and every setting it takes.
type FormatSettings struct {
	ID       string
	Name     string
	Settings []SettingDoc
}

// FormatSettingsList is every format that takes settings, in the order the
// registry lists them.
func (v view) FormatSettingsList() ([]FormatSettings, error) {
	var out []FormatSettings
	for _, f := range v.Facts.WithProperties() {
		settings, err := settingDocs(v, f)
		if err != nil {
			return nil, err
		}
		out = append(out, FormatSettings{ID: f.ID, Name: f.Name, Settings: settings})
	}
	return out, nil
}

// settingDocs is every setting of one format, in the registry's words.
func settingDocs(v view, f Format) ([]SettingDoc, error) {
	out := make([]SettingDoc, 0, len(f.Properties))
	for _, p := range f.Properties {
		takes, err := v.AllowedOf(p)
		if err != nil {
			return nil, err
		}
		if p.Detail == "" {
			return nil, fmt.Errorf("the setting %s of %s has no description in the registry, so its row would be blank", p.Name, f.ID)
		}
		out = append(out, SettingDoc{Name: p.Name, Takes: entities(withCode(takes)),
			Default: entities(template.HTML(template.HTMLEscapeString(p.Default))), Detail: entities(withCode(p.Detail))}) //nolint:gosec // escaped here
	}
	return out, nil
}

// RecipeRow is one recipe as the gallery lists it.
type RecipeRow struct {
	ID      string
	Title   string
	Text    string
	URL     string
	File    string
	Page    string
	Files   int
	Bytes   string
	Formats []string
}

// GalleryRecipes is the recipes of the gallery, described in the reference
// text, in the order the program gives them.
func (v view) GalleryRecipes() ([]RecipeRow, error) { return recipeRows(v, false) }

// PresetRecipes is every preset as a recipe, described by its own title and
// question, each leading to the page of the preset as well.
func (v view) PresetRecipes() ([]RecipeRow, error) { return recipeRows(v, true) }

// recipeRows is the recipes of one kind as rows.
func recipeRows(v view, presets bool) ([]RecipeRow, error) {
	ref, err := docsText(v)
	if err != nil {
		return nil, err
	}
	var out []RecipeRow
	for _, r := range v.Facts.Docs.Recipes {
		if r.Preset != presets {
			continue
		}
		row := RecipeRow{ID: r.ID, URL: "/" + r.File(), File: path.Base(r.File()), Files: r.Files, Bytes: grouped(r.Bytes), Formats: r.Formats}
		if err := describeRecipe(v, &row, r, ref); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("the %s page %q lists recipes and there are none, so it would offer nothing", v.Lang.Code, v.Page.Key)
	}
	return out, nil
}

// describeRecipe fills in the words of one row: a preset is described by its
// own title and question and leads to its page, a recipe of the gallery by the
// reference text.
func describeRecipe(v view, row *RecipeRow, r Recipe, ref *DocsText) error {
	if r.Preset {
		text, ok := v.Lang.Presets[r.ID]
		page, found := find(v.Lang, presetKey(r.ID))
		if !ok || !found {
			return fmt.Errorf("the preset %q is offered as a recipe and has no %s words or page", r.ID, v.Lang.Code)
		}
		row.Title, row.Text, row.Page = text.Title, text.Question, pageURL(v.Lang, page)
		return nil
	}
	text, ok := ref.Recipes[r.ID]
	if !ok || text.Title == "" || text.Text == "" {
		return fmt.Errorf("the recipe %q has no title or description in the %s reference", r.ID, v.Lang.Code)
	}
	row.Title, row.Text = text.Title, text.Text
	return nil
}

// addressableName turns a command name such as "recipe fmt" into a part of an
// address.
func addressableName(name string) string { return strings.ReplaceAll(name, " ", "-") }
