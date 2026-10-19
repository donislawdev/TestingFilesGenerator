package guard

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// changelogCategories are the seven this project uses and the only seven.
//
// Six come from Keep a Changelog. Breaking is ours, added on 2026-08-18,
// because here a broken compatibility is a first class thing: a major version
// is a statement about file hashes in somebody else's CI, so "the bytes of a
// PDF changed" filed under Changed beside the rest reads as a detail when it
// is the one line a reader is looking for. See docs/GIT.md.
var changelogCategories = map[string]bool{
	"Added": true, "Changed": true, "Deprecated": true,
	"Removed": true, "Fixed": true, "Security": true, "Breaking": true,
}

// The changelog has one heading per kind of change in each release, and only
// the seven kinds this project uses.
//
// Found by reading on 2026-08-25: the unreleased section had grown two separate
// "Changed" headings with entries under each. Nothing is lost that way and
// nothing renders wrongly, which is exactly why it survives - a reader looking
// for what changed finds a list, reads it, and never learns there is a second
// one further down. The same drift had put a blank line through the middle of
// the Added list, so half of it rendered with paragraph spacing and half
// without.
//
// Only the file that ships. CHANGELOG-DEV.md is outside the repository, so a
// fresh clone does not have it, and it has the same fault in more places -
// that one is a job of its own rather than something to fix from inside a
// guard's failure message.
func TestTheChangelogHasOneHeadingPerKindOfChange(t *testing.T) {
	path := filepath.Join(repoRoot(t), "CHANGELOG.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the changelog: %v", err)
	}

	var version string
	seen := map[string]map[string]int{}
	var order []string
	inFence := false
	for i, line := range strings.Split(string(raw), "\n") {
		// A fenced block can hold anything, including a line that starts with
		// hashes, and a guard that reads those is reading an example rather
		// than the document.
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		switch {
		case strings.HasPrefix(line, "## "):
			version = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			if _, ok := seen[version]; !ok {
				seen[version] = map[string]int{}
				order = append(order, version)
			}
		case strings.HasPrefix(line, "### "):
			name := strings.TrimSpace(strings.TrimPrefix(line, "### "))
			if version == "" {
				t.Errorf("line %d: %q sits above every release heading, so it belongs to nothing", i+1, name)
				continue
			}
			if !changelogCategories[name] {
				t.Errorf("line %d: %q is not one of the seven kinds of change this project uses.\n"+
					"Reason: docs/GIT.md closes the list on purpose. A kind invented for one entry is a kind\n"+
					"nobody filters on, and the reason Breaking is separate at all is that it has to be findable.",
					i+1, name)
			}
			seen[version][name]++
		}
	}

	for _, v := range order {
		for name, count := range seen[v] {
			if count > 1 {
				t.Errorf("%s has %d %q headings.\n"+
					"Reason: a reader looking for what changed finds the first list, reads it, and never\n"+
					"learns there is a second one further down. Put the entries under one heading.",
					v, count, name)
			}
		}
	}

	// Unreleased first, because that is where somebody looks to find out what
	// is coming and what is not in their build yet.
	if len(order) == 0 {
		t.Fatal("the changelog has no release headings at all")
	}
	if order[0] != "[Unreleased]" {
		t.Errorf("the first release heading is %q and docs/GIT.md puts [Unreleased] at the top", order[0])
	}
}

// Every version in the changelog is linkable, and Unreleased compares from the
// newest one.
//
// What this defends against happened on 2026-09-03, closing the release of
// 0.3.0-rc1. A "## [0.3.0-rc1]" heading went in and the link definitions at the
// bottom were left as they were: the new version had none, so the heading
// rendered as literal square brackets, and [Unreleased] still compared from
// v0.2.0 - which means it showed the release's own changes as if they were
// still coming. Nothing went red. It was found by reading, and the reading
// happened to be looking for something else.
//
// Keep a Changelog is declared at the top of the file and in docs/GIT.md §3, so
// this is a convention the project has written down and nothing was holding it.
// The same class as O178, and closed on the same day.
//
// The oldest version points at its release tag rather than at a comparison,
// because there is nothing before it to compare from. That is why only the
// presence of a definition is required here, and only Unreleased has its target
// checked.
func TestEveryChangelogVersionHasItsLinkDefinition(t *testing.T) {
	path := filepath.Join(repoRoot(t), "CHANGELOG.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the changelog: %v", err)
	}

	var headings []string
	defined := map[string]string{}
	inFence := false
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if strings.HasPrefix(line, "## [") {
			if name, ok := bracketed(line[len("## "):]); ok {
				headings = append(headings, name)
			}
			continue
		}
		// A link definition is a bracketed name, a colon and a target, at the
		// left margin. Anywhere else on a line it is a link being used.
		if strings.HasPrefix(line, "[") {
			if name, ok := bracketed(line); ok {
				rest := strings.TrimPrefix(line[len(name)+2:], ":")
				if strings.HasPrefix(line[len(name)+2:], ":") {
					defined[name] = strings.TrimSpace(rest)
				}
			}
		}
	}

	if len(headings) == 0 {
		t.Fatal("the changelog has no version headings at all, so this guard is reading the wrong file")
	}

	for _, name := range headings {
		if _, ok := defined[name]; !ok {
			t.Errorf("the %q section has no link definition at the bottom of the changelog.\n"+
				"Reason: Keep a Changelog makes every version linkable, and this file declares that\n"+
				"format in its own header. Without the definition the heading renders as literal\n"+
				"square brackets and the reader has no way to the diff.\n"+
				"What to do: add a line like [%s]: <compare URL> beside the others at the bottom.",
				name, name)
		}
	}
	for name := range defined {
		if !slices.Contains(headings, name) {
			t.Errorf("the bottom of the changelog defines a link for %q and there is no such section.\n"+
				"Reason: a definition left behind after a section is renamed or removed points at a\n"+
				"comparison nobody can reach from the document.", name)
		}
	}

	// Unreleased compares from the newest released version. Getting this wrong
	// is worse than leaving it out, because the link works: it just shows a
	// released version's own changes as if they were still coming.
	if len(headings) < 2 {
		return
	}
	newest := headings[1]
	target, ok := defined["Unreleased"]
	if !ok {
		return // already reported above
	}
	want := "compare/v" + newest + "...HEAD"
	if !strings.HasSuffix(target, want) {
		t.Errorf("[Unreleased] compares from something other than the newest version.\n"+
			"Reason: the newest section is %q, so the link should end in %q, and it is %q.\n"+
			"This went wrong on 0.3.0-rc1: it still compared from v0.2.0, so everything the\n"+
			"release shipped showed up under Unreleased as if it were still coming.\n"+
			"What to do: point it at the tag for %s.",
			newest, want, target, newest)
	}
}

// changelogMostWords is how long one entry may be - the owner's decision of
// 2026-10-20, docs/GIT.md §3.
const changelogMostWords = 100

// changelogWordsFrom is the first release held to that ceiling. The sections
// before it were published longer and stay as they were published.
var changelogWordsFrom = []int{0, 5, 0}

// A changelog entry says what is new, how to use it and the one limit a reader
// has to know, in at most a hundred words.
//
// Measured before the owner set the ceiling on 2026-10-20: the 0.5.0 section
// had 40 entries averaging 141 words, the longest 499, and 23 of them over a
// hundred. What made them long was measurement - which reader does what, with
// versions, seconds and gigabytes - and most of it is in the description of the
// setting, which tfg formats and the window show, so the changelog said it
// twice. A word is what sits between spaces, the list marker left out.
//
// Unreleased is held too, so an entry is cut when it is written rather than on
// the day of a release.
func TestEveryChangelogEntryFitsInAHundredWords(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "CHANGELOG.md"))
	if err != nil {
		t.Fatalf("reading the changelog: %v", err)
	}

	first, held := 0, 0
	for _, e := range changelogEntries(string(raw)) {
		if e.section == "0.5.0" {
			first++
		}
		if !changelogHeld(e.section) {
			continue
		}
		held++
		if words := len(strings.Fields(e.text)) - 1; words > changelogMostWords {
			t.Errorf("CHANGELOG.md line %d, under %s: an entry of %d words, and the ceiling is %d.\n"+
				"Reason: docs/GIT.md §3, the owner's decision of 2026-10-20. An entry says what is new,\n"+
				"how to use it and the one limit a reader has to know. How each reader behaves, with\n"+
				"versions and timings, belongs in the description of the setting.\n"+
				"What to do: cut it to the news, the setting or command, and that limit.",
				e.line, e.section, words, changelogMostWords)
		}
	}

	// The guard has to reach the entries it holds. 0.5.0 is the first section
	// under the ceiling and stays in the file for good, so a reading that finds
	// none of its entries, or does not hold them, is reading the wrong thing.
	if first == 0 {
		t.Fatal("no entry found under 0.5.0, so this guard is not reading the changelog's entries")
	}
	if held < first {
		t.Fatalf("%d entries under 0.5.0 and only %d held to the ceiling, so 0.5.0 is not being held", first, held)
	}
}

// changelogEntry is one item of a changelog list, its lines joined.
type changelogEntry struct {
	section string
	line    int
	text    string
}

// changelogEntries reads every item of every list in the changelog, with the
// section it sits under. An indented line goes on the item before it, after a
// blank line as well, so a second paragraph counts with the first.
func changelogEntries(text string) []changelogEntry {
	var out []changelogEntry
	section, open, inFence := "", false, false
	for i, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			continue
		}
		switch {
		case inFence || strings.TrimSpace(line) == "":
		case strings.HasPrefix(line, "## "):
			section, _ = bracketed(line[len("## "):])
			open = false
		case strings.HasPrefix(line, "- "):
			out = append(out, changelogEntry{section: section, line: i + 1, text: line})
			open = true
		case open && strings.HasPrefix(line, " "):
			out[len(out)-1].text += " " + line
		default:
			open = false
		}
	}
	return out
}

// changelogHeld says whether a section is held to the ceiling: Unreleased, and
// every version from changelogWordsFrom on. A heading that does not read as a
// version is held as well, because skipping what it cannot read is how a guard
// goes quiet.
func changelogHeld(section string) bool {
	core, _, _ := strings.Cut(section, "-")
	parts := strings.Split(core, ".")
	if section == "Unreleased" || len(parts) != 3 {
		return true
	}
	version := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return true
		}
		version[i] = n
	}
	return slices.Compare(version, changelogWordsFrom) >= 0
}

// bracketed reads the name out of a line that opens with a square bracket, and
// says whether there was one to read.
func bracketed(line string) (string, bool) {
	if !strings.HasPrefix(line, "[") {
		return "", false
	}
	end := strings.Index(line, "]")
	if end < 2 {
		return "", false
	}
	return line[1:end], true
}
