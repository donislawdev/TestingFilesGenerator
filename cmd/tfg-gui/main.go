// Command tfg-gui is the desktop interface.
//
// It is a separate binary because the graphics toolkit needs CGO and a C
// compiler, so it is built natively on each system rather than cross
// compiled. The command line does not depend on it.
package main

import (
	"os"

	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui"
)

func main() {
	// This process is the window's own, so the memory setting holds each
	// whole run and not only its plan - engine/processmemory.go says why only
	// the two main functions say so.
	engine.OwnProcess()
	os.Exit(gui.Run(os.Args[1:], os.Stderr))
}
