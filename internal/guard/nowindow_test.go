package guard

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// A window binary with no window in it says so, fails, and leaves standard
// output empty.
//
// Why this exists, and how it was found. internal/gui/gui.go was the only
// production file in the tree that no guard executed at all - measured on
// 2026-09-05 with tools/impact.py, which after subtracting the blocks that
// init() lights up on its own reported exactly one such file out of 166.
// Following it found the real gap next door rather than in gui.go itself:
// internal/gui/run_nocgo.go carries a four part D6 message and an exit code,
// and nothing checked that a binary ever produced either. The catalogue guard
// knows the sentence exists as a text entry. That is a different fact.
//
// Why it has to build a binary rather than call gui.Run. The guard binary is
// compiled WITH cgo, so run resolves to run_cgo.go, and calling gui.Run from
// here would try to open a real window on somebody's desktop. The only way to
// reach the other half of that build tag is to build it the way the person who
// meets this message builds it, which is with CGO_ENABLED=0.
//
// Measured 2026-09-05: that build costs 1.7 s, because disabling cgo excludes
// the whole graphics toolkit instead of compiling it.
//
// Three contracts, and the third is the one nothing anywhere covered:
//
//   - the message reaches standard error, and it is the one from
//     internal/gui/text rather than a sentence written twice,
//   - the exit code is 1, because this is a run that did not do what it was
//     asked to do,
//   - standard output stays EMPTY. This project keeps "a failed run writes
//     nothing to standard output" on its regression surface for the command
//     line. The window binary is the other half of that rule and had no guard,
//     which matters because a script reading a report from stdout cannot tell
//     an empty answer from a broken one.
func TestABuildWithNoWindowInItSaysSoAndKeepsStandardOutputEmpty(t *testing.T) {
	// A build that runs and fails is fatal below. Only the absence of the
	// toolchain is a skip, because a guard that cannot be run is not a guard
	// that passed - and tools/linux-check.py runs this suite in a container
	// with no Go in it by design.
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("no Go toolchain here, so no binary can be built to run: %v", err)
	}

	name := "tfg-gui-nocgo"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	built := filepath.Join(t.TempDir(), name)

	// The tag comes from the file, never from memory - that is what
	// TestEverythingThatCompilesUsReadsTheBuildTagsFromOnePlace is about.
	build := exec.Command("go", "build", "-tags", buildTags(), "-o", built, "./cmd/tfg-gui")
	build.Dir = filepath.Join("..", "..")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the window binary with cgo disabled: %v\n%s\n"+
			"That build is the one somebody gets without a C compiler, so it has to "+
			"compile even though it cannot draw anything.", err, out)
	}

	// No linker flags on purpose. The shipped window binary is built for the
	// windows subsystem so it opens without a console, and that is guarded
	// elsewhere. Here the question is what the code SAYS, so it is built plain
	// and its streams can be read.
	run := exec.Command(built)
	var stdout, stderr bytes.Buffer
	run.Stdout = &stdout
	run.Stderr = &stderr
	err := run.Run()

	code := 0
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatalf("running the window binary built without cgo: %v", err)
	}

	if code != 1 {
		t.Errorf("a window binary with no window in it exited %d and it has to exit 1.\n"+
			"Why it matters: this is the only thing somebody building from source without "+
			"a C compiler ever sees, and a zero here tells their script the window opened.\n"+
			"Where it lives: internal/gui/run_nocgo.go.", code)
	}

	if stdout.Len() != 0 {
		t.Errorf("a failed run wrote %d byte(s) to standard output: %q\n"+
			"Why it matters: standard output is where a machine reads a report from. "+
			"A run that did nothing has to leave it empty, so silence and a report "+
			"cannot be confused. The command line keeps this rule and the window "+
			"binary is the other half of it.", stdout.Len(), stdout.String())
	}

	if !strings.Contains(stderr.String(), text.NoWindowInThisBuild) {
		t.Errorf("the binary did not say what a build with no window says.\n"+
			"wanted, from internal/gui/text: %q\n"+
			"got on standard error: %q\n"+
			"Why it matters: without this sentence the binary exits 1 with nothing to "+
			"read, and the reason - no C support, so no OpenGL - is exactly what the "+
			"person cannot guess.", text.NoWindowInThisBuild, stderr.String())
	}
}
