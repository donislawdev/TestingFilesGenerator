//go:build !windows

package guard

import (
	"context"
	"errors"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/tool"
)

// TestAToolNeverWaitsOnAPipe names a pipe as the file every tool reads.
// Opening one for reading waits until something writes to it, which may be
// never - so the open must not wait, and what was opened has to be refused
// before a byte of it is read. An open that waits hangs here until the
// deadline and says so, and one that reads the pipe hands back the checksum
// of nothing instead of the refusal.
//
// Since 2026-09-30 this is asked of the open file rather than of the name
// (tool.OpenRegular), which is what closed the moment between the two looks
// that a review of #157 found - so this one guard holds both halves.
//
// Every tool that reads a file, walked from the registry since 2026-10-19,
// when a second one arrived: a tool added tomorrow is held the day it is.
func TestAToolNeverWaitsOnAPipe(t *testing.T) {
	pipe := filepath.Join(t.TempDir(), "pipe")
	if err := syscall.Mkfifo(pipe, 0o600); err != nil {
		t.Fatalf("making a pipe to name: %v", err)
	}
	asked := 0
	for _, d := range tool.All() {
		if len(d.Inputs) == 0 || d.Inputs[0].Kind != tool.File {
			continue
		}
		asked++
		refusesThePipe(t, d, pipe)
	}
	// checksum, checksum-check and archive-scan read a file. Fewer is a walk
	// that stopped reaching the tools it is for.
	if asked < 3 {
		t.Fatalf("%d tool(s) reading a file were asked about a pipe, and at least three read one", asked)
	}
}

// refusesThePipe holds one tool to turning a pipe away without waiting on it.
func refusesThePipe(t *testing.T, d tool.Descriptor, pipe string) {
	t.Helper()
	answered := make(chan error, 1)
	go func() {
		_, err := d.Start(context.Background(), tool.Request{Inputs: map[string]string{d.Inputs[0].Name: pipe}}, nil)
		answered <- err
	}()
	select {
	case err := <-answered:
		var notAFile *tool.NotRegularError
		if !errors.As(err, &notAFile) {
			t.Errorf("a pipe was named to %s and the answer was %v, not the refusal saying it is not a file", d.ID, err)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("a pipe was named to %s and it is still waiting on it after ten seconds - it opened the pipe", d.ID)
	}
}
