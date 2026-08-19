package guard

import (
	"archive/zip"
	"bytes"
	"context"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/oracle"
)

// An Office package can be a faultless ZIP and still not be a document.
//
// Every other guard here feeds the checker a file this tool just made, so the
// checker only ever sees good input and a check that was deleted would look
// exactly like a check that passes. That is not hypothetical: the three things
// broken below are the three defects that actually happened while these formats
// were being written, and two of them were invisible to every ZIP tool on the
// machine.
//
//   - a part swapped for well formed XML that is not a document. 7-Zip says
//     "Everything is Ok" and so does Python. Only a reader of the FORMAT
//     notices, which is why an independent library reads the package back.
//   - an entry carrying its sizes after the data. Legal ZIP, read by 7-Zip,
//     Explorer and Python, refused by two of LibreOffice's three import
//     filters and accepted by the third.
//   - an archive comment above the measured 512 B. Accepted by every ZIP
//     reader there is and by no Office reader.
//
// Feeding the checker something broken is the only way to know it still says
// no. Without this the read back could be deleted and every run stays green.
func TestTheStructuralCheckerRefusesAPackageThatIsNotADocument(t *testing.T) {
	dir := t.TempDir()

	swapped := map[string]string{
		"docx": "word/document.xml",
		"xlsx": "xl/worksheets/sheet1.xml",
		"pptx": "ppt/slides/slide1.xml",
	}

	checked := 0
	for id, part := range swapped {
		d, err := format.Get(id)
		if err != nil {
			t.Fatalf("%s is not registered - this guard would pass without checking anything", id)
		}

		good := buildPackage(t, d)

		// The checker has to accept what this tool produces, or the three
		// refusals below would prove nothing.
		if res := checkPackage(t, dir, id+"-good"+d.Extension, good); res.Err != nil {
			if !res.Available {
				t.Skip("the structural checker is not available here")
			}
			t.Fatalf("%s: the checker refused a file this tool just made: %v", id, res.Err)
		}

		for _, broken := range []struct {
			what string
			make func([]byte) []byte
		}{
			{"a part that is not a document", func(b []byte) []byte {
				return rewritePackage(t, b, part,
					[]byte(`<?xml version="1.0" encoding="UTF-8"?><nonsense/>`), false, 0)
			}},
			{"an entry with its sizes after the data", func(b []byte) []byte {
				return rewritePackage(t, b, "", nil, true, 0)
			}},
			{"an archive comment past the measured limit", func(b []byte) []byte {
				return rewritePackage(t, b, "", nil, false, 600)
			}},
		} {
			res := checkPackage(t, dir, id+"-"+d.Extension, broken.make(good))
			if !res.Available {
				t.Skip("the structural checker is not available here")
			}
			if res.Err == nil {
				t.Errorf("%s with %s passed the structural checker, which said: %s",
					id, broken.what, res.Output)
			}
			checked++
		}
	}

	if checked == 0 {
		t.Fatal("nothing was broken on purpose - this guard would pass without checking anything")
	}
	t.Logf("%d deliberately broken package(s) refused", checked)
}

func buildPackage(t *testing.T, d format.Descriptor) []byte {
	t.Helper()
	plan, err := d.Generator.Plan(format.Request{Bytes: d.MinBytes + 4096, Seed: 4242, Label: true})
	if err != nil {
		t.Fatalf("%s: planning failed: %v", d.ID, err)
	}
	var buf bytes.Buffer
	if err := d.Generator.Write(context.Background(), &buf, plan); err != nil {
		t.Fatalf("%s: writing failed: %v", d.ID, err)
	}
	return buf.Bytes()
}

// rewritePackage repacks a package, optionally swapping one part, optionally
// letting the writer put the sizes after the data, and optionally hanging a
// comment on the end.
func rewritePackage(t *testing.T, body []byte, swap string, with []byte, descriptor bool, comment int) []byte {
	t.Helper()
	src, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("reading the package back: %v", err)
	}

	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	if comment > 0 {
		if err := zw.SetComment(string(bytes.Repeat([]byte("x"), comment))); err != nil {
			t.Fatalf("setting a comment of %d B: %v", comment, err)
		}
	}
	for _, item := range src.File {
		data := with
		if item.Name != swap {
			r, err := item.Open()
			if err != nil {
				t.Fatalf("opening %s: %v", item.Name, err)
			}
			data, err = io.ReadAll(r)
			r.Close()
			if err != nil {
				t.Fatalf("reading %s: %v", item.Name, err)
			}
		}

		head := &zip.FileHeader{Name: item.Name, Method: item.Method, Modified: item.Modified}
		var w io.Writer
		if descriptor {
			// CreateHeader is what sets the flag. That is the whole point of
			// this case.
			w, err = zw.CreateHeader(head)
			if err != nil {
				t.Fatalf("creating %s: %v", item.Name, err)
			}
			if _, err := w.Write(data); err != nil {
				t.Fatalf("writing %s: %v", item.Name, err)
			}
			continue
		}
		head.Method = zip.Store
		head.CRC32 = crc32.ChecksumIEEE(data)
		head.UncompressedSize64 = uint64(len(data))
		head.CompressedSize64 = uint64(len(data))
		w, err = zw.CreateRaw(head)
		if err != nil {
			t.Fatalf("creating %s: %v", item.Name, err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatalf("writing %s: %v", item.Name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("closing the package: %v", err)
	}
	return out.Bytes()
}

func checkPackage(t *testing.T, dir, name string, body []byte) oracle.Result {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	kind := filepath.Ext(name)
	return oracle.Strict(kind[1:], path)
}
