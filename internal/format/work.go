package format

import "context"

// The work a file costs beyond its bytes (Plan.Work), and how a generator says
// how much of it is done.
//
// A film is the case this exists for: thirty minutes of 1920x1080 is a
// hundred megabytes of pictures that take a minute to code and two gigabytes
// of padding that take five seconds to write, so a bar counting bytes stood at
// five percent for the whole minute and promised three hours
// (docs/WEBM-WYDAJNOSC-2026-10-06.md). Kept out of format.go, which was at its
// ceiling on length, because it is a subject of its own.

type workKey struct{}

// WithWork is ctx carrying report, which Worked calls. The engine sets it on
// the context a generator writes under, when somebody is watching the run.
func WithWork(ctx context.Context, report func(int64)) context.Context {
	return context.WithValue(ctx, workKey{}, report)
}

// Worked says that n more of the plan's Work is done. It goes through the
// context rather than the writer, because the writer a generator is handed is
// not always the engine's - a damaged file is written through the damage -
// and from the goroutine Write runs on, which is the one the engine counts
// the bytes on. With nobody watching it does nothing.
func Worked(ctx context.Context, n int64) {
	if report, ok := ctx.Value(workKey{}).(func(int64)); ok {
		report(n)
	}
}

// AllocCeiling is implemented by a generator that may allocate more objects
// producing one file than the flat ceiling every other one meets. A generator
// without it meets the flat one, which is the case for all but three.
//
// It exists because a borrowed encoder allocates on its own account. The hand
// written generators here sit between 3 and 128 objects a file, and gav1d, the
// AVIF encoder, sits at about a hundred - but gen2brain/jxl allocates per
// block, about 618 000 of them for one 640x480 picture. A single ceiling has to
// fit the heaviest format, so one that fits that one would say nothing about
// the others.
//
// What the ceiling stands in for is untouched by this: the guard also asks
// each format whether its allocation GROWS with the size of the file asked for,
// and that question is the real one. Owner's decision, 2026-08-31. A ratchet,
// like the coverage threshold and the code shape ceilings: it goes down when
// work makes it lowerable, never up to turn a run green.
//
// A method of the generator rather than a field of the descriptor since
// 2026-10-07, when the descriptor reached the field count the type shape guard
// watches - how much a generator allocates is the generator's to say.
type AllocCeiling interface {
	AllocCeiling() int64
}
