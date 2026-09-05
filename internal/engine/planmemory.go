package engine

import (
	"fmt"
	"runtime"
	"runtime/metrics"

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
//
// What it can be wrong about is whose heap it is. ReadMemStats answers for the
// whole process, so anything else allocating in it while a plan is being built
// is counted against the plan - O162. Two gigabytes is a great deal to borrow
// by accident, and every run this tool was designed around is orders of
// magnitude under the ceiling, so the room for a wrong refusal is small. It is
// not nought, and this is where it lives.
type planMemory struct {
	baseline uint64
	// allocsBaseline is the running total of bytes this process has ever
	// allocated, taken beside the baseline above.
	//
	// It is what makes the cheap question in account possible, and the reason
	// it works is an inequality rather than an estimate: every byte the live
	// heap grows by has to have been allocated, so the growth this guard cares
	// about can never exceed the total allocated since the baseline. Under the
	// ceiling on that total means under the ceiling full stop, and no
	// collection has to be paid for to find out.
	allocsBaseline uint64
	seen           int
	nextAt         int
	ceiling        uint64
}

// newPlanMemory takes the reference point, and it is taken here because here
// is before the first target has been looked at.
//
// Both readings come from the same method - collected first - which is the
// part that matters: a baseline read without collecting counts whatever has
// not been swept yet, so it can be HIGHER than a collected reading taken
// later, and the growth between them comes out negative. That is not a small
// error in a number, it is the check quietly answering "nothing to refuse" for
// every run.
//
// Until 2026-09-03 this waited. The reading was taken only once a run had
// announced sixty four files, and account below returned at once until then,
// so a run of sixty three files had no ceiling at all. The comment that
// justified the wait said sixty four files of the dearest shape measured is
// about 340 MB, so nothing that matters is missed. Measured again with the
// plansize probe, this time on containers rather than on plain formats:
//
//	zip, entries=10000, entry_format=pdf    74740758 B a file
//	targz, the same                         74758716 B a file
//	pdf, pages=5000                         25194908 B a file
//	pdf, pages=1000                          5244543 B a file
//
// Five points from one file to sixteen for the first shape, linear to within
// 0.05%. Twenty nine files of it come to 2.17 GB, which is past the ceiling
// and was accepted without a reading, because twenty nine is under sixty four.
// Sixty three files come to 4.71 GB. And the wait was worse than that number
// says, because the count it waited on was the whole run's, added up across
// targets: sixty four targets holding one file each took the reading after
// SIXTY THREE files had been planned, and those then sat inside the baseline
// for the rest of the run, however long it ran.
//
// What the waiting bought is measured rather than assumed, since the number
// that justified it - "putting a collection on every plan took the guard
// package from 124 s past its ten minute limit" - turns out not to describe
// this. A reading here and a first reading at the first file, four runs of the
// guard package interleaved on 2026-09-03: 277.9 s and 283.7 s without them
// against 292.3 s and 306.1 s with, so about six percent. The ranges do not
// overlap, which is the only reason a conclusion is drawn from four runs.
func newPlanMemory(ceiling int64) *planMemory {
	if ceiling <= 0 {
		ceiling = core.MaxPlanBytes
	}
	return &planMemory{
		baseline:       heapInUse(),
		allocsBaseline: allocatedSoFar(),
		nextAt:         1,
		ceiling:        uint64(ceiling),
	}
}

// account is called once per planned file. It returns an error only when the
// plan has already passed the ceiling.
//
// The cheap question is asked first and it is the one nearly every run gets to
// stop at. Until 2026-09-05 there was no cheap question: every reading was a
// forced collection, so a run of one file of one kilobyte paid for TWO of them
// - measured with GODEBUG=gctrace=1, exactly two on every run however small.
// The cost is 519 us against 251 ns for the counter, and it grows with the live
// heap, which is precisely the run this guard exists for.
//
// What the counter cannot do is answer the question this refuses on. It counts
// everything ever allocated, including what has already been thrown away, so it
// says "no" with certainty and "maybe" otherwise. The certain half is enough,
// because it is the half every real run lands in: MaxPlanBytes is two
// gigabytes and planning ten thousand files allocates tens of megabytes.
//
// /gc/heap/live:bytes was the obvious other candidate and it is the wrong one.
// It reports the live heap as of the LAST COLLECTION, so a burst of allocation
// between two collections is invisible to it - which would turn a refusal into
// silence exactly when the plan is growing fastest. The counter has no such
// gap.
func (p *planMemory) account(targetIndex, filesSoFar int) error {
	p.seen++
	if p.seen < p.nextAt {
		return nil
	}

	if allocated := allocatedSoFar() - p.allocsBaseline; allocated <= p.ceiling {
		// Bring the next reading forward to land inside the headroom that is
		// left, for the reason written out below - and on this figure rather
		// than on the swept one, which makes the step SHORTER than it needs to
		// be rather than longer, since what has been allocated is never less
		// than what is still held.
		p.nextAt = p.seen + stepFor(p.ceiling, allocated, p.seen)
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
		p.nextAt = p.seen + stepFor(p.ceiling, used, p.seen)
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

// stepFor is how many more files may go by before the next reading.
//
// It brings the reading forward to land inside the headroom that is left, so
// an expensive file cannot carry the plan far past the ceiling between two of
// them. Half the headroom rather than all of it, because the average is an
// average - a run whose later files are dearer than its earlier ones would
// otherwise step straight over.
//
// One function rather than the same arithmetic twice, since 2026-09-05 there
// are two readings it has to serve: the cheap counter and the swept heap. They
// disagree about the number they are given and agree about what to do with it.
func stepFor(ceiling, used uint64, seen int) int {
	step := planCheckEvery
	if perFile := used / uint64(seen); perFile > 0 {
		if room := int((ceiling - used) / perFile / 2); room < step {
			step = room
		}
	}
	if step < 1 {
		step = 1
	}
	return step
}

// allocatedSoFar is every byte this process has ever allocated, thrown away or
// not.
//
// Cumulative and exact - unlike the live heap, it does not wait for a
// collection to catch up, so a burst of allocation is visible the moment it
// happens. Measured 2026-09-05: 251 ns a call against 519 us for a forced
// collection and its reading.
func allocatedSoFar() uint64 {
	sample := [1]metrics.Sample{{Name: "/gc/heap/allocs:bytes"}}
	metrics.Read(sample[:])
	return sample[0].Value.Uint64()
}

// heapInUse is the live heap, after a collection so that the number is about
// what the plan is holding rather than about what has not been swept yet.
//
// Two readings of a heap that has not been collected differ by whatever the
// allocator happened to be carrying, which for this purpose is noise larger
// than the thing being measured.
//
// It costs what a collection costs, which measured 4.9 ms against 0.023 ms for
// the reading on its own - fifty rounds each, 2026-08-26. That is why the
// schedule above exists: a run of ten thousand files takes a handful of these
// rather than ten thousand.
func heapInUse() uint64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}
