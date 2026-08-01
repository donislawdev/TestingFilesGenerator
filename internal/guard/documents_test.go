package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Six numbering schemes carry the decisions of this project, and nothing was
// watching them.
//
// They collided once already and were pulled apart on 2026-08-01, which is why
// CLAUDE.md says to always write the prefix. What that fix did not buy is a
// check: a reference to a decision that does not exist reads exactly like one
// that does, and the documents live outside the repository, so no diff ever
// shows that one of them drifted.
//
// Numbers are handed out once and never recycled, so a hole in a range means
// one was removed and every document quoting it now points at nothing.

// home is where each prefix is defined. A number counts as defined when it
// appears in its own document - the six documents format their definitions
// differently, and a parser per format would be a guard that breaks whenever
// somebody reformats a table.
var home = map[string]string{
	"D":  "PRODUCT.md",
	"M":  "BACKLOG.md",
	"AR": "ARCHITECTURE.md",
	"RC": "RECIPE.md",
	"PR": "PRESETS.md",
	"MF": "MANIFEST.md",
}

// Longest first, so MF5 is not read as M followed by rubbish.
var identifier = regexp.MustCompile(`\b(AR|MF|RC|PR|D|M)([0-9]+)\b`)

// The summary table in CLAUDE.md, which is what a new session reads instead of
// the six documents.
var declaredRange = regexp.MustCompile("\\| `(AR|MF|RC|PR|D|M)([0-9]+)`[^`]*`(?:AR|MF|RC|PR|D|M)([0-9]+)`")

func TestEveryIdentifierAReferencePointsAtExists(t *testing.T) {
	root := repoRoot(t)
	docs := filepath.Join(root, "docs")

	if _, err := os.Stat(docs); err != nil {
		t.Logf("SKIPPED: docs/ is not here, so nothing was compared. "+
			"The internal documents are excluded from the repository, so this check only runs on a machine that has them. (%v)", err)
		return
	}

	bodies := map[string]string{}
	entries, err := os.ReadDir(docs)
	if err != nil {
		t.Fatalf("reading docs/: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(docs, e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		bodies[e.Name()] = string(b)
	}
	claude, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err == nil {
		bodies["CLAUDE.md"] = string(claude)
	}
	if len(bodies) == 0 {
		t.Fatal("no document was read - this guard would pass without checking anything")
	}

	// What each document defines.
	defined := map[string]map[int]bool{}
	for prefix, file := range home {
		body, ok := bodies[file]
		if !ok {
			t.Errorf("%s defines the %s numbers and is not in docs/", file, prefix)
			continue
		}
		defined[prefix] = map[int]bool{}
		for _, m := range identifier.FindAllStringSubmatch(body, -1) {
			if m[1] != prefix {
				continue
			}
			n, err := strconv.Atoi(m[2])
			if err != nil {
				continue
			}
			defined[prefix][n] = true
		}
		if len(defined[prefix]) == 0 {
			t.Errorf("%s defines no %s number at all, so every reference to one is dangling", file, prefix)
		}
	}

	// A hole means a number was withdrawn, and withdrawing is exactly what the
	// convention forbids.
	for prefix, numbers := range defined {
		highest := 0
		for n := range numbers {
			if n > highest {
				highest = n
			}
		}
		var holes []string
		for n := 1; n <= highest; n++ {
			if !numbers[n] {
				holes = append(holes, fmt.Sprintf("%s%d", prefix, n))
			}
		}
		if len(holes) > 0 {
			t.Errorf("%s runs to %s and %s missing from %s - a number is handed out once and never withdrawn",
				prefix, prefix+strconv.Itoa(highest), strings.Join(holes, ", "), home[prefix])
		}
	}

	// Every reference anywhere has to land on something.
	dangling := 0
	names := make([]string, 0, len(bodies))
	for name := range bodies {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		seen := map[string]bool{}
		for _, m := range identifier.FindAllStringSubmatch(bodies[name], -1) {
			ref := m[1] + m[2]
			if seen[ref] {
				continue
			}
			seen[ref] = true
			numbers, known := defined[m[1]]
			if !known {
				continue
			}
			n, err := strconv.Atoi(m[2])
			if err != nil || numbers[n] {
				continue
			}
			t.Errorf("%s points at %s and %s defines no such number", name, ref, home[m[1]])
			dangling++
		}
	}

	// The table in CLAUDE.md is what a new session reads instead of the six
	// documents, so a range that stopped being true there is worse than no
	// table at all.
	for _, m := range declaredRange.FindAllStringSubmatch(bodies["CLAUDE.md"], -1) {
		prefix := m[1]
		numbers, known := defined[prefix]
		if !known {
			continue
		}
		first, _ := strconv.Atoi(m[2])
		last, _ := strconv.Atoi(m[3])
		highest := 0
		for n := range numbers {
			if n > highest {
				highest = n
			}
		}
		if first != 1 || last != highest {
			t.Errorf("CLAUDE.md announces %s%d-%s%d and %s defines %s1 to %s%d",
				prefix, first, prefix, last, home[prefix], prefix, prefix, highest)
		}
	}

	total := 0
	for prefix, numbers := range defined {
		total += len(numbers)
		t.Logf("%s: %d defined in %s", prefix, len(numbers), home[prefix])
	}
	t.Logf("%d identifiers across %d documents, %d dangling", total, len(bodies), dangling)
}

// A link to a document that is not there is the same defect as a reference to a
// decision that is not there, and it goes unnoticed for the same reason: the
// documents live outside the repository, so no diff shows that one of them was
// renamed.
var markdownLink = regexp.MustCompile(`\]\(([^)#]+?)(?:#[^)]*)?\)`)

func TestEveryLinkBetweenDocumentsLeadsSomewhere(t *testing.T) {
	root := repoRoot(t)
	docs := filepath.Join(root, "docs")

	if _, err := os.Stat(docs); err != nil {
		t.Logf("SKIPPED: docs/ is not here, so nothing was compared. "+
			"The internal documents are excluded from the repository, so this check only runs on a machine that has them. (%v)", err)
		return
	}

	// A document in docs/ links relative to docs/, and CLAUDE.md sits in the
	// root and links relative to the root.
	type source struct{ path, base string }
	var sources []source

	entries, err := os.ReadDir(docs)
	if err != nil {
		t.Fatalf("reading docs/: %v", err)
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			sources = append(sources, source{filepath.Join(docs, e.Name()), docs})
		}
	}
	if claude := filepath.Join(root, "CLAUDE.md"); fileExists(claude) {
		sources = append(sources, source{claude, root})
	}
	if len(sources) == 0 {
		t.Fatal("no document was read - this guard would pass without checking anything")
	}

	checked, dead := 0, 0
	for _, s := range sources {
		body, err := os.ReadFile(s.path)
		if err != nil {
			t.Fatalf("reading %s: %v", s.path, err)
		}
		for _, m := range markdownLink.FindAllStringSubmatch(string(body), -1) {
			target := strings.TrimSpace(m[1])
			if target == "" || strings.HasPrefix(target, "http") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			checked++
			if !fileExists(filepath.Join(s.base, filepath.FromSlash(target))) {
				t.Errorf("%s links to %s and there is nothing there", filepath.Base(s.path), target)
				dead++
			}
		}
	}

	if checked == 0 {
		t.Fatal("no link was examined - this guard would pass without checking anything")
	}
	t.Logf("%d links between documents, %d dead", checked, dead)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
