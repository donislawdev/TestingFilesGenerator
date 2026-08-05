//go:build !cgo

package gui

import (
	"fmt"
	"io"
)

// run answers a build that cannot open a window at all.
//
// The graphics toolkit draws through OpenGL and reaches it through C, so a
// binary built with CGO disabled has no window in it - the code is not merely
// inactive, it was never compiled. Saying so beats a build that starts and
// then fails somewhere less obvious.
//
// Four parts, D6: what cannot be done, why, what does work instead, and what
// to do about it. RUNTIME rather than USAGE because nothing the caller typed
// is wrong - the same reasoning docs/GUI.md section 5 applies to the stub this
// replaces.
func run(errOut io.Writer) int {
	fmt.Fprintln(errOut,
		"tfg-gui: this build has no window in it. It was compiled without C support, "+
			"which the graphics toolkit needs to reach OpenGL. Every feature is available "+
			"from the command line, which needs neither - run \"tfg --help\". "+
			"To get a window, use a tfg-gui built for your system rather than this one.")
	return 1
}
