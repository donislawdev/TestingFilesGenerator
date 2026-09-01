// Package targz generates gzip compressed tar archives, including archives
// that pull in other generators.
//
// The second container, and the second Tier 1 format whose padding channel has
// a ceiling. An archive here does not hold random bytes - it holds real
// generated files of other Tier 1 formats, each one valid on its own.
package targz

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/archive"
)

const (
	generatorVersion = "1"

	// commentPaddingLimit is how much of the comment padding may use.
	//
	// What every OTHER reader takes is far more than this and was measured by
	// bisection on 2026-08-04: 7-Zip 26.02 and p7zip 23.01 both accept 65 535
	// and refuse 65 536, while GNU gzip, GNU tar, bsdtar, Python and node take
	// ten megabytes without a word. None of that is the binding number any
	// more, so it is written here rather than kept as a constant nothing uses.
	//
	// It was 4096 until 2026-09-01, and that number could not be read by a
	// very ordinary reader. Go's compress/gzip takes a header comment of 511
	// bytes and refuses 512, because it reads the field into a fixed buffer -
	// so 4134 of the 11 260 reachable sizes in a twenty kilobyte sweep produced
	// an archive no Go program could open, with a message that reads like a
	// corrupt file. 7-Zip, GNU tar, bsdtar, Python and node all took them
	// without a word, which is why nobody noticed for a month. O163.
	//
	// The comment now carries the label and at most a byte or two beyond it.
	// Bulk padding moved to the extra field, which holds sixteen times more and
	// which Go reads to the end of.
	commentPaddingLimit = 480

	// extraPaddingLimit is how many bytes the gzip extra field may carry.
	//
	// The field's own limit is 65 535, since XLEN is two bytes wide. This build
	// stops at 65 531, which is the number measured across every reader on
	// 2026-08-04 and re-measured against Go on 2026-09-01 - accepted to the
	// last byte, beside a comment, with the cost exactly two bytes more than
	// what is carried.
	//
	// That cost is what makes this the right channel rather than a bigger one:
	// it is byte granular. The comment was too, but it had to hold the label as
	// well, and a tar filler entry is aligned to 512 bytes so it delivers bulk
	// and never the last few. Capping the comment at what Go reads and leaving
	// the rest to the filler was measured and rejected: 831 sizes in the same
	// sweep stopped being reachable at all, because nothing could bridge the
	// gap under one block.
	extraPaddingLimit = 65531

	// fillerName is the entry that carries padding the comment cannot hold.
	// Named plainly, because somebody opening the archive should see what it
	// is rather than wonder.
	fillerName = "tfg-padding.bin"

	// tarBlock is the tar block size. Every header is one block and every
	// entry's content is padded up to a whole number of them, which is why the
	// second padding stage cannot reach an exact byte on its own.
	tarBlock = 512

	// storeBlock is the largest run of bytes gzip emits as one stored block at
	// compression level zero. It decides the framing overhead, so it decides
	// the arithmetic in size.go.
	storeBlock = 65535

	// gzipFraming is the fixed ten byte header plus the eight byte trailer.
	gzipFraming = 18

	// storeBlockCost is what each stored block costs on top of its content.
	storeBlockCost = 5

	writeChunk = 32 * 1024
)

// A fixed timestamp on every entry. Taking one from the clock would make two
// runs of the same recipe differ, and relying on the zero value would be an
// unstated dependency on what the library does with it.
var fixedTime = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

func init() {
	format.Register(format.Descriptor{
		ID:          "targz",
		Extension:   ".tar.gz",
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,
		MinBytes:    minimumBytes(),

		Padding: format.PaddingChannel{
			Name:  "gzip extra field, then a stored filler entry above its limit",
			Where: format.PlacementStart,
			// The extra field precedes everything, so how long it is has to be
			// settled before the first byte goes out. That is the whole reason
			// the size is worked out by arithmetic rather than by measuring
			// what came before.
			//
			// It was the comment until 2026-09-01, and the comment is still
			// here carrying the label. What moved is the padding, because Go
			// reads a comment of 511 bytes and refuses 512 while it reads this
			// field to the end. O163.
			Capacity: extraPaddingLimit,
		},
		Label:  format.LabelInternal,
		Oracle: "7z",
		// The settings every container shares, declared once in the archive
		// package. Listed rather than received whole, so a format takes only
		// the axes it can actually carry.
		Properties: archive.Axes(archive.Entries, archive.EntryFormat, archive.EntrySize,
			archive.Compression, archive.Depth, archive.DirectoryEntries,
			archive.EntryMode, archive.EntryOwner),

		// Neither half of this format has anywhere to put a password, and
		// saying so is worth more than the generic "no such property" - that
		// one reads as a gap in this build, and somebody would go looking for
		// the version that closed it.
		//
		// What the world does here is worse than either. Measured on
		// 2026-09-01: 7-Zip accepts -p on a tar and on a gzip, exits 0,
		// prints nothing, and writes a PLAINTEXT archive. Somebody asking
		// for a locked fixture gets an open one and never finds out.
		Unsupported: []format.UnsupportedSetting{
			{
				Name: archive.Password,
				Why: "neither tar nor gzip has any encryption in it, so there is no field in either " +
					"one to put a password in",
				Instead: "Use zip for an archive with a password, or leave this one open.",
			},
			{
				Name:    archive.Encryption,
				Why:     "neither tar nor gzip has any encryption in it, so there is nothing to choose between",
				Instead: "Use zip for an archive with a password, or leave this one open.",
			},
		},
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
	// own is what every entry records about its permissions and its owner.
	// The zero value is not the default - ReadOwnership fills it, because
	// the mode this format has always written is 644 rather than 0.
	own archive.Ownership
	// squeeze is how hard the stream is compressed, and the zero value is
	// stored - which is what this format has always written.
	squeeze archive.Squeeze
	// target is the size a compressed archive has to come to. It is only set
	// when squeezing, because that is the only case where the writer rather
	// than the plan settles the padding.
	target int64
	// layout is where the files inside sit. It has to be here rather than an
	// argument because tarLength counts from this struct and build writes
	// from it: a layout the two disagreed about would promise one size and
	// write another.
	layout archive.Layout
	// withExtra says whether the header carries a gzip extra field at all,
	// and extraLen says how many bytes it holds. Two fields rather than one
	// with a sentinel, matching withFiller beside them, because an EMPTY
	// field still costs its two byte length while no field costs nothing -
	// and a sentinel makes the zero value of this struct wrong, which is a
	// thing minimumBytes met on the first try.
	withExtra bool
	extraLen  int64
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	groups, err := archive.Groups("targz", r)
	if err != nil {
		return format.Plan{}, err
	}

	own, err := archive.ReadOwnership("targz", r.Properties)
	if err != nil {
		return format.Plan{}, err
	}

	layout, err := archive.ReadLayout("targz", r)
	if err != nil {
		return format.Plan{}, err
	}

	squeeze, err := archive.ReadCompression("targz", r, false)
	if err != nil {
		return format.Plan{}, err
	}

	m := memo{seed: r.Seed, own: own, layout: layout, squeeze: squeeze}
	if m.children, err = planChildren(r, groups, layout); err != nil {
		return format.Plan{}, err
	}

	target, label, err := settleSize(&m, r)
	if err != nil {
		return format.Plan{}, err
	}

	p := describe(target, label, m, groups)
	if m.squeeze.On() {
		// A compressed archive settles its padding at WRITE time, because its
		// length cannot be worked out without compressing it. What is checked
		// here is only that the size is reachable at all, and the check is
		// deliberately the STORED one: compression can only ever make the
		// contents smaller, so an archive that fits stored fits squeezed.
		m.target = target
		if err := reachable(&m, target, label, groups); err != nil {
			return format.Plan{}, err
		}
	} else if err := pad(&m, &p, target, label, groups); err != nil {
		return format.Plan{}, err
	}

	p.Memo = m
	return p, nil
}

// planChildren plans every file the archive will hold.
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
			cp, err := desc.Generator.Plan(format.Request{
				Bytes: g.Bytes,
				Seed:  core.FileSeed(r.Seed, index),
				Label: r.Label,
			})
			if err != nil {
				return nil, fmt.Errorf("targz: the %s file inside cannot be made: %w", g.Format, err)
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

// settleSize returns the size the archive aims at and the label it carries.
//
// The label states the size, so a size that comes from the contents is not
// known until the archive has been measured, and measuring it changes the
// label. The two settle against each other the same way they do in ZIP.
func settleSize(m *memo, r format.Request) (target int64, label string, err error) {
	if r.Label && !r.SizeFromContents {
		label = core.Label("targz", r.Bytes, r.Seed)
	}
	m.comment = label

	if !r.SizeFromContents {
		return r.Bytes, label, nil
	}

	bare := archiveSize(*m)
	if !r.Label {
		return bare, label, nil
	}

	// It converges rather than oscillating, because the size only ever grows
	// and a longer number never shortens the label. Bounded anyway, and an
	// error rather than a guess if it somehow does not settle - a label stating
	// the wrong size is worse than the work of finding out.
	size := bare
	for i := 0; i < 4; i++ {
		m.comment = core.Label("targz", size, r.Seed)
		measured := archiveSize(*m)
		if measured == size {
			return size, m.comment, nil
		}
		size = measured
	}
	return 0, "", fmt.Errorf(
		"targz: the size of this archive and the size written in its label do not settle. Give an explicit size")
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
			// Where the files sit, and whether the directories are named -
			// written every time rather than only when nested, so a harness
			// never has to read a missing key as flat.
			archive.Depth:            m.layout.Depth,
			archive.DirectoryEntries: m.layout.DirEntries,
			// How hard the stream was squeezed. This key used to be the
			// constant "none", written when the format could only store - and
			// the comment beside it said so, which is why it is worth saying
			// that the constant is gone rather than deleting it quietly. It
			// is still written every time, so a harness never has to read a
			// missing key as stored.
			archive.Compression:          m.squeeze.Name,
			format.PropertyLabelEmbedded: label != "",
		},
	}
	// The single format shape keeps the keys it always had, so a test asserting
	// on entry_format does not break the day contains arrives.
	if len(groups) == 1 {
		p.Properties["entry_format"] = groups[0].Format
		p.Properties["entry_size"] = groups[0].Bytes
	}
	return p
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
		return fmt.Errorf("targz: the plan was not produced by this generator")
	}
	if m.squeeze.On() {
		return writeCompressed(ctx, w, m)
	}
	return build(ctx, w, m)
}

// minimumBytes is the structural floor of the format: an archive holding
// nothing, with no label and no padding. The end of archive marker is two
// empty blocks, and gzip frames that into 1052 bytes.
//
// An archive holding nothing is a request somebody can actually make, since
// entries goes down to nought, so this is a size the format really accepts
// rather than a number that describes an unreachable skeleton.
//
// It asks no other format anything, and that is on purpose. ZIP works its
// minimum out from a default TXT entry, and that works because Go initialises
// packages in the order of their import paths, which puts txt before zip. It is
// a rule rather than luck - both comments said "by accident" until 2026-08-25 -
// but it is a rule about names, and targz comes before txt in that same order.
// So the same approach here would panic at startup. Not worked around by
// ordering imports, because gofmt sorts them back.
func minimumBytes() int64 {
	return archiveSize(memo{})
}
