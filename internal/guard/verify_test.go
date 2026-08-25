package guard

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

// "verify" is what turns a folder of files back into evidence. A storage
// migration, a backup restore or a sync is exactly the thing it is pointed at,
// and every one of those fails by losing a file, adding one, or changing the
// content of one. All three have to be caught, because a verify that catches
// two of the three reads as a pass and gets trusted.

// generated runs a small generate and returns the output directory and the
// path of the manifest it wrote.
func generated(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	path := writeRecipe(t, dir, `version: 1
seed: 4242
targets:
  - id: a
    format: txt
    count: 3
    size: 1kb
output:
  dir: `+filepath.ToSlash(out)+`
`)
	if code, _, errOut := run(t, "generate", path); code != cli.ExitOK {
		t.Fatalf("generate gave %d:\n%s", code, errOut)
	}
	return out, filepath.Join(out, "manifest.json")
}

func TestVerifyMatchesADirectoryItJustGenerated(t *testing.T) {
	out, mf := generated(t)

	code, stdout, errOut := run(t, "verify", mf)
	if code != cli.ExitOK {
		t.Fatalf("verify gave %d on an untouched directory:\n%s", code, errOut)
	}
	if !strings.Contains(stdout, "3 files checked") {
		t.Errorf("verify did not say how much it checked, so a run that checked nothing looks the same:\n%s", stdout)
	}

	// --against defaults to the directory holding the manifest. Naming it
	// explicitly has to give the same answer.
	if code, _, errOut := run(t, "verify", mf, "--against", out); code != cli.ExitOK {
		t.Errorf("--against on the same directory gave %d:\n%s", code, errOut)
	}
}

func TestVerifyCatchesAMissingAnExtraAndAChangedFile(t *testing.T) {
	cases := []struct {
		name   string
		break_ func(t *testing.T, out string) string
		kind   string
	}{
		{
			name: "a file went missing",
			break_: func(t *testing.T, out string) string {
				victim := firstGenerated(t, out)
				if err := os.Remove(filepath.Join(out, victim)); err != nil {
					t.Fatalf("removing: %v", err)
				}
				return victim
			},
			kind: "missing",
		},
		{
			name: "a file nobody asked for appeared",
			break_: func(t *testing.T, out string) string {
				if err := os.WriteFile(filepath.Join(out, "stowaway.txt"), []byte("x"), 0o644); err != nil {
					t.Fatalf("writing: %v", err)
				}
				return "stowaway.txt"
			},
			kind: "extra",
		},
		{
			name: "the content changed but the length did not",
			break_: func(t *testing.T, out string) string {
				victim := firstGenerated(t, out)
				full := filepath.Join(out, victim)
				body, err := os.ReadFile(full)
				if err != nil {
					t.Fatalf("reading: %v", err)
				}
				// Same length, different bytes. A check on size alone would
				// call this sound, which is the whole reason the hash is
				// recorded.
				body[0] ^= 0xFF
				if err := os.WriteFile(full, body, 0o644); err != nil {
					t.Fatalf("writing: %v", err)
				}
				return victim
			},
			kind: "wrong-hash",
		},
		{
			name: "the file was truncated",
			break_: func(t *testing.T, out string) string {
				victim := firstGenerated(t, out)
				full := filepath.Join(out, victim)
				body, err := os.ReadFile(full)
				if err != nil {
					t.Fatalf("reading: %v", err)
				}
				if err := os.WriteFile(full, body[:len(body)-1], 0o644); err != nil {
					t.Fatalf("writing: %v", err)
				}
				return victim
			},
			kind: "wrong-size",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, mf := generated(t)
			victim := c.break_(t, out)

			code, stdout, errOut := run(t, "verify", mf)
			if code != cli.ExitVerify {
				t.Fatalf("verify gave %d, expected %d - the difference went unnoticed", code, cli.ExitVerify)
			}
			if stdout != "" {
				t.Errorf("a failed verify wrote to stdout:\n%s", stdout)
			}
			if !strings.Contains(errOut, c.kind) {
				t.Errorf("the report does not name the kind of difference %q:\n%s", c.kind, errOut)
			}
			if !strings.Contains(errOut, victim) {
				t.Errorf("the report does not name the file %q, so nobody knows what to look at:\n%s", victim, errOut)
			}
		})
	}
}

// The manifest normally sits in the directory it describes. Reporting it as a
// file nobody asked for would make the most obvious invocation fail on the
// tool's own output.
func TestVerifyDoesNotReportTheManifestAsAnExtraFile(t *testing.T) {
	_, mf := generated(t)
	code, _, errOut := run(t, "verify", mf)
	if code != cli.ExitOK {
		t.Fatalf("verify gave %d on its own output:\n%s", code, errOut)
	}
}

// A manifest describing nothing is neither a match nor a mismatch, and calling
// it a match invites somebody to trust a run that never produced anything.
func TestVerifyOnAManifestClaimingNoFilesSaysThereWasNothingToCheck(t *testing.T) {
	dir := t.TempDir()
	mf := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(mf, []byte(`{"manifest_version":"1.0","files":[]}`), 0o644); err != nil {
		t.Fatalf("writing: %v", err)
	}
	code, stdout, errOut := run(t, "verify", mf)
	if code != cli.ExitOK {
		t.Fatalf("exit %d, expected %d:\n%s", code, cli.ExitOK, errOut)
	}
	if strings.Contains(stdout, "matches") {
		t.Errorf("verify called an empty manifest a match:\n%s", stdout)
	}
	if !strings.Contains(errOut, "nothing to check") {
		t.Errorf("verify did not say there was nothing to check:\n%s", errOut)
	}
}

// A directory that cannot be read is a different failure from a directory that
// disagrees, and CI has to tell them apart - one is "fix the path", the other
// is "your files changed".
func TestVerifyAgainstAMissingDirectoryIsAReadFailureNotAMismatch(t *testing.T) {
	_, mf := generated(t)
	code, _, errOut := run(t, "verify", mf, "--against", filepath.Join(t.TempDir(), "nope"))
	if code != cli.ExitIO {
		t.Errorf("exit %d, expected %d - a missing directory reported as a content mismatch sends the reader to the wrong place", code, cli.ExitIO)
	}
	if !strings.Contains(errOut, "cannot read the directory") {
		t.Errorf("the message does not say what went wrong:\n%s", errOut)
	}
}

// A manifest from a future schema describes fields this build does not know.
// Acting on the half we recognise is how verify ends up calling a directory
// sound on the strength of the part it could read.
func TestVerifyRefusesAManifestFromASchemaItCannotRead(t *testing.T) {
	dir := t.TempDir()
	mf := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(mf, []byte(`{"manifest_version":"9.0","files":[]}`), 0o644); err != nil {
		t.Fatalf("writing: %v", err)
	}
	code, stdout, errOut := run(t, "verify", mf)
	if code != cli.ExitIO {
		t.Errorf("exit %d, expected %d", code, cli.ExitIO)
	}
	if stdout != "" {
		t.Errorf("a refused verify wrote to stdout:\n%s", stdout)
	}
	if !strings.Contains(errOut, "9.0") {
		t.Errorf("the message does not name the version it found:\n%s", errOut)
	}
}

// --json is what CI reads. It has to carry the differences, and a mismatch
// still must not put anything on stdout.
func TestVerifyJSONCarriesTheDifferencesAndKeepsStdoutClean(t *testing.T) {
	out, mf := generated(t)

	code, stdout, _ := run(t, "verify", mf, "--json")
	if code != cli.ExitOK {
		t.Fatalf("verify --json gave %d on an untouched directory", code)
	}
	var ok struct {
		Matched     bool `json:"matched"`
		Checked     int  `json:"checked"`
		Differences []struct {
			Kind string `json:"kind"`
			Path string `json:"path"`
		} `json:"differences"`
	}
	if err := json.Unmarshal([]byte(stdout), &ok); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if !ok.Matched || ok.Checked != 3 {
		t.Errorf("matched=%v checked=%d, expected true and 3", ok.Matched, ok.Checked)
	}

	victim := firstGenerated(t, out)
	if err := os.Remove(filepath.Join(out, victim)); err != nil {
		t.Fatalf("removing: %v", err)
	}

	code, stdout, errOut := run(t, "verify", mf, "--json")
	if code != cli.ExitVerify {
		t.Fatalf("exit %d, expected %d", code, cli.ExitVerify)
	}
	if stdout != "" {
		t.Errorf("a failed verify wrote to stdout, so a consumer of the pipe gets half an answer:\n%s", stdout)
	}
	var bad struct {
		Matched     bool `json:"matched"`
		Differences []struct {
			Kind string `json:"kind"`
			Path string `json:"path"`
		} `json:"differences"`
	}
	if err := json.Unmarshal([]byte(errOut), &bad); err != nil {
		t.Fatalf("the report is not JSON: %v\n%s", err, errOut)
	}
	if bad.Matched {
		t.Error("the report calls a directory with a missing file a match")
	}
	if len(bad.Differences) != 1 || bad.Differences[0].Kind != "missing" || bad.Differences[0].Path != victim {
		t.Errorf("the report does not carry the difference: %+v", bad.Differences)
	}
}

// An entry the run reported as not produced is not a file to go looking for.
//
// Reporting it missing would turn the manifest's own honesty into a false
// alarm: the run said "this one did not happen", and verify would answer "then
// it is missing". The same goes for an entry that was never meant to reach the
// disk. Both would make every partial run fail verification for no reason.
func TestVerifyDoesNotGoLookingForFilesTheRunSaidItNeverWrote(t *testing.T) {
	dir := t.TempDir()

	body := []byte("this one was really written\n")
	if err := os.WriteFile(filepath.Join(dir, "real.txt"), body, 0o644); err != nil {
		t.Fatalf("writing: %v", err)
	}
	sum := sha256.Sum256(body)

	mf := filepath.Join(dir, "manifest.json")
	doc := fmt.Sprintf(`{
  "manifest_version": "1.0",
  "files": [
    {"path":"real.txt","name":"real.txt","materialized":true,"bytes":%d,"hashes":{"sha256":"%s"}},
    {"path":"never_written.txt","name":"never_written.txt","materialized":true,"bytes":10,
     "hashes":{"sha256":""},"failed":true,"error":"the disk filled up"},
    {"path":"not_on_disk.txt","name":"not_on_disk.txt","materialized":false,"bytes":10,
     "hashes":{"sha256":""}}
  ]
}`, len(body), hex.EncodeToString(sum[:]))
	if err := os.WriteFile(mf, []byte(doc), 0o644); err != nil {
		t.Fatalf("writing the manifest: %v", err)
	}

	code, stdout, errOut := run(t, "verify", mf)
	if code != cli.ExitOK {
		t.Fatalf("verify gave %d - a run that honestly reported a failure now fails verification:\n%s", code, errOut)
	}
	// One file counted, not three. A count of three would mean the rule was
	// applied to the report and not to the search.
	if !strings.Contains(stdout, "1 file checked") {
		t.Errorf("verify counted entries it should not have claimed:\n%s", stdout)
	}
}

// A command takes its file before or after the flags. docs/CLI.md writes the
// path first, and somebody who has just typed "--check" first expects that to
// work too. The flag package on its own stops at the first non flag argument
// and turns the documented form into a usage error.
func TestAPathIsAcceptedBeforeOrAfterTheFlags(t *testing.T) {
	out, mf := generated(t)

	if code, _, errOut := run(t, "verify", mf, "--against", out); code != cli.ExitOK {
		t.Errorf("path first gave %d - that is the form docs/CLI.md documents:\n%s", code, errOut)
	}
	if code, _, errOut := run(t, "verify", "--against", out, mf); code != cli.ExitOK {
		t.Errorf("flags first gave %d:\n%s", code, errOut)
	}

	dir := t.TempDir()
	recipe := writeRecipe(t, dir, "version: 1\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\n")
	if code, _, errOut := run(t, "recipe", "fmt", recipe, "--check"); code != cli.ExitOK {
		t.Errorf("recipe fmt with the path first gave %d:\n%s", code, errOut)
	}
	if code, _, errOut := run(t, "recipe", "fmt", "--check", recipe); code != cli.ExitOK {
		t.Errorf("recipe fmt with the flag first gave %d:\n%s", code, errOut)
	}

	// Two paths, or none, is a real mistake and still has to be refused.
	if code, _, _ := run(t, "verify", mf, mf); code != cli.ExitUsage {
		t.Error("two manifests were accepted, so a typo turns into a run against the wrong file")
	}
	if code, _, _ := run(t, "verify", "--against", out); code != cli.ExitUsage {
		t.Error("verify with no manifest was accepted")
	}
}

// CI reads exit codes and JSON, and nothing else. A command whose report is
// only prose forces a script to parse sentences, which breaks the first time
// somebody improves the wording.
//
// The rule from docs/CLI.md section 2 holds here too: a run that ends non zero
// puts nothing on stdout, so its report goes to stderr.
func TestCleanupAndValidateReportAsJSON(t *testing.T) {
	t.Run("cleanup preview names what it would do", func(t *testing.T) {
		out, mf := generated(t)
		withBystander(t, out)

		code, stdout, _ := run(t, "cleanup", mf, "--json")
		if code != cli.ExitOK {
			t.Fatalf("exit %d", code)
		}
		var r struct {
			Applied     bool `json:"applied"`
			WouldRemove int  `json:"would_remove"`
			Files       []struct {
				Path   string `json:"path"`
				Action string `json:"action"`
				State  string `json:"state"`
			} `json:"files"`
		}
		if err := json.Unmarshal([]byte(stdout), &r); err != nil {
			t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
		}
		if r.Applied {
			t.Error("a preview reports applied=true, so a script cannot tell it from a real run")
		}
		if r.WouldRemove != 3 || len(r.Files) != 3 {
			t.Errorf("would_remove=%d over %d files, expected 3 and 3", r.WouldRemove, len(r.Files))
		}
		for _, f := range r.Files {
			if f.Action != "would-remove" || f.State == "" {
				t.Errorf("entry %+v does not say what would happen and what was found", f)
			}
			if f.Path == bystander {
				t.Error("the preview lists a file that is in nobody's manifest")
			}
		}

		// Nothing may have been deleted by asking.
		bystanderSurvives(t, out)
		if _, err := os.Stat(filepath.Join(out, firstGenerated(t, out))); err != nil {
			t.Error("the preview removed a file")
		}
	})

	t.Run("cleanup that leaves something behind reports on stderr", func(t *testing.T) {
		out, mf := generated(t)
		victim := firstGenerated(t, out)
		full := filepath.Join(out, victim)
		body, err := os.ReadFile(full)
		if err != nil {
			t.Fatalf("reading: %v", err)
		}
		body[0] ^= 0xFF
		if err := os.WriteFile(full, body, 0o644); err != nil {
			t.Fatalf("writing: %v", err)
		}

		code, stdout, errOut := run(t, "cleanup", mf, "--yes", "--json")
		if code != cli.ExitIO {
			t.Fatalf("exit %d, expected %d", code, cli.ExitIO)
		}
		if stdout != "" {
			t.Errorf("a run that ended non zero wrote to stdout:\n%s", stdout)
		}
		var r struct {
			Applied bool `json:"applied"`
			Removed int  `json:"removed"`
			Kept    int  `json:"kept"`
		}
		if err := json.Unmarshal([]byte(errOut), &r); err != nil {
			t.Fatalf("the report is not JSON: %v\n%s", err, errOut)
		}
		if !r.Applied || r.Removed != 2 || r.Kept != 1 {
			t.Errorf("applied=%v removed=%d kept=%d, expected true, 2 and 1", r.Applied, r.Removed, r.Kept)
		}
	})

	t.Run("validate reports every problem separately", func(t *testing.T) {
		// Three faults at once. RC7 already reports them together, and the
		// point of JSON is that a script does not have to split the sentence
		// back apart.
		path := writeRecipe(t, t.TempDir(), `version: 1
targets:
  - id: a
    format: txt
  - id: a
    format: nosuchformat
    size: 1kb
`)
		code, stdout, errOut := run(t, "validate", path, "--json")
		if code != cli.ExitRecipe {
			t.Fatalf("exit %d, expected %d", code, cli.ExitRecipe)
		}
		if stdout != "" {
			t.Errorf("a failed validate wrote to stdout:\n%s", stdout)
		}
		var r struct {
			Valid    bool `json:"valid"`
			Problems []struct {
				What string `json:"what"`
				Why  string `json:"why"`
				Fix  string `json:"fix"`
			} `json:"problems"`
		}
		if err := json.Unmarshal([]byte(errOut), &r); err != nil {
			t.Fatalf("the report is not JSON: %v\n%s", err, errOut)
		}
		if r.Valid {
			t.Error("a recipe with faults is reported as valid")
		}
		if len(r.Problems) < 2 {
			t.Errorf("got %d problems, expected the faults reported separately: %+v", len(r.Problems), r.Problems)
		}
		for _, p := range r.Problems {
			if p.What == "" || p.Why == "" || p.Fix == "" {
				t.Errorf("problem %+v is missing one of what, why or fix - the three parts every refusal here carries", p)
			}
		}
	})

	t.Run("a valid recipe reports on stdout", func(t *testing.T) {
		path := writeRecipe(t, t.TempDir(), "version: 1\ntargets:\n  - id: a\n    format: txt\n    count: 3\n    size: 1kb\n")
		code, stdout, _ := run(t, "validate", path, "--json")
		if code != cli.ExitOK {
			t.Fatalf("exit %d", code)
		}
		var r struct {
			Valid      bool   `json:"valid"`
			Files      int    `json:"files"`
			RecipeHash string `json:"recipe_hash"`
		}
		if err := json.Unmarshal([]byte(stdout), &r); err != nil {
			t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
		}
		if !r.Valid || r.Files != 3 || !strings.HasPrefix(r.RecipeHash, "sha256:") {
			t.Errorf("valid=%v files=%d hash=%q", r.Valid, r.Files, r.RecipeHash)
		}
	})
}

// firstGenerated is the name of one file the run produced, taken from the
// directory rather than guessed from the template.
func firstGenerated(t *testing.T, out string) string {
	t.Helper()
	entries, err := os.ReadDir(out)
	if err != nil {
		t.Fatalf("reading the output directory: %v", err)
	}
	for _, e := range entries {
		if !e.IsDir() && e.Name() != "manifest.json" {
			return e.Name()
		}
	}
	t.Fatal("the run produced no files, so this guard would prove nothing")
	return ""
}

// respellManifestPaths rewrites every claimed path through respell, leaving the
// rest of the manifest as it was.
func respellManifestPaths(t *testing.T, mf string, respell func(string) string) {
	t.Helper()
	raw, err := os.ReadFile(mf)
	if err != nil {
		t.Fatalf("reading the manifest: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("reading the manifest as JSON: %v", err)
	}
	files, ok := doc["files"].([]any)
	if !ok || len(files) == 0 {
		t.Fatal("the manifest claims no files, so this guard would prove nothing")
	}
	for _, entry := range files {
		row, ok := entry.(map[string]any)
		if !ok {
			t.Fatalf("a manifest entry is %T, not an object", entry)
		}
		was, ok := row["path"].(string)
		if !ok {
			t.Fatalf("a manifest entry has no path to respell: %v", row)
		}
		row["path"] = respell(was)
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("writing the manifest back: %v", err)
	}
	if err := os.WriteFile(mf, out, 0o644); err != nil {
		t.Fatalf("saving the manifest: %v", err)
	}
}

// A manifest naming a file the long way round names the same file.
//
// Both spellings pass core.ContainmentProblem, which asks only whether the path
// lands inside the directory - so a manifest written this way is one this tool
// accepts everywhere else. Compared as text they matched nothing, and verify
// answered "extra a.txt" about a file its own manifest listed.
//
// That wording is why this matters more than a spelling nicety. A mismatch
// usually shows as a pair, missing and extra, which reads as a name somebody
// got wrong. One "extra" on its own reads as a directory somebody polluted,
// and that is what a script watching a restore or a sync would act on.
//
// Found by an outside review of the whole tree on 2026-08-23 and measured here
// before it was believed. docs/CODE-REVIEW-2026-08-23.md section 3.7.
func TestVerifyMatchesAPathSpelledTheLongWayRound(t *testing.T) {
	for _, c := range []struct {
		about   string
		respell func(string) string
	}{
		{"a leading dot", func(p string) string { return "./" + p }},
		{"a step down and back up", func(p string) string { return "sub/../" + p }},
	} {
		t.Run(c.about, func(t *testing.T) {
			out, mf := generated(t)
			if err := os.MkdirAll(filepath.Join(out, "sub"), 0o755); err != nil {
				t.Fatalf("making the subdirectory the spelling steps through: %v", err)
			}
			respellManifestPaths(t, mf, c.respell)

			code, stdout, errOut := run(t, "verify", mf)
			if code != cli.ExitOK {
				t.Fatalf("verify gave %d for a manifest naming the files it describes:\n%s", code, errOut)
			}
			if !strings.Contains(stdout, "3 files checked") {
				t.Errorf("verify did not say it checked all three:\n%s", stdout)
			}
		})
	}

	// The other half, and without it the guard above is satisfied by a verify
	// that stopped comparing at all.
	t.Run("a file nobody claimed is still extra", func(t *testing.T) {
		out, mf := generated(t)
		respellManifestPaths(t, mf, func(p string) string { return "./" + p })
		if err := os.WriteFile(filepath.Join(out, "intruder.txt"), []byte("not ours"), 0o644); err != nil {
			t.Fatalf("planting a file: %v", err)
		}

		code, _, errOut := run(t, "verify", mf)
		if code == cli.ExitOK {
			t.Fatal("verify called a directory with an unclaimed file in it a match")
		}
		if !strings.Contains(errOut, "intruder.txt") {
			t.Errorf("verify did not name the unclaimed file:\n%s", errOut)
		}
	})
}
