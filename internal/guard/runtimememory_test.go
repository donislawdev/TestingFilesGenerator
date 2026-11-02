package guard

import (
	"bytes"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
)

// The memory setting is the limit the Go runtime is handed, and the number is
// the setting's - never above what the process started with.
//
// Before 2026-11-02 the setting held only the plan, and a run took about two
// and a half times what it held: in a container of 1 GB, 400 000 text files
// were killed by the system, and with the setting handed over 500 000 were
// written (docs/FILE-CEILING-2026-11-02.md section 12). Asked of the number
// rather than of a process, so no limit is set in this test binary.
func TestTheMemorySettingIsTheLimitTheRuntimeIsHanded(t *testing.T) {
	const gib = int64(1) << 30
	const none = int64(math.MaxInt64)
	machine := func(n int64) func() (int64, error) { return func() (int64, error) { return n, nil } }
	unreadable := func() (int64, error) { return 0, errors.New("not asked") }
	setting := func(text string) core.Memory {
		m, err := core.ParseMemory(text)
		if err != nil {
			t.Fatalf("%q does not read as a memory setting: %v", text, err)
		}
		return m
	}
	cases := []struct {
		name    string
		setting core.Memory
		machine func() (int64, error)
		start   int64
		want    int64
	}{
		{"auto in a container of 1 GB", setting("auto"), machine(gib), none, gib / 2},
		{"a size of its own", setting("8gb"), machine(32 * gib), none, 8 * gib},
		{"auto under a lower GOMEMLIMIT", setting("auto"), machine(32 * gib), gib, gib},
		{"off", setting("off"), machine(gib), none, none},
		{"off under a GOMEMLIMIT", setting("off"), machine(gib), gib, gib},
		{"auto on a machine that could not be asked", setting("auto"), unreadable, none, none},
	}
	for _, c := range cases {
		if got := c.setting.On(c.machine).ForRuntime(c.start); got != c.want {
			t.Errorf("%s: the runtime is handed %d B, not %d B", c.name, got, c.want)
		}
	}
}

// And a plan in a process that is not the program's own leaves the runtime's
// limit as it was.
//
// This test binary is such a process. Its guards plan side by side, some under
// a setting of a few megabytes, and one of them handing that to the runtime
// would hold every guard beside it to it - slowly, and green.
func TestAPlanOutsideTheProgramsOwnProcessLeavesTheRuntimeAlone(t *testing.T) {
	memory, err := core.ParseMemory("64mb")
	if err != nil {
		t.Fatal(err)
	}
	before := debug.SetMemoryLimit(-1)
	if memory.On(core.MachineMemory).ForRuntime(before) == before {
		t.Fatalf("a setting of 64 MB would hand the runtime the limit it already has, %d B, so this is not the case it names", before)
	}
	_, err = engine.Plan([]engine.Target{{ID: "files", Format: "txt", Sizes: engine.Uniform(10, 100)}},
		engine.Options{OutDir: t.TempDir(), ManifestName: "manifest.json", Memory: memory})
	if err != nil {
		t.Fatalf("planning ten text files under 64 MB: %v", err)
	}
	if after := debug.SetMemoryLimit(before); after != before {
		t.Errorf("a plan in a process that is not the program's own changed the runtime's limit from %d B to %d B", before, after)
	}
}

// The command line holds the whole run to --memory, not only its plan.
//
// Asked of the real binary, because the one line that makes the process the
// program's own is in its main and nothing in this test binary runs it. With
// the collector off, the runtime collects only when a memory limit asks it
// to, so a run that collected at all was handed one. A collection a plan asks
// for itself, to measure what it holds, is marked forced in the trace and is
// not counted (pkg.go.dev/runtime, GODEBUG gctrace).
//
// What this does not hold: the window's main. The window binary needs a
// display to run a plan, and its line is the same call.
func TestTheCommandLineHoldsTheWholeRunToItsMemorySetting(t *testing.T) {
	binary := buildCommandLine(t)
	collections := func(memory string) int {
		t.Helper()
		cmd := exec.Command(binary, "generate", "--format", "txt", "--size", "4kb", "--count", "20000",
			"--memory", memory, "--dry-run", "--out", filepath.Join(t.TempDir(), "out"))
		cmd.Env = append(withoutRuntimeSettings(os.Environ()), "GOGC=off", "GODEBUG=gctrace=1")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("tfg generate --memory %s: %v\n%s", memory, err, stderr.String())
		}
		n := 0
		for _, line := range strings.Split(stderr.String(), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "gc ") && strings.Contains(line, " MB goal") && !strings.HasSuffix(line, "(forced)") {
				n++
			}
		}
		return n
	}
	if n := collections("off"); n != 0 {
		t.Fatalf("with the collector off and --memory off, %d collections ran anyway, so a collection here does not say a limit was handed", n)
	}
	if n := collections("48mb"); n == 0 {
		t.Errorf("a run of 20 000 files under --memory 48mb with the collector off never collected. " +
			"The setting did not reach the runtime, so it held the plan and nothing else")
	}
}

// withoutRuntimeSettings is the environment without the variables that set the
// Go runtime's collector, so a machine that sets them does not answer for the
// binary under test.
func withoutRuntimeSettings(env []string) []string {
	var kept []string
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		switch strings.ToUpper(name) {
		case "GOGC", "GOMEMLIMIT", "GODEBUG":
			continue
		}
		kept = append(kept, kv)
	}
	return kept
}
