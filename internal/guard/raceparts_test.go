package guard

// The race job of ci.yml lays its guards out over its parts by what each
// weighs (O298, .github/race-weights, .github/scripts/race_parts.py).
//
// Measured before this existed: split by the checksum of each name, the four
// parts took about 1250, 1158, 795 and 526 s on the slowest processor a runner
// was given, and part 2 was killed at its twenty minutes in one run of three.
// The same part took 689 s on a faster processor, so the parts had to be even
// where the machine is slowest, not on average.

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// racePartsScript is the one place the script is named, so a move breaks one line.
func racePartsScript(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), ".github", "scripts", "race_parts.py")
}

// raceWeights is the weights file the race job lays its parts out by.
func raceWeights(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), ".github", "race-weights")
}

// runRaceParts runs the script and returns what it printed and its exit code.
func runRaceParts(t *testing.T, python string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command(python, append([]string{racePartsScript(t)}, args...)...)
	var out, errOut strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exit):
		code = exit.ExitCode()
	default:
		t.Fatalf("running %s: %v", racePartsScript(t), err)
	}
	return out.String(), errOut.String(), code
}

// guardTestNames asks this test binary for its own tests, the way the race job
// asks with go test -list, so the layout is checked against the real list.
func guardTestNames(t *testing.T) []string {
	t.Helper()
	out, err := exec.Command(os.Args[0], "-test.list", ".*").Output()
	if err != nil {
		t.Fatalf("asking this test binary for its tests: %v", err)
	}
	var names []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Test") || strings.HasPrefix(line, "Fuzz") || strings.HasPrefix(line, "Example") {
			names = append(names, line)
		}
	}
	if len(names) < 100 {
		t.Fatalf("this test binary listed %d tests, and the guard package has well over a thousand. What it printed:\n%s", len(names), out)
	}
	return names
}

// raceJob reads what the race job states: its parts and its timeout in seconds.
func raceJob(t *testing.T) (parts int, timeout float64) {
	t.Helper()
	ci := readFile(t, filepath.Join(repoRoot(t), ".github", "workflows", "ci.yml"))
	matrix := regexp.MustCompile(`(?m)^\s+part: \[([0-9, ]+)\]$`).FindAllStringSubmatch(ci, -1)
	if len(matrix) != 1 {
		t.Fatalf("ci.yml states the race job's parts %d times, and this reads exactly one strategy.matrix.part", len(matrix))
	}
	parts = len(strings.Split(matrix[0][1], ","))
	limit := regexp.MustCompile(`\./internal/guard/ -count=1 -race -timeout ([0-9]+)m `).FindAllStringSubmatch(ci, -1)
	if len(limit) != 1 {
		t.Fatalf("ci.yml runs the guards under the race detector with a stated timeout %d times, and this reads exactly one", len(limit))
	}
	minutes, err := strconv.Atoi(limit[0][1])
	if err != nil {
		t.Fatalf("ci.yml states the guard timeout as %q, which is not a number of minutes: %v", limit[0][1], err)
	}
	return parts, float64(minutes * 60)
}

// writeLines writes lines to a new file and returns its path.
func writeLines(t *testing.T, name string, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
	return path
}

// splitInto returns the names each part of `parts` runs.
func splitInto(t *testing.T, python, names, weights string, parts int) [][]string {
	t.Helper()
	got := make([][]string, parts)
	for part := range parts {
		out, errOut, code := runRaceParts(t, python, "split", names, weights, strconv.Itoa(part), strconv.Itoa(parts))
		if code != 0 {
			t.Fatalf("part %d of %d was refused (exit %d):\n%s", part, parts, code, errOut)
		}
		got[part] = strings.Fields(out)
	}
	return got
}

// Every guard runs in exactly one part, whatever the number of parts.
//
// The job also compares the names it started with the names it was given, but
// only within a part. A test laid into no part, or into two, is invisible there.
func TestEveryGuardRunsInExactlyOneRacePart(t *testing.T) {
	python := pythonForGate(t)
	names := guardTestNames(t)
	list := writeLines(t, "names.txt", names...)
	matrix, _ := raceJob(t)
	for _, parts := range []int{1, matrix, matrix + 1} {
		seen := map[string]int{}
		for part, mine := range splitInto(t, python, list, raceWeights(t), parts) {
			for _, name := range mine {
				if prior, twice := seen[name]; twice {
					t.Fatalf("with %d parts, %s is laid into part %d and part %d, so it runs twice", parts, name, prior, part)
				}
				seen[name] = part
			}
		}
		for _, name := range names {
			if _, ok := seen[name]; !ok {
				t.Errorf("with %d parts, %s is laid into no part, so the race detector never runs it", parts, name)
			}
		}
		if len(seen) != len(names) {
			t.Errorf("with %d parts the parts hold %d names and the binary lists %d", parts, len(seen), len(names))
		}
	}
}

// The parts weigh about the same by the weights, and the heaviest leaves room
// inside its timeout.
//
// The weights are the longest time each test took, so this is the layout where
// the machine is slowest. A part a tenth over the mean is uneven enough to
// matter, and a part past 85% of its timeout is one slower processor from red.
func TestTheRacePartsWeighAboutTheSame(t *testing.T) {
	python := pythonForGate(t)
	names := guardTestNames(t)
	parts, timeout := raceJob(t)
	unlisted, weights := readRaceWeights(t, raceWeights(t))
	load := make([]float64, parts)
	total := 0.0
	for part, mine := range splitInto(t, python, writeLines(t, "names.txt", names...), raceWeights(t), parts) {
		for _, name := range mine {
			w, ok := weights[name]
			if !ok {
				w = unlisted
			}
			load[part] += w
			total += w
		}
	}
	mean := total / float64(parts)
	for part, l := range load {
		if l > 1.1*mean {
			t.Errorf("part %d of %d weighs %.0f s against a mean of %.0f s (all parts: %.0f).\n"+
				"The heavy guards are no longer laid out by weight, and the heaviest part is the one the timeout ends.",
				part, parts, l, mean, load)
		}
		if l > 0.85*timeout {
			t.Errorf("part %d of %d weighs %.0f s, past 85%% of its %.0f s timeout.\n"+
				"Add a part to strategy.matrix.part in ci.yml rather than raising the timeout.", part, parts, l, timeout)
		}
	}
}

// readRaceWeights reads the weights file the way the script does, for the
// cases where this guard has to weigh a part itself.
func readRaceWeights(t *testing.T, path string) (unlisted float64, weights map[string]float64) {
	t.Helper()
	weights = map[string]float64{}
	unlisted = -1
	for _, line := range strings.Split(readFile(t, path), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		if len(fields) != 2 {
			t.Fatalf("%s holds a line that is not a weight: %q", path, line)
		}
		if fields[0] == "unlisted" {
			var err error
			if unlisted, err = strconv.ParseFloat(fields[1], 64); err != nil {
				t.Fatalf("%s holds an unlisted weight that is not a number: %q", path, line)
			}
			continue
		}
		w, err := strconv.ParseFloat(fields[0], 64)
		if err != nil {
			t.Fatalf("%s holds a weight that is not a number: %q", path, line)
		}
		weights[fields[1]] = w
	}
	if unlisted < 0 || len(weights) == 0 {
		t.Fatalf("%s has no unlisted weight or no test named, so there is nothing to lay out by", path)
	}
	return unlisted, weights
}

// The script refuses weights it cannot trust, and says what to change.
//
// A weight for a test that was renamed is the way this list falls behind: the
// renamed test would be laid out as light and nothing would say so.
func TestTheRaceSplitRefusesWeightsItCannotTrust(t *testing.T) {
	python := pythonForGate(t)
	names := writeLines(t, "names.txt", "TestOne", "TestTwo", "TestThree")
	cases := []struct {
		name    string
		weights []string
		part    string
		says    string
	}{
		{"a test that is gone", []string{"unlisted 1", "30 TestGone"}, "0", "TestGone, which the test binary does not list"},
		{"a line that is not a weight", []string{"unlisted 1", "thirty TestOne"}, "0", "a weight is a number of seconds"},
		{"a weight below nought", []string{"unlisted 1", "-3 TestOne"}, "0", "a weight is a number of seconds"},
		{"a test named twice", []string{"unlisted 1", "30 TestOne", "20 TestOne"}, "0", "names TestOne a second time"},
		{"no unlisted weight", []string{"30 TestOne"}, "0", "has no \"unlisted\" line"},
		{"a part past the parts", []string{"unlisted 1", "30 TestOne"}, "4", "part 4 of 4 does not exist"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, errOut, code := runRaceParts(t, python, "split", names, writeLines(t, "weights", c.weights...), c.part, "4")
			if code == 0 {
				t.Fatalf("the split accepted weights with %s and laid out:\n%s", c.name, out)
			}
			if !strings.Contains(errOut, c.says) {
				t.Errorf("the split refused weights with %s without saying %q. It said:\n%s", c.name, c.says, errOut)
			}
		})
	}
	if _, errOut, code := runRaceParts(t, python, "split", names, writeLines(t, "weights", "unlisted 1", "30 TestOne"), "0", "4"); code != 0 {
		t.Fatalf("the split refused weights that are right, so the cases above prove nothing:\n%s", errOut)
	}
}

// A test the weights do not name fails its part once it runs a minute.
//
// Such a test is laid out as light, so the parts stop being even, and the
// timeout would say so only when the part is already lost.
func TestARaceTestMissingFromTheWeightsFailsItsPartWhenItRunsLong(t *testing.T) {
	python := pythonForGate(t)
	weights := writeLines(t, "weights", "unlisted 0.5", "300 TestHeavy")
	quiet := []string{
		"--- PASS: TestHeavy (500.00s)",
		"--- PASS: TestLight (59.90s)",
		"    --- PASS: TestLight/long_subtest (90.00s)",
		"--- SKIP: TestSkipped (0.00s)",
		"--- PASS: TestOdd (1.2.3s)",
	}
	if _, errOut, code := runRaceParts(t, python, "unlisted", weights, writeLines(t, "part.log", quiet...)); code != 0 {
		t.Fatalf("a part whose only long tests are weighted, or are subtests, or whose one odd line is not a duration go test writes, failed:\n%s", errOut)
	}
	long := append(quiet, "--- FAIL: TestNew (60.00s)")
	_, errOut, code := runRaceParts(t, python, "unlisted", weights, writeLines(t, "part.log", long...))
	if code == 0 {
		t.Fatal("a test the weights do not name ran 60 s and its part passed, so the weights can fall behind the tests without a word")
	}
	if !strings.Contains(errOut, "60.0 TestNew") {
		t.Errorf("the part failed without giving the line to add for TestNew. It said:\n%s", errOut)
	}
}

// The race job lays its parts out with the script, asks about unlisted tests
// after its tests ran, and runs when either file changes.
//
// The job runs only on a push that touches one of its watched files, so a pull
// request changing nothing but the weights would merge them unrun.
func TestTheRaceJobLaysItsPartsOutByTheWeights(t *testing.T) {
	ci := readFile(t, filepath.Join(repoRoot(t), ".github", "workflows", "ci.yml"))
	watched := regexp.MustCompile(`(?m)^\s+watched='([^']*)'$`).FindAllStringSubmatch(ci, -1)
	if len(watched) != 1 {
		t.Fatalf("ci.yml sets the race job's watched files %d times, and this reads exactly one", len(watched))
	}
	files := strings.Fields(watched[0][1])
	for _, want := range []string{".github/race-weights", ".github/scripts/race_parts.py"} {
		if !slices.Contains(files, want) {
			t.Errorf("the race job does not run when %s changes, so a change to how its parts are laid out merges unrun", want)
		}
	}
	split := `python3 .github/scripts/race_parts.py split names.txt .github/race-weights "$PART" "$PARTS" > mine.txt`
	unlisted := `python3 .github/scripts/race_parts.py unlisted .github/race-weights part.log`
	for _, line := range []string{split, unlisted} {
		if n := strings.Count(ci, line); n != 1 {
			t.Errorf("ci.yml holds %d copies of\n  %s\nand the race job needs exactly one", n, line)
		}
	}
	if strings.Contains(ci, "| cksum)") {
		t.Error("ci.yml still lays the race parts out by the checksum of each name alone")
	}
	tee, after := strings.Index(ci, "| tee part.log"), strings.Index(ci, unlisted)
	if tee < 0 || after < tee {
		t.Error("ci.yml asks about unlisted tests before the part's log is written, so it reads nothing")
	}
}
