// Package cli parses flags, keeps data and log on separate channels and maps
// every ending onto an exit code.
//
// The command line is not an advanced mode. It is the interface CI drives, so
// an ending CI cannot tell apart is a defect, not a detail.
package cli

import (
	"fmt"
	"io"
)

// Exit codes are a frozen contract. Changing what one means is a breaking
// change and needs a major version bump. See docs/CLI.md.
const (
	ExitOK      = 0
	ExitRuntime = 1
	ExitUsage   = 2
	ExitRecipe  = 3
	ExitFormat  = 4
	ExitIO      = 5
	ExitSpace   = 6
	ExitVerify  = 7
	ExitPartial = 8
)

// Run is the entry point of the command line. No commands exist yet.
//
// It writes to stderr on purpose. A failed run puts nothing on stdout, so a
// consumer of a pipe never receives half of an answer and has to guess
// whether that was all of it.
func Run(_ []string, _ io.Writer, errOut io.Writer) int {
	fmt.Fprintln(errOut, "tfg: no commands are implemented yet.")
	return ExitUsage
}
