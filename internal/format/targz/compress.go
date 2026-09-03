package targz

import (
	"context"
	"fmt"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/archive"
)

// Settling the padding of a COMPRESSED archive, which cannot be done by
// arithmetic.
//
// A stored tar.gz has a length that follows from its parts, and size.go works
// it out exactly: the tar is 1024 plus 512 and the rounded content per entry,
// and gzip frames that predictably. Compression breaks every term of it. The
// project measured the same thing on TIFF - deflate moves the length with the
// seed, while uncompressed is flat - so the only honest way to learn a
// compressed length is to compress and look.
//
// So the padding is settled here instead, at write time, and the shape is:
//
//	the FILLER carries the bulk. It is random, so the compressor cannot shrink
//	it - measured at about +0.031% - but its compressed length is still not
//	exactly predictable, and a tar entry moves in 512 byte blocks anyway.
//	the EXTRA FIELD closes the remainder exactly. It sits in the gzip header,
//	which is not compressed, so n bytes there cost n+2 in the file. That is the
//	one channel here with byte granularity.
//
// Measured 2026-09-01 across levels 1, 6 and 9 and targets from 64 KB to
// 10 MB: every one landed on the ordered size, and the remainder left for the
// extra field came out between 567 and 3759 bytes - far inside the 65 531 the
// field holds (O163).
//
// This costs passes, and the cost is real: one pass over a 10 MB archive is
// 25 ms at level 1 and 140 ms at level 6, against 8 ms stored. It buys the one
// thing that cannot be given up, which is that the file is the size that was
// ordered.
const solveRounds = 8

// counter counts what a write would come to without keeping any of it.
type counter struct{ n int64 }

func (c *counter) Write(p []byte) (int, error) { c.n += int64(len(p)); return len(p), nil }

// measure builds the archive described by m and reports its length.
//
// It writes nothing anybody keeps, but it does GENERATE the files inside,
// because that is the only way to learn what they compress to. That is the
// price of compression in this container and it is paid at write time, never
// at planning time - the guard that keeps a preview cheap is about planning.
func measure(ctx context.Context, m memo) (int64, error) {
	c := &counter{}
	if err := build(ctx, c, m); err != nil {
		return 0, err
	}
	return c.n, nil
}

// settleCompressed finds the filler and extra field that make the archive come
// to exactly m.target.
//
// It walks rather than solving in one step because the two channels do not have
// the same granularity: a tar entry moves in 512 byte blocks and the filler is
// compressed on the way in, so asking for n more bytes of filler does not add
// exactly n to the file. The extra field does add exactly what it is given, so
// it always gets the last word.
func settleCompressed(ctx context.Context, m memo) (memo, error) {
	bare := m
	bare.withFiller, bare.fillerSize = false, 0
	bare.withExtra, bare.extraLen = false, 0

	base, err := measure(ctx, bare)
	if err != nil {
		return m, err
	}
	if base > m.target {
		return m, belowMinimum(m.target, base)
	}
	return settleRound(ctx, bare, m.target, m.target-base, solveRounds)
}

// settleRound tries one filler and either lands or says what to try next.
//
// Written as a walk rather than a loop, and that is not decoration: a loop
// carrying an error check and a decision inside it nests three deep, and this
// project counts how many functions do. A bounded recursion says the same
// thing at two, and a solve that converges reads naturally as "try this, and
// if it is not right, try the next" anyway. left bounds it, so there is no
// depth to worry about.
func settleRound(ctx context.Context, bare memo, target, filler int64, left int) (memo, error) {
	if left == 0 {
		return bare, fmt.Errorf(
			"targz: the padding of this compressed archive does not settle after %d rounds. "+
				"Ask for a different size, or for compression: none", solveRounds)
	}
	if err := ctx.Err(); err != nil {
		return bare, err
	}

	try := bare
	try.withFiller, try.fillerSize = filler > 0, filler
	got, err := measure(ctx, try)
	if err != nil {
		return bare, err
	}

	next, extra, useExtra, done := nextFiller(target, got, filler)
	if done {
		try.withExtra, try.extraLen = useExtra, extra
		return try, nil
	}
	if next < 0 {
		return bare, belowMinimum(target, got)
	}
	return settleRound(ctx, bare, target, next, left-1)
}

// nextFiller reads one measurement and says what to do with it.
//
// The four answers are the whole of the arithmetic, and which one applies is
// decided by how much is left over rather than by preference.
func nextFiller(target, got, filler int64) (next, extra int64, useExtra, done bool) {
	switch deficit := target - got; {
	case deficit < 0 || (deficit > 0 && deficit < 2):
		// Overshot, or left a remainder the extra field cannot hold: it costs
		// two bytes before it holds anything. Give the filler back enough that
		// the field has room to work, and give back A WHOLE BLOCK MORE.
		//
		// The block is the point, and giving back only the overshoot is what
		// used to happen and does not converge. The filler is a tar entry, and
		// a tar entry is padded up to a whole 512 byte block - so handing back
		// fewer than 512 bytes changes WHICH bytes the archive carries and not
		// HOW MANY. What comes out the other side of gzip then wobbles by a
		// byte or so either way, and the walk spends its rounds stepping five
		// bytes at a time across a staircase, never landing.
		//
		// Measured 2026-09-01: a 256 KiB archive at compression best sat at
		// 262 147 and 262 148 B for eight rounds while the filler came down
		// from 258 481 to 258 447. This had been true all along and Go 1.27
		// only moved which sizes land on a step edge, so it looked like the
		// compiler broke it. Probe: tools/probes/targzsettle.
		//
		// Undershooting is safe and overshooting is not: the extra field adds
		// exactly what it is given, up to 65 531 B, so a round that lands under
		// the target finishes on the next pass. A whole block is well inside
		// that.
		next := filler - (2 - deficit) - tarBlock
		if next < 0 && filler > 0 {
			// Try with no filler at all before deciding the size is out of
			// reach. Only an archive that overshoots with nothing in it is
			// genuinely too small.
			next = 0
		}
		return next, 0, false, false
	case deficit == 0:
		// Landed without needing the field at all.
		return filler, 0, false, true
	case deficit-2 > extraPaddingLimit:
		// More left than the header can hold, so the filler takes it.
		return filler + deficit - 2 - extraPaddingLimit, 0, false, false
	default:
		return filler, deficit - 2, true, true
	}
}

// writeCompressed settles the padding and then writes the archive.
//
// Two passes at least, and the reason is in settleCompressed: the length of a
// compressed archive is not knowable without compressing it. Nothing is held
// in memory between them - the measuring pass throws its bytes away as it
// makes them, so an archive larger than memory still works.
func writeCompressed(ctx context.Context, w io.Writer, m memo) error {
	settled, err := settleCompressed(ctx, m)
	if err != nil {
		return err
	}
	return build(ctx, w, settled)
}

// belowMinimum says the archive cannot be made this small once its contents
// are in it.
//
// The number it reports is measured rather than derived: it is what this
// archive actually came to when it was compressed with nothing added, which is
// the smallest it can be. A stored archive can say the same thing by
// arithmetic, and a compressed one cannot.
func belowMinimum(target, floor int64) error {
	return &format.BelowMinimumError{
		Format:    "TAR.GZ",
		Requested: target,
		Minimum:   floor,
		Reason:    "that is what the contents come to once they are compressed, so nothing can be taken away",
		Hint:      fmt.Sprintf("Ask for %d B or more, or hold fewer or smaller files.", floor),
	}
}

// reachable refuses a compressed archive whose size the contents already
// exceed, using the STORED arithmetic.
//
// A cheap early refusal rather than a proof, and the sentence that used to
// stand here claimed the second thing. It said "compression only ever makes the
// contents smaller, so an archive that fits when stored fits when squeezed".
// That is false, and it is the exact assumption zip fell over on: deflate GROWS
// data that is already compressed. Measured on 2026-09-02 with two Office
// documents inside a zip, a band 50 B wide at 2 MB where the space compression
// freed came out negative and the file was written short of its size.
//
// What keeps this format to the byte is not this check. It is
// settleCompressed, which measures the real stream and iterates until the
// padding lands, and refuses when it cannot. This one answers only "even
// stored, the contents already exceed the size asked for", which is worth
// having because it is the answer a preview can give without compressing
// anything.
func reachable(m *memo, target int64, label string, groups []format.Content) error {
	probe := *m
	probe.squeeze = archive.Squeeze{}
	var p format.Plan
	p.Properties = map[string]any{}
	return pad(&probe, &p, target, label, groups)
}
