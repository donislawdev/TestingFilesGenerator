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
	// A directory is a header and nothing else, so it costs exactly one block.
	// Measured 2026-09-01 against archive/tar rather than read off the format:
	// a tar holding one directory and nothing else comes to 1536 B, of which
	// 1024 is the end of archive marker. A guard holds that number.
	//
	// The path length does NOT appear here, and that is measured too. USTAR
	// splits a path across a 155 byte prefix and a 100 byte name, so a header
	// is the same 512 bytes at every depth it accepts - flat all the way to
	// the refusal, with no step. maxDepth is what keeps it on the near side.
	total += int64(len(m.layout.Directories())) * tarBlock
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
	return gzipFixed(tarLength(m)) + commentCost(m.comment) + extraCost(m.withExtra, m.extraLen)
}

// extraCost is what a gzip extra field of n bytes costs in the file.
//
// Measured rather than read off the specification: two bytes of length and
// then the bytes themselves, so an EMPTY field still costs two and no field
// at all costs nothing. That difference is the whole reason noExtra is not
// simply zero.
func extraCost(present bool, n int64) int64 {
	if !present {
		return 0
	}
	return n + 2
}

// maxCost is the largest comment cost we allow, from the limit on its length.
// maxCost is the most padding the header can take, across both channels.
//
// The comment reaches commentPaddingLimit beyond the label and the extra
// field reaches extraPaddingLimit plus its own two bytes. Anything past
// this needs a filler entry inside the tar.
func maxCost() int64 { return commentPaddingLimit + extraPaddingLimit + 2 }

// place works out where a given number of padding bytes goes.
//
// Two channels, and which one is used is decided by size rather than by
// preference. The extra field is where bulk goes: it holds 65 531 bytes, it
// is byte granular, and Go reads all of it - which the comment does not, and
// that is O163. But it cannot cost one byte, because its own length field
// costs two, so a single byte of padding has nowhere to go there.
//
// The comment closes that. It already carries the label, so lengthening it by
// one is free of any structural minimum, and one byte is exactly what the
// extra field cannot do. Without a label there is no comment to lengthen and
// the gap at one stays - which is the gap this format has always had, written
// up in MVP-FORMATS.md section 3.1 as a property rather than a fault.
//
// Returns false when the amount cannot be built at all, so the caller refuses
// rather than producing an archive of the wrong size.
func place(label string, padding int64) (comment string, withExtra bool, extraLen int64, ok bool) {
	switch {
	case padding < 0 || padding > maxCost():
		return "", false, 0, false
	case padding == 0:
		return label, false, 0, true
	case padding == 1:
		if label == "" {
			// Nothing to lengthen, and neither channel starts at one.
			return "", false, 0, false
		}
		return label + " ", false, 0, true
	case padding <= extraPaddingLimit+2:
		return label, true, padding - 2, true
	}
	// Past what the field holds, the comment takes the remainder. It is
	// capped well under what Go reads, so the pair together stay readable.
	rest := padding - (extraPaddingLimit + 2)
	if label == "" || rest > commentPaddingLimit {
		return "", false, 0, false
	}
	return label + strings.Repeat(" ", int(rest)), true, extraPaddingLimit, true
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

	// What the header would have to carry to land on the target on its own.
	// The label is already counted in bare, so this is padding and nothing
	// else - which is the change O163 brought: the comment holds the label
	// and the extra field holds the padding, rather than one field holding
	// both and growing past what a Go reader will take.
	want := target - bare
	if want <= maxCost() {
		comment, withExtra, extra, ok := place(label, want)
		if !ok {
			return &format.BelowMinimumError{
				Format:    "TAR.GZ",
				Requested: target,
				Minimum:   bare + 2,
				Reason: "the padding that would make up the difference cannot be one byte long, " +
					"so this size sits in a gap just above the smallest archive",
				Hint: fmt.Sprintf("Ask for %d B or more, or keep the label on and any size from %d B works.", bare+2, bare),
			}
		}
		m.comment, m.withExtra, m.extraLen = comment, withExtra, extra
		return nil
	}

	// Above what the comment holds, the bulk goes into a stored entry inside
	// the tar. A tar entry is aligned to 512 bytes, so it never lands on an
	// exact size by itself - the comment closes the last few bytes.
	size, header, ok := solveFiller(tarLength(*m), target, label)
	if !ok {
		return fmt.Errorf("targz: no arrangement of padding reaches exactly %d B", target)
	}
	m.withFiller = true
	m.fillerSize = size
	m.comment = header.comment
	m.withExtra = header.withExtra
	m.extraLen = header.extraLen
	p.Properties["padding_entry"] = fillerName
	return nil
}

// headerPadding is where a solved arrangement puts the bytes the filler
// entry could not carry. A record rather than three return values, because
// three of them in a row is where a reader stops being able to tell which is
// which.
type headerPadding struct {
	comment   string
	withExtra bool
	extraLen  int64
}

// solveFiller picks how big the padding entry is and where the rest goes.
//
// The padding entry is a whole number of tar blocks, so it moves the total in
// steps of 512. The largest step that still fits is taken first, and whatever
// is left over goes into the header. Stepping back down by a block adds 512 to
// that leftover, which is how the one byte gap is stepped over when there is
// no label to lengthen.
func solveFiller(base, target int64, label string) (size int64, header headerPadding, ok bool) {
	minCost := commentCost(label)
	withHeader := base + tarBlock
	if gzipFixed(withHeader)+minCost > target {
		return 0, headerPadding{}, false
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
		// What is left for the header once the filler has taken its blocks.
		// minCost is the label, which the comment carries either way.
		padding := target - gzipFixed(withHeader+blocks) - minCost
		if comment, withExtra, extra, ok := place(label, padding); ok {
			return blocks, headerPadding{comment: comment, withExtra: withExtra, extraLen: extra}, true
		}
	}
	return 0, headerPadding{}, false
}

// build writes the archive.
//
// Everything is stored rather than deflated, which is what makes the size above
// exact in a single pass. Reaching an exact size with real compression needs
// the stream measured before the comment can be sized, so it is two passes and
// a separate decision.
func build(ctx context.Context, w io.Writer, m memo) error {
	zw, err := gzip.NewWriterLevel(w, m.squeeze.Level)
	if err != nil {
		return fmt.Errorf("targz: the archive could not be started: %w", err)
	}
	zw.Comment = m.comment
	// The extra field is where padding rides. It precedes everything, like
	// the comment, so how long it is has to be settled before the first byte
	// goes out - which is what the arithmetic above is for.
	if m.withExtra {
		zw.Extra = make([]byte, m.extraLen)
	}
	tw := tar.NewWriter(zw)

	// Directories first, outermost first, and only when asked for. They are
	// not in m.children on purpose: a child's seed is FileSeed(seed, index)
	// over a running index, so a directory in that list would shift the seed
	// of every file after it and rewrite its contents. That is untouchable
	// rule 2 - an edit in one place moving the bytes in another.
	for _, dir := range m.layout.Directories() {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := writeDirectory(tw, dir, m.own); err != nil {
			return fmt.Errorf("targz: the directory %q could not be named: %w", dir, err)
		}
	}

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

// writeDirectory names one directory in the tar.
//
// The mode is 0755 rather than own.Mode, and that is a decision rather than an
// oversight. entry_mode is declared as "the permissions recorded for each file
// inside", and a directory is not a file - recording 644 on one would produce
// an archive that extracts into directories nothing can be written into, which
// is a surprise nobody asked this setting for. The owner fields DO follow
// entry_owner, so an archive that says everything belongs to root says it about
// the directories too.
//
// It costs exactly one block and no more, which is the whole of what the
// arithmetic above had to learn about it. Measured rather than read off the
// format, and a guard holds the number.
func writeDirectory(tw *tar.Writer, name string, own archive.Ownership) error {
	return tw.WriteHeader(&tar.Header{
		Name:     name,
		Size:     0,
		Mode:     0o755,
		Uid:      own.Uid,
		Gid:      own.Gid,
		Uname:    own.Uname,
		Gname:    own.Gname,
		ModTime:  fixedTime,
		Typeflag: tar.TypeDir,
		Format:   tar.FormatUSTAR,
	})
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
