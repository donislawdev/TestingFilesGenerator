package guard

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"unicode"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/font"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// The languages of the window - docs/USTAWIENIA-2026-09-29.md, and the
// owner's decision of that day that the window speaks the system's language
// when nobody chose one (docs/PRODUCT.md D9).

// The language the window speaks is decided the way the owner decided it: a
// choice first, then the system, then English - and matched rather than
// compared, because the system says pl-PL where the catalogue says pl.
func TestTheWindowSpeaksTheLanguageItWasToldTo(t *testing.T) {
	carried := []text.Language{{Tag: "en"}, {Tag: "pl"}, {Tag: "pt"}, {Tag: "zh"}}
	for _, c := range []struct {
		why             string
		saved, system   string
		speaks, missing string
	}{
		{"nothing chosen, a Polish system", "", "pl-PL", "pl", ""},
		{"nothing chosen, a British system", "", "en-GB", "en", ""},
		{"nothing chosen, a Brazilian system", "", "pt-BR", "pt", ""},
		{"nothing chosen, a system in a language the window lacks", "", "de-DE", "en", ""},
		{"nothing chosen, a system that says nothing", "", "", "en", ""},
		{"nothing chosen, the C locale", "", "C", "en", ""},
		// A person who reads traditional Chinese is not served by a catalogue
		// in the simplified script, and English is the better answer.
		{"nothing chosen, a Taiwanese system and simplified Chinese carried", "", "zh-TW", "en", ""},
		{"Polish chosen on a German system", "pl", "de-DE", "pl", ""},
		{"English chosen on a Polish system", "en", "pl-PL", "en", ""},
		{"a choice this build lacks, on a Polish system", "de", "pl-PL", "pl", "de"},
		{"a choice that is not a language at all", "not a tag!", "", "en", "not a tag!"},
	} {
		got := text.Resolve(c.saved, c.system, carried)
		if got.Tag != c.speaks || got.Missing != c.missing {
			t.Errorf("%s: the window speaks %q and says %q is missing, and it should speak %q and say %q",
				c.why, got.Tag, got.Missing, c.speaks, c.missing)
		}
	}
}

// catalogueFiles is every catalogue compiled into the window, by tag.
func catalogueFiles(t *testing.T) map[string]map[string]map[string]string {
	t.Helper()
	return catalogueFilesIn(t, localeDir)
}

// catalogueFilesIn is every catalogue file of one folder, by tag - the
// window's own, or the words of the registries beside them.
func catalogueFilesIn(t *testing.T, dir string) map[string]map[string]map[string]string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no catalogue in %s: %v", dir, err)
	}
	out := map[string]map[string]map[string]string{}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		var entries map[string]map[string]string
		if err := json.Unmarshal(raw, &entries); err != nil {
			t.Fatalf("%s is not a catalogue: %v", path, err)
		}
		out[strings.TrimSuffix(filepath.Base(path), ".json")] = entries
	}
	return out
}

var catalogueField = regexp.MustCompile(`\{\{\s*\.([A-Za-z0-9]+)\s*\}\}`)

// Every language says everything the window says, with the values it is
// handed, in every form its numbers need.
//
// A missing entry is not a crash - the English answers for it - and that is
// exactly why it needs a guard: a Polish window with one English button in it
// is a defect nobody sees in English. A missing plural form is the same shape
// one level down: go-i18n refuses the number, and the window falls back to the
// English sentence for "2 files" in the middle of a Polish screen.
func TestEveryLanguageSaysEverythingTheWindowSays(t *testing.T) {
	files := catalogueFiles(t)
	english, ok := files[text.English]
	if !ok {
		t.Fatal("there is no English catalogue to hold the others to")
	}
	if len(files) < 2 {
		t.Fatal("the build carries one language, so the list on the Preferences screen offers nothing to choose")
	}
	names := map[string]string{}
	for tag, entries := range files {
		name := entries["LanguageName"]["other"]
		if name == "" || (tag != text.English && name == english["LanguageName"]["other"]) {
			t.Errorf("%s.json does not name its own language, so the list would show it as %q", tag, name)
		}
		if other, taken := names[name]; taken {
			t.Errorf("%s.json and %s.json both call themselves %q, and the list could not tell them apart", tag, other, name)
		}
		names[name] = tag
		if tag == text.English {
			continue
		}
		for id, want := range english {
			entry, has := entries[id]
			if !has || entry["other"] == "" {
				t.Errorf("%s.json has no %s, so the window says it in English", tag, id)
				continue
			}
			given := map[string]bool{}
			for _, f := range catalogueField.FindAllStringSubmatch(want["other"]+want["one"], -1) {
				given[f[1]] = true
			}
			for form, sentence := range entry {
				for _, f := range catalogueField.FindAllStringSubmatch(sentence, -1) {
					if !given[f[1]] {
						t.Errorf("%s.json %s (%s) asks for {{.%s}}, which the window never hands it - it would show <no value>",
							tag, id, form, f[1])
					}
				}
			}
			if _, plural := want["one"]; plural {
				everyNumberHasItsForm(t, tag, id, entry)
			}
		}
		for id := range entries {
			if _, known := english[id]; !known {
				t.Errorf("%s.json carries %s, which the window no longer says", tag, id)
			}
		}
	}
}

// everyNumberHasItsForm asks go-i18n for the entry at every count up to 125,
// which covers every rule CLDR has for the languages this window may speak.
func everyNumberHasItsForm(t *testing.T, tag, id string, entry map[string]string) {
	t.Helper()
	raw, err := json.Marshal(map[string]map[string]string{id: entry})
	if err != nil {
		t.Fatal(err)
	}
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	if _, err := bundle.ParseMessageFileBytes(raw, tag+".json"); err != nil {
		t.Errorf("%s.json %s does not load: %v", tag, id, err)
		return
	}
	localizer := i18n.NewLocalizer(bundle, tag)
	for n := 0; n <= 125; n++ {
		_, err := localizer.Localize(&i18n.LocalizeConfig{
			MessageID: id, PluralCount: n, TemplateData: map[string]any{"Count": n},
		})
		if err != nil {
			t.Errorf("%s.json %s cannot say %d: %v", tag, id, n, err)
			return
		}
	}
}

// Every letter every language says is in the typeface the window ships.
//
// A letter the face lacks is drawn from whatever the system has, or as an
// empty box - so a language whose script the window's own face does not carry
// is a decision about the typeface, and it should be taken as one rather than
// discovered on somebody's screen. Measured on 2026-09-29: Inter carries the
// Latin, Cyrillic and Greek scripts and none of Chinese, Japanese, Korean,
// Arabic, Hebrew or Devanagari (docs/USTAWIENIA-2026-09-29.md section 1.1).
func TestEveryLetterEveryLanguageSaysIsInTheTypeface(t *testing.T) {
	faces := map[string]map[rune]bool{
		"Inter-Regular": charactersIn(t, "Inter-Regular", font.Regular),
		"Inter-Bold":    charactersIn(t, "Inter-Bold", font.Bold),
	}
	languages := catalogueFiles(t)
	// The words of the registries, under the same tag with a mark saying where
	// they came from - they reach the same screens in the same face.
	for tag, entries := range registryFiles(t) {
		languages[tag+" "+text.RegistryFolder] = entries
	}
	for tag, entries := range languages {
		missing := map[string][]string{}
		for id, entry := range entries {
			for form, sentence := range entry {
				if form == "description" {
					continue
				}
				for _, r := range sentence {
					if unicode.IsSpace(r) {
						continue
					}
					for face, carries := range faces {
						if !carries[r] {
							missing[face] = append(missing[face], string(r)+" in "+id)
						}
					}
				}
			}
		}
		for face, where := range missing {
			sort.Strings(where)
			t.Errorf("%s has no glyph for what %s.json says: %s", face, tag, strings.Join(where, ", "))
		}
	}
}

// charactersIn reads the characters a TrueType face maps, from its format 12
// cmap subtable - the one covering the whole of Unicode, which both Inter
// files carry (fontTools 4.66, 2026-09-29: platform 3, encoding 10). Read here
// rather than through a font library, because the two in the module graph are
// there for the toolkit, and importing one from a test would change the graph
// the release gate asks about.
func charactersIn(t *testing.T, name string, face []byte) map[rune]bool {
	t.Helper()
	be := binary.BigEndian
	if len(face) < 12 {
		t.Fatalf("%s is too short to be a font", name)
	}
	tables := int(be.Uint16(face[4:]))
	cmap := -1
	for i := 0; i < tables; i++ {
		at := 12 + 16*i
		if at+16 > len(face) {
			break
		}
		if string(face[at:at+4]) == "cmap" {
			cmap = int(be.Uint32(face[at+8:]))
		}
	}
	if cmap < 0 || cmap+4 > len(face) {
		t.Fatalf("%s has no cmap table", name)
	}
	subtables := int(be.Uint16(face[cmap+2:]))
	for i := 0; i < subtables; i++ {
		at := cmap + 4 + 8*i
		platform, encoding := be.Uint16(face[at:]), be.Uint16(face[at+2:])
		if platform != 3 || encoding != 10 {
			continue
		}
		sub := cmap + int(be.Uint32(face[at+4:]))
		if be.Uint16(face[sub:]) != 12 {
			t.Fatalf("%s maps the whole of Unicode in format %d, not 12", name, be.Uint16(face[sub:]))
		}
		groups := int(be.Uint32(face[sub+12:]))
		out := map[rune]bool{}
		for g := 0; g < groups; g++ {
			at := sub + 16 + 12*g
			for r := be.Uint32(face[at:]); r <= be.Uint32(face[at+4:]); r++ {
				out[rune(r)] = true
			}
		}
		// In the state this guard is about: a face that maps its own Latin.
		if !out['A'] || !out['z'] || len(out) < 1000 {
			t.Fatalf("%s maps %d characters and not the Latin alphabet, so this reader is wrong", name, len(out))
		}
		return out
	}
	t.Fatalf("%s has no cmap subtable for the whole of Unicode", name)
	return nil
}

// pseudoChild puts the second half of the pseudo guard in its own process, for
// the reason translation_test.go gives.
const pseudoChild = "TFG_PSEUDO_CHILD"

// The pseudo language disguises every word the window says and keeps every
// value it is handed. It is a tool for seeing how a translation will sit on
// each screen before one exists, and a tool that changed a path it was shown
// would make the screens it draws lie about exactly the part a person reads
// most carefully.
func TestThePseudoLanguageDisguisesTheWordsAndKeepsTheValues(t *testing.T) {
	if os.Getenv(pseudoChild) == "1" {
		pseudoWindow(t)
		return
	}
	run := exec.Command(os.Args[0],
		"-test.run=^TestThePseudoLanguageDisguisesTheWordsAndKeepsTheValues$", "-test.v")
	run.Env = append(os.Environ(), pseudoChild+"=1")
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("the pseudo language did not hold:\n%s", out)
	}
	if !strings.Contains(string(out), "PASS") {
		t.Fatalf("the child said nothing about passing, which means it never ran:\n%s", out)
	}
}

func pseudoWindow(t *testing.T) {
	t.Helper()
	if err := text.LoadBuiltIn(text.Pseudo); err != nil {
		t.Fatalf("the pseudo language would not load: %v", err)
	}
	const where = `C:\some where\{{.Not}}`
	if got := text.WillGoTo(where); !strings.Contains(got, where) {
		t.Errorf("the pseudo language changed a value it was handed: %q became %q", where, got)
	}
	if got := text.ButtonGenerate(); got == "Generate" || !strings.HasPrefix(got, "[") {
		t.Errorf("the pseudo language left a word as it was: %q", got)
	}

	host := newFakeHost(t)
	window.Open(host)
	faces := charactersIn(t, "Inter-Regular", font.Regular)
	tabs := tabsIn(host.content)
	if tabs == nil {
		t.Fatal("the window has no tabs")
	}
	for _, item := range tabs.Items() {
		said := textIn(item.Content)
		if strings.Contains(said, "<no value>") || strings.Contains(said, "{{") {
			t.Errorf("the %s tab shows a value the pseudo language broke:\n%s", item.ID, said)
		}
		for _, r := range said {
			if !unicode.IsSpace(r) && !faces[r] {
				t.Errorf("the %s tab says %q in the pseudo language, which Inter has no glyph for", item.ID, r)
				break
			}
		}
	}
}
