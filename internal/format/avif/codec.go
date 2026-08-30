// The codec: the single call that turns pixels into AV1.
package avif

import (
	"bytes"
	"fmt"
	"image"

	"github.com/gen2brain/gav1d/avif"
)

const (
	// encodeSpeed is the setting that searches least, and it is chosen on a
	// measurement rather than on taste. The picture a file of this format is
	// built around is a gradient with a label on it, and a generator asked for
	// ten thousand files should not spend its time deciding how to compress
	// one. Measured on 2026-08-29 at 320x240: about 6 ms here.
	encodeSpeed = 10

	minQuality     = 1
	maxQuality     = 100
	defaultQuality = 60
)

// encode runs the picture through the encoder and hands back the whole file it
// produced, container included.
//
// The bytes are kept rather than counted and thrown away, because planning has
// to encode anyway to learn the size, and encoding a second time while writing
// would double the cost of every file for nothing.
func encode(m image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	err := avif.Encode(&buf, m, avif.EncodeOptions{
		Quality:      quality,
		QualityAlpha: quality,
		Speed:        encodeSpeed,
	})
	if err != nil {
		return nil, fmt.Errorf("avif: the encoder could not code the picture: %w", err)
	}
	return buf.Bytes(), nil
}
