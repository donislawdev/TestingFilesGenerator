// Package zip generates ZIP archives, including archives that pull in other
// generators.
//
// The first container. An archive here does not hold random bytes - it holds
// real generated files of other Tier 1 formats, each one valid on its own.
package zip

import (
	stdzip "archive/zip"
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
		Extension:   ".zip",
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
			archive.Depth, archive.DirectoryEntries,
			archive.Password, archive.Encryption),
		Container:        true,
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
}

func (generator) Plan(r format.Request) (format.Plan, error) {
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

	m := memo{seed: r.Seed, lock: lock, layout: layout}
	if m.children, err = planChildren(r, groups, layout); err != nil {
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

	if err := withinZip32(target); err != nil {
		return format.Plan{}, err
	}

	p.Memo = m
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
		Reason: "an archive this large would need the zip64 records, and this build works out an " +
			"archive's size before it writes it in a way that cannot account for them",
		Hint: "Ask for less than 4 GiB, or split the contents across several archives.",
	}
}

// planChildren plans every file the archive will hold.
//
// The children are real files of another format, each valid on its own. The
// registry sits on the same layer as this package, so reaching for it needs
// nothing from the engine.
//
// Members are numbered across the whole archive rather than per group, so the
// seed of a member does not move when a group above it changes count. That is
// untouchable rule 2 applied one level down.
func planChildren(r format.Request, groups []format.Content, layout archive.Layout) ([]child, error) {
	var out []child
	index := 0
	// Numbering runs per format rather than per group, so two groups of the
	// same format do not both start at 0001 and collide inside the archive.
	numbered := map[string]int{}
	for _, g := range groups {
		desc, err := format.Get(g.Format)
		if err != nil {
			return nil, err
		}
		for i := 0; i < g.Count; i++ {
			childSeed := core.FileSeed(r.Seed, index)
			cp, err := desc.Generator.Plan(format.Request{
				Bytes: g.Bytes,
				Seed:  childSeed,
				Label: r.Label,
			})
			if err != nil {
				return nil, fmt.Errorf("zip: the %s file inside cannot be made: %w", g.Format, err)
			}
			numbered[g.Format]++
			out = append(out, child{
				name: layout.Path(fmt.Sprintf("%s_%04d%s", g.Format, numbered[g.Format], desc.Extension)),
				desc: desc,
				plan: cp,
			})
			index++
		}
	}
	return out, nil
}

// settleSize returns the size the archive is aiming at, the size it comes to
// with no padding, and the label it carries.
//
// The label carries the size, so a size that comes from the contents is not
// known until the archive has been measured. Measure bare first with no
// comment, then settle the label, then measure again.
func settleSize(m *memo, r format.Request) (target, bare int64, label string, err error) {
	if r.Label && !r.SizeFromContents {
		label = core.Label("zip", r.Bytes, r.Seed)
	}
	m.comment = label

	// Measure the archive with no padding at all. Every entry is stored rather
	// than compressed, so the size follows from the parts exactly and nothing
	// has to be guessed.
	if bare, err = archiveSize(*m); err != nil {
		return 0, 0, "", err
	}

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
			m.comment = core.Label("zip", size, r.Seed)
			measured, err := archiveSize(*m)
			if err != nil {
				return 0, 0, "", err
			}
			settled = measured == size
			size = measured
		}
		if !settled {
			return 0, 0, "", fmt.Errorf(
				"zip: the size of this archive and the size written in its label do not settle. Give an explicit size")
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
			"contains": contentSummary(groups),
			"method":   "store",
			// Where the files sit, and whether the directories are named.
			// Written every time rather than only when nested, like method
			// and entries beside them: a harness reading this should not have
			// to know that a missing key means flat. Rule 6 - what the run
			// produced has to be visible in the manifest, and "the archive is
			// three levels deep" is exactly the kind of thing a test asserts
			// against.
			archive.Depth:                m.layout.Depth,
			archive.DirectoryEntries:     m.layout.DirEntries,
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
			Reason:    fmt.Sprintf("an archive holding %s already needs that much", describeGroups(groups)),
			Hint:      fmt.Sprintf("Ask for %d B or more, or hold fewer or smaller files.", bare),
		}
	case target == bare:
		return nil // Nothing to pad.
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
	withFiller, err := archiveSize(*m)
	if err != nil {
		return err
	}
	m.fillerSize = r.Bytes - withFiller
	if m.fillerSize < 0 {
		return &format.BelowMinimumError{
			Format:    "ZIP",
			Requested: r.Bytes,
			Minimum:   withFiller,
			Reason:    "the padding entry the archive needs at this size does not fit",
			Hint:      fmt.Sprintf("Ask for %d B or more.", withFiller),
		}
	}
	p.Properties["padding_entry"] = fillerName
	return nil
}

// contentSummary is what the archive holds, in the manifest, so a test can
// assert on it without unpacking the file.
func contentSummary(groups []format.Content) []map[string]any {
	out := make([]map[string]any, 0, len(groups))
	for _, g := range groups {
		out = append(out, map[string]any{
			"format": g.Format,
			"count":  g.Count,
			"bytes":  g.Bytes,
		})
	}
	return out
}

// describeGroups is the same thing for a person reading an error.
func describeGroups(groups []format.Content) string {
	parts := make([]string, 0, len(groups))
	for _, g := range groups {
		kind := strings.ToUpper(g.Format)
		parts = append(parts, fmt.Sprintf("%s of %d B", core.Count(g.Count, kind+" file", kind+" files"), g.Bytes))
	}
	return strings.Join(parts, " and ")
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(memo)
	if !ok {
		return fmt.Errorf("zip: the plan was not produced by this generator")
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
			Method:   stdzip.Store,
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

// build writes the archive.
//
// withContents says whether the files inside are actually generated. The
// writing path passes true. Planning passes false and adds the sizes on
// afterwards, which is what keeps measuring an archive from costing as much as
// producing one - see archiveSize.
//
// One function with a mode rather than two, so the structure, the order of the
// entries and the comment cannot drift between what was measured and what is
// written. Only the data writes differ.
func build(ctx context.Context, w io.Writer, m memo, withContents bool) error {
	zw := stdzip.NewWriter(w)
	if err := zw.SetComment(m.comment); err != nil {
		return fmt.Errorf("zip: the archive comment was refused: %w", err)
	}

	// The directories the archive names come before anything that sits in
	// them. See writeDirectories for why they are not children.
	if err := writeDirectories(ctx, zw, m.layout); err != nil {
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
			return fmt.Errorf("zip: the %s file inside could not be checksummed: %w", c.desc.ID, err)
		}
		entry, shut, err := openEntry(zw, m, entryPlan{
			name: c.name, plain: c.plan.Bytes, index: i, withContents: withContents, crc: crc,
		})
		if err != nil {
			return err
		}
		if withContents {
			if err := c.desc.Generator.Write(ctx, entry, c.plan); err != nil {
				return fmt.Errorf("zip: the %s file inside could not be written: %w", c.desc.ID, err)
			}
			if err := shut(); err != nil {
				return err
			}
		}
	}

	if err := writeFillerEntry(ctx, zw, m, withContents); err != nil {
		return err
	}

	return zw.Close()
}

// writeFiller emits the padding entry without holding it in memory.
func writeFiller(ctx context.Context, w io.Writer, seed uint64, n int64) error {
	rng := core.NewRand(seed)
	buf := make([]byte, writeChunk)
	for remaining := n; remaining > 0; {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		size := int64(len(buf))
		if remaining < size {
			size = remaining
		}
		chunk := buf[:size]
		for i := range chunk {
			chunk[i] = byte(rng.UintN(256))
		}
		if _, err := w.Write(chunk); err != nil {
			return err
		}
		remaining -= size
	}
	return nil
}

// archiveSize measures the archive without generating anything it holds.
//
// The structure is built for real - every header, every name, the comment, the
// central directory - and only the file contents are left out and added on as
// numbers. Everything inside is stored rather than compressed, so that is exact
// rather than close.
//
// Measured on the standard library on 2026-08-03, because a document about a
// format has been wrong here five times out of five and this one is arithmetic
// standing in for a measurement:
//
//	an archive of 3 entries with no contents          424 B
//	the same 3 entries holding 4096 B each         12 712 B
//	424 + 3*4096                                   12 712 B   exact
//
// Checked at 0, 1, 1000, 4096 and 100 000 bytes an entry, and the comment moves
// the total by exactly its own length at 0, 1, 2, 50, 4096 and 65 535.
//
// Why it matters: planning used to call this two or three times and each call
// generated every file inside. That made --dry-run cost about what the real run
// costs - measured at 960 ms against a 56 ms baseline for a 256 MB archive,
// while writing it for real took 1585 ms. A dry run is the step this project
// tells people to take before anything large, so it has to be the cheap one.
//
// If the arithmetic is ever wrong the size guard says so immediately: the
// engine refuses any file whose written length differs from its plan, and the
// property test walks about 120 sizes for this format.
func archiveSize(m memo) (int64, error) {
	c := &counter{}
	if err := build(context.Background(), c, m, false); err != nil {
		return 0, err
	}
	total := c.n
	for _, ch := range m.children {
		total += ch.plan.Bytes + m.lock.EntryOverhead()
	}
	if m.withFiller {
		total += m.fillerSize + m.lock.EntryOverhead()
	}
	return total, nil
}

type counter struct{ n int64 }

func (c *counter) Write(p []byte) (int, error) { c.n += int64(len(p)); return len(p), nil }

// minimumBytes is the smallest archive this generator can produce: one empty
// entry, no label, no padding.
// It panics rather than returning a number nobody measured.
//
// This used to answer 1<<62 on any failure, on the reasoning that refusing
// every size is safer than declaring a wrong minimum. It is not safer, it is
// quieter: ZIP would refuse every request ever made with a message about a
// minimum of 4611686018427387904 B, and nothing would say why. That is rule 6
// broken in the way this project keeps finding.
//
// The condition is a programming mistake rather than a runtime one. It needs
// the default entry format to be unregistered when this runs, and the language
// says when that can happen: packages are initialised in the order of their
// import paths, so the txt package is registered before this one by rule rather
// than by luck. This paragraph said "true by accident" until 2026-08-25, when
// an outside review built an argument on the opposite claim - that the
// specification guarantees nothing here. Both were wrong, and the comment being
// wrong about its own mechanism is the worse of the two.
//
// What the rule does not cover is a rename that moves the entry format after
// this package in that order, or a second entry point linking this package
// without that one. The first is what a guard asks, since a format has been
// renamed in this tree before. format.Register already panics on the same class
// of mistake, so this matches it: a build that cannot state its own minimum
// fails at start rather than at every use.
func minimumBytes() int64 {
	desc, err := format.Get(archive.DefaultFormat)
	if err != nil {
		panic(fmt.Sprintf("zip: the default entry format %q is not registered yet, so the minimum size of an archive cannot be worked out. Check the import order in internal/format/all", archive.DefaultFormat))
	}
	cp, err := desc.Generator.Plan(format.Request{Bytes: 0, Seed: 0, Label: false})
	if err != nil {
		panic(fmt.Sprintf("zip: the default entry format %q cannot produce an empty file, so the minimum size of an archive cannot be worked out: %v", archive.DefaultFormat, err))
	}
	n, err := archiveSize(memo{children: []child{{
		name: "txt_0001.txt", desc: desc, plan: cp,
	}}})
	if err != nil {
		panic(fmt.Sprintf("zip: the smallest archive cannot be measured: %v", err))
	}
	return n
}
