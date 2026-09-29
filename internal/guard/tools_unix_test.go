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
	"github.com/donislawdev/TestingFilesGenerator/internal/tool/checksum"
)

// TestAToolNeverOpensAPipe names a pipe as the file to work a checksum out of.
// Opening one for reading waits until something writes to it, which may be
// never - so the tool has to refuse from what the path IS, before it opens it.
// A tool that opened first hangs here until the deadline, and says so.
func TestAToolNeverOpensAPipe(t *testing.T) {
	pipe := filepath.Join(t.TempDir(), "pipe")
	if err := syscall.Mkfifo(pipe, 0o600); err != nil {
		t.Fatalf("making a pipe to name: %v", err)
	}
	d, err := tool.Get(checksum.ID)
	if err != nil {
		t.Fatal(err)
	}
	answered := make(chan error, 1)
	go func() {
		_, err := d.Start(context.Background(), tool.Request{Inputs: map[string]string{checksum.InputFile: pipe}}, nil)
		answered <- err
	}()
	select {
	case err := <-answered:
		var notAFile *checksum.NotAFileError
		if !errors.As(err, &notAFile) {
			t.Errorf("a pipe was named and the answer was %v, not the refusal saying it is not a file", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("a pipe was named and the tool is still waiting on it after ten seconds - it opened the pipe")
	}
}
