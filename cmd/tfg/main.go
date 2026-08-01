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
	"sync/atomic"
	"syscall"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

func main() {
	// Ctrl+C and a CI timeout have to reach the generator, not only kill the
	// process. Without this the run dies mid file, leaving a partial file
	// behind and no manifest - so cleanup has nothing to work with and the
	// leftovers stay for good.
	//
	// The signal is watched through a channel rather than through
	// signal.NotifyContext, because that does not say which signal arrived and
	// the exit code table tells them apart. Every stop used to be reported as
	// Ctrl+C, so a CI job that timed out looked like somebody had cancelled it.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	// Stored before the cancel that lets the run finish, so the value is
	// settled by the time the code below reads it.
	var ending atomic.Int64
	go func() {
		s, ok := <-signals
		if !ok {
			return
		}
		ending.Store(int64(cli.ExitForSignal(s)))
		cancel()
	}()

	code := cli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr)

	// Only an ending the run itself reported as interrupted is rewritten. A
	// run that failed for its own reason keeps its own code.
	if code == cli.ExitInterrupted {
		if fromSignal := ending.Load(); fromSignal != 0 {
			code = int(fromSignal)
		}
	}
	os.Exit(code)
}
