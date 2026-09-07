package guard

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/audit"
	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// Two runs are allowed to share a directory, and verify used to call one of
// them a mismatch caused by the other.
//
// output.manifest exists so that a second run can record itself beside the
// first instead of being refused. Measured on 2026-09-07 with two runs one
// after another into one directory, name templates that do not collide, both
// ending 0:
//
//	verify manifest-alpha.json   3 differences, exit 7
//	  extra b_0001.txt   extra b_0002.txt   extra manifest-beta.json
//	verify manifest-beta.json    4 differences, exit 7
//	  extra a_0001.txt  extra a_0002.txt  extra a_0003.txt  extra manifest-alpha.json
//
// "extra" is the word for a file nobody asked for, so the report read as a
// directory somebody had polluted. Every one of those files was the
// neighbour's, and which neighbour was written down in the same directory the
// whole time.
//
// The rule the repair follows is ATTRIBUTION, NOT SUPPRESSION, and the guards
// below are split along that line. Every file stays in the report. What changes
// is the word it is given and whether it makes the directory a mismatch - so
// untouchable rule 6 is kept literally rather than on trust, and a manifest
// somebody drops into a directory can claim a file out loud and cannot hide one.

// twoRunsSharing writes two runs into one directory, whose files do not
// collide, and gives back the directory and the first run's manifest.
func twoRunsSharing(t *testing.T) (dir string, alpha *manifest.Manifest, alphaPath string) {
	t.Helper()
	dir = t.TempDir()

	run := func(id, name, manifestName string, count int) (*manifest.Manifest, string) {
		opt := engine.Options{OutDir: dir, ManifestName: manifestName, Seed: 11, Command: "test"}
		target := txtTarget(id, count, 2048)
		target.NameTmpl = name
		planned, err := engine.Plan([]engine.Target{target}, opt)
		if err != nil {
			t.Fatalf("planning %s: %v", id, err)
		}
		res, err := engine.Run(context.Background(), planned, opt)
		if err != nil {
			t.Fatalf("running %s: %v", id, err)
		}
		path := engine.ManifestPath(opt)
		if err := res.Manifest.Save(path); err != nil {
			t.Fatalf("saving the manifest of %s: %v", id, err)
		}
		return res.Manifest, path
	}

	alpha, alphaPath = run("alpha", "a_{index:04}.txt", "manifest-alpha.json", 3)
	run("beta", "b_{index:04}.txt", "manifest-beta.json", 2)
	return dir, alpha, alphaPath
}

func TestVerifyNamesTheFilesOfAnotherRunRatherThanCallingThemExtra(t *testing.T) {
	dir, alpha, _ := twoRunsSharing(t)

	diffs, err := audit.Verify(context.Background(), dir, alpha, "manifest-alpha.json")
	if err != nil {
		t.Fatalf("verifying: %v", err)
	}

	// Attribution, not suppression: the neighbour's two files AND its record
	// are all still here. Naming them correctly must not mean dropping them.
	want := map[string]string{
		"b_0001.txt":         "manifest-beta.json",
		"b_0002.txt":         "manifest-beta.json",
		"manifest-beta.json": "",
	}
	if len(diffs) != len(want) {
		t.Fatalf("expected the neighbour's two files and its record, got %v.\n"+
			"Silence is banned, so naming them correctly cannot mean leaving them out", diffs)
	}
	for _, d := range diffs {
		claimedBy, known := want[d.Path]
		if !known {
			t.Errorf("unexpected difference %v", d)
			continue
		}
		if d.Kind != audit.AnotherRun {
			t.Errorf("verify called %s %q. It belongs to the run manifest-beta.json records, and %q is the word for a file nobody asked for",
				d.Path, d.Kind, audit.Extra)
		}
		if d.Want != claimedBy {
			t.Errorf("%s says it is claimed by %q and it is claimed by %q - a reader has to be told WHICH run to ask",
				d.Path, d.Want, claimedBy)
		}
	}
}

// A file no manifest in the directory lists is still extra.
//
// This is the half that keeps the repair from being a way of accepting
// anything. Without it, dropping a manifest into a directory would account for
// every file in it.
//
// Two shapes, because they fail differently. A file nothing claims is the
// ordinary one. A JSON document that is not a manifest is the one somebody
// would reach for to test the sieve, and it has to come back extra as well - it
// gets past the name and is refused by the schema check behind it.
func TestAStrayFileBesideAnotherRunsRecordIsStillExtra(t *testing.T) {
	dir, alpha, _ := twoRunsSharing(t)
	strays := map[string]string{
		"somebody-elses.txt": "not ours",
		"settings.json":      `{"files":[{"path":"b_0001.txt"}]}`,
	}
	for name, body := range strays {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}

	diffs, err := audit.Verify(context.Background(), dir, alpha, "manifest-alpha.json")
	if err != nil {
		t.Fatalf("verifying: %v", err)
	}
	seen := map[string]bool{}
	for _, d := range diffs {
		if _, isStray := strays[d.Path]; !isStray {
			continue
		}
		seen[d.Path] = true
		if d.Kind != audit.Extra {
			t.Errorf("%s was called %q. Nothing in this directory claims it, and the presence of somebody's manifest cannot make every file in the directory accounted for",
				d.Path, d.Kind)
		}
	}
	for name := range strays {
		if !seen[name] {
			t.Errorf("%s is not in the report at all", name)
		}
	}
}

// A record under another extension is reported the way it was before any of
// this existed, and that limit is pinned rather than left to be discovered.
//
// Only files named like a record are opened, because reading the first bytes of
// every unclaimed file was measured and was too expensive - see
// couldBeNamedLikeARecord. The cost of that narrowing is exactly this: a
// neighbour who called their manifest something else is still "extra".
//
// Pinned so that widening it later is a decision somebody makes on purpose,
// with this guard in front of them, rather than a line quietly deleted.
func TestARecordUnderAnotherExtensionIsStillExtra(t *testing.T) {
	dir, alpha, _ := twoRunsSharing(t)
	renamed := filepath.Join(dir, "manifest-beta.record")
	if err := os.Rename(filepath.Join(dir, "manifest-beta.json"), renamed); err != nil {
		t.Fatalf("renaming the neighbour's record: %v", err)
	}

	diffs, err := audit.Verify(context.Background(), dir, alpha, "manifest-alpha.json")
	if err != nil {
		t.Fatalf("verifying: %v", err)
	}
	for _, d := range diffs {
		if d.Kind == audit.AnotherRun {
			t.Errorf("%s was attributed to a record this build does not open, so the sieve is not the one that was measured", d.Path)
		}
	}
}

// The sentence tells a reader which run to ask.
func TestTheSentenceAboutAnotherRunNamesTheRecordThatClaimsTheFile(t *testing.T) {
	dir, alpha, _ := twoRunsSharing(t)
	diffs, err := audit.Verify(context.Background(), dir, alpha, "manifest-alpha.json")
	if err != nil {
		t.Fatalf("verifying: %v", err)
	}
	for _, d := range diffs {
		if d.Path != "b_0001.txt" {
			continue
		}
		if said := d.String(); !strings.Contains(said, "manifest-beta.json") {
			t.Errorf("the sentence does not say which run wrote this file, so a reader still has to guess:\n  %s", said)
		}
		return
	}
	t.Fatal("the neighbour's file is not in the report, so this guard checked nothing")
}

// A shared directory is not a mismatch, and that is what makes output.manifest
// usable in the place it exists for.
//
// A check that is red whenever it worked is the same as no check at all, and
// this one sits in CI by design.
func TestADirectorySharedWithAnotherRunStillMatchesThisManifest(t *testing.T) {
	_, _, alphaPath := twoRunsSharing(t)

	code, stdout, errOut := run(t, "verify", alphaPath)
	if code != cli.ExitOK {
		t.Errorf("verify gave %d on a directory that holds exactly what this manifest says it holds.\n%s", code, errOut)
	}
	if !strings.Contains(stdout, "matches") {
		t.Errorf("the answer somebody asked for is not on stdout:\n%s", stdout)
	}
	// Said, not hidden. One line per neighbouring record rather than one per
	// file, because a neighbour can have written twenty five thousand of them.
	if !strings.Contains(errOut, "manifest-beta.json") {
		t.Errorf("nothing was said about the other run in the directory, so its files are now invisible:\n%s", errOut)
	}
	if strings.Count(errOut, "\n") > 2 {
		t.Errorf("the note is one line per file rather than one per record:\n%s", errOut)
	}
}

// A real disagreement is still a mismatch, neighbour or no neighbour.
func TestARealDifferenceIsStillAMismatchInASharedDirectory(t *testing.T) {
	dir, _, alphaPath := twoRunsSharing(t)
	if err := os.Remove(filepath.Join(dir, "a_0001.txt")); err != nil {
		t.Fatalf("removing one of our own files: %v", err)
	}

	code, _, errOut := run(t, "verify", alphaPath)
	if code != cli.ExitVerify {
		t.Fatalf("a missing file gave %d rather than %d - the neighbour cannot make our own loss acceptable", code, cli.ExitVerify)
	}
	if !strings.Contains(errOut, "1 difference") {
		t.Errorf("the heading counts the neighbour's files as disagreements:\n%s", errOut)
	}
	if !strings.Contains(errOut, "missing") {
		t.Errorf("the missing file is not named:\n%s", errOut)
	}
}

// The machine readable report carries every file, whatever it is called.
//
// This is the guard for attribution against suppression, in the form a script
// reads. A reader that wants only the disagreements filters on kind, and one
// that wants to know what else is in the directory has it.
func TestTheJSONReportCarriesTheOtherRunsFilesAndStillSaysItMatched(t *testing.T) {
	_, _, alphaPath := twoRunsSharing(t)

	code, stdout, _ := run(t, "verify", alphaPath, "--json")
	if code != cli.ExitOK {
		t.Fatalf("verify --json gave %d on a shared directory", code)
	}
	var report struct {
		Matched     bool `json:"matched"`
		Differences []struct {
			Kind string `json:"kind"`
			Path string `json:"path"`
			Want string `json:"expected"`
		} `json:"differences"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if !report.Matched {
		t.Error("matched is false about a directory that holds exactly what this manifest says it holds")
	}
	got := map[string]string{}
	for _, d := range report.Differences {
		if d.Kind != string(audit.AnotherRun) {
			t.Errorf("%s is reported as %q", d.Path, d.Kind)
		}
		got[d.Path] = d.Want
	}
	for _, name := range []string{"b_0001.txt", "b_0002.txt", "manifest-beta.json"} {
		if _, ok := got[name]; !ok {
			t.Errorf("%s is not in the report at all. Calling a file the neighbour's cannot mean not mentioning it", name)
		}
	}
	if got["b_0001.txt"] != "manifest-beta.json" {
		t.Errorf("the report does not say which run claims b_0001.txt, it says %q", got["b_0001.txt"])
	}
}
