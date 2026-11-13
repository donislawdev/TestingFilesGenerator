package zip

import (
	stdzip "archive/zip"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/archive"
)

// fillerEndName is the entry of padding behind the files of an archive laid out
// for a cut, where fillerName stands in front of them. Two names, because two
// entries of one name are one file overwritten when the archive is unpacked.
const fillerEndName = "tfg-padding-end.bin"

// split is how an archive's padding is laid out around its files for a cut
// (settleForACut), and its zero value is the archive nothing cuts.
type split struct {
	// front says an entry of padding stands in front of the files, behind the
	// directories, and frontBytes is how much padding it holds.
	front      bool
	frontBytes int64
	// sized says content is how long the files' entries come to. The files are
	// marked with it, so that a cut inside them says how much of them there
	// was: a cut stops the writing before the mark that would end them.
	sized   bool
	content int64
	// freed is how much shorter the compressor makes the files of a squeezed
	// archive, measured before it is written.
	freed int64
}

// contentLength is the length the files are marked with, -1 for up to the next
// mark.
func (s split) contentLength() int64 {
	if !s.sized {
		return -1
	}
	return s.content
}

// settleForACut lays the padding of an archive a damage will cut after its
// first cutAt bytes (format.Request.CutAt) out around its files, so that the
// cut keeps the share of them it keeps of the whole archive.
//
// With all of its padding behind the files, as an archive nothing cuts has it,
// a large archive cut at any share kept every file and lost only padding.
// APPNOTE names no order of entries and a reader finds them through the central
// directory: Python's zipfile, 7-Zip and bsdtar read an archive whose padding
// entry comes first as they read it today (docs/TRUNCATE-2026-11-11.md section
// 10.3). So an entry of padding goes in front of the files and behind the
// directories, which stay structure.
//
// A stored archive is settled here. A squeezed one is settled as it is written
// (settleSqueezedForACut), because how long its files come to is known only
// once they are squeezed, and planning does not squeeze.
func settleForACut(m *memo, p *format.Plan, cutAt, target int64, label string) error {
	m.cutAt = cutAt
	if !m.cuts(target) || m.squeeze.On() {
		return nil
	}
	c, err := measureCut(*m, label)
	if err != nil {
		return err
	}
	if err := m.splitPadding(c, target, 0, label); err != nil {
		return err
	}
	if m.split.front {
		p.Properties["padding_entry"] = fillerName
	}
	return nil
}

// settleSqueezedForACut is settleForACut for a squeezed archive, as it is
// written. Its files are squeezed into a counter first, to learn how long they
// come to, and the writing pass then has to come to the same
// (writeCompressedFiller). That generates every file inside twice, and only for
// a squeezed archive a damage cuts. Nothing for any other archive.
func (m *memo) settleSqueezedForACut(ctx context.Context, target int64) error {
	if !m.squeeze.On() || !m.cuts(target) {
		return nil
	}
	// padCompressed leaves the comment the label alone.
	label := m.comment
	c, err := measureCut(*m, label)
	if err != nil {
		return err
	}
	squeezed, err := squeezedFiles(ctx, *m)
	if err != nil {
		return err
	}
	return m.splitPadding(c, target, c.content-squeezed, label)
}

// cuts says the archive is laid out for a cut: one falls inside it, and there
// are files for it to keep a share of.
func (m memo) cuts(target int64) bool {
	return m.cutAt > 0 && m.cutAt < target && len(m.children) > 0
}

// splitPadding lays the padding out around the files for the cut at m.cutAt,
// in an archive of target bytes whose files the compressor makes freed bytes
// shorter, nought when they are stored (docs/TRUNCATE-2026-11-11.md section
// 10.4).
//
// The entry in front costs its header before any padding, so the front is
// nothing or at least that, and grows where it cannot land exactly - the cut
// then takes a little more of the files, the "at least" missing promises.
// Where the padding is too short for the files the cut should take, all of it
// goes in front, and the cut keeps more of them by the share of what stands
// behind. The files are marked with their length whether or not anything goes
// in front, so a cut inside them says how much of them there was.
func (m *memo) splitPadding(c cutShape, target, freed int64, label string) error {
	content := c.content - freed
	m.split = split{sized: true, content: content, freed: freed}
	pad := target - (c.bare - freed)
	front := format.PaddingInFront(c.before, content, m.cutAt, target)
	// A squeezed archive keeps its entry of padding behind the files. It takes
	// back what the compressor frees, and the plan names it.
	var behind int64
	if m.squeeze.On() {
		behind = c.endOverhead
	}
	if front <= 0 || pad < c.frontOverhead+behind {
		return nil
	}
	cost := min(c.frontOverhead+max(front, c.frontHead)-c.frontHead, pad-behind)
	m.split.front, m.split.frontBytes = true, cost-c.frontOverhead
	if err := m.padBehind(c, pad-cost, freed, label); err != nil {
		return err
	}
	got, err := archiveSize(*m)
	if err != nil {
		return err
	}
	if got.size != target || got.content.Length != c.content || got.content.At != c.before+c.frontHead+m.split.frontBytes {
		return core.Defect(fmt.Errorf("zip: laid out for a cut, the archive measures %d B with its files at %d for %d B, where %d B with them at %d for %d B was settled",
			got.size, got.content.At, got.content.Length, target, c.before+c.frontHead+m.split.frontBytes, c.content))
	}
	return nil
}

// padBehind puts back bytes of padding behind the files: into the comment
// while they fit beside the label, and into an entry of padding above that. A
// squeezed archive puts them all into the entry, which takes back what the
// compressor frees as it is written.
func (m *memo) padBehind(c cutShape, back, freed int64, label string) error {
	m.comment, m.withFiller, m.fillerSize = label, false, 0
	switch {
	case m.squeeze.On():
		m.withFiller, m.fillerSize = true, back-c.endOverhead-freed
	case back <= int64(commentPaddingLimit-len(label)):
		m.comment = label + strings.Repeat(" ", int(back))
	case back >= c.endOverhead:
		m.withFiller, m.fillerSize = true, back-c.endOverhead
	default:
		return core.Defect(fmt.Errorf("zip: %d B behind the files is more than the comment carries and less than an entry of padding costs (%d B)", back, c.endOverhead))
	}
	return nil
}

// cutShape is what splitPadding needs to know of an archive, measured by
// writing its structure rather than added up from header widths, for the
// reason archiveSize gives.
type cutShape struct {
	// bare is how long the archive comes to with no padding at all: no entry
	// of it, and the comment the label alone.
	bare int64
	// before is how many bytes stand in front of the files, the directories,
	// and content how long the files' entries come to when they are stored.
	before, content int64
	// frontHead is how many bytes an empty entry in front puts before the
	// files, and frontOverhead what it costs in all, its record in the central
	// directory with it.
	frontHead, frontOverhead int64
	// endOverhead is what an empty entry of padding behind the files costs
	// when one stands in front - it carries the longer name.
	endOverhead int64
}

func measureCut(m memo, label string) (cutShape, error) {
	m.comment, m.withFiller, m.fillerSize, m.split = label, false, 0, split{}
	bare, err := archiveSize(m)
	if err != nil {
		return cutShape{}, err
	}
	m.split.front = true
	front, err := archiveSize(m)
	if err != nil {
		return cutShape{}, err
	}
	m.withFiller = true
	both, err := archiveSize(m)
	if err != nil {
		return cutShape{}, err
	}
	c := cutShape{
		bare: bare.size, before: bare.content.At, content: bare.content.Length,
		frontHead: front.content.At - bare.content.At, frontOverhead: front.size - bare.size,
		endOverhead: both.size - front.size,
	}
	if front.content.Length != c.content || both.content.Length != c.content {
		return c, core.Defect(fmt.Errorf("zip: the files measured %d B, then %d B behind an entry of padding and %d B between two",
			c.content, front.content.Length, both.content.Length))
	}
	return c, nil
}

// squeezedFiles is how long the files' entries come to once squeezed: the pass
// that measures, squeezing every file into a counter.
func squeezedFiles(ctx context.Context, m memo) (int64, error) {
	m.withFiller, m.split = false, split{}
	c := &counter{}
	rec := &format.MapRecorder{W: c}
	if err := build(ctx, rec, m, true); err != nil {
		return 0, err
	}
	content, err := contentOf(rec.Map(c.n, true))
	if err != nil {
		return 0, err
	}
	return content.Length, nil
}

// startTheFiles marks where the files start, once the directories are
// written: at the next byte, or, in an archive laid out for a cut, behind an
// entry of padding it writes first - where openFile marks them.
//
// The entry is named fillerName, the name of the one entry of padding an
// archive nothing cuts has, and stored and locked like it. Its index follows
// that of the one behind the files, so the two take different salts.
func startTheFiles(ctx context.Context, zw *stdzip.Writer, zm *archive.StreamMap, m memo, withContents bool) error {
	if !m.split.front {
		zm.MarkNext(format.RoleContent, m.split.contentLength())
		return nil
	}
	if err := zm.Hold(); err != nil {
		return err
	}
	return writeStoredFiller(ctx, zw, zm, m, withContents, storedFiller{
		name: fillerName, size: m.split.frontBytes, index: len(m.children) + 1,
	})
}

// openFile is openEntry for a file inside, which marks where the files start
// when an entry of padding stands in front of them: at the first file's local
// header, which archive/zip writes only once it has finished the entry in
// front with its data descriptor.
func openFile(zw *stdzip.Writer, zm *archive.StreamMap, m memo, e entryPlan) (io.Writer, func() error, error) {
	if e.index != 0 || !m.split.front {
		return openEntry(zw, m, e)
	}
	if err := zm.Hold(); err != nil {
		return nil, nil, err
	}
	entry, shut, err := openEntry(zw, m, e)
	if err != nil {
		return nil, nil, err
	}
	if err := zm.MarkEntry(format.RoleContent, e.name, m.split.contentLength()); err != nil {
		return nil, nil, err
	}
	return entry, shut, zm.Release()
}

// mapped is an archive's length and the region its files take, read off the
// map a counting pass marks (archiveSize).
type mapped struct {
	size    int64
	content format.Region
}

// contentOf is the region of content in a map.
func contentOf(regions []format.Region) (format.Region, error) {
	for _, r := range regions {
		if r.Role == format.RoleContent {
			return r, nil
		}
	}
	return format.Region{}, core.Defect(fmt.Errorf("zip: writing the archive marked no files"))
}
