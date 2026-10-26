package guard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// A run whose manifest this build cannot read back has to say so.
//
// Two ceilings were chosen independently and never held against each other.
// core.MaxFilesPerRun is a million files. manifest.MaxBytes is sixteen
// megabytes.
//
// Measured on 2026-08-26: a run of 25 000 files wrote a manifest of 25 220 640
// B, and from that moment "tfg verify" and "tfg cleanup" both exited 5 on it
// with "is 25220640 B and the limit is 16777216 B". The files stayed on the
// disk and nothing in this toolset could remove them - the manifest is the only
// authority over what may be deleted, so a manifest that cannot be read is a
// set of files with no owner.
//
// The owner's decision on the same day was a note rather than a refusal: the
// run works, and refusing it would take away something this tool does today.
// What was missing was that nobody was told.

// inTheSilentBand is a run that was told nothing until 2026-10-26.
//
// The warning came from 786 B an entry, and a text file of one byte with no
// label costs about 838 - so 20 500 of them came to 16 113 000 B by the
// estimate, under the ceiling, and wrote 17 170 219 B, over it
// (docs/STABILITY-REVIEW-2026-10-26.md, section 2.1). A run here is a run the
// old arithmetic let through in silence, which is the run these guards are for.
var inTheSilentBand = []string{"generate", "--format", "txt", "--size", "1b", "--clean", "--count", "20500", "--dry-run"}

func TestARunSaysWhenItsManifestWillBeTooBigToReadBack(t *testing.T) {
	said := runCLI(t, append(inTheSilentBand, "--out", t.TempDir())...)

	if !strings.Contains(said, "will not be able to read") {
		t.Fatalf("a run whose manifest will be past the ceiling was told nothing:\n%s", firstLines(said, 6))
	}
	if !strings.Contains(said, "verify") || !strings.Contains(said, "cleanup") {
		t.Errorf("the note does not name the two commands that stop working:\n%s", said)
	}
	if !strings.Contains(said, "manifest") {
		t.Errorf("the note does not say what will be too big:\n%s", said)
	}
}

// And a run comfortably under it stays quiet.
//
// Without this the test above passes on a build that warns about every run,
// which teaches people to skip the line - the same reason the site probe skips
// pages carrying noindex rather than reporting noise.
func TestAnOrdinaryRunSaysNothingAboutTheManifestCeiling(t *testing.T) {
	said := runCLI(t, "generate", "--format", "txt", "--size", "200b",
		"--count", "100", "--dry-run", "--out", t.TempDir())

	if strings.Contains(said, "too big") || strings.Contains(said, "will not be able to read") {
		t.Errorf("a hundred files drew a warning about the manifest ceiling:\n%s", said)
	}
}

// The length worked out for a manifest is the length Encode writes, to the
// byte.
//
// This replaces a guard that held an estimate to "a quarter either way", and
// a quarter was wide enough to let a constant 6% low for text and half of what
// an e-mail entry costs stay green while a run of 20 500 files went unwarned.
// The answer is counted entry by entry now, so it is held to the byte: a
// document of no entries, one entry, and many unlike each other - notes, a
// failure, words outside ASCII, characters JSON escapes, properties nested
// several deep.
func TestTheManifestSizeIsTheLengthOfTheManifest(t *testing.T) {
	documents := map[string]*manifest.Manifest{"no entries": newDocument()}

	one := newDocument()
	one.Add(manifest.File{ID: "a", Path: "a.txt", Name: "a.txt", Format: "txt", Bytes: 1})
	documents["one entry"] = one

	many := newDocument()
	many.Run.Preset = &manifest.Preset{ID: "upload-validation", Parameters: map[string]string{"limit": "1mb"}}
	many.Add(manifest.File{ID: "plain", Path: "plain.txt", Name: "plain.txt", Format: "txt", Bytes: 200, Materialized: true,
		Hashes: manifest.Hashes{SHA256: strings.Repeat("ab", 32)}, TargetID: "t"})
	many.Add(manifest.File{ID: "noted", Path: "zażółć 日本.txt", Name: "zażółć 日本.txt", Format: "txt",
		Notes:    []manifest.Note{{Code: "label_omitted", Detail: "it said <nothing> & \"left\" \\ alone"}},
		Expected: manifest.Expected{Outcome: "reject", Reason: "size_limit"}, Purpose: "a <b> & c", Group: "g"})
	many.Add(manifest.File{ID: "deep", Path: "deep.zip", Name: "deep.zip", Format: "zip",
		Properties: map[string]any{"contains": []any{map[string]any{"format": "txt", "count": 3}}, "depth": 2, "flat": true, "ratio": 0.5}})
	many.Add(manifest.File{ID: "failed", Path: "failed.png", Name: "failed.png", Format: "png", Failed: true,
		Error: "a generator stopped", Damage: []manifest.Damage{{Type: "zero-head", Settings: map[string]string{"bytes": "4"}}}})
	documents["many entries"] = many

	for name, m := range documents {
		counted, err := m.EncodedBytes()
		if err != nil {
			t.Fatalf("%s: counting: %v", name, err)
		}
		var written bytes.Buffer
		if err := m.Encode(&written); err != nil {
			t.Fatalf("%s: encoding: %v", name, err)
		}
		if counted != int64(written.Len()) {
			t.Errorf("%s: the manifest was counted at %d B and Encode wrote %d B", name, counted, written.Len())
		}
	}
}

// newDocument is an empty manifest with the header a run gives it.
func newDocument() *manifest.Manifest {
	return manifest.New("testing-files-generator", "0.0.0-dev", "run_x", "tfg generate", 1, "linux", "amd64")
}

// And it says so where a script can read it, not only in prose.
//
// The note above is correct, printed first, and was the whole of the answer
// until 2026-09-06. Two measurements say why that was not enough. On stderr it
// was one line among 25 003, carrying the same "note:" prefix as 25 000
// repetitions of a sentence about labels - grouped since, so it is now one of
// two. And in the machine readable report it was ABSENT ENTIRELY: a run of
// 25 000 files answered exit 0 with a clean --json and no trace of the warning
// anywhere in it.
//
// This tool plugs into CI. A fact that only a person reading prose can learn is
// a fact a pipeline cannot act on, and the thing it cannot act on here is that
// the directory it just filled can never be verified or cleaned up by this
// build - untouchable rule 7 makes the manifest the only authority over what
// may be deleted.
//
// NOT AN EXIT CODE, and that was decided rather than skipped. docs/CLI.md
// defines 8 as "run finished, but not everything was produced", and here
// everything was produced. The sentence would read "0 files could not be
// produced" and point at a manifest nothing can read. Exit codes are a frozen
// contract.
func TestAManifestTooBigToReadBackSaysSoInTheReportAScriptReads(t *testing.T) {
	// stdout only. The summary line goes to the error stream, and a reader that
	// took both would be parsing JSON with a sentence stuck to the end of it.
	code, said, errOut := run(t, append(inTheSilentBand, "--json", "--out", t.TempDir())...)
	if code != cli.ExitOK {
		t.Fatalf("the run ended with %d, so there is no report to read:\n%s", code, errOut)
	}

	// Read out of the document a consumer receives rather than off the struct.
	// Decoding into manifest.Summary would pass by construction the moment the
	// type changed, which is the trap manifestShape in recipe_test.go exists
	// against.
	var report struct {
		Summary map[string]any `json:"summary"`
	}
	if err := json.Unmarshal([]byte(said), &report); err != nil {
		t.Fatalf("the report is not readable as JSON: %v\n%s", err, firstLines(said, 4))
	}
	flag, present := report.Summary["too_large_to_read_back"]
	if !present {
		t.Fatalf("the report carries no too_large_to_read_back at all, so a script has "+
			"no way to learn that this run cannot be verified or cleaned up:\nsummary: %v",
			report.Summary)
	}
	if flag != true {
		t.Errorf("too_large_to_read_back is %v on a run that is over the ceiling", flag)
	}
}

// An ordinary run does not carry the field at all.
//
// The sharp half. A build writing "too_large_to_read_back": false into every
// manifest would satisfy the guard above and change the bytes of every document
// this tool has ever written, for a fact that is almost never worth saying. It
// is absent rather than false, which is what keeps existing manifests byte for
// byte what they were - the same choice recipe_hash, overrides and preset made.
func TestAnOrdinaryRunSaysNothingAboutTheCeilingInItsReport(t *testing.T) {
	code, said, errOut := run(t, "generate", "--format", "txt", "--size", "200b",
		"--count", "20", "--dry-run", "--json", "--out", t.TempDir())
	if code != cli.ExitOK {
		t.Fatalf("the run ended with %d, so there is no report to read:\n%s", code, errOut)
	}

	var report struct {
		Summary map[string]any `json:"summary"`
	}
	if err := json.Unmarshal([]byte(said), &report); err != nil {
		t.Fatalf("the report is not readable as JSON: %v\n%s", err, firstLines(said, 4))
	}
	// Asserted rather than assumed, so an empty summary cannot pass this by
	// having no keys at all.
	if len(report.Summary) == 0 {
		t.Fatal("the report carries no summary, so this guard checked nothing")
	}
	if _, present := report.Summary["too_large_to_read_back"]; present {
		t.Errorf("an ordinary run carries too_large_to_read_back in its manifest.\n" +
			"Absent rather than false is what keeps every manifest already written " +
			"byte for byte what it was.")
	}
}

// A saved manifest carries the field exactly when this build will refuse it,
// asked one byte either side of the ceiling.
//
// The field is written into the document whose length it describes, so the
// edge is where a rule that is nearly right goes wrong: a manifest of exactly
// manifest.MaxBytes is read and must not say otherwise, and one byte more is
// refused and must say so. Saved for real and read back, so the answer is about
// the bytes on the disk rather than about the arithmetic that predicted them.
func TestASavedManifestSaysItIsTooBigExactlyWhenItIs(t *testing.T) {
	for _, past := range []int64{0, 1} {
		m := documentOfLength(t, manifest.MaxBytes+past)
		path := filepath.Join(t.TempDir(), "manifest.json")
		if err := m.Save(path); err != nil {
			t.Fatalf("saving: %v", err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("looking at the saved manifest: %v", err)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading the saved manifest: %v", err)
		}
		says := bytes.Contains(raw, []byte(`"too_large_to_read_back": true`))
		_, loadErr := manifest.Load(path)
		var refused *manifest.TooLargeError
		isRefused := errors.As(loadErr, &refused)

		switch {
		case past == 0 && (info.Size() != manifest.MaxBytes || says || isRefused):
			t.Errorf("a manifest of the ceiling's length: %d B on the disk, field present %v, refused %v (%v) - it is read and must not say it is not",
				info.Size(), says, isRefused, loadErr)
		case past == 1 && (info.Size() <= manifest.MaxBytes || !says || !isRefused):
			t.Errorf("a manifest one byte longer: %d B on the disk, field present %v, refused %v - it is refused and must say so",
				info.Size(), says, isRefused)
		}
		if m.SavedBytes() != info.Size() {
			t.Errorf("Save says it wrote %d B and the disk holds %d B", m.SavedBytes(), info.Size())
		}
	}
}

// documentOfLength is a manifest whose document, without the field, is exactly
// length bytes long - one entry's purpose takes up the difference.
func documentOfLength(t *testing.T, length int64) *manifest.Manifest {
	t.Helper()
	m := newDocument()
	m.Add(manifest.File{ID: "a", Path: "a.txt", Name: "a.txt", Format: "txt", Purpose: "x"})
	now, err := m.EncodedBytes()
	if err != nil {
		t.Fatalf("counting: %v", err)
	}
	m.Files[0].Purpose += strings.Repeat("x", int(length-now))
	// Measured as it stands, before Save has settled the field - so this is the
	// length the rule is asked about, and nothing here has set the field yet
	// for Save to find already set.
	if got, _ := m.EncodedBytes(); got != length || m.Summary.TooLargeToReadBack {
		t.Fatalf("the document is %d B (field set: %v) and was built to be %d B without it, so the edge is not where this asks",
			got, m.Summary.TooLargeToReadBack, length)
	}
	return m
}

// What a run is told about its record before its first byte is what it then
// writes, when every file is produced - the manifest to the byte, and the
// instructions beside it.
//
// The forecast is what the free space is held against and what the warning
// above is said from. A forecast that drifted from the record would put both
// back where the old constant had them, wrong in a direction nobody sees.
func TestTheRecordARunIsToldBeforehandIsTheRecordItWrites(t *testing.T) {
	dir := t.TempDir()
	opt := engine.Options{OutDir: dir, Seed: 7, Command: "test"}
	told := txtTarget("told", 3, 100)
	told.Purpose, told.Group = "A file with a purpose, zażółć <&>", "with a purpose"
	planned, err := engine.Plan([]engine.Target{txtTarget("plain", 300, 200), told}, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}

	forecast, err := engine.PredictRecord(context.Background(), planned, opt)
	if err != nil {
		t.Fatalf("forecasting: %v", err)
	}
	res, err := engine.Run(context.Background(), planned, opt)
	if err != nil {
		t.Fatalf("running: %v", err)
	}
	rec, err := engine.SaveRecord(res, opt)
	if err != nil {
		t.Fatalf("saving the record: %v", err)
	}

	if rec.Instructions == "" || forecast.Instructions == 0 {
		t.Fatalf("no instructions were written or forecast (%q, %d B), so half of this asked nothing", rec.Instructions, forecast.Instructions)
	}
	onDisk := map[string]int64{}
	for name, path := range map[string]string{"manifest": rec.Manifest, "instructions": rec.Instructions} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("looking at the %s: %v", name, err)
		}
		onDisk[name] = info.Size()
	}
	if forecast.Manifest != onDisk["manifest"] || rec.Size.Manifest != onDisk["manifest"] {
		t.Errorf("the manifest was forecast at %d B, recorded as %d B and is %d B on the disk",
			forecast.Manifest, rec.Size.Manifest, onDisk["manifest"])
	}
	if forecast.Instructions != onDisk["instructions"] || rec.Size.Instructions != onDisk["instructions"] {
		t.Errorf("the instructions were forecast at %d B, recorded as %d B and are %d B on the disk",
			forecast.Instructions, rec.Size.Instructions, onDisk["instructions"])
	}
}

// runCLI runs one command and gives back everything it said, on both channels.
//
// Both, because the line under test goes to the error channel - a report on
// stdout has to stay machine readable - and a guard reading only one of them
// would be measuring which channel it picked.
func runCLI(t *testing.T, args ...string) string {
	t.Helper()
	var out, errOut bytes.Buffer
	cli.Run(context.Background(), args, &out, &errOut)
	return out.String() + errOut.String()
}

// firstLines trims a long document down for a failure message.
func firstLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
