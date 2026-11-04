package guard

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/audit"
	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// The limits a manifest is read under: the ceiling on one value, the memory
// setting on what verify and cleanup hold, Ctrl+C while it is read - and the
// two doors it is read through agreeing about all of it. Beside
// manifestread_test.go, which holds what reading must keep doing the way the
// reader it replaced did.

// Load holds the whole of a manifest and keeps the ceiling the whole document
// always had, while ReadClaims reads the same document.
//
// Load is for the guards and anything else that needs every entry, and holding
// every entry of a document of any length is what the ceiling on a document
// was for. ReadClaims holds three fields of what is claimed and has none.
func TestLoadKeepsTheCeilingOnAWholeDocumentAndReadClaimsHasNone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, []byte(longManifest(`{"path":"first.txt"}`, manifest.MaxBytes)), 0o644); err != nil {
		t.Fatal(err)
	}
	var tooLarge *manifest.TooLargeError
	if _, err := manifest.Load(path); !errors.As(err, &tooLarge) {
		t.Errorf("Load read a document past %d B, or refused it for something else: %v", manifest.MaxBytes, err)
	}
	if _, err := manifest.ReadClaims(context.Background(), path, 0); err != nil {
		t.Errorf("ReadClaims refused a document whose only fault is its length: %v", err)
	}
}

// The two doors a manifest is read through refuse the same documents in the
// same words, and give the same claims for the rest.
//
// Load holds the whole of a manifest and ReadClaims three fields of what it
// claims. Both are one reader underneath, and this is what keeps them so: a
// door given its own parsing later, or its own order of faults, would make
// every guard that reads a manifest through Load a guard about a reader verify
// and cleanup do not use.
func TestBothDoorsOfTheManifestRefuseTheSameDocumentsInTheSameWords(t *testing.T) {
	docs := []struct{ name, doc string }{
		{"a key twice", `{"manifest_version":"1.0","files":[],"Files":[]}`},
		{"a second object after it", `{"manifest_version":"1.0","files":[]} {}`},
		{"an ordinary manifest", `{"manifest_version":"1.0","run":{"instructions":"manifest.instructions.md"},"files":[{"path":"a.txt","materialized":true,"bytes":1,"hashes":{"sha256":"00"}},{"path":"b.txt","failed":true}]}`},
	}
	for _, c := range twoFaultDocuments {
		docs = append(docs, struct{ name, doc string }{c.name, c.doc})
	}
	for _, c := range nearlyEmptyDocuments {
		docs = append(docs, struct{ name, doc string }{c.name, c.doc})
	}
	refused, read := 0, 0
	for _, c := range docs {
		path := filepath.Join(t.TempDir(), "manifest.json")
		if err := os.WriteFile(path, []byte(c.doc), 0o644); err != nil {
			t.Fatal(err)
		}
		whole, loadErr := manifest.Load(path)
		claims, readErr := manifest.ReadClaims(context.Background(), path, 0)
		switch {
		case loadErr != nil && readErr != nil:
			refused++
			if loadErr.Error() != readErr.Error() {
				t.Errorf("%s: the two doors refuse it in different words:\n  Load       %v\n  ReadClaims %v", c.name, loadErr, readErr)
			}
		case loadErr == nil && readErr == nil:
			read++
			if fmt.Sprint(whole.Claims()) != fmt.Sprint(claims) {
				t.Errorf("%s: the two doors claim different files:\n  Load       %v\n  ReadClaims %v", c.name, whole.Claims(), claims)
			}
		default:
			t.Errorf("%s: one door reads it and the other does not:\n  Load       %v\n  ReadClaims %v", c.name, loadErr, readErr)
		}
	}
	// The state this depends on: both kinds of answer were asked for.
	if refused < len(twoFaultDocuments) || read < 2 {
		t.Fatalf("%d documents refused and %d read - the corpus no longer holds both kinds", refused, read)
	}
}

// stoppedPartWay is a context cancelled at its nth question - Ctrl+C landing
// while a manifest is read, wherever the reader is when it asks.
type stoppedPartWay struct {
	context.Context
	cancel    context.CancelFunc
	asked, at int
}

func (s *stoppedPartWay) Err() error {
	s.asked++
	if s.asked >= s.at {
		s.cancel()
	}
	return s.Context.Err()
}

// Ctrl+C while a manifest is read stops the reading, and cleanup says it
// removed nothing.
//
// A manifest of a million files takes seconds to read, and the reader that held
// it whole asked nobody anything while it read. This document is cut short in
// its hundred and second entry, so a reader that asks only before it starts, or
// only between the keys of the document, reads on to the cut and refuses the
// document instead - only one that asks as it goes through the entries stops
// first. Cancelled at the fiftieth question, because what matters is that the
// asking goes on, not where it starts.
func TestCtrlCWhileAManifestIsReadStopsTheReadingAndRemovesNothing(t *testing.T) {
	dir := t.TempDir()
	var b strings.Builder
	b.WriteString(`{"manifest_version":"1.0","files":[` + claimOf(t, dir, "ours.txt", "written by the run\n"))
	for i := range 100 {
		fmt.Fprintf(&b, `,{"path":"never_%03d.txt"}`, i)
	}
	b.WriteString(`,{"path":"cut sh`)
	mf := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(mf, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	// The state this depends on: read to the end, the document is refused
	// rather than stopped.
	if code, _, errOut := run(t, "cleanup", mf, "--yes"); code != cli.ExitIO {
		t.Fatalf("cleanup on the document cut short ended %d, expected %d:\n%s", code, cli.ExitIO, errOut)
	}

	base, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopped := &stoppedPartWay{Context: base, cancel: cancel, at: 50}
	var out, errOut bytes.Buffer
	code := cli.Run(stopped, []string{"cleanup", mf, "--yes"}, &out, &errOut)
	if code != cli.ExitInterrupted || !strings.Contains(errOut.String(), "interrupted while reading the manifest and removed nothing") {
		t.Errorf("Ctrl+C while the manifest was read ended %d, expected %d and a sentence saying so:\n%s", code, cli.ExitInterrupted, errOut.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "ours.txt")); err != nil {
		t.Errorf("cleanup removed a file after Ctrl+C: %v", err)
	}
}

// A manifest listing more files than the memory setting lets cleanup check is
// refused before a file is looked at, and the refusal says how much it would
// take.
//
// The reader holds three fields of each claimed file and the checking a few
// times that, so a run of tens of millions of files could still ask a machine
// for more than it has. The memory setting is the person's number for that, the
// one generate plans under. The refusal comes once the whole document has been
// read and found sound, so a document with a fault of its own is refused for
// the fault rather than for its size.
func TestAManifestListingMoreThanTheMemorySettingLetsCleanupCheckIsRefused(t *testing.T) {
	dir := t.TempDir()
	var b strings.Builder
	b.WriteString(`{"manifest_version":"1.0","files":[` + claimOf(t, dir, "ours.txt", "written by the run\n"))
	for i := range 999 {
		fmt.Fprintf(&b, `,{"path":"elsewhere_%03d.txt","materialized":true,"bytes":1,"hashes":{"sha256":"%064d"}}`, i, 0)
	}
	mf := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(mf, []byte(b.String()+"]}"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, errOut := run(t, "cleanup", mf, "--yes", "--memory", "64kb")
	if code != cli.ExitIO || stdout != "" || !strings.Contains(errOut, "lists 1000 files") || !strings.Contains(errOut, "the memory setting gives") {
		t.Errorf("cleanup under a setting too small for the manifest ended %d, expected %d and the refusal:\n%s%s", code, cli.ExitIO, stdout, errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, "ours.txt")); err != nil {
		t.Errorf("cleanup acted on a manifest it refused: %v", err)
	}
	// Room enough, or no limit, and the same document is read - what refused
	// it was the setting.
	for _, setting := range []string{"64mb", "off"} {
		if code, _, errOut := run(t, "verify", mf, "--memory", setting); code != cli.ExitVerify {
			t.Errorf("verify under --memory %s ended %d, expected the %d of a directory missing files:\n%s", setting, code, cli.ExitVerify, errOut)
		}
	}

	// A fault of the document comes before its size.
	if err := os.WriteFile(mf, []byte(b.String()+","+escaping+"]}"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errOut = run(t, "verify", mf, "--memory", "64kb")
	if code != cli.ExitIO || !strings.Contains(errOut, "escaped") || strings.Contains(errOut, "memory setting") {
		t.Errorf("a manifest too large for the setting and with an entry that escapes was not refused for the entry, exit %d:\n%s", code, errOut)
	}
}

// Another run's record that lists one file over and over holds it once.
//
// What verify keeps of another run's record is what it says about the files in
// front of it, so what it holds grows with the directory and not with the
// record. A record naming one of those files in each of its entries was kept
// as a list as long as the record. Nothing in the report tells the two apart -
// the file is another run's either way - so this asks the memory. Two records
// of the same length are read the same way, one naming the file in every entry
// and one naming files that are not there, and what reading each allocated is
// compared. Kept once, the two cost about the same. Kept every time, the first
// costs at least the sixteen bytes of a string for each entry.
//
// Not in parallel, so no other guard allocates while it counts.
func TestAnotherRunsRecordListingOneFileOverAndOverHoldsItOnce(t *testing.T) {
	const entries = 50_000
	read := func(path func(int) string) (int64, []audit.Difference) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "a0000000.txt"), []byte("the other run wrote this\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		b.WriteString(`{"manifest_version":"1.0","files":[`)
		for i := range entries {
			if i > 0 {
				b.WriteByte(',')
			}
			fmt.Fprintf(&b, `{"path":%q,"materialized":true}`, path(i))
		}
		if err := os.WriteFile(filepath.Join(dir, "other.json"), []byte(b.String()+"]}"), 0o644); err != nil {
			t.Fatal(err)
		}
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		diffs, err := audit.Verify(context.Background(), dir, manifest.Claims{}, "manifest.json")
		runtime.ReadMemStats(&after)
		if err != nil {
			t.Fatalf("verifying: %v", err)
		}
		return int64(after.TotalAlloc - before.TotalAlloc), diffs
	}
	same, sameDiffs := read(func(int) string { return "a0000000.txt" })
	apart, apartDiffs := read(func(i int) string { return fmt.Sprintf("b%07d.txt", i) })

	// The state this is about: both records were read to the end and taken
	// for records, and only the first one claims the file.
	for _, c := range []struct {
		name  string
		diffs []audit.Difference
		file  audit.Kind
	}{{"naming the file", sameDiffs, audit.AnotherRun}, {"naming other files", apartDiffs, audit.Extra}} {
		kinds := map[string]audit.Kind{}
		for _, d := range c.diffs {
			kinds[d.Path] = d.Kind
		}
		if kinds["other.json"] != audit.AnotherRun || kinds["a0000000.txt"] != c.file {
			t.Fatalf("the record %s was not read as another run's record claiming what it lists: %v", c.name, c.diffs)
		}
	}

	t.Logf("reading the record naming the file allocated %d B, and the record naming other files %d B", same, apart)
	if grew := same - apart; grew > entries {
		t.Errorf("a record naming one file in each of its %d entries took %d B more to read than one naming none of the files, "+
			"%.1f B an entry - the file was kept once for every entry rather than once", entries, grew, float64(grew)/entries)
	}
}
