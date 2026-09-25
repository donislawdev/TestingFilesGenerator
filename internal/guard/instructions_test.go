package guard

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
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

// What cleanup --with-manifest takes besides the files is said before it is
// taken, and again once it has been. Until 2026-09-25 the preview named only
// the files the manifest lists, and --yes then took the manifest and the
// instructions beside it as well - said by CodeRabbit on #140, measured true.
func TestCleanupNamesTheRecordBeforeItTakesIt(t *testing.T) {
	out, mf, instructions := purposeRun(t)
	_, said, _ := run(t, "cleanup", mf, "--with-manifest")
	for _, want := range []string{"  remove " + mf + "\n", "  remove " + instructions + "\n"} {
		if !strings.Contains(said, want) {
			t.Errorf("the preview does not say %q, and --yes takes it:\n%s", strings.TrimSpace(want), said)
		}
	}
	_, raw, _ := run(t, "cleanup", mf, "--with-manifest", "--json")
	preview := decodeCleanup(t, raw)
	if got := recordActions(preview); !slices.Equal(got, []string{"manifest.json=would-remove", instructionsName + "=would-remove"}) {
		t.Errorf("the preview's record is %v", got)
	}
	if len(preview.Files) != 1 || preview.WouldRemove != 1 {
		t.Errorf("files and would_remove count %d and %d, and the manifest lists one file - the record is not theirs to count",
			len(preview.Files), preview.WouldRemove)
	}
	_, raw, _ = run(t, "cleanup", mf, "--json")
	if got := recordActions(decodeCleanup(t, raw)); len(got) != 0 {
		t.Errorf("without --with-manifest the report names a record nothing will touch: %v", got)
	}

	code, said, errOut := run(t, "cleanup", mf, "--with-manifest", "--yes")
	if code != cli.ExitOK {
		t.Fatalf("cleanup --with-manifest --yes ended %d: %s", code, errOut)
	}
	for _, want := range []string{"  removed " + mf + "\n", "  removed " + instructions + "\n"} {
		if !strings.Contains(said, want) {
			t.Errorf("the run does not say %q, and took it:\n%s", strings.TrimSpace(want), said)
		}
	}
	if left := namesIn(t, out); len(left) != 0 {
		t.Errorf("cleanup --with-manifest left %v behind", left)
	}

	_, mf, _ = purposeRun(t)
	code, raw, errOut = run(t, "cleanup", mf, "--with-manifest", "--yes", "--json")
	if code != cli.ExitOK {
		t.Fatalf("cleanup --with-manifest --yes --json ended %d: %s", code, errOut)
	}
	if got := recordActions(decodeCleanup(t, raw)); !slices.Equal(got, []string{"manifest.json=removed", instructionsName + "=removed"}) {
		t.Errorf("the report's record is %v", got)
	}
}

// The record stays while a file it lists stays, and the manifest stays while
// the instructions it names do - and the report says which and why, in the
// preview and in the run.
func TestCleanupSaysWhyTheRecordStays(t *testing.T) {
	out, mf, _ := purposeRun(t)
	_, raw, _ := run(t, "cleanup", mf, "--json")
	listed := decodeCleanup(t, raw).Files
	if len(listed) != 1 {
		t.Fatalf("the manifest lists %d files, and one was asked for", len(listed))
	}
	changed := filepath.Join(out, listed[0].Path)
	body, err := os.ReadFile(changed)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(changed, append(body, 'x'), 0o644); err != nil {
		t.Fatal(err)
	}
	_, raw, _ = run(t, "cleanup", mf, "--with-manifest", "--json")
	preview := decodeCleanup(t, raw)
	if got := recordActions(preview); !slices.Equal(got, []string{"manifest.json=would-keep", instructionsName + "=would-keep"}) {
		t.Errorf("with a changed file the preview's record is %v", got)
	} else if !strings.Contains(preview.Record[0].Reason, "only record") {
		t.Errorf("the preview does not say why the manifest would stay: %q", preview.Record[0].Reason)
	}
	code, _, errOut := run(t, "cleanup", mf, "--with-manifest", "--yes", "--json")
	if got := recordActions(decodeCleanup(t, fromBrace(errOut))); code != cli.ExitIO ||
		!slices.Equal(got, []string{"manifest.json=kept", instructionsName + "=kept"}) {
		t.Errorf("with a changed file the run ended %d and its record is %v", code, got)
	}

	// Instructions that will not go: a directory with something in it, which
	// no system removes with a plain remove.
	_, mf, instructions := purposeRun(t)
	if err := os.Remove(instructions); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(instructions, "inside"), 0o755); err != nil {
		t.Fatal(err)
	}
	code, said, errOut := run(t, "cleanup", mf, "--with-manifest", "--yes", "--json")
	if got := recordActions(decodeCleanup(t, fromBrace(errOut))); code != cli.ExitIO || said != "" ||
		!slices.Equal(got, []string{"manifest.json=kept", instructionsName + "=kept"}) {
		t.Errorf("instructions that would not go ended %d, put %q on stdout, and the record is %v", code, said, got)
	}
	if _, err := os.Stat(mf); err != nil {
		t.Error("the manifest went while the instructions it names stayed")
	}
}

// Instructions somebody already deleted are not promised for removal, and do
// not keep a manifest they asked to have removed.
func TestInstructionsAlreadyGoneHoldNothingBack(t *testing.T) {
	out, mf, instructions := purposeRun(t)
	if err := os.Remove(instructions); err != nil {
		t.Fatal(err)
	}
	_, raw, _ := run(t, "cleanup", mf, "--with-manifest", "--json")
	preview := decodeCleanup(t, raw)
	if got := recordActions(preview); !slices.Equal(got, []string{"manifest.json=would-remove", instructionsName + "=would-keep"}) {
		t.Errorf("with the instructions gone the preview's record is %v", got)
	} else if preview.Record[1].Reason != "it is already gone" {
		t.Errorf("the preview keeps the instructions for %q rather than because they are gone", preview.Record[1].Reason)
	}
	code, raw, errOut := run(t, "cleanup", mf, "--with-manifest", "--yes", "--json")
	if code != cli.ExitOK {
		t.Fatalf("instructions already gone ended the run %d: %s", code, errOut)
	}
	if got := recordActions(decodeCleanup(t, raw)); !slices.Equal(got, []string{"manifest.json=removed", instructionsName + "=kept"}) {
		t.Errorf("with the instructions gone the run's record is %v", got)
	}
	if left := namesIn(t, out); len(left) != 0 {
		t.Errorf("cleanup --with-manifest left %v behind", left)
	}
}

// Instructions not named after the manifest cleanup was given are never taken.
// A manifest can be edited, and one pointing at the instructions of another run
// in the same directory had cleanup remove them - said by review on #140,
// measured true. A manifest renamed after its run keeps its instructions the
// same way, and in both cases the report says what stays and why.
func TestCleanupLeavesInstructionsNotNamedAfterItsManifest(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	for _, name := range []string{"run1", "run2"} {
		path := filepath.Join(dir, name+".yaml")
		src := "version: 1\ntargets:\n  - id: " + name + "\n    format: txt\n    size: 1kb\n    purpose: A file for the guard.\n" +
			"output:\n  manifest: " + name + ".json\n"
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		if code, said := tfg("generate", path, "--out", out); code != cli.ExitOK {
			t.Fatalf("%s ended %d:\n%s", name, code, said)
		}
	}
	first := filepath.Join(out, "run1.json")
	raw, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	doctored := bytes.Replace(raw, []byte(`"instructions": "run1.instructions.md"`), []byte(`"instructions": "run2.instructions.md"`), 1)
	if bytes.Equal(doctored, raw) {
		t.Fatal("run1.json names no instructions of its own to change, so this guard asked nothing")
	}
	if err := os.WriteFile(first, doctored, 0o644); err != nil {
		t.Fatal(err)
	}
	_, said, _ := run(t, "cleanup", first, "--with-manifest", "--json")
	if got := recordActions(decodeCleanup(t, said)); !slices.Equal(got, []string{"run1.json=would-remove", "run2.instructions.md=would-keep"}) {
		t.Errorf("the preview's record for a manifest naming another run's instructions is %v", got)
	}
	code, said, errOut := run(t, "cleanup", first, "--with-manifest", "--yes", "--json")
	report := decodeCleanup(t, said)
	if got := recordActions(report); code != cli.ExitOK || !slices.Equal(got, []string{"run1.json=removed", "run2.instructions.md=kept"}) {
		t.Errorf("the run ended %d and its record is %v: %s", code, got, errOut)
	} else if !strings.Contains(report.Record[1].Reason, "not named after this manifest") {
		t.Errorf("the report keeps another run's instructions for %q", report.Record[1].Reason)
	}
	if _, err := os.Stat(filepath.Join(out, "run2.instructions.md")); err != nil {
		t.Error("cleanup removed the instructions of another run because an edited manifest named them")
	}

	out, mf, instructions := purposeRun(t)
	renamed := filepath.Join(out, "old.json")
	if err := os.Rename(mf, renamed); err != nil {
		t.Fatal(err)
	}
	code, said, errOut = run(t, "cleanup", renamed, "--with-manifest", "--yes", "--json")
	if got := recordActions(decodeCleanup(t, said)); code != cli.ExitOK || !slices.Equal(got, []string{"old.json=removed", instructionsName + "=kept"}) {
		t.Errorf("a renamed manifest ended %d and its record is %v: %s", code, got, errOut)
	}
	if _, err := os.Stat(instructions); err != nil {
		t.Error("cleanup took instructions named after a manifest it was not given")
	}
}

// The window says instructions it could not write the way the command line
// does - the path with a character nobody can see written as its escape, in
// the name and in the system's own sentence, which carries the path again.
func TestTheWindowSaysInstructionsItCouldNotWriteWithTheEscape(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out"+rightToLeft+"dir")
	blocker := core.SiblingPath(filepath.Join(out, instructionsName), core.WritingMarker)
	if err := os.MkdirAll(blocker, 0o755); err != nil {
		t.Fatal(err)
	}
	host, content := presetScreen(t)
	choosePreset(t, content, "tabular-import")
	fill(t, content, text.FieldOutputDir(), out)
	press(t, content, text.ButtonGenerate())
	waitForManifest(t, host, out)
	join(host)

	// The box itself holds what was typed, raw, which is right for a box - and
	// it is the only place the raw path may stand. Taken out once rather than
	// everywhere: the message carries the path too, and taking every copy out
	// would take out exactly the raw one this guard is looking for.
	said := everythingSaid(content)
	if n := strings.Count(said, out); n != 1 {
		t.Errorf("the output directory stands raw %d times on the screen, and only its own box may hold it:\n%s", n, said)
	}
	said = strings.Replace(said, out, "", 1)
	if !strings.Contains(said, "instructions could not be saved") {
		t.Fatalf("the window did not say the instructions were not written, so this guard checked nothing:\n%s", said)
	}
	saysNothingUnseen(t, "the window's line about instructions it could not write", said)
}

// purposeRun generates one file with a purpose and gives the output
// directory, the manifest and the instructions beside it.
func purposeRun(t *testing.T) (out, mf, instructions string) {
	t.Helper()
	dir := t.TempDir()
	out = filepath.Join(dir, "out")
	generateFrom(t, dir, out, "purpose: A file for the guard.\n")
	return out, filepath.Join(out, "manifest.json"), filepath.Join(out, instructionsName)
}

// cleanupRecord is the part of a cleanup report these guards read, as a
// script receives it rather than through the type that writes it.
type cleanupRecord struct {
	WouldRemove int `json:"would_remove"`
	Files       []struct {
		Path string `json:"path"`
	} `json:"files"`
	Record []struct {
		Path   string `json:"path"`
		Action string `json:"action"`
		Reason string `json:"reason"`
	} `json:"record"`
}

func decodeCleanup(t *testing.T, raw string) cleanupRecord {
	t.Helper()
	var r cleanupRecord
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatalf("the report is not JSON: %v\n%s", err, raw)
	}
	return r
}

// recordActions is each file of the record by its own name, with what was
// done to it, in the order the report gives them.
func recordActions(r cleanupRecord) []string {
	var out []string
	for _, e := range r.Record {
		out = append(out, filepath.Base(e.Path)+"="+e.Action)
	}
	return out
}

// fromBrace is the JSON report at the end of what a failed run said, after
// the sentences in front of it.
func fromBrace(said string) string {
	if i := strings.Index(said, "{"); i >= 0 {
		return said[i:]
	}
	return said
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
