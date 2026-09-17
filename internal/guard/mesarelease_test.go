package guard

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/legal"
)

// The software renderer reaches the Windows archive through a download the
// release workflow makes, pinned by version and sum in .github/mesa-dist-win.
// That file is the one thing the workflow can read. The registry in
// internal/legal is the one thing the notices and the bill of materials are
// rendered from. Two written copies of one fact, and these guards are what
// make that safe - the same shape as the notices held to the registry.

// rendererPin reads .github/mesa-dist-win: the version, the archive, its
// sum, and each file with its sum, exactly as the fetch script reads them.
func rendererPin(t *testing.T) (version, archive, sum string, files map[string]string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "mesa-dist-win"))
	if err != nil {
		t.Fatalf("no pin for the software renderer: %v", err)
	}
	files = map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		key, value, found := strings.Cut(strings.TrimSpace(line), "=")
		if !found {
			continue
		}
		switch key {
		case "version":
			version = value
		case "archive":
			archive = value
		case "sha256":
			sum = value
		case "file":
			path, fileSum, ok := strings.Cut(value, " ")
			if !ok {
				t.Fatalf("a file line of the pin has no sum: %q", line)
			}
			files[path] = fileSum
		default:
			t.Fatalf("the pin carries a key the fetch script does not read: %q", key)
		}
	}
	return version, archive, sum, files
}

// The pin the workflow downloads from and the registry the notices are
// rendered from name the same release, the same archive, the same sums and
// the same files. A bump in one without the other is refused here, before
// anything is built with it.
func TestTheRendererPinAgreesWithTheRegistry(t *testing.T) {
	version, archive, sum, files := rendererPin(t)
	companions := legal.Companions()
	if len(companions) != 1 {
		t.Fatalf("the registry holds %d companion(s) and the pin describes one", len(companions))
	}
	c := companions[0]
	if version != c.Source.Version || archive != c.Source.Archive || sum != c.Source.SHA256 {
		t.Errorf(".github/mesa-dist-win pins %s %s %s and the registry says %s %s %s.\n"+
			"The workflow downloads what the pin says and the notices describe what the registry says,\n"+
			"so the two have to be one release.", version, archive, sum, c.Source.Version, c.Source.Archive, c.Source.SHA256)
	}
	if len(files) != len(c.Files) {
		t.Errorf("the pin names %d file(s) and the registry %d", len(files), len(c.Files))
	}
	for _, f := range c.Files {
		inside := c.Source.Inside + filepath.Base(f.Path)
		if got, ok := files[inside]; !ok {
			t.Errorf("the registry carries %s, taken from %s inside the archive, and the pin names no such file", f.Path, inside)
		} else if got != f.SHA256 {
			t.Errorf("the pin says %s is %s and the registry says %s", inside, got, f.SHA256)
		}
	}
}

// The fetch script reads the pin and nothing else, checks the archive's sum
// before it unpacks anything, then checks each file's sum, and takes the
// files from the release the pin names. Read as text, in the order the
// lines come: a check after the unpack is a check of what has already been
// put beside the program.
func TestTheFetchScriptChecksTheSumBeforeItUnpacks(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "scripts", "fetch_software_renderer.sh"))
	if err != nil {
		t.Fatalf("no fetch script: %v", err)
	}
	script := string(raw)
	for what, want := range map[string]string{
		"it reads the pin": `pin=".github/mesa-dist-win"`,
		"it downloads from the project's own releases": "github.com/pal1000/mesa-dist-win/releases/download/${version}/${archive}",
		"it stops on the first failure":                "set -euo pipefail",
		"it refuses a download it cannot sum":          "sha256sum -c -",
		"it counts what it put beside the program":     `if [ "${count}" != "2" ]`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("%s: the fetch script does not contain %q", what, want)
		}
	}
	archiveSum := strings.Index(script, `echo "${sha256}  ${archive}" | sha256sum -c -`)
	unpack := strings.Index(script, "7z e ")
	fileSum := strings.LastIndex(script, "sha256sum -c -")
	if archiveSum < 0 || unpack < 0 || !(archiveSum < unpack && unpack < fileSum) {
		t.Errorf("the fetch script has to check the archive's sum (at %d), then unpack (at %d), then check "+
			"each file's sum (at %d) - in that order, or something unreviewed is beside the program before "+
			"anything says so.", archiveSum, unpack, fileSum)
	}
	// No copy of the pin in the script: a version or a sum written here as
	// well would be the second copy this whole file exists to prevent.
	if regexp.MustCompile(`[0-9a-f]{64}`).MatchString(script) {
		t.Error("the fetch script carries a sum of its own, and the pin is the only place a sum lives")
	}
	if strings.Contains(script, legal.Companions()[0].Source.Version) {
		t.Error("the fetch script carries a version of its own, and the pin is the only place the version lives")
	}
}

// Both workflows that build the window put the renderer beside it, on
// Windows only, after the program is built and before the archive is
// packed. The one that builds from a branch as well, by the owner's
// decision: a build from a branch has to be the build a guest without a
// driver can be handed.
func TestEveryWindowBuildFetchesTheRendererBeforeItPacks(t *testing.T) {
	// The call at the start of a line, after indentation and nothing else.
	// The first version of this guard asked whether the text was anywhere
	// in the file, and the mutation runner answered with the call commented
	// out: found, green, and no renderer in the archive.
	call := regexp.MustCompile(`(?m)^[ \t]*\.github/scripts/fetch_software_renderer\.sh "\$\{work\}"[ \t]*$`)
	for _, name := range []string{"release.yml", "dev-build.yml"} {
		text := workflowText(t, name)
		build := strings.Index(text, `-o "${work}/tfg-gui.exe" ./cmd/tfg-gui`)
		fetch := -1
		if at := call.FindStringIndex(text); at != nil {
			fetch = at[0]
		}
		pack := strings.Index(text, "7z a -tzip")
		if build < 0 || fetch < 0 || pack < 0 {
			t.Errorf("%s: the window build (%d), the fetch (%d) or the packing (%d) is missing", name, build, fetch, pack)
			continue
		}
		if !(build < fetch && fetch < pack) {
			t.Errorf("%s fetches the renderer at %d, builds the window at %d and packs at %d - the fetch "+
				"goes between the two, into the directory the archive is packed from.", name, fetch, build, pack)
		}
		// Windows only, read from the lines just above the call.
		before := text[:fetch]
		guard := strings.LastIndex(before, `if [ "$os" = "windows" ]; then`)
		if guard < 0 || strings.Contains(text[guard:fetch], "fi\n") {
			t.Errorf("%s calls the fetch script outside an \"os = windows\" condition, and nothing of the "+
				"kind ships for Linux or macOS.", name)
		}
	}
}

// The signing script signs the libraries beside the program and puts the
// archive back together whole.
//
// Measured on 2026-09-17 on an archive shaped like the window's: the repack
// as it stood, from os.listdir, came out holding "opengl/" and nothing under
// it - a valid archive, no error, and no renderer. That is the quietest
// defect the script could have had, and it had it from the day it was
// written, waiting for the first file in a subdirectory.
func TestTheSigningRepacksWholeAndSignsTheLibraries(t *testing.T) {
	script := signingScript(t)
	for what, want := range map[string]string{
		"it walks every directory when it repacks":      "os.walk(work)",
		"it signs the libraries beside the program":     `n.endswith(".dll")`,
		"it counts the repacked files against the held": "if repacked != inside:",
		"it verifies each signed file's certificate":    "actual = certificate_of(target)",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("%s: sign_release.py does not contain %q", what, want)
		}
	}
	if strings.Contains(script, "for name in sorted(os.listdir(work)):") {
		t.Error("sign_release.py still repacks from os.listdir, which names a directory and none of its contents")
	}
}

// The published Windows archive of the window is checked for the renderer
// and every library in every Windows archive for our signature - on the
// bytes a person downloads, by the workflow that runs after publication.
// The notes say the renderer is there and why.
func TestThePublishedWindowArchiveIsCheckedForTheRenderer(t *testing.T) {
	verify := workflowText(t, "verify-release.yml")
	for what, want := range map[string]string{
		"it reads the renderer's files from the registry": "companions.go",
		"it checks every library, not only the program":   "-Include *.exe, *.dll",
		"it asks the window's archive for the renderer":   "tfg-gui_*_windows_amd64.zip",
		"it counts what it checked":                       "if ($checked -lt 3)",
	} {
		if !strings.Contains(verify, want) {
			t.Errorf("%s: verify-release.yml does not contain %q", what, want)
		}
	}

	release := workflowText(t, "release.yml")
	for what, want := range map[string]string{
		"the notes say the renderer is in the archive": "software OpenGL renderer",
		"the notes say where it is":                    "next to the program",
		"the notes say when it is used":                "offers no OpenGL 2.1",
	} {
		if !strings.Contains(release, want) {
			t.Errorf("%s: the release notes in release.yml do not contain %q", what, want)
		}
	}
}
