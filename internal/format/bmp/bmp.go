// Package bmp generates BMP images.
//
// The first uncompressed image format, and that changes what the generator
// has to do. In PNG the encoder decides the size and a padding chunk makes up
// whatever difference is left. Here the size follows from the dimensions by
// plain arithmetic, so the picture itself can be made to fill the request and
// the padding only settles the last few bytes of a row.
//
// That is not a detail of taste. A tester who opens a 10 MB BMP expects to
// see a picture worth 10 MB, not a thumbnail followed by ten megabytes of
// filler nobody can see.
package bmp

import (
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"io"
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/imagelabel"
)

const (
	generatorVersion = "1"

	// fileHeader is BITMAPFILEHEADER: the two signature bytes, the file size,
	// two reserved fields and the offset where the pixels start.
	fileHeader = 14
	// infoHeader is BITMAPINFOHEADER, the 40 byte variant every reader
	// understands. Later variants exist and none of them buys us anything.
	infoHeader = 40
	// headers is what stands before the padding gap.
	headers = fileHeader + infoHeader

	// bitsPerPixel is 24, three bytes per pixel with no palette and no alpha.
	// The most widely readable shape a BMP has.
	bitsPerPixel = 24

	minDimension = 1
	maxDimension = 20000

	// A BMP declares its own size in a four byte unsigned field, so the file
	// cannot pass 4 GiB whatever we do. Reasoned from the format, not
	// measured - a file that size will not fit on the machine this was
	// written on.
	maxFileBytes = 1<<32 - 1

	// The tallest label band the rasteriser can produce, so the strip that
	// carries the label is built once at a known height and the rest of the
	// picture streams past without ever being held.
	maxBandHeight = 24
)

func init() {
	format.Register(format.Descriptor{
		ID:          "bmp",
		Extension:   ".bmp",
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,

		// The smallest BMP this generator can produce: one pixel, whose row
		// is rounded up to four bytes.
		MinBytes: headers + int64(stride(1)),

		Padding: format.PaddingChannel{
			// Measured against five independent readers - Pillow, the Windows
			// Imaging Component, GDI+, exiftool and ffprobe - at every size
			// from one byte to 100 MiB, odd sizes included. All five read the
			// image and return identical pixels.
			//
			// This is the one place the format itself sets aside: bfOffBits
			// says where the pixels begin, so anything between the header and
			// that offset is space the file describes rather than space it
			// keeps quiet about. Bytes after the pixel data are also accepted
			// by all five, but no field mentions them.
			Name:     "the gap between the header and the pixel data",
			Where:    format.PlacementInside,
			Capacity: maxFileBytes,
		},
		Label:  format.LabelVisible,
		Oracle: "pillow",
		Properties: []format.Property{
			{
				Name: "width", Kind: format.PropertyInt,
				Min: minDimension, Max: maxDimension, Unit: "pixels",
				Detail: "How wide the picture is. Left out, the picture is sized to fill the bytes you asked for.",
			},
			{
				Name: "height", Kind: format.PropertyInt,
				Min: minDimension, Max: maxDimension, Unit: "pixels",
				Detail: "How tall the picture is. Left out, the picture is sized to fill the bytes you asked for.",
			},
		},
		GeneratorVersion: generatorVersion,
		Generator:        generator{},
	})
}

type generator struct{}

type memo struct {
	width, height int
	seed          uint64
	label         string
	// gap is how many bytes sit between the header and the pixel data.
	gap int64
}

// stride is the length of one row on disk. Every row is rounded up to a
// multiple of four bytes, which is why a one pixel picture costs four and not
// three.
func stride(width int) int {
	return (width*3 + 3) / 4 * 4
}

func pixelBytes(width, height int) int64 {
	return int64(stride(width)) * int64(height)
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	label := ""
	if r.Label {
		label = core.Label("bmp", r.Bytes, r.Seed)
	}

	w, h, err := chooseSize(r)
	if err != nil {
		return format.Plan{}, err
	}

	bare := headers + pixelBytes(w, h)
	if r.Bytes < bare {
		return format.Plan{}, &format.BelowMinimumError{
			Format:    "BMP",
			Requested: r.Bytes,
			Minimum:   bare,
			Reason: fmt.Sprintf(
				"a %dx%d picture is %d B of pixels once each row is rounded up to four bytes, and the %d B header sits in front of it",
				w, h, pixelBytes(w, h), headers),
			Hint: fmt.Sprintf("Ask for %d B or more, or set a smaller width and height", bare),
		}
	}
	if r.Bytes > maxFileBytes {
		return format.Plan{}, &format.BelowMinimumError{
			Format:    "BMP",
			Requested: r.Bytes,
			Minimum:   maxFileBytes,
			Reason:    "a BMP states its own size in a four byte field, so the format cannot describe a file this large",
			Hint:      "Ask for 4 GiB or less, or pick a format with no size field of its own such as gif.",
		}
	}

	m := memo{width: w, height: h, seed: r.Seed, label: label, gap: r.Bytes - bare}

	p := format.Plan{
		Bytes:       r.Bytes,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties: map[string]any{
			"width":       w,
			"height":      h,
			"bit_depth":   bitsPerPixel,
			"compression": "none",
			"row_order":   "bottom-up",
		},
	}

	labelled := r.Label && imagelabel.Fits(w, len(label))
	if r.Label && !labelled {
		p.Notes = append(p.Notes, format.Note{
			Code: "label_omitted",
			Detail: fmt.Sprintf(
				"The picture is %d px wide and the label needs more room, so this file carries no visible label. Its name and the manifest still identify it.",
				w),
		})
	}
	p.Properties["label_embedded"] = labelled
	p.Memo = m
	return p, nil
}

// chooseSize settles the picture size.
//
// Named dimensions are used as given. Left out, the picture is grown to fill
// the request, because a BMP stores its pixels uncompressed and the size is
// therefore arithmetic rather than whatever an encoder decides. The remainder
// is always smaller than one row.
func chooseSize(r format.Request) (int, int, error) {
	_, wSet := r.Properties["width"]
	_, hSet := r.Properties["height"]

	if wSet || hSet {
		w, err := dimension(r.Properties, "width", 0)
		if err != nil {
			return 0, 0, err
		}
		h, err := dimension(r.Properties, "height", 0)
		if err != nil {
			return 0, 0, err
		}
		switch {
		case wSet && hSet:
			return w, h, nil
		case wSet:
			// One side named, the other worked out from what is left.
			return w, fill(r.Bytes, w), nil
		default:
			return fillWidth(r.Bytes, h), h, nil
		}
	}

	avail := r.Bytes - headers
	if avail < int64(stride(minDimension)) {
		return minDimension, minDimension, nil
	}
	// A square is the shape that puts the most pixels in the fewest wasted
	// bytes of row rounding, and it is what a person expects to see when they
	// did not say.
	w := int(isqrt(uint64(avail / 3)))
	if w < minDimension {
		w = minDimension
	}
	if w > maxDimension {
		w = maxDimension
	}
	for w > minDimension && int64(stride(w)) > avail {
		w--
	}
	return w, fill(r.Bytes, w), nil
}

// fill is the tallest picture of this width that still fits, at least one row.
func fill(bytes int64, width int) int {
	avail := bytes - headers
	if avail < int64(stride(width)) {
		return minDimension
	}
	h := avail / int64(stride(width))
	if h > maxDimension {
		h = maxDimension
	}
	return int(h)
}

// fillWidth is the widest picture of this height that still fits.
func fillWidth(bytes int64, height int) int {
	avail := bytes - headers
	if height < 1 || avail < 4 {
		return minDimension
	}
	perRow := avail / int64(height)
	// Undo the rounding: a row of w pixels costs stride(w) bytes.
	w := int(perRow / 3)
	if w > maxDimension {
		w = maxDimension
	}
	for w > minDimension && int64(stride(w))*int64(height) > avail {
		w--
	}
	if w < minDimension {
		w = minDimension
	}
	return w
}

// isqrt is an integer square root, so that the picture a size produces is the
// same on every machine. Floating point would almost certainly agree, and
// "almost certainly" is not what byte stability across platforms means.
func isqrt(n uint64) uint64 {
	if n == 0 {
		return 0
	}
	x := n
	y := (x + 1) / 2
	for y < x {
		x = y
		y = (x + n/x) / 2
	}
	return x
}

func dimension(props map[string]string, key string, fallback int) (int, error) {
	raw, ok := props[key]
	if !ok || raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("bmp: %s must be a whole number of pixels, got %q", key, raw)
	}
	if n < minDimension || n > maxDimension {
		return 0, fmt.Errorf("bmp: %s must be between %d and %d pixels, got %d", key, minDimension, maxDimension, n)
	}
	return n, nil
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(memo)
	if !ok {
		return fmt.Errorf("bmp: the plan was not produced by this generator")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if err := writeHeaders(w, m, p.Bytes); err != nil {
		return err
	}
	if err := writeGap(ctx, w, m.seed, m.gap); err != nil {
		return err
	}
	return writePixels(ctx, w, m)
}

func writeHeaders(w io.Writer, m memo, total int64) error {
	var head [headers]byte
	head[0], head[1] = 'B', 'M'
	binary.LittleEndian.PutUint32(head[2:6], uint32(total))
	// Two reserved fields stay zero.
	binary.LittleEndian.PutUint32(head[10:14], uint32(headers+m.gap))

	info := head[fileHeader:]
	binary.LittleEndian.PutUint32(info[0:4], infoHeader)
	binary.LittleEndian.PutUint32(info[4:8], uint32(int32(m.width)))
	binary.LittleEndian.PutUint32(info[8:12], uint32(int32(m.height)))
	binary.LittleEndian.PutUint16(info[12:14], 1)
	binary.LittleEndian.PutUint16(info[14:16], bitsPerPixel)
	// Compression stays zero, which means none.
	binary.LittleEndian.PutUint32(info[20:24], uint32(pixelBytes(m.width, m.height)))
	// 2835 pixels per metre is 72 dpi, what most tools write.
	binary.LittleEndian.PutUint32(info[24:28], 2835)
	binary.LittleEndian.PutUint32(info[28:32], 2835)
	// No palette, so the last two fields stay zero.

	_, err := w.Write(head[:])
	return err
}

// gapChunkSize is how much filler is built before each write. It also sets how
// often cancellation is noticed.
const gapChunkSize = 32 * 1024

// writeGap emits the padding without ever holding it in memory. The gap can be
// most of the file when the dimensions were named, so a fixed buffer is the
// difference between a large file and a failed run.
func writeGap(ctx context.Context, w io.Writer, seed uint64, n int64) error {
	if n <= 0 {
		return nil
	}
	rng := core.NewRand(seed)
	size := int64(gapChunkSize)
	if n < size {
		size = n
	}
	buf := make([]byte, size)
	for remaining := n; remaining > 0; {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		take := int64(len(buf))
		if remaining < take {
			take = remaining
		}
		chunk := buf[:take]
		for i := 0; i < len(chunk); i += 8 {
			var eight [8]byte
			binary.BigEndian.PutUint64(eight[:], rng.Uint64())
			copy(chunk[i:], eight[:])
		}
		if _, err := w.Write(chunk); err != nil {
			return err
		}
		remaining -= take
	}
	return nil
}

// writePixels streams the picture a row at a time.
//
// Only the label band is ever built as an image, and it is at most 24 rows
// tall whatever the picture is. Everything else comes from the formula
// straight into one reused row buffer, so a 64 MiB picture costs a row rather
// than 64 MiB.
func writePixels(ctx context.Context, w io.Writer, m memo) error {
	row := make([]byte, stride(m.width))
	off := int(m.seed % 256)

	band := labelBand(m, off)
	bandH := 0
	if band != nil {
		bandH = band.Bounds().Dy()
	}

	// A BMP stores its rows bottom up, so the label at the top of the picture
	// is the last thing written.
	for y := m.height - 1; y >= 0; y-- {
		if y%64 == 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}
		if y < bandH {
			copyBandRow(row, band, y, m.width)
		} else {
			fillRow(row, y, m.width, off)
		}
		if _, err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}

// fillRow writes one row of the gradient in the order a BMP wants it: blue,
// green, red, and the rounding bytes left as they were.
func fillRow(row []byte, y, width, off int) {
	for x := 0; x < width; x++ {
		row[x*3] = uint8((x + y + off) % 256)
		row[x*3+1] = uint8((y + off) % 256)
		row[x*3+2] = uint8((x + off) % 256)
	}
	for i := width * 3; i < len(row); i++ {
		row[i] = 0
	}
}

func copyBandRow(row []byte, band *image.RGBA, y, width int) {
	for x := 0; x < width; x++ {
		i := band.PixOffset(x, y)
		row[x*3] = band.Pix[i+2]
		row[x*3+1] = band.Pix[i+1]
		row[x*3+2] = band.Pix[i]
	}
	for i := width * 3; i < len(row); i++ {
		row[i] = 0
	}
}

// labelBand rasterises the top strip of the picture, label included, or
// returns nil when there is no label to draw.
func labelBand(m memo, off int) *image.RGBA {
	if m.label == "" || !imagelabel.Fits(m.width, len(m.label)) {
		return nil
	}
	h := maxBandHeight
	if m.height < h {
		h = m.height
	}
	img := image.NewRGBA(image.Rect(0, 0, m.width, h))
	for y := 0; y < h; y++ {
		for x := 0; x < m.width; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8((x + off) % 256),
				G: uint8((y + off) % 256),
				B: uint8((x + y + off) % 256),
				A: 255,
			})
		}
	}
	imagelabel.Draw(img, m.label)
	return img
}
