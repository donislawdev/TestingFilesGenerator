package guard

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // DecodeConfig of the jpg pictures
	_ "image/png"  // and of the png ones
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	pic "github.com/donislawdev/TestingFilesGenerator/internal/format/picture"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/rtf"
)

// The pictures an RTF document carries, from 2026-10-15
// (docs/RTF-2-2026-10-15.md). Read from the bytes, with no tool outside this
// process, so these run wherever the suite does.

// rtfPictureHead is one picture's paragraph up to its data, in the shape
// LibreOffice writes a picture: the name, the format, the size in pixels and on
// the page, and with binary the count of bytes that follow.
var rtfPictureHead = regexp.MustCompile(`inline_([0-9]{4,5})(\.[a-z]+) \{\\pict\\([a-z]+)\\picw([0-9]+)\\pich([0-9]+)\\picwgoal([0-9]+)\\pichgoal([0-9]+)(?:\\bin([0-9]+) )?`)

// rtfPicture is one picture as a document carries it.
type rtfPicture struct {
	name, blip       string
	w, h, gw, gh     int
	binary           bool
	data             []byte
	longestHexLine   int
	hexLineCount     int
	closedByTheBrace bool
}

// rtfPicturesOf reads every picture out of a document, in order, each from
// where the last one ended, so the bytes of a binary picture are never taken
// for the start of another.
func rtfPicturesOf(t *testing.T, body []byte) []rtfPicture {
	t.Helper()
	var out []rtfPicture
	for pos := 0; ; {
		m := rtfPictureHead.FindSubmatchIndex(body[pos:])
		if m == nil {
			return out
		}
		field := func(i int) string { return string(body[pos+m[2*i] : pos+m[2*i+1]]) }
		number := func(i int) int { n, _ := strconv.Atoi(field(i)); return n }
		p := rtfPicture{name: "inline_" + field(1) + field(2), blip: field(3),
			w: number(4), h: number(5), gw: number(6), gh: number(7), binary: m[16] >= 0}
		k := pos + m[1]
		if p.binary {
			n := number(8)
			p.data = body[k:min(k+n, len(body))]
			k += len(p.data)
		} else {
			var digits []byte
			for bytes.HasPrefix(body[k:], []byte("\r\n")) {
				end := k + 2
				for end < len(body) && strings.IndexByte("0123456789abcdef", body[end]) >= 0 {
					end++
				}
				p.longestHexLine = max(p.longestHexLine, end-k-2)
				p.hexLineCount++
				digits = append(digits, body[k+2:end]...)
				k = end
			}
			var err error
			if p.data, err = hex.DecodeString(string(digits)); err != nil {
				t.Fatalf("%s is not hex: %v", p.name, err)
			}
		}
		p.closedByTheBrace = k < len(body) && body[k] == '}'
		out = append(out, p)
		pos = k
	}
}

// TestAPictureInAnRTFIsTheFileItsFormatWritesAlone is the claim of inline in
// both ways a picture's bytes can be written.
//
// The document carries as many pictures as were ordered, each named for its
// place and its format, in the word RTF names the format by, written the way
// picture_data says, its data closed by the brace of its group, and each the
// very bytes the format writes on its own with the seed of its place - so a
// picture is a real file a tester can cut out, not something that only looks
// like one. Its size in pixels is the file's own, and on the page it stands at
// 96 pixels to the inch, no wider than the column of the page RTF assumes when
// a document names none (owner's decision of 2026-10-15).
func TestAPictureInAnRTFIsTheFileItsFormatWritesAlone(t *testing.T) {
	blips := map[string]string{"png": "pngblip", "jpg": "jpegblip"}
	for id, blip := range blips {
		desc, err := format.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		for _, data := range []string{rtf.Hex, rtf.Binary} {
			props := map[string]string{"inline": "2", "inline_format": id, "inline_size": "2kb", rtf.PictureData: data}
			found := rtfPicturesOf(t, writeEncoded(t, "rtf", 65536, props))
			if len(found) != 2 {
				t.Errorf("%s %s: inline=2 was ordered and the document carries %d pictures", id, data, len(found))
				continue
			}
			for i, p := range found {
				where := fmt.Sprintf("%s %s picture %d", id, data, i+1)
				if want := fmt.Sprintf("inline_%04d%s", i+1, desc.Extension); p.name != want || p.blip != blip {
					t.Errorf("%s is %s with \\%s, and %s with \\%s was ordered", where, p.name, p.blip, want, blip)
				}
				if p.binary != (data == rtf.Binary) || !p.closedByTheBrace {
					t.Errorf("%s: binary %v, closed by its brace %v", where, p.binary, p.closedByTheBrace)
				}
				if !p.binary && (p.longestHexLine != 128 || p.hexLineCount != (2*2048+127)/128) {
					t.Errorf("%s: %d lines of hex, the longest %d digits, and 2 KiB is %d lines of 128", where, p.hexLineCount, p.longestHexLine, (2*2048+127)/128)
				}
				if alone := htmlPictureAlone(t, desc, core.FileSeed(7741, i)); !bytes.Equal(p.data, alone) {
					t.Errorf("%s is %d B and not the file the format writes on its own with the seed of its place", where, len(p.data))
				}
				rtfPictureSize(t, where, p)
			}
		}
	}
}

// rtfPictureSize holds a picture's sizes to the file it is: in pixels its own,
// and on the page 15 twips to a pixel, or the column's 8 640 twips wide with
// the height in proportion.
func rtfPictureSize(t *testing.T, where string, p rtfPicture) {
	t.Helper()
	c, _, err := image.DecodeConfig(bytes.NewReader(p.data))
	if err != nil {
		t.Errorf("%s does not decode: %v", where, err)
		return
	}
	gw, gh := c.Width*15, c.Height*15
	if gw > 8640 {
		gw, gh = 8640, c.Height*8640/c.Width
	}
	if p.w != c.Width || p.h != c.Height || p.gw != gw || p.gh != gh {
		t.Errorf("%s says %dx%d pixels and %dx%d twips, and the file is %dx%d, which is %dx%d", where, p.w, p.h, p.gw, p.gh, c.Width, c.Height, gw, gh)
	}
}

// TestAnRTFPictureStaysInsideTheColumn is the other side of the rule above,
// from the sizes either side of the column: a picture whose width at 96 pixels
// to the inch is inside the column keeps it, and the largest one, 640 pixels,
// is narrowed to the column. Both are asserted to be there, so the guard is in
// the state it is about rather than passing on pictures that never reach it.
func TestAnRTFPictureStaysInsideTheColumn(t *testing.T) {
	var narrowed, kept bool
	for _, size := range []string{"1kb", "8kb"} {
		for _, p := range rtfPicturesOf(t, writeEncoded(t, "rtf", 65536, map[string]string{"inline": "1", "inline_size": size})) {
			rtfPictureSize(t, "png of "+size, p)
			narrowed = narrowed || (p.w*15 > 8640 && p.gw == 8640)
			kept = kept || (p.w*15 <= 8640 && p.gw == p.w*15)
		}
	}
	if !narrowed || !kept {
		t.Errorf("no picture was narrowed to the column (%v) or none kept its own width (%v), so one side of the rule went unasked", narrowed, kept)
	}
}

// TestAnRTFRefusesPicturesItCannotCarry holds the refusal of every format but
// the two RTF names, with no pictures ordered as well: a value the setting
// cannot take is a mistake either way, as it is in a page and a message. The
// formats come from the registry, so a picture format added tomorrow is asked
// about too, and png and jpg are asked to be taken, so the refusal is about the
// format rather than about everything.
func TestAnRTFRefusesPicturesItCannotCarry(t *testing.T) {
	d, err := format.Get("rtf")
	if err != nil {
		t.Fatal(err)
	}
	asked := 0
	for _, id := range append(pic.Formats(), "txt") {
		for _, count := range []string{"0", "1"} {
			_, err := d.Generator.Plan(format.Request{Bytes: 65536, Seed: 7741,
				Properties: map[string]string{"inline": count, "inline_format": id}})
			if id == "png" || id == "jpg" {
				if err != nil {
					t.Errorf("inline=%s with inline_format=%s was refused: %v", count, id, err)
				}
				continue
			}
			asked++
			var bad *format.PropertyValueError
			if !errors.As(err, &bad) || bad.Key != "inline_format" || !strings.Contains(bad.Remedy.String(), "png, jpg") {
				t.Errorf("inline=%s with inline_format=%s was answered with %v, not a refusal naming png and jpg", count, id, err)
			}
		}
	}
	if asked < 4 {
		t.Errorf("only %d refusals were asked about, so the build has hardly any picture format RTF does not name", asked)
	}
}

// TestAnRTFOfPicturesTooLargeToCountIsRefused holds the ceiling the sums are
// counted under, both ways past it: one jpg picture so large that twice it, in
// hex, does not fit in an int64 - without the check of each picture on its own
// it came back planned, measured on 2026-10-15 - and one under the ceiling that
// hex makes twice as large. jpg has no ceiling of its own, png refuses past one
// chunk of about 2 GB. A document planned past it would be counted in an int64
// that wrapped round, a size that is a lie rather than a refusal. One picture
// rather than ten thousand: a jpg of that size takes milliseconds to plan, and
// ten thousand of them took 34 seconds.
func TestAnRTFOfPicturesTooLargeToCountIsRefused(t *testing.T) {
	d, err := format.Get("rtf")
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []string{"8000000000gb", "1000000000gb"} {
		_, err = d.Generator.Plan(format.Request{Bytes: 65536, Seed: 7741,
			Properties: map[string]string{"inline": "1", "inline_format": "jpg", "inline_size": size}})
		var above *format.AboveMaximumError
		if !errors.As(err, &above) || above.Maximum != int64(1)<<60 {
			t.Errorf("one jpg picture of %s in hex was answered with %v, not a refusal at the ceiling", size, err)
		}
	}
}

// TestAnRTFTooSmallForItsPicturesSaysHowTheyAreWritten holds the refusal a
// tester meets first when ordering pictures: the minimum of that order, and a
// reason that says why it is that large - twice the picture in hex, the picture
// itself in binary - and the minimum is then accepted.
func TestAnRTFTooSmallForItsPicturesSaysHowTheyAreWritten(t *testing.T) {
	d, err := format.Get("rtf")
	if err != nil {
		t.Fatal(err)
	}
	for data, says := range map[string]string{rtf.Hex: "in hex", rtf.Binary: "byte for byte"} {
		props := map[string]string{"inline": "1", rtf.PictureData: data}
		_, err := d.Generator.Plan(format.Request{Bytes: 1024, Seed: 7741, Properties: props})
		var below *format.BelowMinimumError
		if !errors.As(err, &below) || !strings.Contains(below.Reason.String(), says) {
			t.Errorf("%s: 1 KiB with an 8 KiB picture was answered with %v, not a refusal saying %q", data, err, says)
			continue
		}
		if _, err := d.Generator.Plan(format.Request{Bytes: below.Minimum, Seed: 7741, Properties: props}); err != nil {
			t.Errorf("%s: the minimum %d B the refusal names was itself refused: %v", data, below.Minimum, err)
		}
	}
}
