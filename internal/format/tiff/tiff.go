// Package tiff generates uncompressed TIFF images.
//
// The second format whose size is arithmetic rather than whatever an encoder
// decides, after BMP - and it is built the same way for the same reason. The
// pixels are stored uncompressed, so a request for 10 MB can be answered with
// a picture worth 10 MB instead of a thumbnail followed by filler nobody can
// see.
//
// Written by hand rather than taken from a library, and that was a decision
// with a measurement behind it (docs/STACK.md section 4.9). The candidate,
// x/image/tiff, encodes two of the five compressions its own enum declares,
// gates Predictor on the one it cannot do, writes little-endian only and emits
// a single directory - two of the four variant axes this project documents. It
// would also have been the first outside encoder inside the byte stability
// contract D11, and its version is raised by the window toolkit rather than by
// us, so updating Fyne would have moved TIFF hashes in the command line binary.
//
// The padding channel is the gap between the header and the pixel data, which
// StripOffsets points past. That is the one candidate of five the format
// itself talks about: everything before that offset is space the file
// describes rather than space it keeps quiet about - the same shape as
// bfOffBits in BMP. Measured on five independent readers at every size from
// 1 B to 10 MiB (docs/MVP-FORMATS.md section 2.11).
package tiff

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

	// header is the eight byte TIFF header: the byte order mark, the magic
	// number 42 and the offset of the first directory.
	header = 8

	// samplesPerPixel is three, one byte each for red, green and blue. No
	// alpha and no palette, which is the shape every reader understands.
	samplesPerPixel = 3
	bitsPerSample   = 8

	// entryCount is how many directory entries every file we write carries.
	// Fixed, because the picture never changes which tags it needs.
	entryCount = 12
	// directory is the whole IFD: the entry count, the entries, and the
	// offset of the next directory, which is always zero because we write one.
	directory = 2 + 12*entryCount + 4
	// heap is the space after the directory for values too long for the four
	// bytes an entry holds: BitsPerSample is three shorts, and the two
	// resolution values are rationals of eight bytes each.
	heap = samplesPerPixel*2 + 8 + 8

	// overhead is everything in the file that is not a pixel and not padding.
	overhead = header + directory + heap

	minDimension = 1
	maxDimension = 20000

	// StripOffsets and StripByteCounts are LONG, which is a four byte
	// unsigned field, so neither the offset of the pixels nor their length
	// can pass 4 GiB. Reasoned from the format, not measured - a file that
	// size will not fit on the machine this was written on.
	maxFileBytes = 1<<32 - 1

	// The tallest label band the rasteriser produces, so the strip carrying
	// the label is built once at a known height and the rest of the picture
	// streams past without ever being held.
	maxBandHeight = 24
)

// TIFF data types, from the specification.
const (
	typeShort    = 3
	typeLong     = 4
	typeRational = 5
)

// Tags this generator writes, in the ascending order a directory requires.
const (
	tagImageWidth      = 256
	tagImageLength     = 257
	tagBitsPerSample   = 258
	tagCompression     = 259
	tagPhotometric     = 262
	tagStripOffsets    = 273
	tagSamplesPerPixel = 277
	tagRowsPerStrip    = 278
	tagStripByteCounts = 279
	tagXResolution     = 282
	tagYResolution     = 283
	tagResolutionUnit  = 296
)

func init() {
	format.Register(format.Descriptor{
		ID:          "tiff",
		Extension:   ".tiff",
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,

		// The smallest TIFF this generator can produce: one pixel of three
		// bytes, plus everything that is not a pixel.
		MinBytes: overhead + samplesPerPixel,

		Padding: format.PaddingChannel{
			// Measured against five independent readers - Pillow, the Windows
			// Imaging Component, GDI+, exiftool and x/image - at every size
			// from one byte to 10 MiB, odd sizes included. All five read the
			// image and return identical pixels.
			//
			// Five channels passed that measurement and this is the one the
			// format itself sets aside: StripOffsets says where the pixels
			// begin, so anything between the header and that offset is space
			// the file describes. Bytes after the directory are accepted by
			// all five too, but no field mentions them.
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

// pixelBytes is the length of the image data. A TIFF row is not rounded up to
// anything, which is the one place this format is simpler than BMP.
func pixelBytes(width, height int) int64 {
	return int64(width) * int64(height) * samplesPerPixel
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	label := ""
	if r.Label {
		label = core.Label("tiff", r.Bytes, r.Seed)
	}

	w, h, err := chooseSize(r)
	if err != nil {
		return format.Plan{}, err
	}

	bare := overhead + pixelBytes(w, h)
	if r.Bytes < bare {
		return format.Plan{}, &format.BelowMinimumError{
			Format:    "TIFF",
			Requested: r.Bytes,
			Minimum:   bare,
			Reason: fmt.Sprintf(
				"a %dx%d picture is %d B of pixels at three bytes each, and the header, the directory and its values take another %d B",
				w, h, pixelBytes(w, h), overhead),
			Hint: fmt.Sprintf("Ask for %d B or more, or set a smaller width and height", bare),
		}
	}
	if r.Bytes > maxFileBytes {
		return format.Plan{}, &format.AboveMaximumError{
			Format:    "TIFF",
			Requested: r.Bytes,
			Maximum:   maxFileBytes,
			Reason:    "a TIFF locates its pixels with a four byte offset, so the format cannot describe a file this large",
			Hint:      "Ask for 4 GiB or less, or pick a format with no offset of its own such as gif.",
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
			"bit_depth":   bitsPerSample * samplesPerPixel,
			"compression": "none",
			"byte_order":  "little-endian",
			"row_order":   "top-down",
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
	p.Properties[format.PropertyLabelEmbedded] = labelled
	p.Memo = m
	return p, nil
}

// chooseSize settles the picture size.
//
// Named dimensions are used as given. Left out, the picture is grown to fill
// the request, because the pixels are stored uncompressed and the size is
// therefore arithmetic rather than whatever an encoder decides. What is left
// over goes into the gap.
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

	avail := r.Bytes - overhead
	if avail < samplesPerPixel {
		return minDimension, minDimension, nil
	}
	// A square puts the most pixels into the request and is what a person
	// expects to see when they did not say otherwise.
	w := int(isqrt(uint64(avail / samplesPerPixel)))
	if w < minDimension {
		w = minDimension
	}
	if w > maxDimension {
		w = maxDimension
	}
	for w > minDimension && pixelBytes(w, 1) > avail {
		w--
	}
	return w, fill(r.Bytes, w), nil
}

// fill is the tallest picture of this width that still fits, at least one row.
func fill(bytes int64, width int) int {
	avail := bytes - overhead
	rowBytes := pixelBytes(width, 1)
	if rowBytes <= 0 || avail < rowBytes {
		return minDimension
	}
	h := avail / rowBytes
	if h > maxDimension {
		h = maxDimension
	}
	return int(h)
}

// fillWidth is the widest picture of this height that still fits.
func fillWidth(bytes int64, height int) int {
	avail := bytes - overhead
	if height < 1 || avail < samplesPerPixel {
		return minDimension
	}
	w := avail / (int64(height) * samplesPerPixel)
	if w > maxDimension {
		w = maxDimension
	}
	if w < minDimension {
		w = minDimension
	}
	for w > minDimension && pixelBytes(int(w), height) > avail {
		w--
	}
	return int(w)
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
		return 0, fmt.Errorf("tiff: %s must be a whole number of pixels, got %q", key, raw)
	}
	if n < minDimension || n > maxDimension {
		return 0, fmt.Errorf("tiff: %s must be between %d and %d pixels, got %d", key, minDimension, maxDimension, n)
	}
	return n, nil
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(memo)
	if !ok {
		return fmt.Errorf("tiff: the plan was not produced by this generator")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if err := writeHeader(w, m); err != nil {
		return err
	}
	if err := writeGap(ctx, w, m.seed, m.gap); err != nil {
		return err
	}
	if err := writePixels(ctx, w, m); err != nil {
		return err
	}
	return writeDirectory(w, m)
}

// pixelsAt is where the image data begins, which is what StripOffsets holds.
func pixelsAt(m memo) int64 {
	return header + m.gap
}

// directoryAt is where the IFD begins, which is what the header points at.
func directoryAt(m memo) int64 {
	return pixelsAt(m) + pixelBytes(m.width, m.height)
}

func writeHeader(w io.Writer, m memo) error {
	var head [header]byte
	head[0], head[1] = 'I', 'I'
	binary.LittleEndian.PutUint16(head[2:4], 42)
	binary.LittleEndian.PutUint32(head[4:8], uint32(directoryAt(m)))
	_, err := w.Write(head[:])
	return err
}

// writeDirectory emits the IFD and the values that did not fit inside it.
//
// The entries have to be in ascending tag order, which the specification
// requires and readers rely on. The heap follows the directory, so its offsets
// are known once the directory length is - and that length is a constant here,
// because the set of tags never changes.
func writeDirectory(w io.Writer, m memo) error {
	heapAt := uint32(directoryAt(m) + directory)

	var buf []byte
	buf = binary.LittleEndian.AppendUint16(buf, entryCount)

	// Values longer than four bytes live in the heap, in the order they are
	// referenced here.
	bitsAt := heapAt
	xResAt := bitsAt + samplesPerPixel*2
	yResAt := xResAt + 8

	entry := func(tag, kind uint16, count, value uint32) {
		buf = binary.LittleEndian.AppendUint16(buf, tag)
		buf = binary.LittleEndian.AppendUint16(buf, kind)
		buf = binary.LittleEndian.AppendUint32(buf, count)
		buf = binary.LittleEndian.AppendUint32(buf, value)
	}
	// A SHORT that fits in the four byte value field sits in its low half.
	short := func(tag uint16, v uint16) {
		entry(tag, typeShort, 1, uint32(v))
	}

	short(tagImageWidth, uint16(m.width))
	short(tagImageLength, uint16(m.height))
	entry(tagBitsPerSample, typeShort, samplesPerPixel, bitsAt)
	short(tagCompression, 1) // none
	short(tagPhotometric, 2) // RGB
	entry(tagStripOffsets, typeLong, 1, uint32(pixelsAt(m)))
	short(tagSamplesPerPixel, samplesPerPixel)
	short(tagRowsPerStrip, uint16(m.height))
	entry(tagStripByteCounts, typeLong, 1, uint32(pixelBytes(m.width, m.height)))
	entry(tagXResolution, typeRational, 1, xResAt)
	entry(tagYResolution, typeRational, 1, yResAt)
	short(tagResolutionUnit, 2) // inches

	// No second directory.
	buf = binary.LittleEndian.AppendUint32(buf, 0)

	// The heap, in the order the entries above point at it.
	for i := 0; i < samplesPerPixel; i++ {
		buf = binary.LittleEndian.AppendUint16(buf, bitsPerSample)
	}
	// 72 dpi, written as the rational 72/1, which is what most tools write.
	for i := 0; i < 2; i++ {
		buf = binary.LittleEndian.AppendUint32(buf, 72)
		buf = binary.LittleEndian.AppendUint32(buf, 1)
	}

	if len(buf) != directory+heap {
		return fmt.Errorf("tiff: the directory came out %d B and the header says %d B", len(buf), directory+heap)
	}
	_, err := w.Write(buf)
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
		core.FillRandomBE(chunk, rng)
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
//
// A TIFF stores its rows top down, so the label band is written first - the
// one place this is simpler than BMP, which stores them the other way up.
func writePixels(ctx context.Context, w io.Writer, m memo) error {
	row := make([]byte, m.width*samplesPerPixel)
	off := int(m.seed % 256)

	band := labelBand(m, off)
	bandH := 0
	if band != nil {
		bandH = band.Bounds().Dy()
	}

	for y := 0; y < m.height; y++ {
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

// fillRow writes one row of the gradient in the order a TIFF wants it: red,
// green, blue.
func fillRow(row []byte, y, width, off int) {
	for x := 0; x < width; x++ {
		row[x*3] = uint8((x + off) % 256)
		row[x*3+1] = uint8((y + off) % 256)
		row[x*3+2] = uint8((x + y + off) % 256)
	}
}

func copyBandRow(row []byte, band *image.RGBA, y, width int) {
	for x := 0; x < width; x++ {
		i := band.PixOffset(x, y)
		row[x*3] = band.Pix[i]
		row[x*3+1] = band.Pix[i+1]
		row[x*3+2] = band.Pix[i+2]
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
