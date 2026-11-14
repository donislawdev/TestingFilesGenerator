package guard

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/oracle"
)

// mapSize is the large size every format's map is also read at, beside the
// smallest one it writes. Above it the picture formats take the path that
// codes the picture only while writing, where the generator learns the length
// of its content late (docs/TRUNCATE-2026-11-11.md section 6.2): 16 KiB is the
// line for png, and avif, jxl and the films take it at every size.
const mapSize = 1 << 20

// cutMapSize and cutMapSmall are the sizes a file laid out for a cut is
// written at. 64 KiB, for every setting, is above the 16 KiB line and mostly
// padding already - a megabyte cost the race job of CI about 16 s more for
// nothing new. 8 KiB, with the settings at their defaults, is under the line,
// so png lays out a picture it measured while planning, and has room for
// padding, which the smallest file has none of. The race job is at the edge of
// its timeout (TestTheRacePartsWeighAboutTheSame), which is why not both sizes
// for every setting.
const (
	cutMapSize  = 64 << 10
	cutMapSmall = 8 << 10
)

// mappedFile is one file written through a map recorder, with the map its
// generator marked.
type mappedFile struct {
	name   string
	format string
	path   string
	// setting is the one setting moved from its default, empty for none.
	setting map[string]string
	marked  [][3]any
}

// The map a generator marks while it writes is the map its file's structure
// reads back, for every format, every value of every setting that picks a
// shape, at the smallest size and a large one as a file nothing cuts, and laid
// out for a cut half way and written whole (format.Request.CutAt), where a
// format may put padding on both sides of its content.
//
// The map is what the manifest's damage[].cut is worked out from, and the
// offsets in it are the generator's own word (internal/format/layout.go). The
// second reading is internal/oracle/layout.py, written to each format's rules
// rather than to what the generator meant. A generator that marks nothing is
// content from start to end, which a text is and a picture is not, so a new
// binary format that forgets its marks is red here.
//
// A format that splits its padding for a cut is also cut at the two ends:
// after the first byte, where the content cannot keep its share with any
// padding in front, and before the last byte, where it keeps it only with all
// of the padding in front. Those are the branches that round the front to
// nothing and to everything (docs/TRUNCATE-2026-11-11.md section 8.9), and
// each has to give the layout it promises as well as read back.
//
// Canary: the first map with two regions, with the boundary between them moved
// by a byte, must not be taken for the map read back.
func TestEveryFormatsMapIsTheOneItsStructureReadsBack(t *testing.T) {
	dir := t.TempDir()
	var files []mappedFile
	ends := 0
	for _, f := range format.All() {
		// A format all of whose settings were skipped would leave the sweep
		// without it and the guard green, so each one has to reach a file.
		before := len(files)
		variants := append([]map[string]string{{}}, settingVariants(f)...)
		for n, setting := range variants {
			// A setting the format refuses without another beside it is
			// skipped, as the witness guard does, rather than taking the
			// whole sweep down.
			_, err := f.Generator.Plan(format.Request{Bytes: mapSize, Label: true, Properties: setting})
			var below *format.BelowMinimumError
			if err != nil && !errors.As(err, &below) {
				continue
			}
			smallest := f.SmallestAccepted(format.Request{Label: true, Properties: setting})
			for _, from := range []int64{smallest, max(smallest, mapSize)} {
				size := healthySizeFrom(t, f, setting, from)
				name := fmt.Sprintf("%s-%d-%d", f.ID, n, size)
				files = append(files, writeMapped(t, dir, f, name, setting, size, 0))
			}
			cuts := []int64{cutMapSize}
			if n == 0 {
				cuts = append(cuts, cutMapSmall)
			}
			for _, from := range cuts {
				cut := healthySizeFrom(t, f, setting, max(smallest, from))
				name := fmt.Sprintf("%s-%d-%d-cut", f.ID, n, cut)
				half := writeMapped(t, dir, f, name, setting, cut, cut/2)
				files = append(files, half)
				if n == 0 && from == cutMapSize && paddingOnBothSides(half.marked) {
					at := writeTheEnds(t, dir, f, name, setting, cut)
					files = append(files, at...)
					ends += len(at)
				}
			}
		}
		if len(files) == before {
			t.Errorf("%s: no setting of it was written, so its map was never read back", f.ID)
		}
	}

	reqs := make([]oracle.LayoutRequest, len(files))
	for i, m := range files {
		reqs[i] = oracle.LayoutRequest{Format: m.format, Path: m.path, Settings: m.setting}
	}
	got, available, err := oracle.LayoutBatch(t.Context(), reqs)
	if !available {
		t.Skip("the reader of maps needs python, and there is none on this machine")
	}
	if err != nil {
		t.Fatalf("reading the maps back: %v", err)
	}

	padded, split, canaried := 0, 0, false
	for i, m := range files {
		if got[i].Error != "" {
			t.Errorf("%s: the structure gives no map: %s", m.name, got[i].Error)
			continue
		}
		read := normalRegions(got[i].Regions)
		if !sameMap(m.marked, read) {
			t.Errorf("%s: the generator marked %s and the structure reads %s", m.name, mapNames(m.marked), mapNames(read))
			continue
		}
		if contents := countRole(m.marked, format.RoleContent); contents > 1 {
			t.Errorf("%s: %d regions of content - a cut counts the content as one, and the regions after a cut are never marked", m.name, contents)
		}
		if countRole(m.marked, format.RolePadding) > 0 {
			padded++
		}
		if paddingOnBothSides(m.marked) {
			split++
		}
		if !canaried && len(m.marked) >= 2 {
			canaried = true
			moved := slices.Clone(m.marked)
			moved[0] = [3]any{moved[0][0], moved[0][1], moved[0][2].(int64) + 1}
			moved[1] = [3]any{moved[1][0], moved[1][1].(int64) + 1, moved[1][2].(int64) - 1}
			if sameMap(moved, read) {
				t.Errorf("canary: %s with its first boundary moved by a byte passed for the map read back", m.name)
			}
		}
	}
	// Canary of the order: a mark for bytes already written is reported.
	late := &orderedRecorder{MapRecorder: format.MapRecorder{W: io.Discard}}
	_, _ = late.Write(make([]byte, 10))
	format.Mark(late, format.RoleContent, 5, -1)
	if len(late.late) != 1 {
		t.Errorf("canary: a mark at 5 after 10 B were written was not reported as late")
	}

	// What this asserts, so a sweep that reached no padding cannot pass.
	if !canaried {
		t.Errorf("no file had two regions, so the canary asked nothing")
	}
	if padded == 0 {
		t.Errorf("no file of %d had padding in its map, so the role a cut misses most was never asked", len(files))
	}
	if split == 0 {
		t.Errorf("no file of %d had padding on both sides of its content, so a layout for a cut was never read back", len(files))
	}
	if ends == 0 {
		t.Errorf("no format split its padding at %d B, so the ends of the split were never cut", cutMapSize)
	}
	// Canary of the sides: a map with padding behind its content alone.
	if front, back := paddingSides([][3]any{{string(format.RoleStructure), int64(0), int64(2)},
		{string(format.RoleContent), int64(2), int64(5)}, {string(format.RolePadding), int64(7), int64(3)}}); front || !back {
		t.Errorf("canary: padding behind the content alone read as front %v, back %v", front, back)
	}
	// Canaries of the first end: padding behind a head in front of the
	// content is a miss, and so is a file with no padding behind.
	region := func(role format.Role, at, length int64) [3]any { return [3]any{string(role), at, length} }
	if firstEndLayout([][3]any{region(format.RoleStructure, 0, 2), region(format.RolePadding, 2, 4),
		region(format.RoleContent, 6, 5), region(format.RolePadding, 11, 3)}) {
		t.Errorf("canary: padding between a head and the content passed for the layout due at the first byte")
	}
	if firstEndLayout([][3]any{region(format.RolePadding, 0, 4), region(format.RoleContent, 4, 5), region(format.RoleEnd, 9, 3)}) {
		t.Errorf("canary: a file with no padding behind its content passed for the layout due at the first byte")
	}
	askTheLastEndCanaries(t, dir)
	askTheReaderOfMapsToStop(t, dir)
	t.Logf("%d files, %d with padding, %d of them on both sides of the content, %d cut at the ends of a split", len(files), padded, split, ends)
}

// writeTheEnds writes a file of f that splits its padding for a cut, cut
// after its first byte and before its last, and reports each that does not
// put all of its padding behind the content and in front of it.
func writeTheEnds(t *testing.T, dir string, f format.Descriptor, name string, setting map[string]string, size int64) []mappedFile {
	t.Helper()
	var out []mappedFile
	label := format.Request{Bytes: size, Label: true, Seed: 7}.LabelText(f.ID)
	for _, end := range []struct {
		name string
		at   int64
		due  func(m mappedFile) bool
		says string
	}{
		{"first", 1, func(m mappedFile) bool { return firstEndLayout(m.marked) }, "padding behind the content, and in front of it none - or, where the content would start the file, one padding region that does instead"},
		{"last", size - 1, func(m mappedFile) bool { return lastEndLayout(t, m, label) }, "padding in front of the content only - or, behind it, the label alone where the format keeps it in a field that closes the file"},
	} {
		m := writeMapped(t, dir, f, name+"-"+end.name, setting, size, end.at)
		if !end.due(m) {
			t.Errorf("%s: cut after %d B of %d B it marked %s, where %s was due", m.name, end.at, size, mapNames(m.marked), end.says)
		}
		out = append(out, m)
	}
	return out
}

// firstEndLayout is the layout due for a cut after a file's first byte: its
// padding behind the content, and nothing in front of it. Unless the content
// would start the file, as a package's first part does: then a cut after one
// byte would keep a byte of it where the share is none, so the smallest
// padding region the format writes starts the file and the content follows
// it (docs/TRUNCATE-2026-11-11.md section 10).
func firstEndLayout(m [][3]any) bool {
	front, back := paddingSides(m)
	if !back {
		return false
	}
	return !front || (len(m) > 1 && m[0][0] == string(format.RolePadding) && m[0][1] == int64(0) &&
		m[1][0] == string(format.RoleContent))
}

// lastEndLayout is the layout due for a cut before a file's last byte: all of
// its padding in front of the content. Behind the content it allows one padding
// region, and only when that holds the file's label and nothing else. A ZIP
// keeps its label in the archive comment, which closes the file, so no layout
// can move it in front (docs/TRUNCATE-2026-11-11.md section 10.4).
func lastEndLayout(t *testing.T, m mappedFile, label string) bool {
	t.Helper()
	front, back := paddingSides(m.marked)
	return front && (!back || onlyTheLabelBehind(t, m, label))
}

// onlyTheLabelBehind says the padding behind the content of m is one region,
// and its bytes are label.
func onlyTheLabelBehind(t *testing.T, m mappedFile, label string) bool {
	t.Helper()
	at := slices.IndexFunc(m.marked, func(r [3]any) bool { return r[0] == string(format.RoleContent) })
	var behind [][3]any
	for _, r := range m.marked[at+1:] {
		if r[0] == string(format.RolePadding) {
			behind = append(behind, r)
		}
	}
	if len(behind) != 1 {
		return false
	}
	data, err := os.ReadFile(m.path)
	if err != nil {
		t.Fatalf("%s: %v", m.name, err)
	}
	start, n := behind[0][1].(int64), behind[0][2].(int64)
	return start+n <= int64(len(data)) && string(data[start:start+n]) == label
}

// askTheLastEndCanaries asks lastEndLayout about files made for it: the label
// alone behind the content passes, and padding of its length that is not it,
// or the label with padding beside it, does not.
func askTheLastEndCanaries(t *testing.T, dir string) {
	t.Helper()
	file := func(tail string) mappedFile {
		path := filepath.Join(dir, "canary-last-end")
		if err := os.WriteFile(path, []byte("ppccccc"+tail), 0o600); err != nil {
			t.Fatal(err)
		}
		return mappedFile{name: "canary", path: path, marked: [][3]any{
			{string(format.RolePadding), int64(0), int64(2)}, {string(format.RoleContent), int64(2), int64(5)},
			{string(format.RolePadding), int64(7), int64(len(tail))},
		}}
	}
	if !lastEndLayout(t, file("tfg"), "tfg") {
		t.Errorf("canary: the label alone behind the content failed the layout due at the last byte")
	}
	if lastEndLayout(t, file("tfx"), "tfg") {
		t.Errorf("canary: padding of the label's length that is not the label passed for the layout due at the last byte")
	}
	if lastEndLayout(t, file("tfg "), "tfg") {
		t.Errorf("canary: the label with padding beside it passed for the layout due at the last byte")
	}
}

// askTheReaderOfMapsToStop hands the reader of maps two tar.gz files it must
// refuse rather than read for ever: a squeezed one cut half way, whose last
// Huffman code runs off the end, where the bits read as zeros and a code of
// zeros for a byte decoded again and again, and one whose tar header gives a
// size below zero, which held the walk through the tar in place
// (docs/REVIEW-240-2026-11-13.md). A loop comes back as the guard's time
// running out, a refusal of another kind as the words it gives. And a TIFF
// with its directory first and its last pixel byte gone, which was mapped
// with its pixels running past the end of the file (review of #244).
func askTheReaderOfMapsToStop(t *testing.T, dir string) {
	t.Helper()
	// Letters drawn unevenly and without a period, so the stream codes mostly
	// single bytes in a block of its own codes.
	var text bytes.Buffer
	x := uint32(1)
	for range 4000 {
		x = x*1103515245 + 12345
		text.WriteByte("aaaaaaaabbbcd"[x>>16%13])
	}
	squeezed := gzipped(t, text.Bytes(), gzip.BestCompression)
	header := make([]byte, 512)
	copy(header, "a.txt")
	copy(header[124:], "-0000001000")
	// A TIFF of an odd size has its directory right behind the header.
	tiff := writeEncoded(t, "tiff", 4097, nil)
	cases := []struct {
		format, name, says string
		data               []byte
		setting            map[string]string
	}{
		{"targz", "cut", "runs off the end of the file at bit", squeezed[:len(squeezed)/2], map[string]string{"compression": "best"}},
		{"targz", "below-zero", "gives a size below zero", gzipped(t, append(header, make([]byte, 1024)...), gzip.NoCompression), nil},
		{"tiff", "short", "the strips run to 4097 and the file is 4096 B", tiff[:len(tiff)-1], nil},
	}
	reqs := make([]oracle.LayoutRequest, len(cases))
	for i, c := range cases {
		path := filepath.Join(dir, "canary-"+c.name+"."+c.format)
		if err := os.WriteFile(path, c.data, 0o600); err != nil {
			t.Fatal(err)
		}
		reqs[i] = oracle.LayoutRequest{Format: c.format, Path: path, Settings: c.setting}
	}
	got, _, err := oracle.LayoutBatch(t.Context(), reqs)
	if err != nil {
		t.Fatalf("canary: reading the maps back: %v", err)
	}
	for i, c := range cases {
		if !strings.Contains(got[i].Error, c.says) {
			t.Errorf("canary: the reader of maps answered a %s %s with %q, where %q was due", c.format, c.name, got[i].Error, c.says)
		}
	}
}

// gzipped is data in one gzip member, at level.
func gzipped(t *testing.T, data []byte, level int) []byte {
	t.Helper()
	var out bytes.Buffer
	zw, err := gzip.NewWriterLevel(&out, level)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := zw.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// writeMapped writes one healthy file of f through a map recorder, laid out
// for a cut at cutAt when that is not zero.
func writeMapped(t *testing.T, dir string, f format.Descriptor, name string, setting map[string]string, size, cutAt int64) mappedFile {
	t.Helper()
	p, err := f.Generator.Plan(format.Request{Bytes: size, Label: true, Seed: 7, CutAt: cutAt, Properties: setting})
	if err != nil {
		t.Fatalf("%s: planning: %v", name, err)
	}
	path := filepath.Join(dir, name+"."+f.ID)
	fh, err := os.Create(path)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	rec := &orderedRecorder{MapRecorder: format.MapRecorder{W: fh}}
	werr := f.Generator.Write(t.Context(), rec, p)
	if cerr := fh.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		t.Fatalf("%s: writing: %v", name, werr)
	}
	for _, late := range rec.late {
		t.Errorf("%s: %s - a cut there stops the generator before the mark, and the map says nothing of the bytes it cut", name, late)
	}
	return mappedFile{name: name, format: f.ID, path: path, setting: setting, marked: mergedRegions(rec.Map(size, true))}
}

// orderedRecorder is a map recorder that also counts the bytes reaching it,
// and keeps every mark that came after a byte of its own region had already
// arrived.
//
// That is the order a cut needs. A generator stops at the cut, so a region
// marked after its bytes went out is never marked when the cut falls in it -
// and a file with no marks at all reads as content from start to end.
type orderedRecorder struct {
	format.MapRecorder
	n    int64
	late []string
}

func (o *orderedRecorder) Write(p []byte) (int, error) {
	n, err := o.MapRecorder.Write(p)
	o.n += int64(n)
	return n, err
}

func (o *orderedRecorder) MarkRegion(r format.Region) {
	if o.n > r.At {
		o.late = append(o.late, fmt.Sprintf("%s at %d was marked after %d B had been written", r.Role, r.At, o.n))
	}
	o.MapRecorder.MarkRegion(r)
}

// mergedRegions is a map as [role, at, length], neighbours of one role as one
// and empty regions dropped - the shape layout.py answers in.
func mergedRegions(regions []format.Region) [][3]any {
	var out [][3]any
	for _, r := range regions {
		if r.Length == 0 {
			continue
		}
		if n := len(out); n > 0 && out[n-1][0] == string(r.Role) && out[n-1][1].(int64)+out[n-1][2].(int64) == r.At {
			out[n-1][2] = out[n-1][2].(int64) + r.Length
			continue
		}
		out = append(out, [3]any{string(r.Role), r.At, r.Length})
	}
	return out
}

// normalRegions is a map read back, with its numbers as int64 rather than the
// float64 JSON gives.
func normalRegions(in [][3]any) [][3]any {
	out := make([][3]any, len(in))
	for i, r := range in {
		role, _ := r[0].(string)
		at, _ := r[1].(float64)
		length, _ := r[2].(float64)
		out[i] = [3]any{role, int64(at), int64(length)}
	}
	return out
}

func sameMap(a, b [][3]any) bool {
	return slices.Equal(a, b)
}

func countRole(m [][3]any, role format.Role) int {
	n := 0
	for _, r := range m {
		if r[0] == string(role) {
			n++
		}
	}
	return n
}

// mapNames is the map as words, for a message.
func mapNames(m [][3]any) string {
	var parts []string
	for _, r := range m {
		parts = append(parts, fmt.Sprintf("%s@%d+%d", r[0], r[1], r[2]))
	}
	return strings.Join(parts, " ")
}

// paddingOnBothSides says whether a map has padding before its content and
// after it, the layout a format writes for a cut (format.Request.CutAt).
func paddingOnBothSides(m [][3]any) bool {
	front, back := paddingSides(m)
	return front && back
}

// paddingSides says whether a map has padding in front of its content, and
// whether it has padding behind it.
func paddingSides(m [][3]any) (front, back bool) {
	at := slices.IndexFunc(m, func(r [3]any) bool { return r[0] == string(format.RoleContent) })
	if at < 0 {
		return false, false
	}
	isPadding := func(r [3]any) bool { return r[0] == string(format.RolePadding) }
	return slices.ContainsFunc(m[:at], isPadding), slices.ContainsFunc(m[at+1:], isPadding)
}
