// Package zip generates ZIP archives, including archives that pull in other
// generators.
//
// The first container. An archive here does not hold random bytes - it holds
// real generated files of other Tier 1 formats, each one valid on its own.
package zip

import (
	stdzip "archive/zip"
	"compress/flate"
	"context"
	"fmt"
	"hash/crc32"
	"io"

	"strings"
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/archive"
)

const (
	generatorVersion = "1"

	// commentCapacity is the hard limit of the archive comment. The field
	// that carries its length is two bytes wide, so this is a property of the
	// format rather than a choice. ZIP is the only Tier 1 format whose
	// padding channel has a ceiling.
	commentCapacity = 65535

	// commentPaddingLimit is how much of that we actually use.
	//
	// Measured on CI: p7zip 17.06 on macOS segfaults reading an archive whose
	// comment is filled to the maximum. 7-Zip 26.02 on Windows and p7zip
	// 23.01 on Linux read the same file without complaint, so the crash is a
	// defect in that build rather than in the archive - the structural check
	// agrees the file is well formed.
	//
	// It is still our problem. p7zip is what "7z" means on a lot of machines,
	// and a fixture that crashes the archiver a tester reaches for is a
	// fixture nobody will trust. Padding through a stored entry reaches every
	// size just as exactly, so the enormous comment bought nothing and now it
	// is gone.
	commentPaddingLimit = 4096

	// fillerName is the entry that carries padding the comment cannot hold.
	// Named plainly, because somebody opening the archive should see what it
	// is rather than wonder.
	fillerName = "tfg-padding.bin"

	writeChunk = 32 * 1024
)

// A fixed timestamp on every entry. Taking one from the clock would make two
// runs of the same recipe differ, and relying on the zero value would be an
// unstated dependency on what the library does with it.
var fixedTime = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

func init() {
	format.Register(format.Descriptor{
		ID:          "zip",
		Name:        "ZIP",
		Extension:   ".zip",
		MediaType:   "application/zip", // IANA
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,
		MinBytes:    minimumBytes(),

		Padding: format.PaddingChannel{
			Name:     "archive comment, then a stored filler entry above its limit",
			Where:    format.PlacementEnd,
			Capacity: commentCapacity,
		},
		Label:  format.LabelInternal,
		Oracle: "7z",
		// The settings every container shares, declared once in the archive
		// package. Listed rather than received whole, so a format takes only
		// the axes it can actually carry.
		Properties: archive.Axes(archive.Entries, archive.EntryFormat, archive.EntrySize,
			archive.Compression, archive.Depth, archive.DirectoryEntries,
			archive.Password, archive.Encryption),
		Container:        &archive.ArchiveMembers,
		GeneratorVersion: generatorVersion,
		Generator:        generator{},
	})
}

type generator struct{}

type child struct {
	name string
	desc format.Descriptor
	plan format.Plan
}

type memo struct {
	children   []child
	comment    string
	fillerSize int64
	withFiller bool
	seed       uint64
	// lock is what the archive is locked with, and the zero value is an
	// archive that is not. It reaches build through here rather than
	// through an argument because the counting pass and the writing pass
	// have to agree about it exactly, and they share this.
	lock archive.Lock
	// layout is where the files inside sit, and it travels the same way and
	// for the same reason: archiveSize counts by running build against a
	// counter, so a layout the two passes disagreed about would produce an
	// archive whose size had been promised for a different shape.
	layout archive.Layout
	// squeeze is how hard the entries are compressed, and the zero value is
	// stored - which is what every archive written before this was.
	//
	// It changes WHEN the padding is settled, which is the whole difficulty.
	// A stored archive's length follows from its declared parts, so the plan
	// works the padding out. A compressed one does not: the project measured
	// on TIFF that deflate moves the length with the seed. So the plan still
	// works out the padding for the STORED archive, and the writer adds back
	// exactly what the compressor freed - see build.
	squeeze archive.Squeeze
	// target is the size that was ordered, carried so the writer can say what
	// the smallest archive would have been when it turns out this one cannot be
	// made. Only the compressed path needs it: a stored archive settles its
	// padding while planning, so a size it cannot reach is refused there.
	target int64
	// cutAt is where a damage will cut the archive (format.Request.CutAt), 0
	// when nothing does, and split is how its padding is laid out around the
	// files for that cut (cut.go). The zero value of split is the archive
	// nothing cuts.
	cutAt int64
	split split
}

func (g generator) Plan(r format.Request) (format.Plan, error) {
	return g.PlanContext(context.Background(), r)
}

// PlanContext is Plan, stoppable between one file inside and the next.
func (generator) PlanContext(ctx context.Context, r format.Request) (format.Plan, error) {
	groups, err := archive.Groups("zip", r)
	if err != nil {
		return format.Plan{}, err
	}

	lock, err := archive.ReadLock("zip", r.Properties)
	if err != nil {
		return format.Plan{}, err
	}

	layout, err := archive.ReadLayout("zip", r)
	if err != nil {
		return format.Plan{}, err
	}

	squeeze, err := archive.ReadCompression("zip", r, lock.On())
	if err != nil {
		return format.Plan{}, err
	}

	m := memo{seed: r.Seed, lock: lock, layout: layout, squeeze: squeeze}
	if m.children, err = planChildren(ctx, r, groups, layout); err != nil {
		return format.Plan{}, err
	}

	target, bare, label, err := settleSize(&m, r)
	if err != nil {
		return format.Plan{}, err
	}

	p := describe(target, label, m, groups)
	if err := pad(&m, &p, r, target, bare, label, groups); err != nil {
		return format.Plan{}, err
	}
	if err := settleForACut(&m, &p, r.CutAt, target, label); err != nil {
		return format.Plan{}, err
	}

	if err := withinZip32(target); err != nil {
		return format.Plan{}, err
	}

	p.Memo = m
	p.Holds = format.OneAtATime(m.children, func(c child) format.Plan { return c.plan })
	return p, nil
}

// zip32Ceiling is where ZIP stops being able to describe itself in thirty two
// bits and archive/zip starts writing the zip64 records instead.
const zip32Ceiling = 1<<32 - 1

// withinZip32 refuses an archive that would cross into zip64.
//
// Not because zip64 is wrong - archive/zip writes it correctly - but because
// archiveSize cannot see it coming. That measurement builds the whole structure
// with the contents left out and then adds the planned sizes back, so every
// entry is nought bytes long while it is being measured, and nought bytes never
// triggers zip64. Measured on 2026-08-26 with tools/probes/zip64, the plan
// against what really came out of the writer:
//
//	64 MiB          agree
//	4 GiB - 1 MiB   agree
//	4 GiB + 1 MiB   the writer wrote 112 B more than the plan
//	5 GiB           the writer wrote 112 B more than the plan
//
// Those 112 B are a wider data descriptor, a zip64 extra field in the central
// directory, and the zip64 end of central directory record with its locator.
//
// Nothing that works is being taken away, and that is measured rather than
// assumed: the engine compares what the writer produced against the plan and
// deletes any file that misses, so an archive past this line could not succeed
// before either. What changes is when the person finds out - here, before the
// first byte, instead of after four gigabytes have been written and removed,
// with a message about the generator disagreeing with its own plan that reads
// like a defect in the tool.
//
// The line is drawn at the format's own boundary rather than at the exact byte
// where the writer changes shape. That byte is real - measured, an archive of
// exactly 4294967296 B still agrees, because what crosses uint32 is the ENTRY
// and an entry is smaller than the archive holding it - but it moves with the
// number of entries and the length of their names. Predicting it here would be
// a second copy of a rule that lives in the standard library, which is the kind
// of copy this package refuses elsewhere. So the trailer's own few hundred
// bytes are given away, and the sentence says the limit belongs to this build.
// The total is the only thing asked about, and that is a correction rather
// than a simplification. The first version also checked each contained file
// and the padding entry, which reads like thoroughness and is unreachable: an
// archive holds its entries, so an entry past the line puts the archive past
// it first. Both branches were shown to be dead by mutation - removing either
// left the guard green - and this project deletes a defence nothing can turn
// red rather than keeping it for the look of it.
func withinZip32(total int64) error {
	if total <= zip32Ceiling {
		return nil
	}
	return &format.AboveMaximumError{
		Format:    "ZIP",
		Requested: total,
		Maximum:   zip32Ceiling,
		Reason: core.Says("format.AnArchiveThisLargeWouldNeed", "an archive this large would need the zip64 records, and this build works out an "+
			"archive's size before it writes it in a way that cannot account for them"),
		Hint: core.Says("format.AskForLessThan4GiB2", "Ask for less than 4 GiB, or split the contents across several archives."),
	}
}

// settleSize returns the size the archive is aiming at, the size it comes to
// with no padding, and the label it carries.
//
// The label carries the size, so a size that comes from the contents is not
// known until the archive has been measured. Measure bare first with no
// comment, then settle the label, then measure again.
func settleSize(m *memo, r format.Request) (target, bare int64, label string, err error) {
	if r.Label && !r.SizeFromContents {
		label = r.LabelText("zip")
	}
	m.comment = label

	// Measure the archive with no padding at all. Every entry is stored rather
	// than compressed, so the size follows from the parts exactly and nothing
	// has to be guessed.
	measured, err := archiveSize(*m)
	if err != nil {
		return 0, 0, "", err
	}
	bare = measured.size

	if !r.SizeFromContents {
		return r.Bytes, bare, label, nil
	}

	// The contents decide the size, and the label states the size, so the two
	// settle against each other: a longer number makes a longer file.
	//
	// It converges rather than oscillating, because the size only ever grows
	// and a longer number never shortens the label. Bounded anyway, and an
	// error rather than a guess if it somehow does not settle - a label stating
	// the wrong size is worse than the work of finding out.
	if r.Label {
		size := bare
		settled := false
		for i := 0; i < 4 && !settled; i++ {
			m.comment = r.LabelSized("zip", size)
			measured, err := archiveSize(*m)
			if err != nil {
				return 0, 0, "", err
			}
			settled = measured.size == size
			size = measured.size
		}
		if !settled {
			return 0, 0, "", core.Defect(fmt.Errorf(
				"zip: the size of this archive and the size written in its label do not settle. Give an explicit size"))
		}
		label = m.comment
		bare = size
	}
	return bare, bare, label, nil
}

// describe builds the plan and the properties that reach the manifest.
func describe(target int64, label string, m memo, groups []format.Content) format.Plan {
	p := format.Plan{
		Bytes:       target,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties: map[string]any{
			"entries":  len(m.children),
			"contains": archive.ContentSummary(groups),
			"method":   "store",
			// Where the files sit, and whether the directories are named.
			// Written every time rather than only when nested, like method
			// and entries beside them: a harness reading this should not have
			// to know that a missing key means flat. Rule 6 - what the run
			// produced has to be visible in the manifest, and "the archive is
			// three levels deep" is exactly the kind of thing a test asserts
			// against.
			archive.Depth:            m.layout.Depth,
			archive.DirectoryEntries: m.layout.DirEntries,
			// What a reader will meet inside. Recorded every time, like the
			// method beside it, so a harness never has to read a missing key
			// as "stored".
			archive.Compression:          m.squeeze.Name,
			format.PropertyLabelEmbedded: label != "",
		},
	}
	// The lock goes into the manifest, password and all. A locked fixture
	// whose password is not written down is a file no test can open, which
	// makes it worth nothing - so the manifest says it in plain text, on
	// purpose. Left out entirely when the archive is open, rather than
	// written as "none" beside an empty password, because a key that is
	// there says something happened.
	if m.lock.On() {
		p.Properties[archive.Encryption] = m.lock.Method
		p.Properties[archive.Password] = m.lock.Password
	}
	// The single format shape keeps the keys it always had, so a test asserting
	// on entry_format does not break the day contains arrives.
	if len(groups) == 1 {
		p.Properties["entry_format"] = groups[0].Format
		p.Properties["entry_size"] = groups[0].Bytes
	}
	return p
}

// pad decides where the difference between the bare archive and the size that
// was asked for goes.
func pad(m *memo, p *format.Plan, r format.Request, target, bare int64, label string, groups []format.Content) error {
	switch {
	case target < bare:
		return &format.BelowMinimumError{
			Format:    "ZIP",
			Requested: target,
			Minimum:   bare,
			Reason:    core.Says("format.AnArchiveHoldingAlreadyNeedsThat2", "an archive holding %s already needs that much", core.A("Groups", describeGroups(groups))),
			Hint:      core.Says("format.AskForBOrMoreOr6", "Ask for %d B or more, or hold fewer or smaller files.", core.A("Bare", bare)),
		}
	// Exactly the bare size, and nothing to add - but only when the entries
	// are stored. A squeezed archive comes out SHORTER than its stored
	// arithmetic says, and the filler entry is the only thing that can give
	// those bytes back. Returning here leaves withFiller false, so the
	// writer has nowhere to put them and the archive ends up short: measured
	// 2026-09-02 at 8382 B, where every level but none produced about 2.6 kB
	// and --dry-run had already said the size was fine.
	case target == bare && !m.squeeze.On():
		return nil // Nothing to pad.
	}

	// A compressed archive puts ALL of its padding in the filler entry rather
	// than sharing it with the comment, and that is not a preference.
	//
	// The writer cannot know how much the compressor will free until it has
	// run, so the padding has to be the one thing that can still move at write
	// time - and the comment cannot, because it is written before the entries.
	// Giving the filler the whole job keeps one number to adjust instead of
	// two.
	if m.squeeze.On() {
		return padCompressed(m, p, r, groups)
	}

	needed := r.Bytes - bare
	room := int64(commentPaddingLimit - len(label))
	if needed <= room {
		m.comment = label + strings.Repeat(" ", int(needed))
		return nil
	}

	// Above what the comment takes, the rest goes into a stored entry whose
	// bytes land in the archive one for one. This is the only padding channel
	// in Tier 1 with a ceiling.
	m.comment = label + strings.Repeat(" ", int(room))
	m.withFiller = true
	measured, err := archiveSize(*m)
	if err != nil {
		return err
	}
	withFiller := measured.size
	m.fillerSize = r.Bytes - withFiller
	if m.fillerSize < 0 {
		return &format.BelowMinimumError{
			Format:    "ZIP",
			Requested: r.Bytes,
			Minimum:   withFiller,
			Reason:    core.Says("format.ThePaddingEntryTheArchiveNeeds", "the padding entry the archive needs at this size does not fit"),
			Hint:      core.Says("format.AskForBOrMore", "Ask for %d B or more.", core.A("Min", withFiller)),
		}
	}
	p.Properties["padding_entry"] = fillerName
	return nil
}

// describeGroups is the same thing for a person reading an error.
func describeGroups(groups []format.Content) core.Said {
	parts := make(core.Conjoined, 0, len(groups))
	for _, g := range groups {
		parts = append(parts, core.SaysN("format.GroupOf", "%d %s file of %d B", "%d %s files of %d B",
			core.A("Count", g.Count), core.A("Kind", strings.ToUpper(g.Format)), core.A("Bytes", g.Bytes)))
	}
	return core.Says("format.Groups", "%s", core.A("Groups", parts))
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(memo)
	if !ok {
		return core.Defect(fmt.Errorf("zip: the plan was not produced by this generator"))
	}
	if err := m.settleSqueezedForACut(ctx, p.Bytes); err != nil {
		return err
	}
	return build(ctx, w, m, true)
}

// entryPlan is one entry, as both passes see it.
//
// A record rather than five arguments, because the two passes have to
// describe the entry identically and a positional list of five is where
// that stops being obvious.
type entryPlan struct {
	name  string
	plain int64
	index int
	// withContents is false during the pass that only measures.
	withContents bool
	// stored keeps this entry out of the compressor however hard the archive
	// is squeezed. The padding entry sets it: it is random, so deflate grows
	// it rather than shrinking it, and a padding entry whose length nobody can
	// aim cannot do the one job it has.
	stored bool
	// crc is the checksum of the contents, and it is only ever filled for a
	// lock that needs the contents known before the first byte. Zero
	// otherwise, which is what an AE-2 entry carries anyway and what the
	// counting pass writes into a header nobody reads.
	crc uint32
}

// openEntry starts the next entry and gives back what its contents go to.
//
// Two shapes, and the difference is the whole of what locking costs here. An
// open archive streams: CreateHeader writes a header with the sizes left blank
// and puts them in a descriptor after the data, which is what this format has
// always done. A locked one cannot, because the entry carries a salt and an
// authentication code that the declared size has to include - so it goes
// through CreateRaw with the sizes stated up front, which is also exactly the
// shape 7-Zip writes. Measured, in docs/MVP-FORMATS.md section 2.16.
//
// plain is the size of the contents before locking. index numbers the entry
// across the archive, and it is what gives each one its own salt.
//
// The returned closer finishes the entry. It writes the authentication code
// for a locked entry and does nothing for an open one, and it is not called at
// all during the counting pass - where no contents are written, and CreateRaw
// is content to be handed a header and no bytes.
func openEntry(zw *stdzip.Writer, m memo, e entryPlan) (io.Writer, func() error, error) {
	nothingToShut := func() error { return nil }

	if !m.lock.On() {
		entry, err := zw.CreateHeader(&stdzip.FileHeader{
			Name:     e.name,
			Method:   m.methodFor(e),
			Modified: fixedTime,
		})
		return entry, nothingToShut, err
	}

	h := &stdzip.FileHeader{
		Name:     e.name,
		Method:   m.lock.ZipMethod(),
		Modified: fixedTime,
		Extra:    m.lock.Extra(),
	}
	// Bit 0 says the entry is encrypted. The CRC stays zero because AE-2
	// carries none - which is what lets the contents be written in one pass.
	h.Flags |= 1
	h.CRC32 = e.crc
	h.CompressedSize64 = uint64(e.plain + m.lock.EntryOverhead())
	h.UncompressedSize64 = uint64(e.plain)

	raw, err := zw.CreateRaw(h)
	if err != nil {
		return nil, nil, err
	}
	if !e.withContents {
		// The counting pass writes nothing, and that has to include the salt
		// and the verifier. Building the locked writer here would emit both
		// straight away, so the measurement would count them and the
		// arithmetic below would add them again - eighteen bytes an entry,
		// which is what the engine caught when this was written the other way.
		return raw, nothingToShut, nil
	}
	locked, err := m.lock.NewEntryWriter(raw, m.seed, e.index, e.crc)
	if err != nil {
		return nil, nil, err
	}
	return locked, locked.Close, nil
}

// plaintextCRC is the checksum of what an entry is about to hold, worked out
// by generating it once and throwing the bytes away.
//
// Only ZipCrypto asks for this, and only when the contents are really being
// written. That scheme puts the high byte of the plaintext CRC in the twelve
// byte header it prepends, so a reader can turn away a wrong password without
// decrypting anything - which means the checksum has to be known before the
// first byte of the entry goes out, and the contents arrive as a stream.
//
// Generating twice rather than buffering, and that is the trade taken
// deliberately. Holding the entry to hash it would break the guard that says
// a generator does not keep a whole file in memory, and the second pass is
// free of risk because a generator producing different bytes on two calls in
// one process is itself a guarded impossibility. It costs processor time on
// locked archives and nothing at all on open ones.
func plaintextCRC(ctx context.Context, m memo, withContents bool, write func(io.Writer) error) (uint32, error) {
	if !withContents || !m.lock.NeedsPlaintextCRC() {
		return 0, nil
	}
	sum := crc32.NewIEEE()
	if err := write(sum); err != nil {
		return 0, err
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return sum.Sum32(), nil
}

func build(ctx context.Context, w io.Writer, m memo, withContents bool) error {
	zm := archive.NewStreamMap(w)
	zw := stdzip.NewWriter(zm.Buffer)
	// The map: the directories' entries, the files' entries as the content,
	// the filler entry, the central directory, and the comment. What only the
	// writing tells - where the filler and the directory start - is marked as
	// archive/zip writes it (archive.StreamMap).
	format.Mark(w, format.RoleStructure, 0, -1)
	if err := zw.SetComment(m.comment); err != nil {
		return core.Defect(fmt.Errorf("zip: the archive comment was refused: %w", err))
	}

	// squeezed is how many bytes the entries actually came to once compressed.
	// The plan worked the padding out for a STORED archive, and every
	// structural field of a zip is fixed width - so the compressed archive is
	// shorter by exactly the difference in entry data, and the filler gives
	// that difference back.
	var squeezed int64
	if m.squeeze.On() {
		zw.RegisterCompressor(stdzip.Deflate, func(dst io.Writer) (io.WriteCloser, error) {
			return flate.NewWriter(&tally{to: dst, n: &squeezed}, m.squeeze.Level)
		})
	}

	// The directories the archive names come before anything that sits in
	// them. See writeDirectories for why they are not children.
	if err := writeDirectories(ctx, zw, m.layout); err != nil {
		return err
	}
	if err := startTheFiles(ctx, zw, zm, m, withContents); err != nil {
		return err
	}

	for i, c := range m.children {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		crc, err := plaintextCRC(ctx, m, withContents, func(w io.Writer) error {
			return c.desc.Generator.Write(ctx, w, c.plan)
		})
		if err != nil {
			return core.Defect(fmt.Errorf("zip: the %s file inside could not be checksummed: %w", c.desc.ID, err))
		}
		entry, shut, err := openFile(zw, zm, m, entryPlan{
			name: c.name, plain: c.plan.Bytes, index: i, withContents: withContents, crc: crc,
		})
		if err != nil {
			return err
		}
		if withContents {
			if err := c.desc.Generator.Write(ctx, entry, c.plan); err != nil {
				return core.Defect(fmt.Errorf("zip: the %s file inside could not be written: %w", c.desc.ID, err))
			}
			if err := shut(); err != nil {
				return err
			}
		}
	}

	if err := writeFillerEntry(ctx, zw, zm, m, withContents, m.freedBy(withContents, &squeezed)); err != nil {
		return err
	}

	return zm.Close(zw)
}
