package guard

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/audit"
	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// A run into a directory that already holds one, told it may take a number
// (engine.Options.NumberIfTaken, docs/RUN-FOLDERS-2026-11-14.md).
//
// What these guards hold, in the order it matters: nothing an earlier run left
// changes, a name an earlier record still lists is never written again while
// that record lies there, the number goes where the owner chose, and the
// manifest is saved under the name the run claimed.

// numberingOptions is what a run that may take a number is started with.
func numberingOptions(dir string) engine.Options {
	return engine.Options{OutDir: dir, Seed: 7741, Command: "test",
		ManifestName: engine.DefaultManifestName, NumberIfTaken: true}
}

// runAndSave plans, runs and saves one run, failing the test on any refusal.
func runAndSave(t *testing.T, targets []engine.Target, opt engine.Options) (*engine.Result, engine.Record) {
	t.Helper()
	planned, err := engine.Plan(targets, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}
	res, err := engine.Run(context.Background(), planned, opt)
	if err != nil {
		t.Fatalf("the run was refused: %v", err)
	}
	rec, err := engine.SaveRecord(res, opt)
	if err != nil {
		t.Fatalf("saving the record: %v", err)
	}
	return res, rec
}

// contentsOf is every file in dir by name, with the hash of what it holds.
func contentsOf(t *testing.T, dir string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("listing %s: %v", dir, err)
	}
	out := map[string]string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		sum := sha256.Sum256(body)
		out[e.Name()] = hex.EncodeToString(sum[:])
	}
	return out
}

// changedFrom names every file of before that is gone or holds something else
// in after.
func changedFrom(before, after map[string]string) []string {
	var changed []string
	for name, sum := range before {
		if after[name] != sum {
			changed = append(changed, name)
		}
	}
	sort.Strings(changed)
	return changed
}

// The earlier run is left as it was, to the byte, and the second run writes
// what an empty directory would have got under numbered names - the same seed
// and the same batch give the same bytes (docs/RUN-FOLDERS-2026-11-14.md 9.1).
func TestARunThatTakesANumberLeavesTheEarlierRunAsItWas(t *testing.T) {
	// The canary: the comparison has to see a file that changed and a file
	// that went, or a green answer below says nothing.
	if got := changedFrom(map[string]string{"a": "1", "b": "2"}, map[string]string{"a": "9"}); len(got) != 2 {
		t.Fatalf("the comparison found %v in a directory where a changed and b went - it cannot see what it guards", got)
	}

	dir := t.TempDir()
	seedDirectory(t, dir)
	before := contentsOf(t, dir)

	res, rec := runAndSave(t, []engine.Target{txtTarget("files", 2, 4096)}, numberingOptions(dir))
	after := contentsOf(t, dir)

	if changed := changedFrom(before, after); len(changed) > 0 {
		t.Fatalf("the numbered run changed or removed %v, which an earlier run wrote", changed)
	}
	if res.Number != 2 {
		t.Fatalf("the run carries number %d, and a directory holding one run gives the next one 2", res.Number)
	}
	if want := filepath.Join(dir, "manifest-2.json"); rec.Manifest != want {
		t.Errorf("the record was saved to %s, and the run's manifest is %s", rec.Manifest, want)
	}
	for _, pair := range [][2]string{{"files_0001.txt", "files-2_0001.txt"}, {"files_0002.txt", "files-2_0002.txt"}} {
		if after[pair[1]] == "" {
			t.Errorf("%s is not in the directory, and it is the name the run's second file of that place takes", pair[1])
			continue
		}
		if after[pair[1]] != after[pair[0]] {
			t.Errorf("%s holds other bytes than %s, and the number was to change the name and nothing else", pair[1], pair[0])
		}
	}
	said := ""
	for _, n := range res.RunNotes {
		said += n.String()
	}
	if !strings.Contains(said, "carries the number 2") {
		t.Errorf("the run took a number and said nothing about it (untouchable rule 6). It said: %q", said)
	}
}

// The case the rule is built around. Somebody deletes an earlier run's files
// and keeps its manifest. The names are free on the disk, and a run writing
// them again would hand its files to that manifest's cleanup.
func TestANameAnEarlierRecordStillListsIsNotWrittenAgain(t *testing.T) {
	dir := t.TempDir()
	seedDirectory(t, dir)
	for _, name := range []string{"files_0001.txt", "files_0002.txt"} {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}

	res, _ := runAndSave(t, []engine.Target{txtTarget("files", 2, 4096)}, numberingOptions(dir))
	if res.Number != 2 {
		t.Errorf("the run carries number %d, and the manifest of the first run still lies in the directory", res.Number)
	}

	claims, err := manifest.ReadClaims(context.Background(), manifestOf(dir), 0)
	if err != nil {
		t.Fatalf("reading the first manifest: %v", err)
	}
	cands, err := audit.Inspect(context.Background(), dir, claims)
	if err != nil {
		t.Fatalf("asking what cleanup of the first run would do: %v", err)
	}
	for _, c := range cands {
		if c.Removable(true) {
			t.Errorf("cleanup of the first run would remove %s, a file the second run wrote", c.Path)
		}
	}
}

// numberedNames runs targets as a preview into a directory holding an earlier
// run, and answers with the names the run would write.
func numberedNames(t *testing.T, targets []engine.Target) []string {
	t.Helper()
	dir := t.TempDir()
	seedDirectory(t, dir)
	opt := numberingOptions(dir)
	opt.DryRun = true
	planned, err := engine.Plan(targets, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}
	res, err := engine.Run(context.Background(), planned, opt)
	if err != nil {
		t.Fatalf("the preview was refused: %v", err)
	}
	var names []string
	for _, f := range res.Manifest.Files {
		names = append(names, f.Name)
	}
	return names
}

// Where the number goes, by the owner's choice of 2026-11-14: after the batch
// in a name this tool put together, before the ending in a name somebody
// wrote whole. A run in a directory holding another one takes the number even
// when none of its own names would collide - the number follows the manifest.
func TestTheNumberGoesWhereTheOwnerChose(t *testing.T) {
	boundary := txtTarget("limit", 3, 4096)
	boundary.BoundaryLimit = 4096
	boundary.Sizes = []int64{4095, 4096, 4097}
	written := txtTarget("invoices", 1, 4096)
	written.NameTmpl = "invoice_{index:04}.txt"
	ownEnding := txtTarget("data", 1, 4096)
	ownEnding.NameTmpl = "data_{index:04}.bin"
	noDot := txtTarget("bare", 1, 4096)
	noDot.NameTmpl = "README"

	for _, tc := range []struct {
		name   string
		target engine.Target
		want   []string
	}{
		{"another batch of the same format", txtTarget("other", 1, 4096), []string{"other-2_0001.txt"}},
		{"a boundary set", boundary, []string{"limit-2_under_1b.txt", "limit-2_at_limit.txt", "limit-2_over_1b.txt"}},
		{"a template", written, []string{"invoice_0001-2.txt"}},
		{"a template with an ending of its own", ownEnding, []string{"data_0001-2.bin"}},
		{"a template with no ending", noDot, []string{"README-2"}},
		{"an ending of two dots", engine.Target{ID: "pack", Format: "targz", Sizes: []int64{16384}, Label: true}, []string{"pack-2_0001.tar.gz"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := numberedNames(t, []engine.Target{tc.target})
			if strings.Join(got, " ") != strings.Join(tc.want, " ") {
				t.Errorf("the run would write %v, want %v", got, tc.want)
			}
		})
	}
}

// A directory that holds no run keeps the names asked for: an empty one, a
// missing one, and one holding only files of somebody else's.
func TestARunIntoAFreeDirectoryKeepsItsNames(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(t *testing.T, dir string) string
	}{
		{"empty", func(t *testing.T, dir string) string { return dir }},
		{"missing", func(t *testing.T, dir string) string { return filepath.Join(dir, "not-yet") }},
		{"holding a file of its own", func(t *testing.T, dir string) string {
			if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("mine"), 0o644); err != nil {
				t.Fatal(err)
			}
			return dir
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := tc.prepare(t, t.TempDir())
			res, rec := runAndSave(t, []engine.Target{txtTarget("files", 1, 4096)}, numberingOptions(dir))
			if res.Number != 0 {
				t.Errorf("the run took number %d into a directory holding no run", res.Number)
			}
			if rec.Manifest != filepath.Join(dir, engine.DefaultManifestName) {
				t.Errorf("the record went to %s", rec.Manifest)
			}
			if _, err := os.Stat(filepath.Join(dir, "files_0001.txt")); err != nil {
				t.Errorf("the file was not written under the name asked for: %v", err)
			}
		})
	}
}

// Whatever holds a name of number 2 - a file, a directory, the reservation a
// killed run left, its manifest - moves the run on to 3. Nothing is written
// over and nothing is refused.
func TestANumberAnythingHoldsIsPassedOver(t *testing.T) {
	for _, tc := range []struct {
		name string
		hold func(t *testing.T, dir string)
	}{
		{"a file of the run's names", func(t *testing.T, dir string) { writeHolder(t, filepath.Join(dir, "files-2_0002.txt")) }},
		{"a directory of the run's names", func(t *testing.T, dir string) {
			if err := os.Mkdir(filepath.Join(dir, "files-2_0001.txt"), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{"the manifest of number 2", func(t *testing.T, dir string) { writeHolder(t, filepath.Join(dir, "manifest-2.json")) }},
		{"a reservation a killed run left", func(t *testing.T, dir string) {
			writeHolder(t, manifest.ReservationPath(filepath.Join(dir, "manifest-2.json")))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			seedDirectory(t, dir)
			tc.hold(t, dir)
			before := contentsOf(t, dir)
			res, _ := runAndSave(t, []engine.Target{txtTarget("files", 2, 4096)}, numberingOptions(dir))
			if res.Number != 3 {
				t.Errorf("the run took number %d, and something holds a name of number 2", res.Number)
			}
			if changed := changedFrom(before, contentsOf(t, dir)); len(changed) > 0 {
				t.Errorf("the run changed %v", changed)
			}
		})
	}
}

// writeHolder puts a small file of somebody else's at path.
func writeHolder(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("held"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The instructions a run writes beside its manifest follow the manifest's
// number, so the pair still names each other.
func TestTheInstructionsFollowTheNumberedManifest(t *testing.T) {
	dir := t.TempDir()
	seedDirectory(t, dir)
	target := txtTarget("explained", 1, 4096)
	target.Purpose = "a file with a reason"
	_, rec := runAndSave(t, []engine.Target{target}, numberingOptions(dir))
	if want := filepath.Join(dir, manifest.InstructionsName("manifest-2.json")); rec.Instructions != want {
		t.Errorf("the instructions went to %s, want %s beside manifest-2.json", rec.Instructions, want)
	}
}

// The command line numbers only when told to: without the flag a second run is
// refused as it always was, with it the run goes ahead under a number and says
// so, and a number too long for a name is refused with the code a taken name
// gets.
func TestTheCommandLineNumbersOnlyWhenTold(t *testing.T) {
	dir := t.TempDir()
	run := func(extra ...string) (int, string) {
		return tfg(append([]string{"generate", "--format", "txt", "--size", "4kb", "--out", dir}, extra...)...)
	}
	if code, said := run(); code != cli.ExitOK {
		t.Fatalf("the first run ended %d: %s", code, said)
	}
	before := contentsOf(t, dir)

	if code, said := run(); code != cli.ExitIO {
		t.Errorf("a second run without --number-if-taken ended %d, and it is refused as a taken name is: %s", code, said)
	}
	code, said := run("--number-if-taken")
	if code != cli.ExitOK {
		t.Fatalf("a second run with --number-if-taken ended %d: %s", code, said)
	}
	for _, want := range []string{"carries the number 2", "manifest-2.json"} {
		if !strings.Contains(said, want) {
			t.Errorf("the run with a number did not say %q. It said:\n%s", want, said)
		}
	}
	if changed := changedFrom(before, contentsOf(t, dir)); len(changed) > 0 {
		t.Errorf("the numbered run changed %v", changed)
	}

	long := strings.Repeat("n", core.MaxNameBytes-len(".txt")) + ".txt"
	if code, said := run("--number-if-taken", "--name", long); code != cli.ExitIO || !strings.Contains(said, "would carry the number 3") {
		t.Errorf("a number making a name too long ended %d, want %d with the reason. It said:\n%s", code, cli.ExitIO, said)
	}
}

// A name the number makes longer than any system takes is refused before
// anything is written, as a fault of the directory rather than of the name -
// the name was asked for at the limit on purpose, as the preset that asks about
// the longest name a system stores does (filename-handling).
func TestANameTheNumberMakesTooLongIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	dir := t.TempDir()
	seedDirectory(t, dir)
	before := contentsOf(t, dir)
	long := txtTarget("long", 1, 4096)
	long.NameTmpl = strings.Repeat("n", core.MaxNameBytes-len(".txt")) + ".txt"
	opt := numberingOptions(dir)
	planned, err := engine.Plan([]engine.Target{long}, opt)
	if err != nil {
		t.Fatalf("the name fits before it is numbered, and planning refused it: %v", err)
	}
	res, runErr := engine.Run(context.Background(), planned, opt)
	var refusal *engine.NumberTooLongError
	if !errors.As(runErr, &refusal) {
		t.Fatalf("the run ended with %v, want a refusal saying the number made a name too long", runErr)
	}
	if refusal.Dir != dir || refusal.Number != 2 || len(refusal.Name) != core.MaxNameBytes+2 {
		t.Errorf("the refusal names %s, number %d and a name of %d bytes - want %s, 2 and %d",
			refusal.Dir, refusal.Number, len(refusal.Name), dir, core.MaxNameBytes+2)
	}
	if res.Started || res.Number != 0 {
		t.Errorf("a refused run reports started %v and number %d, so a caller would save a record for it", res.Started, res.Number)
	}
	if changed := changedFrom(before, contentsOf(t, dir)); len(changed) > 0 || len(contentsOf(t, dir)) != len(before) {
		t.Errorf("the refused run left the directory different")
	}
}
