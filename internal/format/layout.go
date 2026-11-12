package format

import "io"

// The map of a file: which of its bytes are the content a reader hands back,
// and which are the structure around it, the padding that makes up the size
// asked for, and the end of the file.
//
// A damage aims at a ROLE of the bytes and is handed an offset. truncate cut
// "the end of the content" and hit the padding instead, because the padding
// is most of a large file and sits after the content in 13 formats of 32 -
// missing=90 and missing=1 gave the same file with the whole picture in it
// (docs/TRUNCATE-2026-11-11.md sections 1 and 2). The map is what lets the
// manifest say where a cut fell, and the next damages aim where they mean to.
//
// Marked by the generator WHILE it writes, never worked out in the plan. Six
// formats choose their picture without coding it when planning, so the length
// of their content is not known before the writer has it, and a map in the plan
// would code every damaged file of them twice (section 6.2). The offsets are
// the generator's own rather than counted by the writer it is handed: a PNG's
// end is held back in a buffer while the picture streams out, and archive/zip
// buffers everything it writes, so a count taken on the engine's side sees the
// bytes later than they were written (section 6.9). Whether the offsets are
// true is measured by a second reading of the file's structure, the guard
// TestEveryFormatsMapIsTheOneItsStructureReadsBack.

// Role is what a region of a file is for. The words reach the manifest
// (damage[].cut.in), so each is a public name under untouchable rule 10.
type Role string

const (
	// RoleStructure is the bytes a reader needs to find the content: a
	// signature, headers, a table of what is where.
	RoleStructure Role = "structure"
	// RoleContent is what a reader hands back: the pixels, the samples, the
	// pages, the records, the entries of an archive.
	RoleContent Role = "content"
	// RolePadding is the bytes that make up the size asked for, which no
	// reader hands back.
	RolePadding Role = "padding"
	// RoleEnd is what closes the file: an end marker, a trailer, a central
	// directory.
	RoleEnd Role = "end"
)

// Region is one stretch of the bytes a generator writes.
type Region struct {
	Role Role
	// At is the offset of the region's first byte in the file the generator
	// writes - before any damage, so a cut file has the map of its whole.
	At int64
	// Length is how many bytes the region takes, or -1 when the generator does
	// not know yet. The region then runs to the next one marked, or to the end
	// of the file.
	Length int64
}

// Mapper is a writer that keeps the map of the file written through it. The
// engine hands one to a generator when a damage needs the map, and a guard
// hands one when it reads the map back.
type Mapper interface {
	MarkRegion(Region)
}

// Mark says that a region of role starts at offset at of the file written to
// w, length bytes long or -1 when that is not known yet. Marked before the
// region's first byte where its extent is known, and as soon as it is known
// otherwise, because a generator stops at a cut and a mark after it never
// arrives.
//
// w is the writer the generator was handed. A file written into another one -
// a picture inside an archive - writes to a writer of the archive's own, so
// its marks go nowhere instead of into the archive's map. Nothing at all when
// nobody keeps a map, which is every file not damaged.
func Mark(w io.Writer, role Role, at, length int64) {
	if m, ok := w.(Mapper); ok {
		m.MarkRegion(Region{Role: role, At: at, Length: length})
	}
}

// MapRecorder passes every byte to W and keeps the regions marked on it.
type MapRecorder struct {
	W       io.Writer
	regions []Region
}

func (m *MapRecorder) Write(p []byte) (int, error) { return m.W.Write(p) }

// MarkRegion keeps r.
func (m *MapRecorder) MarkRegion(r Region) { m.regions = append(m.regions, r) }

// Map is the regions marked, with each length not stated filled in from the
// next region, or from whole for the last one.
//
// whole is how long the file written would have been, whether or not it was
// all written. A file whose generator marked nothing is content from start to
// end, which is what a text is: its words are the file. A region whose end is
// not known - the last one marked, of a file cut before the next mark - keeps
// -1 when complete is false, so a cut inside it says "not known" rather than
// stretching the region to the end of a whole that was never written.
func (m *MapRecorder) Map(whole int64, complete bool) []Region {
	if len(m.regions) == 0 {
		return []Region{{Role: RoleContent, At: 0, Length: whole}}
	}
	out := make([]Region, len(m.regions))
	copy(out, m.regions)
	for i := range out {
		if out[i].Length >= 0 {
			continue
		}
		switch {
		case i+1 < len(out):
			out[i].Length = out[i+1].At - out[i].At
		case complete:
			out[i].Length = whole - out[i].At
		}
	}
	return out
}
