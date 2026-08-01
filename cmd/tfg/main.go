// Command tfg generates test files and tells you how the system under test
// should react to them.
//
// This binary builds without CGO on every supported system. It must never
// import the gui package - that would pull in the graphics toolkit and end
// cross compilation.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

func main() {
	// Ctrl+C and a CI timeout have to reach the generator, not only kill the
	// process. Without this the run dies mid file, leaving a partial file
	// behind and no manifest - so cleanup has nothing to work with and the
	// leftovers stay for good.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	os.Exit(cli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
