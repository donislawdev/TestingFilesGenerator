package guard

import (
	"context"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format/isobmff"
)

// boxCounter reads a run of boxes as it is written, keeping only their
// headers: how many there are, the shortest, and whether the run ended at a
// box's end. Gigabytes go through it without being held.
type boxCounter struct {
	header    [isobmff.BoxHeader]byte
	have      int    // header bytes seen of the box being read
	left      uint64 // content still to come of the box being read
	boxes     int
	shortest  uint64
	notFree   string
	collected int64
}

func (c *boxCounter) Write(p []byte) (int, error) {
	n := len(p)
	c.collected += int64(n)
	for len(p) > 0 {
		if c.left > 0 {
			step := min(uint64(len(p)), c.left)
			c.left -= step
			p = p[step:]
			continue
		}
		k := copy(c.header[c.have:], p)
		c.have += k
		p = p[k:]
		if c.have < isobmff.BoxHeader {
			continue
		}
		size := uint64(binary.BigEndian.Uint32(c.header[:4]))
		if string(c.header[4:]) != "free" {
			c.notFree = string(c.header[4:])
		}
		if size < isobmff.BoxHeader {
			return 0, fmt.Errorf("a box says it is %d B, shorter than its header", size)
		}
		c.boxes++
		if c.shortest == 0 || size < c.shortest {
			c.shortest = size
		}
		c.left, c.have = size-isobmff.BoxHeader, 0
	}
	return n, nil
}

// Padding of any length at all is carried in free boxes, past what one box's
// four byte length can say.
//
// AVIF, JPEG XL and MP4 pad with free boxes (internal/format/isobmff), and a
// box states its length in four bytes, so padding past two gigabytes is
// several boxes. The split is where it can go wrong without a word: a leftover
// of one to seven bytes is no box at all. The lengths are the ones around the
// split - one box exactly full, then one byte, seven and eight past it - each
// read back box by box as it is written, every byte of it.
func TestFreeBoxesCarryPaddingOfAnyLength(t *testing.T) {
	// The largest one box carries: a length of two gigabytes, header and all.
	full := int64(1 << 31)
	cases := []struct {
		total int64
		boxes int
	}{
		{isobmff.BoxHeader, 1},
		{full, 1},
		{full + 1, 2},
		{full + 7, 2},
		{full + 8, 2},
	}
	for _, c := range cases {
		var got boxCounter
		if err := isobmff.WritePadding(context.Background(), &got, 7, c.total); err != nil {
			t.Fatalf("%d B of padding: %v", c.total, err)
		}
		if got.collected != c.total || got.left != 0 || got.have != 0 {
			t.Errorf("%d B of padding came out as %d B, ending %d B into a box", c.total, got.collected, int(got.left)+got.have)
		}
		if got.boxes != c.boxes || got.notFree != "" {
			t.Errorf("%d B of padding is %d boxes (a %q among them), and it takes %d free boxes", c.total, got.boxes, got.notFree, c.boxes)
		}
		if got.shortest < isobmff.BoxHeader {
			t.Errorf("%d B of padding has a box of %d B", c.total, got.shortest)
		}
	}
}
