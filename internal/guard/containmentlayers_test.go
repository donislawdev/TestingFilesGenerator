package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// A path from somebody else's manifest is stopped twice, by two checks that
// know nothing about each other. The text of the path is judged when the
// manifest is read, in manifest.Load, and where the path actually lands is
// judged when it is used, in the audit walk. The first catches a written climb
// such as "../x", the second catches a name that holds no climb at all and
// still leaves through a link.
//
// Measured 2026-08-04, and it is the reason these two guards exist: every test
// that went through verify or cleanup stayed green with either check switched
// off, because the other one caught the same input. Two layers covering each
// other are worth having and cannot be told apart from the outside, so each is
// asked its own question here.
//
// What this does not do is replace the end to end guards. Those prove the
// outcome a person sees. These prove that a particular door is shut.
func TestTheTextualContainmentCheckCatchesEveryWrittenClimb(t *testing.T) {
	for _, climb := range []string{
		"../VICTIM.txt",
		"..",
		`..\VICTIM.txt`,
		"a/../../VICTIM.txt",
		"../../out/../VICTIM.txt",
	} {
		t.Run(climb, func(t *testing.T) {
			if core.ContainmentProblem(climb) == "" {
				t.Errorf("%q is called a path that stays inside the directory", climb)
			}
		})
	}
}

// The other direction, so the rule above cannot be satisfied by refusing
// everything. An ordinary entry and one naming a subdirectory both have to
// pass, or a fixture set arranged in folders stops working.
func TestTheTextualContainmentCheckLetsOrdinaryPathsThrough(t *testing.T) {
	for _, ok := range []string{
		"invoice.txt",
		"sub/invoice.txt",
		"a/b/c.txt",
		"my file.txt",
	} {
		t.Run(ok, func(t *testing.T) {
			if problem := core.ContainmentProblem(ok); problem != "" {
				t.Errorf("%q was refused: %s", ok, problem)
			}
		})
	}
}

// The door itself. A manifest arrives from outside - it travels with a fixture
// set, it turns up in a pull request, it gets edited by hand - so the check
// belongs where it is read rather than in each command that reads it. Asked
// here directly, because through verify or cleanup the audit walk refuses the
// same entry and the answer would be the same either way.
func TestLoadingAManifestRefusesAnEntryThatClimbsOut(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatalf("making the output directory: %v", err)
	}
	mf := escapingManifest(t, out, "../VICTIM.txt", 21)

	if _, err := manifest.Load(mf); err == nil {
		t.Error("a manifest whose entry climbs out of the directory was read without complaint, so every command that reads one inherits the escape")
	} else if !strings.Contains(strings.ToLower(err.Error()), "outside") &&
		!strings.Contains(strings.ToLower(err.Error()), "climb") {
		t.Errorf("the refusal does not say what is wrong with the entry: %v", err)
	}
}

// And the reverse, so the door is not simply nailed shut. A manifest naming a
// file in a subdirectory is ordinary and has to load.
func TestLoadingAManifestAcceptsAnEntryInASubdirectory(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatalf("making the output directory: %v", err)
	}
	mf := escapingManifest(t, out, "sub/invoice.txt", 21)

	if _, err := manifest.Load(mf); err != nil {
		t.Errorf("a manifest naming a file in a subdirectory was refused: %v", err)
	}
}
