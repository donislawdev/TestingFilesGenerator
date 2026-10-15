package guard

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// What this defends. The OSV actions are pinned by commit, and each runs a
// Docker image named by a tag, ghcr.io/google/osv-scanner-action:v2.6.0, which
// whoever owns it can push again. Pinning the action does not reach that
// layer, and the guard on action references cannot see it either: the tag is
// written in the action's own repository, not here (O281). So every job that
// runs the scanner first runs .github/scripts/osv_scanner_image.sh, which holds
// the image the runner pulled during set-up to the digest recorded in
// .github/osv-scanner-digest.
//
// Why the wiring rather than the check itself. Deleting that step, or moving it
// below the scan, turns the check off with nothing going red - the scan simply
// runs whatever came. So does moving it below the pull request's checkout of
// the branch it merges into, in a quieter way: the digest would then be read
// from the base, and the Dependabot pull request that writes a new one would
// be held to the old one for as long as it stayed open.
//
// That the script behaves is the next test's question, asked of the script
// itself with a docker that answers from files. What neither checks is that
// the runner still pulls the image before the first step - that is read from
// the log of a run, and the script fails if it stops doing so.

const (
	scannerDigestFile  = "osv-scanner-digest"
	scannerImageScript = "osv_scanner_image.sh"
	scannerImageStep   = "the scanner image is the one recorded"
	scannerActions     = "google/osv-scanner-action/"
)

// TestTheScannerImageIsCheckedBeforeItRuns asks every workflow that runs the
// OSV scanner whether each of its jobs checks the image first.
func TestTheScannerImageIsCheckedBeforeItRuns(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, ".github", "workflows")
	// Failed rather than skipped: the workflows are part of this repository,
	// and a skip here would pass the day they moved and the check went with
	// them (review of #187).
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("the workflows are not in %s, so nothing here can say whether the scanner image is checked: %v", dir, err)
	}

	recorded, err := os.ReadFile(filepath.Join(root, ".github", scannerDigestFile))
	if err != nil {
		t.Fatalf("%s is missing, so the check before every scan fails on every run: %v", scannerDigestFile, err)
	}
	if digest := strings.TrimSpace(string(recorded)); !isImageDigest(digest) {
		t.Errorf("%s holds %q, which is not one sha256 digest, so the check before every scan fails on every run.\n"+
			"It holds a single line: sha256: and 64 lowercase hexadecimal characters.", scannerDigestFile, digest)
	}
	if _, err := os.Stat(filepath.Join(root, ".github", "scripts", scannerImageScript)); err != nil {
		t.Fatalf("%s is missing, so the step that calls it fails before every scan: %v", scannerImageScript, err)
	}

	// Asking the predicate about inputs the tree does not contain today, so a
	// change that weakened it could not stay green on a clean tree.
	for _, c := range []struct {
		in  string
		ok  bool
		why string
	}{
		{"sha256:" + strings.Repeat("0a", 32), true, "a whole digest"},
		{"sha256:" + strings.Repeat("0a", 31), false, "two characters short"},
		{"sha256:" + strings.Repeat("0A", 32), false, "upper case, which docker never prints"},
		{strings.Repeat("0a", 32), false, "the digest without its algorithm"},
		{"v2.6.0", false, "a tag, which is what this replaces"},
		{"", false, "nothing at all"},
	} {
		if isImageDigest(c.in) != c.ok {
			t.Errorf("isImageDigest(%q) should be %v, because it is %s", c.in, c.ok, c.why)
		}
	}

	jobs := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || (!strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml")) {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		for _, job := range workflowJobs(withoutYamlComments(string(body))) {
			if job.firstScanner < 0 {
				continue
			}
			jobs++
			for _, problem := range job.scannerCheckProblems() {
				t.Errorf("%s, job %s: %s", name, job.name, problem)
			}
		}
	}

	// Asserted rather than assumed. A walk that found no job running the
	// scanner would report every job checked, which is how this guard would
	// break: a renamed action, a moved directory, a reader that stopped
	// seeing "uses:".
	if jobs == 0 {
		t.Errorf("no job in %s runs %s, so this guard checked nothing.\n"+
			"Either the scanner left the workflows - then this guard and %s go with it - "+
			"or the way this reads them stopped working.", dir, scannerActions, scannerDigestFile)
	}
}

// isImageDigest reports whether s is one sha256 image digest as docker prints
// it: the algorithm, a colon and 64 lowercase hexadecimal characters.
func isImageDigest(s string) bool {
	hex, found := strings.CutPrefix(s, "sha256:")
	if !found || len(hex) != 64 {
		return false
	}
	for _, r := range hex {
		digit := r >= '0' && r <= '9'
		lower := r >= 'a' && r <= 'f'
		if !digit && !lower {
			return false
		}
	}
	return true
}

// workflowJob is where, within one job, the lines this guard asks about are.
// Each position is a line index into the job's lines, or -1 when absent.
type workflowJob struct {
	name          string
	lines         []string
	firstScanner  int
	check         int
	firstCheckout int
}

// workflowJobs cuts a workflow into its jobs: a key indented by exactly two
// spaces under "jobs:" opens one. Only as much of YAML as the workflows here
// use - the alternative is a YAML parser for a question about three lines.
func workflowJobs(workflow string) []workflowJob {
	var jobs []workflowJob
	inJobs := false
	for _, line := range strings.Split(workflow, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, " ") {
			inJobs = strings.TrimSpace(line) == "jobs:"
			continue
		}
		if !inJobs {
			continue
		}
		if opensJob(line) {
			jobs = append(jobs, workflowJob{name: strings.TrimSuffix(strings.TrimSpace(line), ":")})
			continue
		}
		if len(jobs) > 0 {
			jobs[len(jobs)-1].lines = append(jobs[len(jobs)-1].lines, line)
		}
	}
	for i := range jobs {
		jobs[i].locate()
	}
	return jobs
}

// opensJob reports whether a line under "jobs:" is a job's own key.
func opensJob(line string) bool {
	rest, found := strings.CutPrefix(line, "  ")
	return found && !strings.HasPrefix(rest, " ") && strings.HasSuffix(rest, ":")
}

// locate finds the first use of the scanner, the check step and the first
// "git checkout" in the job.
func (j *workflowJob) locate() {
	j.firstScanner, j.check, j.firstCheckout = -1, -1, -1
	for i, line := range j.lines {
		s := strings.TrimSpace(line)
		if j.firstScanner < 0 && strings.HasPrefix(s, "uses:") && strings.Contains(s, scannerActions) {
			j.firstScanner = i
		}
		if j.check < 0 && s == "- name: "+scannerImageStep {
			j.check = i
		}
		if j.firstCheckout < 0 && strings.Contains(s, "git checkout") {
			j.firstCheckout = i
		}
	}
}

// scannerCheckProblems says what is wrong with the check in a job that runs
// the scanner, one sentence per problem.
func (j workflowJob) scannerCheckProblems() []string {
	if j.check < 0 {
		return []string{"runs the OSV scanner with no step named \"" + scannerImageStep + "\" before it, " +
			"so an image pushed again under the same tag would run unchecked."}
	}
	var problems []string
	if j.check > j.firstScanner {
		problems = append(problems, "checks the scanner image only after the scanner has run, "+
			"so whatever image came has already run by then.")
	}
	if j.firstCheckout >= 0 && j.check > j.firstCheckout {
		problems = append(problems, "checks the scanner image after a git checkout, so it reads "+
			scannerDigestFile+" from another commit - for a pull request, the branch it merges into, "+
			"which holds an update that writes a new digest to the old one for ever.")
	}
	if !strings.Contains(strings.Join(j.stepBody(j.check), "\n"), ".github/scripts/"+scannerImageScript) {
		problems = append(problems, "has a step named \""+scannerImageStep+"\" that does not run "+
			".github/scripts/"+scannerImageScript+", so the name promises a check nobody makes.")
	}
	return problems
}

// stepBody is the lines of the step whose "- name:" line is at, up to the
// next line indented no deeper than that one.
func (j workflowJob) stepBody(at int) []string {
	indent := len(j.lines[at]) - len(strings.TrimLeft(j.lines[at], " "))
	var body []string
	for _, line := range j.lines[at+1:] {
		if len(line)-len(strings.TrimLeft(line, " ")) <= indent {
			break
		}
		body = append(body, line)
	}
	return body
}

// scannerDockerStub answers the two questions osv_scanner_image.sh asks docker,
// from files in $SCANNER_STUB: "ids" is what image ls lists, "digests-<id>" is
// what image inspect prints for an id, and "fails" makes every call fail the
// way docker does with no daemon. Anything else it is asked is an error, so a
// script that started asking something new would not pass by accident.
const scannerDockerStub = `#!/bin/sh
if [ -f "$SCANNER_STUB/fails" ]; then
  echo "Cannot connect to the Docker daemon" >&2
  exit 1
fi
case "$1 $2" in
"image ls") cat "$SCANNER_STUB/ids" 2>/dev/null; exit 0 ;;
"image inspect") for last; do :; done; cat "$SCANNER_STUB/digests-$last" 2>/dev/null; exit 0 ;;
esac
echo "docker stub: not a question this stub answers: $*" >&2
exit 2
`

// scannerScriptCase is one runner the script is put in front of.
type scannerScriptCase struct {
	name    string
	digest  *string           // the digest file, or nil for none at all
	ids     []string          // the images of the scanner on the runner
	pulled  map[string]string // the digest each id was pulled as
	noDock  bool              // docker cannot be asked at all
	passes  bool
	because string // a piece of the output that says why
}

// TestTheScannerImageScriptRefusesEveryImageButTheRecordedOne runs
// .github/scripts/osv_scanner_image.sh itself, with a docker in front of the
// real one that answers from files.
//
// What it defends. The check before every OSV scan is this script, and the
// guard above only asks whether it is called. A script reduced to "exit 0"
// passed every test there was (review of #187). Each case here is a runner the
// script has to refuse, or the one it has to let through, and each asserts the
// reason it gives as well as its exit code, so a refusal for the wrong reason -
// a docker that was never found, a file that was never read - does not count.
//
// The image ids are the stub's own, img1 and img2, and the recorded image has
// to be named by id in the output. A run that reached the real docker instead
// could not print them, so the test knows it is talking to the stub.
//
// On Windows the bash a lookup finds has to be Git's (uname says Msys). Bare
// names on this machine have started WSL instead (docs/ENVIRONMENT.md), and a
// Linux bash would answer a question about a different system - the test says
// so and stops there rather than reporting about the wrong one.
func TestTheScannerImageScriptRefusesEveryImageButTheRecordedOne(t *testing.T) {
	bash := scannerTestBash(t)
	script := filepath.ToSlash(filepath.Join(repoRoot(t), ".github", "scripts", scannerImageScript))
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("%s is missing, so the step that calls it fails before every scan: %v", scannerImageScript, err)
	}

	const repository = "ghcr.io/google/osv-scanner-action@"
	hex := strings.Repeat("71ad04ab", 8)
	other := "sha256:" + strings.Repeat("d54770fe", 8)
	recorded := "sha256:" + hex
	text := func(s string) *string { return &s }

	for _, c := range []scannerScriptCase{
		{name: "the recorded image", digest: text(recorded + "\n"), ids: []string{"img1"},
			pulled: map[string]string{"img1": repository + recorded}, passes: true, because: "img1 is " + repository + recorded},
		{name: "white space around the digest", digest: text("  " + recorded + "\n\n"), ids: []string{"img1"},
			pulled: map[string]string{"img1": repository + recorded}, passes: true, because: "img1 is " + repository + recorded},
		{name: "another image behind the tag", digest: text(recorded), ids: []string{"img1"},
			pulled: map[string]string{"img1": repository + other}, because: other},
		{name: "two images, one of them another", digest: text(recorded), ids: []string{"img1", "img2"},
			pulled: map[string]string{"img1": repository + recorded, "img2": repository + other}, because: "img2 is not the one recorded"},
		{name: "no image on the runner", digest: text(recorded), because: "No image of"},
		{name: "a digest split in two", digest: text("sha256:" + hex[:32] + " " + hex[32:] + "\n"), ids: []string{"img1"},
			pulled: map[string]string{"img1": repository + recorded}, because: "does not hold one sha256 digest"},
		{name: "two digests", digest: text(recorded + "\n" + recorded + "\n"), ids: []string{"img1"},
			pulled: map[string]string{"img1": repository + recorded}, because: "does not hold one sha256 digest"},
		{name: "a tag", digest: text("v2.6.0\n"), ids: []string{"img1"},
			pulled: map[string]string{"img1": repository + recorded}, because: "does not hold one sha256 digest"},
		{name: "no digest file", ids: []string{"img1"},
			pulled: map[string]string{"img1": repository + recorded}, because: "does not hold one sha256 digest"},
		{name: "docker cannot be asked", digest: text(recorded), noDock: true, because: "Cannot connect to the Docker daemon"},
	} {
		out, passed := runScannerScript(t, bash, script, c)
		if passed != c.passes || !strings.Contains(out, c.because) {
			t.Errorf("%s: the script %s, and it should %s, saying %q. It printed:\n%s",
				c.name, verdictOf(passed), verdictOf(c.passes), c.because, out)
		}
	}
}

// verdictOf names an exit.
func verdictOf(passed bool) string {
	if passed {
		return "let the scan run"
	}
	return "stopped the scan"
}

// runScannerScript puts the script in front of one runner and reports what it
// printed and whether it exited 0.
func runScannerScript(t *testing.T, bash, script string, c scannerScriptCase) (string, bool) {
	t.Helper()
	stub := t.TempDir()
	write := func(name, body string, mode os.FileMode) {
		if err := os.WriteFile(filepath.Join(stub, name), []byte(body), mode); err != nil {
			t.Fatalf("writing the stub's %s: %v", name, err)
		}
	}
	write("docker", scannerDockerStub, 0o755)
	write("ids", strings.Join(c.ids, "\n"), 0o644)
	for id, digest := range c.pulled {
		write("digests-"+id, digest+"\n", 0o644)
	}
	if c.noDock {
		write("fails", "", 0o644)
	}
	digestFile := filepath.Join(stub, "recorded-digest")
	if c.digest != nil {
		write("recorded-digest", *c.digest, 0o644)
	}

	cmd := exec.Command(bash, script, filepath.ToSlash(digestFile))
	cmd.Env = append(os.Environ(),
		"PATH="+stub+string(os.PathListSeparator)+os.Getenv("PATH"),
		"SCANNER_STUB="+filepath.ToSlash(stub))
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		t.Fatalf("%s: bash did not run the script: %v", c.name, err)
	}
	return string(out), err == nil
}

// scannerTestBash is the bash the script runs under on this machine, or a
// skip that says why there is none to trust.
func scannerTestBash(t *testing.T) string {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skipf("no bash on this machine, so the script cannot be run here: %v. The Linux and macOS jobs run it.", err)
	}
	if runtime.GOOS != "windows" {
		return bash
	}
	out, err := exec.Command(bash, "-c", "uname -o").Output()
	if system := strings.TrimSpace(string(out)); err != nil || system != "Msys" {
		t.Skipf("the bash found first is %s, which says it runs on %q rather than Msys, so it would answer "+
			"for another system (docs/ENVIRONMENT.md). The Linux and macOS jobs run the script.", bash, system)
	}
	return bash
}
