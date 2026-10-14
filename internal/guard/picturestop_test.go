package guard

import (
	"context"
	"errors"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	pic "github.com/donislawdev/TestingFilesGenerator/internal/format/picture"
)

// A picture format stops writing soon after the run it belongs to is stopped.
//
// Ctrl+C reaches a generator only through its context, and a generator that
// notices it and writes on regardless keeps a stopped run going: webp did,
// coding a picture to its last row after the stop, 5.2 s for 600 MB (O279,
// measured 2026-10-14 through a page carrying the picture). A picture inside a
// page or a mail message is written by the same generator, so this holds them
// too.
//
// Counted in bytes rather than timed, so a slow machine cannot turn it red and
// a fast one cannot hide the defect: the writer below stops the run once the
// first MiB has reached it, and what comes after has to be a small part of the
// file rather than all of it.

// stoppingWriter cancels a run once a number of bytes have reached it, and
// keeps counting what still arrives.
type stoppingWriter struct {
	cancel context.CancelFunc
	after  int64
	n      int64
}

func (s *stoppingWriter) Write(p []byte) (int, error) {
	s.n += int64(len(p))
	if s.n >= s.after {
		s.cancel()
	}
	return len(p), nil
}

func TestEveryPictureFormatStopsSoonAfterARunIsStopped(t *testing.T) {
	const size, stopAt = int64(64 << 20), int64(1 << 20)
	formats := pic.Formats()
	if len(formats) == 0 {
		t.Fatal("this build has no picture format, so nothing below is asked")
	}
	for _, id := range formats {
		d, err := format.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		p, err := d.Generator.Plan(format.Request{Bytes: size, Seed: 7741, Label: true})
		if err != nil {
			t.Errorf("%s: %d B cannot be planned, so its stop is not asked: %v", id, size, err)
			continue
		}
		ctx, cancel := context.WithCancel(context.Background())
		w := &stoppingWriter{cancel: cancel, after: stopAt}
		err = d.Generator.Write(ctx, w, p)
		cancel()
		if !errors.Is(err, context.Canceled) {
			t.Errorf("%s: stopped after 1 MiB of %d B and the write ended with %v, not the stop", id, size, err)
		}
		if w.n > size/4 {
			t.Errorf("%s: stopped after 1 MiB and wrote %d B of %d B before it noticed", id, w.n, size)
		}
	}
}
