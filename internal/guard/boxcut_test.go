package guard

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"io"
	"testing"

	gavif "github.com/gen2brain/gav1d/avif"
	gjxl "github.com/gen2brain/jxl"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// An AVIF or a JPEG XL laid out for a cut comes to every size asked for, its
// picture is where its boxes say, and the cut keeps the share of it.
//
// Both put free boxes in front of the picture for a cut, and a free box costs
// eight bytes before it carries anything, so each side is nothing or at least
// that: where the share wants less than a box in front, a box goes there, and
// where the padding left behind would be shorter than a box, it goes in front
// too (format.SplitFront). In an AVIF the box in front moves the media data,
// and the item locations in the meta box give its place in bytes from the
// start of the file, so they move with it. No other guard asks for the sizes
// and the cuts where those happen. So this writes consecutive sizes cut at
// their last byte and at the half, and cuts of one size a byte at a time from
// just in front of the picture, holds each to its size, to the share and, for
// an AVIF, to an item location that lands on the media data, and asserts it
// met each layout: nothing in front where the share wants nothing, a box where
// it wants less, the share to the byte, and all of the padding in front where
// what would stay behind is shorter than a box. One file of each layout is
// decoded and must give the pixels of the file nothing cuts.
//
// An AVIF of 5 GiB cut at 90% is planned and only its head is written: its
// media data starts past where four bytes reach, so the item location is
// widened to eight.
//
// Canary: an item location a byte off is refused.
func TestAnAVIFOrAJPEGXLLaidOutForACutComesToEverySizeAndReads(t *testing.T) {
	for _, id := range []string{"avif", "jxl"} {
		desc, err := format.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		met := map[string]bool{}
		decoded := map[string]bool{}
		check := func(size, cutAt int64) {
			data, regions := writeBoxes(t, desc, size, cutAt)
			layout, err := boxLayoutOf(id, data, regions, size, cutAt)
			if err != nil {
				t.Errorf("%s %d B cut at %d: %v", id, size, cutAt, err)
				return
			}
			met[layout] = true
			if !decoded[layout] {
				decoded[layout] = true
				whole, _ := writeBoxes(t, desc, size, 0)
				if err := samePicture(id, data, whole); err != nil {
					t.Errorf("%s %d B cut at %d, %s: %v", id, size, cutAt, layout, err)
				}
			}
		}
		smallest := desc.SmallestAccepted(format.Request{Label: true})
		for size := smallest; size < smallest+64; size++ {
			check(size, size/2)
		}
		// Cuts a byte apart where the share starts to want padding in front -
		// it wants cutAt times the part of the file that is not the picture,
		// less the head - and near the end, where what the share leaves behind
		// runs out: j bytes from the end it leaves the padding's part of j.
		const size = 4_000
		_, regions := writeBoxes(t, desc, size, 0)
		head, pic, pad := boxParts(t, regions)
		rest := size - pic.Length
		start := (head - 3) * size / rest
		for cutAt := start; cutAt < start+12*size/rest; cutAt++ {
			check(size, cutAt)
		}
		for j := int64(1); j <= 10*size/pad; j++ {
			check(size, size-j)
		}
		for _, layout := range []string{"nothing in front, the share wants none", "a box in front, the share wants less",
			"the share", "all in front, what would stay behind is shorter than a box"} {
			if !met[layout] {
				t.Errorf("%s: no size met the layout %q - met %v", id, layout, met)
			}
		}
	}

	avifTooLargeForFourBytes(t)

	// Canary: an item location a byte off.
	desc, _ := format.Get("avif")
	data, _ := writeBoxes(t, desc, 64<<10, 32<<10)
	broken := bytes.Clone(data)
	at, _, err := avifItemPlace(broken)
	if err != nil {
		t.Fatal(err)
	}
	binary.BigEndian.PutUint32(broken[at:], binary.BigEndian.Uint32(broken[at:])+1)
	if err := avifPlaceLands(broken); err == nil {
		t.Errorf("canary: an item location a byte off passed")
	}
}

// writeBoxes writes a file of size bytes laid out for a cut after cutAt,
// whole, and the regions it marked.
func writeBoxes(t *testing.T, desc format.Descriptor, size, cutAt int64) ([]byte, []format.Region) {
	t.Helper()
	p, err := desc.Generator.Plan(format.Request{Bytes: size, Label: true, Seed: 7, CutAt: cutAt})
	if err != nil {
		t.Fatalf("%s %d B: planning: %v", desc.ID, size, err)
	}
	var out bytes.Buffer
	rec := &format.MapRecorder{W: &out}
	if err := desc.Generator.Write(t.Context(), rec, p); err != nil {
		t.Fatalf("%s %d B: writing: %v", desc.ID, size, err)
	}
	return out.Bytes(), rec.Map(size, true)
}

// pictureRegion is the region a file marked as its content.
func pictureRegion(regions []format.Region) format.Region {
	for _, r := range regions {
		if r.Role == format.RoleContent {
			return r
		}
	}
	return format.Region{}
}

// boxParts reads a file's map as what stands in front of all of its padding,
// its picture and all of its padding, wherever the padding stands. Neither
// format closes with an end region, and one would stand behind the padding,
// where head does not count it, so meeting one fails.
func boxParts(t *testing.T, regions []format.Region) (head int64, pic format.Region, pad int64) {
	t.Helper()
	for _, r := range regions {
		switch r.Role {
		case format.RoleEnd:
			t.Fatalf("an end region at %d, which neither format writes", r.At)
		case format.RoleStructure:
			head += r.Length
		case format.RolePadding:
			pad += r.Length
		case format.RoleContent:
			pic = r
		}
	}
	return head, pic, pad
}

// A JPEG XL nothing cuts has nothing behind its codestream and no free box
// anywhere. ffmpeg 7.1.5 with libjxl never finishes a JPEG XL with any box
// behind its codestream, and ffmpeg 9.0.1 takes one with a free box in its
// first two kilobytes for a movie (O341, docs/TRUNCATE-2026-11-11.md section
// 13.4). So this writes the smallest sizes, where the padding is one box that
// carries nothing or a few bytes, and larger ones, and requires the codestream
// box to be the last box and no box to be a free one.
//
// Canary: the same file with an empty box behind its codestream is refused.
func TestAJPEGXLKeepsNothingBehindItsCodestream(t *testing.T) {
	desc, err := format.Get("jxl")
	if err != nil {
		t.Fatal(err)
	}
	smallest := desc.SmallestAccepted(format.Request{Label: true})
	for _, size := range []int64{smallest, smallest + 1, smallest + 7, smallest + 8, smallest + 9, 2 << 10, 64 << 10, 1 << 20} {
		data, _ := writeBoxes(t, desc, size, 0)
		if err := jxlNothingBehind(data); err != nil {
			t.Errorf("%d B: %v", size, err)
		}
	}

	data, _ := writeBoxes(t, desc, 2<<10, 0)
	broken := append(bytes.Clone(data), 0, 0, 0, 8, 's', 'k', 'i', 'p')
	if err := jxlNothingBehind(broken); err == nil {
		t.Errorf("canary: a JPEG XL with an empty box behind its codestream passed")
	}
}

// jxlNothingBehind requires a JPEG XL's codestream box to be its last box, and
// none of its boxes to be a free one.
func jxlNothingBehind(data []byte) error {
	last := ""
	for pos := 0; pos+8 <= len(data); {
		size := int(binary.BigEndian.Uint32(data[pos:]))
		kind := string(data[pos+4 : pos+8])
		if size < 8 || pos+size > len(data) {
			return fmt.Errorf("a %q box of %d B at %d", kind, size, pos)
		}
		if kind == "free" {
			return fmt.Errorf("a free box at %d", pos)
		}
		last = kind
		pos += size
	}
	if last != "jxlc" {
		return fmt.Errorf("the last box is %q, where the codestream should end the file", last)
	}
	return nil
}

// boxLayoutOf holds a file laid out for a cut to its size, its picture's place
// and the share the cut keeps of the picture, and names the layout it took.
func boxLayoutOf(id string, data []byte, regions []format.Region, size, cutAt int64) (string, error) {
	if int64(len(data)) != size {
		return "", fmt.Errorf("came to %d B", len(data))
	}
	if id == "avif" {
		if err := avifPlaceLands(data); err != nil {
			return "", err
		}
	} else if err := jxlNoFreeAhead(data); err != nil {
		return "", err
	}
	content := -1
	for i, r := range regions {
		if r.Role == format.RoleContent {
			content = i
		}
	}
	if content < 0 {
		return "", errors.New("no content marked")
	}
	pic := regions[content]
	front, back := int64(0), int64(0)
	if content > 0 && regions[content-1].Role == format.RolePadding {
		front = regions[content-1].Length
	}
	for _, r := range regions[content+1:] {
		if r.Role != format.RolePadding {
			return "", fmt.Errorf("a %s region behind the picture", r.Role)
		}
		back += r.Length
	}
	if pic.At+pic.Length+back != size {
		return "", fmt.Errorf("the picture ends at %d and %d B of padding follow it in a %d B file", pic.At+pic.Length, back, size)
	}
	if (front > 0 && front < 8) || (back > 0 && back < 8) {
		return "", fmt.Errorf("a side shorter than a box: %d in front, %d behind", front, back)
	}
	head := pic.At - front
	want := format.PaddingInFront(head, pic.Length, cutAt, size)
	kept, share := cutAt-head-front, pic.Length*cutAt/size
	switch {
	case front == 0 && want <= 0:
		return "nothing in front, the share wants none", nil
	case front == 0:
		return "", fmt.Errorf("nothing in front and %d B behind, where the share wants %d B in front", back, want)
	case front == want && kept == share:
		return "the share", nil
	case front == 8 && want > 0 && want < 8:
		return "a box in front, the share wants less", nil
	case back == 0 && front > want && front-want < 8:
		return "all in front, what would stay behind is shorter than a box", nil
	}
	return "", fmt.Errorf("%d B in front where the share wants %d, keeping %d B of the picture where the share is %d", front, want, kept, share)
}

// samePicture decodes a file laid out for a cut and the file nothing cuts with
// the coder's own decoder, and requires the same pixels.
func samePicture(id string, laid, whole []byte) error {
	decode := gjxl.Decode
	if id == "avif" {
		decode = func(r io.Reader, _ ...gjxl.Options) (image.Image, error) { return gavif.Decode(r) }
	}
	a, err := decode(bytes.NewReader(laid))
	if err != nil {
		return fmt.Errorf("the decoder refused it: %w", err)
	}
	b, err := decode(bytes.NewReader(whole))
	if err != nil {
		return fmt.Errorf("the decoder refused the file nothing cuts: %w", err)
	}
	if a.Bounds() != b.Bounds() {
		return fmt.Errorf("a %v picture where nothing cut gives %v", a.Bounds(), b.Bounds())
	}
	for y := a.Bounds().Min.Y; y < a.Bounds().Max.Y; y++ {
		for x := a.Bounds().Min.X; x < a.Bounds().Max.X; x++ {
			if a.At(x, y) != b.At(x, y) {
				return fmt.Errorf("the pixel at %d,%d differs from the file nothing cuts", x, y)
			}
		}
	}
	return nil
}

// avifItemPlace finds the one item location of an AVIF as tfg's coder writes
// it - version 0, no base offset, one item with one extent - and gives where
// its offset field is in the file and the place the field holds.
func avifItemPlace(data []byte) (field int, place int64, err error) {
	for pos := 0; pos+8 <= len(data); {
		size := int(binary.BigEndian.Uint32(data[pos:]))
		if size < 8 || pos+size > len(data) {
			break
		}
		if string(data[pos+4:pos+8]) == "meta" {
			for at := pos + 12; at+8 <= pos+size; {
				n := int(binary.BigEndian.Uint32(data[at:]))
				if n < 8 {
					break
				}
				if string(data[at+4:at+8]) == "iloc" {
					b := data[at+8 : at+n]
					offSize := int(b[4] >> 4)
					if b[0] != 0 || b[5]>>4 != 0 || binary.BigEndian.Uint16(b[6:]) != 1 || binary.BigEndian.Uint16(b[12:]) != 1 {
						return 0, 0, fmt.Errorf("an item location box this does not read: % x", b[:min(len(b), 16)])
					}
					field = at + 8 + 14
					for _, c := range data[field : field+offSize] {
						place = place<<8 | int64(c)
					}
					return field, place, nil
				}
				at += n
			}
		}
		pos += size
	}
	return 0, 0, errors.New("no item location box")
}

// avifPlaceLands requires the item location of an AVIF to give the first byte
// after the header of its media data box.
func avifPlaceLands(data []byte) error {
	_, place, err := avifItemPlace(data)
	if err != nil {
		return err
	}
	for pos := int64(0); pos+8 <= int64(len(data)); {
		size := int64(binary.BigEndian.Uint32(data[pos:]))
		if size < 8 {
			break
		}
		if string(data[pos+4:pos+8]) == "mdat" {
			if place != pos+8 {
				return fmt.Errorf("the item location gives %d where the media data starts at %d", place, pos+8)
			}
			return nil
		}
		pos += size
	}
	return errors.New("no media data box")
}

// jxlNoFreeAhead requires no free box in front of a JPEG XL's codestream box.
// ffmpeg 9.0.1 takes a JPEG XL container with a free box in its first two
// kilobytes for a movie and refuses it, and one with a skip box there for a
// picture (docs/TRUNCATE-2026-11-11.md section 13).
func jxlNoFreeAhead(data []byte) error {
	for pos := 0; pos+8 <= len(data); {
		size := int(binary.BigEndian.Uint32(data[pos:]))
		switch kind := string(data[pos+4 : pos+8]); {
		case kind == "jxlc":
			return nil
		case kind == "free":
			return fmt.Errorf("a free box at %d, in front of the codestream", pos)
		case size < 8:
			return fmt.Errorf("a box of %d B at %d", size, pos)
		}
		pos += size
	}
	return errors.New("no codestream box")
}

// avifTooLargeForFourBytes plans an AVIF of 5 GiB cut at 90% and holds the
// head it writes to an item location of eight bytes that gives where the map
// says the media data starts. It writes nothing past the head.
func avifTooLargeForFourBytes(t *testing.T) {
	t.Helper()
	const size = 5 << 30
	desc, _ := format.Get("avif")
	p, err := desc.Generator.Plan(format.Request{Bytes: size, Label: true, Seed: 7, CutAt: size / 10 * 9})
	if err != nil {
		t.Fatalf("planning %d B: %v", int64(size), err)
	}
	head := &headOnly{keep: 1024}
	rec := &format.MapRecorder{W: head}
	if err := desc.Generator.Write(t.Context(), rec, p); err == nil {
		t.Fatalf("writing %d B went through a writer that stops after its head", int64(size))
	}
	pic := pictureRegion(rec.Map(size, true))
	field, place, err := avifItemPlace(head.got)
	if err != nil {
		t.Fatalf("%d B: %v", int64(size), err)
	}
	if width := head.got[field-14+4] >> 4; width != 8 || place != pic.At+8 || pic.At <= 1<<32 {
		t.Errorf("%d B cut at 90%%: an item location of %d bytes gives %d, where the media data starts at %d",
			int64(size), width, place, pic.At+8)
	}
}

// headOnly keeps the first keep bytes written to it and refuses the rest.
type headOnly struct {
	keep int
	got  []byte
}

func (h *headOnly) Write(p []byte) (int, error) {
	if len(h.got)+len(p) > h.keep {
		n := h.keep - len(h.got)
		h.got = append(h.got, p[:n]...)
		return n, errors.New("only the head is kept")
	}
	h.got = append(h.got, p...)
	return len(p), nil
}
