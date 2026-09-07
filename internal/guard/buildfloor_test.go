package guard

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The oldest compiler this module admits is the one it is actually built with.
//
// The "go" line in go.mod is a FLOOR: the oldest compiler the module admits,
// and the only statement of the three below that the compiler itself enforces.
// A floor lower than what CI runs is a hole, and it is not a tidiness question
// here, because bytes are the contract. The comment above the go line records
// what moved when this project went to 1.27.0 on 2026-09-01: Go 1.27 changed
// compress/flate, so every format that puts bytes through deflate produces
// different ones - eleven of the fifty one pinned cases moved, and all seven of
// the pinned standard library paths.
//
// Measured on 2026-09-06 against a build of this tree, which is what made this
// worth a guard rather than a note. With the floor at 1.26.5 a build under Go
// 1.26 succeeded, answered "0.3.0-rc1" to "tfg version", wrote "0.3.0-rc1" into
// every manifest, and produced different bytes from the release of that name
// for png, docx and targz. Same source, same version number, different files -
// which is the exact failure D11 and the version number exist to make
// diagnosable. The diagnosis was already in place and works: manifest.tool.go
// records the toolchain, so the two runs read "go1.27.0" and "go1.26.8" and
// somebody who thinks to look can tell. Nothing made them look.
//
// THERE IS NO TOOLCHAIN DIRECTIVE TO COMPARE AGAINST, and that is a measurement
// rather than an oversight. Go refuses to build a module whose toolchain
// directive is not newer than its go directive - once the floor was raised to
// meet the pin, the build answered "updates to go.mod needed" until the
// directive came out. So the pin now lives in GO_VERSION in the workflows, and
// this holds the floor against every one of them.
//
// The remaining copies are the ones a person reads before they have the
// repository. README.md states the minimum in prose, CONTRIBUTING.md states it
// again since 2026-09-07, and prose is the copy that rots - so both are held
// here. Same arrangement as the build tags in
// TestTheInstallInstructionsCarryTheBuildTags and for the same reason: one file
// holds the fact, everything inside a checkout reads it, and the copies that
// have to live outside get a guard instead.
//
// Equality rather than "the floor is at least the pin", deliberately. Comparing
// versions means arithmetic on version strings, and arithmetic on version
// strings is precisely where this class of defect hides. Two strings either
// match or they do not, and that is checkable without a comparison anybody has
// to be right about.
func TestTheBuildFloorIsThePinnedToolchainAndTheDocumentsSaySo(t *testing.T) {
	root := repoRoot(t)
	floor := goModFloor(t, root)

	checked := 0
	for _, wf := range workflowFiles(t, root) {
		raw, err := os.ReadFile(wf)
		if err != nil {
			t.Fatalf("reading %s: %v", filepath.Base(wf), err)
		}
		pin := regexp.MustCompile(`(?m)^\s*GO_VERSION:\s*"?([0-9][0-9.]*)"?\s*$`)
		for _, m := range pin.FindAllStringSubmatch(string(raw), -1) {
			checked++
			if m[1] == floor {
				continue
			}
			t.Errorf("%s builds with Go %s and go.mod admits Go %s, so the compiler CI runs "+
				"is not the oldest one this module accepts.\n"+
				"What to do: make them the same number. A floor below what CI runs lets "+
				"somebody build a copy that answers the same version and produces "+
				"different bytes.", filepath.Base(wf), m[1], floor)
		}
	}
	if checked == 0 {
		t.Fatal("no workflow declared GO_VERSION, so this guard checked nothing against the " +
			"floor. That is where the pin lives now that go.mod has no toolchain directive.")
	}

	// Every document that states the requirement, not only the shop window.
	// CONTRIBUTING.md states it too since 2026-09-07, and a second copy of a
	// number is a second thing to keep in step.
	for _, name := range filesThatTellSomebodyHowToBuild {
		stated, at, text := minimumGoIn(t, root, name)
		if stated == floor {
			continue
		}
		t.Errorf("%s line %d tells somebody they need Go %s and go.mod admits Go %s:\n"+
			"  %s\n"+
			"What to do: make the sentence name %s. It is read before anybody has the "+
			"repository, so it is the one copy nothing else can correct.",
			name, at, stated, floor, strings.TrimSpace(text), floor)
	}
}

// goModFloor returns the version on the go directive.
func goModFloor(t *testing.T, root string) string {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("reading go.mod: %v", err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "go" {
			return fields[1]
		}
	}
	// Asserted rather than assumed. A guard that silently found no line would
	// be green on a go.mod that lost it, which is the failure this file is
	// about.
	t.Fatal(`go.mod has no "go" line, so the build floor could not be read and this guard ` +
		"checked nothing")
	return ""
}

// workflowFiles lists the CI definitions, which are where the pin lives.
func workflowFiles(t *testing.T, root string) []string {
	t.Helper()

	found, err := filepath.Glob(filepath.Join(root, ".github", "workflows", "*.yml"))
	if err != nil {
		t.Fatalf("listing the workflows: %v", err)
	}
	if len(found) == 0 {
		t.Fatal("no workflow files were found, so this guard could not read the pinned Go " +
			"version from any of them")
	}
	return found
}

// minimumGoIn returns the version a document names as the minimum, the line it
// sits on and that line's text.
//
// Matched on the sentence rather than on a position, because a position moves
// the first time somebody adds a paragraph above it.
func minimumGoIn(t *testing.T, root, name string) (string, int, string) {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}

	want := regexp.MustCompile(`Needs Go ([0-9][0-9.]*)`)
	found, at, text := "", 0, ""
	for i, line := range strings.Split(string(raw), "\n") {
		m := want.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		// A second sentence naming a version is two answers to one question,
		// and the guard would then check whichever came last.
		if found != "" {
			t.Fatalf("%s names a minimum Go version twice, on lines %d and %d, so "+
				"there is no single sentence to hold against go.mod", name, at, i+1)
		}
		found, at, text = m[1], i+1, line
	}
	if found == "" {
		t.Fatalf(`%s has no "Needs Go <version>" sentence, so this guard checked `+
			"nothing. It is a statement of the requirement somebody sees before "+
			"they clone.", name)
	}
	return found, at, text
}
