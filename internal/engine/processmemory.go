package engine

import (
	"runtime/debug"
	"sync"
	"sync/atomic"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// The memory setting handed to the Go runtime, in a process that is the
// program's own.
//
// The setting said how much memory a run may take and held only the plan to
// it. The rest of a run - the plan, the manifest entries and what the writers
// throw away - was left to the collector's default, which lets the heap grow
// to about twice what is live before it collects, and a run took about two and
// a half times what it held. In a container of 1 GB that killed a run of
// 400 000 text files under auto, and one of three runs of 300 000
// (docs/FILE-CEILING-2026-11-02.md section 12).
var (
	ownedOnce sync.Once
	owned     atomic.Bool
	// startLimit is the runtime's limit when the process said it was the
	// program's own. Written once, before owned is set, so a reader that sees
	// owned sees it too.
	startLimit int64
)

// OwnProcess says this process is the program's own - the command line or the
// window, with nothing beside it that a limit on its memory would hold back -
// so every plan from now on hands its memory setting to the Go runtime as the
// limit of the whole process. Only the two main functions call it.
//
// Not done for every caller of the engine, because the limit is the process's
// and not the run's. The guards plan hundreds of runs side by side in one test
// binary, some under a setting of a few megabytes, and handing that to the
// runtime would hold every guard beside them to it.
//
// Never undone. The command line ends with its one run, and the window hands
// over the setting of each run it plans, so a changed setting is in force from
// the next run and off gives the runtime back the limit it started with.
func OwnProcess() {
	ownedOnce.Do(func() {
		startLimit = debug.SetMemoryLimit(-1)
		owned.Store(true)
	})
}

// handToRuntime gives the runtime the limit for a run under limits, in a
// process that is the program's own, and does nothing anywhere else.
//
// Called when a plan starts rather than when the writing does, because the
// plan is the first thing both surfaces do for a run and the manifest is saved
// after the writing has returned - a limit set and taken back inside Run would
// leave both outside it.
func handToRuntime(limits core.MemoryLimits) {
	if owned.Load() {
		debug.SetMemoryLimit(limits.ForRuntime(startLimit))
	}
}
