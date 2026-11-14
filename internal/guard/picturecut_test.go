package guard

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// A BMP, a TIFF or a WebP laid out for a cut comes to every size asked for,
// its pixels are where its own headers say and are the pixels of the file
// nothing cuts, and the cut keeps the share of them.
//
// Each grows its picture to fill a file whose width and height are left out,
// and with them named the picture is small and the rest of the file padding.
// For a cut each splits that padding around the pixels (format.SplitFront):
// a BMP between bfOffBits and the pixels and behind them, a TIFF the same
// with its directory moved up behind the header, a WebP in a private chunk in
// front of the picture - which needs a VP8X chunk first, so its front is that
// and an even chunk of at least eight bytes, or nothing - and behind it. The
// share guard cuts two sizes of each. This writes consecutive sizes cut at
// the half, and cuts of one size a byte at a time where the share starts to
// want padding in front and where the padding behind runs out, with the
// picture's size named and left out, holds each to its size, its pointers and
// the share, and asserts it met each layout (docs/TRUNCATE-2026-11-11.md
// section 15.4). The pixels are compared as bytes rather than decoded: the
// same bytes where the headers point give the same picture in every reader,
// which is what section 15.3 measured for the layouts.
//
// Canary: a TIFF whose StripOffsets is a byte off, and a BMP whose bfOffBits
// is, are refused.
func TestABMPATIFFOrAWebPLaidOutForACutComesToEverySizeAndKeepsItsPixels(t *testing.T) {
	named := map[string]string{"width": "16", "height": "12"}
	for _, id := range []string{"bmp", "tiff", "webp"} {
		desc, err := format.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		met := map[string]bool{}
		for _, props := range []map[string]string{nil, named} {
			wholes := map[int64][]byte{}
			check := func(size, cutAt int64) {
				data, regions := writePicture(t, desc, size, cutAt, props)
				layout, err := pictureLayoutOf(id, data, regions, size, cutAt)
				if err != nil {
					t.Errorf("%s %v %d B cut at %d: %v", id, props, size, cutAt, err)
					return
				}
				met[layout] = true
				if _, ok := wholes[size]; !ok {
					whole, wholeRegions := writePicture(t, desc, size, 0, props)
					wholes[size] = whole[pictureRegion(wholeRegions).At:][:pictureRegion(wholeRegions).Length]
				}
				pics := pictureRegion(regions)
				if !bytes.Equal(data[pics.At:pics.At+pics.Length], wholes[size]) {
					t.Errorf("%s %v %d B cut at %d, %s: the pixels are not those of the file nothing cuts", id, props, size, cutAt, layout)
				}
				if err := picturePointers(id, data, pics); err != nil {
					t.Errorf("%s %v %d B cut at %d, %s: %v", id, props, size, cutAt, layout, err)
				}
			}
			smallest := desc.SmallestAccepted(format.Request{Label: true, Properties: props})
			for size := smallest; size < smallest+6; size++ {
				check(size, size/2)
			}
			if props == nil {
				// A picture that fills its file leaves less than a row of
				// padding, so a byte of padding is hundreds of bytes of cut:
				// the sweeps below are for the small picture.
				continue
			}
			// Cuts a byte apart where the share starts to want padding in
			// front - past head times the whole file over the part that is
			// not the picture - up to a VP8X and a chunk later, and near the
			// end, where what the share leaves behind runs out.
			const size = 16_000
			_, regions := writePicture(t, desc, size, 0, props)
			pics := pictureRegion(regions)
			head := pictureHead(id, pics)
			rest := size - pics.Length
			start := max(1, (head-3)*size/rest)
			for cutAt := start; cutAt < start+(vp8xAndAChunk+5)*size/rest; cutAt++ {
				check(size, cutAt)
			}
			for j := int64(1); j <= (vp8xAndAChunk+3)*size/rest; j++ {
				check(size, size-j)
			}
		}
		for _, layout := range pictureLayouts[id] {
			if !met[layout] {
				t.Errorf("%s: no size met the layout %q - met %v", id, layout, met)
			}
		}
	}

	// Canaries: a pointer a byte off.
	for _, id := range []string{"tiff", "bmp"} {
		desc, _ := format.Get(id)
		data, regions := writePicture(t, desc, 64<<10, 32<<10, named)
		pics := pictureRegion(regions)
		broken := bytes.Clone(data)
		at := map[string]int{"tiff": 8 + 2 + 12*5 + 8, "bmp": 10}[id]
		binary.LittleEndian.PutUint32(broken[at:], binary.LittleEndian.Uint32(broken[at:])+1)
		if picturePointers(id, broken, pics) == nil {
			t.Errorf("canary: a %s whose pixel offset is a byte off passed", id)
		}
	}
}

// vp8xAndAChunk is the least a WebP puts in front of its picture for a cut:
// the VP8X chunk and the smallest chunk there is.
const vp8xAndAChunk = 18 + 8

// pictureLayouts is every layout of the split each format has to meet.
var pictureLayouts = map[string][]string{
	"bmp":  {"nothing in front, the share wants none", "the share"},
	"tiff": {"nothing in front, the share wants none", "the share"},
	"webp": {"nothing in front, the share wants none", "a chunk in front, the share wants less", "the share",
		"all in front, what would stay behind is shorter than a chunk"},
}

// writePicture writes a file of desc through its own plan, with the settings
// props names, and gives its bytes and its map.
func writePicture(t *testing.T, desc format.Descriptor, size, cutAt int64, props map[string]string) ([]byte, []format.Region) {
	t.Helper()
	p, err := desc.Generator.Plan(format.Request{Bytes: size, Label: true, Seed: 7, CutAt: cutAt, Properties: props})
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

// pictureHead is what stands in front of all of the padding of a file laid
// out for a cut: the headers of a BMP, a TIFF's header, directory and values,
// a WebP's RIFF header - its VP8X chunk counts as the price of a front.
func pictureHead(id string, pics format.Region) int64 {
	return map[string]int64{"bmp": 54, "tiff": 8 + 150 + 22, "webp": 12}[id]
}

// pictureLayoutOf says which layout of the split a file laid out for a cut has,
// or what is wrong with it: its length, and padding in front of the picture
// that is not the share, as format.SplitFront has it.
func pictureLayoutOf(id string, data []byte, regions []format.Region, size, cutAt int64) (string, error) {
	if int64(len(data)) != size {
		return "", fmt.Errorf("came to %d B", len(data))
	}
	if id != "webp" {
		return filmLayoutOfMap(regions, size, cutAt, 1)
	}
	pics := pictureRegion(regions)
	if pics.Length <= 0 {
		return "", errors.New("no picture marked")
	}
	front, back := int64(0), size-pics.At-pics.Length
	if pics.At > 12 {
		front = pics.At - 12 - (vp8xAndAChunk - 8)
	}
	pad := size - 12 - pics.Length
	simple := format.PaddingInFront(12, pics.Length, cutAt, size)
	switch {
	case front == 0 && simple <= 0:
		return "nothing in front, the share wants none", nil
	case front == 0 && pad < vp8xAndAChunk:
		return "nothing in front, too little padding for a VP8X and a chunk", nil
	case front == 0:
		return "", fmt.Errorf("nothing in front and %d B behind, where the share wants %d B in front", back, simple)
	case front < 8 || front%2 != 0:
		return "", fmt.Errorf("%d B in front, where a chunk is even and at least 8 B", front)
	}
	want := format.PaddingInFront(12+vp8xAndAChunk-8, pics.Length, cutAt, size)
	switch {
	case front == 8 && want < 8:
		return "a chunk in front, the share wants less", nil
	case front >= want && front-want < 2:
		// The front rounds up to an even chunk, a byte at most.
		return "the share", nil
	case front > want && back < 8+2:
		return "all in front, what would stay behind is shorter than a chunk", nil
	}
	return "", fmt.Errorf("%d B in front and %d B behind, where the share wants %d B in front", front, back, want)
}

// picturePointers requires a BMP's, a TIFF's or a WebP's own headers to put
// its pixels where its map says, and a file to be as long as it says.
func picturePointers(id string, data []byte, pics format.Region) error {
	le := binary.LittleEndian
	switch id {
	case "bmp":
		if at := int64(le.Uint32(data[10:14])); at != pics.At {
			return fmt.Errorf("bfOffBits gives %d and the pixels are at %d", at, pics.At)
		}
		if n := int64(le.Uint32(data[2:6])); n != int64(len(data)) {
			return fmt.Errorf("bfSize gives %d B for a %d B file", n, len(data))
		}
	case "tiff":
		ifd := int64(le.Uint32(data[4:8]))
		if ifd != 8 {
			return fmt.Errorf("the directory is at %d, not right behind the header", ifd)
		}
		strips := map[uint16]int64{}
		for i := range int64(le.Uint16(data[ifd:])) {
			e := data[ifd+2+12*i:]
			strips[le.Uint16(e)] = int64(le.Uint32(e[8:12]))
		}
		if strips[273] != pics.At || strips[279] != pics.Length {
			return fmt.Errorf("the directory gives the pixels at %d for %d B, and they are at %d for %d B", strips[273], strips[279], pics.At, pics.Length)
		}
	case "webp":
		return webpPointers(data, pics)
	}
	return nil
}

// webpPointers walks a WebP's chunks: they have to fill the length the RIFF
// header gives, with at most seven bytes past it, the picture's chunk has to
// be where the map says, and a VP8X chunk has to stand first and give the
// picture's size.
func webpPointers(data []byte, pics format.Region) error {
	le := binary.LittleEndian
	end := 8 + int64(le.Uint32(data[4:8]))
	if tail := int64(len(data)) - end; tail < 0 || tail > 7 {
		return fmt.Errorf("%d B past the RIFF payload", tail)
	}
	var names []string
	at, picture := int64(12), int64(-1)
	for at+8 <= end {
		name, n := string(data[at:at+4]), int64(le.Uint32(data[at+4:at+8]))
		names = append(names, name)
		if name == "VP8L" {
			picture = at
		}
		at += 8 + n + n%2
	}
	if at != end || picture != pics.At {
		return fmt.Errorf("the chunks %v end at %d of %d, the picture's at %d where the map has it at %d", names, at, end, picture, pics.At)
	}
	if names[0] != "VP8X" {
		if picture != 12 {
			return fmt.Errorf("the chunks are %v, something in front of the picture without a VP8X", names)
		}
		return nil
	}
	bits := le.Uint32(data[picture+9:])
	w, h := bits&0x3FFF, bits>>14&0x3FFF
	cw := uint32(data[24]) | uint32(data[25])<<8 | uint32(data[26])<<16
	ch := uint32(data[27]) | uint32(data[28])<<8 | uint32(data[29])<<16
	if cw != w || ch != h || !bytes.Equal(data[20:24], make([]byte, 4)) {
		return fmt.Errorf("the VP8X gives a %dx%d canvas and flags % x for a %dx%d picture", cw+1, ch+1, data[20:24], w+1, h+1)
	}
	return nil
}
