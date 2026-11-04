package cli

import (
	"flag"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// --memory, on every command that plans: generate, validate and preset show -
// and on verify and cleanup, which read a manifest under it.
//
// One function registers it on all three, so the flag cannot read one way on
// one command and another on the next, and core.ParseMemory reads it, so the
// flag and the box on the window's Preferences screen give the same answer to
// the same text - docs/STABILITY-REVIEW-2026-10-26.md section 10.8.
//
// The help is a fixed sentence with no number from this machine in it. The
// site prints the help of every command, and a number read off the machine
// that built the site would be a number about nobody's machine. The floor of
// the plan under auto is a number of the program rather than of the machine,
// so it is said, from the constant that sets it.

// memoryHelp is what --help says about the flag.
var memoryHelp = "how much memory this run may use: auto (half of what this machine gives the program), " +
	"a size such as 8gb, or off for no limit. Pictures wait for room rather than all being held at once, " +
	"and a plan larger than this is refused before anything is written. " +
	"When a run gets close to this amount, the program frees memory more often, which makes the run slower. " +
	"A run that needs more memory than this still gets it. " +
	"Under auto, a plan of up to " + core.HumanBytes(core.MaxPlanBytes) + " is never refused"

// memoryValue is the flag's value, auto until somebody writes one.
type memoryValue struct{ core.Memory }

func (m *memoryValue) String() string {
	if m == nil {
		return core.Memory{}.String()
	}
	return m.Memory.String()
}

func (m *memoryValue) Set(text string) error {
	parsed, err := core.ParseMemory(text)
	if err != nil {
		return err
	}
	m.Memory = parsed
	return nil
}

// memoryFlag puts --memory on a command, its value landing in v.
func memoryFlag(fs *flag.FlagSet, v *memoryValue) {
	fs.Var(v, "memory", memoryHelp)
}

// checkingMemoryHelp is what --help says about the flag on verify and cleanup,
// which hold what a manifest claims rather than a plan. The same three
// answers, read by the same parser, and a sentence about what they hold.
var checkingMemoryHelp = "how much memory checking may use: auto (half of what this machine gives the program), " +
	"a size such as 8gb, or off for no limit. A manifest listing more files than that can check is refused before any file is looked at. " +
	"When checking gets close to this amount, the program frees memory more often, which makes it slower"

// checkingMemoryFlag puts --memory on verify or cleanup, its value landing in v.
func checkingMemoryFlag(fs *flag.FlagSet, v *memoryValue) {
	fs.Var(v, "memory", checkingMemoryHelp)
}
