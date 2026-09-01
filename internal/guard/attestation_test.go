package guard

import (
	"os"
	"path/filepath"
	"regexp"
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

// Every workflow that names the SBOM predicate type names the SAME one.
//
// This fact is written five times across three files, and it cannot be written
// once: the notes tell a person what to type, the attesting half checks that
// promise against the statement it just made, and the verifying half runs the
// command for real. They are three different jobs in three different files and
// a workflow cannot read a constant out of another one.
//
// What it cost when they disagreed, on the first real release, on
// 2026-08-28: the notes promised https://spdx.dev/Document, the statement
// carried https://spdx.dev/Document/v2.3, and the attesting half stopped the
// release. It was RIGHT to - the promise was written from the action's
// documentation rather than from an attestation, and the provenance plan said
// out loud that it was unmeasured - but the value that was wrong was written in
// five places and only one of them was checked.
//
// The one that matters most is not the check. It is the pair of commands a
// person types out of the release notes: with the wrong URI, gh answers "no
// attestation found", which reads exactly like a release nobody attested.
func TestEveryWorkflowNamesTheSamePredicateType(t *testing.T) {
	dir := filepath.Join(repoRoot(t), ".github", "workflows")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("no workflows here: %v", err)
	}

	uri := regexp.MustCompile(`https://spdx\.dev/[A-Za-z0-9./-]*`)
	found := map[string][]string{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yml") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", entry.Name(), err)
		}
		for _, match := range uri.FindAllString(string(body), -1) {
			found[match] = append(found[match], entry.Name())
		}
	}

	if len(found) == 0 {
		t.Fatal("no workflow names the SBOM predicate type, so this guard checked nothing")
	}
	if len(found) > 1 {
		for value, files := range found {
			t.Errorf("%q is named in %v", value, files)
		}
		t.Error("the workflows disagree about the SBOM predicate type. One of them tells a " +
			"person what to type, one checks that promise against the statement, and one " +
			"runs the command for real - so a disagreement here is a release page whose " +
			"own instructions answer \"no attestation found\".\n" +
			"What to do: the value is whatever the statement actually carries. Read it out " +
			"of a bundle rather than out of documentation.")
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

	// BOTH commands, because they answer different questions and the notes say
	// so themselves. One asks what is inside a file and needs the predicate
	// type spelled out, since gh asks for build provenance unless told
	// otherwise. The other asks where a Linux archive came from and must not
	// carry that flag.
	//
	// Asking only whether the words appear was not enough, and the full
	// mutation run of 2026-09-01 said so: replacing one of the two left the
	// guard green, because the other still carried the phrase. That is the
	// third guard in this tree to fail the same way in one run - "is this text
	// in the file" stops meaning anything the day the text appears twice.
	if n := strings.Count(notes, "gh attestation verify"); n < 2 {
		t.Errorf("the release notes give %d attestation command(s) and there are two things to check - "+
			"what is inside a file, and where a Linux archive came from", n)
	}
	if !strings.Contains(notes, "--predicate-type https://spdx.dev/Document/v2.3") {
		t.Error("the release notes never name the predicate type, so somebody following them asks " +
			"for build provenance and is told the bill of materials is not there")
	}
	// The binaries are still unsigned, and the notes have said so since the
	// first release. Provenance is a different question and must not be read
	// as an answer to that one.
	if !strings.Contains(notes, "not signed") {
		t.Error("the release notes stopped saying the binaries are unsigned, which is still true")
	}
}
