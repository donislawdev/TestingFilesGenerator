package engine

import (
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// How much of one target is planned at once (O355).
//
// The files of a target are planned a stretch at a time, the files of a
// stretch beside each other (beside, in parallel.go), and their answers are
// read in order before the next stretch starts. So a plan of many pictures
// takes as long as its widest stretch rather than as every file in a row,
// while everything decided per file - its name, the run's total, a refusal -
// is still decided one file after the other, in the order it always was.
//
// A stretch is shared out only when its files are worth it. A file that plans
// in a few microseconds - text, JSON - gains nothing from goroutines and pays
// for handing it to one: measured on 2026-11-18 with tools/probes/plan-ab.py,
// a hundred thousand text files planned 3% slower beside each other, while two
// thousand PNGs planned 5.8 times faster and twenty thousand PDFs 1.6 times
// (docs/PLAN-AND-WHEEL-2026-11-18.md). So every stretch is timed, and the next
// one goes on this goroutine while a file costs less than planWorthSharing.
// The clock decides how fast, never what: the plan is the same either way.
//
// Three limits on a stretch shared out, each from something a plan of one
// file at a time had without asking.
//
//   - The first file of a target is planned alone. A target refused at its
//     first file - the commonest refusal there is, a size its format cannot
//     make - is then refused at the cost of one file rather than of a
//     stretch, and that one file says what a file of the target holds.
//   - What the files planned at once hold stays inside what the memory
//     setting leaves (the same room the writers are held to, roomLeft). A
//     picture plans by drawing and encoding itself, so sixteen of them at once
//     hold sixteen pictures. How much one holds is known only once it is
//     planned, so a file not yet planned is taken to hold as much as the
//     dearest one so far - or more, in proportion to its size, since a picture
//     that is not given its width and height grows with the bytes asked for.
//   - A stretch never passes the next reading of the plan's ceiling
//     (planMemory.ahead), so a plan that crosses the ceiling is refused at the
//     file it was refused at before, not a stretch later.
//
// A run that is cancelled loses at most one stretch of work it did not need,
// and a target refused part way through plans at most one stretch past the
// file that refused.

// planStretchPerWriter is how many files a stretch of the plan gives each
// goroutine before they all wait for the slowest.
//
// A stretch ends when its last file is planned, so the goroutines that finish
// first stand idle while it is - about one file's time per stretch, and a
// longer stretch shares that over more files. A refusal part way through a
// stretch throws the rest of it away, and a longer stretch throws away more.
// The walk of a refused batch (batchminimum.go) refuses part way through by
// design, so it takes one file a goroutine, where a stretch costs one file's
// time whatever it holds and never more than asking one file at a time.
const planStretchPerWriter = 8

// planWorthSharing is the least planning one file has to take for the files
// of a stretch to be planned beside each other.
//
// Measured on 2026-11-18, the time of a file planned alone: text, JSON and CSV
// under what the clock of Windows can tell apart, a PDF of 16 kB 50 us, a DOCX
// 107 us, a PNG of 640x480 7.4 ms - and a PDF already plans 1.6 times faster
// shared out. The clock of Windows moves in ticks, so a stretch of text now
// and then reads as a tick and the next stretch is shared out, which the one
// after takes back. That costs a stretch, never a different plan.
const planWorthSharing = 20 * time.Microsecond

// pace is what one target has shown so far about what its files hold.
type pace struct {
	// run is what the memory setting lets the run hold, nought for no limit
	// (core.MemoryLimits.Run).
	run int64
	// seen says a file of the target has been planned, so most and perByte
	// mean something.
	seen bool
	// most is the most a planned file of the target holds.
	most int64
	// perByte is the most a planned file held for each byte it plans to be,
	// for files asked for more bytes than any so far.
	perByte float64
	// cost is what planning one file took in the last stretch: the stretch's
	// time, times the goroutines it had, over its files.
	cost time.Duration
}

// saw takes in one planned file of the target.
func (p *pace) saw(plan format.Plan) {
	p.seen = true
	p.most = max(p.most, plan.Holds)
	if plan.Bytes > 0 {
		p.perByte = max(p.perByte, float64(plan.Holds)/float64(plan.Bytes))
	}
}

// holds is what a file of the target asked for size bytes is taken to hold.
func (p *pace) holds(size int64) int64 {
	return max(p.most, int64(p.perByte*float64(size)))
}

// stretch is how many of the remaining files to plan next, and on how many
// goroutines. perWriter is planStretchPerWriter or one, largest the most
// bytes any of the remaining files is asked for, and ahead how many files may
// be planned before the ceiling of the plan is read again.
func (p *pace) stretch(remaining, perWriter int, largest int64, ahead int) (files, width int) {
	if !p.seen || remaining <= 1 || ahead <= 1 {
		return 1, 1
	}
	width = widthFor(remaining)
	if width <= 1 {
		return 1, 1
	}
	if p.cost < planWorthSharing {
		// Files too cheap to share out, a stretch of them on this goroutine,
		// timed again so a target whose files grow dearer is shared out after
		// all.
		return min(remaining, width*perWriter, ahead), 1
	}
	if one := p.holds(largest); p.run > 0 && one > 0 {
		width = min(width, int(max(roomLeft(p.run, int64(width)*one)/one, 1)))
	}
	if width <= 1 {
		return 1, 1
	}
	return min(remaining, width*perWriter, ahead), width
}

// planStretch plans the files of one stretch (beside) and times them for the
// next stretch's decision.
func planStretch[T any](p *pace, out []T, from, width int, ask func(i int) T) {
	start := time.Now()
	beside(out, from, width, ask)
	p.cost = time.Since(start) * time.Duration(width) / time.Duration(max(len(out), 1))
}

// answer is what planning one file came to.
type answer struct {
	plan format.Plan
	err  error
}
