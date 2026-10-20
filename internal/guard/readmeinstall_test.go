package guard

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// What this defends. The step README gives for installing tfg on a GitHub
// Actions runner checks the archive - its checksum and its attestation - before
// it unpacks it and puts it on the path every later step uses.
//
// Why it needed a guard. The step is copied into other people's workflows,
// where it decides which program their build runs. A checksum from the same
// release only says the archive arrived whole. The attestation says this
// project's release workflow made it, which is what SECURITY.md tells people
// to check, and the first version of the step left it out (CodeRabbit on #201).
// With the wrong predicate type gh answers "no attestation found" on a good
// archive: Windows and macOS archives verify only against the SPDX statement
// made over the signed files, measured 2026-10-20 on v0.5.0.
//
// What this does NOT check. That the step runs. It was run as written on
// Windows and Linux, and on a Mac without the gh line, on 2026-10-20 - not on a
// runner.

func readmeInstallStep(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "README.md"))
	if err != nil {
		t.Fatalf("README.md is missing: %v", err)
	}
	text := string(body)
	const opens = "- name: install tfg"
	start := strings.Index(text, opens)
	if start < 0 {
		t.Fatalf("README.md has no step called %q, so this guard is not reading what it describes", opens)
	}
	end := strings.Index(text[start:], "```")
	if end < 0 {
		t.Fatalf("the README step %q never closes its code block", opens)
	}
	return text[start : start+end]
}

func TestTheReadmeInstallStepChecksTheArchiveBeforeOpeningIt(t *testing.T) {
	step := readmeInstallStep(t)

	opened := len(step)
	for _, unpack := range []string{"unzip ", "tar -x"} {
		if at := strings.Index(step, unpack); at >= 0 && at < opened {
			opened = at
		}
	}
	if opened == len(step) {
		t.Fatalf("the README install step unpacks nothing, so this guard is not reading what it describes")
	}

	// The predicate the release workflows name, read out of them so the two
	// cannot drift (TestEveryWorkflowNamesTheSamePredicateType holds them to
	// one value).
	predicate := regexp.MustCompile(`https://spdx\.dev/[A-Za-z0-9./-]*`).
		FindString(workflowText(t, "verify-release.yml"))
	if predicate == "" {
		t.Fatalf("verify-release.yml names no SPDX predicate type, so this guard has nothing to compare")
	}

	for _, check := range []struct{ text, why string }{
		{"shasum -a 256 -c", "the checksum the release lists"},
		{"gh attestation verify", "the attestation the release workflow made"},
		{"--predicate-type " + predicate, "the statement the signed archives carry, the one verify-release.yml checks"},
	} {
		at := strings.Index(step, check.text)
		if at < 0 {
			t.Errorf("the README install step no longer has %q (%s), so a copied workflow runs "+
				"whatever the download answered", check.text, check.why)
			continue
		}
		if at > opened {
			t.Errorf("the README install step unpacks the archive before %q (%s)", check.text, check.why)
		}
	}

	// gh on a runner refuses to start without a token in the environment.
	if !strings.Contains(step, "GH_TOKEN: ${{ github.token }}") {
		t.Errorf("the README install step does not give gh a token, and on a runner gh will not " +
			"start without GH_TOKEN")
	}
}
