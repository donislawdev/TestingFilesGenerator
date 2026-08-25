package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// Both binaries carry the format registrations, asked of the compiler.
//
// This exists because of a defect the whole suite was blind to, found on
// 2026-08-05 by running the built window and looking at it. The format menu was
// empty - "(Select one)" with nothing under it - and every guard was green,
// because the formats register themselves through a blank import and this test
// package writes that import for its own use. So the registry was full
// everywhere a test could see and empty in the binary somebody would be handed.
//
// No test that imports the code can catch that class, which is why this one
// asks "go list -deps" instead. It is the same technique as the network guard
// beside it and it answers the same shape of question: what actually goes into
// the binary, rather than what is true where the tests run.
//
// The failure it prevents is total rather than partial. A window with no
// formats offers nothing at all, and a command line with none refuses every
// run - and both would say something puzzling rather than obviously broken.
func TestBothBinariesCarryTheFormatRegistrations(t *testing.T) {
	const registrations = "github.com/donislawdev/TestingFilesGenerator/internal/format/all"

	// Relative to this package, because that is where the test runs from. And
	// the window is asked about with C support on, because that is the build
	// that has a window in it - with CGO off the binary is the stub that says
	// so, links neither the screens nor the formats, and is right not to.
	for _, target := range []string{"../../cmd/tfg", "../../cmd/tfg-gui"} {
		linked := linkedWithCGO(t, target)

		found := false
		for _, p := range linked {
			if p == registrations {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s does not link the format registrations, so its registry is empty and it can produce nothing.\n"+
				"Blank import %s from the package that asks the registry, the way internal/cli does.",
				target, registrations)
			continue
		}

		// And the formats themselves, not only the package that gathers them.
		// A registration package that stopped registering would still be linked.
		generators := 0
		for _, p := range linked {
			if strings.HasPrefix(p, "github.com/donislawdev/TestingFilesGenerator/internal/format/") &&
				!strings.HasSuffix(p, "/all") {
				generators++
			}
		}
		if generators < 13 {
			t.Errorf("%s links %d format package(s) and this build has thirteen formats", target, generators)
		}
		t.Logf("%s links the registrations and %d format package(s)", target, generators)
	}
}

// linkedWithCGO asks what a build with C support pulls in.
//
// Listing is not compiling, so no C compiler is needed here - the setting only
// decides which files the build constraints let in. Without it the window
// binary lists as the stub that has no window, and this guard would then be
// asking about a build nobody runs.
func linkedWithCGO(t *testing.T, target string) []string {
	t.Helper()
	cmd := exec.Command("go", "list", "-deps", target)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("go list is not available here: %v", err)
	}
	var pkgs []string
	for _, line := range strings.Split(string(out), "\n") {
		if p := strings.TrimSpace(line); p != "" {
			pkgs = append(pkgs, p)
		}
	}
	if len(pkgs) < 20 {
		t.Fatalf("go list returned %d packages for %s, which is too few to be the real dependency set",
			len(pkgs), target)
	}
	return pkgs
}

// The archive works its minimum out from another format, so that format has to
// be registered first - and what makes it first is the order of import paths.
//
// Go initialises packages in the order of their import paths, so the txt
// package runs before the zip package by rule. Two comments in the tree called
// that an accident until 2026-08-25, and an outside review of the whole tree
// built the opposite argument on it - that the language guarantees nothing
// here and this is a panic waiting for a toolchain change. Neither was right,
// and the fix the review offered would have been refused by the layer guard:
// internal/format/zip is allowed to import internal/format and the label
// drawer, and nothing else.
//
// What is really fragile is narrower, and it is a name rather than a
// toolchain: rename or move either package so that the entry format no longer
// sorts first, and zip panics at start with the window not yet on screen.
// Formats in this tree have been renamed before - csvfile, jsonfile, htmlfile,
// logfile, svgfile and xmlfile all carry a suffix to stay clear of the standard
// library - so this is a rename away rather than a theory.
//
// Asked of the constant rather than of a run, because at test time every
// package is initialised and any order would look right from in here.
func TestTheArchiveEntryFormatSortsBeforeTheArchive(t *testing.T) {
	const module = "github.com/donislawdev/TestingFilesGenerator/internal/format/"

	entry := entryFormatOfZip(t)
	archive := module + "zip"
	if entryPath := module + entry; entryPath >= archive {
		t.Errorf("%q does not sort before %q, so the archive can be initialised first and "+
			"panic while working out its own minimum. Go initialises packages in the order of "+
			"their import paths - see the specification, Program initialization",
			entryPath, archive)
	}

	// The entry format is really registered under that id, or the constant
	// names something nobody would find and the comparison above is about a
	// package that does not exist.
	if _, err := format.Get(entry); err != nil {
		t.Errorf("the archive says its entries are %q and no such format is registered: %v", entry, err)
	}
}

// entryFormatOfZip reads the id the archive builds its minimum from.
//
// From the source because the constant is unexported and this guard lives
// outside its package. Naming it here as well would be the second copy of a
// value, which is the shape this project spends its time removing.
func entryFormatOfZip(t *testing.T) string {
	t.Helper()
	source := readFile(t, filepath.Join(repoRoot(t), "internal", "format", "zip", "zip.go"))
	const marker = "defaultEntryFmt = "
	at := strings.Index(source, marker)
	if at < 0 {
		t.Fatalf("internal/format/zip/zip.go no longer declares %s, so this guard reads nothing", marker)
	}
	rest := source[at+len(marker):]
	open := strings.Index(rest, `"`)
	shut := strings.Index(rest[open+1:], `"`)
	if open < 0 || shut < 0 {
		t.Fatal("the default entry format is not a plain string constant any more")
	}
	return rest[open+1 : open+1+shut]
}
