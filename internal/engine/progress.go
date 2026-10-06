// Part of package engine. See engine.go.
package engine

import (
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// How far a run has got, and what it is measured against. Moved out of
// engine.go on 2026-10-06, when progress began to count the work a file costs
// as well as its bytes and that file passed its ceiling on length - the ceiling
// is a ratchet, so the answer is a split by subject. The gate that hands the
// numbers out while files are written in parallel stays in parallel.go,
// because it holds a lock and that file is where everything concurrent in this
// package lives.

// TotalBytes is what a plan will occupy on disk. Known before the first byte
// is written, which is what the free space guard and --dry-run stand on.
func TotalBytes(files []PlannedFile) int64 {
	var n int64
	for _, f := range files {
		n += f.Plan.Bytes
	}
	return n
}

// totalWork is what the planned files cost to make, their bytes included.
func totalWork(files []PlannedFile) int64 {
	var n int64
	for _, f := range files {
		n += f.Plan.Bytes + f.Plan.Work
	}
	return n
}

// Progress is how far a run has got. Both counts are known from the plan, so
// the fractions are exact rather than estimated.
//
// A fraction of the run and the time it has left come from the work, not the
// bytes. The work is the bytes plus what the plans counted beyond them
// (format.Plan.Work), so for a run of files whose cost is their bytes the two
// are the same numbers, and for a film they are the pictures as well as the
// padding - the bytes alone stood at five percent while a film's pictures were
// coded and promised hours for a minute's work.
type Progress struct {
	FilesDone  int
	FilesTotal int
	BytesDone  int64
	BytesTotal int64
	WorkDone   int64
	WorkTotal  int64
}

// Percent is how far the run has got, out of a hundred, by the work. The
// window's bar and the command line's both draw this, so the two cannot
// disagree and neither can fall back to the bytes on its own.
func (p Progress) Percent() int { return core.Percent(p.WorkDone, p.WorkTotal) }

// Left is how long the run has left at the pace it has kept so far, by the
// work, and false while there is not enough to go on: in the first second,
// before any work is done and once all of it is. A number that swings wildly
// for the first second is worse than no number. The two surfaces carried a
// copy of this each until 2026-10-06.
func (p Progress) Left(elapsed time.Duration) (time.Duration, bool) {
	if elapsed < time.Second || p.WorkDone <= 0 || p.WorkDone >= p.WorkTotal {
		return 0, false
	}
	return time.Duration(float64(elapsed) * float64(p.WorkTotal-p.WorkDone) / float64(p.WorkDone)), true
}
