package jpg

import (
	"fmt"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// What the encoder's stream passes through on its way to a file: a count
// while planning, and the start marker taken off while writing.

// counter counts bytes and keeps none.
type counter struct{ n int64 }

func (c *counter) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	return len(p), nil
}

// headSkipper drops the first keep bytes and passes the rest through, so the
// encoder's own start marker does not follow the padding we already wrote.
//
// It also holds on to what it dropped, because a stream that does not begin
// with the start marker means the encoder changed under us, and writing the
// rest of it after our own marker would produce a file nobody could read.
type headSkipper struct {
	w       io.Writer
	keep    int
	head    []byte
	written int64
}

func (h *headSkipper) Write(p []byte) (int, error) {
	n := len(p)
	if len(h.head) < h.keep {
		take := h.keep - len(h.head)
		if take > len(p) {
			take = len(p)
		}
		h.head = append(h.head, p[:take]...)
		p = p[take:]
	}
	if len(p) == 0 {
		return n, nil
	}
	written, err := h.w.Write(p)
	h.written += int64(written)
	return n, err
}

func (h *headSkipper) check() error {
	if len(h.head) != soiSize || h.head[0] != 0xFF || h.head[1] != 0xD8 {
		return core.Defect(fmt.Errorf("jpg: the encoder did not begin with the start of image marker, got % x", h.head))
	}
	return nil
}
