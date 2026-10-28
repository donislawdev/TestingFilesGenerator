package cli

import (
	"flag"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// --memory, on every command that plans: generate, validate and preset show.
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
