package guard

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"runtime"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/donislawdev/TestingFilesGenerator/internal/tool"
	"github.com/donislawdev/TestingFilesGenerator/internal/tool/archivescan"
)

// The archives inside an archive, and the bounds of what the scanner holds:
// a trap in an archive inside is a trap, the archives together can be built to
// explode, and nothing an archive holds makes the report, or what is held to
// read it, grow without a bound. docs/SKANER-W-GLAB-2026-10-19.md.

// noteItems is every item of the note with this key in a result, as the
// command line prints it.
func noteItems(r tool.Result, id string) []string {
	for _, n := range r.Notes {
		if n.ID == id {
			return n.Items
		}
	}
	return nil
}

// holdsText says whether one item of a list holds this text.
func holdsText(items []string, text string) bool {
	return slices.ContainsFunc(items, func(item string) bool { return strings.Contains(item, text) })
}

// chainOf is an archive inside an archive, depth times, around one file.
func chainOf(t *testing.T, depth int) []byte {
	t.Helper()
	data := zipOf(t, zipPart{name: "bottom.txt", body: "hello"})
	for level := range depth {
		data = zipOf(t, zipPart{name: fmt.Sprintf("level%02d.zip", level), body: string(data)})
	}
	return data
}

// storedZip is a ZIP of one entry stored as it is, so the archive is as large
// as what it holds.
func storedZip(t *testing.T, name string, data []byte) []byte {
	t.Helper()
	var out strings.Builder
	z := zip.NewWriter(&out)
	w, err := z.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
	if err == nil {
		_, err = w.Write(data)
	}
	if err == nil {
		err = z.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	return []byte(out.String())
}

// bigInside is a gzip of a ZIP of 300 MiB of zeros: an archive inside larger
// than the scanner holds, whose size nothing says before it is read.
func bigInside(t *testing.T) []byte {
	t.Helper()
	var out strings.Builder
	g := gzip.NewWriter(&out)
	z := zip.NewWriter(g)
	w, err := z.CreateHeader(&zip.FileHeader{Name: "zeros.bin", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	chunk := make([]byte, 1<<20)
	for range 300 {
		if _, err := w.Write(chunk); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := g.Close(); err != nil {
		t.Fatal(err)
	}
	return []byte(out.String())
}

// roomInside is a gzip of a.zip, which holds 120 MiB of zeros and b.zip, which
// holds 100 MiB: b.zip fits in what the scanner holds, and not in what is
// left of it while a.zip is held.
func roomInside(t *testing.T) []byte {
	t.Helper()
	var out strings.Builder
	g := gzip.NewWriter(&out)
	a := zip.NewWriter(g)
	zeros := func(w io.Writer, mib int) {
		chunk := make([]byte, 1<<20)
		for range mib {
			if _, err := w.Write(chunk); err != nil {
				t.Fatal(err)
			}
		}
	}
	filler, err := a.CreateHeader(&zip.FileHeader{Name: "filler.bin", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	zeros(filler, 120)
	inner, err := a.CreateHeader(&zip.FileHeader{Name: "b.zip", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	bz := zip.NewWriter(inner)
	w, err := bz.CreateHeader(&zip.FileHeader{Name: "zeros.bin", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	zeros(w, 100)
	for _, c := range []io.Closer{bz, a, g} {
		if err := c.Close(); err != nil {
			t.Fatal(err)
		}
	}
	return []byte(out.String())
}

// TestTheArchiveScanLooksIntoTheArchivesInside holds the scanner to reading
// every archive inside the one given, and to saying a trap in one with the way
// to it - in the notes both surfaces show, and in the report of the archive
// inside that --json carries.
func TestTheArchiveScanLooksIntoTheArchivesInside(t *testing.T) {
	bad := zipOf(t, zipPart{name: "../evil.txt", body: "x"})
	emptyBzip2 := string([]byte{'B', 'Z', 'h', '9', 0x17, 0x72, 0x45, 0x38, 0x50, 0x90, 0, 0, 0, 0})
	// Bytes no two pieces of a megabyte share, so an archive read back from
	// the wrong piece fails its checksum.
	pattern := make([]byte, 3<<20+7)
	for i := range pattern {
		pattern[i] = byte(i % 251)
	}
	member, gz := gzipOf(t, make([]byte, 64<<20)), []byte(nil)
	for range 10 {
		gz = append(gz, member...)
	}
	for _, c := range []struct {
		what, file string
		data       []byte
		// note and item are a note the result has to hold, and words one
		// of its items has to hold.
		note, item string
		safe       bool
	}{
		{what: "a trap in an archive inside", file: "inside.zip", data: zipOf(t, zipPart{name: "inner.zip", body: string(bad)}),
			note: "leaves_folder", item: "inner.zip > ../evil.txt"},
		{what: "a trap two archives deep", file: "twice.zip", data: zipOf(t, zipPart{name: "a.tar", body: string(tarOf(t, false,
			tarPart{name: "b.zip", body: string(bad)}))}), note: "leaves_folder", item: "a.tar > b.zip > ../evil.txt"},
		{what: "archives inside, deeper than the scanner looks", file: "chain.zip", data: chainOf(t, archivescan.MostDepth+2),
			note: "unchecked", item: "deeper than this tool looks"},
		{what: "an archive inside larger than the scanner holds", file: "big.zip.gz", data: bigInside(t),
			note: "unchecked", item: "larger than the"},
		{what: "an archive inside larger than what is left while the one around it is held", file: "room.zip.gz", data: roomInside(t),
			note: "unchecked", item: "what is left of the"},
		{what: "an archive inside the scanner does not read", file: "bz.zip", data: zipOf(t, zipPart{name: "x.bz2", body: emptyBzip2}),
			note: "unchecked", item: "bzip2 archive"},
		{what: "an archive inside held in several pieces", file: "pieces.zip", safe: true, note: "nested", item: "big.zip",
			data: zipOf(t, zipPart{name: "big.zip", body: string(storedZip(t, "data.bin", pattern))})},
		{what: "text that begins with the letters of bzip2", file: "text.zip", data: zipOf(t, zipPart{name: "x.txt", body: "BZh9 is a word, not bzip2"}),
			safe: true},
		// Two archives of 640 MiB of zeros each: neither is built to explode
		// on its own, and together they are.
		{what: "archives inside that explode only together", file: "together.zip", data: zipOf(t, zipPart{name: "a.gz", body: string(gz)},
			zipPart{name: "b.gz", body: string(gz)}), note: "bomb", item: "with the archives inside it"},
	} {
		report, r := scanArchive(t, archiveFile(t, c.file, c.data), nil)
		if report.Safe != c.safe || (r.Concluded == "safe") != c.safe {
			t.Errorf("%s (%s): safe is %v and the conclusion %q, and it is %v", c.what, c.file, report.Safe, r.Concluded, c.safe)
		}
		if c.note != "" && !holdsText(noteItems(r, c.note), c.item) {
			t.Errorf("%s (%s): the note %s says %q, and has to say %q", c.what, c.file, c.note, noteItems(r, c.note), c.item)
		}
	}
}

// TestTheArchiveScanReportsAnArchiveInsideOnItsOwn holds --json to naming in
// found what is in the archive given, and carrying the archive inside that is
// not safe as a report of its own, with the names in it.
func TestTheArchiveScanReportsAnArchiveInsideOnItsOwn(t *testing.T) {
	inner := zipOf(t, zipPart{name: "../evil.txt", body: "x"})
	report, _ := scanArchive(t, archiveFile(t, "outer.zip", zipOf(t, zipPart{name: "ok.txt"}, zipPart{name: "inner.zip", body: string(inner)})), nil)
	if _, ok := report.Found["leaves_folder"]; ok || !slices.Equal(report.Found["nested"], []string{"inner.zip"}) {
		t.Errorf("found is %v: it names what is in the archive given, so inner.zip and nothing about ../evil.txt", report.Found)
	}
	if len(report.Inside) != 1 || report.Inside[0].File != "inner.zip" || !slices.Equal(report.Inside[0].Found["leaves_folder"], []string{"../evil.txt"}) {
		t.Errorf("inside is %+v, and has to carry inner.zip with ../evil.txt leaving the folder", report.Inside)
	}
	if report.Safe || report.Inside[0].Safe {
		t.Errorf("an archive with a trap inside is not safe, and neither is the archive inside")
	}
}

// TestTheArchiveScanReportStaysTheSameSizeWhateverAnArchiveHolds holds the
// report to its bounds: a note lists so many findings and counts the rest,
// --json lists as many and says how many more, and a name is cut where a
// report stops showing it, with its length.
func TestTheArchiveScanReportStaysTheSameSizeWhateverAnArchiveHolds(t *testing.T) {
	var parts []zipPart
	for i := range archivescan.MostListed + 50 {
		parts = append(parts, zipPart{name: fmt.Sprintf("q?%03d", i)})
	}
	long := strings.Repeat("x", 2000)
	// One byte, then characters of two: the cut at 1024 falls inside one.
	polish := "a" + strings.Repeat(string(rune(0x105)), 600)
	parts = append(parts, zipPart{name: long})
	report, r := scanArchive(t, archiveFile(t, "many.zip", zipOf(t, parts...)), nil)
	listed := report.Found["unwritable"]
	if len(listed) != archivescan.MostListed || report.More["unwritable"] != 51 {
		t.Errorf("--json lists %d names that cannot be written and says %d more, and there are %d", len(listed), report.More["unwritable"], len(parts))
	}
	items := noteItems(r, "unwritable")
	if len(items) != archivescan.MostListed+1 || items[len(items)-1] != "and 51 more" {
		t.Errorf("the note has %d items ending %q, and has to list %d and count the rest", len(items), items[len(items)-1], archivescan.MostListed)
	}
	cut, _ := scanArchive(t, archiveFile(t, "long.zip", zipOf(t, zipPart{name: long})), nil)
	if got := cut.Found["unwritable"]; len(got) != 1 || len(got[0]) != archivescan.MostShownName {
		t.Errorf("a name of 2000 bytes is listed as %d bytes, and is cut at %d", len(got[0]), archivescan.MostShownName)
	}
	_, r = scanArchive(t, archiveFile(t, "long.zip", zipOf(t, zipPart{name: long})), nil)
	if !holdsText(noteItems(r, "unwritable"), "(2 000 B in all)") {
		t.Errorf("the note shows %q, and has to say the name is cut and how long it is", noteItems(r, "unwritable"))
	}
	whole, _ := scanArchive(t, archiveFile(t, "polish.zip", zipOf(t, zipPart{name: polish})), nil)
	if got := whole.Found["unwritable"]; len(got) != 1 || len(got[0]) != archivescan.MostShownName-1 || !utf8.ValidString(got[0]) {
		t.Errorf("a name cut inside a character is listed as %d bytes, valid UTF-8 %v - the cut goes back to where the character starts", len(got[0]), utf8.ValidString(got[0]))
	}
}

// TestTheArchiveScanHoldsNoMoreOfANameThanTheName measures the heap while a
// TAR.GZ is read whose every entry is named in a header of a megabyte, and is
// an archive with a trap in it. archive/tar hands such a name back as a slice
// of the whole header, and holding the slice - to compare names, or as the
// name of an archive inside kept for the report - held every header: 200 of
// them were 200 MB, measured at 1.3 GB on 900
// (docs/SKANER-W-GLAB-2026-10-19.md §1).
func TestTheArchiveScanHoldsNoMoreOfANameThanTheName(t *testing.T) {
	var buf strings.Builder
	g := gzip.NewWriter(&buf)
	tw := tar.NewWriter(g)
	pad := strings.Repeat("p", 1_000_000)
	bad := zipOf(t, zipPart{name: "../evil.txt", body: "x"})
	for i := range 200 {
		name := fmt.Sprintf("f%03d.zip", i)
		if err := tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(bad)), Format: tar.FormatPAX,
			PAXRecords: map[string]string{"comment": pad, "path": name}}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(bad); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := g.Close(); err != nil {
		t.Fatal(err)
	}
	path := archiveFile(t, "padded.tar.gz", []byte(buf.String()))
	d, err := tool.Get(archivescan.ID)
	if err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	var before, now runtime.MemStats
	runtime.ReadMemStats(&before)
	most, calls := uint64(0), 0
	sample := func(int64, int64) {
		if calls++; calls%4 == 0 {
			runtime.GC()
			runtime.ReadMemStats(&now)
			most = max(most, now.HeapAlloc)
		}
	}
	r, err := d.Start(context.Background(), tool.Request{Inputs: map[string]string{archivescan.InputArchive: path}}, sample)
	if err != nil || r.Concluded != "unsafe" || calls < 4 {
		t.Fatalf("the padded archive: %v, concluded %q, %d reports of progress", err, r.Concluded, calls)
	}
	if grew := int64(most) - int64(before.HeapAlloc); grew > 64<<20 {
		t.Errorf("reading 200 names of 8 bytes held %d MB at the most - a name kept holds its whole header", grew>>20)
	}
}
