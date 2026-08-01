// Command tfg generates test files and tells you how the system under test
// should react to them.
//
// This binary builds without CGO on every supported system. It must never
// import the gui package - that would pull in the graphics toolkit and end
// cross compilation.
package main

import (
	"os"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
