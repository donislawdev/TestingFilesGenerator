// Package webp generates lossless WebP images.
//
// The third format whose size is arithmetic rather than whatever an encoder
// decides, after BMP and TIFF, and it is built that way on purpose: a request
// for 10 MB is answered with a picture worth 10 MB instead of a thumbnail
// followed by filler nobody can see. The bitstream lives in vp8l.go and says
// there why it compresses nothing.
//
// Written by hand rather than taken from a library, and the reason is
// measured rather than assumed: x/image/webp holds decode.go and doc.go and
// nothing else, so the ecosystem offers no encoder to take. Checked at v0.43.0
// and again at v0.45.0 on 2026-09-02, when the module was raised - a claim
// about what somebody else ships has to be re-read when their version moves,
// not carried across with the number changed. Pure Go
// encoders exist outside it, and taking one would have put somebody else's
// release inside the byte stability contract D11 - their next version would
// move the hashes in our users' test suites. See docs/STACK.md section 4.2.
//
// Lossless only. There is no quality setting and no lossy variant, because
// lossy WebP is VP8, which needs a transform, a quantiser and an arithmetic
// coder - out of proportion to what this format has to do here. That is a
// scope decision written down rather than a fidelity level quietly lowered,
// and tfg formats says so.
//
// The padding channel is two stages, and the second exists for a reason worth
// stating: every RIFF chunk block costs an EVEN number of bytes - eight of
// header, the payload, and a pad byte when the payload is odd. A file built
// only out of chunks can therefore only ever have an even length, and half of
// every size anybody could ask for would be unreachable. So the bulk goes in a
// private chunk, and one to seven bytes after the end of the RIFF payload
// carry whatever is left. Measured on six readers (docs/MVP-FORMATS.md section
// 2.13): there is no dead zone at all, which is better than PNG or GIF manage.
package webp

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/imagelabel"
)

const (
	generatorVersion = "1"

	// riffHeader is "RIFF", the size, and "WEBP".
	riffHeader = 12
	// chunkHeader is a four character name and a four byte size.
	chunkHeader = 4

	// paddingName is the private chunk the filler travels in. Unknown chunks
	// are what the container sets aside to be ignored, so this is the channel
	// the format itself describes.
	paddingName = "TFGp"

	// tailMax is how many bytes may sit after the RIFF payload. Seven is
	// everything a chunk cannot reach on its own, and no more - the bulk
	// belongs inside the container.
	tailMax = 7

	samplesPerPixel = 3

	minDimension = 1
	maxDimension = 16383 // the format stores each side less one in fourteen bits

	// A RIFF size is a four byte unsigned field.
	maxFileBytes = 1<<32 - 1

	// The tallest label band the rasteriser produces, so the rows carrying the
	// label are built once and the rest of the picture streams past.
	maxBandHeight = 24
)

func init() {
	format.Register(format.Descriptor{
		ID:          "webp",
		Extension:   ".webp",
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,

		MinBytes: bareBytes(minDimension, minDimension),

		Padding: format.PaddingChannel{
			// Measured against six independent readers - Pillow, ffprobe,
			// exiftool, x/image, the Windows Imaging Component and Chromium -
			// on files this generator writes, with the pixels compared rather
			// than only decoded.
			//
			// A chunk placed BEFORE the image chunk was tried and refused by
			// two of them, so it is not this. A negative control matters here
			// more than usual: a truncated WebP is accepted by ffprobe and by
			// the Windows Imaging Component, so those two cannot say no about
			// this format and the evidence rests on the four that can.
			Name:     "a private chunk, with the odd byte after the payload",
			Where:    format.PlacementEnd,
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
	// chunk is how many bytes of filler the private chunk carries, and tail
	// how many sit after the RIFF payload. A chunk of -1 means none at all,
	// which is not the same as one carrying nothing.
	chunk int64
	tail  int64
}

// bareBytes is the file with no padding of any kind.
func bareBytes(width, height int) int64 {
	stream := streamBytes(int64(width) * int64(height))
	return riffHeader + chunkHeader + 4 + stream + stream%2
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	label := ""
	if r.Label {
		label = core.Label("webp", r.Bytes, r.Seed)
	}

	w, h, err := chooseSize(r)
	if err != nil {
		return format.Plan{}, err
	}

	bare := bareBytes(w, h)
	if r.Bytes < bare {
		return format.Plan{}, &format.BelowMinimumError{
			Format:    "WEBP",
			Requested: r.Bytes,
			Minimum:   bare,
			Reason: fmt.Sprintf(
				"a %dx%d picture is %d B of pixels at three bytes each, and the container and the coding tables take another %d B",
				w, h, int64(w)*int64(h)*samplesPerPixel, bare-int64(w)*int64(h)*samplesPerPixel),
			Hint: fmt.Sprintf("Ask for %d B or more, or set a smaller width and height", bare),
		}
	}
	if r.Bytes > maxFileBytes {
		return format.Plan{}, &format.AboveMaximumError{
			Format:    "WEBP",
			Requested: r.Bytes,
			Maximum:   maxFileBytes,
			Reason:    "a WebP declares its length in a four byte field, so the format cannot describe a file this large",
			Hint:      "Ask for 4 GiB or less, or pick a format with no length field of its own such as gif.",
		}
	}

	m := memo{width: w, height: h, seed: r.Seed, label: label}
	settlePadding(&m, r.Bytes-bare)

	p := format.Plan{
		Bytes:       r.Bytes,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties: map[string]any{
			"width":       w,
			"height":      h,
			"compression": "lossless",
			"bit_depth":   24,
			"animated":    false,
			"frame_count": 1,
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

// settlePadding decides how the bytes above the bare picture are spent.
//
// Under eight there is no room for a chunk at all, so they go after the RIFF
// payload. From eight upwards the chunk carries an even payload and the tail
// carries nothing or one byte, which is what makes every size reachable - the
// chunk alone could only ever step in twos.
func settlePadding(m *memo, delta int64) {
	m.chunk = -1
	switch {
	case delta <= 0:
		m.tail = 0
	case delta <= tailMax:
		m.tail = delta
	default:
		m.tail = (delta - chunkHeader - 4) % 2
		m.chunk = delta - chunkHeader - 4 - m.tail
	}
}

// chooseSize settles the picture size.
//
// Named dimensions are used as given. Left out, the picture is grown to fill
// the request, because a pixel always costs three bytes here and the size is
// therefore arithmetic. What is left over goes into the padding.
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
			return w, fillHeight(r.Bytes, w), nil
		default:
			return fillWidth(r.Bytes, h), h, nil
		}
	}

	avail := (r.Bytes - bareBytes(minDimension, minDimension)) / samplesPerPixel
	if avail < 1 {
		return minDimension, minDimension, nil
	}
	// A square puts the most pixels into the request and is what a person
	// expects when they said nothing.
	side := int(isqrt(uint64(avail + 1)))
	if side < minDimension {
		side = minDimension
	}
	if side > maxDimension {
		side = maxDimension
	}
	for side > minDimension && bareBytes(side, side) > r.Bytes {
		side--
	}
	return side, side, nil
}

func fillHeight(bytes int64, width int) int {
	h := int64(minDimension)
	if width >= minDimension {
		room := bytes - bareBytes(width, minDimension)
		if room > 0 {
			h += room / (int64(width) * samplesPerPixel)
		}
	}
	if h > maxDimension {
		h = maxDimension
	}
	for h > minDimension && bareBytes(width, int(h)) > bytes {
		h--
	}
	return int(h)
}

func fillWidth(bytes int64, height int) int {
	w := int64(minDimension)
	if height >= minDimension {
		room := bytes - bareBytes(minDimension, height)
		if room > 0 {
			w += room / (int64(height) * samplesPerPixel)
		}
	}
	if w > maxDimension {
		w = maxDimension
	}
	for w > minDimension && bareBytes(int(w), height) > bytes {
		w--
	}
	return int(w)
}

// isqrt is the whole number square root, worked out rather than taken from
// floating point so the answer is the same on every machine.
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
		return 0, fmt.Errorf("webp: %s must be a whole number of pixels, got %q", key, raw)
	}
	if n < minDimension || n > maxDimension {
		return 0, fmt.Errorf("webp: %s must be between %d and %d pixels, got %d", key, minDimension, maxDimension, n)
	}
	return n, nil
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(memo)
	if !ok {
		return fmt.Errorf("webp: the plan was not produced by this generator")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// The length of the bitstream is arithmetic, so the headers can go out
	// before a single pixel is coded and nothing has to be held back.
	stream := streamBytes(int64(m.width) * int64(m.height))
	body := chunkHeader + 4 + stream + stream%2
	if m.chunk >= 0 {
		body += chunkHeader + 4 + m.chunk
	}

	if err := writeContainer(w, body, stream); err != nil {
		return err
	}

	written, err := writePicture(ctx, w, m)
	if err != nil {
		return err
	}
	// The chunk header above already promised this number. A mismatch would
	// leave a file every reader mistrusts, so it is an error rather than a
	// silent short write.
	if written != stream {
		return fmt.Errorf("webp: the bitstream came to %d B where the header promised %d B", written, stream)
	}
	if stream%2 == 1 {
		if err := writeAll(w, []byte{0}); err != nil {
			return err
		}
	}

	if m.chunk >= 0 {
		if err := writeFiller(ctx, w, paddingName, m.seed, m.chunk); err != nil {
			return err
		}
	}
	if m.tail > 0 {
		return writeAll(w, filler(m.seed+1, m.tail))
	}
	return nil
}

// writeContainer emits everything before the first coded pixel: the RIFF
// header, the form name, and the header of the chunk the bitstream goes in.
//
// It is a function of its own rather than five more error checks inside Write,
// because this project counts how many functions carry a lot of decisions and
// Write was the sixth to reach the band.
func writeContainer(w io.Writer, body, stream int64) error {
	for _, part := range []struct {
		text string
		num  uint32
	}{
		{text: "RIFF"},
		{num: uint32(4 + body)},
		{text: "WEBP"},
		{text: "VP8L"},
		{num: uint32(stream)},
	} {
		var err error
		if part.text != "" {
			err = writeAll(w, []byte(part.text))
		} else {
			err = writeUint32(w, part.num)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// writeFiller emits the padding chunk without holding its payload in memory.
func writeFiller(ctx context.Context, w io.Writer, name string, seed uint64, size int64) error {
	if err := writeAll(w, []byte(name)); err != nil {
		return err
	}
	if err := writeUint32(w, uint32(size)); err != nil {
		return err
	}
	rng := core.NewRand(seed)
	buf := make([]byte, 32*1024)
	for left := size; left > 0; {
		n := int64(len(buf))
		if left < n {
			n = left
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		core.FillRandomLE(buf[:n], rng)
		if err := writeAll(w, buf[:n]); err != nil {
			return err
		}
		left -= n
	}
	if size%2 == 1 {
		return writeAll(w, []byte{0})
	}
	return nil
}

func filler(seed uint64, size int64) []byte {
	out := make([]byte, size)
	core.FillRandomLE(out, core.NewRand(seed))
	return out
}

func writeUint32(w io.Writer, v uint32) error {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	return writeAll(w, b[:])
}

func writeAll(w io.Writer, b []byte) error {
	_, err := w.Write(b)
	return err
}
