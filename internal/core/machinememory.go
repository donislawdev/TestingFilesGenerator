package core

import "math"

// How much memory this machine gives this program, asked of the system once
// per run - MachineMemory, one file per system beside this one.
//
// The number is the setting auto halves (Memory.On), so it has to be the
// memory the process can really have rather than the memory in the box. On a
// laptop those are the same. In a container started with --memory 2g, on a CI
// runner, or in a job with a commit limit, the box has far more than the
// process may take, and a run that plans from the box is the one the system
// ends without a word - which is the failure the setting exists for. So every
// system's answer is the smallest of the numbers it offers: what is fitted,
// what the process is allowed to commit, and what any control group it is in
// holds it to.
//
// Only the standard library asks. golang.org/x/sys would offer these calls
// ready made, and it is not linked into the command line binary - putting it
// there is untouchable rule 11 rather than tidying, the same reason given in
// diskspace_windows.go.

// smallest is the smallest of a machine's answers, as a count of bytes. None
// of them is ever above what an int64 holds on a real machine, and one that
// was is held to the largest rather than wrapped into a negative.
func smallest(answers ...uint64) int64 {
	least := uint64(math.MaxInt64)
	for _, a := range answers {
		least = min(least, a)
	}
	return int64(least)
}
