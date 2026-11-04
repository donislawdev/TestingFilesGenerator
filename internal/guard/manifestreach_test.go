package guard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// The manifest of a run past the ceiling the whole document once had is one
// verify and cleanup read.
//
// From 2026-08-03 until 2026-11-04 a manifest was read whole under sixteen
// megabytes, so a run of more than about twenty thousand files wrote one both
// commands refused - measured on 2026-08-26 at 25 000 files, generate ended 0
// and verify and cleanup 5, and the files were files nothing in this toolset
// could remove. For two months the answer was a note saying so, in both
// surfaces and in a field of the manifest (O184). Now the reader takes the
// document a piece at a time, the note and the field are gone, and this is the
// run they were about: the manifest a run wrote, past the old ceiling, read back
// by both commands.
//
// The run is three files and the length is copies of its own entries, each
// marked as never written, because writing twenty thousand files takes the
// better part of a minute on Windows and the length is what is asked about.
func TestTheManifestOfARunPastTheOldCeilingIsVerifiedAndCleanedUp(t *testing.T) {
	out, mf := generated(t)
	keys := keysOf(t, mf)
	lengthened := false
	for i, k := range keys {
		if k.key != "files" {
			continue
		}
		var entries []map[string]any
		if err := json.Unmarshal(k.value, &entries); err != nil || len(entries) == 0 {
			t.Fatalf("the run's list of files does not read as entries: %v", err)
		}
		model, err := json.Marshal(entries[0])
		if err != nil {
			t.Fatal(err)
		}
		for n := 0; n <= manifest.MaxBytes/len(model); n++ {
			copied := map[string]any{}
			for field, value := range entries[0] {
				copied[field] = value
			}
			name := fmt.Sprintf("never_%07d.txt", n)
			copied["id"], copied["path"], copied["name"], copied["materialized"] = name, name, name, false
			entries = append(entries, copied)
		}
		if keys[i].value, err = json.Marshal(entries); err != nil {
			t.Fatal(err)
		}
		lengthened = true
	}
	if !lengthened {
		t.Fatal("the run's manifest has no list of files to lengthen")
	}
	rewritten(t, mf, keys)
	// The state this is about: past the length the old reader refused.
	if info, err := os.Stat(mf); err != nil || info.Size() <= manifest.MaxBytes {
		t.Fatalf("the manifest is not past %d B: %v %v", manifest.MaxBytes, info, err)
	}

	if code, stdout, errOut := run(t, "verify", mf); code != cli.ExitOK || !strings.Contains(stdout, "3 files checked") {
		t.Errorf("verify on the manifest of a run past the old ceiling ended %d:\n%s%s", code, stdout, errOut)
	}
	if code, _, errOut := run(t, "cleanup", mf, "--yes"); code != cli.ExitOK {
		t.Errorf("cleanup on the manifest of a run past the old ceiling ended %d:\n%s", code, errOut)
	}
	if n := textFilesIn(t, out); n != 0 {
		t.Errorf("cleanup left %d of the three files the manifest lists", n)
	}
}

// manifestSamples are the documents both guards of the manifest's layout ask
// about - its counted length and its streamed bytes - so neither keeps a
// second copy of the cases: none, one, and many entries with text that
// encoding/json escapes, nested properties, notes and damage.
func manifestSamples() map[string]*manifest.Manifest {
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
	return documents
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
	for name, m := range manifestSamples() {
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

// What a run is told about its record before its first byte is what it then
// writes, when every file is produced - the manifest to the byte, and the
// instructions beside it.
//
// The forecast is what the free space is held against. A forecast that
// drifted from the record would put that back where the old constant had
// it, wrong in a direction nobody sees.
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
