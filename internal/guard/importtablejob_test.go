package guard

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The one guard that builds the window with cgo is skipped by the test matrix
// and run by a CI job of its own - two halves that only mean anything together.
//
// Half of that is a skip, and a skip is the shape this project has been caught
// by more than once: a guard that stops reaching the state it watches and is
// green honestly. Here the ways are concrete. The matrix sets the variable and
// the job misspells the test's name in its -run pattern, so go test says "no
// tests to run" and exits 0. The test is renamed and the workflow is not. The
// job ends up on a runner without gcc, the guard skips, and the step is green.
// The variable is set in the job too, by a copy and paste, and the guard skips
// everywhere. Each of those leaves every check green and the import table read
// by nobody, which is the whole premise of the software renderer gone (O218).
//
// So this reads the guard's own source for the test's name and the variable it
// consults, and holds ci.yml to both: exactly one job runs that test by name,
// on Windows, without the variable, and reads its own log for the PASS line -
// and the matrix job sets the variable. Nothing here is typed twice.
func TestTheImportTableGuardIsRunByTheJobThatNamesIt(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Skipf("the workflow is not here: %v", err)
	}
	name, variable := importTableGuard(t, root)

	jobs := ciJobs(string(raw))
	if len(jobs) < 5 {
		t.Fatalf("only %d job(s) were read out of ci.yml, so this guard is not reading the file it thinks it is", len(jobs))
	}

	// The job that names the test, by its -run pattern anchored on both ends -
	// a pattern that merely contains the name would also match a renamed test
	// that starts the same way.
	pattern := "-run '^" + name + "$'"
	var runners []string
	for job, block := range jobs {
		if strings.Contains(block, pattern) {
			runners = append(runners, job)
		}
	}
	if len(runners) != 1 {
		t.Fatalf("%d job(s) in ci.yml run %s by name, and exactly one has to.\n"+
			"The matrix skips it when %s is set, so a job that no longer names it leaves the import "+
			"table read by nobody - and go test with a pattern that matches no test exits 0.\n"+
			"Jobs found: %s", len(runners), name, variable, strings.Join(runners, ", "))
	}
	job := runners[0]
	block := jobs[job]
	t.Logf("job %q runs %s", job, name)

	if !strings.Contains(block, "runs-on: windows-latest") {
		t.Errorf("job %q runs %s somewhere other than windows-latest, where the guard skips because "+
			"a Windows binary with cgo cannot be built there", job, name)
	}
	if strings.Contains(block, variable+":") {
		t.Errorf("job %q sets %s, which asks the very guard it exists to run to skip", job, variable)
	}
	// The PASS line, because the two ways this step can be green on nothing
	// were both measured: a pattern matching no test, and a skip on a runner
	// without a C compiler. Reading the log for the test's own PASS line is
	// what turns either into a red step.
	if !strings.Contains(block, "'--- PASS: "+name+"'") {
		t.Errorf("job %q does not read its log for the line '--- PASS: %s'.\n"+
			"Without it the step is green when go test finds no such test and when the guard skips "+
			"for want of gcc - measured, both exit 0.", job, name)
	}
	if !strings.Contains(block, "pipefail") {
		t.Errorf("job %q pipes go test through tee without pipefail, so a failing go test is "+
			"hidden behind tee's exit code", job)
	}

	// And the other half: the matrix skips it, by the variable the guard reads.
	matrix, ok := jobs["test"]
	if !ok {
		t.Fatal("ci.yml has no job called test, so the matrix that is meant to skip the guard cannot be checked")
	}
	if !strings.Contains(matrix, variable+": \"1\"") {
		t.Errorf("the test matrix does not set %s, so the cold cgo build this job exists to take out "+
			"of the matrix is still in it - measured at 822 s against 434 s warm", variable)
	}
}

// importTableGuard reads the name of the guard that builds the window with cgo
// and the name of the variable it skips on, out of its own source.
//
// Anchored on the function whose first statement consults the variable, so
// that the name read here is the name of the test that actually skips - a
// second test in the same file would otherwise answer for it.
func importTableGuard(t *testing.T, root string) (name, variable string) {
	t.Helper()
	source, err := os.ReadFile(filepath.Join(root, "internal", "guard", "noimport_test.go"))
	if err != nil {
		t.Fatalf("reading the guard's source: %v", err)
	}
	head := regexp.MustCompile(`(?m)^func (Test\w+)\(t \*testing\.T\) \{\n\s*if os\.Getenv\(importTableJobVariable\)`)
	m := head.FindStringSubmatch(string(source))
	if m == nil {
		t.Fatal("noimport_test.go has no test whose first statement consults importTableJobVariable, so there is nothing for the matrix to skip")
	}
	name = m[1]
	decl := regexp.MustCompile(`(?m)^const importTableJobVariable = "([A-Z][A-Z0-9_]*)"`)
	v := decl.FindStringSubmatch(string(source))
	if v == nil {
		t.Fatal("noimport_test.go does not declare importTableJobVariable as a literal, so the workflow cannot be held to its name")
	}
	return name, v[1]
}

// ciJobs cuts the workflow into its jobs, keyed by job name, with comments
// taken out of each. A job runs from its key to the next key at the same
// indentation, the way nextJobAfter reads it, and only keys under jobs: count -
// the same correction the timeout guard needed, because the triggers under on:
// sit at that indentation too. Cut before the comments go rather than after,
// because a comment line at a job's indentation is a line nextJobAfter knows to
// step over and a blank one is not.
func ciJobs(text string) map[string]string {
	_, after, found := strings.Cut(text, "\njobs:")
	if !found {
		return nil
	}
	jobs := map[string]string{}
	key := regexp.MustCompile(`^  ([A-Za-z_][A-Za-z0-9_-]*):`)
	for after != "" {
		nl := strings.Index(after, "\n")
		var line string
		if nl < 0 {
			line, after = after, ""
		} else {
			line, after = after[:nl], after[nl+1:]
		}
		m := key.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		block := line + "\n" + after
		end := nextJobAfter(block)
		if end > 0 {
			jobs[m[1]] = withoutYamlComments(block[:end])
			after = block[end:]
		} else {
			jobs[m[1]] = withoutYamlComments(block)
			after = ""
		}
	}
	return jobs
}
