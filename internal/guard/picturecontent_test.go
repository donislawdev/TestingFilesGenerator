package guard

import (
	"bytes"
	"errors"
	"slices"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// What this defends. A picture of a given width and height, seed and settings
// is the same picture whatever size of file it is asked for: only the padding
// around it moves (docs/SIZE-FIXES-2026-11-16.md section 3.1).
//
// Why it matters to somebody using the tool. The smallest file a picture fits
// in is then one number, so the size a refusal names is a size the next run
// takes - the button under a refusal in the window puts exactly that number in
// the field. While the label drawn into the picture stated the byte count, the
// picture changed with the size asked for, a compressed one changed its length
// with it, and asking for the minimum a refusal named was refused again with a
// new one: JPG 640x480 named 41549 B, then 42484 B, measured on 2026-11-16 in
// ten formats of thirty two.
//
// How. Every format that declares both a width and a height draws a picture -
// read from the registry, so a new picture format is checked the day it is
// registered. Each writes a file at two sizes far enough apart that a size in
// the label would differ in its number of digits, and what the two files draw
// has to be equal: the bytes the file map marks as content. A drawing written
// as text, an SVG, has no padding apart from its content - its test drawing
// takes more shapes for a larger file, by design - so there it is the label
// written into the drawing that has to be equal. The plans have to say the
// label is drawn, because a picture with no label proves nothing here.
func TestAPictureIsTheSameWhateverSizeItIsAskedFor(t *testing.T) {
	cases := picturesBySize(t)
	if len(cases) == 0 {
		t.Fatal("no registered format declares a width and a height - this guard would pass without checking anything")
	}
	askThePictureCanaries(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			floor := minimumAtOneByte(t, c)
			small := floor*2 + 1000
			large := small*10 + 7
			a, b := drawnAt(t, c, small), drawnAt(t, c, large)
			if !bytes.Equal(a, b) {
				t.Errorf("a %d B file and a %d B file draw different pictures (%d and %d B compared) - the picture depends on the size asked for, so a refusal's minimum is not a size the next run takes",
					small, large, len(a), len(b))
			}
		})
	}
}

// askThePictureCanaries plants what each half of the comparison has to tell
// apart: the same picture behind different padding, two pictures a byte apart,
// and a drawing written as text whose label differs.
func askThePictureCanaries(t *testing.T) {
	t.Helper()
	content := []format.Region{{Role: format.RoleStructure, At: 0, Length: 1}, {Role: format.RoleContent, At: 1, Length: 3}, {Role: format.RolePadding, At: 4, Length: 2}}
	if !bytes.Equal(drawnPart([]byte("abcdef"), content, "png"), drawnPart([]byte("abcdyy"), content, "png")) {
		t.Fatal("canary: the same picture behind different padding was taken for two pictures")
	}
	if bytes.Equal(drawnPart([]byte("abcdef"), content, "png"), drawnPart([]byte("abXdef"), content, "png")) {
		t.Fatal("canary: two pictures a byte apart were taken for the same picture")
	}
	whole := []format.Region{{Role: format.RoleContent, At: 0, Length: -1}}
	if bytes.Equal(drawnPart([]byte("<t>tfg - svg - 1 B - seed 7</t>"), whole, "svg"), drawnPart([]byte("<t>tfg - svg - 10 B - seed 7</t>"), whole, "svg")) {
		t.Fatal("canary: two drawings whose labels differ were taken for the same drawing")
	}
}

// drawnPicture is one picture format with the settings it is written with.
type drawnPicture struct {
	name    string
	format  format.Descriptor
	setting map[string]string
}

// picturesBySize is every registered format that declares a width and a
// height, at 160x120, and once more drawing a placeholder where the format
// offers one - at 320x240, because a placeholder lays its label out beside its
// own words and needs the room. 160 px is the narrowest width of the ladders
// whose label is drawn.
func picturesBySize(t *testing.T) []drawnPicture {
	t.Helper()
	var out []drawnPicture
	for _, d := range format.All() {
		props := map[string]format.Property{}
		for _, p := range d.Properties {
			props[p.Name] = p
		}
		_, w := props["width"]
		_, h := props["height"]
		if !w || !h {
			continue
		}
		out = append(out, drawnPicture{name: d.ID, format: d, setting: map[string]string{"width": "160", "height": "120"}})
		if content, ok := props["content"]; ok && slices.Contains(content.Choices, "placeholder") {
			out = append(out, drawnPicture{name: d.ID + "-placeholder", format: d,
				setting: map[string]string{"width": "320", "height": "240", "content": "placeholder"}})
		}
	}
	return out
}

// minimumAtOneByte is the minimum a request for 1 B is refused with.
func minimumAtOneByte(t *testing.T, c drawnPicture) int64 {
	t.Helper()
	_, err := c.format.Generator.Plan(format.Request{Bytes: 1, Label: true, Seed: 7, Properties: c.setting})
	var below *format.BelowMinimumError
	if !errors.As(err, &below) {
		t.Fatalf("asking for 1 B gave %v, where a refusal naming the minimum was due", err)
	}
	return below.Minimum
}

// drawnAt writes the file of size bytes and returns what it draws.
func drawnAt(t *testing.T, c drawnPicture, size int64) []byte {
	t.Helper()
	p, err := c.format.Generator.Plan(format.Request{Bytes: size, Label: true, Seed: 7, Properties: c.setting})
	if err != nil {
		t.Fatalf("planning %d B: %v", size, err)
	}
	if drawn, _ := p.Properties[format.PropertyLabelEmbedded].(bool); !drawn {
		t.Fatalf("a %d B file says it carries no label, so comparing it says nothing about the label", size)
	}
	var buf bytes.Buffer
	rec := &format.MapRecorder{W: &buf}
	if err := c.format.Generator.Write(t.Context(), rec, p); err != nil {
		t.Fatalf("writing %d B: %v", size, err)
	}
	if int64(buf.Len()) != size {
		t.Fatalf("asked for %d B and wrote %d B", size, buf.Len())
	}
	part := drawnPart(buf.Bytes(), rec.Map(size, true), c.format.ID)
	if len(part) == 0 {
		t.Fatalf("a %d B file has neither content marked apart from its padding nor its label written as text, so there is nothing to compare", size)
	}
	return part
}

// drawnPart is what a file draws: the bytes its map marks as content where
// the map marks padding apart from them, or else the label written into the
// drawing as text, up to the markup that closes it.
func drawnPart(data []byte, regions []format.Region, id string) []byte {
	padded := slices.ContainsFunc(regions, func(r format.Region) bool { return r.Role == format.RolePadding && r.Length > 0 })
	if !padded {
		at := bytes.Index(data, []byte("tfg - "+id+" - "))
		if at < 0 {
			return nil
		}
		end := bytes.IndexByte(data[at:], '<')
		if end < 0 {
			return nil
		}
		return data[at : at+end]
	}
	var out []byte
	for _, r := range regions {
		if r.Role == format.RoleContent && r.Length > 0 && r.At+r.Length <= int64(len(data)) {
			out = append(out, data[r.At:r.At+r.Length]...)
		}
	}
	return out
}
