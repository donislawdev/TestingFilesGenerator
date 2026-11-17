package guard

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
	"github.com/donislawdev/TestingFilesGenerator/internal/tool"
	"github.com/donislawdev/TestingFilesGenerator/internal/version"
)

// The window in the language it speaks, end to end - docs/OKNO-PO-POLSKU-
// 2026-10-05.md. A list shows its values under names in that language and
// hands back the values, the engine's refusals arrive in it, and the words
// around them - time left, the licence - follow. What these guards hold is
// what a translation can get wrong without a word: two values under one name,
// a sentence naming a value the list calls something else, a refusal that
// never went through the catalogue.

// declaredLists is every closed list a window offers, by whose it is, with
// the setting the values belong to.
type declaredList struct {
	owner, setting string
	values         []string
}

func declaredLists() []declaredList {
	var out []declaredList
	add := func(owner string, props []format.Property) {
		for _, p := range props {
			if len(p.Choices) > 0 {
				out = append(out, declaredList{owner, p.Name, p.Choices})
			}
		}
	}
	for _, d := range format.All() {
		add(string(text.FormatOwner(d.ID)), d.Properties)
	}
	for _, p := range preset.All() {
		add(string(text.PresetOwner(p.ID)), append(append([]format.Property{}, p.Parameters...), p.Globals()...))
	}
	for _, d := range tool.All() {
		add(string(text.ToolOwner(d.ID)), d.Settings)
	}
	for _, d := range damage.All() {
		add(string(text.DamageOwner(d.ID)), d.Parameters)
	}
	out = append(out, declaredList{"recipe", recipe.KeyExpected, recipe.Outcomes()},
		declaredList{"recipe", recipe.KeyExpectedReason, recipe.Reasons()})
	return out
}

// TestEveryListSaysEachValueOnceInEveryLanguage holds a list to naming its
// values apart. Two values under one name are two rows nobody can choose
// between, and the one somebody picks writes a value they did not mean into
// the recipe.
func TestEveryListSaysEachValueOnceInEveryLanguage(t *testing.T) {
	lists := declaredLists()
	if len(lists) < 20 {
		t.Fatalf("found %d list(s), so the registries were not filled in this process", len(lists))
	}
	for tag, entries := range registryFiles(t) {
		for _, l := range lists {
			seen := map[string]string{}
			for _, v := range l.values {
				name := v
				if e, has := entries[text.ChoiceKey(l.setting, v)]; has && e["other"] != "" {
					name = e["other"]
				}
				if other, twice := seen[strings.ToLower(name)]; twice {
					t.Errorf("%s calls %s and %s of %s %s both %q", tag, other, v, l.owner, l.setting, name)
				}
				seen[strings.ToLower(name)] = v
			}
		}
	}
}

// valueWord is a value written on its own in a sentence, not part of a longer
// word, a key, a flag or a path - in any case, because a value opening a
// sentence is written with a capital (the first version matched lower case
// only and passed four Polish sentences opening with Mixed, Fixed and
// Realistic, 2026-10-06).
func valueWord(value string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)(?:^|[^\w.\-=:/])` + regexp.QuoteMeta(value) + `(?:$|[^\w\-=:/])`)
}

// TestNoTranslationNamesAListValueTheListCallsOtherwise holds the sentence
// under a setting to the words of the list above it. A Polish window whose
// list says średnik and whose sentence says "choose semicolon" is the mixed
// language this work removed, one line down from where it was (the owner's
// decision of 2026-10-05: a value of a list is said in a sentence by its name
// in the list). Values somebody types - all in a list of format ids, none in
// a date - are not on a list and are not asked about.
//
// English too, since its lists have names of their own: a menu offering a test
// pattern under a sentence about test is the same two words for one thing.
// English is the language the values are written in, so one of its sentences
// may use a value as the word it is - LibreOffice reads both - and there a
// value that is a plain word is held only in the sentence of its own setting.
// A value spelled as a key, record-per-line, is held everywhere.
func TestNoTranslationNamesAListValueTheListCallsOtherwise(t *testing.T) {
	byOwner := map[string][]declaredList{}
	for _, l := range declaredLists() {
		byOwner[l.owner] = append(byOwner[l.owner], l)
	}
	checked := map[string]int{}
	for tag, entries := range registryFiles(t) {
		wrong, asked := valuesNamedOtherwise(entries, byOwner, tag == text.English)
		checked[tag] = asked
		for _, w := range wrong {
			t.Errorf("%s %s", tag, w)
		}
	}
	// Each language on its own, English among them: one language asked about
	// fifty values would hide another asked about none.
	if len(checked) < 2 {
		t.Fatalf("%d language(s) read, so no translation was held to a list", len(checked))
	}
	for tag, n := range checked {
		if n < 20 {
			t.Errorf("%s was asked about %d value(s), so its sentences were not held to its lists", tag, n)
		}
	}
	if checked[text.English] == 0 {
		t.Error("English was not asked at all, and its lists name values as much as any other language's")
	}

	// The canaries, on the png's list of what a picture shows: a sentence
	// saying test where the list says test pattern has to be found, and one
	// saying the name, which holds the value inside it, must not be.
	name := map[string]string{"other": "test pattern"}
	said := func(sentence string) map[string]map[string]string {
		return map[string]map[string]string{text.ChoiceKey("content", "test"): name,
			"Detail.format/png.content": {"other": sentence}}
	}
	if wrong, _ := valuesNamedOtherwise(said("What the picture shows. test checks a reader."), byOwner, true); len(wrong) == 0 {
		t.Error("canary: a sentence saying test under a list saying test pattern passed")
	}
	if wrong, _ := valuesNamedOtherwise(said("What the picture shows. A test pattern checks a reader."), byOwner, true); len(wrong) != 0 {
		t.Errorf("canary: a sentence saying the name was taken for one saying the value: %v", wrong)
	}
}

// valuesNamedOtherwise is every sentence of one catalogue naming a value of a
// list that the catalogue gives another name, and how many values it asked
// about. The names of a list are cut out of a sentence before its values are
// looked for, so a test pattern is not read as test.
func valuesNamedOtherwise(entries map[string]map[string]string, byOwner map[string][]declaredList, english bool) ([]string, int) {
	var wrong []string
	checked := 0
	for key, entry := range entries {
		if !strings.HasPrefix(key, "Detail.") {
			continue
		}
		owner, setting := ownerOfDetail(key), strings.TrimPrefix(key, "Detail."+ownerOfDetail(key)+".")
		for _, l := range byOwner[owner] {
			names := map[string]string{}
			var longestFirst []string
			for _, v := range l.values {
				if n := entries[text.ChoiceKey(l.setting, v)]["other"]; n != "" && !strings.EqualFold(n, v) {
					names[v] = n
					longestFirst = append(longestFirst, n)
				}
			}
			sort.Slice(longestFirst, func(i, j int) bool { return len(longestFirst[i]) > len(longestFirst[j]) })
			sentence := entry["other"]
			for _, n := range longestFirst {
				sentence = regexp.MustCompile(`(?i)`+regexp.QuoteMeta(n)).ReplaceAllString(sentence, " ")
			}
			for v, n := range names {
				if len(v) < 3 || english && l.setting != setting && !strings.ContainsAny(v, "_-") {
					continue
				}
				checked++
				if valueWord(v).MatchString(sentence) {
					wrong = append(wrong, fmt.Sprintf("%s names %q, which the %s list above it calls %q:\n  %s",
						key, v, l.setting, n, entry["other"]))
				}
			}
		}
	}
	sort.Strings(wrong)
	return wrong, checked
}

// ownerOfDetail is the owner part of Detail.<owner>.<setting>, where an owner
// holds one slash: format/pdf.
func ownerOfDetail(key string) string {
	rest := strings.TrimPrefix(key, "Detail.")
	if at := strings.LastIndex(rest, "."); at > 0 {
		return rest[:at]
	}
	return rest
}

// TestThePresetTitlesInTheWindowAreTheSites holds the window's titles of the
// presets to the ones the site already says, the way the questions are held -
// one Polish title in two files that nothing compared would drift.
func TestThePresetTitlesInTheWindowAreTheSites(t *testing.T) {
	raw, err := os.ReadFile(repoRoot(t) + "/web/content/pl/site.json")
	if err != nil {
		t.Fatalf("reading the Polish site: %v", err)
	}
	var site struct {
		Presets map[string]struct {
			Title string `json:"title"`
		} `json:"presets"`
	}
	if err := json.Unmarshal(raw, &site); err != nil {
		t.Fatal(err)
	}
	window := registryFiles(t)["pl"]
	ids := preset.IDs()
	sort.Strings(ids)
	for _, id := range ids {
		said := window[text.TitleKey(id)]["other"]
		if said == "" || said != site.Presets[id].Title {
			t.Errorf("the window calls the %s preset %q and the site calls it %q", id, said, site.Presets[id].Title)
		}
	}
}

// listWordsChild runs the guards below in a process of their own - each loads
// a catalogue, which changes the words of everything that runs after it.
const (
	listWordsPolishChild  = "TFG_LIST_WORDS_POLISH_CHILD"
	listWordsPseudoChild  = "TFG_LIST_WORDS_PSEUDO_CHILD"
	listWordsEnglishChild = "TFG_LIST_WORDS_ENGLISH_CHILD"
)

// engineRefusals is a refusal from each package of the engine, made by asking
// it for something it refuses - not built by hand, so each is the error a
// window is really shown.
func engineRefusals(t *testing.T) map[string]error {
	t.Helper()
	out := map[string]error{}
	avif, _ := format.Get("avif")
	_, out["format minimum"] = avif.Generator.Plan(format.Request{Bytes: 1, Label: true})
	png, _ := format.Get("png")
	out["format value"] = png.CheckProperties(map[string]string{"width": "99999"})
	_, out["engine"] = engine.Plan([]engine.Target{{ID: "a", Format: "txt", Sizes: engine.Uniform(1, 10)}}, engine.Options{})
	_, out["recipe"] = recipe.Parse([]byte("version: 1\n"), "r.yaml")
	_, out["preset"] = preset.Expand("nosuch", nil)
	_, out["damage"] = damage.Get("nosuch")
	_, out["tool"] = tool.Get("nosuch")
	_, out["size"] = core.ParseSize("abc")
	for what, err := range out {
		var said interface{ Said() core.Said }
		if err == nil || !errors.As(err, &said) {
			t.Fatalf("asking the %s for something it refuses gave %v, which is not a sentence kept as data", what, err)
		}
	}
	return out
}

// TestTheEnginesRefusalsReachThePolishWindowInPolish is the whole path for the
// engine's sentences: a refusal from each package, the catalogue compiled
// into this build, and the window's way of saying an error. A refusal that
// reached the window as its English would be the mixed window this work is
// about, and the file guards above would not notice a window that never asks.
func TestTheEnginesRefusalsReachThePolishWindowInPolish(t *testing.T) {
	if os.Getenv(listWordsPolishChild) != "1" {
		runChild(t, "TestTheEnginesRefusalsReachThePolishWindowInPolish", listWordsPolishChild)
		return
	}
	refusals := engineRefusals(t)
	if err := text.LoadBuiltIn("pl"); err != nil {
		t.Fatalf("Polish would not load: %v", err)
	}
	for what, err := range refusals {
		for _, one := range spreadAll(t, err) {
			said := text.Refusal(one, "")
			if said == one.Error() {
				t.Errorf("the %s refusal reached the Polish window in English: %q", what, said)
			}
		}
	}
	// A list of a declared setting: the value under its Polish name in the
	// box, and the value itself handed back.
	pdf, _ := format.Get("pdf")
	fields, _ := parts.PropertyFields(pdf, parts.NewFields(), nil)
	for _, f := range fields {
		menu, isMenu := f.Control.(*parts.Chooser)
		if !isMenu || f.Name != "orientation" {
			continue
		}
		test.NewTempWindow(t, menu)
		menu.SetSelected("portrait")
		if f.Value() != "portrait" {
			t.Errorf("the orientation menu hands back %q, and a recipe needs portrait", f.Value())
		}
		// Read off the words the box draws, not off the menu: the menu's
		// Selected is the value, and it is meant to stay the value.
		if shown := wordsInTheBox(t, menu).Text; shown != text.ChoiceName("orientation", "portrait") {
			t.Errorf("the orientation menu shows %q, and the Polish window calls portrait %q", shown, text.ChoiceName("orientation", "portrait"))
		}
		return
	}
	t.Fatal("the PDF declares no orientation menu, so nothing was asked of a list")
}

// spreadAll opens a refusal carrying several into the ones it carries, the
// way the window does before it says them. Every refusal opens into one at
// least, and a guard is failed by one that opens into none: until 2026-10-06
// a refusal unwrapped to the list of what it wrapped, empty for most, so the
// window marked no box for them and the guards above asked them nothing.
func spreadAll(t *testing.T, err error) []error {
	t.Helper()
	if joined, several := err.(interface{ Unwrap() []error }); several {
		var out []error
		for _, one := range joined.Unwrap() {
			out = append(out, spreadAll(t, one)...)
		}
		if len(out) == 0 {
			t.Errorf("%q opens into no refusal at all, so a window would say nothing of it", err)
		}
		return out
	}
	return []error{err}
}

// TestNoSentenceOfTheEngineReachesTheWindowUndisguised says every refusal of
// the engine in the pseudo language: every one of them comes out disguised,
// which is the proof it went through the catalogue - a refusal said around it
// would arrive as its English.
func TestNoSentenceOfTheEngineReachesTheWindowUndisguised(t *testing.T) {
	if os.Getenv(listWordsPseudoChild) != "1" {
		runChild(t, "TestNoSentenceOfTheEngineReachesTheWindowUndisguised", listWordsPseudoChild)
		return
	}
	refusals := engineRefusals(t)
	if err := text.LoadBuiltIn(text.Pseudo); err != nil {
		t.Fatalf("the pseudo language would not load: %v", err)
	}
	for what, err := range refusals {
		for _, one := range spreadAll(t, err) {
			said := text.Refusal(one, "")
			if !strings.HasPrefix(said, "[") || strings.Contains(said, one.Error()) {
				t.Errorf("the %s refusal reached the window undisguised: %q", what, said)
			}
		}
	}
}

// TestTheEnglishWindowSaysTimeAndTheLicenceAsTheCommandLineDoes holds the
// window's own composition of two things the command line also says to the
// command line's words, in English: time left, rounded the same way, and the
// licence notice, unchanged and with nothing added.
func TestTheEnglishWindowSaysTimeAndTheLicenceAsTheCommandLineDoes(t *testing.T) {
	if os.Getenv(listWordsEnglishChild) != "1" {
		runChild(t, "TestTheEnglishWindowSaysTimeAndTheLicenceAsTheCommandLineDoes", listWordsEnglishChild)
		return
	}
	if err := text.LoadBuiltIn(text.English); err != nil {
		t.Fatal(err)
	}
	for _, d := range []time.Duration{0, 999 * time.Millisecond, 59 * time.Second, time.Minute, 59*time.Minute + 30*time.Second,
		time.Hour, 3*time.Hour + 7*time.Minute, 49 * time.Hour} {
		if got, want := text.Roughly(d), core.Roughly(d); got != want {
			t.Errorf("%v is %q in the window and %q on the command line", d, got, want)
		}
	}
	if got := text.Licence(version.LicenceNotice); got != strings.TrimSpace(version.LicenceNotice) {
		t.Errorf("the English window's licence notice is not the notice the command line prints:\n%s", got)
	}
}
