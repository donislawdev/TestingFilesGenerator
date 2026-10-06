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
