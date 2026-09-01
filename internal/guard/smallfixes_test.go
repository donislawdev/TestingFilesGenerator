package guard

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
)

// A size is written the way a person writes it, and nothing else.
//
// Measured on 2026-08-03: "--size 1e5" quietly meant 100000 bytes, while a
// recipe refuses "1_000" and "0x10" outright because guessing at a spelling is
// what that reader was written to stop doing. Two doors into one idea applying
// opposite rules, and the difference was nobody's decision - it fell out of
// ParseFloat being permissive about what counts as a number.
func TestASizeIsWrittenTheWayAPersonWritesIt(t *testing.T) {
	dir := t.TempDir()

	refused := []string{"1e5", "1E5", "0x10", "1_000", "Inf", "1e5kb"}
	for _, size := range refused {
		t.Run("refused "+size, func(t *testing.T) {
			code, _, errOut := run(t,
				"generate", "--format", "txt", "--size", size, "--dry-run", "--out", dir)
			if code == cli.ExitOK {
				t.Errorf("the size %q was accepted", size)
			}
			if strings.Contains(errOut, "goroutine") {
				t.Errorf("a stack trace reached the user:\n%s", errOut)
			}
		})
	}

	// The decimal point stays. 1.5gib is a real thing people write, and the
	// fix must not reach it.
	accepted := map[string]string{
		"1.5gib":  "1610612736",
		"10mb":    "10485760",
		"1048576": "1048576",
		"700kB":   "716800",
		"0":       "0",
	}
	for size, want := range accepted {
		t.Run("accepted "+size, func(t *testing.T) {
			code, _, errOut := run(t,
				"generate", "--format", "txt", "--size", size, "--dry-run", "--out", dir)
			if code != cli.ExitOK {
				t.Fatalf("the size %q was refused: exit %d\n%s", size, code, errOut)
			}
			if !strings.Contains(errOut, want+" B total") {
				t.Errorf("the size %q came to something other than %s B:\n%s", size, want, errOut)
			}
		})
	}
}

// The command recorded in the manifest exists to be run again. It was built by
// joining the arguments with spaces, so an argument holding one arrived as two
// - "--name my file.txt" reads as a name of "my" and a stray word - and the
// recorded command described a different run.
func TestTheRecordedCommandCanBeRunAgain(t *testing.T) {
	dir := t.TempDir()

	code, stdout, errOut := run(t,
		"generate", "--format", "txt", "--size", "1kb",
		"--name", "my file.txt", "--json", "--out", dir)
	if code != cli.ExitOK {
		t.Fatalf("exit %d:\n%s", code, errOut)
	}

	var m struct {
		Run struct {
			Command string `json:"command"`
		} `json:"run"`
	}
	if err := json.Unmarshal([]byte(stdout), &m); err != nil {
		t.Fatalf("the manifest did not parse: %v", err)
	}
	if !strings.Contains(m.Run.Command, `"my file.txt"`) {
		t.Errorf("the recorded command splits a name that holds a space, so running it again gives a different run:\n%s", m.Run.Command)
	}
}

// A file already sitting under the name a run writes through is not written
// over. The temporary file is created with os.Create, which truncates, and the
// collision check only ever looked at the name a file ends up with.
//
// The name carries this process's id, so the test can build the exact one the
// run is about to use. That is also why it is unlikely to be met by accident
// and easy to leave behind: a run killed outright leaves exactly this shape.
func TestAFileUnderTheTemporaryNameIsNotWrittenOver(t *testing.T) {
	dir := t.TempDir()
	name := fmt.Sprintf("files_0001.txt.tfg-partial-%d", os.Getpid())
	victim := filepath.Join(dir, name)
	if err := os.WriteFile(victim, []byte("somebody's own work\n"), 0o644); err != nil {
		t.Fatalf("writing: %v", err)
	}

	code, _, errOut := run(t, "generate", "--format", "txt", "--size", "1kb", "--out", dir)
	if code == cli.ExitOK {
		t.Error("a run wrote through a name that was already taken")
	}

	body, err := os.ReadFile(victim)
	if err != nil {
		t.Fatalf("the file is gone: %v", err)
	}
	if string(body) != "somebody's own work\n" {
		t.Errorf("the file was written over:\n%q\n%s", body, errOut)
	}
}

// failingWriter refuses everything, which is what a closed pipe looks like.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("the pipe is closed") }

// A run that could not deliver what was asked for has not succeeded, whatever
// happened on the disk. "tfg generate --json | head" is the ordinary way this
// happens, and the error used to be dropped on the floor.
func TestAManifestThatCannotReachStandardOutputIsNotASuccess(t *testing.T) {
	dir := t.TempDir()
	var errOut strings.Builder
	code := cli.Run(t.Context(),
		[]string{"generate", "--format", "txt", "--size", "1kb", "--json", "--out", dir},
		failingWriter{}, &errOut)

	if code == cli.ExitOK {
		t.Error("the run reported success while the manifest never left the process")
	}
	if !strings.Contains(errOut.String(), "standard output") {
		t.Errorf("nothing says what could not be delivered:\n%s", errOut.String())
	}
}

// Every job in the workflow has a ceiling on how long it may run. Without one
// a hung job holds a runner until the GitHub default of six hours, and the
// fuzzing job is the one most able to hang.
// topLevelKey matches a key of the workflow itself, such as "on:" or "env:".
// Anything else in column one is content somebody wrote inside a step.
var topLevelKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*:`)

func TestEveryCIJobHasACeilingOnItsTime(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Skipf("the workflow is not here: %v", err)
	}
	text := string(raw)

	// Counted rather than listed, so a job added later is covered without
	// anybody remembering to come back here.
	//
	// Only the keys under "jobs:", which is the correction the first version of
	// this needed: every key two spaces in also matched the triggers under
	// "on:", so it demanded four ceilings that have nothing to run.
	jobs, ceilings := 0, 0
	inJobs := false
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "jobs:") {
			inJobs = true
			continue
		}
		// Leaving the block takes a top level key, not merely a line starting
		// in column one. A shell string inside a "run:" step can carry those,
		// and one did: the dependency gate listed its expected modules that
		// way, this scan called the jobs block finished at the first of them,
		// and from then on it counted one job instead of seven. Seven ceilings
		// against one job is not fewer, so the guard was green whatever it was
		// given - found by mutation on 2026-08-04, not by reading.
		if inJobs && len(line) > 0 && !strings.HasPrefix(line, " ") && topLevelKey.MatchString(line) {
			inJobs = false
		}
		trimmed := strings.TrimSpace(line)
		// Commented out does not count, and that was mutation's correction:
		// counting the text anywhere in the file left a commented ceiling
		// counting as a real one, so the guard stayed green on a job that had
		// lost its own.
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "timeout-minutes:") {
			ceilings++
		}
		if inJobs && strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   ") &&
			strings.HasSuffix(trimmed, ":") {
			jobs++
		}
	}
	if jobs == 0 {
		t.Fatal("no job was found, so this guard would pass without checking anything")
	}
	if ceilings < jobs {
		t.Errorf("%d job(s) and %d ceiling(s) - a job with no timeout-minutes holds a runner for six hours when it hangs", jobs, ceilings)
	}
}

// The window binary is built. Nothing built it, so it could stop compiling and
// no run would say so until somebody tried to make a release.
func TestTheWindowBinaryIsBuiltSomewhere(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Skipf("the workflow is not here: %v", err)
	}
	// Asked of the job that runs on every operating system, not of the file.
	//
	// It used to ask the file, and the full mutation run of 2026-09-01 called
	// that a HOLE. Removing both builds from the platform matrix left the guard
	// green, because a third build had appeared since - in the sbom job, which
	// runs on Linux alone. So the substring was still there and the protection
	// was gone: a window binary that stops compiling on Windows or macOS would
	// have passed CI in silence, which is the whole thing this watches for.
	//
	// That is the failure a guard asking "is this text in the file" always has,
	// and it arrives the day the text appears a second time. Nothing about the
	// text changed - the tree grew a second writer of it.
	const matrixJob = "\n  test:\n"
	at := strings.Index(string(raw), matrixJob)
	if at < 0 {
		t.Fatal("ci.yml has no job called test, so this guard is reading a file it does not understand")
	}
	// The job ends where the next one begins, at the next key on its own
	// indentation.
	block := string(raw)[at+1:]
	if end := nextJobAfter(block); end > 0 {
		block = block[:end]
	}
	if !strings.Contains(block, "./cmd/tfg-gui") {
		t.Error("the job that runs on every operating system does not build ./cmd/tfg-gui, " +
			"so the window can stop compiling on one of them without a red run.\n" +
			"  A build elsewhere is not the same promise - the sbom job runs on Linux alone.")
	}
}

// nextJobAfter is where the job starting at the top of block ends, which is the
// next key at the same indentation. Zero when it runs to the end of the file.
func nextJobAfter(block string) int {
	for i := 1; i < len(block); i++ {
		if block[i-1] != '\n' {
			continue
		}
		rest := block[i:]
		if len(rest) > 2 && rest[0] == ' ' && rest[1] == ' ' && rest[2] != ' ' && rest[2] != '#' {
			return i
		}
	}
	return 0
}

// The formatter leaves nothing beside the file it settled. It writes through a
// temporary name now, so that a process ending mid write does not leave a
// recipe at half its length - and the temporary name must not outlive the run.
func TestSettlingARecipeLeavesNothingBesideIt(t *testing.T) {
	dir := t.TempDir()
	path := writeRecipe(t, dir, "version: 1\ntargets:\n  - id: a\n    format: txt\n    size:   1kb\n")

	if code, _, errOut := run(t, "recipe", "fmt", path, "-w"); code != cli.ExitOK {
		t.Fatalf("exit %d:\n%s", code, errOut)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	for _, e := range entries {
		if e.Name() != filepath.Base(path) {
			t.Errorf("settling left %s beside the recipe", e.Name())
		}
	}
	_ = engine.DefaultManifestName
}
