// Package avif generates AVIF images: AV1 pictures in an ISO base media
// container.
//
// The first format in this tool whose pixels are coded by somebody else's
// encoder, and that is a decision with a measurement under it rather than a
// convenience. Every image format here so far was written by hand, because a
// borrowed encoder puts somebody else's release inside the byte stability
// contract (D11). AV1 is where that stops being affordable: the coefficient
// tables alone in rav1e, the implementation ours would have been written
// after, are fourteen times the size of this project's entire WebP encoder,
// and the whole of it is half again the size of this whole program. The
// comparison and the roads turned down are in docs/STACK.md section 4.2.1.
//
// The encoder is gen2brain/gav1d, which is AV1 written in Go and brings no
// module of its own with it, so the binary stays static, free of cgo (D10) and
// free of a socket (D16). It is pinned, and raising it moves bytes, which makes
// it the same kind of component as the Go compiler this project already pins -
// see the toolchain lines in go.mod.
//
// It is built with the tag in .github/build-tags, and that is not a preference:
// the encoder's AVX2 path reads past the end of a buffer and takes the process
// down on some picture sizes. Measured, along with the fact that the bytes come
// out the same either way, in docs/STACK.md section 4.2.1.
//
// One thing follows from not owning the encoder. What a picture codes to cannot
// be worked out with arithmetic the way BMP, TIFF and WebP work theirs out, so
// the picture is chosen from a ladder of sizes, each carrying a measured
// ceiling. Planning stays arithmetic and the coding happens while writing,
// which is what keeps a preview of a large run cheap.
//
// The padding channel is a free box, which is the box the container defines
// for space that means nothing. It goes after everything the encoder wrote,
// takes any length at all, and needs no second stage - measured on 2026-08-29
// at 0, 1, 7, 100 and 101 bytes, and again with two boxes, four boxes and the
// sixty four bit box length that files above four gigabytes need. There is no
// dead zone above the minimum. Pillow reads every one of them and refuses a
// truncated or corrupted file, which is what makes it a witness. ffprobe reads
// the broken ones too, so it is not.
package avif

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand/v2"
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

const (
	generatorVersion = "1"

	// boxHeader is what every box costs before its content: a four byte
	// length and a four character name.
	boxHeader = 8

	// maxFreePayload keeps a single box inside the four byte length field the
	// container uses by default. Anything larger is spread over several boxes,
	// which is measured to work rather than assumed.
	maxFreePayload = 1<<31 - boxHeader

	minDimension = 1
	maxDimension = 16384

	// maxPixels bounds the picture because the encoder holds all of it while
	// it works, and it is the same number PNG, JPG and GIF use. It refuses the
	// picture that would otherwise end as an out of memory error with nothing
	// said about why.
	//
	// It stood at two million for part of a day, and that number was wrong in
	// a way worth recording: it came from measuring a DIFFERENT encoder, one
	// that ran the codec as WebAssembly and cost about 200 B of heap per pixel.
	// This one costs 7.6 to 8. The number outlived the thing it described, and
	// what made it visible was somebody asking whether people would want Full
	// HD - which is 2 073 600 pixels, so the old ceiling refused it by 73 600.
	//
	// Measured on 2026-08-29 with the encoder that is actually here: 1080p
	// costs 16 MB and 266 ms, 4K costs 63 MB and 1.06 s, 8K costs 253 MB and
	// 4.7 s. All of them encode, none of them fail.
	maxPixels = 40_000_000

	// minimumBytes is the smallest AVIF this generator produces, and it is one
	// number for every seed rather than one sample.
	//
	// The picture varies with the seed through an offset taken modulo 256, and
	// a changed pixel changes how many bytes the coder emits, so a floor taken
	// from one seed is a number the tool would print and then refuse. There
	// are exactly 256 pictures at one pixel, so the largest of them settles it
	// for every seed there is - measured across all of them on 2026-08-29, and
	// it ranges from 308 to this. Frozen here rather than computed at startup,
	// because 256 encodes would be paid by every run of the program including
	// the ones that only ask for help. The guard that sweeps seeds against the
	// announced minimum is what reddens if this ever stops being the worst
	// case, which is also what would catch a codec raised underneath us.
	//
	// The free box header is included, because this generator always writes
	// one even when it carries nothing. That is what removes the unreachable
	// band above the floor that PNG and GIF both have.
	minimumBytes = 311
)

func init() {
	format.Register(format.Descriptor{
		ID:          "avif",
		Extension:   ".avif",
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,

		MinBytes: minimumBytes,

		Padding: format.PaddingChannel{
			// Measured on files this encoder produces, not on a container
			// built by hand, and with the pixels compared rather than only
			// decoded. Pillow and ffprobe both read every padded file. The
			// negative control is the half that matters: Pillow refuses a
			// truncated file and a corrupted payload, so it can say no, while
			// ffprobe accepts both and therefore witnesses nothing.
			Name:     "a free box after the picture",
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
			{
				Name: "quality", Kind: format.PropertyInt,
				Min: minQuality, Max: maxQuality, Default: strconv.Itoa(defaultQuality),
				Detail: "How much detail the picture keeps. Higher looks better and takes more of the file, leaving less room for padding.",
			},
		},
		JointLimits: []format.JointLimit{{
			Of: "width", By: "height", Max: maxPixels,
			Unit: "megapixels", Per: 1_000_000,
			Why: "the encoder holds the whole picture in memory while it works",
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

	// coded is the file the encoder produced, and it is filled in only when the
	// recipe named the picture size. Left out, the picture is chosen from the
	// ladder without coding it and the work happens while writing.
	coded []byte
	// total is the size the file has to come to, so writing can work out how
	// much free box follows the picture. There is always at least one, empty,
	// which is what makes every size above the minimum reachable a byte at a
	// time.
	total int64
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	label := ""
	if r.Label {
		label = core.Label("avif", r.Bytes, r.Seed)
	}

	q, err := quality(r.Properties)
	if err != nil {
		return format.Plan{}, err
	}

	m, err := chooseSize(r, label, q)
	if err != nil {
		return format.Plan{}, err
	}

	if err := checkFits(r.Bytes, m); err != nil {
		return format.Plan{}, err
	}
	m.total = r.Bytes

	p := format.Plan{
		Bytes:       r.Bytes,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties: map[string]any{
			"width":       m.width,
			"height":      m.height,
			"quality":     m.quality,
			"compression": "av1",
			"bit_depth":   8,
			"animated":    false,
			"frame_count": 1,
		},
	}

	carries := labelled(m.width, label)
	if r.Label && !carries {
		p.Notes = append(p.Notes, format.Note{
			Code: "label_omitted",
			Detail: fmt.Sprintf(
				"The picture is %d px wide and the label needs more room, so this file carries no visible label. Its name and the manifest still identify it.",
				m.width),
		})
	}
	p.Properties[format.PropertyLabelEmbedded] = carries
	p.Memo = m
	return p, nil
}

// checkFits refuses a request the format cannot answer.
//
// The gate is the declared floor, not this seed's own encoded size, so the
// number the tool announces is the number it accepts whatever the seed. A
// picture named by hand can be larger than the smallest one, and then its own
// size is the floor that applies.
func checkFits(want int64, m memo) error {
	floor := int64(minimumBytes)
	reason := fmt.Sprintf(
		"the smallest picture this format draws codes to %d B at worst, and the file always carries a free box, which costs %d B even when it holds nothing",
		minimumBytes-boxHeader, boxHeader)

	// A picture named by hand can be larger than the smallest one, and then its
	// own size is the floor that applies - which is known here, because naming
	// a size is what makes planning code the picture.
	if m.coded != nil {
		if own := int64(len(m.coded)) + boxHeader; own > floor {
			floor = own
			reason = fmt.Sprintf(
				"a %dx%d picture at quality %d codes to %d B, and the file always carries a free box, which costs %d B even when it holds nothing",
				m.width, m.height, m.quality, len(m.coded), boxHeader)
		}
	}

	if want >= floor {
		return nil
	}
	return &format.BelowMinimumError{
		Format:    "AVIF",
		Requested: want,
		Minimum:   floor,
		Reason:    reason,
		Hint:      fmt.Sprintf("Ask for %d B or more, or set a smaller width and height, or a lower quality", floor),
	}
}

// sizeLadder is tried from the largest down when the recipe names no picture
// size, so a small file gets a small picture instead of being refused.
//
// It starts at 640x480, the same top rung JPG uses, so the two formats answer
// the same request with the same sized picture. Measured on 2026-08-29 with the
// encoder that is here: that rung costs about 39 ms against about 9 ms for the
// one below it, and only files large enough to hold it ever pay that.
//
// It stops there rather than climbing to Full HD, and that is measured too:
// 1920x1080 costs 266 ms, so a run of ten thousand large files would spend
// three quarters of an hour on pictures nobody asked to be that big. Anybody
// who does want them says so - width and height are settings, and 40 megapixels
// is the ceiling, which covers 4K and 8K.
//
// Ceiling is the LARGEST file that rung has ever been seen to produce, the free
// box included, and it is what lets planning choose a picture without coding
// one. Measured on 2026-08-29 across all 256 seeds, four orders of magnitude of
// requested size - the label carries the byte count, so its length moves with
// it - and with the label on and off.
//
// A ceiling rather than a floor, and the difference is the whole point.
// Planning promises an exact size, so the rung it picks has to be one whose
// picture is CERTAIN to leave room for the padding. Set too high, a ceiling
// costs a smaller picture than the file had room for and nothing else. Set too
// low, it would let planning pick a picture that does not fit, so writing
// checks and says so rather than trusting this table.
var sizeLadder = []rung{
	{640, 480, 16389}, {320, 240, 6792}, {256, 256, 4890}, {160, 120, 3392},
	{80, 60, 832}, {40, 30, 549}, {20, 15, 448}, {8, 6, 348},
	{4, 3, 344}, {2, 2, 327}, {1, 1, minimumBytes},
}

type rung struct {
	width, height int
	ceiling       int64
}

// chooseSize settles the picture size and encodes it.
func chooseSize(r format.Request, label string, q int) (memo, error) {
	_, wSet := r.Properties["width"]
	_, hSet := r.Properties["height"]
	if wSet || hSet {
		return namedSize(r, label, q)
	}

	// The ceilings were measured at the default quality and speak for nothing
	// else, so a run that asked for a different one takes the slow road: code
	// each step until one fits. Measured on 2026-08-29, which is how this was
	// found - quality 100 at 20 kB picked 640x480 off the table and produced
	// no file at all, because that picture is several times the ceiling.
	if _, named := r.Properties["quality"]; named && q != defaultQuality {
		return measuredSize(r, label, q)
	}

	// The largest rung certain to leave room for the padding. Nothing is coded
	// here, which is what keeps a preview of ten thousand files as cheap as it
	// is for every other format.
	pick := sizeLadder[len(sizeLadder)-1]
	for _, step := range sizeLadder {
		if r.Bytes >= step.ceiling {
			pick = step
			break
		}
	}
	return memo{width: pick.width, height: pick.height, quality: q, seed: r.Seed, label: label}, nil
}

// measuredSize walks the ladder coding each step, and takes the first that
// leaves room for the padding. Only reached when the request named a quality
// the ceilings were not measured at.
func measuredSize(r format.Request, label string, q int) (memo, error) {
	var smallest memo
	for _, step := range sizeLadder {
		m, err := coded(memo{width: step.width, height: step.height, quality: q, seed: r.Seed, label: label})
		if err != nil {
			return memo{}, err
		}
		smallest = m
		if r.Bytes >= int64(len(m.coded))+boxHeader {
			return m, nil
		}
	}
	// Nothing fitted, not even one pixel. checkFits turns this into the
	// refusal that names the minimum.
	return smallest, nil
}

// namedSize handles a recipe that asked for a picture of its own size.
func namedSize(r format.Request, label string, q int) (memo, error) {
	w, err := dimension(r.Properties, "width", sizeLadder[0].width)
	if err != nil {
		return memo{}, err
	}
	h, err := dimension(r.Properties, "height", sizeLadder[0].height)
	if err != nil {
		return memo{}, err
	}
	if err := checkJointLimits(w, h); err != nil {
		return memo{}, err
	}
	return coded(memo{width: w, height: h, quality: q, seed: r.Seed, label: label})
}

// coded fills in the encoded bytes for a picture that has been settled.
func coded(m memo) (memo, error) {
	b, err := encode(picture(m), m.quality)
	if err != nil {
		return memo{}, err
	}
	m.coded = b
	return m, nil
}

// checkJointLimits asks the registry's own declaration about a pair of
// dimensions, so the refusal, the sentence tfg formats avif prints and the
// field a window draws all come from one line.
func checkJointLimits(w, h int) error {
	d, err := format.Get("avif")
	if err != nil {
		return err
	}
	for _, j := range d.JointLimits {
		if bad := j.Allows(int64(w), int64(h)); bad != "" {
			return &format.PropertyValueError{
				Format: "avif", Key: j.Of + " and " + j.By,
				Value:  fmt.Sprintf("%dx%d", w, h),
				Reason: bad + ". Ask for a smaller pair",
			}
		}
	}
	return nil
}

func dimension(props map[string]string, key string, fallback int) (int, error) {
	raw, ok := props[key]
	if !ok || raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("avif: %s must be a whole number of pixels, got %q", key, raw)
	}
	if n < minDimension || n > maxDimension {
		return 0, fmt.Errorf("avif: %s must be between %d and %d pixels, got %d", key, minDimension, maxDimension, n)
	}
	return n, nil
}

func quality(props map[string]string) (int, error) {
	raw, ok := props["quality"]
	if !ok || raw == "" {
		return defaultQuality, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("avif: quality must be a whole number, got %q", raw)
	}
	if n < minQuality || n > maxQuality {
		return 0, fmt.Errorf("avif: quality must be between %d and %d, got %d", minQuality, maxQuality, n)
	}
	return n, nil
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(memo)
	if !ok {
		return fmt.Errorf("avif: the plan was not produced by this generator")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	coded := m.coded
	if coded == nil {
		// Planning chose the picture without coding it, so this is where the
		// work actually happens.
		b, err := encode(picture(m), m.quality)
		if err != nil {
			return err
		}
		coded = b
	}

	pad := m.total - int64(len(coded))
	if pad < boxHeader {
		// The ladder's ceiling for this picture was too low. Said out loud
		// rather than written short, because a short file is a wrong file.
		return fmt.Errorf("avif: a %dx%d picture coded to %d B and the file was to be %d B, which leaves no room for the free box every one of these carries",
			m.width, m.height, len(coded), m.total)
	}

	if _, err := w.Write(coded); err != nil {
		return err
	}
	return writePadding(ctx, w, m.seed, pad)
}

// writePadding fills the rest of the file with free boxes, without ever
// holding their content.
//
// Several boxes rather than one when the padding is larger than a box length
// can say. The step below keeps the leftover from landing between one and
// seven bytes, which no box could then carry.
func writePadding(ctx context.Context, w io.Writer, seed uint64, total int64) error {
	rng := core.NewRand(seed)
	buf := make([]byte, 32*1024)

	for total > 0 {
		payload := nextPayload(total)
		if err := writeBoxHeader(w, payload); err != nil {
			return err
		}
		if err := writeFiller(ctx, w, buf, rng, payload); err != nil {
			return err
		}
		total -= boxHeader + payload
	}
	return nil
}

// nextPayload is how much filler the next free box carries.
//
// A box states its length in four bytes, so padding larger than that is spread
// over several boxes. The step down is what keeps the leftover from landing
// between one and seven bytes, which no box could then carry.
func nextPayload(total int64) int64 {
	payload := total - boxHeader
	if payload <= maxFreePayload {
		return payload
	}
	payload = maxFreePayload
	if total-boxHeader-payload < boxHeader {
		payload -= boxHeader
	}
	return payload
}

func writeBoxHeader(w io.Writer, payload int64) error {
	var b [boxHeader]byte
	binary.BigEndian.PutUint32(b[:4], uint32(boxHeader+payload))
	copy(b[4:], "free")
	_, err := w.Write(b[:])
	return err
}

func writeFiller(ctx context.Context, w io.Writer, buf []byte, rng *rand.Rand, size int64) error {
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
		fillBytes(buf[:n], rng)
		if _, err := w.Write(buf[:n]); err != nil {
			return err
		}
		left -= n
	}
	return nil
}

func fillBytes(b []byte, rng *rand.Rand) {
	for i := 0; i < len(b); i += 8 {
		v := rng.Uint64()
		for j := 0; j < 8 && i+j < len(b); j++ {
			b[i+j] = byte(v >> (8 * uint(j)))
		}
	}
}
