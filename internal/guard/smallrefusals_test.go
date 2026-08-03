package guard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// A handful of refusals the audit of 2026-08-03 found saying the wrong thing.
// None of them lost data. All of them told somebody something untrue about
// their own request, which is rule 3 of this project: the value and the text
// beside it have to agree.

// "--count -5" was reported as "asks for 0 files", because anything below one
// built an empty list and the planner then described the list rather than the
// request. The recipe reader got this right and the flag path did not.
func TestACountBelowOneIsReportedWithTheNumberThatWasWritten(t *testing.T) {
	for _, count := range []string{"-5", "0"} {
		t.Run(count, func(t *testing.T) {
			dir := t.TempDir()
			code, _, errOut := run(t,
				"generate", "--format", "txt", "--size", "1kb",
				"--count", count, "--dry-run", "--out", dir)

			if code == cli.ExitOK {
				t.Fatalf("a count of %s was accepted", count)
			}
			if !strings.Contains(errOut, count) {
				t.Errorf("the refusal does not name the number that was written:\n%s", errOut)
			}
			if count != "0" && strings.Contains(errOut, "0 files") {
				t.Errorf("the refusal describes a number nobody typed:\n%s", errOut)
			}
		})
	}
}

// Pointing --out at a file gave two messages for one fault, and the first said
// "there is nothing at that path" about a path with something at it. The system
// reports a kind of error our mapping had no sentence for.
func TestOutPointingAtAFileSaysSoOnce(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("writing: %v", err)
	}

	code, _, errOut := run(t, "generate", "--format", "txt", "--size", "1kb", "--out", file)

	if code == cli.ExitOK {
		t.Fatal("writing into a file was accepted")
	}
	if !strings.Contains(errOut, "is a file, not a directory") {
		t.Errorf("the refusal does not say what is wrong with the path:\n%s", errOut)
	}
	if strings.Contains(errOut, "there is nothing at that path") {
		t.Errorf("the refusal says the path is empty when something is there:\n%s", errOut)
	}
	// One fault, one message. Counting the lines that start with our prefix.
	if n := strings.Count(errOut, "tfg: "); n != 1 {
		t.Errorf("one mistake produced %d messages:\n%s", n, errOut)
	}
}

// A pair of dimensions the format cannot deliver is the caller's request, not
// a fault in the tool. It used to end with 1, which tells CI this build is
// broken - the same defect closed for declared ranges on 2026-08-03, surviving
// one layer deeper because the declaration bounds each side on its own.
func TestAPictureTooLargeToHoldIsTheCallersRequest(t *testing.T) {
	dir := t.TempDir()
	code, _, errOut := run(t,
		"generate", "--format", "png", "--size", "200mb",
		"--set", "width=20000", "--set", "height=20000",
		"--dry-run", "--out", dir)

	if code != cli.ExitFormat {
		t.Errorf("exit %d, expected %d - a pair the format cannot deliver is a FORMAT refusal, not a RUNTIME one:\n%s",
			code, cli.ExitFormat, errOut)
	}
	if !strings.Contains(errOut, "megapixels") {
		t.Errorf("the refusal does not say what the limit is about:\n%s", errOut)
	}
}

// And the tool has to admit the limit before somebody hits it. "tfg formats
// png" advertised each side up to 20000 and said nothing about the two
// multiplied, so it offered a pair it then refused.
func TestTheFormatListAdmitsTheLimitOnTwoSettingsAtOnce(t *testing.T) {
	code, stdout, errOut := run(t, "formats", "png")
	if code != cli.ExitOK {
		t.Fatalf("exit %d:\n%s", code, errOut)
	}
	// Declared, not merely mentioned. It began as a sentence inside the
	// description of each setting, which a person could read and nothing else
	// could - and a window building two number fields from Min and Max would
	// still offer twenty thousand in both. The registry carries the rule now,
	// so the printed line, the refusal and a future field all come from it.
	d, err := format.Get("png")
	if err != nil {
		t.Fatalf("png is not registered: %v", err)
	}
	if len(d.JointLimits) == 0 {
		t.Fatal("png declares no joint limit, so the rule binding width and height lives only in prose and in the generator")
	}
	if !strings.Contains(stdout, d.JointLimits[0].Describe()) {
		t.Errorf("the declared joint limit is not printed, so the tool still offers a pair it refuses:\n%s", stdout)
	}
}

// And a script sees it too, or a window has nothing to build from.
func TestTheJointLimitReachesMachineOutput(t *testing.T) {
	code, stdout, errOut := run(t, "formats", "png", "--json")
	if code != cli.ExitOK {
		t.Fatalf("exit %d:\n%s", code, errOut)
	}
	var list []struct {
		JointLimits []struct {
			Of, By string
			Max    int64
			Detail string
		} `json:"joint_limits"`
	}
	if err := json.Unmarshal([]byte(stdout), &list); err != nil {
		t.Fatalf("the list did not parse: %v", err)
	}
	if len(list) != 1 || len(list[0].JointLimits) == 0 {
		t.Fatalf("the machine readable list carries no joint limit:\n%s", stdout)
	}
	j := list[0].JointLimits[0]
	if j.Of == "" || j.By == "" || j.Max == 0 || j.Detail == "" {
		t.Errorf("the joint limit arrives incomplete: %+v", j)
	}
}

// The command line could not state a reason for an expectation at all, so a run
// driven by flags could never fill the category the closed list exists to make
// countable. The recipe could. That is one surface knowing something the other
// does not, which is what D1 is about.
func TestTheCommandLineCanStateWhyAnOutcomeIsExpected(t *testing.T) {
	dir := t.TempDir()

	code, stdout, errOut := run(t,
		"generate", "--format", "txt", "--size", "1kb",
		"--expected", "reject", "--expected-reason", "size_limit",
		"--json", "--out", dir)
	if code != cli.ExitOK {
		t.Fatalf("exit %d:\n%s", code, errOut)
	}

	var m manifestShape
	if err := json.Unmarshal([]byte(stdout), &m); err != nil {
		t.Fatalf("the manifest did not parse: %v", err)
	}
	if len(m.Files) != 1 {
		t.Fatalf("the manifest describes %d files", len(m.Files))
	}
	if got := m.Files[0].Expected.Reason; got != "size_limit" {
		t.Errorf("the reason reached the manifest as %q", got)
	}
}

// The list is closed so that a report can group by reason, and a typo would
// make a category of one. Both surfaces have to use the same list rather than
// each carrying a copy.
func TestAReasonNobodyRecognisesIsRefusedOnBothSurfaces(t *testing.T) {
	dir := t.TempDir()

	code, _, errOut := run(t,
		"generate", "--format", "txt", "--size", "1kb",
		"--expected", "reject", "--expected-reason", "because_i_said_so",
		"--dry-run", "--out", dir)
	if code == cli.ExitOK {
		t.Fatal("a reason nobody recognises was accepted")
	}
	if !strings.Contains(errOut, "size_limit") {
		t.Errorf("the refusal does not offer the list to pick from:\n%s", errOut)
	}

	// The same list, not a second copy of it.
	for _, r := range recipe.Reasons() {
		if !strings.Contains(errOut, r) {
			t.Errorf("the command line offers a list missing %q, so the two surfaces disagree about it", r)
		}
	}
}

// A reason with no outcome describes nothing. It used to be accepted and
// carried into the manifest beside an empty outcome.
func TestAReasonWithNoOutcomeIsRefused(t *testing.T) {
	dir := t.TempDir()
	code, _, errOut := run(t,
		"generate", "--format", "txt", "--size", "1kb",
		"--expected-reason", "size_limit", "--dry-run", "--out", dir)
	if code == cli.ExitOK {
		t.Fatal("a reason with no outcome was accepted")
	}
	if !strings.Contains(errOut, "--expected") {
		t.Errorf("the refusal does not say what is missing:\n%s", errOut)
	}
}
