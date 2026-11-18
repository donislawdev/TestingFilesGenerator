package guard

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestMain lets a job of ci.yml run the part of the guards it was handed in a
// file, rather than on its command line.
//
// Why a file. The guards are split over the parts of three jobs - the tests on
// each system, coverage and the race detector - because one process of them
// took 995 to 1080 s of an eighteen minute limit on Windows and macOS
// (2026-11-18, O361). A part is a list of test names. The race job hands its
// list to go test as one -run pattern, and on Linux that is fine. Half of this
// package is about 34 000 characters of names, and Windows hands a program at
// most 32 767 characters of command line, so the same pattern cannot reach a
// test binary there. A file can.
//
// The file is taken only when nobody asked for tests by name or for the list.
// Several guards run this binary again with -test.run, to have one test run in
// a process of its own, and the child inherits the variable: it has to run the
// one test it was started for, not the whole part over again.
func TestMain(m *testing.M) {
	flag.Parse()
	if err := runThePartListed(os.Getenv(guardPartVariable)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	os.Exit(m.Run())
}

// guardPartVariable names the file of test names one part runs. ci.yml sets
// it, and this is the one place in Go that reads it.
const guardPartVariable = "TFG_GUARD_PART"

// guardName is a name go test -list prints for this package, and nothing else:
// a name is joined into a pattern, so a stray character would change what the
// pattern matches rather than fail.
var guardName = regexp.MustCompile(`^(Test|Fuzz|Example)[A-Za-z0-9_]*$`)

// runThePartListed points -test.run at the names in the file at path, one or
// more to a line. Nothing happens when path is empty or a run or a list was
// asked for by name.
//
// A relative path is read from the root of the module, where ci.yml writes
// the file and runs go test. go test starts this binary in the package's own
// directory, and read from there the same path names a file that is not
// there.
func runThePartListed(path string) error {
	if path == "" || flag.Lookup("test.run").Value.String() != "" || flag.Lookup("test.list").Value.String() != "" {
		return nil
	}
	if !filepath.IsAbs(path) {
		root, err := moduleRoot()
		if err != nil {
			return fmt.Errorf("%s names %s, a path read from the root of the module, and the root cannot be found: %w", guardPartVariable, path, err)
		}
		path = filepath.Join(root, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%s names %s as the tests to run, and it cannot be read: %w", guardPartVariable, path, err)
	}
	names := strings.Fields(string(data))
	if len(names) == 0 {
		return fmt.Errorf("%s names %s as the tests to run, and it lists none. A part given no test would pass having run nothing", guardPartVariable, path)
	}
	for _, name := range names {
		if !guardName.MatchString(name) {
			return fmt.Errorf("%s names %s as the tests to run, and %q in it is not a test name go test -list prints", guardPartVariable, path, name)
		}
	}
	return flag.Set("test.run", "^("+strings.Join(names, "|")+")$")
}

// TestAPartRunsTheTestsItIsHandedAndNoOther runs this binary again with a part
// of two cheap guards and asks which tests started.
//
// The first two children are started the way ci.yml starts a part: the file
// at the root of a module, named by a path relative to it, and the binary in
// the package's directory below, where go test starts it. Handed the file, the
// two in it run and nothing else. Handed the file and one test by name, only
// that test runs - the child a guard starts. The third is handed an empty file
// by its full path and refuses, having read it, before running anything. A
// TestMain that ignored the file would run all of the package, so every child
// has a deadline and a part that reached a third test fails here in seconds
// rather than in eighteen minutes.
func TestAPartRunsTheTestsItIsHandedAndNoOther(t *testing.T) {
	part := []string{"TestAGridAsksEveryPartOfAFieldForItsSizeOnce", "TestANewBatchIsTheOneOpen"}
	module := t.TempDir()
	writeOrFail(t, filepath.Join(module, "go.mod"), []byte("module part\n"))
	writeOrFail(t, filepath.Join(module, "mine.txt"), []byte(strings.Join(part, "\n")+"\n"))
	inPackage := filepath.Join(module, "internal", "guard")
	if err := os.MkdirAll(inPackage, 0o755); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(t.TempDir(), "empty.txt")
	writeOrFail(t, empty, []byte("\n"))

	ran, out, err := runPart(t, inPackage, "mine.txt")
	if err != nil || !slices.Equal(ran, part) {
		t.Fatalf("handed %v in mine.txt at the root of the module, as ci.yml hands a part, the binary ran %v (%v):\n%s", part, ran, err, out)
	}
	ran, out, err = runPart(t, inPackage, "mine.txt", "-test.run=^"+part[1]+"$")
	if err != nil || !slices.Equal(ran, part[1:]) {
		t.Fatalf("handed the part and asked for %s by name, the binary ran %v (%v), and a guard's child would run the part again:\n%s", part[1], ran, err, out)
	}
	ran, out, err = runPart(t, "", empty)
	if err == nil || len(ran) != 0 || !strings.Contains(out, guardPartVariable+" names "+empty+" as the tests to run, and it lists none") {
		t.Fatalf("handed an empty part, the binary ran %v (%v) without refusing in words naming %s and the file it read:\n%s", ran, err, guardPartVariable, out)
	}
}

// runPart runs this test binary in dir, or in this package's directory when
// dir is empty, with the part file and args. It returns the top level tests
// that started, sorted - they start in the order of the source, not of the
// file.
func runPart(t *testing.T, dir, part string, args ...string) (ran []string, out string, err error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], append([]string{"-test.v", "-test.count=1", "-test.timeout=90s"}, args...)...)
	cmd.Dir = dir
	defer func() { slices.Sort(ran) }()
	cmd.Env = append(slices.DeleteFunc(os.Environ(), func(kv string) bool {
		return strings.HasPrefix(kv, guardPartVariable+"=")
	}), guardPartVariable+"="+part)
	raw, err := cmd.CombinedOutput()
	for _, line := range strings.Split(string(raw), "\n") {
		if name, ok := strings.CutPrefix(strings.TrimSpace(line), "=== RUN   "); ok && !strings.Contains(name, "/") {
			ran = append(ran, name)
		}
	}
	return ran, string(raw), err
}
