package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

// A release says two things about itself that nobody has to take on trust:
// where the files came from, and what is inside them.
//
// Neither is a code signature and neither pretends to be. A signature says who
// vouches for a file. Provenance says which source and which build produced it,
// signed with an identity that lives for minutes and recorded in a public log,
// so a stolen key cannot forge it and a fork cannot claim to be this
// repository. The binaries are unsigned and the release notes say so.
//
// The whole point is that a person can check it, so these guards ask whether
// the release actually makes the statements and actually publishes them.

// publishJob is the part of the release workflow these guards read.
type publishJob struct {
	Permissions map[string]string `yaml:"permissions"`
	Steps       []struct {
		Name string            `yaml:"name"`
		Uses string            `yaml:"uses"`
		With map[string]any    `yaml:"with"`
		Run  string            `yaml:"run"`
		Env  map[string]string `yaml:"env"`
	} `yaml:"steps"`
}

func releasePublishJob(t *testing.T) publishJob {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "release.yml"))
	if err != nil {
		t.Skipf("no release workflow here: %v", err)
	}
	var workflow struct {
		Jobs map[string]publishJob `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(body, &workflow); err != nil {
		t.Fatalf("reading the release workflow: %v", err)
	}
	job, ok := workflow.Jobs["publish"]
	if !ok {
		t.Fatal("the release workflow has no publish job, so nothing here could have been checked")
	}
	return job
}

// The two permissions nothing else grants. Without them the steps below do not
// fail loudly - they cannot mint a token, and a release would go out carrying
// no statement at all while every other job stayed green.
func TestTheReleaseAsksForWhatAnAttestationNeeds(t *testing.T) {
	job := releasePublishJob(t)
	for _, permission := range []string{"id-token", "attestations"} {
		if job.Permissions[permission] != "write" {
			t.Errorf("the publish job does not ask for %s: write, and an attestation cannot be made without it",
				permission)
		}
	}
	if job.Permissions["contents"] != "write" {
		t.Error("the publish job cannot write the release itself any more")
	}
}

// The build states what IT made, over a list of exactly those files.
//
// It says nothing about what is inside them any more, and that moved rather
// than disappeared: three of these archives get a signature afterwards, which
// changes their bytes, so the statement about what a person downloads is made
// by attest-release.yml over the signed files. A statement about the wrong
// bytes verifies against nothing.
func TestTheReleaseAttestsWhatItBuilt(t *testing.T) {
	job := releasePublishJob(t)

	attested := false
	for _, step := range job.Steps {
		action, _, _ := strings.Cut(step.Uses, "@")
		if action != "actions/attest-build-provenance" {
			continue
		}
		attested = true
		// A list rather than a glob: the statement then covers exactly what was
		// built, under the names it was built under.
		if got, _ := step.With["subject-checksums"].(string); got != "incoming/build.sha256" {
			t.Errorf("the build attests %q rather than the list of what it produced", got)
		}
	}
	if !attested {
		t.Error("the build makes no statement about what it produced, " +
			"so the signing step has nothing to check before it signs")
	}
}

// The document is generated where the versions can be read, and it travels with
// the build to the machine that signs. The statements end up beside the
// downloads rather than only in the attestation store: a person behind a proxy
// cannot reach that store, and Scorecard reads assets by extension and never
// opens it.
func TestTheReleaseMakesItsDocumentAndHandsItOver(t *testing.T) {
	job := releasePublishJob(t)

	var script strings.Builder
	for _, step := range job.Steps {
		script.WriteString(step.Run)
		script.WriteString("\n")
	}
	all := script.String()

	for what, want := range map[string]string{
		"the bill of materials is generated":           "go run ./internal/legal/cmd/sbom",
		"it is written where the build is handed over": "-o \"incoming/verify-tfg_${version}.spdx.json\"",
		"the statement travels with the build":         "build.provenance.sigstore.json",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("%s: the publish job never runs %q", what, want)
		}
	}

	// And the ritual that follows publishes both statements as assets. One is
	// renamed by the signing script, the other is uploaded by the workflow that
	// makes it.
	signing := signingScript(t)
	if !strings.Contains(signing, "provenance.sigstore.json") {
		t.Error("the signing script never publishes the build's statement, " +
			"so verifying a Linux or macOS archive offline is impossible")
	}
	attest := workflowText(t, "attest-release.yml")
	if !strings.Contains(attest, "sbom.sigstore.json") {
		t.Error("nothing publishes the statement about what is inside the signed files")
	}
}

// And the notes tell somebody how to use any of it. A statement nobody knows
// about is a statement nobody checks.
// What the attesting half downloads is named BY the checksums, not beside them.
//
// Found by the first real release, on 2026-08-28, after the signing was already
// done. That job fetched the release with a hand written list of patterns -
// archives, the SBOM, the checksums - and then ran sha256sum -c over the
// checksums. The checksums also describe the provenance bundle, which no
// pattern matched, so sha256sum said "No such file or directory" about a file
// that was sitting on the release page the whole time, and the run stopped.
//
// Two lists of what a release holds is one list too many, and the failure mode
// is not a missing file: it is a check that reports a problem with the release
// when the problem is in the checker. That is expensive here, because it lands
// at the one moment when somebody is holding a card and half a release.
//
// So the names come out of the file being verified. Adding an asset cannot make
// the two disagree again, because there is only one list.
func TestTheAttestingHalfFetchesWhatTheChecksumsName(t *testing.T) {
	attest := workflowText(t, "attest-release.yml")

	if strings.Contains(attest, "--pattern '*.zip'") {
		t.Error("attest-release.yml fetches the release with its own list of patterns. " +
			"That list is a second description of what a release holds, and when it " +
			"disagrees with the checksums the run stops with a message about the release " +
			"rather than about itself.\n" +
			"What to do: read the names out of verify-SHA256SUMS.txt and fetch those.")
	}
	if !strings.Contains(attest, "done < verify-SHA256SUMS.txt") {
		t.Error("attest-release.yml does not take the list of files to fetch from " +
			"verify-SHA256SUMS.txt, so nothing keeps what it downloads and what it " +
			"verifies in step")
	}
}

func TestTheReleaseNotesSayHowToCheckWhatWasDownloaded(t *testing.T) {
	job := releasePublishJob(t)
	var notes string
	for _, step := range job.Steps {
		if step.Name == "notes" {
			notes = step.Run
		}
	}
	if notes == "" {
		t.Fatal("the publish job writes no notes, so there was nothing to read")
	}
	for _, want := range []string{"gh attestation verify", "spdx.json"} {
		if !strings.Contains(notes, want) {
			t.Errorf("the release notes never mention %q", want)
		}
	}
	// The binaries are still unsigned, and the notes have said so since the
	// first release. Provenance is a different question and must not be read
	// as an answer to that one.
	if !strings.Contains(notes, "not signed") {
		t.Error("the release notes stopped saying the binaries are unsigned, which is still true")
	}
}
