package guard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/goccy/go-yaml"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
	"github.com/donislawdev/TestingFilesGenerator/internal/site"
)

// The pages of the documentation state what the program says about itself:
// the help of every command, the keys of a recipe, the fields of a manifest and
// the recipes to download. The functions here read those out of the program,
// and the guards below hold the English sentences beside them to it in both
// directions, so a flag, a key or a field added without a word about it stops
// the render, and a sentence about one that is gone turns a guard red
// (docs/DOCS-SITE-2026-11-24.md).

// recipeGalleryDir holds the recipes the gallery offers for download.
const recipeGalleryDir = "web/recipes"

// docsAnswers are read once per process, since several site guards render the
// whole site and the answers cannot change between them.
var docsAnswers struct {
	once    sync.Once
	docs    []site.CommandDoc
	recipes []site.Recipe
	err     error
}

// commandDocsAndRecipes asks the program for the help of every command and
// runs every recipe of the gallery dry.
func commandDocsAndRecipes(t *testing.T, presets []site.PresetFacts) ([]site.CommandDoc, []site.Recipe) {
	t.Helper()
	root := repoRoot(t)
	docsAnswers.once.Do(func() {
		docsAnswers.docs, docsAnswers.err = commandDocsOfTheProgram()
		if docsAnswers.err == nil {
			docsAnswers.recipes, docsAnswers.err = galleryRecipesOf(filepath.Join(root, filepath.FromSlash(recipeGalleryDir)), presets)
		}
	})
	if docsAnswers.err != nil {
		t.Fatal(docsAnswers.err)
	}
	return docsAnswers.docs, docsAnswers.recipes
}

// commandDocsOfTheProgram is every command tfg --help lists, in its order and
// read out of its text, each as its own --help describes it.
func commandDocsOfTheProgram() ([]site.CommandDoc, error) {
	top, err := runQuietly("--help")
	if err != nil {
		return nil, err
	}
	names, err := commandNamesIn(string(top))
	if err != nil {
		return nil, fmt.Errorf("reading the command block out of tfg --help: %w", err)
	}
	summaries := map[string]string{}
	for _, c := range cli.Commands() {
		summaries[c.Name] = c.Summary
	}
	out := make([]site.CommandDoc, 0, len(names))
	for _, name := range names {
		var stdout, stderr bytes.Buffer
		args := append(strings.Fields(name), "--help")
		if code := cli.Run(context.Background(), args, &stdout, &stderr); code != cli.ExitOK {
			return nil, fmt.Errorf("tfg %s ended with %d: %s", strings.Join(args, " "), code, stderr.String())
		}
		doc, err := parseCommandHelp(name, summaries[name], stdout.String())
		if err != nil {
			return nil, err
		}
		out = append(out, doc)
	}
	return out, nil
}

// flagDefault is the end flag.PrintDefaults gives a flag whose default is not
// its zero value.
var flagDefault = regexp.MustCompile(` \(default (.*)\)$`)

// parseCommandHelp reads one --help into its parts.
//
// The help is the text a reader compares the page against, so it is read
// rather than the flag sets behind it. A command whose --help prints no help
// of its own, as version prints the version, gets the summary of tfg --help
// and nothing else. A help that has a Flags block and gives no flag, or a flag
// without words, is an error: a change in what flag.PrintDefaults writes would
// otherwise leave the page without its tables and every check green.
func parseCommandHelp(name, summary, help string) (site.CommandDoc, error) {
	doc := site.CommandDoc{Name: name, Summary: sentence(summary)}
	head := "tfg " + name + " - "
	if !strings.HasPrefix(help, head) {
		return doc, nil
	}
	first, rest, _ := strings.Cut(help, "\n")
	doc.Summary = sentence(strings.TrimPrefix(first, head))
	usageSeen, flagsSeen := false, false
	for _, block := range strings.Split(strings.ReplaceAll(rest, "\r\n", "\n"), "\n\n") {
		block = strings.Trim(block, "\n")
		switch {
		case block == "":
		case strings.HasPrefix(block, "Usage:\n"):
			usageSeen = true
			for _, line := range strings.Split(strings.TrimPrefix(block, "Usage:\n"), "\n") {
				doc.Usage = append(doc.Usage, strings.TrimPrefix(line, "  "))
			}
		case strings.HasPrefix(block, "Flags:\n"):
			flagsSeen = true
			flags, err := parseFlags(strings.TrimPrefix(block, "Flags:\n"))
			if err != nil {
				return doc, fmt.Errorf("tfg %s --help: %w", name, err)
			}
			doc.Flags = append(doc.Flags, flags...)
		case usageSeen:
			doc.Notes = append(doc.Notes, strings.Join(strings.Fields(block), " "))
		default:
			doc.About = append(doc.About, strings.Join(strings.Fields(block), " "))
		}
	}
	if !usageSeen {
		return doc, fmt.Errorf("tfg %s --help has no Usage block, so its section would not say how to run it", name)
	}
	if flagsSeen && len(doc.Flags) == 0 {
		return doc, fmt.Errorf("tfg %s --help has a Flags block and no flag was read out of it", name)
	}
	return doc, nil
}

// parseFlags reads the block flag.PrintDefaults writes: "  -name type" and the
// words under it after a tab, or "  -x" and the words on the same line after a
// tab for a short name.
func parseFlags(block string) ([]site.FlagDoc, error) {
	var out []site.FlagDoc
	for _, line := range strings.Split(block, "\n") {
		switch {
		case strings.HasPrefix(line, "  -"):
			head, words, short := strings.Cut(strings.TrimPrefix(line, "  -"), "\t")
			name, takes, _ := strings.Cut(head, " ")
			f := site.FlagDoc{Name: name, Takes: strings.TrimSpace(takes)}
			if short {
				f.Text = strings.TrimSpace(words)
			}
			out = append(out, f)
		case strings.HasPrefix(line, "    \t") && len(out) > 0:
			last := &out[len(out)-1]
			last.Text = strings.TrimSpace(last.Text + " " + strings.TrimPrefix(line, "    \t"))
		default:
			return nil, fmt.Errorf("a line of the Flags block is in no shape this reads: %q", line)
		}
	}
	for i := range out {
		if m := flagDefault.FindStringSubmatch(out[i].Text); m != nil {
			out[i].Text = strings.TrimSuffix(out[i].Text, m[0])
			out[i].Default = m[1]
			if unquoted, err := strconv.Unquote(m[1]); err == nil {
				out[i].Default = unquoted
			}
		}
		if out[i].Name == "" || out[i].Text == "" {
			return nil, fmt.Errorf("the flag %q has no name or no words", out[i].Name)
		}
	}
	return out, nil
}

// sentence starts a line of the help with a capital and ends it with a stop,
// as a sentence on a page.
func sentence(s string) string {
	s = strings.TrimSuffix(strings.TrimSpace(s), ".")
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:] + "."
}

// manifestFieldsOf is every field a manifest can carry, by dotted path, in the
// order the structures declare them. An element of a list is written as [] and
// a value of a map keyed by the user's words as *.
func manifestFieldsOf() []string {
	var out []string
	var walk func(t reflect.Type, prefix string)
	walk = func(t reflect.Type, prefix string) {
		for t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		switch t.Kind() {
		case reflect.Slice:
			walk(t.Elem(), prefix+"[]")
			return
		case reflect.Map:
			walk(t.Elem(), prefix+".*")
			return
		case reflect.Struct:
		default:
			return
		}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.Anonymous {
				walk(f.Type, prefix)
				continue
			}
			name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			if !f.IsExported() || name == "" || name == "-" {
				continue
			}
			path := name
			if prefix != "" {
				path = prefix + "." + name
			}
			out = append(out, path)
			walk(f.Type, path)
		}
	}
	walk(reflect.TypeOf(manifest.Manifest{}), "")
	return out
}

// galleryRecipesOf is every recipe of the gallery, by file name, and every
// preset as the recipe tfg preset eject prints, each with what a dry run
// counts.
func galleryRecipesOf(dir string, presets []site.PresetFacts) ([]site.Recipe, error) {
	names, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	var out []site.Recipe
	for _, file := range names {
		body, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		r := site.Recipe{ID: strings.TrimSuffix(filepath.Base(file), ".yaml"), Body: body}
		if r.Files, r.Bytes, r.Formats, err = dryRunOf(file); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	for _, p := range presets {
		body, err := runQuietly("preset", "eject", p.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, site.Recipe{ID: p.ID, Preset: true, Body: body,
			Files: p.Budget.Files, Bytes: p.Budget.Bytes, Formats: p.Budget.Formats})
	}
	return out, nil
}

// dryRunOf is what a dry run of one recipe counts: files, bytes and formats.
func dryRunOf(file string) (int, int64, []string, error) {
	dir, err := os.MkdirTemp("", "tfg-gallery-")
	if err != nil {
		return 0, 0, nil, err
	}
	defer os.RemoveAll(dir)
	printed, err := runQuietly("generate", file, "--dry-run", "--json", "--out", dir)
	if err != nil {
		return 0, 0, nil, err
	}
	// The bytes are added up from the files. The summary of a dry run counts
	// what was written, which is nothing.
	var m struct {
		Summary struct {
			FileCount int            `json:"file_count"`
			ByFormat  map[string]int `json:"by_format"`
		} `json:"summary"`
		Files []struct {
			Bytes int64 `json:"bytes"`
		} `json:"files"`
	}
	if err := json.Unmarshal(printed, &m); err != nil {
		return 0, 0, nil, fmt.Errorf("reading the dry run of %s: %w", filepath.Base(file), err)
	}
	var total int64
	for _, f := range m.Files {
		total += f.Bytes
	}
	formats := make([]string, 0, len(m.Summary.ByFormat))
	for f := range m.Summary.ByFormat {
		formats = append(formats, f)
	}
	sort.Strings(formats)
	return m.Summary.FileCount, total, formats, nil
}

// describedBothWays is what is wrong between the items the program has and
// the sentences written about them: an item without one, and a sentence about
// an item the program does not have.
func describedBothWays[V any](what string, items []string, said map[string]V) []string {
	var out []string
	for _, item := range items {
		if _, ok := said[item]; !ok {
			out = append(out, fmt.Sprintf("the %s %q has no sentence", what, item))
		}
	}
	for item := range said {
		if !slices.Contains(items, item) {
			out = append(out, fmt.Sprintf("there is a sentence about the %s %q, which the program does not have", what, item))
		}
	}
	sort.Strings(out)
	return out
}

// The command reference is read out of each command's own --help.
//
// Its canary: a help in every shape the reader knows - a flag with a type and
// a default, a short flag on one line, a paragraph on each side of the usage -
// has to come back whole, and a Flags block it cannot read has to be an error.
func TestTheCommandReferenceIsReadFromTheHelp(t *testing.T) {
	canary := "tfg demo - do one thing.\n\nWhat it is for.\n\nUsage:\n  tfg demo <file>\n\nA note.\n\n" +
		"Flags:\n  -count int\n    \thow many (default 1)\n  -name string\n    \tthe name (default \"files\")\n  -w\twrite it back\n"
	doc, err := parseCommandHelp("demo", "", canary)
	want := site.CommandDoc{Name: "demo", Summary: "Do one thing.", About: []string{"What it is for."},
		Usage: []string{"tfg demo <file>"}, Notes: []string{"A note."}, Flags: []site.FlagDoc{
			{Name: "count", Takes: "int", Default: "1", Text: "how many"},
			{Name: "name", Takes: "string", Default: "files", Text: "the name"},
			{Name: "w", Text: "write it back"},
		}}
	if err != nil || !reflect.DeepEqual(doc, want) {
		t.Fatalf("the canary help came back as %+v (%v), and it is %+v", doc, err, want)
	}
	if _, err := parseCommandHelp("demo", "", "tfg demo - x.\n\nUsage:\n  tfg demo\n\nFlags:\n  count int\n"); err == nil {
		t.Fatal("a Flags block in a shape nothing reads was taken without an error")
	}

	docs, _ := commandDocsAndRecipes(t, presetFactsFromTheProgram(t))
	for _, d := range docs {
		if d.Summary == "" {
			t.Errorf("tfg %s has no summary, so its row on the page would be blank", d.Name)
		}
	}
	for _, d := range docs {
		if d.Name != "generate" {
			continue
		}
		read := map[string]bool{}
		for _, f := range d.Flags {
			read[f.Name] = true
		}
		if !maps.Equal(read, flagsOfGenerate(t)) {
			t.Errorf("the reference reads the flags %v of generate and its help has %v", read, flagsOfGenerate(t))
		}
	}
}

// The English reference says something about every key of a recipe, every
// field of a manifest and every recipe of the gallery, and about nothing else.
//
// Its canary: an item without a sentence and a sentence about an item that is
// not there have to give two problems.
func TestTheReferenceDescribesEveryKeyFieldAndRecipe(t *testing.T) {
	if got := describedBothWays("key", []string{"a", "b"}, map[string]string{"a": "x", "c": "y"}); len(got) != 2 {
		t.Fatalf("the canary gave %d problem(s) and has two: %v", len(got), got)
	}
	root := rootLanguageOnDisk(t)
	if root.Docs == nil {
		t.Fatalf("the %s language file has no reference text, so the reference pages cannot be rendered", root.Code)
	}
	_, recipes := commandDocsAndRecipes(t, presetFactsFromTheProgram(t))
	var gallery []string
	for _, r := range recipes {
		if !r.Preset {
			gallery = append(gallery, r.ID)
		}
	}
	problems := describedBothWays("recipe key", recipe.Keys(), root.Docs.RecipeKeys)
	problems = append(problems, describedBothWays("manifest field", manifestFieldsOf(), root.Docs.ManifestFields)...)
	problems = append(problems, describedBothWays("recipe of the gallery", gallery, root.Docs.Recipes)...)
	for _, p := range problems {
		t.Error(p)
	}
}

// galleryRecipeProblems is what is wrong with one recipe of the gallery
// before it is run: it has to write into a folder of its own name, so two
// recipes run one after the other never write into one folder.
func galleryRecipeProblems(id string, body []byte) []string {
	var r struct {
		Output struct {
			Dir string `yaml:"dir"`
		} `yaml:"output"`
	}
	if err := yaml.Unmarshal(body, &r); err != nil {
		return []string{id + " does not read as YAML: " + err.Error()}
	}
	if r.Output.Dir != "./"+id {
		return []string{fmt.Sprintf("%s writes into %q, and a recipe of the gallery writes into ./%s", id, r.Output.Dir, id)}
	}
	return nil
}

// Every recipe of the gallery is valid, makes files and writes into a folder of
// its own name.
//
// Its canary: a recipe that writes into a folder of another name has to be
// reported.
func TestEveryRecipeOfTheGalleryIsValidAndWritesToItsOwnFolder(t *testing.T) {
	if got := galleryRecipeProblems("one", []byte("version: 1\noutput:\n  dir: ./two\n")); len(got) != 1 {
		t.Fatalf("the canary, a recipe writing into another folder, gave %d problem(s): %v", len(got), got)
	}
	_, recipes := commandDocsAndRecipes(t, presetFactsFromTheProgram(t))
	gallery := 0
	for _, r := range recipes {
		if r.Files == 0 {
			t.Errorf("the recipe %s makes no file, so the gallery would offer an empty set", r.ID)
		}
		if r.Preset {
			continue
		}
		gallery++
		for _, p := range galleryRecipeProblems(r.ID, r.Body) {
			t.Error(p)
		}
		file := filepath.Join(repoRoot(t), filepath.FromSlash(recipeGalleryDir), r.ID+".yaml")
		if _, err := runQuietly("validate", file); err != nil {
			t.Errorf("the recipe %s does not validate: %v", r.ID, err)
		}
	}
	if gallery == 0 {
		t.Errorf("%s holds no recipe, so the page that links to it would offer nothing", recipeGalleryDir)
	}
}

// rootLanguageOnDisk is the language served at the root.
func rootLanguageOnDisk(t *testing.T) site.Language {
	t.Helper()
	for _, l := range languagesOnDisk(t) {
		if l.Dir == "" {
			return l
		}
	}
	t.Fatal("no language is served at the root")
	return site.Language{}
}

// untranslatedDocs is every page of the root language written in it alone
// that sits under the documentation hub.
func untranslatedDocs(root site.Language) []string {
	var out []string
	for _, p := range root.Pages {
		if p.Untranslated && p.Parent == "docs" {
			out = append(out, p.Key)
		}
	}
	return out
}

// Every translated hub says something about every English page under it, and
// about nothing else.
//
// The canary is the one of TestTheReferenceDescribesEveryKeyFieldAndRecipe,
// which this shares the comparison with. Here it is asked that the English
// pages exist at all, so the comparison cannot pass by comparing nothing.
func TestEveryLanguageLeadsToTheEnglishDocumentation(t *testing.T) {
	langs := languagesOnDisk(t)
	english := untranslatedDocs(rootLanguageOnDisk(t))
	if len(english) == 0 {
		t.Fatal("no page under the documentation hub is marked untranslated, so this would compare nothing")
	}
	for _, l := range langs {
		if l.Dir == "" {
			if l.Docs != nil && len(l.Docs.Guides) != 0 {
				t.Errorf("%s is served at the root and describes its own pages to itself in guides", l.Code)
			}
			continue
		}
		var guides map[string]site.GuideText
		if l.Docs != nil {
			guides = l.Docs.Guides
		}
		for _, p := range describedBothWays("English page", english, guides) {
			t.Errorf("%s: %s", l.Code, p)
		}
	}
}
