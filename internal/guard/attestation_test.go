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

// Two statements, two actions, and each over the files the release actually
// publishes rather than over a glob that might match something else.
func TestTheReleaseAttestsEveryFileItPublishes(t *testing.T) {
	job := releasePublishJob(t)

	wanted := map[string]bool{
		"actions/attest-build-provenance": false,
		"actions/attest":                  false,
	}
	for _, step := range job.Steps {
		action, _, _ := strings.Cut(step.Uses, "@")
		if _, want := wanted[action]; !want {
			continue
		}
		wanted[action] = true
		// The list of checksums is the list the release publishes, so the
		// statement covers exactly what is on the page, under the names people
		// download. A glob would cover whatever happened to be in the
		// directory.
		if got, _ := step.With["subject-checksums"].(string); got != "incoming/SHA256SUMS.txt" {
			t.Errorf("%s attests %q rather than the checksums of what is published", action, got)
		}
	}
	for action, found := range wanted {
		if !found {
			t.Errorf("the release never runs %s, so it publishes files nobody can check", action)
		}
	}
}

// The document has to be generated and the statements have to end up beside the
// downloads. An attestation only in the store is one a person behind a proxy
// cannot reach, and Scorecard reads assets by extension and never opens it.
func TestTheReleasePublishesItsDocumentAndItsStatements(t *testing.T) {
	job := releasePublishJob(t)

	var script strings.Builder
	sbomAttested := false
	for _, step := range job.Steps {
		script.WriteString(step.Run)
		script.WriteString("\n")
		if _, ok := step.With["sbom-path"]; ok {
			sbomAttested = true
		}
	}
	all := script.String()

	for what, want := range map[string]string{
		"the bill of materials is generated":             "go run ./internal/legal/cmd/sbom",
		"it is written where the release publishes from": "-o \"incoming/tfg_${version}.spdx.json\"",
		"the provenance bundle becomes an asset":         "provenance.sigstore.json",
		"the sbom bundle becomes an asset":               "sbom.sigstore.json",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("%s: the publish job never runs %q", what, want)
		}
	}
	if !sbomAttested {
		t.Error("no step attests the bill of materials, so the document travels unsigned")
	}
}

// And the notes tell somebody how to use any of it. A statement nobody knows
// about is a statement nobody checks.
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
