// Part of package engine. See engine.go.
package engine

import "github.com/donislawdev/TestingFilesGenerator/internal/core"

// Whether a run will hold more than its memory setting gives it, said before
// its first byte (O296, docs/FILE-CEILING-2026-11-02.md section 13).
//
// A run holds its whole plan to the end, and every file adds its answer and its
// manifest entry as it finishes. The plan is already weighed while it is built
// (planmemory.go), and what the files add is not, so a run of 600 000 text files
// in a container of 1 GB passed every check, wrote for fifteen minutes and was
// ended by the system with no sentence and no manifest. With Docker's default
// swap the same run finished, so this says and does not refuse: whether a run
// past its setting is ended depends on swap the program cannot see.
//
// Measured against the setting rather than against the machine. Under auto the
// setting is half the machine, and that is also the limit the runtime is handed
// (processmemory.go): past it the collector works harder and the run is slower,
// which is certain, and the system may end it, which is not. In 1 GB, 300 000
// text files held about 420 MB and finished at the usual speed, 400 000 and
// 500 000 held more than the 512 MiB and finished more slowly, and 600 000 held
// about 840 MB and was ended.

// entryHolds is what one file adds to the memory of a run from the moment it is
// written to the end of the run: its answer and its manifest entry.
//
// Measured 2026-11-03 with the runsize probe, 20 000 files each, as the live
// heap after writing less the live heap after planning: 520 B a file for txt,
// png and zip, 519 for docx and 518 for pdf. The entry does not grow with the
// format, because the properties it carries are the target's own map, shared by
// reference. TestAFileAddsWhatTheForecastSaysToTheRunsMemory keeps the number
// honest.
const entryHolds = 520

// memoryLimits is the memory setting worked out on this machine. Asked once
// by planning and once by the run, rather than carried between them, because
// the two are separate calls and a caller may plan without running.
//
// Here rather than in engine.go since 2026-11-03, when the forecast below took
// that file past its ceiling: this file is about what a run holds against its
// setting, and this is where the setting meets the machine.
func (o Options) memoryLimits() core.MemoryLimits {
	machine := o.MachineMemory
	if machine == nil {
		machine = core.MachineMemory
	}
	return o.Memory.On(machine)
}

// HoldForecast is how much a run will hold at its end, against what its memory
// setting gives it.
type HoldForecast struct {
	// Holds is the heap as the run starts, its plan among it, and what every
	// file will add as it is written.
	Holds int64
	// Room is what the memory setting gives the run. Nought when it gives no
	// limit to measure against - off, or a machine that could not be asked.
	Room int64
	// Over says Holds is past Room.
	Over bool
}

// PredictHold works out what a run of this many files will hold, for the
// sentence both surfaces say before the first byte.
//
// The heap is read the cheap way first, and that reading is never less than
// what is live, so a run it says has room has room. Only a run that may not is
// read again after a collection. That costs what heapInUse says it does, and it
// is paid by a run that is about to be warned.
func PredictHold(files int, opt Options) HoldForecast {
	return predictHold(files, opt.memoryLimits())
}

func predictHold(files int, limits core.MemoryLimits) HoldForecast {
	if limits.Run <= 0 || limits.Unknown != nil {
		return HoldForecast{}
	}
	added := int64(files) * entryHolds
	if held := int64(heapHeldAtMost()) + added; held <= limits.Run {
		return HoldForecast{Holds: held, Room: limits.Run}
	}
	holds := int64(heapInUse()) + added
	return HoldForecast{Holds: holds, Room: limits.Run, Over: holds > limits.Run}
}
