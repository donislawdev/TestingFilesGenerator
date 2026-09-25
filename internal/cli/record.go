// Part of package cli. See cli.go.
package cli

// What a run leaves about itself once the files are written: the manifest and
// the instructions beside it, and what is said when a run stopped or its record
// will be too large to read back. Out of generate.go on 2026-09-25, when the
// instructions made that file the largest in the tree.

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// whatSurvived says what a stopped run left behind, for the sentence produce
// prints about a run that ended early.
//
// The window has said this since it had a progress bar - "Stopped after N
// files. The manifest describes exactly those." The command line said "context
// canceled" and left the reader to work out whether the directory was safe to
// reuse. Same run, same facts, and only one surface was saying them.
//
// Only for a stop, and only for a run that got past its preflight. A run
// refused before it wrote anything has nothing to describe, and a run that
// failed for its own reason already says what went wrong in its own words -
// adding a count to either would be answering a question nobody asked.
//
// The claim it makes is the one the manifest keeps: every file that reached the
// disk has an entry, hole allowed. That is the row of the regression surface
// about a stopped run naming what it produced, and it is what makes "tfg
// cleanup" able to take them away again.
//
// PROVEN BY RUNNING IT, NOT BY A GUARD, and that is worth knowing before
// trusting it. Measured on 2026-09-06 in a Linux container against a real
// signal, because a signal cannot be delivered to this process from the shell
// on the machine this was written on:
//
//	SIGINT  into 3000 files  exit 130  "897 files written"  897 on disk
//	SIGTERM into 3000 files  exit 143  "755 files written"  755 on disk
//
// A guard reaches the sentence but not the count. The command line plans before
// it runs, and planning honours the context, so a run started with a finished
// context returns from PlanContext and never arrives here - res is nil and the
// count is never built. Landing between the two needs a cancel timed to arrive
// after planning and before the last file, which is a clock, and a guard built
// on a clock goes red on a busy machine rather than on a defect.
//
// So !res.Started is not reddenable from this surface today. It stays because
// the state it refuses is reachable in the engine - Run sets Manifest at
// construction and Started only after preflight, so a stop returned between
// those two would otherwise print "0 files written, and the manifest describes
// exactly those" about a run that wrote nothing and saved no manifest. That is
// an invented fact rather than a missing one, which is the half of untouchable
// rule 5 that costs trust.
func whatSurvived(runErr error, res *engine.Result) string {
	stopped := errors.Is(runErr, context.Canceled) || errors.Is(runErr, context.DeadlineExceeded)
	if !stopped || res == nil || !res.Started || res.Manifest == nil {
		return ""
	}
	return fmt.Sprintf(" %s written, and the manifest describes exactly those.",
		core.Count(len(res.Manifest.Files), "file", "files"))
}

// echoManifestReach says so when a run will write a manifest this build cannot
// read back.
//
// Measured on 2026-08-26: a run of 25 000 files wrote a manifest of 25 220 640
// B against a read ceiling of 16 777 216 B, and from that point "tfg verify"
// and "tfg cleanup" both exited 5 on it. The files stayed on the disk and
// nothing in this toolset could remove them, because the manifest is the only
// authority over what may be deleted.
//
// A note rather than a refusal, by the owner's decision on the same day: the
// run itself works, and refusing it would take away something this tool does
// today. What was missing was that nobody was told. It is printed before the
// first byte and on a dry run, which is the step this project tells people to
// take before anything large.
func echoManifestReach(planned []engine.PlannedFile, errOut io.Writer) {
	noted := 0
	for _, f := range planned {
		if len(f.Plan.Notes) > 0 {
			noted++
		}
	}
	size, over := manifest.TooLargeToReadBack(len(planned), noted)
	if !over {
		return
	}
	fmt.Fprintf(errOut,
		"note: this run will write a manifest of roughly %s and this build reads at most %s, "+
			"so tfg verify and tfg cleanup will not be able to read it. Split the run to keep each manifest readable.\n",
		core.HumanBytes(size), core.HumanBytes(manifest.MaxBytes))
}

// saveManifest saves the record of a run - the manifest, and the instructions
// beside it when any file was given a purpose - and says where each went.
func saveManifest(res *engine.Result, opt engine.Options, errOut io.Writer) int {
	// Asked of the engine rather than joined here. The engine claimed this
	// exact name before the first file was written, so working it out a second
	// way is a chance for the saver and the claim to mean different files.
	//
	// The instructions are saved with it, by the same function the window
	// calls, so the two surfaces leave the same directory behind.
	rec, err := engine.SaveRecord(res, opt)
	path := rec.Manifest
	if err != nil {
		fmt.Fprintf(errOut, "tfg: cannot write the manifest to %s: %s\n", core.Shown(path), describeError(err))
		// What that leaves behind, because the line above is about the manifest
		// and the person's problem is the files. Rule 6: a run that wrote files
		// nothing can remove says so rather than leaving it to be discovered by
		// running cleanup and being told the manifest will not parse.
		//
		// Nothing that agrees with the number, on purpose - see core.Count. "3
		// files written" reads the same at one as at three.
		if n := len(res.Manifest.Files); n > 0 {
			fmt.Fprintf(errOut,
				"tfg: %s written and nothing to record what this run left. Cleanup works from a manifest, so clearing %s is a job by hand.\n",
				core.Count(n, "file", "files"), core.Shown(opt.OutDir))
		}
		return ExitIO
	}
	fmt.Fprintf(errOut, "manifest: %s\n", core.Shown(path))
	if rec.Instructions != "" {
		fmt.Fprintf(errOut, "instructions: %s\n", core.Shown(rec.Instructions))
	}
	// Said rather than failed. The manifest was saved, and it holds the same
	// facts the instructions would have put in words. Not "the files are
	// complete": this runs after a stopped or partly failed run as well, and
	// the line above it has already said how that went.
	if rec.Missed != nil {
		fmt.Fprintf(errOut,
			"tfg: cannot write the instructions to %s: %s. The manifest was saved and holds the same facts.\n",
			core.Shown(rec.Missed.Path), describeError(rec.Missed.Err))
	}
	return ExitOK
}
