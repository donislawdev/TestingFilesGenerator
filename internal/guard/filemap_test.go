package guard

import (
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
	t.Logf("%d files, %d with padding, %d of them on both sides of the content, %d cut at the ends of a split", len(files), padded, split, ends)
}

// writeTheEnds writes a file of f that splits its padding for a cut, cut
// after its first byte and before its last, and reports each that does not
// put all of its padding behind the content and in front of it.
func writeTheEnds(t *testing.T, dir string, f format.Descriptor, name string, setting map[string]string, size int64) []mappedFile {
	t.Helper()
	var out []mappedFile
	for _, end := range []struct {
		name        string
		at          int64
		front, back bool
	}{{"first", 1, false, true}, {"last", size - 1, true, false}} {
		m := writeMapped(t, dir, f, name+"-"+end.name, setting, size, end.at)
		if front, back := paddingSides(m.marked); front != end.front || back != end.back {
			t.Errorf("%s: cut after %d B of %d B it marked %s - padding in front %v and behind %v, where %v and %v were due",
				m.name, end.at, size, mapNames(m.marked), front, back, end.front, end.back)
		}
		out = append(out, m)
	}
	return out
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
