package engine

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// A target whose damage changes the length of its files plans them through
// this: a format that takes the size the file will have ON DISK, asks the real
// generator for the size the damage needs, and answers in the size on disk.
//
// A wrapper rather than a second size carried beside the plan, and that is a
// measurement: on 2026-11-10 six places in the engine and the command line
// read the plan's byte count as the size of the file - the manifest, the
// length check after writing, the progress bar, the run's total, the free
// space check and --dry-run - and a seventh added later would have been the
// one nobody updated. Here every one of them reads the size on disk without a
// line of theirs changing, and so do drawing a size from a range and
// SmallestAccepted, because the refusals they jump along come back in that
// size too (docs/DAMAGE-ETAP-2-2026-11-09.md section 12.5).
//
// A chain that keeps the length is never wrapped, so zero-head plans, writes
// and refuses exactly as it did before lengths could change.

// SmallestDamaged is the smallest file on disk a target of this format with
// this damage is given, for a request shaped like r.
//
// For a caller that has to know before planning - a guard measuring each pair
// at its smallest, or a list announcing the number. Asked of the same format a
// run plans through, so the number cannot be one the run refuses.
func SmallestDamaged(d format.Descriptor, chain damage.Chain, r format.Request) int64 {
	floor, _, err := chain.Floor()
	if err != nil {
		return d.SmallestAccepted(r)
	}
	if chain.ChangesLength() {
		d = damagedFormat(d, chain)
	}
	return max(floor, d.SmallestAccepted(r))
}

// damagedFormat is d, planning through chain.
func damagedFormat(d format.Descriptor, chain damage.Chain) format.Descriptor {
	d.Generator = &damagedGenerator{inner: d.Generator, format: d.ID, chain: chain}
	return d
}

type damagedGenerator struct {
	inner  format.Generator
	format string
	chain  damage.Chain
}

// damagedMemo carries the real generator's plan from planning to writing.
type damagedMemo struct {
	plan format.Plan
}

func (g *damagedGenerator) Plan(r format.Request) (format.Plan, error) {
	return g.PlanContext(context.Background(), r)
}

// PlanContext is Plan, stoppable, so a container wrapped here still stops
// between the files inside it.
func (g *damagedGenerator) PlanContext(ctx context.Context, r format.Request) (format.Plan, error) {
	size := r.Bytes
	want, err := g.chain.Before(size)
	var small *damage.TooSmallError
	if errors.As(err, &small) {
		return format.Plan{}, g.belowTheDamage(ctx, r, small)
	}
	if err != nil {
		return format.Plan{}, err
	}
	r.Bytes = want
	p, err := format.PlanWith(ctx, g.inner, r)
	if err != nil && g.chain.Stretches() {
		p, err = g.stretch(ctx, r, err)
	}
	if err != nil {
		return format.Plan{}, g.inSizeOnDisk(ctx, r, size, err)
	}
	return format.Plan{
		Bytes:       size,
		Exact:       p.Exact,
		Properties:  p.Properties,
		Determinism: p.Determinism,
		Notes:       p.Notes,
		Memo:        damagedMemo{plan: p},
		Work:        p.Work,
		Holds:       p.Holds,
	}, nil
}

// belowTheDamage is a size under the damage's own floor, refused in the shape a
// format refuses a size with, so that drawing a size from a range jumps past
// it instead of stopping. A fixed size of a chain that keeps the length never
// gets here: checkDamage refuses it first, in the damage's own words.
//
// The format may need more than the damage does - junk appended to a PNG
// needs the PNG under it as well - so the floor is asked in turn, and the
// larger answer is the one named.
func (g *damagedGenerator) belowTheDamage(ctx context.Context, r format.Request, small *damage.TooSmallError) error {
	size := r.Bytes
	r.Bytes = small.Floor
	_, deeper := g.PlanContext(ctx, r)
	var below *format.BelowMinimumError
	if errors.As(deeper, &below) && below.Minimum > small.Floor {
		below.Requested = size
		return below
	}
	return &format.BelowMinimumError{
		Format: g.format, Requested: size, Minimum: small.Floor,
		Reason: core.Says("engine.DamagedFloorWhy", "%s needs at least %s",
			core.A("Damage", small.Damage), core.A("Floor", core.ExactBytes(small.Floor))),
		Hint: g.askFor(small.Floor, true)}
}

// Write hands the real plan to the real generator. The damage itself is not
// applied here: the engine opens the chain around the writer it passes in, as
// it does for every damaged file.
func (g *damagedGenerator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	memo, ok := p.Memo.(damagedMemo)
	if !ok {
		return core.Defect(errors.New("a damaged file was written from a plan its damage did not make"))
	}
	return g.inner.Write(ctx, w, memo.plan)
}

// stretch moves a size the format cannot write exactly up to the first one it
// can, for a chain that cuts the file back to its size anyway.
//
// Only above the format's own floor. A size inside a band the format cannot
// reach - eleven byte counts above a PNG's picture, an odd count in UTF-16 -
// costs a few more bytes cut, which is what makes truncate's missing an "at
// least" (owner's call, 2026-11-10). A size under the floor would cost half
// the file and more, so it is refused instead, naming the smallest size that
// works.
func (g *damagedGenerator) stretch(ctx context.Context, r format.Request, first error) (format.Plan, error) {
	var below *format.BelowMinimumError
	if !errors.As(first, &below) || below.Minimum <= r.Bytes {
		return format.Plan{}, first
	}
	want := r.Bytes
	floor, err := g.firstWritable(ctx, r, 0)
	if err != nil || want < floor {
		return format.Plan{}, first
	}
	r.Bytes, err = g.firstWritable(ctx, r, want)
	if err != nil {
		return format.Plan{}, first
	}
	return format.PlanWith(ctx, g.inner, r)
}

// firstWritable is the smallest size at or above from that the real generator
// takes, jumping along the sizes its refusals name, as drawSizes does.
// Sixty four rounds for the reason SmallestAccepted gives.
func (g *damagedGenerator) firstWritable(ctx context.Context, r format.Request, from int64) (int64, error) {
	at := from
	for round := 0; round < 64; round++ {
		r.Bytes = at
		_, err := format.PlanWith(ctx, g.inner, r)
		if err == nil {
			return at, nil
		}
		var below *format.BelowMinimumError
		if !errors.As(err, &below) || below.Minimum <= at {
			return 0, err
		}
		at = below.Minimum
	}
	return 0, core.Defect(errors.New("a format kept naming larger sizes it could not write"))
}

// inSizeOnDisk is a refusal of the real generator, said in the size the
// person asked for rather than the one the damage needed.
//
// Without it a person asking for a 30 B png cut in half would read "png
// cannot be smaller than 74 B. Requested: 60 B" - a number they never wrote,
// and a way out that does not work.
//
// The size a format's refusal names is the next one worth trying, not always
// one it writes - measured 2026-11-10, wav refuses 0 B naming 96 and then 96
// naming 98 - so the way out is walked to a size it really writes first.
// Translated after one step, the refusal named a size that was refused in
// turn.
func (g *damagedGenerator) inSizeOnDisk(ctx context.Context, r format.Request, size int64, err error) error {
	var below *format.BelowMinimumError
	if errors.As(err, &below) {
		first := below.Minimum
		if writable, werr := g.firstWritable(ctx, r, first); werr == nil {
			first = writable
		}
		least := g.smallestReaching(first)
		return &format.BelowMinimumError{
			Format: g.format, Requested: size, Minimum: least,
			Reason: core.Says("engine.DamagedBelowWhy", "%s has to start from a %s file of at least %s",
				core.A("Damage", g.names()), core.A("Format", g.format), core.A("Minimum", core.ExactBytes(first))),
			Hint: g.askFor(least, true)}
	}
	var above *format.AboveMaximumError
	if errors.As(err, &above) {
		most := g.largestWithin(above.Maximum)
		return &format.AboveMaximumError{
			Format: g.format, Requested: size, Maximum: most,
			Reason: core.Says("engine.DamagedAboveWhy", "%s has to start from a %s file of at most %s",
				core.A("Damage", g.names()), core.A("Format", g.format), core.A("Maximum", core.ExactBytes(above.Maximum))),
			Hint: g.askFor(most, false)}
	}
	return err
}

// smallestReaching is the smallest size on disk whose damage needs at least
// written bytes from the generator. What the damage needs grows with the size
// asked for, so this is found by halving.
func (g *damagedGenerator) smallestReaching(written int64) int64 {
	needs := func(size int64) bool {
		w, err := g.chain.Before(size)
		return err == nil && w >= written
	}
	lo, hi := int64(0), int64(1)
	for !needs(hi) && hi < lengthLimit {
		lo, hi = hi, hi*2
	}
	for hi-lo > 1 {
		mid := lo + (hi-lo)/2
		if needs(mid) {
			hi = mid
		} else {
			lo = mid
		}
	}
	return hi
}

// largestWithin is the largest size on disk whose damage needs no more than
// written bytes from the generator.
func (g *damagedGenerator) largestWithin(written int64) int64 {
	return g.smallestReaching(written+1) - 1
}

// lengthLimit is where the search above stops - far past any size a run
// accepts, and far enough below the top of an int64 that doubling cannot wrap.
const lengthLimit = int64(1) << 60

// names is the chain as a person writes it on the command line.
//
// The format's own reason is NOT carried into the refusal. It speaks about the
// size it refused - "a 1x1 picture already encodes to that much" - and beside
// a size the person did not write, "that much" points at the wrong number.
func (g *damagedGenerator) names() string {
	names := make([]string, 0, len(g.chain))
	for _, s := range g.chain {
		names = append(names, s.String())
	}
	return strings.Join(names, ", ")
}

// askFor is the way out, in the size on disk.
func (g *damagedGenerator) askFor(size int64, atLeast bool) core.Said {
	if atLeast {
		return core.Says("engine.DamagedSizeAtLeast", "Ask for %s or more.", core.A("Size", core.ExactBytes(size)))
	}
	return core.Says("engine.DamagedSizeAtMost", "Ask for %s or less.", core.A("Size", core.ExactBytes(size)))
}
