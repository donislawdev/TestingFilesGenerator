// Package isobmff is what the formats built of ISO base media boxes share:
// the header every box starts with, and the free boxes their padding is
// carried in. AVIF and JPEG XL each held a copy of the same padding, and MP4
// would have been the third (docs/MP4-2026-10-07.md section 4).
package isobmff

import (
	"context"
	"encoding/binary"
	"io"
	"math/rand/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// BoxHeader is what every box costs before its content: a four byte length
// and a four character name.
const BoxHeader = 8

// maxFreePayload keeps a single box inside the four byte length field the
// container uses by default. Anything larger is spread over several boxes,
// which is measured to work rather than assumed.
const maxFreePayload = 1<<31 - BoxHeader

// WriteBoxHeader writes the header of a box of kind whose content is payload
// bytes long, in the four byte length every box here uses.
func WriteBoxHeader(w io.Writer, kind string, payload int64) error {
	var b [BoxHeader]byte
	binary.BigEndian.PutUint32(b[:4], uint32(BoxHeader+payload))
	copy(b[4:], kind)
	_, err := w.Write(b[:])
	return err
}

// WritePadding fills total bytes with free boxes, without ever holding their
// content - bytes drawn from seed, so the same file comes out every time.
// total is at least BoxHeader, because no box is shorter.
//
// Several boxes rather than one when the padding is larger than a box length
// can say. The step below keeps the leftover from landing between one and
// seven bytes, which no box could then carry.
func WritePadding(ctx context.Context, w io.Writer, seed uint64, total int64) error {
	rng := core.NewRand(seed)
	buf := make([]byte, 32*1024)

	for total > 0 {
		payload := nextPayload(total)
		if err := WriteBoxHeader(w, "free", payload); err != nil {
			return err
		}
		if err := writeFiller(ctx, w, buf, rng, payload); err != nil {
			return err
		}
		total -= BoxHeader + payload
	}
	return nil
}

// nextPayload is how much filler the next free box carries.
//
// A box states its length in four bytes, so padding larger than that is spread
// over several boxes. The step down is what keeps the leftover from landing
// between one and seven bytes, which no box could then carry.
func nextPayload(total int64) int64 {
	payload := total - BoxHeader
	if payload <= maxFreePayload {
		return payload
	}
	payload = maxFreePayload
	if total-BoxHeader-payload < BoxHeader {
		payload -= BoxHeader
	}
	return payload
}

func writeFiller(ctx context.Context, w io.Writer, buf []byte, rng *rand.Rand, size int64) error {
	for left := size; left > 0; {
		n := int64(len(buf))
		if left < n {
			n = left
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		core.FillRandomLE(buf[:n], rng)
		if _, err := w.Write(buf[:n]); err != nil {
			return err
		}
		left -= n
	}
	return nil
}
