// Command tfg-gui is the desktop interface.
//
// It is a separate binary because the graphics toolkit needs CGO and a C
// compiler, so it is built natively on each system rather than cross
// compiled. The command line does not depend on it.
package main

import (
	"os"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui"
)

func main() {
	os.Exit(gui.Run(os.Stderr))
}
