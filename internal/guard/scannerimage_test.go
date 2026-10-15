package guard

import (
	"os"
	"path/filepath"
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
// What this does NOT check, and it is checked another way. That the script
// behaves, and that the runner still pulls the image before the first step.
// The script needs Docker and the image, which a Go test on any of the three
// systems does not have, and running bash from a test here starts a different
// bash than a lookup finds (releasenote_test.go). It was run by hand on the
// same image, four ways (docs/OSV-2-2026-10-15.md): the recorded digest passes,
// a digest one character off fails and prints the one pulled, a file of the
// wrong shape fails, and no image at all fails rather than passing. Whether
// the runner pulls first is read from the log of a run, and the script fails
// if it stops doing so.

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
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("the workflows are not here: %v", err)
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
