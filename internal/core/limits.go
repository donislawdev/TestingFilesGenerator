package core

import (
	"errors"
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
var ErrTooManyFiles = errors.New(
	"this build plans at most 1000000 files in one run, because the whole plan is worked out in memory before anything is written - " +
		"that is what lets a run that cannot succeed be refused before the first byte. " +
		"Ask for fewer files, or split the work into several runs")

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
