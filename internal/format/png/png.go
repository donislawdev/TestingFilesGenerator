// Package png generates PNG images.
//
// The first compressed format, and the first time the output size does not
// follow from the content. What lands on disk is whatever deflate decides,
// so the exact size comes from a padding chunk that makes up the difference.
package png

import (
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	stdpng "image/png"
	"io"
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/imagelabel"
)

const (
	generatorVersion = "1"

	// chunkOverhead is what a PNG chunk costs beyond its data: four bytes of
	// length, four of type, four of CRC.
	chunkOverhead = 12

	// iendSize is the size of the closing chunk, which is always last and
	// always empty.
	iendSize = 12

	// paddingChunk is a private ancillary chunk.
	//
	// The naming rules carry meaning. Lower case first letter means ancillary,
	// so a decoder may skip it. Lower case second means private, so it is
	// ours and no standard assigns it. Upper case third is reserved and must
	// stay upper case. Lower case fourth means safe to copy.
	//
	// Measured against five independent decoders - Pillow, FFmpeg, Tcl/Tk,
	// the Windows Imaging Component and exiftool. All five read the image and
	// return identical pixels. A tEXt chunk in the same position also works,
	// but exiftool reports "Text/EXIF chunk(s) found after PNG IDAT" for it,
	// and a warning is exactly what makes a tester think the generator is
	// broken. A private chunk draws no comment from any of the five.
	paddingChunk = "tfGp"

	// defaultWidth and defaultHeight are used when the recipe names no size
	// for the picture. Small enough that encoding costs little, large enough
	// that the label is readable.
	defaultWidth  = 640
	defaultHeight = 480

	minDimension = 1
	maxDimension = 20000

	// The picture is built in memory as one buffer and encoded twice, once
	// while planning and once while writing. Measured: 10000x10000 peaks at
	// 1.17 GB and 20000x20000 at 4.65 GB, which is enough to end a run on an
	// ordinary machine.
	//
	// The documented range reaches 8K, which is 33 megapixels, and shapes
	// such as 1x10000. This budget covers both with room to spare and refuses
	// what would otherwise be an out of memory error with no explanation.
	maxPixels = 40_000_000

	// A chunk carries its length in four bytes and the format caps it at
	// 2^31-1. Beyond that the padding would need several chunks.
	maxChunkData = 1<<31 - 1
)

func init() {
	format.Register(format.Descriptor{
		ID:          "png",
		Extension:   ".png",
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,

		// The smallest PNG this generator can produce - a one pixel picture
		// plus the closing chunk. Measured rather than guessed, and a test
		// keeps this number honest.
		MinBytes: minimumBytes(),

		Padding: format.PaddingChannel{
			Name:     "private ancillary chunk before IEND",
			Where:    format.PlacementEnd,
			Capacity: maxChunkData,
		},
		Label:  format.LabelVisible,
		Oracle: "pillow",
		Properties: []format.Property{
			// The declaration bounds each side on its own and cannot say that
			// the two multiplied have a limit as well, so the sentence carries
			// it. Without that, "tfg formats png" offered 20000 by 20000 and
			// the run then refused it - the tool advertising a pair it does not
			// accept. Naming the whole rule in the registry needs a shape for a
			// limit on two settings at once, which is a change to AR9 rather
			// than a line here. Recorded in docs/OBSERVATIONS.md, O45.
			{
				Name: "width", Kind: format.PropertyInt,
				Min: minDimension, Max: maxDimension, Unit: "pixels",
				Detail: "How wide the picture is. Left out, a size is chosen that fits the bytes you asked for. Width times height cannot pass 40 megapixels, so both sides cannot be at their largest at once.",
			},
			{
				Name: "height", Kind: format.PropertyInt,
				Min: minDimension, Max: maxDimension, Unit: "pixels",
				Detail: "How tall the picture is. Left out, a size is chosen that fits the bytes you asked for. Width times height cannot pass 40 megapixels, so both sides cannot be at their largest at once.",
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
	// body is the exact number of bytes the encoded picture takes before the
	// closing chunk. Worked out during planning so that a size this format
	// cannot reach is refused before any file exists.
	body int64
	// padData is how many bytes of padding the chunk carries. A negative
	// value means no chunk at all, which happens when the picture lands
	// exactly on the requested size.
	padData int64
	withPad bool
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	label := ""
	if r.Label {
		label = core.Label("png", r.Bytes, r.Seed)
	}

	m, explicit, err := chooseSize(r, label)
	if err != nil {
		return format.Plan{}, err
	}
	_ = explicit
	w, h := m.width, m.height
	body := m.body

	p := format.Plan{
		Bytes:       r.Bytes,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties: map[string]any{
			"width":       w,
			"height":      h,
			"colour_type": "rgba",
			"bit_depth":   8,
			"interlaced":  false,
		},
	}

	// What the picture takes on its own, closing chunk included.
	bare := body + iendSize

	switch {
	case r.Bytes == bare:
		// The picture lands exactly on the requested size. No padding chunk.
		m.withPad = false

	case r.Bytes < bare:
		return format.Plan{}, &format.BelowMinimumError{
			Format:    "PNG",
			Requested: r.Bytes,
			Minimum:   bare,
			Reason:    fmt.Sprintf("a %dx%d picture already encodes to that much before any padding", w, h),
			Hint:      fmt.Sprintf("Ask for %d B or more, or set a smaller width and height with --set width=... --set height=...", bare),
		}

	case r.Bytes < bare+chunkOverhead:
		// Between the two lies a gap of eleven byte counts that no
		// combination of chunks can hit, because the smallest chunk that
		// exists is an empty one and it costs twelve bytes.
		return format.Plan{}, &format.BelowMinimumError{
			Format:    "PNG",
			Requested: r.Bytes,
			Minimum:   bare + chunkOverhead,
			Reason: fmt.Sprintf(
				"a %dx%d picture encodes to exactly %d B, and the padding chunk that makes up any difference costs %d B on its own, so nothing between those two is reachable",
				w, h, bare, chunkOverhead),
			Hint: fmt.Sprintf("Ask for exactly %d B or for %d B or more.", bare, bare+chunkOverhead),
		}

	default:
		m.withPad = true
		m.padData = r.Bytes - bare - chunkOverhead
		if m.padData > maxChunkData {
			return format.Plan{}, &format.BelowMinimumError{
				Format:    "PNG",
				Requested: r.Bytes,
				Minimum:   bare,
				Reason:    "the padding needed is larger than one chunk can carry, and this generator writes only one",
				Hint:      "Ask for a smaller size, or set larger dimensions so the picture itself carries more of it.",
			}
		}
	}

	if r.Label && !imagelabel.Fits(w, len(label)) {
		p.Notes = append(p.Notes, format.Note{
			Code: "label_omitted",
			Detail: fmt.Sprintf(
				"The picture is %d px wide and the label needs more room, so this file carries no visible label. Its name and the manifest still identify it.",
				w),
		})
	}
	p.Properties["label_embedded"] = r.Label && imagelabel.Fits(w, len(label))
	p.Memo = m
	return p, nil
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(memo)
	if !ok {
		return fmt.Errorf("png: the plan was not produced by this generator")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// The closing chunk is always the final twelve bytes, so hold those back
	// and let everything before them stream straight out. Nothing is buffered
	// beyond that, which matters because a picture can be very large.
	holder := &tailHolder{w: w, keep: iendSize}
	if err := encode(holder, m); err != nil {
		return err
	}

	if holder.written != m.body {
		return fmt.Errorf("png: the picture encoded to %d B where planning said %d B", holder.written, m.body)
	}
	if string(holder.tail[4:8]) != "IEND" {
		return fmt.Errorf("png: the encoded stream does not end with IEND")
	}

	if m.withPad {
		if err := writePaddingChunk(ctx, w, paddingChunk, m.seed, m.padData); err != nil {
			return err
		}
	}

	_, err := w.Write(holder.tail)
	return err
}

// sizeLadder is tried from the largest down when the recipe names no picture
// size. The first rung that leaves room for the padding chunk wins, so a
// small file gets a small picture instead of being refused.
var sizeLadder = [][2]int{
	{640, 480}, {320, 240}, {160, 120}, {80, 60},
	{40, 30}, {20, 15}, {8, 6}, {4, 3}, {2, 2}, {1, 1},
}

// chooseSize settles the picture size and measures what it encodes to.
//
// When the recipe names width and height, those are used and the size is
// either reachable or an error. When it does not, the ladder is walked from
// the largest rung down until one fits, which is what lets a request for a
// few hundred bytes succeed rather than being told the default picture is too
// big for it.
func chooseSize(r format.Request, label string) (memo, bool, error) {
	wRaw, wSet := r.Properties["width"]
	hRaw, hSet := r.Properties["height"]

	if wSet || hSet {
		w, err := dimension(r.Properties, "width", defaultWidth)
		if err != nil {
			return memo{}, true, err
		}
		h, err := dimension(r.Properties, "height", defaultHeight)
		if err != nil {
			return memo{}, true, err
		}
		_, _ = wRaw, hRaw
		// A PropertyValueError rather than a plain one, and the difference is
		// the exit code somebody's CI reads. This used to end with 1, which
		// means the tool itself broke, for a pair of numbers the caller chose.
		// That is the same defect closed for declared ranges on 2026-08-03,
		// surviving one layer deeper: the declaration bounds each dimension on
		// its own and cannot express a limit on the two multiplied.
		if int64(w)*int64(h) > maxPixels {
			return memo{}, true, &format.PropertyValueError{
				Format: "png", Key: "width and height",
				Value: fmt.Sprintf("%dx%d", w, h),
				Reason: fmt.Sprintf(
					"together they come to %d megapixels and the limit is %d, because the picture is held in memory while it is encoded. Each side may go up to %d, but not both at once - ask for a smaller pair",
					int64(w)*int64(h)/1_000_000, maxPixels/1_000_000, maxDimension),
			}
		}
		m := memo{width: w, height: h, seed: r.Seed, label: label}
		body, err := encodedBodySize(m)
		if err != nil {
			return memo{}, true, err
		}
		m.body = body
		return m, true, nil
	}

	var smallest memo
	for i, rung := range sizeLadder {
		m := memo{width: rung[0], height: rung[1], seed: r.Seed, label: label}
		body, err := encodedBodySize(m)
		if err != nil {
			return memo{}, false, err
		}
		m.body = body
		smallest = m

		bare := body + iendSize
		if r.Bytes == bare || r.Bytes >= bare+chunkOverhead {
			return m, false, nil
		}
		_ = i
	}
	// Nothing fitted, not even one pixel. The caller turns this into the
	// error that names the minimum.
	return smallest, false, nil
}

func dimension(props map[string]string, key string, fallback int) (int, error) {
	raw, ok := props[key]
	if !ok || raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("png: %s must be a whole number of pixels, got %q", key, raw)
	}
	if n < minDimension || n > maxDimension {
		return 0, fmt.Errorf("png: %s must be between %d and %d pixels, got %d", key, minDimension, maxDimension, n)
	}
	return n, nil
}

// picture builds the image. Deterministic from the seed, and compressible, so
// that the encoded result is small next to any realistic requested size and
// the padding chunk carries the difference.
func picture(m memo) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, m.width, m.height))
	// The seed shifts the gradient, so two seeds give visibly different
	// pictures rather than the same picture with different padding.
	off := int(m.seed % 256)
	// SetRGBA rather than Set, and the difference is not style. Set takes a
	// color.Color interface, so every pixel boxes a colour onto the heap - it
	// measured 786443 allocations for a 16 MiB image, 87% of everything this
	// generator allocates. SetRGBA takes the concrete type and allocates
	// nothing. The pixels are identical either way, because an RGBA image
	// stores an RGBA colour without converting it.
	for y := 0; y < m.height; y++ {
		for x := 0; x < m.width; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8((x + off) % 256),
				G: uint8((y + off) % 256),
				B: uint8((x + y + off) % 256),
				A: 255,
			})
		}
	}
	if m.label != "" {
		imagelabel.Draw(img, m.label)
	}
	return img
}

func encode(w io.Writer, m memo) error {
	enc := stdpng.Encoder{CompressionLevel: stdpng.DefaultCompression}
	return enc.Encode(w, picture(m))
}

// encodedBodySize is how many bytes the picture takes before the closing
// chunk. Encoding once during planning is what lets a size this format cannot
// reach be refused before any file exists.
//
// Measured cost: a 3840x2160 gradient encodes in about 176 ms, so this
// doubles that for pictures of that size. At the default 640x480 it is a
// couple of milliseconds. Nothing is held in memory - the bytes are counted
// and dropped.
func encodedBodySize(m memo) (int64, error) {
	holder := &tailHolder{w: io.Discard, keep: iendSize}
	if err := encode(holder, m); err != nil {
		return 0, err
	}
	return holder.written, nil
}

// minimumBytes is the smallest file this generator can produce: a one pixel
// picture with no label and no padding chunk.
func minimumBytes() int64 {
	body, err := encodedBodySize(memo{width: 1, height: 1})
	if err != nil {
		// Encoding a single pixel cannot fail. If it somehow does, refusing
		// every size is safer than declaring a minimum we did not measure.
		return 1 << 62
	}
	return body + iendSize
}

// padChunkSize is how much padding is built before each write. It also sets
// how often cancellation is noticed.
const padChunkSize = 32 * 1024

// writePaddingChunk emits the padding chunk without ever holding it in memory.
//
// The length is known in advance and the checksum is fed as the bytes go by,
// so a gigabyte of padding costs a 32 KiB buffer rather than a gigabyte.
// Measured before this was streamed: a 600 MiB PNG took 613 MB of memory
// while the same size of text took 42 MB.
func writePaddingChunk(ctx context.Context, w io.Writer, kind string, seed uint64, n int64) error {
	var head [8]byte
	binary.BigEndian.PutUint32(head[0:4], uint32(n))
	copy(head[4:8], kind)
	if _, err := w.Write(head[:]); err != nil {
		return err
	}

	crc := crc32.NewIEEE()
	crc.Write([]byte(kind))

	rng := core.NewRand(seed)
	buf := make([]byte, padChunkSize)
	for remaining := n; remaining > 0; {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		size := int64(len(buf))
		if remaining < size {
			size = remaining
		}
		chunk := buf[:size]
		for i := 0; i < len(chunk); i += 8 {
			var eight [8]byte
			binary.BigEndian.PutUint64(eight[:], rng.Uint64())
			copy(chunk[i:], eight[:])
		}

		crc.Write(chunk)
		if _, err := w.Write(chunk); err != nil {
			return err
		}
		remaining -= size
	}

	var tail [4]byte
	binary.BigEndian.PutUint32(tail[:], crc.Sum32())
	_, err := w.Write(tail[:])
	return err
}

// tailHolder passes bytes through while keeping the last keep bytes back, so
// the caller can insert something in front of them.
type tailHolder struct {
	w       io.Writer
	keep    int
	tail    []byte
	written int64
}

func (t *tailHolder) Write(p []byte) (int, error) {
	n := len(p)
	buf := append(t.tail, p...)
	if len(buf) > t.keep {
		emit := buf[:len(buf)-t.keep]
		if _, err := t.w.Write(emit); err != nil {
			return 0, err
		}
		t.written += int64(len(emit))
		buf = buf[len(buf)-t.keep:]
	}
	t.tail = append([]byte(nil), buf...)
	return n, nil
}
