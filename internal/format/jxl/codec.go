// The codec: the single call that turns pixels into a JPEG XL codestream.
package jxl

import (
	"bytes"
	"fmt"
	"image"

	"github.com/gen2brain/jxl"
)

const (
	// encodeEffort is how hard the encoder searches, and it is set to the
	// lowest setting on a measurement rather than on taste.
	//
	// The library's own default is 7, the highest it implements. For the
	// picture this generator builds - a gradient with a label on it - that
	// setting is both slower AND larger, which is the opposite of what the
	// knob is for. Measured on 2026-08-31, five interleaved rounds, ranges
	// that do not overlap: at 320x240 effort 1 takes 25 ms against 63 ms, at
	// 640x480 it takes 77 ms against 233 ms. Asked across sixteen seeds at
	// 320x240, effort 1 produced the smaller file in fourteen of them.
	//
	// Worth writing down because it does not generalise: at 80x60 effort 7 is
	// the smaller file, 240 B against 286 B. Search pays off on a picture with
	// few blocks to search. Ours are mostly not that.
	encodeEffort = 1

	// encodeThreads keeps one frame on one goroutine.
	//
	// The library's default is GOMAXPROCS, and the bytes were measured
	// identical at 1, 2, 3, 4, 8 and 16 threads, so this is not fixing a
	// determinism bug that exists. It is removing a machine dependent input
	// from a component whose output is the contract (D11), which is cheaper
	// than trusting that it will stay harmless.
	//
	// The price is measured and real, and it is paid at the top of the ladder
	// only: 640x480 takes 77 ms here against 43 ms with every core, while
	// 320x240 takes 25.5 ms against 22.3 ms. One frame per goroutine also
	// halves what the encode allocates.
	encodeThreads = 1

	minQuality = 1
	maxQuality = 100

	// defaultQuality is the library's own default, kept because it is the
	// setting the ladder ceilings below were measured at.
	defaultQuality = 90
)

// encode runs the picture through the encoder and hands back the codestream.
//
// What comes back is the bare codestream, starting FF 0A, not a container -
// measured, rather than taken from the encoder's documentation. The container
// this format writes is built in jxl.go, around these bytes.
func encode(m image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	err := jxl.Encode(&buf, m, jxl.EncodeOptions{
		Quality: quality,
		Effort:  encodeEffort,
		Threads: encodeThreads,
	})
	if err != nil {
		return nil, fmt.Errorf("jxl: the encoder could not code the picture: %w", err)
	}
	return buf.Bytes(), nil
}
