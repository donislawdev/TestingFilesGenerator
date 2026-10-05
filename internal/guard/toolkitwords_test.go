package guard

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// The graphics toolkit's own words - the menu of a box to type in, the dialogs
// choosing a folder or a file - in the language the window speaks rather than
// the system's (docs/OKNO-PO-POLSKU-2026-10-05.md section 3.7). The window
// hands the toolkit a file of words for every language it carries, so these
// guards hold the files to the toolkit this build links: a word the toolkit
// asks for that no file has is a word it says in its own language, which may
// be neither the window's nor English.

// toolkitDir is where every language keeps the toolkit's words.
var toolkitDir = filepath.Join(localeDir, text.ToolkitFolder)

// toolkitAsks is every word the pinned toolkit asks its catalogue for, read
// from its source - lang.L("...") and lang.X("...") with a literal - and from
// its own English file, with the English it says.
func toolkitAsks(t *testing.T) map[string]string {
	t.Helper()
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "fyne.io/fyne/v2").Output()
	if err != nil {
		t.Fatalf("finding the toolkit's source: %v", err)
	}
	dir := strings.TrimSpace(string(out))
	raw, err := os.ReadFile(filepath.Join(dir, "lang", "translations", "base.en.json"))
	if err != nil {
		t.Fatalf("reading the toolkit's English: %v", err)
	}
	var base map[string]json.RawMessage
	if err := json.Unmarshal(raw, &base); err != nil {
		t.Fatal(err)
	}
	asks := map[string]string{}
	for key, value := range base {
		asks[key] = englishOf(value)
	}
	literal := regexp.MustCompile(`lang\.[LX]\("((?:[^"\\]|\\.)*)"`)
	for _, sub := range []string{"widget", "dialog", "internal", "app", "container"} {
		_ = filepath.WalkDir(filepath.Join(dir, sub), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			for _, m := range literal.FindAllStringSubmatch(string(src), -1) {
				if _, known := asks[m[1]]; !known {
					asks[m[1]] = m[1]
				}
			}
			return nil
		})
	}
	if len(asks) < 20 {
		t.Fatalf("found %d word(s) the toolkit asks for, so the walk of its source found nothing", len(asks))
	}
	return asks
}

// englishOf is a word of the toolkit's English file, a string or {"other": ...}.
func englishOf(raw json.RawMessage) string {
	var plain string
	if json.Unmarshal(raw, &plain) == nil {
		return plain
	}
	var forms map[string]string
	_ = json.Unmarshal(raw, &forms)
	return forms["other"]
}

// TestTheToolkitIsGivenEveryWordItAsksFor holds the window's English file of
// the toolkit's words to the toolkit: every word it asks for is there, in the
// toolkit's own English. A new word arriving with a new version of the
// toolkit turns this red rather than reaching a window in the system's
// language.
func TestTheToolkitIsGivenEveryWordItAsksFor(t *testing.T) {
	asks := toolkitAsks(t)
	files := catalogueFilesIn(t, toolkitDir)
	english, ok := files[text.English]
	if !ok {
		t.Fatalf("%s has no en.json", toolkitDir)
	}
	keys := make([]string, 0, len(asks))
	for key := range asks {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		entry, has := english[key]
		switch {
		case !has:
			t.Errorf("the toolkit asks for %q and the window does not give it, so it says it in the language of the system", key)
		case entry["other"] != asks[key]:
			t.Errorf("the toolkit says %q as %q and the window's English copy says %q", key, asks[key], entry["other"])
		case entry["hash"] != englishHash(entry["other"]):
			t.Errorf("the English copy of %q carries hash %q, and its English hashes to %q", key, entry["hash"], englishHash(entry["other"]))
		}
	}
}

// TestEveryLanguageGivesTheToolkitEveryWord holds every language the window
// carries to every word of the toolkit, translated from the English it says
// now.
func TestEveryLanguageGivesTheToolkitEveryWord(t *testing.T) {
	files := catalogueFilesIn(t, toolkitDir)
	english := files[text.English]
	for tag := range catalogueFiles(t) {
		entries, has := files[tag]
		if !has {
			t.Errorf("the window speaks %s and %s has no %s.json, so the toolkit's menus and dialogs speak the system's language there", tag, toolkitDir, tag)
			continue
		}
		if tag == text.English {
			continue
		}
		for key, want := range english {
			entry, has := entries[key]
			switch {
			case !has || entry["other"] == "":
				t.Errorf("%s has no word for the toolkit's %q", tag, key)
			case entry["hash"] != want["hash"]:
				t.Errorf("%s translates the toolkit's %q from an English it no longer says (%q). Record hash %s.", tag, key, want["other"], want["hash"])
			}
		}
		for key := range entries {
			if _, known := english[key]; !known {
				t.Errorf("%s carries %q, which the toolkit is not given", tag, key)
			}
		}
	}
}
