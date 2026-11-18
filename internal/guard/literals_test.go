package guard

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
)

// Values to type, marked with backticks in the description of a setting and
// drawn as code (core.Pieces, docs/SIZE-FIXES-2026-11-16.md section 3.9). The
// owner's report of 2026-11-18: in the explanation of the groups of the
// picture sizes preset, nothing told the names to type from the words round
// them. These hold the marks to two promises - a value is the same in every
// language, and every surface that shows one shows it as a value.

// TestAValueToTypeIsTheSameInEveryLanguage reads every catalogue of the window
// and every word list of the site, and asks each translation of a text to
// carry exactly the values its English carries.
//
// A value is what somebody types, so it is the same in every language - a
// translator who put `all` into their own language would send the reader to
// type a word the program refuses. Both ways: a value the translation lost or
// changed, and a value it marked that the English does not have.
func TestAValueToTypeIsTheSameInEveryLanguage(t *testing.T) {
	if got := literalDisagreements(map[string]string{"x": "Leave it at `all`."},
		map[string]string{"x": "Leave it at `every`."}); len(got) == 0 {
		t.Fatal("canary: a translation that changed `all` into `every` agreed with the English, so this compares nothing")
	}

	root := repoRoot(t)
	marked := 0
	for _, dir := range []string{"", "registry", "said", "toolkit"} {
		base := filepath.Join(root, "internal", "gui", "text", "locale", dir)
		english := catalogueTexts(t, filepath.Join(base, "en.json"))
		marked += countMarked(english)
		entries, err := os.ReadDir(base)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if e.IsDir() || e.Name() == "en.json" || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			other := catalogueTexts(t, filepath.Join(base, e.Name()))
			for _, wrong := range literalDisagreements(english, other) {
				t.Errorf("internal/gui/text/locale/%s %s: %s", dir, e.Name(), wrong)
			}
		}
	}
	site := filepath.Join(root, "web", "content")
	english := siteTexts(t, filepath.Join(site, "en", "site.json"))
	siteMarked := countMarked(english)
	languages, err := os.ReadDir(site)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range languages {
		if !l.IsDir() || l.Name() == "en" {
			continue
		}
		other := siteTexts(t, filepath.Join(site, l.Name(), "site.json"))
		for _, wrong := range literalDisagreements(english, other) {
			t.Errorf("web/content/%s/site.json: %s", l.Name(), wrong)
		}
	}
	// Reached, not assumed: catalogues with no marked text at all would leave
	// this green while comparing nothing.
	if marked < 10 || siteMarked < 2 {
		t.Fatalf("the window's English names values to type in %d texts and the site's in %d - fewer than were marked "+
			"on 2026-11-18, so the files read are not the ones that carry them", marked, siteMarked)
	}
}

// literalDisagreements is every text of a translation whose values differ from
// its English, by key. A text the translation has not got is the business of
// the guards of missing translations, not this one.
func literalDisagreements(english, other map[string]string) []string {
	var out []string
	for key, said := range other {
		want, ok := english[key]
		if !ok {
			continue
		}
		if a, b := sortedLiterals(want), sortedLiterals(said); !slices.Equal(a, b) {
			out = append(out, fmt.Sprintf("%s names %q to type where the English names %q", key, b, a))
		}
	}
	slices.Sort(out)
	return out
}

func sortedLiterals(s string) []string {
	l := core.Literals(s)
	slices.Sort(l)
	return l
}

func countMarked(texts map[string]string) int {
	n := 0
	for _, s := range texts {
		if len(core.Literals(s)) > 0 {
			n++
		}
	}
	return n
}

// catalogueTexts is every text of one catalogue of the window, by key and form.
func catalogueTexts(t *testing.T, path string) map[string]string {
	t.Helper()
	var raw map[string]map[string]any
	readJSON(t, path, &raw)
	out := map[string]string{}
	for key, entry := range raw {
		for form, v := range entry {
			if s, ok := v.(string); ok && form != "hash" && form != "description" {
				out[key+"/"+form] = s
			}
		}
	}
	return out
}

// siteTexts is every string of one word list of the site, by its path.
func siteTexts(t *testing.T, path string) map[string]string {
	t.Helper()
	var raw any
	readJSON(t, path, &raw)
	out := map[string]string{}
	var walk func(at string, v any)
	walk = func(at string, v any) {
		switch it := v.(type) {
		case map[string]any:
			for k, x := range it {
				walk(at+"/"+k, x)
			}
		case []any:
			for i, x := range it {
				walk(fmt.Sprintf("%s/%d", at, i), x)
			}
		case string:
			out[at] = it
		}
	}
	walk("", raw)
	return out
}

func readJSON(t *testing.T, path string, into any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, into); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

// TestEveryValueToTypeIsShownAsAValue asks the three surfaces that draw a
// marked description to draw its values as values and its marks nowhere.
//
// The window, through the very explanation the button beside the size box
// opens, and through parts.Explanation for every description a setting
// declares - each value in the type of code in the literal ink, the words
// round it as they are, and no backtick left on the screen. The site, through
// every published preset page: the values in <code>, no backtick. The command
// line prints the marks as they are, which is what the CLI guards already
// read.
func TestEveryValueToTypeIsShownAsAValue(t *testing.T) {
	// The canary: plain Prose of a marked text draws no value as code, and
	// the check below has to say so.
	if problems := drawnAsValues(parts.Prose("Leave it at `all`."), "Leave it at `all`."); len(problems) == 0 {
		t.Fatal("canary: a paragraph drawing the backticks as they are passed, so this reads nothing")
	}

	descriptions := declaredDescriptions()
	if countMarked(descriptions) < 8 {
		t.Fatalf("%d declared descriptions name a value to type - fewer than were marked on 2026-11-18", countMarked(descriptions))
	}
	for where, said := range descriptions {
		for _, p := range drawnAsValues(parts.Explanation(said), said) {
			t.Errorf("%s: %s", where, p)
		}
	}

	// The real button: the explanation of the size box on Single batch.
	g := window.NewGenerate(newFakeHost(t))
	button := explanationBeside(t, g.Object(), text.FieldSize())
	button.Tapped(&fyne.PointEvent{})
	if button.Shown() == nil {
		t.Fatal("the explanation of the size box did not open")
	}
	for _, p := range drawnAsValues(button.Shown(), text.DetailSize()) {
		t.Errorf("the explanation of the size box: %s", p)
	}
	button.Tapped(&fyne.PointEvent{})

	// The site splits the marks itself, since it imports nothing of ours, so
	// every published page of a preset whose description names values is held
	// to the values core.Literals reads out of the program's own words.
	wanted := map[string][]string{}
	for _, p := range preset.All() {
		for _, param := range p.Parameters {
			wanted[p.ID] = append(wanted[p.ID], core.Literals(param.Detail)...)
			// And what it takes: its choices, its default and the values its
			// shape names (O358). The site's words for a size or a length of
			// time are its own and shorter, so a size names 2mb alone.
			wanted[p.ID] = append(wanted[p.ID], siteTakes(param)...)
		}
	}
	published := map[string]int{}
	root := filepath.Join(repoRoot(t), "web", "public")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Base(path) != "index.html" {
			return err
		}
		id := filepath.Base(filepath.Dir(path))
		if len(wanted[id]) == 0 {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		page, rel := string(b), filepath.ToSlash(strings.TrimPrefix(path, root))
		published[id]++
		if strings.Contains(page, core.Backtick) {
			t.Errorf("%s: a backtick is published on the page", rel)
		}
		for _, v := range wanted[id] {
			if !strings.Contains(page, "<code>"+v+"</code>") {
				t.Errorf("%s: %q is not published as a value, in <code>", rel, v)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for id, values := range wanted {
		if len(values) > 0 && published[id] == 0 {
			t.Errorf("no published page of the %s preset was read, so the site was not asked about %q", id, values)
		}
	}
	if len(published) < 2 {
		t.Fatalf("published pages of %d preset(s) naming values were read - two name them since 2026-11-18", len(published))
	}
}

// drawnAsValues is what is wrong with how o draws said: a backtick on the
// screen, a value not drawn in the type of code in the literal ink, or words
// that are not the description's.
func drawnAsValues(o fyne.CanvasObject, said string) []string {
	var values, words []string
	var problems []string
	walk(o, func(obj fyne.CanvasObject) {
		// A description naming no value is drawn as Prose, a label.
		if label, ok := obj.(*widget.Label); ok {
			if strings.Contains(label.Text, core.Backtick) {
				problems = append(problems, fmt.Sprintf("%q draws its backticks", label.Text))
			}
			words = append(words, label.Text)
			return
		}
		rich, ok := obj.(*widget.RichText)
		if !ok {
			return
		}
		for _, seg := range rich.Segments {
			ts, ok := seg.(*widget.TextSegment)
			if !ok {
				continue
			}
			if strings.Contains(ts.Text, core.Backtick) {
				problems = append(problems, fmt.Sprintf("%q draws its backticks", ts.Text))
			}
			words = append(words, ts.Text)
			if ts.Style.TextStyle.Monospace && ts.Style.ColorName == parts.ColorNameLiteral {
				values = append(values, ts.Text)
			}
		}
	})
	if want := core.Literals(said); !slices.Equal(values, want) {
		problems = append(problems, fmt.Sprintf("draws %q as values where the description names %q", values, want))
	}
	if plain := strings.ReplaceAll(said, core.Backtick, ""); !strings.Contains(strings.Join(words, ""), plain) {
		problems = append(problems, "does not draw the words of the description")
	}
	return problems
}

// declaredDescriptions is the description of every setting a format or a
// preset declares, and of every box of the two batch screens, by where it is.
func declaredDescriptions() map[string]string {
	out := map[string]string{
		"Size":         text.DetailSize(),
		"Size range":   text.DetailSizeRange(),
		"Boundary":     text.DetailBoundary(),
		"File names":   text.DetailNameTemplate(),
		"Seed":         text.DetailSeed(),
		"Smallest why": text.SmallestNeedsTheSides(),
	}
	// The start of every explanation as the window words it - what a setting
	// takes, its choices and its default, marked since O358.
	for _, d := range format.All() {
		for _, p := range d.Properties {
			out[d.ID+"."+p.Name] = p.Detail
			out[d.ID+"."+p.Name+" takes"] = parts.Allowed(p)
		}
	}
	for _, p := range preset.All() {
		for _, param := range p.Parameters {
			out["preset "+p.ID+"."+param.Name] = param.Detail
			out["preset "+p.ID+"."+param.Name+" takes"] = parts.Allowed(param)
		}
	}
	return out
}

// siteTakes is the values the settings table of the site shows as code for p:
// each choice, the default, the values its shape marks, and 2mb for a size.
func siteTakes(p format.Property) []string {
	out := slices.Clone(p.Choices)
	if p.Default != "" {
		out = append(out, p.Default)
	}
	out = append(out, core.Literals(p.Shape)...)
	if p.Kind == format.PropertySize {
		out = append(out, "2mb")
	}
	return out
}
