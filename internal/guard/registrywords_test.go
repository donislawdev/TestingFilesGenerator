package guard

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
)

// The words of the registries in the window's language -
// docs/JEZYKI-REJESTRY-2026-09-29.md, and text/registry.go for the mechanism.
// The registries stay English and the window looks their sentences up, so
// everything these guards hold is what a lookup can get wrong without a word:
// a sentence nobody translated, a translation of a sentence the registry no
// longer says, and two ways of describing one setting.

// registryDir is where every language keeps the words of the registries.
var registryDir = filepath.Join(localeDir, text.RegistryFolder)

// writeRegistryCatalogue writes registry/en.json out of the registries, the
// way TFG_WRITE_SCREEN_REFERENCE writes the stored screens.
const writeRegistryCatalogue = "TFG_WRITE_REGISTRY_CATALOGUE"

// englishHash is what an entry in another language records of the English it
// translated, in the field go-i18n reserves for it and never shows.
func englishHash(english string) string {
	sum := sha256.Sum256([]byte(english))
	return "sha256-" + hex.EncodeToString(sum[:6])
}

// registryWords is text.RegistryWords by key, held to being a list worth
// asserting about.
func registryWords(t *testing.T) map[string]text.RegistryWord {
	t.Helper()
	words := text.RegistryWords()
	// Every format, preset and damage together say far more than this. Fewer
	// means the registries were not filled in this process, and every guard
	// below would then be comparing nothing with nothing.
	if len(words) < 150 {
		t.Fatalf("the registries say %d sentence(s) the window shows, so they were not registered here", len(words))
	}
	out := make(map[string]text.RegistryWord, len(words))
	for _, w := range words {
		out[w.Key] = w
	}
	return out
}

// registryReflection is registry/en.json as the registries write it: every
// sentence, the hash another language records of it, and where it stands.
func registryReflection(t *testing.T) []byte {
	t.Helper()
	entries := map[string]map[string]string{}
	for key, w := range registryWords(t) {
		entries[key] = map[string]string{"description": w.Where, "hash": englishHash(w.English), "other": w.English}
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(entries); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// TestTheRegistryCatalogueSaysWhatTheRegistriesSay keeps the English copy a
// translator works from equal to the registries, the way en.json is kept equal
// to the code. It is also where the hashes every other language is held to come
// from, so a stale copy would hold them to last month's English.
func TestTheRegistryCatalogueSaysWhatTheRegistriesSay(t *testing.T) {
	want := registryReflection(t)
	path := filepath.Join(registryDir, "en.json")
	if os.Getenv(writeRegistryCatalogue) == "1" {
		if err := os.MkdirAll(registryDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, want, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s, %d B", path, len(want))
		return
	}
	have, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("there is no %s: %v\nRun: %s=1 go test ./internal/guard/ -run ^TestTheRegistryCatalogueSaysWhatTheRegistriesSay$",
			path, err, writeRegistryCatalogue)
	}
	if !bytes.Equal(bytes.ReplaceAll(have, []byte("\r\n"), []byte("\n")), want) {
		t.Errorf("%s is not what the registries say - a sentence, a setting or a preset changed.\n"+
			"Run: %s=1 go test ./internal/guard/ -run ^TestTheRegistryCatalogueSaysWhatTheRegistriesSay$\n"+
			"and then bring every other language in %s up to it.", path, writeRegistryCatalogue, registryDir)
	}
}

// registryFiles is every language's file of registry words, by tag.
func registryFiles(t *testing.T) map[string]map[string]map[string]string {
	t.Helper()
	return catalogueFilesIn(t, registryDir)
}

// TestEveryLanguageSaysEverythingTheRegistriesSay is the registries' half of
// TestEveryLanguageSaysEverythingTheWindowSays, with the one thing that half
// does not need: a translation that has gone stale.
//
// A key made of identifiers cannot tell that the English under it changed. A
// Polish sentence describing a default the registry no longer has is worse
// than an English one, because it looks translated - so every entry records the
// hash of the English it translated, and a different hash is a failure that
// shows the English as it is now.
func TestEveryLanguageSaysEverythingTheRegistriesSay(t *testing.T) {
	words := registryWords(t)
	files := registryFiles(t)
	for tag := range catalogueFiles(t) {
		if _, has := files[tag]; !has {
			t.Errorf("the window speaks %s and %s has no %s.json, so its settings and presets are English there",
				tag, registryDir, tag)
		}
	}
	if len(files) < 2 {
		t.Fatal("no language but English has registry words, so this guard asserts about nothing")
	}
	for tag, entries := range files {
		if tag == text.English {
			continue
		}
		languageSaysTheRegistries(t, tag, entries, words)
	}
}

// languageSaysTheRegistries holds one language to every registry sentence.
func languageSaysTheRegistries(t *testing.T, tag string, entries map[string]map[string]string, words map[string]text.RegistryWord) {
	t.Helper()
	keys := make([]string, 0, len(words))
	for key := range words {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		w := words[key]
		entry, has := entries[key]
		switch {
		case !has || entry["other"] == "":
			t.Errorf("%s has no %s, so the window says it in English: %q", tag, key, w.English)
		case entry["hash"] != englishHash(w.English):
			t.Errorf("%s translates %s from an English the registry no longer says. It says now:\n  %q\n"+
				"Translate it again and record hash %s.", tag, key, w.English, englishHash(w.English))
		case strings.Contains(entry["other"], "{{"):
			t.Errorf("%s puts a value into %s, which is handed none: %q", tag, key, entry["other"])
		}
	}
	for key := range entries {
		if _, known := words[key]; !known {
			t.Errorf("%s carries %s, which no registry says any more", tag, key)
		}
	}
}

// TestOneEnglishSentenceIsOneSentenceInEveryLanguage holds the translations of
// one English sentence to agreeing. A width is declared by four formats with
// one sentence, the format of a set by every preset that reads it, and a window
// describing the same setting in two ways from one screen to the next is the
// kind of difference nobody reports and everybody notices.
func TestOneEnglishSentenceIsOneSentenceInEveryLanguage(t *testing.T) {
	words := registryWords(t)
	for tag, entries := range registryFiles(t) {
		if tag == text.English {
			continue
		}
		said := map[string]string{}
		first := map[string]string{}
		keys := make([]string, 0, len(words))
		for key := range words {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			english, translated := words[key].English, entries[key]["other"]
			if was, seen := said[english]; seen && was != translated {
				t.Errorf("%s says %q two ways: %q under %s and %q under %s",
					tag, english, was, first[english], translated, key)
			}
			if _, seen := said[english]; !seen {
				said[english], first[english] = translated, key
			}
		}
	}
}

// TestTheWindowDescribesASettingAsTheCommandLineDoes holds the window's own
// wording of what a setting takes to the one tfg formats prints.
//
// The window cannot print Property.Allowed: it is one English sentence. So it
// puts the same sentence together out of its catalogue - and a second way of
// saying one thing is the start of two surfaces describing one setting
// differently (D1). In English the two have to agree to the letter, for every
// setting, every limit on two settings, and the sizes of a run.
func TestTheWindowDescribesASettingAsTheCommandLineDoes(t *testing.T) {
	if got := text.AllowedWholeNumber(); got != "whole number" {
		t.Fatalf("this process speaks another language (%q), so nothing here is English to compare", got)
	}
	checked := 0
	each := func(owner text.Owner, declared []format.Property) {
		for _, p := range declared {
			if got, want := parts.Allowed(p), p.Allowed(); got != want {
				t.Errorf("%s %s: the window says %q and tfg formats says %q", owner, p.Name, got, want)
			}
			checked++
		}
	}
	for _, d := range format.All() {
		each(text.FormatOwner(d.ID), d.Properties)
		for _, j := range d.JointLimits {
			if got, want := parts.JointLimit(text.FormatOwner(d.ID), j), j.Describe(); got != want {
				t.Errorf("%s: the window says %q and tfg formats says %q", d.ID, got, want)
			}
		}
	}
	for _, p := range preset.All() {
		each(text.PresetOwner(p.ID), append(append([]format.Property{}, p.Parameters...), p.Globals()...))
	}
	for _, d := range damage.All() {
		each(text.DamageOwner(d.ID), d.Parameters)
	}
	for _, n := range []int64{0, 1023, 1024, 1536, 10 << 20, 2516582400, 1 << 40} {
		if got, want := text.HumanBytes(n), core.HumanBytes(n); got != want {
			t.Errorf("%d bytes: the window says %q and the command line %q", n, got, want)
		}
	}
	if checked < 80 {
		t.Fatalf("only %d setting(s) were compared, so the registries were not filled here", checked)
	}
	t.Logf("%d settings described the same way on both surfaces", checked)
}

// TestThePresetsSayInTheWindowWhatTheySayOnTheSite holds the two Polish copies
// of a preset's question and of what it finds to one wording. The site had
// them first (web/content/<tag>/site.json), and the window keeps its own copy
// rather than reading the site's, so the site stays a renderer of its files and
// the window a program of its own - which is what makes this guard necessary.
func TestThePresetsSayInTheWindowWhatTheySayOnTheSite(t *testing.T) {
	compared := 0
	for tag, entries := range registryFiles(t) {
		if tag == text.English {
			continue
		}
		// A window language the site does not have is nothing to compare. Any
		// other failure to read is a comparison lost without a word, which
		// the count below only notices when EVERY language was lost.
		raw, err := os.ReadFile(filepath.Join(repoRoot(t), "web", "content", tag, "site.json"))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			t.Fatalf("the %s site file could not be read: %v", tag, err)
		}
		var site struct {
			Presets map[string]struct {
				Question string   `json:"question"`
				Catches  []string `json:"catches"`
			} `json:"presets"`
		}
		if err := json.Unmarshal(raw, &site); err != nil {
			t.Fatalf("the %s site file is not readable: %v", tag, err)
		}
		for _, p := range preset.All() {
			onSite := site.Presets[p.ID]
			said := []string{entries[text.QuestionKey(p.ID)]["other"]}
			want := append([]string{onSite.Question}, onSite.Catches...)
			for i := range p.Catches {
				said = append(said, entries[text.CatchKey(p.ID, i+1)]["other"])
			}
			if strings.Join(said, "\n") != strings.Join(want, "\n") {
				t.Errorf("%s %s: the window says\n  %s\nand the site says\n  %s",
					tag, p.ID, strings.Join(said, "\n  "), strings.Join(want, "\n  "))
			}
			compared++
		}
	}
	if compared == 0 {
		t.Fatal("no language has both a site and registry words, so this guard compared nothing")
	}
}

// registryPseudoChild puts the next guard in a process of its own, for the
// reason translation_test.go gives.
const registryPseudoChild = "TFG_REGISTRY_PSEUDO_CHILD"

// TestNoSentenceOfTheRegistriesReachesTheWindowUndisguised is what proves the
// window asks for the registries' sentences rather than drawing them.
//
// Every other guard here holds the files. None of them would notice a screen
// that put a format's Detail on the screen directly - the file would be
// complete and the Polish window would show English. So this opens the window
// in the pseudo language, which disguises every sentence that is looked up,
// chooses every format and every preset in turn, and fails on any registry
// sentence that arrives as it is written.
func TestNoSentenceOfTheRegistriesReachesTheWindowUndisguised(t *testing.T) {
	if os.Getenv(registryPseudoChild) == "1" {
		registryInPseudo(t)
		return
	}
	run := exec.Command(os.Args[0],
		"-test.run=^TestNoSentenceOfTheRegistriesReachesTheWindowUndisguised$", "-test.v")
	run.Env = append(os.Environ(), registryPseudoChild+"=1")
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("a sentence of the registries reached the window undisguised:\n%s", out)
	}
	if !strings.Contains(string(out), "PASS") {
		t.Fatalf("the child said nothing about passing, which means it never ran:\n%s", out)
	}
}

func registryInPseudo(t *testing.T) {
	t.Helper()
	// Sentences rather than every word: a label of one short word may be
	// made of letters the pseudo language leaves alone.
	var sentences []string
	for _, w := range text.RegistryWords() {
		if strings.Count(w.English, " ") >= 3 {
			sentences = append(sentences, w.English)
		}
	}
	if err := text.LoadBuiltIn(text.Pseudo); err != nil {
		t.Fatalf("the pseudo language would not load: %v", err)
	}
	host := newFakeHost(t)
	window.Open(host)
	undisguised := func(where, said string) {
		for _, s := range sentences {
			if strings.Contains(said, s) {
				t.Errorf("%s shows %q as the registry wrote it", where, s)
			}
		}
	}

	generate := selectTab(t, host.content, text.TabOneTarget())
	for _, id := range format.IDs() {
		chooseFormat(t, generate, id)
		undisguised("the single batch screen with "+id, saidOn(generate))
	}
	// Proof the walk reached the sentences at all: a setting's sentence waits
	// behind the button beside its name, and the disguised one is there.
	pdf, _ := format.Get("pdf")
	chooseFormat(t, generate, "pdf")
	want := text.SettingDetail(text.FormatOwner("pdf"), pdf.Properties[0].Name, pdf.Properties[0].Detail)
	if !strings.Contains(saidOn(generate), want) {
		t.Fatalf("the single batch screen with pdf does not say %q, so this walk reads nothing it should", want)
	}

	presets := selectTab(t, host.content, text.TabPresets())
	for _, id := range preset.IDs() {
		choose(t, presets, text.FieldPreset(), id)
		undisguised("the presets screen with "+id, saidOn(presets))
	}
}

// saidOn is everything a screen says: the words drawn on it and what waits
// behind every button that explains a field, which is where a setting's
// sentence goes (parts.alsoSaying).
func saidOn(o fyne.CanvasObject) string {
	var b strings.Builder
	b.WriteString(textIn(o))
	walk(o, func(obj fyne.CanvasObject) {
		if d, is := obj.(*parts.DetailButton); is {
			b.WriteString(d.Explanation())
			b.WriteString("\n")
		}
	})
	return b.String()
}

// registryPolishChild and registryCopyChild put the next two guards in
// processes of their own - each loads a catalogue, which changes the words for
// everything that runs after it.
const (
	registryPolishChild = "TFG_REGISTRY_POLISH_CHILD"
	registryCopyChild   = "TFG_REGISTRY_COPY_CHILD"
)

// runChild runs one guard again in a process of its own and fails with what it
// said, the way the translation guards do.
func runChild(t *testing.T, name, env string) {
	t.Helper()
	run := exec.Command(os.Args[0], "-test.run=^"+name+"$", "-test.v")
	run.Env = append(os.Environ(), env+"=1")
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("%s failed in its own process:\n%s", name, out)
	}
	if !strings.Contains(string(out), "PASS") {
		t.Fatalf("the child said nothing about passing, which means it never ran:\n%s", out)
	}
}

// TestThePolishWindowSaysTheRegistriesInPolish is the whole path through the
// files this build carries: the registry folder compiled in with the rest of
// the catalogue, read into the language chosen, and asked where the window
// asks. Every other guard here reads the files from the disk, which would pass
// on a build that embedded none of them.
func TestThePolishWindowSaysTheRegistriesInPolish(t *testing.T) {
	if os.Getenv(registryPolishChild) != "1" {
		runChild(t, "TestThePolishWindowSaysTheRegistriesInPolish", registryPolishChild)
		return
	}
	polish := registryFiles(t)["pl"]
	if err := text.LoadBuiltIn("pl"); err != nil || text.Speaking() != "pl" {
		t.Fatalf("the window would not speak Polish (%v, speaking %s)", err, text.Speaking())
	}
	for _, d := range format.All() {
		for _, p := range d.Properties {
			key := text.DetailKey(text.FormatOwner(d.ID), p.Name)
			if want := polish[key]["other"]; want != "" && text.SettingDetail(text.FormatOwner(d.ID), p.Name, p.Detail) != want {
				t.Errorf("%s: the Polish window says %q", key, text.SettingDetail(text.FormatOwner(d.ID), p.Name, p.Detail))
			}
		}
	}
	if got := text.HumanBytes(10 << 20); got != "10,0 MB" {
		t.Errorf("ten megabytes read %q in the Polish window", got)
	}
	// What a run says about a limit nobody gave comes back with the name of
	// the value it is about, which is the half of its key the preset knows.
	expanded, err := preset.Expand("size-boundaries", nil)
	if err != nil {
		t.Fatal(err)
	}
	about := 0
	for _, n := range expanded.Spoken() {
		if n.About == "" {
			continue
		}
		about++
		if got, want := text.PresetNote(expanded.Preset.ID, n.About, n.Said), polish[text.NoteKey(expanded.Preset.ID, n.About)]["other"]; got != want {
			t.Errorf("the note about %s reads %q in the Polish window, and %q was translated", n.About, got, want)
		}
	}
	if about == 0 {
		t.Error("size-boundaries run with no limit said nothing about the limit it made up, so no note could be found in Polish")
	}
}

// TestTheEnglishCopyOfTheRegistriesNeverAnswersForThem holds the English on
// screen to the registries themselves. registry/en.json is written for a
// translator, and a copy one commit behind the registries must not be able to
// put last month's sentence in an English window.
func TestTheEnglishCopyOfTheRegistriesNeverAnswersForThem(t *testing.T) {
	if os.Getenv(registryCopyChild) != "1" {
		runChild(t, "TestTheEnglishCopyOfTheRegistriesNeverAnswersForThem", registryCopyChild)
		return
	}
	made := fstest.MapFS{
		"locale/en.json": &fstest.MapFile{Data: []byte(`{"LanguageName": {"other": "English"}}`)},
		"locale/" + text.RegistryFolder + "/en.json": &fstest.MapFile{
			Data: []byte(`{"Detail.format/pdf.pages": {"other": "a sentence the registry no longer says"}}`)},
	}
	if err := text.Load(made, "locale"); err != nil {
		t.Fatalf("the catalogue would not load: %v", err)
	}
	if got := text.SettingDetail(text.FormatOwner("pdf"), "pages", "today's sentence"); got != "today's sentence" {
		t.Errorf("the English window said %q where the registry says today's sentence", got)
	}
}
