// Package gif generates GIF images.
//
// A compressed format, so the size of the picture is whatever LZW decides and
// the last bytes come from a padding channel - the same shape as PNG. The
// channel here is the comment extension, a block the format sets aside for
// arbitrary content.
//
// It has one awkward property PNG does not, and it is measured rather than
// guessed: an empty comment costs three bytes and a comment carrying a single
// byte costs five, because a sub block pays for its own length. Sizes one,
// two and four bytes above the bare picture are therefore unreachable, and
// the generator says so instead of rounding.
package gif

import (
	"context"
	"fmt"
	"image"
	"image/color"
	stdgif "image/gif"
	"io"
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/imagelabel"
)

const (
	generatorVersion = "1"

	// trailerSize is the single byte that closes a GIF data stream.
	trailerSize = 1

	// An empty comment extension is the introducer, the label, and the block
	// terminator: 0x21 0xFE 0x00.
	emptyComment = 3

	// The smallest comment that carries anything: the three bytes above plus
	// a sub block, which is its own length byte and one byte of content.
	smallestCarryingComment = 5

	// A sub block holds at most 255 bytes and pays one byte for its length,
	// so 256 bytes of file buy 255 bytes of padding.
	subBlockMax     = 255
	subBlockCost    = subBlockMax + 1
	labelBackground = 0
	labelInk        = 1
	// reservedSlots is how many palette entries the label keeps for itself.
	reservedSlots = 2
	// maxGradient is how many shades are left when the table is full.
	maxGradient = 256 - reservedSlots

	minDimension = 1
	maxDimension = 20000

	// The picture is held in memory while it is encoded, one byte per pixel
	// plus the encoder's own working set. The same budget as PNG, which has
	// the same shape of cost.
	maxPixels = 40_000_000
)

func init() {
	format.Register(format.Descriptor{
		ID:          "gif",
		Extension:   ".gif",
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,

		MinBytes: minimumBytes(),

		Padding: format.PaddingChannel{
			// Measured against six independent readers - Pillow, the Windows
			// Imaging Component, GDI+, exiftool, ffprobe and Tk 9.0 - at every
			// size from one byte to 100 MiB, odd sizes included. All six read
			// the image and return identical pixels, and none of them minds
			// content outside printable ASCII.
			//
			// Bytes after the closing trailer are accepted just as widely, and
			// are not used: nothing in the format describes them, while a
			// comment is a block the format put there for this.
			Name:     "the comment extension",
			Where:    format.PlacementEnd,
			Capacity: 0,
		},
		Label:  format.LabelVisible,
		Oracle: "pillow",
		Properties: []format.Property{
			{
				Name: "width", Kind: format.PropertyInt,
				Min: minDimension, Max: maxDimension, Unit: "pixels",
				Detail: "How wide the picture is. Left out, a size is chosen that fits the bytes you asked for.",
			},
			{
				Name: "height", Kind: format.PropertyInt,
				Min: minDimension, Max: maxDimension, Unit: "pixels",
				Detail: "How tall the picture is. Left out, a size is chosen that fits the bytes you asked for.",
			},
		},
		JointLimits: []format.JointLimit{{
			Of: "width", By: "height", Max: maxPixels,
			Unit: "megapixels", Per: 1_000_000,
			Why: "the picture is held in memory while it is encoded",
		}},
		GeneratorVersion: generatorVersion,
		Generator:        generator{},
	})
}

type generator struct{}

type memo struct {
	width, height int
	seed          uint64
	label         string
	// body is the encoded picture up to but not including the trailer.
	body int64
	// payload is how many bytes of filler the comment carries, and blocks how
	// many sub blocks carry them. Both zero means no comment at all.
	payload int64
	blocks  int64
	comment bool
}

func checkJointLimits(w, h int) error {
	d, err := format.Get("gif")
	if err != nil {
		return err
	}
	for _, j := range d.JointLimits {
		if bad := j.Allows(int64(w), int64(h)); bad != "" {
			return &format.PropertyValueError{
				Format: "gif", Key: j.Of + " and " + j.By,
				Value:  fmt.Sprintf("%dx%d", w, h),
				Reason: bad + fmt.Sprintf(". Each side may go up to %d, but not both at once - ask for a smaller pair", maxDimension),
			}
		}
	}
	return nil
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	label := ""
	if r.Label {
		label = core.Label("gif", r.Bytes, r.Seed)
	}

	m, err := chooseSize(r, label)
	if err != nil {
		return format.Plan{}, err
	}
	w, h := m.width, m.height
	bare := m.body + trailerSize

	p := format.Plan{
		Bytes:       r.Bytes,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties: map[string]any{
			"width":       w,
			"height":      h,
			"palette":     paletteSize(w, h),
			"animated":    false,
			"frame_count": 1,
		},
	}

	if err := settlePadding(&m, r.Bytes, bare); err != nil {
		return format.Plan{}, err
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

// settlePadding decides what the comment has to carry, or refuses a size no
// arrangement of comments can produce.
func settlePadding(m *memo, want, bare int64) error {
	delta := want - bare
	switch {
	case delta == 0:
		m.comment = false
		return nil

	case delta == emptyComment:
		// An empty comment is legal and costs exactly three bytes.
		m.comment = true
		return nil

	case delta < 0:
		return &format.BelowMinimumError{
			Format:    "GIF",
			Requested: want,
			Minimum:   bare,
			Reason:    fmt.Sprintf("a %dx%d picture already encodes to that much before any padding", m.width, m.height),
			Hint:      fmt.Sprintf("Ask for %d B or more, or set a smaller width and height", bare),
		}

	case delta < smallestCarryingComment:
		// Three byte counts between the two are unreachable. An empty comment
		// costs three bytes and the smallest one carrying anything costs
		// five, so bare+1, bare+2 and bare+4 have nothing that can produce
		// them. bare+3 is reachable and is offered as the nearer answer.
		return &format.BelowMinimumError{
			Format:    "GIF",
			Requested: want,
			Minimum:   bare + smallestCarryingComment,
			Reason: fmt.Sprintf(
				"a %dx%d picture encodes to exactly %d B, an empty comment adds %d B and the smallest comment that carries any padding adds %d B, so nothing else in between is reachable",
				m.width, m.height, bare, emptyComment, smallestCarryingComment),
			Hint: fmt.Sprintf("Ask for exactly %d B, exactly %d B, or %d B or more.",
				bare, bare+emptyComment, bare+smallestCarryingComment),
		}
	}

	m.comment = true
	m.blocks, m.payload = commentShape(delta)
	if m.payload < m.blocks {
		return fmt.Errorf(
			"gif: %d B of padding does not divide into %d sub blocks carrying %d B - this is a bug in the size arithmetic, not in the request",
			delta, m.blocks, m.payload)
	}
	return nil
}

// commentShape works out how to spend delta bytes on a comment extension.
//
// A comment costs three bytes of its own, then one byte per sub block plus
// whatever that sub block carries. So delta = 3 + payload + blocks, and the
// only freedom is how the payload is cut up. Using the fewest sub blocks that
// can hold it is what makes every delta from five upwards reachable - cutting
// always at 255 would miss delta 260, because one full sub block gives 259 and
// the next size up would be 261.
func commentShape(delta int64) (blocks, payload int64) {
	if delta == emptyComment {
		return 0, 0
	}
	blocks = (delta - emptyComment + subBlockCost - 1) / subBlockCost
	payload = delta - emptyComment - blocks
	return blocks, payload
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(memo)
	if !ok {
		return fmt.Errorf("gif: the plan was not produced by this generator")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// The trailer is always the final byte, so hold it back and let the rest
	// stream out. Nothing else is buffered.
	holder := &tailHolder{w: w, keep: trailerSize}
	if err := encode(holder, m); err != nil {
		return err
	}
	if holder.written != m.body {
		return fmt.Errorf("gif: the picture encoded to %d B where planning said %d B", holder.written, m.body)
	}
	if holder.tail[0] != 0x3B {
		return fmt.Errorf("gif: the encoded stream does not end with the trailer")
	}

	if m.comment {
		if err := writeComment(ctx, w, m.seed, m.blocks, m.payload); err != nil {
			return err
		}
	}

	_, err := w.Write(holder.tail)
	return err
}

// writeComment emits the comment extension without ever holding it in memory.
func writeComment(ctx context.Context, w io.Writer, seed uint64, blocks, payload int64) error {
	if _, err := w.Write([]byte{0x21, 0xFE}); err != nil {
		return err
	}

	rng := core.NewRand(seed)
	buf := make([]byte, subBlockMax+1)

	// The payload is spread as evenly as the sub blocks allow, so no block is
	// empty and none is over the limit.
	base, extra := int64(0), int64(0)
	if blocks > 0 {
		base, extra = payload/blocks, payload%blocks
	}
	for i := int64(0); i < blocks; i++ {
		if i%64 == 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}
		size := base
		if i < extra {
			size++
		}
		buf[0] = byte(size)
		chunk := buf[1 : 1+size]
		for j := 0; j < len(chunk); j += 8 {
			var eight [8]byte
			putUint64(eight[:], rng.Uint64())
			copy(chunk[j:], eight[:])
		}
		if _, err := w.Write(buf[:1+size]); err != nil {
			return err
		}
	}

	_, err := w.Write([]byte{0x00})
	return err
}

func putUint64(b []byte, v uint64) {
	b[0] = byte(v >> 56)
	b[1] = byte(v >> 48)
	b[2] = byte(v >> 40)
	b[3] = byte(v >> 32)
	b[4] = byte(v >> 24)
	b[5] = byte(v >> 16)
	b[6] = byte(v >> 8)
	b[7] = byte(v)
}

// sizeLadder is tried from the largest down when the recipe names no picture
// size, exactly as PNG does. The first rung that leaves a reachable remainder
// wins, so a small file gets a small picture instead of being refused.
var sizeLadder = [][2]int{
	{640, 480}, {320, 240}, {160, 120}, {80, 60},
	{40, 30}, {20, 15}, {8, 6}, {4, 3}, {2, 2}, {1, 1},
}

func chooseSize(r format.Request, label string) (memo, error) {
	_, wSet := r.Properties["width"]
	_, hSet := r.Properties["height"]

	if wSet || hSet {
		w, err := dimension(r.Properties, "width", 640)
		if err != nil {
			return memo{}, err
		}
		h, err := dimension(r.Properties, "height", 480)
		if err != nil {
			return memo{}, err
		}
		if err := checkJointLimits(w, h); err != nil {
			return memo{}, err
		}
		m := memo{width: w, height: h, seed: r.Seed, label: label}
		body, err := encodedBodySize(m)
		if err != nil {
			return memo{}, err
		}
		m.body = body
		return m, nil
	}

	var smallest memo
	for _, rung := range sizeLadder {
		m := memo{width: rung[0], height: rung[1], seed: r.Seed, label: label}
		body, err := encodedBodySize(m)
		if err != nil {
			return memo{}, err
		}
		m.body = body
		smallest = m

		bare := body + trailerSize
		if r.Bytes == bare || r.Bytes == bare+emptyComment || r.Bytes >= bare+smallestCarryingComment {
			return m, nil
		}
	}
	return smallest, nil
}

func dimension(props map[string]string, key string, fallback int) (int, error) {
	raw, ok := props[key]
	if !ok || raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("gif: %s must be a whole number of pixels, got %q", key, raw)
	}
	if n < minDimension || n > maxDimension {
		return 0, fmt.Errorf("gif: %s must be between %d and %d pixels, got %d", key, minDimension, maxDimension, n)
	}
	return n, nil
}

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

func encode(w io.Writer, m memo) error {
	return stdgif.Encode(w, picture(m), &stdgif.Options{NumColors: 256})
}

func encodedBodySize(m memo) (int64, error) {
	holder := &tailHolder{w: io.Discard, keep: trailerSize}
	if err := encode(holder, m); err != nil {
		return 0, err
	}
	return holder.written, nil
}

// minimumBytes is the smallest file this generator can produce: a one pixel
// picture with no label and no comment.
func minimumBytes() int64 {
	body, err := encodedBodySize(memo{width: 1, height: 1})
	if err != nil {
		return 1 << 62
	}
	return body + trailerSize
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
