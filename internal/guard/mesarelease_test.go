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

// activeLines is a shell script or a workflow with its comment lines taken
// out - every line whose first character that is not blank is a hash. The
// guards below read the result, so that an operation commented out is an
// operation gone: the mutation runner answered the first version of one of
// them with the call commented out, and the text was still in the file.
//
// Whole lines only, and that is the limit of what can be done without
// reading the shell: a hash after code starts a comment unless it sits in
// quotes, and the workflows are YAML whose run blocks are one string each,
// so there is no string boundary to stop at. What a guard over these looks
// for is quoted by nature - a URL handed to curl, a condition in brackets -
// and a rule that dropped string contents would drop the things asked
// about. A Python script has a reader of its own, activePython.
func activeLines(text string) string {
	var kept []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// activePython is a Python script with its comments and its docstrings
// taken out: a comment to the end of its line, a triple-quoted string
// whole, each replaced by nothing so that the lines stay where they were.
// Short string literals stay, on purpose. Measured with Python's own
// tokenizer on 2026-09-17: of the ten things the two guards over
// sign_release.py look for, five ARE string contents by nature - arguments
// handed to a subprocess, the PowerShell text the script runs, a constant -
// so a reader that dropped every literal would turn those guards red on the
// correct script. What this closes is the shape that script actually has,
// which activeLines left in: a docstring naming an operation (one of them
// names os.listdir, the very call the repack guard refuses). A short
// literal holding an operation is not caught here - the guards that look
// for CODE ask for the statement at the start of a line instead, see
// statementIn, and a literal does not start a line.
func activePython(text string) string {
	var out strings.Builder
	for i := 0; i < len(text); {
		rest := text[i:]
		switch {
		case rest[0] == '#':
			end := strings.IndexByte(rest, '\n')
			if end < 0 {
				end = len(rest)
			}
			i += end
		case strings.HasPrefix(rest, `"""`) || strings.HasPrefix(rest, `'''`):
			end := closingQuote(rest, 3, rest[:3], false)
			out.WriteString(strings.Repeat("\n", strings.Count(rest[:end], "\n")))
			i += end
		case rest[0] == '"' || rest[0] == '\'':
			end := closingQuote(rest, 1, rest[:1], true)
			out.WriteString(rest[:end])
			i += end
		default:
			out.WriteByte(rest[0])
			i++
		}
	}
	return out.String()
}

// closingQuote is the index just past the quote that closes a string open
// at the start of text, searched from at, with a backslash escaping the
// character after it. A short string stops at the end of its line, because
// Python does - and a string that never closes runs to the end of the text
// rather than being guessed at.
func closingQuote(text string, at int, quote string, shortString bool) int {
	for i := at; i < len(text); i++ {
		switch {
		case text[i] == '\\':
			i++
		case shortString && text[i] == '\n':
			return i
		case strings.HasPrefix(text[i:], quote):
			return i + len(quote)
		}
	}
	return len(text)
}

// statementIn reports whether a line of code begins with the statement,
// after indentation and before anything else. The statement is a regular
// expression. A string literal holding the same text does not begin a line
// - `note = "os.walk(work)"` begins with note - and a docstring is gone
// before this is asked, so what is left is the operation itself.
func statementIn(code, statement string) bool {
	return regexp.MustCompile(`(?m)^[ \t]*` + statement).MatchString(code)
}

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
	script := activeLines(string(raw))
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
		text := activeLines(workflowText(t, name))
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
	script := activePython(signingScript(t))
	// Each one a statement at the start of a line, not a text anywhere in
	// the file: an outside review of #109 pointed out that the text inside
	// a string literal satisfied the first version, and the script's own
	// docstrings name operations - see activePython.
	for what, want := range map[string]string{
		"it walks every directory when it repacks":      `for .+ in os\.walk\(work\):`,
		"it signs the libraries beside the program":     `signed = .+n\.endswith\("\.dll"\)`,
		"it counts the repacked files against the held": `if repacked != inside:`,
		"it verifies each signed file's certificate":    `actual = certificate_of\(target\)`,
	} {
		if !statementIn(script, want) {
			t.Errorf("%s: no line of sign_release.py begins with %s", what, want)
		}
	}
	if statementIn(script, `for name in sorted\(os\.listdir\(work\)\):`) {
		t.Error("sign_release.py still repacks from os.listdir, which names a directory and none of its contents")
	}
}

// The reader of the signing script drops what is not code and keeps what
// is, in the shapes the script has: a comment after code, a docstring across
// lines, a short literal that holds a hash or a quote. Each shape is pressed
// on its own, because the reader that gets one of them wrong is green on the
// real script all the same.
func TestTheSigningScriptReaderKeepsCodeAndDropsProse(t *testing.T) {
	for _, c := range []struct{ label, in, want string }{
		{"a comment after code", "x = 1  # os.walk(work)\ny = 2\n", "x = 1  \ny = 2\n"},
		{"a docstring across lines", "def f():\n    \"\"\"walks with\n    os.walk(work)\n    \"\"\"\n    return 1\n", "def f():\n    \n\n\n    return 1\n"},
		{"a docstring in single quotes", "'''os.walk(work)'''\nz = 3\n", "\nz = 3\n"},
		{"a short literal with a hash in it", "note = \"a # b\"\n", "note = \"a # b\"\n"},
		{"a short literal with the other quote in it", "note = 'say \"hi\"'\nq = 1\n", "note = 'say \"hi\"'\nq = 1\n"},
		{"an escaped quote inside a literal", "note = \"a \\\" b\"  # c\n", "note = \"a \\\" b\"  \n"},
		{"triple quotes inside a short literal", "note = \"'''\"\nq = 1\n", "note = \"'''\"\nq = 1\n"},
		{"a short literal that never closes stops at its line", "note = \"open\nq = 1\n", "note = \"open\nq = 1\n"},
	} {
		if got := activePython(c.in); got != c.want {
			t.Errorf("%s: activePython(%q) = %q, want %q", c.label, c.in, got, c.want)
		}
	}
	// And the statement check reads the result the way the guards do.
	code := activePython("note = \"os.walk(work)\"\n\"\"\"\nfor x in os.walk(work):\n\"\"\"\n    for base, names in os.walk(work):  # here\n")
	if !statementIn(code, `for .+ in os\.walk\(work\):`) {
		t.Errorf("the statement is there and was not found in %q", code)
	}
	if statementIn(activePython("note = \"for x in os.walk(work):\"\n"), `for .+ in os\.walk\(work\):`) {
		t.Error("a literal holding the statement counted as the statement")
	}
	if statementIn(activePython("\"\"\"\nfor x in os.walk(work):\n\"\"\"\n"), `for .+ in os\.walk\(work\):`) {
		t.Error("a docstring holding the statement counted as the statement")
	}
	if statementIn(activePython("# for x in os.walk(work):\n"), `for .+ in os\.walk\(work\):`) {
		t.Error("a comment holding the statement counted as the statement")
	}
}

// The published Windows archive of the window is checked for the renderer
// and every library in every Windows archive for our signature - on the
// bytes a person downloads, by the workflow that runs after publication.
// The notes say the renderer is there and why.
func TestThePublishedWindowArchiveIsCheckedForTheRenderer(t *testing.T) {
	verify := activeLines(workflowText(t, "verify-release.yml"))
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

	release := activeLines(workflowText(t, "release.yml"))
	// When it is used is the condition the program tests - no window from
	// the first attempt - and not the cause a person would name for it. The
	// first version of the notes said "when the graphics driver offers no
	// OpenGL 2.1", which is the usual reason and not the test the code
	// makes: an outside review of #109 pointed at opening.go, where the
	// second attempt follows any first attempt that left no window.
	for what, want := range map[string]string{
		"the notes say the renderer is in the archive": "software OpenGL renderer",
		"the notes say where it is":                    "next to the program",
		"the notes say when it is used":                "loads it only after its first attempt",
	} {
		if !strings.Contains(release, want) {
			t.Errorf("%s: the release notes in release.yml do not contain %q", what, want)
		}
	}
}
