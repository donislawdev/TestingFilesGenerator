package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/legal"
)

// Somebody who downloaded one file can ask it what is inside it.
//
// Until 2026-08-28 both surfaces answered that question by pointing at
// THIRD-PARTY-NOTICES.md, which is true and useless to the person holding only
// the binary: the window is downloaded on its own and the file is not in it.
// Now the command prints the list and the about screen shows it.
//
// This guard asks the built binary rather than the package, because the whole
// point of the list is that it comes from the build. A test calling the
// function proves the function, and the function reads a record that only a
// real binary has - measured the same day: debug.ReadBuildInfo in a binary
// built by go test reports no dependencies at all.
func TestTheLicenceCommandListsWhatTheBinaryLinks(t *testing.T) {
	binary := buildCommandLine(t)

	out, err := exec.Command(binary, "license").Output()
	if err != nil {
		t.Fatalf("running the licence command: %v", err)
	}
	said := string(out)

	linked := linkedModules(t, "../../cmd/tfg")
	if len(linked) < 2 {
		t.Fatalf("the command line binary reports %d modules, too few to be the real set", len(linked))
	}
	for _, path := range linked {
		if !strings.Contains(said, path) {
			t.Errorf("the binary links %s and its licence command never names it.\n"+
				"The list is what somebody with only this file can read, so a module missing "+
				"from it is a notice that did not travel.", path)
		}
	}

	// The runtime, which no build reports and every binary contains. It is the
	// one entry that cannot come from a dependency list, so it is the one that
	// would go missing without anybody noticing.
	if !strings.Contains(said, "Go runtime and standard library") {
		t.Error("the licence command does not name the Go runtime, which every binary here links")
	}

	// And the versions, because a licence notice for a release nobody ships is
	// the defect this project found in its own notices file on the same day.
	for _, want := range []string{"go1.", "v"} {
		if !strings.Contains(said, want) {
			t.Errorf("the licence command names no version (looking for %q)", want)
		}
	}
}

// The window has to say the same, or the two surfaces disagree about what the
// program is made of - and the one somebody reads is whichever they happened to
// download. D1 says the surfaces do not drift, and a licence is the worst place
// to start.
func TestTheWindowShowsTheSameLicencesAsTheCommandDoes(t *testing.T) {
	shown := textIn(window.About(newFakeHost(t)))
	if len(legal.Reviewed()) < 20 {
		t.Fatalf("the registry holds %d entries, too few to be the real set", len(legal.Reviewed()))
	}
	for _, item := range legal.Reviewed() {
		if !strings.Contains(shown, item.Name) {
			t.Errorf("the about screen never names %q, which this binary carries", item.Name)
		}
		if !strings.Contains(shown, item.SPDX) {
			t.Errorf("the about screen names %q and not the licence it ships under", item.Name)
		}
	}
}

// An asset entry says which module brings it, and that answer is used to decide
// whether the thing ships in the binary being asked. A path typed by hand is a
// path that can be wrong, and wrong here means a font quietly dropping off a
// list rather than an error.
func TestEveryAssetEntryNamesTheModuleItComesFrom(t *testing.T) {
	for _, asset := range legal.Assets() {
		out, err := exec.Command("go", "list", "-f", "{{if .Module}}{{.Module.Path}}{{end}}", asset.Package).Output()
		if err != nil {
			t.Skipf("go list for %s is not available here: %v", asset.Package, err)
		}
		got := strings.TrimSpace(string(out))
		if got != asset.Module {
			t.Errorf("%s says it comes from %s and the package %s belongs to %s",
				asset.Name, asset.Module, asset.Package, got)
		}
	}
}

// buildCommandLine builds the real command line binary, the way a release does.
func buildCommandLine(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "tfg")
	if os.Getenv("GOOS") == "windows" || filepath.Separator == '\\' {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-trimpath", "-o", binary, "../../cmd/tfg")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Skipf("building the command line binary is not possible here: %v\n%s", err, out)
	}
	return binary
}

// linkedModules is what one target actually links, asked of the compiler.
func linkedModules(t *testing.T, target string) []string {
	t.Helper()
	cmd := exec.Command("go", "list", "-deps", "-f", "{{if .Module}}{{.Module.Path}}{{end}}", target)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("go list is not available here: %v", err)
	}
	seen := map[string]bool{}
	var paths []string
	for _, line := range strings.Split(string(out), "\n") {
		path := strings.TrimSpace(line)
		if path == "" || strings.Contains(path, "donislawdev") || seen[path] {
			continue
		}
		seen[path] = true
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}
