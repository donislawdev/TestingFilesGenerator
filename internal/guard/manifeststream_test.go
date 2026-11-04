package guard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// The manifest is written an entry at a time, and the bytes are still the ones
// encoding/json writes.
//
// Until 2026-11-02 Encode was one call into encoding/json, which builds the
// whole document in memory before its first write and indents it into a second
// copy while every entry is still held. At 100 000 text files saving the
// manifest peaked at 9.3 KB a file against 4.0 KB for writing every file, and a
// run of 200 000 in a container of 1 GB was killed after its last file with no
// manifest written (docs/FILE-CEILING-2026-11-02.md section 8). Encode now
// streams. Two things can go wrong with that and each half of this guard is
// about one of them.
//
// The bytes. verify, cleanup, the exports and every reader somebody wrote
// against a manifest read these bytes, and the streamed layout is assembled by
// hand. So it is compared with encoding/json's own rendering of the same value
// - not with size.go, which counts the same hand made layout and would agree
// with a mistake both made the same way.
func TestTheManifestIsWrittenAsEncodingJSONWritesIt(t *testing.T) {
	documents := manifestSamples()
	documents["a list that was never made"] = neverListed(t)
	documents["two thousand entries"] = manyEntries(2000)
	for name, m := range documents {
		var streamed bytes.Buffer
		if err := m.Encode(&streamed); err != nil {
			t.Fatalf("%s: Encode: %v", name, err)
		}
		if want := encodingJSON(t, m); !bytes.Equal(streamed.Bytes(), want) {
			at := firstByteApart(streamed.Bytes(), want)
			t.Errorf("%s: Encode wrote %d B and encoding/json writes %d B, apart from byte %d:\n got: %q\nwant: %q",
				name, streamed.Len(), len(want), at, bytesAround(streamed.Bytes(), at), bytesAround(want, at))
		}
	}
}

// And the document is never handed over whole. This is the half that says the
// streaming is there at all: going back to json.NewEncoder writes the same
// bytes in ONE write, the half above stays green, and this one turns red.
func TestTheManifestIsNotHeldWholeToBeWritten(t *testing.T) {
	m := manyEntries(2000)
	var w writeSizes
	if err := m.Encode(&w); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	// The question only means something for a document many writes long, so
	// that is asserted rather than assumed.
	if w.total < 512<<10 {
		t.Fatalf("the document of two thousand entries is only %d B - too short to tell a whole write from a streamed one", w.total)
	}
	if w.largest*8 > w.total {
		t.Errorf("a manifest of %d B reached its writer in %d writes, the largest %d B. "+
			"Written whole, the document sits in memory beside every entry it was made from", w.total, w.writes, w.largest)
	}
}

// writeSizes counts what a writer was handed and the largest single piece.
type writeSizes struct{ total, writes, largest int }

func (w *writeSizes) Write(p []byte) (int, error) {
	w.total += len(p)
	w.writes++
	w.largest = max(w.largest, len(p))
	return len(p), nil
}

// keptSizes is writeSizes that also keeps what it was handed.
type keptSizes struct {
	writeSizes
	kept bytes.Buffer
}

func (w *keptSizes) Write(p []byte) (int, error) {
	_, _ = w.writeSizes.Write(p)
	return w.kept.Write(p)
}

// generate --json sends the manifest a part at a time too, and after a run what
// it sends is the file the run saved.
//
// It used to encode the whole document into memory a second time to send it,
// beside every entry the run still held, for a run whose manifest was on the
// disk already - observation O300. Both halves: a dry run, which saves nothing,
// and a real run, whose manifest is on the disk - and what that one sends is
// the saved file to the byte, so what a script reads is what verify reads.
func TestGenerateSendsItsManifestAPartAtATime(t *testing.T) {
	for _, dry := range []bool{true, false} {
		dir := t.TempDir()
		args := []string{"generate", "--format", "txt", "--size", "1b", "--count", "800", "--json", "--out", dir}
		if dry {
			args = append(args, "--dry-run")
		}
		var sent keptSizes
		var errOut bytes.Buffer
		if code := cli.Run(context.Background(), args, &sent, &errOut); code != cli.ExitOK {
			t.Fatalf("dry run %v: generate ended %d:\n%s", dry, code, errOut.String())
		}
		// The question only means something for a document many writes long.
		if sent.total < 512<<10 {
			t.Fatalf("dry run %v: the manifest sent is only %d B - too short to tell a whole write from a streamed one", dry, sent.total)
		}
		if sent.largest*8 > sent.total {
			t.Errorf("dry run %v: a manifest of %d B reached standard output in %d writes, the largest %d B - held whole to be sent",
				dry, sent.total, sent.writes, sent.largest)
		}
		if dry {
			if !json.Valid(sent.kept.Bytes()) {
				t.Errorf("the manifest a dry run sent is not a JSON document")
			}
			continue
		}
		saved, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(saved, sent.kept.Bytes()) {
			t.Errorf("the manifest sent to standard output (%d B) is not the file the run saved (%d B)", sent.total, len(saved))
		}
	}
}

// encodingJSON is the manifest as encoding/json renders it in one piece, with
// the indent Encode has always used.
func encodingJSON(t *testing.T, m *manifest.Manifest) []byte {
	t.Helper()
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		t.Fatalf("encoding/json: %v", err)
	}
	return b.Bytes()
}

// neverListed is a manifest read from a document with no list of files at
// all, so its list is nil - encoded as null rather than [], and a streamed
// layout that assumed a list would write the wrong one.
func neverListed(t *testing.T) *manifest.Manifest {
	t.Helper()
	var m manifest.Manifest
	if err := json.Unmarshal([]byte(`{"manifest_version":"1.0"}`), &m); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if m.Files != nil {
		t.Fatalf("the list came back as %v rather than nil, so this case is not the one it names", m.Files)
	}
	return &m
}

// manyEntries is a manifest long enough that a whole write and a streamed one
// cannot be confused, with entries that carry escaped text and nested values.
func manyEntries(n int) *manifest.Manifest {
	m := newDocument()
	for i := range n {
		name := fmt.Sprintf("files_%04d <%d> & \"x\".txt", i+1, i)
		m.Add(manifest.File{ID: fmt.Sprintf("f_%04d", i+1), Path: name, Name: name, Format: "txt", Bytes: int64(i),
			Hashes:     manifest.Hashes{SHA256: strings.Repeat("cd", 32)},
			Properties: map[string]any{"line": "a\u2028b", "depth": i % 3, "nested": map[string]any{"k": []any{1, "two"}}}})
	}
	return m
}

func firstByteApart(a, b []byte) int {
	for i := range min(len(a), len(b)) {
		if a[i] != b[i] {
			return i
		}
	}
	return min(len(a), len(b))
}

func bytesAround(b []byte, at int) []byte {
	return b[max(0, at-40):min(len(b), at+40)]
}
