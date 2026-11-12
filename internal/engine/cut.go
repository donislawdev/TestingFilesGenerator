package engine

import (
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// Where a cut fell in the file it was made in, for the manifest
// (damage[].cut, docs/TRUNCATE-2026-11-11.md section 6.9).
//
// Found out while writing rather than planned. The generator marks the map of
// the file as it writes it (format.Mark), and the map is all a cut needs: the
// role of the first byte it drops, and how much of the content lies before it.

// wrote is what writing a file found out that its manifest entry carries.
type wrote struct {
	sha string
	// cut is where the damage that cut the file fell, nil for a file nothing
	// cut.
	cut *manifest.Cut
}

// cutMap is the map a file's generator marks while it writes, and the offset
// the cut falls at.
type cutMap struct {
	format.MapRecorder
	at int64
}

// cutMapper is sink with a map recorder around it, and the recorder - or sink
// and nil when nothing cuts the file, which leaves every other file written
// exactly as before.
func cutMapper(f PlannedFile, sink io.Writer) (io.Writer, *cutMap) {
	if !f.Damaged() {
		return sink, nil
	}
	_, at, ok := f.Target.Damage.Cut(f.Plan.Bytes)
	if !ok {
		return sink, nil
	}
	m := &cutMap{MapRecorder: format.MapRecorder{W: sink}, at: at}
	return m, m
}

// cut is where the cut fell, nil for a file nothing cut. The map is not
// complete: the generator stopped at the cut, so a region it had not ended by
// then has no end it can be given.
func (m *cutMap) cut(f PlannedFile) *manifest.Cut {
	if m == nil {
		return nil
	}
	return cutIn(m.Map(wholeOf(f), false), m.at)
}

// wholeOf is how long the file the generator was asked for is. A file cut
// short is planned through the damage, which asks the generator for more than
// reaches the disk.
func wholeOf(f PlannedFile) int64 {
	if m, ok := f.Plan.Memo.(damagedMemo); ok {
		return m.plan.Bytes
	}
	return f.Plan.Bytes
}

// cutIn is where a cut at offset at fell in the map of a file.
//
// The role is that of the region the first dropped byte lies in, and a gap the
// map left is counted to the region before it - a map with gaps is a defect the
// guard TestEveryFormatsMapIsTheOneItsStructureReadsBack catches, and here it
// costs a word rather than a file. The content's length is stated only when
// every region of content has a known end: a generator stopped at the cut never
// marks what lies after it, and a total missing those would be a guess.
func cutIn(regions []format.Region, at int64) *manifest.Cut {
	c := &manifest.Cut{In: string(format.RoleContent)}
	var total int64
	known, any := true, false
	for _, r := range regions {
		if r.At <= at {
			c.In = string(r.Role)
		}
		if r.Role != format.RoleContent {
			continue
		}
		any = true
		known = known && r.Length >= 0
		total += max(0, r.Length)
		c.ContentKeptBytes += max(0, min(at, endOf(r, at))-r.At)
	}
	if known && any {
		c.ContentBytes = &total
	}
	return c
}

// endOf is where r ends, or at for a region whose end is not known: the last
// one marked, so the cut lies inside it.
func endOf(r format.Region, at int64) int64 {
	if r.Length < 0 {
		return at
	}
	return r.At + r.Length
}

// withCutNote is notes with the note a cut that missed the content gets.
func (res *Result) withCutNote(notes []manifest.Note, c *manifest.Cut) []manifest.Note {
	if code, said, ok := cutNote(c); ok {
		return append(notes, res.note(code, said))
	}
	return notes
}

// cutNote is the note a cut that missed the content gets, or false.
//
// Only for a cut in the padding, the case that looks like a cut and is not one:
// a picture cut at 90% that every viewer shows whole. Said in two ways, because
// padding before the content takes all of it with any share.
func cutNote(c *manifest.Cut) (string, core.Said, bool) {
	if c == nil || c.In != string(format.RolePadding) {
		return "", core.Said{}, false
	}
	if c.ContentKeptBytes == 0 {
		return cutInPadding, core.Says("engine.CutBeforeContent",
			"The cut fell in the padding before the content, so none of the content is left."), true
	}
	return cutInPadding, core.Says("engine.CutAfterContent",
		"The cut fell in the padding after the content, so the content is whole and only the end of the file is missing."), true
}

// cutInPadding is the code of that note. A public name under untouchable rule
// 10 (owner's call, 2026-11-12).
const cutInPadding = "cut_in_padding"

// forecastCut is the longest cut entry a file can be given, for the record a
// run is told before its first byte (PredictRecord).
//
// The real one is known only after writing, so this is the most it can come
// to: the padding, whose note is the longer of the two, and both numbers as
// long as the whole file, which neither can pass. More than the run writes,
// which is the safe side for the free space it is held against.
func forecastCut(f PlannedFile) *manifest.Cut {
	whole := wholeOf(f)
	return &manifest.Cut{In: string(format.RolePadding), ContentBytes: &whole, ContentKeptBytes: whole}
}
