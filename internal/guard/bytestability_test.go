package guard

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// Byte stability is guaranteed inside a major version. The compiler takes part
// in producing bytes, so an upgrade of Go is a change that can break it.
//
// Go promises nothing about the output of its compressors, so documentation
// cannot be leaned on. Measurement is the only mechanism there is.
//
// Deflate sits under both PNG and ZIP, which is two of the five formats of
// the first milestone. A drift here moves hashes in half of them.
//
// If this test goes red after an upgrade, that is a breaking change - it
// needs a major version bump, a changelog entry and a decision by the owner.
// Never edit the golden file to make it green again. That single move turns a
// guard into decoration.

const goldenFile = "testdata/stdlib-golden.json"

type goldenEntry struct {
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type golden struct {
	Note       string                 `json:"note"`
	MeasuredOn string                 `json:"measured_on"`
	GoVersion  string                 `json:"go_version"`
	Paths      map[string]goldenEntry `json:"paths"`
}

// payload is the 64 KiB input. Defined here so the test is self contained -
// nothing about it depends on a file or an earlier session.
func payload() []byte {
	b := make([]byte, 64*1024)
	for i := range b {
		b[i] = byte((i*31 + 7) % 251)
	}
	return b
}

// picture is the 64 by 64 image used for the PNG paths.
func picture() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8((x * 4) % 256),
				G: uint8((y * 4) % 256),
				B: uint8((x * y) % 256),
				A: 255,
			})
		}
	}
	return img
}

func TestStandardLibraryOutputHasNotDrifted(t *testing.T) {
	want := loadGolden(t)

	got := map[string][]byte{
		"flate_level_1": flateAt(t, 1),
		"flate_default": flateAt(t, flate.DefaultCompression),
		"gzip_default":  gzipDefault(t),
		"gzip_store":    gzipStore(t),
		"zip_deflate":   zipDeflate(t),
		"png_default":   pngAt(t, png.DefaultCompression),
		"png_best":      pngAt(t, png.BestCompression),
	}

	if len(want.Paths) != len(got) {
		t.Fatalf("golden file describes %d paths, the test produces %d - one of them was changed alone",
			len(want.Paths), len(got))
	}

	for name, produced := range got {
		expected, ok := want.Paths[name]
		if !ok {
			t.Errorf("path %q has no golden value", name)
			continue
		}
		sum := sha256.Sum256(produced)
		actual := hex.EncodeToString(sum[:])
		if actual != expected.SHA256 || len(produced) != expected.Bytes {
			t.Errorf("%s drifted under %s\n  want %d bytes, sha256 %s\n  got  %d bytes, sha256 %s\n"+
				"  this is a breaking change - bump the major version, write the changelog entry,\n"+
				"  and do not edit the golden file to make this green",
				name, runtime.Version(), expected.Bytes, expected.SHA256, len(produced), actual)
		}
	}
}

// TestGeneratingTwiceGivesTheSameBytes catches a source of drift the golden
// file cannot see - output that varies between two runs of one binary.
func TestGeneratingTwiceGivesTheSameBytes(t *testing.T) {
	pairs := []struct {
		name string
		fn   func() []byte
	}{
		{"flate_default", func() []byte { return flateAt(t, flate.DefaultCompression) }},
		{"gzip_default", func() []byte { return gzipDefault(t) }},
		{"gzip_store", func() []byte { return gzipStore(t) }},
		{"zip_deflate", func() []byte { return zipDeflate(t) }},
		{"png_default", func() []byte { return pngAt(t, png.DefaultCompression) }},
	}
	for _, p := range pairs {
		if !bytes.Equal(p.fn(), p.fn()) {
			t.Errorf("%s produced different bytes on two runs in one process", p.name)
		}
	}
}

func loadGolden(t *testing.T) golden {
	t.Helper()
	b, err := os.ReadFile(filepath.FromSlash(goldenFile))
	if err != nil {
		t.Fatalf("reading %s: %v\nthe golden values are part of the repository - see .gitignore", goldenFile, err)
	}
	var g golden
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatalf("parsing %s: %v", goldenFile, err)
	}
	if len(g.Paths) == 0 {
		t.Fatalf("%s describes no paths - this guard would pass without checking anything", goldenFile)
	}
	return g
}

func flateAt(t *testing.T, level int) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, level)
	if err != nil {
		t.Fatalf("flate writer: %v", err)
	}
	if _, err := w.Write(payload()); err != nil {
		t.Fatalf("flate write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("flate close: %v", err)
	}
	return buf.Bytes()
}

func gzipDefault(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(payload()); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

// gzipStore is gzip at compression level zero, which TAR.GZ writes.
//
// A different path through flate than the two above, and it was outside this
// file until 2026-08-04. The four toolchain measurement that D11 rests on
// covered flate at level 1 and at the default, gzip at the default, ZIP and
// PNG - not the stored path. TAR.GZ puts its whole archive through it, and its
// exact size arithmetic depends on the block framing this produces, so a drift
// here moves every byte of that format and the sizes with it.
func gzipStore(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := gzip.NewWriterLevel(&buf, gzip.NoCompression)
	if err != nil {
		t.Fatalf("gzip writer: %v", err)
	}
	if _, err := w.Write(payload()); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

func zipDeflate(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	// The modification time is set on purpose. A zero value happens to give a
	// stable hash today, and relying on that is an unstated dependency that
	// breaks without warning.
	w, err := zw.CreateHeader(&zip.FileHeader{
		Name:     "payload.bin",
		Method:   zip.Deflate,
		Modified: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("zip header: %v", err)
	}
	if _, err := w.Write(payload()); err != nil {
		t.Fatalf("zip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func pngAt(t *testing.T, level png.CompressionLevel) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: level}
	if err := enc.Encode(&buf, picture()); err != nil {
		t.Fatalf("png encode: %v", err)
	}
	return buf.Bytes()
}
