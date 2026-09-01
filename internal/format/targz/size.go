package targz

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/archive"
)

// How the size of this format is worked out.
//
// ZIP measures itself: it builds the structure with the contents left out and
// counts the bytes. That works because every part of a ZIP lands in the file
// one for one. Here the bytes pass through deflate, so building the structure
// without the contents would not answer the question, and building it with
// them would make --dry-run cost what the run costs. That is the trap the
// container planning guard exists to stop.
//
// So this is arithmetic, and the arithmetic was measured rather than derived
// from the specification. Compression level zero emits the input in stored
// blocks of at most 65 535 bytes, each costing five bytes, plus one empty
// block to close the stream:
//
//	gzip = 18 + 5*(ceil(n/65535) + 1) + n + comment
//
// Measured on 2026-08-04 against the standard library over 343 inputs, every
// boundary around 65 535, 131 070, 196 605, 262 140, one mebibyte and four,
// with no misses. Handing the same bytes over in chunks of 1, 512, 4 096,
// 32 768, 65 535 and 65 536 gives the same length as one Write, so how tar
// feeds the stream does not move the framing - 36 combinations, all equal.
//
// The tar layer is simpler and was measured the same way, nine shapes without
// a miss:
//
//	tar = 1024 + sum over entries of (512 + roundUp512(size))
//
// This holds for USTAR headers, and USTAR refuses a name over 100 characters
// rather than quietly switching to a format that adds blocks. Names here are
// about twenty characters, and the refusal is what keeps that true.
//
// If the arithmetic is ever wrong the size guard says so immediately: the
// engine refuses any file whose written length differs from its plan, and a
// guard compares this against a real build across a spread of shapes.

// roundUpBlock rounds a length up to a whole number of tar blocks.
func roundUpBlock(n int64) int64 {
	if r := n % tarBlock; r != 0 {
		return n + tarBlock - r
	}
	return n
}

// tarLength is the length of the tar stream before it reaches gzip.
func tarLength(m memo) int64 {
	total := int64(2 * tarBlock) // the end of archive marker
	for _, c := range m.children {
		total += tarBlock + roundUpBlock(c.plan.Bytes)
	}
	if m.withFiller {
		total += tarBlock + roundUpBlock(m.fillerSize)
	}
	return total
}

// gzipFixed is the whole file except the comment.
func gzipFixed(tarLen int64) int64 {
	blocks := tarLen / storeBlock
	if tarLen%storeBlock != 0 {
		blocks++
	}
	return gzipFraming + storeBlockCost*(blocks+1) + tarLen
}

// commentCost is what a comment adds to the file: its bytes plus the zero that
// ends it, and nothing at all when there is none.
//
// Counted in bytes rather than characters because that is what lands in the
// file. gzip headers are Latin-1, so Go writes one byte per character and
// refuses anything outside it - which keeps the two counts equal for the label
// and for the spaces that pad it, both being plain ASCII.
func commentCost(comment string) int64 {
	if comment == "" {
		return 0
	}
	return int64(len(comment)) + 1
}

// archiveSize is the exact size of the archive described by m.
func archiveSize(m memo) int64 {
	return gzipFixed(tarLength(m)) + commentCost(m.comment)
}

// maxCost is the largest comment cost we allow, from the limit on its length.
func maxCost() int64 { return commentPaddingLimit + 1 }

// validCost says whether a comment of exactly this cost can be built.
//
// There is a gap at one, and it is a property of the writer rather than of the
// format. A zero length comment with the flag set is legal and costs the one
// terminating byte - measured, and all five readers accept it - but Go leaves
// the flag off for an empty string, so the smallest comment it will emit is one
// character and costs two. That gap only bites without a label, because a label
// already costs more than that and grows a byte at a time from there.
func validCost(label string, cost int64) bool {
	if label == "" {
		return cost == 0 || cost >= 2
	}
	return cost >= int64(len(label))+1
}

// commentOfCost builds a comment that costs exactly cost and starts with the
// label.
//
// The cost has to have passed validCost, which is the same question asked
// before the decision to use one - both callers ask it. Repeating it here would
// be a third copy of one rule, and a copy is where two answers come from. What
// happens without it is a panic inside strings.Repeat rather than a wrong file,
// which is the failure this project prefers of the two.
func commentOfCost(label string, cost int64) string {
	if cost == 0 {
		return ""
	}
	return label + strings.Repeat(" ", int(cost)-1-len(label))
}

// pad decides where the difference between the bare archive and the size that
// was asked for goes.
func pad(m *memo, p *format.Plan, target int64, label string, groups []format.Content) error {
	fixed := gzipFixed(tarLength(*m))
	bare := fixed + commentCost(label)

	if target < bare {
		return &format.BelowMinimumError{
			Format:    "TAR.GZ",
			Requested: target,
			Minimum:   bare,
			Reason:    fmt.Sprintf("an archive holding %s already needs that much, and nothing is compressed", describeGroups(groups)),
			Hint:      fmt.Sprintf("Ask for %d B or more, or hold fewer or smaller files.", bare),
		}
	}

	// What the comment would have to cost to land on the target on its own.
	want := target - fixed
	if want <= maxCost() {
		if !validCost(label, want) {
			return &format.BelowMinimumError{
				Format:    "TAR.GZ",
				Requested: target,
				Minimum:   bare + 2,
				Reason:    "the gzip comment that would make up the difference cannot be one byte long, so this size sits in a gap just above the smallest archive",
				Hint:      fmt.Sprintf("Ask for %d B or more, or keep the label on and any size from %d B works.", bare+2, bare),
			}
		}
		m.comment = commentOfCost(label, want)
		return nil
	}

	// Above what the comment holds, the bulk goes into a stored entry inside
	// the tar. A tar entry is aligned to 512 bytes, so it never lands on an
	// exact size by itself - the comment closes the last few bytes.
	size, comment, ok := solveFiller(tarLength(*m), target, label)
	if !ok {
		return fmt.Errorf("targz: no arrangement of padding reaches exactly %d B", target)
	}
	m.withFiller = true
	m.fillerSize = size
	m.comment = comment
	p.Properties["padding_entry"] = fillerName
	return nil
}

// solveFiller picks how big the padding entry is and what the comment says.
//
// The padding entry is a whole number of tar blocks, so it moves the total in
// steps of 512. The largest step that still fits is taken first, and whatever
// is left over becomes comment. Stepping back down by a block adds 512 to that
// leftover, which is how the one byte gap is stepped over when there is no
// label.
func solveFiller(base, target int64, label string) (size int64, comment string, ok bool) {
	minCost := commentCost(label)
	withHeader := base + tarBlock
	if gzipFixed(withHeader)+minCost > target {
		return 0, "", false
	}

	lo, hi := int64(0), target/tarBlock+2
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if gzipFixed(withHeader+mid*tarBlock)+minCost <= target {
			lo = mid
		} else {
			hi = mid - 1
		}
	}

	// Three candidates is enough: each step down adds 512 to the leftover, and
	// the comment holds several thousand.
	for k := lo; k >= 0 && k > lo-3; k-- {
		blocks := k * tarBlock
		cost := target - gzipFixed(withHeader+blocks)
		if validCost(label, cost) && cost <= maxCost() {
			return blocks, commentOfCost(label, cost), true
		}
	}
	return 0, "", false
}

// build writes the archive.
//
// Everything is stored rather than deflated, which is what makes the size above
// exact in a single pass. Reaching an exact size with real compression needs
// the stream measured before the comment can be sized, so it is two passes and
// a separate decision.
func build(ctx context.Context, w io.Writer, m memo) error {
	zw, err := gzip.NewWriterLevel(w, gzip.NoCompression)
	if err != nil {
		return fmt.Errorf("targz: the archive could not be started: %w", err)
	}
	zw.Comment = m.comment
	tw := tar.NewWriter(zw)

	for _, c := range m.children {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := writeEntry(ctx, tw, tarEntry{name: c.name, size: c.plan.Bytes, own: m.own},
			func(dst io.Writer) error {
				return c.desc.Generator.Write(ctx, dst, c.plan)
			}); err != nil {
			return fmt.Errorf("targz: the %s file inside could not be written: %w", c.desc.ID, err)
		}
	}

	if m.withFiller {
		if err := writeEntry(ctx, tw, tarEntry{name: fillerName, size: m.fillerSize, own: m.own},
			func(dst io.Writer) error {
				return writeFiller(ctx, dst, m.seed, m.fillerSize)
			}); err != nil {
			return err
		}
	}

	if err := tw.Close(); err != nil {
		return err
	}
	return zw.Close()
}

// writeEntry emits one tar entry.
//
// USTAR is asked for by name rather than left to the library. The library
// would otherwise reach for a format that carries long names in extra blocks,
// and those blocks are invisible to the arithmetic above - the size would come
// out wrong only for archives holding a long name. Asking for USTAR turns that
// into a refusal at the point of writing.
func writeEntry(ctx context.Context, tw *tar.Writer, e tarEntry, body func(io.Writer) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := tw.WriteHeader(&tar.Header{
		Name:     e.name,
		Size:     e.size,
		Mode:     e.own.Mode,
		Uid:      e.own.Uid,
		Gid:      e.own.Gid,
		Uname:    e.own.Uname,
		Gname:    e.own.Gname,
		ModTime:  fixedTime,
		Typeflag: tar.TypeReg,
		Format:   tar.FormatUSTAR,
	}); err != nil {
		return err
	}
	return body(tw)
}

// tarEntry is one entry's header, as both the measuring pass and the
// writing pass describe it.
//
// A record rather than more arguments, because every field of a USTAR
// header is fixed width - so none of this changes the size of anything, and
// a reader should be able to see that at a glance rather than by counting.
type tarEntry struct {
	name string
	size int64
	own  archive.Ownership
}

// writeFiller emits the padding entry without holding it in memory.
func writeFiller(ctx context.Context, w io.Writer, seed uint64, n int64) error {
	rng := core.NewRand(seed)
	buf := make([]byte, writeChunk)
	for remaining := n; remaining > 0; {
		if err := ctx.Err(); err != nil {
			return err
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
