package guard

import (
	"debug/pe"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Windows decides whether a program gets a console from one number in its PE
// header. These are the two that matter.
const (
	subsystemWindows = 2 // a windowed program, no console
	subsystemConsole = 3 // a console program, and the system opens one
)

// The window binary starts without a black console window beside it.
//
// Go links every binary as a console program unless the linker is told
// otherwise, so the window binary shipped with a terminal standing next to it
// for as long as there has been a window binary. Closing that terminal kills
// the program. The owner confirmed on 2026-08-19 that it is in the way, and it
// is the first thing anybody sees - the program introduces itself as a raw
// build before it has drawn anything.
//
// This builds a binary and reads the number out of it, rather than reading the
// command that builds it. The difference is not pedantry here: the flag has to
// survive two different linkers, because a build with CGO links through gcc
// and a build without it links through Go's own linker, and a guard reading
// the build command would pass on a flag that had quietly stopped doing
// anything. Measured on 2026-08-19, both ways produce a windowed binary and
// the same build without the flag produces a console one, so building here
// with CGO off measures what the shipped build does. It costs about a second
// once the packages are cached, and it needs no C compiler, so it also runs on
// the machines that have none.
//
// The command line binary is checked for the opposite, and that half is not
// symmetry for its own sake. A console program turned windowed writes its
// output nowhere at all - for a tool whose purpose is to run inside somebody
// else's CI, that is a worse defect than the one being fixed here, and a
// single careless flag applied to both commands would cause it.
func TestTheWindowBinaryStartsWithoutAConsole(t *testing.T) {
	flags := windowLinkerFlags(t)

	for _, binary := range []struct {
		name    string
		pkg     string
		ldflags string
		want    uint16
		why     string
	}{
		{
			name:    "tfg-gui.exe",
			pkg:     "./cmd/tfg-gui",
			ldflags: flags,
			want:    subsystemWindows,
			why: "Windows attaches a console window to a console program, so the window binary\n" +
				"opens with an empty black terminal beside it that kills the program when closed.",
		},
		{
			name:    "tfg.exe",
			pkg:     "./cmd/tfg",
			ldflags: "",
			want:    subsystemConsole,
			why: "A windowed program has nowhere to write, so every message this tool prints\n" +
				"would go nowhere - in a build agent, where it is the only thing anybody reads.",
		},
	} {
		t.Run(binary.name, func(t *testing.T) {
			// This one has to build something, which the three other guards
			// that shell out to the toolchain already say out loud when they
			// cannot. tools/linux-check.py cross compiles the test binary here
			// and carries it into a container that deliberately has no Go, so
			// without this the whole Linux run failed on a missing toolchain
			// rather than on anything about this tree - and said so in words
			// that read like a defect in the product.
			//
			// A build that runs and fails is still fatal below. Only the
			// absence of the toolchain is a skip, because a guard that cannot
			// be run is not a guard that passed.
			if _, err := exec.LookPath("go"); err != nil {
				t.Skipf("no Go toolchain here, so nothing can be built to read a header from: %v", err)
			}

			built := filepath.Join(t.TempDir(), binary.name)

			args := []string{"build"}
			if binary.ldflags != "" {
				args = append(args, "-ldflags="+binary.ldflags)
			}
			args = append(args, "-o", built, binary.pkg)

			build := exec.Command("go", args...)
			build.Dir = filepath.Join("..", "..")
			// Windows because the header being read is a Windows one, and CGO
			// off because the linker flag reaches the same number either way
			// and this has to run where there is no C compiler.
			build.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0")
			if out, err := build.CombinedOutput(); err != nil {
				t.Fatalf("building %s: %v\n%s", binary.pkg, err, out)
			}

			got := subsystemOf(t, built)
			if got != binary.want {
				t.Errorf("%s was built as subsystem %d (%s) and it has to be %d (%s).\n%s\n"+
					"The linker flags for the window binary live in .github/gui-ldflags and\n"+
					"nowhere else. This was built with %q.",
					binary.name, got, subsystemName(got), binary.want, subsystemName(binary.want),
					binary.why, binary.ldflags)
			}
		})
	}
}

// The workflow builds the window binary with the flags the repository
// declares, rather than with a copy of them.
//
// Two places holding the same flags is the shape this project has been bitten
// by before - the version lives in internal/version and the resource script
// carries a copy, and that copy is guarded for exactly this reason. Here the
// copy can simply not exist: the workflow reads the file. This checks that it
// still does, because somebody writing the flags out by hand would restore the
// drift without noticing they had.
func TestTheWorkflowBuildsTheWindowWithTheFlagsTheRepositoryDeclares(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Skipf("the workflow is not here: %v", err)
	}

	var line string
	for _, candidate := range strings.Split(string(raw), "\n") {
		if strings.Contains(candidate, "go build") && strings.Contains(candidate, "./cmd/tfg-gui") {
			line = candidate
			break
		}
	}
	if line == "" {
		t.Fatal("nothing in the workflow builds ./cmd/tfg-gui, so this guard would pass without checking anything")
	}
	if !strings.Contains(line, ".github/gui-ldflags") {
		t.Errorf("the workflow builds the window binary without reading .github/gui-ldflags:\n  %s\n"+
			"The flags belong in that file and are read from it, so that the release build and\n"+
			"this one cannot say different things about whether the program opens a console.",
			strings.TrimSpace(line))
	}
}

// windowLinkerFlags reads the one place the window binary's linker flags live.
func windowLinkerFlags(t *testing.T) string {
	t.Helper()

	path := filepath.Join("..", "..", ".github", "gui-ldflags")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("no linker flags for the window binary: %v\n"+
			".github/gui-ldflags is where they live, and without it the window binary\n"+
			"is built as a console program.", err)
	}
	flags := strings.TrimSpace(string(raw))
	if flags == "" {
		t.Fatal(".github/gui-ldflags is empty, so the window binary would be built as a console program")
	}
	return flags
}

// subsystemOf reads the Windows subsystem out of a compiled binary.
func subsystemOf(t *testing.T, path string) uint16 {
	t.Helper()

	f, err := pe.Open(path)
	if err != nil {
		t.Fatalf("reading the header of %s: %v", path, err)
	}
	defer f.Close()

	switch header := f.OptionalHeader.(type) {
	case *pe.OptionalHeader64:
		return header.Subsystem
	case *pe.OptionalHeader32:
		return header.Subsystem
	default:
		t.Fatalf("%s has no optional header, so its subsystem cannot be read", path)
		return 0
	}
}

func subsystemName(subsystem uint16) string {
	switch subsystem {
	case subsystemWindows:
		return "windowed"
	case subsystemConsole:
		return "console"
	default:
		return "neither windowed nor console"
	}
}
