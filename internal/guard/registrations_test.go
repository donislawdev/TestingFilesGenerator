package guard

import (
	"os"
	"os/exec"
	"strings"
	"testing"
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
