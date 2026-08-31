// Package jxl generates JPEG XL images: a JPEG XL codestream in the container
// the format defines for it.
//
// The second format here whose pixels are coded by somebody else's encoder,
// after AVIF, and it is the same encoder author. The road was already surveyed
// once, so this one was walked in the order docs/STACK.md section 4.2.1 sets
// out: ask whether it encodes at all, whether it brings a module with it,
// whether it drags a socket into the command line binary, whether it is
// deterministic, and whether anything on this machine can REFUSE the format -
// because a reader that accepts a truncated file witnesses nothing.
//
// The encoder is gen2brain/jxl, which is JPEG XL written in Go with an empty
// go.mod and no cgo, so the binary stays static (D10) and free of a socket
// (D16). It is pinned, and raising it moves bytes.
//
// Its assembly was put through the sweep that found the crash in the AVIF
// encoder, because it is the same author and the same class of code. It came
// back clean: 240 picture sizes, three runs, no crash, and the bytes identical
// with the assembly and without it. This project builds with the tag in
// .github/build-tags anyway, for AVIF's sake, and that costs this format
// nothing.
//
// Two things follow from not owning the encoder.
//
// The first is that what a picture codes to cannot be worked out with
// arithmetic the way BMP, TIFF and WebP work theirs out, so the picture is
// chosen from a ladder of sizes, each carrying a measured ceiling. Planning
// stays arithmetic and the coding happens while writing, which is what keeps a
// preview of a large run cheap.
//
// The second is the allocation ceiling. This encoder allocates about 618 000
// objects coding one 640x480 picture, where the AVIF one allocates about a
// hundred, so the flat ceiling in internal/guard/resources_test.go does not
// fit it and the descriptor below declares its own. The check that ceiling
// stands in for - that a generator's cost follows the picture and not the size
// of the file asked for - still applies here unchanged, and this format passes
// it. Owner's decision, 2026-08-31.
//
// The padding channel is a free box, which is the box the container defines
// for space that means nothing. Measured on 2026-08-31 at 0, 1, 7, 64, 100,
// 101, 4096, 100 000 and 1 048 576 bytes, and again with two boxes, four
// boxes, a private box type, a skip box and the sixty four bit box length.
// libjxl through pillow-jxl-plugin reads every one of them, and refuses a
// truncated file and a corrupted one, which is what makes it a witness. The
// pure Go decoder in the encoder's own module agrees on all of it.
//
// Trailing bytes after a bare codestream were measured too, and they work -
// both readers take them, down to a single byte, with no dead zone at all.
// They are not used. They are not a structure the format defines, only
// something two readers tolerate, and this project's fourth untouchable rule
// says fidelity does not drop for the implementer's convenience. The free box
// says "nothing here" in the format's own words, and it is what AVIF writes.
package jxl

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

	// containerOverhead is everything the container costs around the
	// codestream: the signature box, the file type box, and the header of the
	// box the codestream sits in. The free box header is counted separately,
	// because it is what makes every size above the minimum reachable.
	containerOverhead = len(signature) + len(fileType) + boxHeader

	minDimension = 1
	maxDimension = 16384

	// maxPixels bounds the picture because the encoder holds all of it while
	// it works, and it is the same number PNG, JPG, GIF and AVIF use, so 4K
	// and 8K go through. It refuses the picture that would otherwise end as an
	// out of memory error with nothing said about why.
	maxPixels = 40_000_000

	// minimumBytes is the smallest JPEG XL this generator produces, and it is
	// one number for every seed rather than one sample.
	//
	// The picture varies with the seed through an offset taken modulo 256, and
	// a changed pixel changes how many bytes the coder emits, so a floor taken
	// from one seed is a number the tool would print and then refuse. There
	// are exactly 256 pictures at one pixel, so the largest of them settles it
	// for every seed there is. Measured across all of them by
	// tools/probes/jxlladder on 2026-08-31.
	//
	// The container and the free box header are included, because this
	// generator always writes both even when the box carries nothing. That is
	// what removes the unreachable band above the floor that PNG and GIF have.
	//
	// One number worth keeping: at four seeds the one pixel rung looked LARGER
	// than the eight by six one, which would have made the rungs below it
	// unreachable and the fallback wrong. At 256 seeds it is not - the ladder
	// decreases the whole way down. A ladder shaped from a short sweep would
	// have been shaped around noise.
	minimumBytes = 147
)

// signature is the box every JPEG XL container opens with. Its length, its
// name, and the four bytes that let a reader spot a file whose newlines have
// been mangled in transit.
// An array rather than a slice, so its length is a constant and the container
// overhead above can be one too.
var signature = [12]byte{
	0x00, 0x00, 0x00, 0x0C, 'J', 'X', 'L', ' ', 0x0D, 0x0A, 0x87, 0x0A,
}

// fileType is the box saying what this file is.
//
// The trailing brand is not decoration and was measured, which is the only
// reason it is here: libjxl REFUSES the file without it, while the pure Go
// decoder accepts it either way. A reader that is stricter than the one in
// the module we build against is exactly the reader worth writing for.
var fileType = [20]byte{
	0x00, 0x00, 0x00, 0x14, 'f', 't', 'y', 'p',
	'j', 'x', 'l', ' ', // brand
	0x00, 0x00, 0x00, 0x00, // minor version
	'j', 'x', 'l', ' ', // compatible brand
}

func init() {
	format.Register(format.Descriptor{
		ID:          "jxl",
		Extension:   ".jxl",
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,

		MinBytes: minimumBytes,

		// This encoder allocates per block rather than per file, so the flat
		// ceiling every hand written format here meets does not describe it.
		// Measured on 2026-08-31, lowest of five rounds: 1 222 objects for one
		// pixel, 156 679 at 320x240, 618 461 at 640x480. The number below is
		// today's measurement at the top of the ladder with room for the
		// runtime to move, and it is a ratchet like every other ceiling in
		// this project - it goes down when work makes it lowerable, never up
		// to turn a run green.
		AllocCeiling: 700_000,

		Padding: format.PaddingChannel{
			// Measured on files this encoder produces, not on a container
			// built by hand. libjxl through pillow-jxl-plugin and the pure Go
			// decoder both read every padded file. The negative control is the
			// half that matters: both refuse a truncated file and a corrupted
			// payload, so both can say no. exiftool reads JPEG XL too and
			// accepts the truncated file, so it witnesses nothing.
			Name:     "a free box after the picture",
			Where:    format.PlacementEnd,
			Capacity: 0,
		},
		Label:  format.LabelVisible,
		Oracle: "pillow-jxl",
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

	// coded is the codestream the encoder produced, and it is filled in only
	// when the recipe named the picture size. Left out, the picture is chosen
	// from the ladder without coding it and the work happens while writing.
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
		label = core.Label("jxl", r.Bytes, r.Seed)
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
			"compression": "jpeg-xl",
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
// The gate is the declared floor, not this seed's own coded size, so the
// number the tool announces is the number it accepts whatever the seed. A
// picture named by hand can be larger than the smallest one, and then its own
// size is the floor that applies.
func checkFits(want int64, m memo) error {
	floor := int64(minimumBytes)
	reason := fmt.Sprintf(
		"the smallest picture this format draws codes to %d B at worst, the container around it costs %d B, and the file always carries a free box, which costs %d B even when it holds nothing",
		minimumBytes-containerOverhead-boxHeader, containerOverhead, boxHeader)

	// A picture named by hand can be larger than the smallest one, and then its
	// own size is the floor that applies - which is known here, because naming
	// a size is what makes planning code the picture.
	if m.coded != nil {
		if own := int64(len(m.coded) + containerOverhead + boxHeader); own > floor {
			floor = own
			reason = fmt.Sprintf(
				"a %dx%d picture at quality %d codes to %d B, the container around it costs %d B, and the file always carries a free box, which costs %d B even when it holds nothing",
				m.width, m.height, m.quality, len(m.coded), containerOverhead, boxHeader)
		}
	}

	if want >= floor {
		return nil
	}
	return &format.BelowMinimumError{
		Format:    "JXL",
		Requested: want,
		Minimum:   floor,
		Reason:    reason,
		Hint:      fmt.Sprintf("Ask for %d B or more, or set a smaller width and height, or a lower quality", floor),
	}
}

// sizeLadder is tried from the largest down when the recipe names no picture
// size, so a small file gets a small picture instead of being refused.
//
// It starts at 640x480, the same top rung JPG and AVIF use, so the picture
// formats answer the same request with the same sized picture.
//
// It stops there rather than climbing to Full HD, and that is measured: at
// this encoder's settings 640x480 costs about 77 ms and 1920x1080 costs about
// 1.5 s, so a run of ten thousand large files would spend four hours on
// pictures nobody asked to be that big. Anybody who does want them says so -
// width and height are settings, and 40 megapixels is the ceiling.
//
// Ceiling is the LARGEST file that rung has ever been seen to produce, the
// container and the free box header included, and it is what lets planning
// choose a picture without coding one. Measured by tools/probes/jxlladder on
// 2026-08-31 across all 256 seeds, four orders of magnitude of requested size -
// the label carries the byte count, so its length moves with it - and with the
// label on and off.
//
// A ceiling rather than a floor, and the difference is the whole point.
// Planning promises an exact size, so the rung it picks has to be one whose
// picture is CERTAIN to leave room for the padding. Set too high, a ceiling
// costs a smaller picture than the file had room for and nothing else. Set too
// low, it would let planning pick a picture that does not fit, so writing
// checks and says so rather than trusting this table.
var sizeLadder = []rung{
	{640, 480, 22762}, {320, 240, 7949}, {256, 256, 6485}, {160, 120, 3542},
	{80, 60, 1267}, {40, 30, 731}, {20, 15, 452}, {8, 6, 222},
	{4, 3, 212}, {2, 2, 206}, {1, 1, minimumBytes},
}

type rung struct {
	width, height int
	ceiling       int64
}

// chooseSize settles the picture size, and codes it only when it has to.
func chooseSize(r format.Request, label string, q int) (memo, error) {
	_, wSet := r.Properties["width"]
	_, hSet := r.Properties["height"]
	if wSet || hSet {
		return namedSize(r, label, q)
	}

	// The ceilings were measured at the default quality and speak for nothing
	// else, so a run that asked for a different one takes the slow road: code
	// each step until one fits. This is not caution - measured at AVIF, a
	// quality the table did not describe picked a rung several times its
	// ceiling and produced no file at all.
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
		if r.Bytes >= int64(len(m.coded)+containerOverhead+boxHeader) {
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

// coded fills in the codestream for a picture that has been settled.
func coded(m memo) (memo, error) {
	b, err := encode(picture(m), m.quality)
	if err != nil {
		return memo{}, err
	}
	m.coded = b
	return m, nil
}

// checkJointLimits asks the registry's own declaration about a pair of
// dimensions, so the refusal, the sentence tfg formats jxl prints and the
// field a window draws all come from one line.
func checkJointLimits(w, h int) error {
	d, err := format.Get("jxl")
	if err != nil {
		return err
	}
	for _, j := range d.JointLimits {
		if bad := j.Allows(int64(w), int64(h)); bad != "" {
			return &format.PropertyValueError{
				Format: "jxl", Key: j.Of + " and " + j.By,
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
		return 0, fmt.Errorf("jxl: %s must be a whole number of pixels, got %q", key, raw)
	}
	if n < minDimension || n > maxDimension {
		return 0, fmt.Errorf("jxl: %s must be between %d and %d pixels, got %d", key, minDimension, maxDimension, n)
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
		return 0, fmt.Errorf("jxl: quality must be a whole number, got %q", raw)
	}
	if n < minQuality || n > maxQuality {
		return 0, fmt.Errorf("jxl: quality must be between %d and %d, got %d", minQuality, maxQuality, n)
	}
	return n, nil
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(memo)
	if !ok {
		return fmt.Errorf("jxl: the plan was not produced by this generator")
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

	pad := m.total - int64(len(coded)+containerOverhead)
	if pad < boxHeader {
		// The ladder's ceiling for this picture was too low. Said out loud
		// rather than written short, because a short file is a wrong file.
		return fmt.Errorf("jxl: a %dx%d picture coded to %d B, the container costs %d B and the file was to be %d B, which leaves no room for the free box every one of these carries",
			m.width, m.height, len(coded), containerOverhead, m.total)
	}

	if _, err := w.Write(signature[:]); err != nil {
		return err
	}
	if _, err := w.Write(fileType[:]); err != nil {
		return err
	}
	if err := writeBoxHeader(w, "jxlc", int64(len(coded))); err != nil {
		return err
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
		if err := writeBoxHeader(w, "free", payload); err != nil {
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

func writeBoxHeader(w io.Writer, kind string, payload int64) error {
	var b [boxHeader]byte
	binary.BigEndian.PutUint32(b[:4], uint32(boxHeader+payload))
	copy(b[4:], kind)
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
