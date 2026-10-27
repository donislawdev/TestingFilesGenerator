package guard

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// The memory this program reads off the machine is the memory a second,
// independent source reports - docs/STABILITY-REVIEW-2026-10-26.md section
// 10.8.
//
// Asked of the system on whichever of the three CI runs on, and checked
// against something that is not this code: /proc/meminfo on Linux, the
// system's management interface on Windows, the sysctl command on macOS. A
// read of the wrong field of the right structure, a structure laid out one
// field off, or the last byte lost on macOS - each gives a number, and only a
// second source can say it is the wrong one.
//
// The branch is asserted rather than assumed (CLAUDE.md, trap 1). On Linux the
// answer is the memory fitted or the limit of the control group, whichever is
// smaller, and the test says which it expected. On Windows the commit limit of
// a job could make the answer smaller than the memory fitted - a runner placed
// in such a job would be a surprise, and this says so rather than passing.
func TestTheMemoryOfTheMachineIsWhatTheSystemSays(t *testing.T) {
	got, err := core.MachineMemory()
	if err != nil {
		t.Fatalf("the memory of this machine could not be read: %v", err)
	}

	switch runtime.GOOS {
	case "linux":
		fitted := memTotal(t)
		limit, limited, err := core.CgroupMemoryLimit(os.ReadFile)
		if err != nil {
			t.Fatalf("the control group of this process could not be read: %v", err)
		}
		want, branch := fitted, "the memory fitted"
		if limited && limit < fitted {
			want, branch = limit, "the limit of the control group"
		}
		if got != want {
			t.Errorf("read %d B, expected %d B - %s (MemTotal %d B, group limit %d B, limited %v)", got, want, branch, fitted, limit, limited)
		}
		t.Logf("%d B, %s", got, branch)
	case "windows":
		visible := secondOpinion(t, "pwsh", "-NoProfile", "-NonInteractive", "-Command",
			"(Get-CimInstance Win32_OperatingSystem).TotalVisibleMemorySize") * 1024
		if got != visible {
			t.Errorf("read %d B and the management interface says %d B of memory is fitted - the wrong field, a structure out of step, or a commit limit on this process", got, visible)
		}
	case "darwin":
		fitted := secondOpinion(t, "sysctl", "-n", "hw.memsize")
		if got != fitted {
			t.Errorf("read %d B and sysctl says %d B - the byte syscall.Sysctl drops is the first suspect", got, fitted)
		}
	default:
		t.Skipf("no second source for %s, and CI does not run there", runtime.GOOS)
	}
}

// memTotal is MemTotal from /proc/meminfo, in bytes.
func memTotal(t *testing.T) int64 {
	t.Helper()
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	lines := bufio.NewScanner(f)
	for lines.Scan() {
		fields := strings.Fields(lines.Text())
		if len(fields) == 3 && fields[0] == "MemTotal:" && fields[2] == "kB" {
			n, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			return n * 1024
		}
	}
	t.Fatal("/proc/meminfo has no MemTotal line in kB")
	return 0
}

// secondOpinion runs a command that prints one number and reads it.
func secondOpinion(t *testing.T, name string, args ...string) int64 {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		t.Fatalf("%s printed %q, not a number", name, out)
	}
	return n
}
