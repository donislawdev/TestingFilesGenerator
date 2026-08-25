package guard

import (
	"os"
	"path/filepath"
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
