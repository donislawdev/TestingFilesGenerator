// Animation, which is the whole reason this format is in the set twice over.
//
// A GIF is the only picture format here that can carry more than one frame, so
// a still one cannot tell a tester whether the system under test keeps an
// animation, flattens it to the first frame, or re-encodes it. What moves is a
// single bright square, redrawn on its own each frame and disposed with
// "restore to previous" so it leaves no trail.
//
// Split out of gif.go on 2026-08-29 because that file reached 478 lines and
// this project counts how many files crowd the size ceiling. The counter is
// meant to fall.
package gif

import (
	"fmt"
	"image"
	stdgif "image/gif"
	"io"
	"strconv"
)

// frameCount reads the frames setting, which is the one thing about a GIF that
// no other image format here has to answer.
func frameCount(props map[string]string) (int, error) {
	raw, ok := props["frames"]
	if !ok || raw == "" {
		return defaultFrames, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("gif: frames must be a whole number, got %q", raw)
	}
	if n < minFrames || n > maxFrames {
		return 0, fmt.Errorf("gif: frames must be between %d and %d, got %d", minFrames, maxFrames, n)
	}
	return n, nil
}

// marker is where the travelling square sits on frame i, and how big it is.
//
// It rides at three quarters of the height rather than the middle, because the
// label is burned into a band across the top and the two would collide on a
// short picture.
func marker(width, height, frames, i int) (x, y, side int) {
	side = width / markerDivisor
	if side > maxMarkerSide {
		side = maxMarkerSide
	}
	if side < 1 {
		side = 1
	}
	if side > height {
		side = height
	}
	x = (width - side) * i / frames
	y = (height - side) * 3 / 4
	return x, y, side
}

// markerFrame is one step of the animation: the square alone, at its own
// offset, and nothing else. Keeping it to the square is what stops the frames
// costing as much as the picture, in bytes and in memory both.
func markerFrame(m memo, i int) *image.Paletted {
	x, y, side := marker(m.width, m.height, m.frames, i)
	img := image.NewPaletted(image.Rect(x, y, x+side, y+side), buildPalette(paletteSize(m.width, m.height)))
	for j := range img.Pix {
		img.Pix[j] = labelInk
	}
	return img
}

func encode(w io.Writer, m memo) error {
	if m.frames <= 1 {
		// The plain encoder, byte for byte what this package wrote before it
		// could animate. Reached by asking for frames: 1.
		return stdgif.Encode(w, picture(m), &stdgif.Options{NumColors: 256})
	}

	base := picture(m)
	g := &stdgif.GIF{
		LoopCount: 0,
		// Without this the encoder writes no global colour table and gives
		// every frame its own. Measured: a 12 px square then cost 233 B
		// instead of 41 B, because 192 B of it was a second copy of the
		// palette.
		Config: image.Config{ColorModel: base.Palette, Width: m.width, Height: m.height},
		Image:  make([]*image.Paletted, 0, m.frames),
	}
	g.Image = append(g.Image, base)
	g.Delay = append(g.Delay, frameDelay)
	g.Disposal = append(g.Disposal, stdgif.DisposalNone)
	for i := 1; i < m.frames; i++ {
		g.Image = append(g.Image, markerFrame(m, i))
		g.Delay = append(g.Delay, frameDelay)
		g.Disposal = append(g.Disposal, stdgif.DisposalPrevious)
	}
	return stdgif.EncodeAll(w, g)
}
