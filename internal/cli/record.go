// Part of package cli. See cli.go.
package cli

// What a run leaves about itself once the files are written: the manifest and
// the instructions beside it, what is said when a run stopped, and the
// manifest --json sends. Out of generate.go on 2026-09-25, when the
// instructions made that file the largest in the tree.

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
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

// echoHold says so when a run will hold more than its memory setting gives it
// (O296, engine.PredictHold says why and what was measured).
//
// Before the first byte, and that is the point of it: a run the system ends
// prints nothing afterwards, so the sentence has to be in the log already. A
// note rather than a refusal, because the same run finishes where there is swap
// to carry it. And on a dry run, which is the step this project tells people
// to take before anything large.
func echoHold(planned []engine.PlannedFile, opt engine.Options, errOut io.Writer) {
	hold := engine.PredictHold(len(planned), opt)
	if !hold.Over {
		return
	}
	fmt.Fprintf(errOut,
		"note: this run will hold about %s in memory by its end, more than the %s the memory setting gives it, "+
			"so it will run slower and the system may end it before the manifest is written. "+
			"Split the run, or give it more with --memory if this machine has the room.\n",
		core.HumanBytes(hold.Holds), core.HumanBytes(hold.Room))
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

// sendManifest puts the manifest on standard output for --json, a part at a
// time as Encode writes it.
//
// It used to be encoded whole into memory a second time, beside every entry
// the run still held, and handed over in one write - for a run whose manifest
// was already on the disk (observation O300,
// docs/MANIFEST-STREAMING-2026-11-04.md section 9). Encoded again rather than
// copied from the disk, because Encode streams and writes the same bytes the
// saved file holds, and a copy would be a second way for the two to differ -
// a file somebody replaced in between would be sent as this run's.
func sendManifest(res *engine.Result, dryRun bool, out, errOut io.Writer) int {
	sent := &watched{w: out}
	err := res.Manifest.Encode(sent)
	switch {
	case err == nil:
		return ExitOK
	case sent.err == nil:
		fmt.Fprintf(errOut, "tfg: cannot render the manifest: %s\n", describeError(err))
		return ExitRuntime
	case dryRun:
		fmt.Fprintf(errOut, "tfg: the manifest could not be sent to standard output: %s\n", describeError(err))
		return ExitIO
	}
	// The error is read rather than dropped. A closed pipe is the ordinary
	// way this fails - "tfg generate --json | head" - and a run that could
	// not deliver what was asked for has not succeeded, whatever happened
	// on the disk.
	fmt.Fprintf(errOut, "tfg: the files were written and the manifest could not be sent to standard output: %s\n", describeError(err))
	return ExitIO
}

// watched passes writes on and keeps the first that failed, so a failure to
// write can be told from a failure to render.
type watched struct {
	w   io.Writer
	err error
}

func (s *watched) Write(p []byte) (int, error) {
	n, err := s.w.Write(p)
	if err != nil && s.err == nil {
		s.err = err
	}
	return n, err
}
