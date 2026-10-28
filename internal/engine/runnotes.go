// Part of package engine. See engine.go.
package engine

import "github.com/donislawdev/TestingFilesGenerator/internal/core"

// What a run says about the memory it ran in - Result.RunNotes.
//
// Both are said once per run rather than once per file, for the reason
// manifest.Notes gives: the same sentence twenty five thousand times buries the
// line that matters. And neither reaches the manifest, because both are about
// the machine rather than about any file on it.

// noteMemory says the machine could not be asked, when it could not. Auto then
// worked to the old ceiling, and a run that does not say so has a limit nobody
// can see the source of (untouchable rule 6).
func (res *Result) noteMemory(limits core.MemoryLimits) {
	if limits.Unknown == nil {
		return
	}
	res.RunNotes = append(res.RunNotes, core.Says("engine.MemoryUnknown",
		"the memory of this machine could not be read (%v), so this run worked to %s, the most any run took before the memory setting existed. "+
			"Set the memory setting yourself to choose the limit",
		core.A("Cause", limits.Unknown), core.A("Limit", core.Bytes(core.MaxPlanBytes))))
}

// noteOverrun says a file was written past the memory setting, when one was.
// Only the first is named. The rest of the run's shape is the same question
// asked again, and the answer to all of them is the one setting.
func (res *Result) noteOverrun(o *Overrun) {
	if o == nil {
		return
	}
	res.RunNotes = append(res.RunNotes, core.Says("engine.MemoryOverrun",
		"%s holds about %s while it is written, more than the %s the memory setting left the files being written, "+
			"so it was written on its own and the run went past the setting until it finished",
		core.A("Name", core.Shown(o.Name)), core.A("Holds", core.Bytes(o.Holds)), core.A("Room", core.Bytes(o.Room))))
}
