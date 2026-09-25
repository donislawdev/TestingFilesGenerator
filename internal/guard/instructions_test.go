package guard

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// The instructions beside a manifest say what every file is for, and these
// guards hold the rules they were built on (docs/PRESET-INSTRUCTIONS-2026-09-25.md).

// instructionsName is what the instructions of a run with the default
// manifest are called.
var instructionsName = manifest.InstructionsName("manifest.json")

// Every file of every preset says what it is for, in words a person can read.
//
// The owner's request of 2026-09-25 was exactly this: a preset writes dozens of
// files and nothing said what any of them was for. A target left without a
// purpose would be a line in the instructions saying no purpose was given, in
// a set this tool built itself. Asked of the ejected recipe rather than of the
// preset's tables, because that is what reaches the manifest - and PR5 says the
// two are the same thing.
//
// D17 as well, because the sentences are text a person reads: no semicolon and
// no long dash.
func TestEveryFileOfEveryPresetSaysWhatItIsFor(t *testing.T) {
	for _, id := range preset.IDs() {
		expanded, err := preset.Expand(id, preset.Args{})
		if err != nil {
			t.Fatalf("%s refused its own defaults: %v", id, err)
		}
		rec, err := recipe.Parse(expanded.Source, id)
		if err != nil {
			t.Fatalf("the recipe %s ejects does not read back: %v", id, err)
		}
		if len(rec.Targets) == 0 {
			t.Fatalf("%s has no targets, so nothing was asked of it", id)
		}
		for _, target := range rec.Targets {
			switch p := target.Purpose; {
			case strings.TrimSpace(p) == "":
				t.Errorf("%s: target %q says nothing about what its files are for", id, target.ID)
			case strings.ContainsAny(p, ";"+string(rune(0x2014))+string(rune(0x2013))):
				t.Errorf("%s: the purpose of %q carries a semicolon or a long dash (D17): %s", id, target.ID, p)
			}
		}
	}
}

// A name reaches the reader of the instructions as text, whatever it holds.
//
// filename-handling writes names built to be read as something else - a
// command in backticks, a right to left override, a space at each end - and the
// instructions list every one of them. A name that Markdown reads as markup
// shows the reader a different file, which is the one thing the instructions
// exist to prevent.
func TestTheInstructionsShowEveryNameAsItIsAndNothingElse(t *testing.T) {
	override := string(rune(0x202E))
	names := []string{"a`b.txt", "`start.txt", " both ends ", "photo" + override + "gpj.txt", "plain.txt"}
	m := manifest.New("tfg", "0.0.0", "run", "tfg", 0, "os", "arch")
	for _, name := range names {
		m.Add(manifest.File{Name: name, Path: name, Materialized: true, Format: "txt", Bytes: 1,
			TargetID: "t" + name, Group: "names", Purpose: "line one\nline two",
			Expected: manifest.Expected{Outcome: "accept"}})
	}
	md := string(m.Instructions("manifest.json"))
	for _, want := range []string{"``a`b.txt``", "`` `start.txt ``", "`  both ends  `", "`" + core.Shown("photo"+override+"gpj.txt") + "`", "`plain.txt`"} {
		if !strings.Contains(md, want) {
			t.Errorf("the instructions do not show a name as %s:\n%s", want, md)
		}
	}
	if strings.Contains(md, override) {
		t.Error("a right to left override reached the instructions raw, so the name is shown as another one")
	}
	if !strings.Contains(md, "  line one line two\n") {
		t.Errorf("a purpose written across two lines broke the list rather than standing on one line:\n%s", md)
	}
}

// Many files of one target read as one entry, and a file that failed as its own.
func TestTheInstructionsFoldARunOfOneTargetIntoOneEntry(t *testing.T) {
	m := manifest.New("tfg", "0.0.0", "run", "tfg", 0, "os", "arch")
	for _, name := range []string{"bulk_0001.jpg", "bulk_0002.jpg", "bulk_0003.jpg"} {
		m.Add(manifest.File{Name: name, Path: name, Materialized: true, Format: "jpg", Bytes: 10,
			TargetID: "bulk", Purpose: "Many files at once.", Expected: manifest.Expected{Outcome: "accept"}})
	}
	m.Add(manifest.File{Name: "bulk_0004.jpg", Path: "bulk_0004.jpg", Format: "jpg", TargetID: "bulk",
		Failed: true, Error: "the disk said no", Purpose: "Many files at once."})
	md := string(m.Instructions("manifest.json"))
	if !strings.Contains(md, "- 3 files, `bulk_0001.jpg` to `bulk_0003.jpg` - jpg, 30 B in all.") {
		t.Errorf("three files of one target were not one entry:\n%s", md)
	}
	if !strings.Contains(md, "- `bulk_0004.jpg`") || !strings.Contains(md, "Not written: the disk said no") {
		t.Errorf("a file that failed was folded in with the rest rather than said on its own:\n%s", md)
	}
	if got := strings.Count(md, "Many files at once."); got != 2 {
		t.Errorf("the purpose is said %d times, and once for the run and once for the failed file was expected", got)
	}
}

// A recipe with a purpose leaves instructions, verify does not take them for
// something nobody asked for, and they go with the manifest and not before it.
func TestTheInstructionsGoWhereTheManifestGoes(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	generateFrom(t, dir, out, "purpose: A file for the guard.\n")
	if _, err := os.Stat(filepath.Join(out, instructionsName)); err != nil {
		t.Fatalf("a recipe with a purpose left no instructions: %v", err)
	}
	if got := recordedInstructions(t, out); got != instructionsName {
		t.Fatalf("the manifest names %q as its instructions, and %q was written", got, instructionsName)
	}
	if code, said := tfg("verify", filepath.Join(out, "manifest.json")); code != cli.ExitOK {
		t.Fatalf("verify ended %d on a directory the run just wrote, so it took the instructions for something else:\n%s", code, said)
	}
	if code, said := tfg("cleanup", filepath.Join(out, "manifest.json"), "--yes"); code != cli.ExitOK {
		t.Fatalf("cleanup ended %d: %s", code, said)
	}
	if _, err := os.Stat(filepath.Join(out, instructionsName)); err != nil {
		t.Fatal("cleanup without --with-manifest took the instructions, which describe the manifest that stayed")
	}
	if code, said := tfg("cleanup", filepath.Join(out, "manifest.json"), "--yes", "--with-manifest"); code != cli.ExitOK {
		t.Fatalf("cleanup --with-manifest ended %d: %s", code, said)
	}
	if left := namesIn(t, out); len(left) != 0 {
		t.Errorf("cleanup --with-manifest left %v behind", left)
	}
}

// A run nobody explained writes no instructions and names none.
func TestARunWithNoPurposeWritesNoInstructions(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	generateFrom(t, dir, out, "")
	if _, err := os.Stat(filepath.Join(out, instructionsName)); err == nil {
		t.Error("a run with no purpose anywhere wrote instructions that could only say nothing")
	}
	if got := recordedInstructions(t, out); got != "" {
		t.Errorf("the manifest names instructions %q that nobody wrote", got)
	}
}

// Instructions an earlier run left are never written over, and a target
// cannot take their name.
func TestTheNameOfTheInstructionsIsHeldBeforeAnyFile(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, instructionsName), []byte("an earlier run"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, said := tfg("generate", writeOneTargetRecipe(t, dir, "purpose: A file for the guard.\n"), "--out", out)
	if code != cli.ExitIO || !strings.Contains(said, instructionsName) {
		t.Errorf("instructions already there ended %d rather than %d, saying:\n%s", code, cli.ExitIO, said)
	}
	if left := namesIn(t, out); len(left) != 1 {
		t.Errorf("the refused run wrote into the directory anyway: %v", left)
	}

	taken := filepath.Join(dir, "taken")
	code, said = tfg("generate", writeOneTargetRecipe(t, dir, "purpose: A file for the guard.\n    name: "+instructionsName+"\n"), "--out", taken)
	if code == cli.ExitOK || !strings.Contains(said, "instructions beside its manifest") {
		t.Errorf("a target named like the instructions ended %d, saying:\n%s", code, said)
	}
}

// A manifest naming anything but a plain name beside it as its instructions is
// not acted on, because cleanup --with-manifest removes that file.
func TestAManifestCannotSendCleanupAfterAFileOfSomebodyElses(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	generateFrom(t, dir, out, "purpose: A file for the guard.\n")
	victim := filepath.Join(dir, "victim.instructions.md")
	if err := os.WriteFile(victim, []byte("not the run's"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(out, "manifest.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doctored := bytes.Replace(raw, []byte(`"instructions": "`+instructionsName+`"`), []byte(`"instructions": "../victim.instructions.md"`), 1)
	if bytes.Equal(doctored, raw) {
		t.Fatal("the manifest carries no instructions field to change, so this guard asked nothing")
	}
	if err := os.WriteFile(path, doctored, 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _ := tfg("cleanup", path, "--yes", "--with-manifest"); code == cli.ExitOK {
		t.Error("cleanup acted on a manifest that points its instructions outside the directory")
	}
	if _, err := os.Stat(victim); err != nil {
		t.Error("cleanup removed a file outside the directory the manifest describes")
	}
}

// Instructions that cannot be written are said, the run stands, and nothing of
// them is left behind or named.
func TestInstructionsThatCannotBeWrittenAreSaidAndNotPretended(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	// A directory where the instructions are written first, under their
	// temporary name, so the one write that is not a test file fails.
	blocker := core.SiblingPath(filepath.Join(out, instructionsName), core.WritingMarker)
	if err := os.MkdirAll(blocker, 0o755); err != nil {
		t.Fatal(err)
	}
	code, said := tfg("generate", writeOneTargetRecipe(t, dir, "purpose: A file for the guard.\n"), "--out", out)
	if code != cli.ExitOK {
		t.Fatalf("the run failed over its instructions, which are words about files that were written: exit %d\n%s", code, said)
	}
	if !strings.Contains(said, "cannot write the instructions") {
		t.Errorf("instructions that were not written went unsaid:\n%s", said)
	}
	if _, err := os.Stat(filepath.Join(out, instructionsName)); err == nil {
		t.Error("the empty claim on the name of the instructions was left behind")
	}
	if got := recordedInstructions(t, out); got != "" {
		t.Errorf("the manifest names instructions %q that were never written", got)
	}
}

// The preset screen offers the instructions its run wrote, and opens that file.
func TestTheWindowOpensTheInstructionsItsRunWrote(t *testing.T) {
	dir := t.TempDir()
	host, content := presetScreen(t)
	choosePreset(t, content, "filename-handling")
	fill(t, content, text.FieldOutputDir(), dir)
	if shownButton(content, text.ButtonOpenInstructions()) != nil {
		t.Fatal("the button is on the screen before anything was written")
	}
	press(t, content, text.ButtonGenerate())
	waitForManifest(t, host, dir)
	join(host)

	button := shownButton(content, text.ButtonOpenInstructions())
	if button == nil {
		t.Fatal("the run wrote instructions and the window offers no way to open them")
	}
	button.OnTapped()
	if want := filepath.Join(dir, instructionsName); host.file != want || host.fileCount != 1 {
		t.Errorf("the button opened %q (%d times), and the instructions are %q", host.file, host.fileCount, want)
	}
}

// generateFrom runs a one-target recipe into out, with extra lines under the
// target, and fails the guard if the run does not succeed.
func generateFrom(t *testing.T, dir, out, extra string) {
	t.Helper()
	if code, said := tfg("generate", writeOneTargetRecipe(t, dir, extra), "--out", out); code != cli.ExitOK {
		t.Fatalf("the run ended %d:\n%s", code, said)
	}
}

// writeOneTargetRecipe writes a recipe of one small text file, with extra lines under
// the target, and gives its path.
func writeOneTargetRecipe(t *testing.T, dir, extra string) string {
	t.Helper()
	path := filepath.Join(dir, "recipe.yaml")
	src := "version: 1\ntargets:\n  - id: one\n    format: txt\n    size: 1kb\n"
	if extra != "" {
		src += "    " + extra
	}
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// tfg runs the command line and gives its exit code and everything it said.
func tfg(args ...string) (int, string) {
	var out, errOut bytes.Buffer
	code := cli.Run(context.Background(), args, &out, &errOut)
	return code, out.String() + errOut.String()
}

// recordedInstructions is what the manifest in dir names as its instructions.
func recordedInstructions(t *testing.T, dir string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("the run left no manifest: %v", err)
	}
	var m struct {
		Run struct {
			Instructions string `json:"instructions"`
		} `json:"run"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m.Run.Instructions
}
