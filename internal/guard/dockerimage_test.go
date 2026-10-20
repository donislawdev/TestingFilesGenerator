package guard

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

// What this defends. The container image, ghcr.io/donislawdev/tfg, carries the
// program of a published release and nothing else, and nothing is pushed that
// was not run first.
//
// Why it needed a guard. The image is a second way to download the program, and
// it is the one nobody looks inside. Three edits would each leave it working
// and wrong without anything going red: an operating system put under the
// program (a FROM with a distribution, a RUN that installs something), the
// archive unpacked before its checksum and attestation are asked, or a push
// moved into a job a pull request runs. The workflow tries every image before
// it keeps it - every format through the image against the program it came out
// of, measured equal on 2026-10-20 - but that proof only counts while the
// order holds, and the order is what this reads.
//
// What this does NOT check. That the image works: docker.yml runs it on every
// pull request that touches it, on both architectures, and that run is the
// proof. Nor that ghcr.io shows the package publicly, which is a setting the
// owner turns on once (docs/DOCKER-ACTION-2026-10-20.md).

const (
	dockerfilePath = ".github/docker/Dockerfile"
	dockerWorkflow = "docker.yml"
)

// imageWorkflow is the shape of docker.yml, as much of it as these questions
// need.
type imageWorkflow struct {
	Permissions map[string]string `yaml:"permissions"`
	Jobs        map[string]struct {
		Needs       any               `yaml:"needs"`
		If          string            `yaml:"if"`
		Permissions map[string]string `yaml:"permissions"`
		Steps       []struct {
			Name string `yaml:"name"`
			If   string `yaml:"if"`
			Run  string `yaml:"run"`
			Uses string `yaml:"uses"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

func readImageWorkflow(t *testing.T) imageWorkflow {
	t.Helper()
	var parsed imageWorkflow
	if err := yaml.Unmarshal([]byte(workflowText(t, dockerWorkflow)), &parsed); err != nil {
		t.Fatalf("%s does not parse as YAML, so the image it describes is never made: %v", dockerWorkflow, err)
	}
	for _, job := range []string{"image", "publish"} {
		if _, ok := parsed.Jobs[job]; !ok {
			t.Fatalf("%s has no job called %s, so this guard is not reading the workflow it describes", dockerWorkflow, job)
		}
	}
	return parsed
}

// dockerInstructions is the Dockerfile as a list of instructions, each with
// its keyword in capitals, comments and blank lines left out.
func dockerInstructions(t *testing.T) []string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repoRoot(t), filepath.FromSlash(dockerfilePath)))
	if err != nil {
		t.Fatalf("%s is missing, so docker.yml has nothing to build: %v", dockerfilePath, err)
	}
	var out []string
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasSuffix(line, "\\") {
			t.Fatalf("%s continues a line with a backslash: %q. This guard reads one instruction "+
				"per line, so teach it to join them rather than let it read half of one", dockerfilePath, line)
		}
		keyword, rest, _ := strings.Cut(line, " ")
		out = append(out, strings.ToUpper(keyword)+" "+strings.TrimSpace(rest))
	}
	if len(out) == 0 {
		t.Fatalf("%s holds no instruction", dockerfilePath)
	}
	return out
}

// The program, its notices, a place to write, a user who is not root.
func TestTheImageHoldsTheReleasedProgramAndNothingElse(t *testing.T) {
	steps := dockerInstructions(t)

	if steps[0] != "FROM scratch" {
		t.Errorf("%s starts with %q.\nThe image is built FROM scratch: the program is static and "+
			"reads nothing an operating system would provide, so a base image only adds "+
			"files nobody tried and a list of vulnerabilities to answer for.", dockerfilePath, steps[0])
	}

	copies := 0
	var user, entrypoint string
	for _, step := range steps {
		keyword, rest, _ := strings.Cut(step, " ")
		switch keyword {
		case "RUN", "ADD", "FROM":
			if step != steps[0] {
				t.Errorf("%s has %q.\nThe image is the release's program copied in, nothing "+
					"run, fetched or layered: a RUN needs a shell scratch does not have, an ADD "+
					"can fetch from the network, and a second FROM is a second base.", dockerfilePath, step)
			}
		case "COPY":
			copies++
			if strings.Contains(rest, "--from") || strings.Contains(rest, "://") {
				t.Errorf("%s copies from somewhere other than the unpacked release: %q", dockerfilePath, step)
			}
		case "USER":
			user = rest
		case "ENTRYPOINT":
			entrypoint = rest
		}
	}

	want := map[string]bool{"COPY tfg /tfg": false, "COPY LICENSE THIRD-PARTY-NOTICES.md /licenses/": false}
	for _, step := range steps {
		if _, ok := want[step]; ok {
			want[step] = true
		}
	}
	for step, found := range want {
		if !found {
			t.Errorf("%s does not have %q.\nThe program and the notices its licences require are "+
				"the two things the image is for.", dockerfilePath, step)
		}
	}
	if copies != len(want) {
		t.Errorf("%s copies %d time(s) and the image holds exactly %d things from the release", dockerfilePath, copies, len(want))
	}

	if !regexp.MustCompile(`^[1-9][0-9]*:[1-9][0-9]*$`).MatchString(user) {
		t.Errorf("%s runs as %q.\nThe image runs as a numeric user that is not root: scratch has "+
			"no user database to look a name up in, and a program running as root writes files "+
			"on a Linux host that only root can delete.", dockerfilePath, user)
	}
	if entrypoint != `["/tfg"]` {
		t.Errorf("%s has the entrypoint %q, and it has to be [\"/tfg\"]: the exec form, so the "+
			"program is process 1 and docker stop reaches it as its own clean stop", dockerfilePath, entrypoint)
	}
}

// The archive is opened only after the checksum and the attestation, and the
// image is built from what was opened.
func TestTheImageIsMadeOfAReleaseThatCheckedOut(t *testing.T) {
	job := readImageWorkflow(t).Jobs["image"]

	fetch, build := -1, -1
	for i, step := range job.Steps {
		if strings.Contains(step.Run, "gh release download") {
			fetch = i
		}
		if strings.Contains(step.Run, "docker build") {
			build = i
		}
	}
	if fetch < 0 || build < 0 {
		t.Fatalf("the image job has no step that downloads the release (%d) or none that builds (%d), "+
			"so this guard is not reading the job it describes", fetch, build)
	}
	if fetch > build {
		t.Errorf("the image job builds before it downloads the release, so the image cannot hold it")
	}

	run := job.Steps[fetch].Run
	unpack := strings.Index(run, "tar -x")
	if unpack < 0 {
		t.Fatalf("the download step never unpacks the archive, so the build has nothing from the release")
	}
	for _, check := range []struct{ text, why string }{
		{"sha256sum -c", "the checksum the release lists"},
		{"gh attestation verify", "the attestation release.yml made"},
		{`--signer-workflow "$GITHUB_REPOSITORY/.github/workflows/release.yml"`, "held to the workflow that built the release"},
	} {
		at := strings.Index(run, check.text)
		if at < 0 {
			t.Errorf("the download step no longer asks %q (%s), so the image carries whatever the "+
				"download answered", check.text, check.why)
			continue
		}
		if at > unpack {
			t.Errorf("the download step unpacks the archive before it asks %q (%s)", check.text, check.why)
		}
	}
}

// Nothing a pull request runs can push, and the job that can push runs only
// after the images were tried.
func TestOnlyAnImageThatWasTriedIsPushed(t *testing.T) {
	parsed := readImageWorkflow(t)

	if len(parsed.Permissions) != 1 || parsed.Permissions["contents"] != "read" {
		t.Errorf("%s gives every job %v, and the default has to be contents: read alone - the "+
			"job that pushes asks for more itself", dockerWorkflow, parsed.Permissions)
	}

	pushes := regexp.MustCompile(`docker (push|login)|imagetools create`)
	for name, job := range parsed.Jobs {
		for _, step := range job.Steps {
			if name != "publish" && (pushes.MatchString(step.Run) || strings.Contains(step.Uses, "attest")) {
				t.Errorf("the %s job has the step %q, which pushes or attests.\nOnly the publish job "+
					"may: it is the one a pull request never runs and the one that waits for the "+
					"images to be tried.", name, step.Name)
			}
		}
		if name != "publish" && job.Permissions["packages"] == "write" {
			t.Errorf("the %s job can write packages, and only the publish job may", name)
		}
	}

	publish := parsed.Jobs["publish"]
	if !strings.Contains(publish.If, "needs.which.outputs.publish == 'true'") {
		t.Errorf("the publish job runs on %q, and it has to run only when the which job says to "+
			"publish - never for a pull request", publish.If)
	}
	needs, _ := yaml.Marshal(publish.Needs)
	if !regexp.MustCompile(`\bimage\b`).Match(needs) {
		t.Errorf("the publish job needs %s, and it has to need the image job: that is what makes "+
			"it wait for every image to be tried", strings.TrimSpace(string(needs)))
	}

	// The attestation is pushed with the login the push made, so the logout
	// comes after it. Logged out first, the attestation step stops with "No
	// credentials found" once the tags are already public - CodeRabbit on #201,
	// confirmed in @sigstore/oci, which reads ~/.docker/config.json.
	attest, logout := -1, -1
	for i, step := range publish.Steps {
		if strings.Contains(step.Uses, "attest") {
			attest = i
		}
		if strings.Contains(step.Run, "docker logout") {
			if logout >= 0 {
				t.Errorf("the publish job logs out of the registry in more than one step")
			}
			logout = i
		}
	}
	if attest < 0 {
		t.Errorf("the publish job no longer attests the image it pushes")
	}
	if logout < 0 || logout < attest {
		t.Errorf("the publish job logs out (step %d) before it attests (step %d), and the "+
			"attestation needs the login to reach the registry", logout, attest)
	} else if !strings.Contains(publish.Steps[logout].If, "always()") {
		t.Errorf("the logout runs on %q, and it has to run on always(), so it is not skipped "+
			"when a step before it fails", publish.Steps[logout].If)
	}

	// And the trying compares bytes, before the image is kept.
	tried, kept := -1, -1
	for i, step := range parsed.Jobs["image"].Steps {
		if strings.Contains(step.Run, "diff program.sums image.sums") {
			tried = i
		}
		if strings.Contains(step.Run, "docker save") {
			kept = i
		}
	}
	if tried < 0 {
		t.Errorf("the image job no longer compares the files the image writes with the files the " +
			"program writes, so an image that writes different bytes is pushed")
	}
	if kept < 0 || kept < tried {
		t.Errorf("the image job keeps the image (step %d) before it tries it (step %d)", kept, tried)
	}
}
