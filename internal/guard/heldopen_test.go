package guard

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// heldOpenGenerator is a file that never finishes.
//
// It exists for one guard - TestARunStoppedPartWayNamesEveryFileThatFinished -
// which needs a run where one writer is still owed bytes while its neighbours
// have already been renamed into place. That state used to be bought with
// size, a first file sixteen times the size of the rest, and that was a race:
// which writer finishes first is decided by which one gets a processor, not by
// how much work it was given. Measured on 2026-09-08, a starved machine failed
// it fourteen times in twenty five. The note above that guard carries the
// numbers.
//
// Writing nothing and waiting removes the race rather than widening it. No
// amount of scheduling pressure can make this file finish, so the hole is a
// property of the plan instead of an outcome of a contest.
//
// This is put in place AFTER planning, on one PlannedFile, so it never enters
// the registry and no other file sees it. Every step of the engine below
// planning is the one that ships.
type heldOpenGenerator struct{}

// heldOpenDeadline is a safety net and not the mechanism.
//
// The cancellation this generator waits for arrives as soon as any other
// writer finishes a file, which on an idle machine is eleven milliseconds and
// on the most starved machine measured was under fifteen seconds. If it never
// arrives at all the run is deadlocked, and a deadlock would hang the whole
// package until the suite timeout kills it with no word about which test was
// stuck. Failing here instead says what happened, on the guard that caused it.
const heldOpenDeadline = 90 * time.Second

// Plan is never called. This generator replaces the real one after planning,
// so the file it belongs to already carries the plan the txt format made.
// Refusing rather than returning a zero plan, because a zero plan would let a
// future caller get a silently empty file out of this.
func (*heldOpenGenerator) Plan(format.Request) (format.Plan, error) {
	return format.Plan{}, errors.New("guard: this generator is installed after planning and has no plan of its own")
}

// Write emits nothing and returns when the run is cancelled.
//
// Returning the context error is what a generator interrupted half way does,
// so the engine sees the same thing it would see from any format that was cut
// off - the temporary file is removed, no entry claims the file, and the index
// stays empty.
func (*heldOpenGenerator) Write(ctx context.Context, _ io.Writer, _ format.Plan) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(heldOpenDeadline):
		return errors.New("guard: the run was never cancelled, so no other writer ever " +
			"finished a file and the hole this guard is about could not occur")
	}
}
