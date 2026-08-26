package engine

import (
	"fmt"
	"runtime"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// The ceiling on what a plan is allowed to cost.
//
// core.MaxFilesPerRun counts files, and the number it settled on was justified
// by a measurement: "about 2.4 kB a file, straight all the way", so a million
// files is roughly 2.4 GB. That measurement was taken on txt, and txt is the
// cheapest format this tool has. Re-measured on 2026-08-26 with the plansize
// probe, at two thousand files each:
//
//	txt                     850 B a file
//	docx                   2228 B a file
//	zip                    6153 B a file
//	pdf                    7527 B a file
//	pdf with pages=1000 5244230 B a file
//
// So the ceiling that was meant to keep a plan inside memory does not know
// what a plan costs. "--format pdf --set pages=1000 --count 10000" passes
// every check this tool has - ten thousand is far under the file ceiling and a
// thousand pages is under the page ceiling - and asks for about 52 GB before a
// single byte is written.
//
// Counting files was the wrong quantity. This counts the thing that actually
// runs out, and it does it by measuring rather than by estimating: no format
// has to declare what its memo costs, and a format added later is covered
// without being told about.

const (
	// planCheckFirst is both when the first reading is taken and how many
	// files a run needs before any of this happens at all.
	//
	// The second half is what keeps it cheap. A reading means collecting
	// first, which measured 4.9 ms against 0.023 ms for the reading on its own
	// - fifty rounds each, 2026-08-26 - and a test suite plans thousands of
	// times, almost always a handful of files. Putting a collection on every
	// plan took the guard package from 124 s past its ten minute limit.
	//
	// Sixty four files of the dearest shape measured is about 340 MB, well
	// under the ceiling, so nothing that matters is missed by waiting.
	planCheckFirst = 64

	// planCheckEvery is the longest this waits between readings.
	//
	// An upper bound rather than a fixed interval, and the arithmetic says
	// why: at a fixed 256 a shape costing 26 MB a file - a pdf at its page
	// ceiling - would allocate 6.7 GB between two readings, so a check meant
	// to hold the plan to two gigabytes would let it reach seven. Once the
	// cost of a file is known the next reading is brought forward to land
	// inside the headroom that is left.
	planCheckEvery = 4096
)

// planMemory watches how much the plan has taken and refuses when it passes
// the ceiling.
//
// It answers "how big is it now" rather than "how big will it be", on purpose.
// A projection needs the eventual file count, which is not known until every
// target has been walked, and it needs the per file cost to be uniform, which
// it is not - a recipe can put a thousand page pdf next to a text file. Asking
// the real heap needs neither and cannot be wrong about the shape of the run.
type planMemory struct {
	baseline uint64
	started  bool
	expected int
	seen     int
	nextAt   int
	ceiling  uint64
}

func newPlanMemory(ceiling int64) *planMemory {
	if ceiling <= 0 {
		ceiling = core.MaxPlanBytes
	}
	return &planMemory{nextAt: planCheckFirst, ceiling: uint64(ceiling)}
}

// expect is told how many files a target is about to contribute, before any of
// them are planned.
//
// This is where the baseline is taken, and it is taken only once the run is
// known to be big enough to be worth measuring. Both readings then come from
// the same method - collected first - which is the part that matters: a
// baseline read without collecting counts whatever has not been swept yet, so
// it can be HIGHER than a collected reading taken later, and the growth
// between them comes out negative. That is not a small error in a number, it
// is the check quietly answering "nothing to refuse" for every run.
func (p *planMemory) expect(files int) {
	p.expected += files
	if !p.started && p.expected >= planCheckFirst {
		p.baseline = heapInUse()
		p.started = true
	}
}

// account is called once per planned file. It returns an error only when the
// plan has already passed the ceiling.
func (p *planMemory) account(targetIndex, filesSoFar int) error {
	if !p.started {
		return nil
	}
	p.seen++
	if p.seen < p.nextAt {
		return nil
	}

	grown := heapInUse()
	if grown <= p.baseline {
		// The collector handed memory back between the two readings. That is
		// a plan smaller than the one already measured, so there is nothing to
		// refuse, and treating a negative as enormous would refuse a run for
		// the collector having done its job.
		p.nextAt = p.seen + planCheckEvery
		return nil
	}
	used := grown - p.baseline
	if used <= p.ceiling {
		// Bring the next reading forward to land inside the headroom that is
		// left, so an expensive file cannot carry the plan far past the
		// ceiling between two readings. Half the headroom rather than all of
		// it, because the average is an average - a run whose later files are
		// dearer than its earlier ones would otherwise step straight over.
		perFile := used / uint64(p.seen)
		step := planCheckEvery
		if perFile > 0 {
			if room := int((p.ceiling - used) / perFile / 2); room < step {
				step = room
			}
		}
		if step < 1 {
			step = 1
		}
		p.nextAt = p.seen + step
		return nil
	}

	return &RecipeError{
		Setting: core.TargetAddress(targetIndex, SettingCount),
		Detail: fmt.Sprintf("the plan for this run has reached %s after %s and the ceiling is %s",
			core.HumanBytes(int64(used)),
			core.Count(filesSoFar, "file", "files"),
			core.HumanBytes(int64(p.ceiling))),
		Because: core.PlanTooLargeWhy,
		Remedy:  core.PlanTooLargeFix,
	}
}

// heapInUse is the live heap, after a collection so that the number is about
// what the plan is holding rather than about what has not been swept yet.
//
// Two readings of a heap that has not been collected differ by whatever the
// allocator happened to be carrying, which for this purpose is noise larger
// than the thing being measured.
func heapInUse() uint64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}
