// Package gui holds the desktop window.
//
// Nothing outside this package imports the graphics toolkit. Otherwise the
// command line stops building where there is no graphics environment, which
// is most of CI. The toolkit is not wired in yet.
package gui

import (
	"fmt"
	"io"
)

// Run opens the window. Nothing is implemented yet.
func Run(errOut io.Writer) int {
	fmt.Fprintln(errOut, "tfg-gui: the interface is not implemented yet.")
	return 2
}
