// Package ico generates Windows icon files.
//
// An ICO is a directory of images rather than an image, so it carries an
// explicit offset saying where each one starts. That offset is the padding
// channel: anything between the directory and the picture is space the file
// describes.
//
// The picture inside can be a device independent bitmap or a whole PNG, and
// both are in use. This generator writes either, because which one a system
// under test accepts is exactly the sort of thing a tester needs to find out.
package ico

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
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

	// directoryHeader is ICONDIR: two reserved bytes, the type, and how many
	// images follow.
	directoryHeader = 6
	// directoryEntry is one ICONDIRENTRY. This generator writes a single
	// image, so there is exactly one.
	directoryEntry = 16
	// preamble is everything before the padding gap.
	preamble = directoryHeader + directoryEntry

	// infoHeader is the BITMAPINFOHEADER a device independent bitmap starts
	// with.
	infoHeader = 40

	// A directory entry stores each side in one byte, with zero meaning 256.
	// That is the format's own ceiling, not ours.
	minDimension = 1
	maxDimension = 256

	// The offset and the length of the image are four byte fields, so the
	// file cannot pass 4 GiB. Reasoned from the format, not measured.
	maxFileBytes = 1<<32 - 1

	embedBMP = "bmp"
	embedPNG = "png"
)

func init() {
	format.Register(format.Descriptor{
		ID:          "ico",
		Extension:   ".ico",
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,

		MinBytes: minimumBytes(),

		Padding: format.PaddingChannel{
			// Measured against five independent readers - Pillow, the Windows
			// Imaging Component, GDI+, exiftool and ffprobe - at every size
			// from one byte to 100 MiB, odd sizes included, and with both
			// kinds of embedded picture. All five read the icon and return
			// identical pixels.
			Name:     "the gap between the directory and the image data",
			Where:    format.PlacementInside,
			Capacity: maxFileBytes,
		},
		Label:  format.LabelVisible,
		Oracle: "pillow",
		Properties: []format.Property{
			{
				Name: "width", Kind: format.PropertyInt,
				Min: minDimension, Max: maxDimension, Unit: "pixels",
				Detail: "How wide the icon is. Left out, the largest standard icon size that fits is used.",
			},
			{
				Name: "height", Kind: format.PropertyInt,
				Min: minDimension, Max: maxDimension, Unit: "pixels",
				Detail: "How tall the icon is. Left out, the largest standard icon size that fits is used.",
			},
			{
				Name: "embed", Kind: format.PropertyChoice,
				Choices: []string{embedBMP, embedPNG},
				Default: embedBMP,
				Detail:  "What sits inside the icon. A bitmap is read by every version of Windows. A PNG makes the file much smaller and needs Windows Vista or newer.",
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
	embed         string
	// image is the picture as it will sit inside the file. Small by
	// construction, because an icon side is at most 256 pixels.
	image []byte
	// gap is how many bytes stand between the directory and that picture.
	gap int64
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	label := ""
	if r.Label {
		label = core.Label("ico", r.Bytes, r.Seed)
	}

	embed, err := embedding(r.Properties)
	if err != nil {
		return format.Plan{}, err
	}

	w, h, err := chooseSize(r, label, embed)
	if err != nil {
		return format.Plan{}, err
	}

	blob, err := build(memo{width: w, height: h, seed: r.Seed, label: label, embed: embed})
	if err != nil {
		return format.Plan{}, err
	}

	bare := int64(preamble) + int64(len(blob))
	if err := reachable(r.Bytes, bare, w, h, embed, len(blob)); err != nil {
		return format.Plan{}, err
	}

	m := memo{
		width: w, height: h, seed: r.Seed, label: label, embed: embed,
		image: blob, gap: r.Bytes - bare,
	}

	p := format.Plan{
		Bytes:       r.Bytes,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties: map[string]any{
			"width":       w,
			"height":      h,
			"embed":       embed,
			"bit_depth":   32,
			"image_count": 1,
		},
	}

	labelled := label != "" && imagelabel.Fits(w, len(label))
	if r.Label && !labelled {
		p.Notes = append(p.Notes, format.Note{
			Code: "label_omitted",
			Detail: fmt.Sprintf(
				"The icon is %d px wide and the label needs more room, so this file carries no visible label. Its name and the manifest still identify it.",
				w),
		})
	}
	p.Properties["label_embedded"] = labelled
	p.Memo = m
	return p, nil
}

// reachable refuses a size this icon cannot have: below what the picture
// already costs, or beyond what the directory's four byte fields can describe.
func reachable(want, bare int64, w, h int, embed string, picture int) error {
	if want < bare {
		return &format.BelowMinimumError{
			Format:    "ICO",
			Requested: want,
			Minimum:   bare,
			Reason: fmt.Sprintf(
				"a %dx%d icon holding a %s is %d B of picture, and the %d B directory sits in front of it",
				w, h, embed, picture, preamble),
			Hint: fmt.Sprintf("Ask for %d B or more, set a smaller width and height, or set embed=%s", bare, embedPNG),
		}
	}
	if want > maxFileBytes {
		return &format.BelowMinimumError{
			Format:    "ICO",
			Requested: want,
			Minimum:   maxFileBytes,
			Reason:    "an icon states where its picture starts in a four byte field, so the format cannot describe a file this large",
			Hint:      "Ask for 4 GiB or less, or pick a format with no offset field of its own such as gif.",
		}
	}
	return nil
}

func embedding(props map[string]string) (string, error) {
	raw, ok := props["embed"]
	if !ok || raw == "" {
		return embedBMP, nil
	}
	switch raw {
	case embedBMP, embedPNG:
		return raw, nil
	}
	return "", &format.PropertyValueError{
		Format: "ico", Key: "embed", Value: raw,
		Reason: fmt.Sprintf("it has to be %s or %s", embedBMP, embedPNG),
	}
}

// standardSizes are the icon sizes Windows itself uses, tried from the largest
// down when the recipe names none.
var standardSizes = []int{256, 128, 64, 48, 32, 16, 8, 4, 2, 1}

func chooseSize(r format.Request, label, embed string) (int, int, error) {
	_, wSet := r.Properties["width"]
	_, hSet := r.Properties["height"]

	if wSet || hSet {
		w, err := dimension(r.Properties, "width", maxDimension)
		if err != nil {
			return 0, 0, err
		}
		h, err := dimension(r.Properties, "height", maxDimension)
		if err != nil {
			return 0, 0, err
		}
		return w, h, nil
	}

	smallest := standardSizes[len(standardSizes)-1]
	for _, side := range standardSizes {
		blob, err := build(memo{width: side, height: side, seed: r.Seed, label: label, embed: embed})
		if err != nil {
			return 0, 0, err
		}
		if r.Bytes >= int64(preamble)+int64(len(blob)) {
			return side, side, nil
		}
	}
	return smallest, smallest, nil
}

func dimension(props map[string]string, key string, fallback int) (int, error) {
	raw, ok := props[key]
	if !ok || raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("ico: %s must be a whole number of pixels, got %q", key, raw)
	}
	if n < minDimension || n > maxDimension {
		return 0, fmt.Errorf("ico: %s must be between %d and %d pixels, got %d - an icon stores each side in a single byte", key, minDimension, maxDimension, n)
	}
	return n, nil
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(memo)
	if !ok {
		return fmt.Errorf("ico: the plan was not produced by this generator")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if err := writeDirectory(w, m); err != nil {
		return err
	}
	if err := writeGap(ctx, w, m.seed, m.gap); err != nil {
		return err
	}
	_, err := w.Write(m.image)
	return err
}

func writeDirectory(w io.Writer, m memo) error {
	var head [preamble]byte
	// Two reserved bytes stay zero, then the type: 1 means icon.
	binary.LittleEndian.PutUint16(head[2:4], 1)
	binary.LittleEndian.PutUint16(head[4:6], 1)

	e := head[directoryHeader:]
	e[0] = sideByte(m.width)
	e[1] = sideByte(m.height)
	// No palette, one reserved byte, one colour plane.
	binary.LittleEndian.PutUint16(e[4:6], 1)
	binary.LittleEndian.PutUint16(e[6:8], 32)
	binary.LittleEndian.PutUint32(e[8:12], uint32(len(m.image)))
	binary.LittleEndian.PutUint32(e[12:16], uint32(int64(preamble)+m.gap))

	_, err := w.Write(head[:])
	return err
}

// sideByte is how a directory entry states one side: a single byte, in which
// zero means 256.
func sideByte(n int) byte {
	if n >= 256 {
		return 0
	}
	return byte(n)
}

const gapChunkSize = 32 * 1024

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

// build produces the picture that sits inside the icon.
func build(m memo) ([]byte, error) {
	img := picture(m)
	if m.embed == embedPNG {
		var buf bytes.Buffer
		enc := stdpng.Encoder{CompressionLevel: stdpng.DefaultCompression}
		if err := enc.Encode(&buf, img); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}
	return deviceIndependentBitmap(img), nil
}

func picture(m memo) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, m.width, m.height))
	off := int(m.seed % 256)
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
	if m.label != "" && imagelabel.Fits(m.width, len(m.label)) {
		imagelabel.Draw(img, m.label)
	}
	return img
}

// deviceIndependentBitmap lays the picture out the way an icon wants it.
//
// Two things about this shape catch people out and both are the format, not a
// choice. The header states twice the real height, because it counts the
// colour rows and the transparency mask together. And the mask is present even
// when nothing is transparent, one bit per pixel with each row rounded up to
// four bytes.
func deviceIndependentBitmap(img *image.RGBA) []byte {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()

	colourBytes := w * 4 * h
	maskStride := (w + 31) / 32 * 4
	maskBytes := maskStride * h

	out := make([]byte, infoHeader+colourBytes+maskBytes)

	binary.LittleEndian.PutUint32(out[0:4], infoHeader)
	binary.LittleEndian.PutUint32(out[4:8], uint32(int32(w)))
	binary.LittleEndian.PutUint32(out[8:12], uint32(int32(h*2)))
	binary.LittleEndian.PutUint16(out[12:14], 1)
	binary.LittleEndian.PutUint16(out[14:16], 32)
	// Compression stays zero, which means none.
	binary.LittleEndian.PutUint32(out[20:24], uint32(colourBytes+maskBytes))

	// Bottom up, and blue first, as every bitmap is.
	at := infoHeader
	for y := h - 1; y >= 0; y-- {
		for x := 0; x < w; x++ {
			i := img.PixOffset(x, y)
			out[at] = img.Pix[i+2]
			out[at+1] = img.Pix[i+1]
			out[at+2] = img.Pix[i]
			out[at+3] = img.Pix[i+3]
			at += 4
		}
	}
	// The mask stays all zeros: every pixel is opaque.
	return out
}

// minimumBytes is the smallest file this generator can produce with nothing
// set: a one pixel icon holding a bitmap, no label, no padding.
//
// Asking for a PNG inside raises this floor, because a PNG carries its own
// signature and chunks where a bitmap of one pixel is a header and four bytes.
// The refusal says so when it happens, rather than this number pretending to
// cover both.
func minimumBytes() int64 {
	blob, err := build(memo{width: 1, height: 1, embed: embedBMP})
	if err != nil {
		return 1 << 62
	}
	return int64(preamble) + int64(len(blob))
}
