// The pixels and the colour table.
//
// Split out of gif.go on 2026-08-29, when animation pushed that file over the
// line count this project crowds against. The three files are three jobs: the
// container and the size arithmetic, this one, and the frames.
package gif

import (
	"image"
	"image/color"

	"github.com/donislawdev/TestingFilesGenerator/internal/format/imagelabel"
)

// paletteSize is how many entries the colour table gets.
//
// It follows the picture rather than being fixed at 256, and the reason is the
// smallest file this format can produce. A GIF writes its colour table in
// full, three bytes an entry, so a fixed 256 entry table puts 768 bytes into a
// one pixel picture and pushes the minimum from about fifty bytes to eight
// hundred. A generator for testing has to be able to make small files.
//
// The table is a power of two because the format says so. Two slots are always
// reserved for the label, and the gradient takes what is left.
func paletteSize(width, height int) int {
	// The gradient walks x+y, so a picture cannot show more distinct shades
	// than the length of that diagonal.
	distinct := width + height - 1
	if distinct > maxGradient {
		distinct = maxGradient
	}
	need := reservedSlots + distinct
	// Four is the floor: two label slots plus at least two shades.
	size := 4
	for size < need {
		size *= 2
	}
	if size > 256 {
		size = 256
	}
	return size
}

// palettes are built once, when the package loads, and handed out from there.
//
// A color.Palette is a slice of interfaces, so every entry put into one boxes
// a colour onto the heap - 256 objects for a full table, every time a picture
// was built. Measured: 300 allocations to write one file against a ceiling of
// 128. Building them up front moves that cost out of the write entirely, and
// there are only seven possible tables because the size is a power of two.
var palettes = func() map[int]color.Palette {
	out := map[int]color.Palette{}
	for size := 4; size <= 256; size *= 2 {
		out[size] = makePalette(size)
	}
	return out
}()

// palette holds the two label colours in fixed slots so the rasteriser lands
// on them exactly rather than on whatever happens to be nearest, and fills the
// rest with a spread the gradient indexes straight into. No quantiser runs,
// which is what keeps the encoding cheap and the bytes the same everywhere.
func buildPalette(size int) color.Palette {
	if p, ok := palettes[size]; ok {
		return p
	}
	return makePalette(size)
}

func makePalette(size int) color.Palette {
	p := make(color.Palette, size)
	p[labelBackground] = color.RGBA{R: 16, G: 16, B: 16, A: 255}
	p[labelInk] = color.RGBA{R: 240, G: 240, B: 240, A: 255}
	shades := size - reservedSlots
	for i := reservedSlots; i < size; i++ {
		// A smooth ramp across whatever room the table has, so the picture
		// reads as a gradient the way the other image formats do. An earlier
		// version multiplied the index by odd numbers and wrapped, which
		// spread the colours nicely and looked like interference on screen -
		// and cost size as well, because neighbouring pixels that share
		// nothing are what LZW is worst at.
		t := 0
		if shades > 1 {
			t = (i - reservedSlots) * 255 / (shades - 1)
		}
		blue := 2 * t
		if t > 127 {
			blue = 2 * (255 - t)
		}
		p[i] = color.RGBA{R: uint8(t), G: uint8(255 - t), B: uint8(blue), A: 255}
	}
	return p
}

func picture(m memo) *image.Paletted {
	size := paletteSize(m.width, m.height)
	shades := size - reservedSlots
	if shades < 1 {
		shades = 1
	}
	img := image.NewPaletted(image.Rect(0, 0, m.width, m.height), buildPalette(size))
	off := int(m.seed % 256)
	for y := 0; y < m.height; y++ {
		row := img.Pix[y*img.Stride : y*img.Stride+m.width]
		for x := 0; x < m.width; x++ {
			row[x] = byte(reservedSlots + (x+y+off)%shades)
		}
	}
	if m.label != "" && imagelabel.Fits(m.width, len(m.label)) {
		imagelabel.Draw(img, m.label)
	}
	return img
}
