package guard

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/legal"
)

// Signing splits a release in two, and the split is the point.
//
// The key is on a card in a USB reader and cannot be exported. No GitHub hosted
// runner can reach it, and a self hosted runner in a public repository is a
// machine strangers can aim a pull request at. So the build happens where
// builds belong, the signature happens where the card is, and what travels
// between them is an artefact plus a statement about it.
//
// These guards are over the seam. Every one of them is about a failure that is
// quiet: an unsigned file published for a quarter of an hour, a build signed
// without being checked, a second certificate signing just as willingly, a
// release page that looks finished and is missing a file.

func workflowText(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", name))
	if err != nil {
		t.Skipf("no %s here: %v", name, err)
	}
	return string(body)
}

func signingScript(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "scripts", "sign_release.py"))
	if err != nil {
		t.Skipf("no signing script here: %v", err)
	}
	return string(body)
}

// The build workflow hands the build over. It does not publish it.
func TestTheReleaseHandsTheBuildOverInsteadOfPublishingIt(t *testing.T) {
	release := workflowText(t, "release.yml")

	// An unsigned executable on a release page - even for a quarter of an hour,
	// even on a draft whose asset URLs exist - is an executable somebody
	// downloads. Three of these archives are signed afterwards, so none of them
	// goes up here.
	if regexp.MustCompile(`gh release create[^\n]*incoming`).MatchString(release) {
		t.Error("the release workflow uploads archives when it opens the draft, " +
			"and three of them have not been signed yet")
	}
	for _, want := range []string{
		"name: unsigned-build-${{ github.ref_name }}",
		"sign_release.py",
	} {
		if !strings.Contains(release, want) {
			t.Errorf("the release workflow never mentions %q, so the build reaches nobody", want)
		}
	}
	// The checksums a person reads are written where the final bytes are made.
	// This phase writes its own list, for its own statement, under its own name.
	if !strings.Contains(release, "sha256sum -- * > build.sha256") {
		t.Error("the build no longer lists what it produced, so the statement below it names nothing")
	}
	if strings.Contains(release, "sha256sum -- * > SHA256SUMS.txt") {
		t.Error("the build writes the checksums a user reads, and three of the files still change afterwards")
	}
}

// Two questions asked before anything is built, both of which used to be
// answered by somebody remembering.
func TestTheReleaseChecksTheTreeAndTheChangelogFirst(t *testing.T) {
	release := workflowText(t, "release.yml")

	// By name. A query for every check answers about whatever happened to
	// report, so a workflow that never ran reads exactly like one that passed.
	if !strings.Contains(release, "gh run list --workflow=CI --commit=") {
		t.Error("nothing asks whether CI passed on the commit being released.\n" +
			"This workflow builds and publishes and runs none of the linters, scanners or gates.")
	}
	if !strings.Contains(release, "grep -q \"^## \\[${version}\\]\" CHANGELOG.md") {
		t.Error("nothing checks that the changelog has a section for the version being released")
	}
	if !strings.Contains(release, "[Unreleased]") {
		t.Error("nothing checks that entries were moved out of Unreleased, so changes could ship undescribed")
	}
}

// What the script refuses is worth more than what it does.
func TestTheSigningScriptRefusesBeforeItSigns(t *testing.T) {
	script := signingScript(t)

	for what, want := range map[string]string{
		"it verifies the build before touching it":                  "gh\", \"attestation\", \"verify\"",
		"it timestamps, or the signature dies with the certificate": "/tr",
		"it reads the certificate back out of the signed file":      "certificate_of(program)",
		"it selects the certificate by OID rather than by a name":   "1.3.6.1.5.5.7.3.3",
		"it reads the pin from one place":                           "codesign.go",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("%s: the script does not contain %q", what, want)
		}
	}

	// Nothing here publishes. The release stays a draft until a person reads it.
	for _, forbidden := range []string{"--draft=false", "release edit", "gh release publish"} {
		if strings.Contains(script, forbidden) {
			t.Errorf("the signing script contains %q, and nothing in this ritual may publish", forbidden)
		}
	}
}

// The workflow that speaks about signed bytes must not claim to have built them.
func TestTheAttestationWorkflowDoesNotClaimToHaveBuiltAnything(t *testing.T) {
	attest := workflowText(t, "attest-release.yml")

	if strings.Contains(attest, "attest-build-provenance") {
		t.Error("the attestation workflow claims build provenance for files a person signed " +
			"on their own machine, which is false in the one document nobody should have to doubt")
	}
	for what, want := range map[string]string{
		"it downloads what the release holds":                       "gh release download",
		"it cross-checks the digest it was given":                   "CLAIMED",
		"it checks the checksums describe the files they came with": "sha256sum -c SHA256SUMS.txt",
		"it attests the document against those bytes":               "sbom-path",
		"it publishes the statement as an asset":                    "gh release upload",
	} {
		if !strings.Contains(attest, want) {
			t.Errorf("%s: the workflow does not contain %q", what, want)
		}
	}

	// The notes tell people to pass a predicate type, because gh asks for build
	// provenance unless told otherwise. The URI is written by hand in one place
	// and checked against a real statement in the other, at the only moment the
	// real value exists.
	release := workflowText(t, "release.yml")
	promised := regexp.MustCompile(`--predicate-type (\S+)`).FindStringSubmatch(release)
	if promised == nil {
		t.Fatal("the release notes no longer tell anybody which predicate type to ask for")
	}
	if !strings.Contains(attest, promised[1]) {
		t.Errorf("the notes promise --predicate-type %s and the attestation workflow never checks it.\n"+
			"A wrong URI answers \"no attestation found\", which reads like a broken release.",
			promised[1])
	}
}

// The pin is a digest of bytes, not a name somebody can rename.
func TestThePinnedCertificateIsADigestWithADate(t *testing.T) {
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(legal.CodeSigningSHA256) {
		t.Errorf("the pinned certificate is %q, which is not a sha256", legal.CodeSigningSHA256)
	}
	expires, err := time.Parse("2006-01-02", legal.CodeSigningExpires)
	if err != nil {
		t.Fatalf("the certificate expiry %q is not a date: %v", legal.CodeSigningExpires, err)
	}
	// Not an assertion about today - a certificate expiring is a fact of life
	// and a red suite on the day it happens helps nobody. The script refuses to
	// sign with an expired certificate, which is where the refusal belongs.
	t.Logf("the pinned certificate expires %s", expires.Format("2006-01-02"))

	// And the script has to read this file rather than carry its own copy.
	if !strings.Contains(signingScript(t), "CodeSigningSHA256") {
		t.Error("the signing script does not read the pin from internal/legal, " +
			"so there are two written copies of one digest")
	}
}
