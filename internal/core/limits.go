package core

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

// The numbers a person can write that this build will not honour.
//
// They live here rather than beside one caller because both surfaces ask the
// same question. The recipe reader and the --count flag each turn a number into
// a list, and a ceiling enforced on one side only is a ceiling somebody reaches
// through the other - the same argument that put ParseSize in this package.

// MaxFilesPerRun is the largest number of files this build will plan.
//
// The plan is held in memory before anything is written. That is the price of
// AR2 and it buys the promise that an impossible run is refused before the
// first byte reaches the disk. The price is linear and it was measured on
// 2026-08-03: 27 MB at ten thousand files, 99 MB at fifty thousand, 481 MB at
// two hundred thousand - about 2.4 kB a file, straight all the way. A million
// files is therefore roughly 2.4 GB.
//
// Without a ceiling the number came from whoever wrote the count, and two
// different crashes came with it. "--count 9223372036854775807" reached
// make([]int64) and panicked with a Go stack trace under exit code 2, which the
// frozen table says means a mistyped flag. The same count in a recipe grew the
// list one entry at a time and reached a 13 GB allocation before the operating
// system refused it. Both found by audit on 2026-08-03.
//
// A million is a hundred times the largest preset in docs/PRESETS.md, which
// asks for 10 040 files. High enough that no real request meets it, low enough
// that meeting it costs a refusal rather than the machine. Lowering it is the
// owner's call and costs one edit - the number is written once on purpose.
const MaxFilesPerRun = 1_000_000

// ErrTooManyFiles is a request for more files than MaxFilesPerRun.
//
// It carries the four parts every refusal in this tool carries, so both callers
// report it in the same words rather than each phrasing it again.
var ErrTooManyFiles = errors.New(TooManyFilesWhy + ". " + TooManyFilesFix)

// The same refusal in the two parts a report keeps apart. ErrTooManyFiles is
// built from them rather than beside them, so the sentence and the parts cannot
// come to disagree - the compiler is the proof.
var (
	// Built from the constant rather than repeating it. The comment on
	// MaxFilesPerRun says the number is written once on purpose, and until
	// 2026-08-25 it was written twice - so lowering the ceiling would have left
	// the sentence saying the old one, which is a refusal that lies about its
	// own rule. A variable rather than a constant because Sprintf is not a
	// constant expression, and that is the whole cost.
	TooManyFilesWhy = fmt.Sprintf(
		"this build plans at most %d files in one run, because the whole plan is worked out in memory before anything is written - "+
			"that is what lets a run that cannot succeed be refused before the first byte", MaxFilesPerRun)
	TooManyFilesFix = "Ask for fewer files, or split the work into several runs"

	// The same refusal for the quantity that actually runs out.
	//
	// MaxFilesPerRun counts files and was justified by a measurement taken on
	// txt. Re-measured on 2026-08-26 across formats, a planned file costs 850 B
	// for txt, 6153 B for zip, 7527 B for pdf and 5244230 B for a pdf of a
	// thousand pages - so the file ceiling lets a run ask for about 52 GB of
	// plan while every number in it is legal. The sentence says memory rather
	// than files because that is what the person has to change.
	PlanTooLargeWhy = fmt.Sprintf(
		"the whole plan is held in memory before anything is written, and this build works to a ceiling of %s for it - "+
			"how much a file costs to plan depends on the format, so a ceiling on the number of files alone cannot see this",
		HumanBytes(MaxPlanBytes))
	PlanTooLargeFix = "Ask for fewer files, or make each one cheaper to plan - fewer pages, fewer entries - or split the work into several runs"
)

// MaxPlanBytes is the most memory this build will let a plan take.
//
// Two gigabytes, which is the figure the file ceiling beside it was always
// meant to imply: its comment works out "a million files is therefore roughly
// 2.4 GB" and treats that as the real constraint. This makes the real
// constraint the one that is checked, so the two cannot disagree again.
//
// It is deliberately generous. Every run this tool was designed around is
// orders of magnitude under it - the largest preset asks for 10 040 files,
// which is about 9 MB of plan for txt and 76 MB for pdf - and the shapes it
// refuses are the ones that would otherwise end as an out of memory kill with
// no message at all.
const MaxPlanBytes = 2 << 30

// PartialMarker is what a file being written is called before it is finished.
//
// Every file goes out under a temporary name and is renamed into place, so the
// output directory never holds a half written file. The full name is
// "<final>.tfg-partial-<process id>", with the process id there because two
// runs writing into one directory used to meet on the temporary file.
//
// The marker is declared here rather than built at the point of use, because
// two parts of the tool have to agree on it: the engine writes it, and the
// reading side has to recognise one that outlived its run. A second spelling
// would mean verify reports our own leftovers as files it knows nothing about,
// which is what it did until 2026-08-03.
const PartialMarker = ".tfg-partial-"

// IsPartialName says whether a file name is one of ours, left behind by a run
// that did not get to finish.
//
// It matters because such a file cannot be removed by cleanup - untouchable
// rule 7 makes the manifest the whole authority over what may be deleted, and a
// file that was never finished never reached it. So it sits in the directory,
// and the only thing that can help is saying clearly what it is.
//
// Measured on 2026-08-03 with tools/probes/hard-kill-probe.py: three runs
// killed with taskkill /F left one of these every time, and verify reported it
// as "extra" - a word that tells a reader nothing about a file this tool wrote
// itself.
func IsPartialName(name string) bool {
	return strings.Contains(name, PartialMarker)
}

// WritingMarker is what a file being REPLACED is called while it is filled.
//
// A different job from PartialMarker, and the two are not interchangeable. That
// one marks a file this run is producing, under a name nobody held before. This
// one marks the half written copy of a file that already exists and belongs to
// somebody - the manifest being rewritten over an earlier one, or the recipe
// that "recipe fmt -w" is formatting in place. The full name is
// "<final>.tfg-writing", with no process id, because the name is claimed
// exclusively rather than made unique.
//
// Declared here beside PartialMarker on 2026-09-06, and the argument for it is
// the one already written above: two parts of the tool have to agree on the
// spelling, the writing side and the reading side. Until that day this marker
// had TWO spellings and no reader at all - an unexported constant in
// core/replace.go and a bare literal in manifest.go - so verify reported our
// own half written manifest as "extra", the word that means somebody else put
// it here. That is exactly the failure the comment on PartialMarker describes
// as fixed in 2026-08-03, arriving a second time through the other marker.
const WritingMarker = ".tfg-writing"

// IsWritingName says whether a file name is one of ours, left behind by a
// replacement that did not finish.
//
// A suffix rather than a contained string, which is the difference from
// IsPartialName: that one has a process id after it, this one ends the name.
//
// Reaching one of these needs the process to die between the create and the
// rename, because every error path in the writers removes it. That is a hard
// kill, a CI timeout that outruns the grace period, or power loss - the same
// conditions PartialMarker exists for.
func IsWritingName(name string) bool {
	return strings.HasSuffix(name, WritingMarker)
}

// RunLockName is the name a run holds for as long as it is writing into a
// directory.
//
// The third of these, and the only one that is a whole name rather than a
// suffix on somebody else's: it belongs to the run, not to a file. A run takes
// it exclusively before the first byte and gives it back when it ends, so a
// second run starting into the same directory is refused rather than allowed
// to write over what the first one is producing.
//
// Why it had to exist, measured on 2026-09-07 with two runs started on the
// same wall clock instant, eight times: twice both runs ended 0, each said it
// had produced sixty files, and sixty files were on the disk - every one of
// the first run's belonging to the second. Five times the runs ended 8, and
// that was luck rather than a defence: Windows refuses to rename onto a file
// another process holds open, which is not something Linux does.
//
// The protection this restores already existed and was keyed to the wrong
// thing. A run claims its manifest name for its whole length, so two runs
// writing manifest.json into one directory have always been refused - measured
// the same day, four times out of four. output.manifest was the one way out of
// that, and it was never meant to be a way out of this.
//
// Declared here beside the other two for the reason written above them: two
// parts of the tool have to agree on the spelling. The engine writes it, and
// verify has to recognise one that outlived its run rather than call it a file
// somebody else put there.
const RunLockName = ".tfg-run-lock"

// IsRunLockName says whether a name is that lock.
//
// A whole name rather than a suffix, so this is equality rather than a search.
// Written as a function anyway, because every reader of the other two markers
// asks through one and a reader that compares the constant itself is a reader
// that will not be found when the spelling changes.
func IsRunLockName(name string) bool {
	return name == RunLockName
}

// AddSizes adds one file size to a running total and says when the total has
// left the range it is measured in.
//
// A total that wraps is worse than one that is refused. The free space guard
// asks whether the disk holds the total, a negative total is smaller than any
// disk, and the run starts writing. Measured on 2026-08-03: two files of 2^62
// were reported as "-9223372036854775808 B total" and the dry run ended with
// code 0, which is the guard being satisfied rather than skipped.
func AddSizes(total, size int64) (int64, error) {
	if size < 0 {
		return 0, errors.New("a file cannot be smaller than zero bytes")
	}
	if total > math.MaxInt64-size {
		return 0, errors.New(
			"the sizes in this run add up to more than a number of bytes can hold, so the total cannot be measured and the free space check cannot be trusted. " +
				"Ask for fewer files or smaller ones")
	}
	return total + size, nil
}
