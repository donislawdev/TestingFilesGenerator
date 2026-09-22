package guard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The site's own words never count the formats themselves.
//
// internal/site/view.go runs every piece of language text through the facts, so
// a string may say {{ .Facts.FormatCount }} and mean it. That mechanism exists
// because of a measured defect: a page title read "PDF, DOCX, PNG, ZIP and 16
// more formats" and the sixteen was typed.
//
// It happened again. On 2026-09-22, while two formats were being added, the
// English pages still described the tool as producing "real files of twenty
// formats" and the formats page still said "All twenty open in the software
// that owns them" - in the description, the Open Graph card and the schema
// metadata. The Polish schema said "w dwudziestu formatach". They had been
// wrong since the twenty first format arrived and nothing had noticed, because
// TestTheSiteSaysWhatTheToolSays compares the published pages against what the
// program renders NOW - and the program rendered the same stale sentence.
//
// So the mechanism was never the missing piece. What was missing is something
// that notices prose going around it, which is this.
func TestTheSiteNeverCountsTheFormatsInItsOwnWords(t *testing.T) {
	const escape = "{{ .Facts.FormatCount }}"

	// Two rules, because the two ways this has gone wrong do not look alike.
	//
	// A digit has to stand next to the word, which is what keeps "tfg generate
	// --format png --size 2mb" and "generating ten thousand files" out of it.
	// That shape is the 2026 title, "16 more formats".
	digit := regexp.MustCompile(`(?i)\b\d+([- ]\w+)? (more )?(formats?|format\w+)\b`)

	// A written out number does not have to stand next to it, because the
	// sentence that went stale this time never says the word: "All twenty open
	// in the software that owns them". So any of these words in a value that
	// talks about formats at all is the error.
	//
	// The list is only numbers a format COUNT could plausibly be. "one" and
	// "ten" are left out deliberately - measured on this content, they appear
	// nine times in prose that means neither, and Polish "ten" is not a number
	// at all. A guard nobody can leave green is one somebody turns off.
	written := regexp.MustCompile(`(?i)\b(` + strings.Join([]string{
		"sixteen", "seventeen", "eighteen", "nineteen",
		"twenty", "thirty", "forty", "fifty", "sixty",
		"szesnastu", "dwadzieścia", "dwudziestu",
		"trzydzieści", "trzydziestu", "czterdziestu",
	}, "|") + `)\b`)
	mentionsFormats := regexp.MustCompile(`(?i)\bformat`)

	dir := filepath.Join(repoRoot(t), "web", "content")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading the site content: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("the site has no language directories - this guard would pass against anything")
	}

	checked := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(dir, e.Name(), "site.json")
		body, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("reading %s: %v", path, err)
			continue
		}
		var tree any
		if err := json.Unmarshal(body, &tree); err != nil {
			t.Errorf("%s is not readable as JSON: %v", path, err)
			continue
		}
		walkStrings(tree, func(where, value string) {
			checked++
			if strings.Contains(value, escape) {
				return
			}
			found := digit.FindString(value)
			if found == "" && mentionsFormats.MatchString(value) {
				found = written.FindString(value)
			}
			if found != "" {
				t.Errorf("%s/%s says %q, and that number is typed rather than counted. "+
					"Write %s instead - the site expands it from the registry, so it cannot "+
					"go stale the way this one did twice",
					e.Name(), where, found, escape)
			}
		})
	}
	if checked == 0 {
		t.Fatal("no string was read out of the site content - this guard would pass against anything")
	}
	t.Logf("%d site strings checked for a typed format count", checked)
}

// walkStrings visits every string in a decoded JSON tree, with the path it sits
// at, so a failure names the key rather than only the file.
func walkStrings(node any, visit func(where, value string)) {
	switch v := node.(type) {
	case string:
		visit("", v)
	case []any:
		for _, item := range v {
			walkStrings(item, visit)
		}
	case map[string]any:
		for key, item := range v {
			walkStrings(item, func(where, value string) {
				if where == "" {
					visit(key, value)
					return
				}
				visit(key+"."+where, value)
			})
		}
	}
}
