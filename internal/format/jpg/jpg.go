// Package jpg generates JPEG images.
//
// The first lossy format. What comes back out of a decoder is not what went
// in, and that is the format working as designed rather than a fidelity we
// gave up - so D4 is judged on whether the picture reads, and D11 on whether
// the bytes repeat. Both were measured before this file existed: three
// separate processes encode the same picture to the same digest
// (MVP-FORMATS.md 2.10).
//
// Like PNG, the encoded size does not follow from the content, so the exact
// size comes from padding. Unlike PNG, the padding goes at the FRONT and may
// need several segments, because one comment carries 65533 bytes at most.
package jpg

import (
	"context"
	"fmt"
	"image"
	"image/color"
	stdjpeg "image/jpeg"
	"io"
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/imagelabel"
)

const (
	generatorVersion = "1"

	// soiSize is the start of image marker, which is always the first two
	// bytes and is what the padding is inserted after.
	soiSize = 2

	// comOverhead is what one comment segment costs beyond its payload: the
	// marker FF FE and a two byte length. The length field counts itself,
	// which is why the payload stops four bytes short of 65535 rather than
	// two.
	comOverhead = 4

	// maxCOMPayload is the most one comment segment can carry.
	maxCOMPayload = 65533

	// comStep is what one full segment adds to the file. Used to work out how
	// many segments a given amount of padding needs.
	comStep = maxCOMPayload + comOverhead

	// defaultWidth and defaultHeight are used when the recipe names no size
	// for the picture. Small enough that encoding costs little, large enough
	// that the label is readable.
	defaultWidth  = 640
	defaultHeight = 480

	minDimension = 1
	maxDimension = 20000

	minQuality     = 1
	maxQuality     = 100
	defaultQuality = 90

	// The picture is built in memory as one buffer and encoded twice, once
	// while planning and once while writing. The same budget as PNG, and for
	// the same reason: it refuses what would otherwise be an out of memory
	// error with no explanation.
	maxPixels = 40_000_000
)

func init() {
	format.Register(format.Descriptor{
		ID:          "jpg",
		Extension:   ".jpg",
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,

		// The smallest JPEG this generator can produce. Measured rather than
		// guessed - the document estimated 100 to 160 B and the real number is
		// several times that, because a JPEG carries its quantisation tables
		// and four standard Huffman tables whatever the picture is. At one
		// pixel those tables are the whole file.
		MinBytes: declaredMinimumBytes,

		Padding: format.PaddingChannel{
			// Measured against five independent readers - Pillow, the Windows
			// Imaging Component, GDI+, exiftool and ffprobe - at every size
			// from one byte to 10 MiB, odd sizes and the segment boundary
			// included. All five read the image and return identical pixels.
			//
			// Four other places also passed every reader: a comment before
			// the scan, an APP15 segment, and raw bytes after the end of
			// image marker either bare or wrapped in a comment. A comment is
			// chosen because it is the one the format provides for arbitrary
			// text - bytes after the end marker are outside the image, and a
			// validator written to the specification is entitled to call
			// those rubbish. It sits after the start marker rather than
			// before the scan so its position does not depend on how many
			// table segments the encoder emits.
			//
			// Tk 9.0.4 is not on that list of readers and that is measured
			// too: it has no JPEG decoder at all.
			Name:  "comment segment after the start of image marker",
			Where: format.PlacementStart,
			// No limit of its own. One segment carries 65533 B and this
			// generator writes as many as the size needs.
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
			{
				Name: "quality", Kind: format.PropertyInt,
				Min: minQuality, Max: maxQuality, Default: strconv.Itoa(defaultQuality),
				Detail: "How much detail survives compression, from 1 to 100. JPEG throws detail away by design, so a low number gives a visibly softer picture. It changes how large the picture encodes to, not the size of the file you asked for.",
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
	quality       int
	seed          uint64
	label         string
	// body is the exact number of bytes the encoded picture takes, start
	// marker included. Worked out during planning so that a size this format
	// cannot reach is refused before any file exists.
	body int64
	// padPayload is how many bytes of padding the comment segments carry
	// between them, not counting their headers.
	padPayload int64
	// segments is how many comment segments carry it, and it is never zero -
	// this generator always writes at least one, which is what removes the
	// unreachable band every other format has above its floor.
	segments int
}

// checkJointLimits asks the registry's own declaration about a pair of
// dimensions, rather than repeating the rule here. Same reasoning as PNG: the
// refusal, the sentence "tfg formats jpg" prints and the field a window would
// draw then all come from one line.
func checkJointLimits(w, h int) error {
	d, err := format.Get("jpg")
	if err != nil {
		return err
	}
	for _, j := range d.JointLimits {
		if bad := j.Allows(int64(w), int64(h)); bad != "" {
			return &format.PropertyValueError{
				Format: "jpg", Key: j.Of + " and " + j.By,
				Value:  fmt.Sprintf("%dx%d", w, h),
				Reason: bad + fmt.Sprintf(". Each side may go up to %d, but not both at once - ask for a smaller pair", maxDimension),
			}
		}
	}
	return nil
}

// segmentsFor is how many comment segments are needed to add exactly extra
// bytes to the file, and how much payload they carry between them.
//
// Each segment costs four bytes of header and carries up to 65533, so k
// segments can add anything from 4k up to k*65537. Taking the smallest k that
// reaches extra leaves a payload that always fits, and because k grows one at
// a time there is no gap: one segment covers 4 to 65537 and two cover 8 to
// 131074, which overlap.
func segmentsFor(extra int64) (segments int, payload int64) {
	k := int((extra + comStep - 1) / comStep)
	if k < 1 {
		k = 1
	}
	return k, extra - int64(k)*comOverhead
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	label := ""
	if r.Label {
		label = core.Label("jpg", r.Bytes, r.Seed)
	}

	quality, err := qualityOf(r.Properties)
	if err != nil {
		return format.Plan{}, err
	}

	m, err := chooseSize(r, label, quality)
	if err != nil {
		return format.Plan{}, err
	}
	w, h := m.width, m.height
	bare := m.body

	p := format.Plan{
		Bytes:       r.Bytes,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties: map[string]any{
			"width":       w,
			"height":      h,
			"quality":     quality,
			"progressive": false,
			"subsampling": "4:4:4",
		},
	}

	// The floor is a constant rather than this picture's own size, so that the
	// number the tool prints is the number it takes whatever the seed is. See
	// minimumBytes. Below it nothing is produced. At it and above it, every
	// single byte count is reachable, because the comment segment is always
	// present and its payload grows one at a time.
	floor := declaredMinimum()
	if floor < bare+comOverhead {
		// A picture larger than one pixel was asked for by name, so the floor
		// that applies is its own rather than the format's.
		floor = bare + comOverhead
	}
	// The gate is the floor, NOT this seed's own bare size plus a segment.
	// Measured while writing this: with the gate on the seed, 601 B was
	// accepted for one seed while the tool printed 602 B as its minimum - the
	// number a run takes has to be the number the tool announces, whatever the
	// seed, and that is a row on the regression surface.
	if r.Bytes < floor {
		return format.Plan{}, &format.BelowMinimumError{
			Format:    "JPG",
			Requested: r.Bytes,
			Minimum:   floor,
			Reason: fmt.Sprintf(
				"a %dx%d picture at quality %d encodes to %d B and always carries a comment segment, which costs %d B even when empty",
				w, h, quality, bare, comOverhead),
			Hint: fmt.Sprintf("Ask for %d B or more, or set a smaller width and height, or a lower quality", floor),
		}
	}
	m.segments, m.padPayload = segmentsFor(r.Bytes - bare)

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
		return fmt.Errorf("jpg: the plan was not produced by this generator")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// The start marker goes out first, then the padding, then the rest of the
	// encoded picture. Nothing is buffered: the encoder streams straight
	// through once its own two byte marker has been dropped.
	if _, err := w.Write([]byte{0xFF, 0xD8}); err != nil {
		return err
	}
	if err := writeComments(ctx, w, m.seed, m.segments, m.padPayload); err != nil {
		return err
	}

	skip := &headSkipper{w: w, keep: soiSize}
	if err := encode(skip, m); err != nil {
		return err
	}
	if err := skip.check(); err != nil {
		return err
	}
	if skip.written+soiSize != m.body {
		return fmt.Errorf("jpg: the picture encoded to %d B where planning said %d B",
			skip.written+soiSize, m.body)
	}
	return nil
}

// sizeLadder is tried from the largest down when the recipe names no picture
// size. The first rung that leaves room for the padding wins, so a small file
// gets a small picture instead of being refused.
var sizeLadder = [][2]int{
	{640, 480}, {320, 240}, {160, 120}, {80, 60},
	{40, 30}, {20, 15}, {8, 6}, {4, 3}, {2, 2}, {1, 1},
}

// chooseSize settles the picture size and measures what it encodes to.
func chooseSize(r format.Request, label string, q int) (memo, error) {
	_, wSet := r.Properties["width"]
	_, hSet := r.Properties["height"]

	if wSet || hSet {
		w, err := dimension(r.Properties, "width", defaultWidth)
		if err != nil {
			return memo{}, err
		}
		h, err := dimension(r.Properties, "height", defaultHeight)
		if err != nil {
			return memo{}, err
		}
		if err := checkJointLimits(w, h); err != nil {
			return memo{}, err
		}
		m := memo{width: w, height: h, quality: q, seed: r.Seed, label: label}
		body, err := encodedSize(m)
		if err != nil {
			return memo{}, err
		}
		m.body = body
		return m, nil
	}

	var smallest memo
	for _, rung := range sizeLadder {
		m := memo{width: rung[0], height: rung[1], quality: q, seed: r.Seed, label: label}
		body, err := encodedSize(m)
		if err != nil {
			return memo{}, err
		}
		m.body = body
		smallest = m

		if r.Bytes >= body+comOverhead {
			return m, nil
		}
	}
	// Nothing fitted, not even one pixel. The caller turns this into the
	// error that names the minimum.
	return smallest, nil
}

func dimension(props map[string]string, key string, fallback int) (int, error) {
	raw, ok := props[key]
	if !ok || raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("jpg: %s must be a whole number of pixels, got %q", key, raw)
	}
	if n < minDimension || n > maxDimension {
		return 0, fmt.Errorf("jpg: %s must be between %d and %d pixels, got %d", key, minDimension, maxDimension, n)
	}
	return n, nil
}

func qualityOf(props map[string]string) (int, error) {
	raw, ok := props["quality"]
	if !ok || raw == "" {
		return defaultQuality, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("jpg: quality must be a whole number, got %q", raw)
	}
	if n < minQuality || n > maxQuality {
		return 0, fmt.Errorf("jpg: quality must be between %d and %d, got %d", minQuality, maxQuality, n)
	}
	return n, nil
}

// picture builds the image. Deterministic from the seed, and smooth, so that
// it compresses well and the encoded result stays small next to any realistic
// requested size.
func picture(m memo) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, m.width, m.height))
	// The seed shifts the gradient, so two seeds give visibly different
	// pictures rather than the same picture with different padding.
	off := int(m.seed % 256)
	// SetRGBA rather than Set, for the reason measured on PNG: the interface
	// version boxes a colour onto the heap for every pixel.
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
	// Draw decides for itself whether the label fits and does nothing when it
	// does not, so there is no condition here beyond having one at all. An
	// earlier version asked Fits first, which reads as caution and is dead
	// code: Fits IS scaleFor() > 0, and Draw returns on the same test. Proven
	// by removing it - not one golden value moved.
	if m.label != "" {
		imagelabel.Draw(img, m.label)
	}
	return img
}

func encode(w io.Writer, m memo) error {
	return stdjpeg.Encode(w, picture(m), &stdjpeg.Options{Quality: m.quality})
}

// encodedSize is how many bytes the picture takes on its own, start marker
// included. Encoding once during planning is what lets a size this format
// cannot reach be refused before any file exists.
func encodedSize(m memo) (int64, error) {
	c := &counter{}
	if err := encode(c, m); err != nil {
		return 0, err
	}
	return c.n, nil
}

// minimumBytes is the smallest size this generator accepts, and it is ONE
// NUMBER FOR EVERY SEED. That is the whole reason it is computed rather than
// measured once.
//
// This is the third time this project has met a floor that walks. PDF's
// walked with the seed on 2026-08-03 and OOXML's on 2026-08-19, and both were
// closed by making the varying part stop varying in LENGTH. JPEG cannot do
// that: the picture is seeded, a seeded picture differs by a pixel, and a
// changed pixel changes how many bytes the entropy coder emits. Measured on a
// one pixel picture the floor moved between 596 and 597 depending on the seed,
// which is enough to make the tool print a minimum it then refuses.
//
// So the floor is the WORST case rather than one sample. The picture varies
// with seed only through an offset taken modulo 256, so there are exactly 256
// pictures to try and the largest of them settles it for every seed there is.
//
// Four bytes are added because this generator always writes at least one
// comment segment, even an empty one. That costs the four bytes of its marker
// and length, and it buys something worth more: there is NO unreachable band
// above the minimum. Every format before this one has one - PNG cannot produce
// the eleven sizes above its floor - because they accept the bare file and
// then need a whole chunk to go one byte higher. Here the segment is always
// there and its payload grows one byte at a time.
//
// Quality is the default rather than the lowest, because the minimum has to be
// the floor of an ORDINARY run - a number reachable only by also passing
// quality=1 would be a promise the tool does not keep.
// declaredMinimum is minimumBytes computed once. Plan needs it on every
// refusal and it costs 256 encodes, which is fine at startup and not fine
// per request.
//
// A plain package variable rather than a once guard, because package
// variables are initialised before init runs and that is all this needed.
// The first version reached for sync.OnceValue and the concurrency guard
// stopped it, correctly: adding concurrency somewhere new is a decision,
// and there was no concurrency to manage here.
var declaredMinimumBytes = minimumBytes()

func declaredMinimum() int64 { return declaredMinimumBytes }

func minimumBytes() int64 {
	var worst int64
	for off := uint64(0); off < 256; off++ {
		n, err := encodedSize(memo{width: 1, height: 1, quality: defaultQuality, seed: off})
		if err != nil {
			// Encoding a single pixel cannot fail. If it somehow does,
			// refusing every size is safer than declaring a minimum we did
			// not measure.
			return 1 << 62
		}
		if n > worst {
			worst = n
		}
	}
	return worst + comOverhead
}

// padChunkSize is how much padding is built before each write. It also sets
// how often cancellation is noticed.
const padChunkSize = 32 * 1024

// writeComments emits the padding as segments, without ever holding it in
// memory. The payload is spread as evenly as the count allows, so every
// segment stays inside what its length field can describe.
func writeComments(ctx context.Context, w io.Writer, seed uint64, segments int, payload int64) error {
	if segments == 0 {
		return nil
	}
	rng := core.NewRand(seed)
	buf := make([]byte, padChunkSize)
	// Built once. Taking a slice of an array declared inside the loop puts it
	// on the heap every time round, which measured 1024 allocations for a
	// 64 MiB file against a ceiling of 128.
	head := [4]byte{0xFF, 0xFE, 0, 0}

	base := payload / int64(segments)
	extra := payload % int64(segments)

	for i := 0; i < segments; i++ {
		n := base
		if int64(i) < extra {
			n++
		}
		if n > maxCOMPayload {
			return fmt.Errorf("jpg: a comment segment of %d B was planned and the format allows %d", n, maxCOMPayload)
		}

		head[2] = byte((n + 2) >> 8)
		head[3] = byte((n + 2) & 0xFF)
		if _, err := w.Write(head[:]); err != nil {
			return err
		}

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
			for j := 0; j < len(chunk); j += 8 {
				var eight [8]byte
				v := rng.Uint64()
				for k := 0; k < 8; k++ {
					eight[k] = byte(v >> (8 * k))
				}
				copy(chunk[j:], eight[:])
			}
			if _, err := w.Write(chunk); err != nil {
				return err
			}
			remaining -= size
		}
	}
	return nil
}

// counter counts bytes and keeps none.
type counter struct{ n int64 }

func (c *counter) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	return len(p), nil
}

// headSkipper drops the first keep bytes and passes the rest through, so the
// encoder's own start marker does not follow the padding we already wrote.
//
// It also holds on to what it dropped, because a stream that does not begin
// with the start marker means the encoder changed under us, and writing the
// rest of it after our own marker would produce a file nobody could read.
type headSkipper struct {
	w       io.Writer
	keep    int
	head    []byte
	written int64
}

func (h *headSkipper) Write(p []byte) (int, error) {
	n := len(p)
	if len(h.head) < h.keep {
		take := h.keep - len(h.head)
		if take > len(p) {
			take = len(p)
		}
		h.head = append(h.head, p[:take]...)
		p = p[take:]
	}
	if len(p) == 0 {
		return n, nil
	}
	written, err := h.w.Write(p)
	h.written += int64(written)
	return n, err
}

func (h *headSkipper) check() error {
	if len(h.head) != soiSize || h.head[0] != 0xFF || h.head[1] != 0xD8 {
		return fmt.Errorf("jpg: the encoder did not begin with the start of image marker, got % x", h.head)
	}
	return nil
}
