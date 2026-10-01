package guard

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
	"github.com/donislawdev/TestingFilesGenerator/internal/site"
)

// TestTranslatedTextStaysOnTheTranslatedPages holds the boundary the owner set
// when the site was allowed into this repository on 2026-08-26.
//
// D9 says text in the repository is English, and the criterion is the place
// rather than the reader. The site extends that rule to the languages it is
// translated into and the extension is only as good as its border - so the
// border is machine checked here rather than remembered. A character above
// ASCII may stand in the text of a language that is not the root one, and in
// the pages rendered from it. Anywhere else it is either a translation that
// leaked or an English page somebody typed a curly quote into, and both are
// defects.
//
// One thing from a translation is allowed to appear on every page, and it is
// the list of languages in the header. A visitor reading English has to be
// shown the way to Deutsch and to the Japanese, in the names those languages
// call themselves - a menu that said "German" would not be found by the person
// it is for. So the list is cut out of a page before the page is read, and
// nothing else is: a language name anywhere else in English text is a leak.
//
// This was a check on Polish alone until 2026-10-01, when the site was
// translated into a further twenty languages at the owner's request. The
// border moved with them: it is read from the language files, so a twenty
// third language needs no edit here, and a language that is not listed has no
// directory the border would let its text into.
func TestTranslatedTextStaysOnTheTranslatedPages(t *testing.T) {
	root := webRoot(t)
	languageMenu := regexp.MustCompile(`(?s)<ul class="langlist">.*?</ul>`)
	// Relative to the web directory, with a trailing slash so that content/de
	// does not let content/deutsch in.
	var homes []string
	for _, l := range languagesOnDisk(t) {
		if l.Dir == "" {
			continue
		}
		homes = append(homes, "content/"+l.Code+"/", "public/"+l.Dir+"/")
	}
	text := map[string]bool{".html": true, ".json": true, ".css": true, ".xml": true, ".txt": true}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !text[strings.ToLower(filepath.Ext(p))] {
			return err
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return relErr
		}
		slashed := filepath.ToSlash(rel)
		translated := slices.ContainsFunc(homes, func(home string) bool { return strings.HasPrefix(slashed, home) })
		b, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		page := string(b)
		if !translated {
			page = languageMenu.ReplaceAllString(page, "")
		}
		for n, line := range strings.Split(page, "\n") {
			for col, r := range line {
				if r > 127 && !translated {
					t.Errorf("web/%s:%d:%d holds %q - only the pages of a translation may carry it", slashed, n+1, col+1, r)
					return nil
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the site: %v", err)
	}
}

// TestEveryPageSaysWhichLanguageItIsAndWhereItsTranslationsAre holds what a
// search engine needs to serve the right language to the right reader.
//
// The site is generated, so each page is consistent with the language file it
// came from. What nothing else asks is whether those files, taken together,
// say one coherent thing to a crawler. Every page has to name its own language
// and carry the full set of alternates, its own included, plus x-default - and
// the page an alternate points at has to point back. A pair that does not
// reciprocate is dropped by Google, and the page it was meant to connect goes
// on competing with its own translation. The same page may not reuse the title
// or the description of a sibling in its language either: two pages with one
// title are one result to a search engine, and a snippet it writes itself.
func TestEveryPageSaysWhichLanguageItIsAndWhereItsTranslationsAre(t *testing.T) {
	s := siteUnderTest(t)
	rendered, err := s.Render()
	if err != nil {
		t.Fatalf("rendering the site: %v", err)
	}
	var root site.Language
	byDir := map[string]site.Language{}
	for _, l := range s.Languages {
		if l.Dir == "" {
			root = l
			continue
		}
		byDir[l.Dir] = l
	}
	languageOf := func(file string) site.Language {
		first, _, _ := strings.Cut(file, "/")
		if l, ok := byDir[first]; ok {
			return l
		}
		return root
	}
	fileOf := func(address string) string {
		path := strings.Trim(strings.TrimPrefix(address, siteOrigin), "/")
		if path == "" {
			return "index.html"
		}
		return path + "/index.html"
	}

	htmlTag := regexp.MustCompile(`<html lang="([^"]+)"( dir="rtl")?>`)
	alternate := regexp.MustCompile(`<link rel="alternate" hreflang="([^"]+)" href="([^"]+)">`)
	canonical := regexp.MustCompile(`<link rel="canonical" href="([^"]+)">`)
	locale := regexp.MustCompile(`<meta property="og:locale" content="([^"]+)">`)
	title := regexp.MustCompile(`<title>([^<]*)</title>`)
	description := regexp.MustCompile(`<meta name="description" content="([^"]*)">`)

	pages := map[string]map[string]string{}
	canonicals := map[string]string{}
	seenTitle, seenDescription := map[string]string{}, map[string]string{}
	for file, body := range rendered {
		if filepath.Base(file) != "index.html" {
			continue
		}
		text := string(body)
		lang := languageOf(file)
		tag := htmlTag.FindStringSubmatch(text)
		if tag == nil || tag[1] != lang.Code || (tag[2] != "") != lang.RTL {
			t.Errorf("%s should open as lang=%q with dir=rtl %v and says %q", file, lang.Code, lang.RTL, tag)
		}
		if m := locale.FindStringSubmatch(text); m == nil || m[1] != lang.Locale {
			t.Errorf("%s should carry og:locale %q and says %q", file, lang.Locale, m)
		}
		if m := canonical.FindStringSubmatch(text); m == nil {
			t.Errorf("%s has no canonical address", file)
		} else {
			canonicals[file] = m[1]
		}
		found := map[string]string{}
		for _, m := range alternate.FindAllStringSubmatch(text, -1) {
			found[m[1]] = m[2]
		}
		pages[file] = found

		for _, l := range s.Languages {
			if _, ok := found[l.Code]; !ok {
				t.Errorf("%s does not name its %s translation, so a search engine is not told it exists", file, l.Code)
			}
		}
		if _, ok := found["x-default"]; !ok || len(found) != len(s.Languages)+1 {
			t.Errorf("%s names %d alternates, and the languages are %d plus x-default", file, len(found), len(s.Languages))
		}
		for _, field := range []struct {
			name    string
			pattern *regexp.Regexp
			seen    map[string]string
		}{{"title", title, seenTitle}, {"description", description, seenDescription}} {
			m := field.pattern.FindStringSubmatch(text)
			if m == nil {
				t.Errorf("%s has no %s", file, field.name)
				continue
			}
			key := lang.Code + "\x00" + m[1]
			if other, dup := field.seen[key]; dup {
				t.Errorf("%s and %s have one %s in %s: %q", file, other, field.name, lang.Code, m[1])
			}
			field.seen[key] = file
		}
	}
	for file, found := range pages {
		lang := languageOf(file)
		if found[lang.Code] != canonicals[file] {
			t.Errorf("%s names %q as its own %s address and its canonical is %q", file, found[lang.Code], lang.Code, canonicals[file])
		}
		for code, address := range found {
			if code == "x-default" {
				continue
			}
			other, ok := pages[fileOf(address)]
			if !ok {
				t.Errorf("%s names %s as its %s translation and nothing is published there", file, address, code)
				continue
			}
			if back := other[lang.Code]; back != canonicals[file] {
				t.Errorf("%s names %s as its %s translation, and that page points back to %q instead of %q", file, address, code, back, canonicals[file])
			}
		}
	}
}

// TestEveryLanguageDescribesEveryDamage holds the page about corrupt files to
// what the program can break a file with.
//
// The table of damages is made from the registry, so a damage added there is a
// row on every page - and a row needs a sentence in every language, or the page
// would show a blank where the explanation goes. Read the other way as well: a
// sentence about a damage the program no longer has is a description of
// something that cannot be asked for. In English the sentence is the one the
// program prints under the name, word for word, because those are one sentence
// in two places and nothing else compares them.
func TestEveryLanguageDescribesEveryDamage(t *testing.T) {
	facts := factsFromTheProgram(t)
	if len(facts.Damages) == 0 {
		t.Fatal("the program lists no damage, so this guard would pass against any language file")
	}
	for _, lang := range languagesOnDisk(t) {
		for _, d := range facts.Damages {
			said, ok := lang.Damages[d.ID]
			if !ok {
				t.Errorf("the damage %q has no sentence in %s, so its row would be blank", d.ID, lang.Code)
				continue
			}
			if lang.Code == "en" && said != damageDetail(t, d.ID) {
				t.Errorf("tfg damage describes %q as %q and the English page says %q", d.ID, damageDetail(t, d.ID), said)
			}
		}
		for id := range lang.Damages {
			if !slices.ContainsFunc(facts.Damages, func(d site.Damage) bool { return d.ID == id }) {
				t.Errorf("%s describes a damage %q that the program does not have", lang.Code, id)
			}
		}
	}
}

// damageDetail is the sentence the program prints under one damage.
func damageDetail(t *testing.T, id string) string {
	t.Helper()
	d, err := damage.Get(id)
	if err != nil {
		t.Fatalf("the program has no damage %q: %v", id, err)
	}
	return d.Detail
}
