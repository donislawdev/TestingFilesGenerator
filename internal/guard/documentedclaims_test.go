package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/archive"
)

// The security policy says what the release actually does about signing.
//
// It said the opposite until 2026-09-06: "The released binaries are not signed.
// Signing is not set up, the release notes say so, and your operating system
// will warn you." Every part of that had stopped being true. The release
// workflow writes notes saying the Windows binaries are signed and the macOS
// ones are signed and notarised by Apple, internal/legal/codesign.go pins the
// certificate by the SHA-256 of its DER bytes, and there is a guard family
// around the wording of those notes.
//
// The effect of a stale sentence there is not cosmetic, and that is why this
// exists. SECURITY.md is the one document a cautious person reads before
// deciding whether to trust a download. Telling them the signature they can see
// is not one the project makes is the exact reasoning that leads somebody to
// ignore a mismatch, or to skip a check they have been told there is nothing to
// do.
//
// Tied to the release notes rather than to a list written here, so the two
// cannot drift apart again. Found by an outside review on 2026-09-05.
func TestTheSecurityPolicySaysWhatTheReleaseDoesAboutSigning(t *testing.T) {
	policy := readRepoFile(t, "SECURITY.md")
	notes := workflowText(t, "release.yml")

	if strings.Contains(policy, "binaries are not signed") ||
		strings.Contains(policy, "Signing is not set up") {
		t.Error("SECURITY.md still says the released binaries are not signed.\n" +
			"They are: the Windows ones are signed and the macOS ones are signed and notarised. " +
			"A person who reads that sentence has been told the signature they can see is not ours.")
	}

	// What the release notes claim, and therefore what the policy has to agree
	// with. Asking the workflow rather than repeating its words keeps one
	// source of truth for the day a platform is added or dropped.
	claims := map[string]string{
		"Windows binaries are signed":             "Windows",
		"macOS binaries are signed and notarised": "macOS",
		"Linux binaries are not signed":           "Linux",
	}
	checked := 0
	for inNotes, platform := range claims {
		if !strings.Contains(notes, inNotes) {
			continue
		}
		checked++
		if !strings.Contains(policy, platform) {
			t.Errorf("the release notes say %q and SECURITY.md never mentions %s.\n"+
				"That document is where somebody decides whether to trust a download, so it has "+
				"to name which platforms carry a signature and which do not.", inNotes, platform)
		}
	}
	if checked == 0 {
		t.Fatal("no sentence about signing was found in release.yml, so this guard compared " +
			"SECURITY.md against nothing. If the wording of the notes changed, change the claims " +
			"above with it rather than leaving a check that asks nothing.")
	}
}

// The setting that locks an archive says what locking it does not give.
//
// This is not a defect in the design and the review that raised it said so. The
// salt comes from the run seed rather than from crypto/rand on purpose: a
// random one would give two runs of one recipe different bytes, which
// untouchable rule 3 forbids. The password is in the manifest on purpose too.
// Both are right for a tool that produces fixtures.
//
// What was missing is the sentence where somebody meets it. "aes-256" carries a
// meaning everywhere else it is written, and a person choosing it here gets an
// archive that offers no confidentiality at all: same recipe, same seed, same
// password gives the same key on any machine, and the password is written down
// beside the file.
//
// Asked of the declaration rather than of the README, because the declaration
// is what both surfaces show - tfg formats prints it and the window puts it
// beside the field.
func TestTheLockingSettingSaysItIsAFixtureRatherThanProtection(t *testing.T) {
	zip, err := format.Get("zip")
	if err != nil {
		t.Fatalf("zip is not registered, so this guard has nothing to read: %v", err)
	}

	var detail string
	for _, p := range zip.Properties {
		if p.Name == archive.Encryption {
			detail = p.Detail
		}
	}
	if detail == "" {
		t.Fatalf("zip declares no %q setting with a description, so this guard asks nothing",
			archive.Encryption)
	}

	// The two concrete reasons it is not protection. A rewrite that drops both
	// has dropped the meaning, whatever else it says.
	for _, word := range []string{"seed", "manifest"} {
		if !strings.Contains(detail, word) {
			t.Errorf("the description of %q never mentions the %s:\n  %s\n"+
				"The two reasons a locked archive from this tool is not protection are that the "+
				"key comes from the run seed and that the password is written into the manifest. "+
				"Somebody choosing aes-256 here has to be told that, because that name means "+
				"something else everywhere they have met it before.", archive.Encryption, word, detail)
		}
	}
}

// readRepoFile reads a file from the root of the repository.
func readRepoFile(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), name))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return string(raw)
}
