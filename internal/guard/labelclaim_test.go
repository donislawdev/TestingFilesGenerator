package guard

import (
	"image"
	"strconv"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/imagelabel"
)

// A picture says it carries its label only when the text of the label is whole
// in it.
//
// The label is drawn at a scale chosen from the width, so a wide picture
// draws it large, and a low one cuts it off at its foot. Until 2026-10-27 the
// manifest asked only the width: a BMP 512 px wide and 1 px tall said
// label_embedded true with not one pixel of text in it, and so did a film
// 1920 px wide and 16 px tall whose label needs 22 - eleven formats in all
// (docs/STABILITY-REVIEW-2026-10-26.md, R14). The formats are read from the
// registry, so one added with a width and a height is asked too, and every
// one is asked at a height with room as well, so "never" cannot pass for
// "only when it is true".
func TestAPictureSaysItCarriesItsLabelOnlyWhenItsTextIsWhole(t *testing.T) {
	asked := 0
	for _, id := range format.IDs() {
		d, err := format.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		width, wok := settingNamed(d, "width")
		height, hok := settingNamed(d, "height")
		if !wok || !hok {
			continue
		}
		asked++
		// As wide as 1920 px where the format goes that far, which draws the
		// label four times larger in the films and the pictures alike.
		wide := min(width.Max, 1920)
		claims := func(tall int64) (bool, []format.Note) {
			t.Helper()
			props := map[string]string{"width": strconv.FormatInt(wide, 10), "height": strconv.FormatInt(tall, 10)}
			r := format.Request{Seed: 1, Label: true, Properties: props}
			// Room in the bytes as well, so that a label left out is about
			// the picture and not about the size - an SVG at its smallest has
			// no bytes for one.
			r.Bytes = format.SmallestRemembered(d, r) + 64<<10
			p, err := d.Generator.Plan(r)
			if err != nil {
				t.Fatalf("%s at %dx%d: %v", id, wide, tall, err)
			}
			embedded, ok := p.Properties[format.PropertyLabelEmbedded].(bool)
			if !ok {
				t.Fatalf("%s at %dx%d has no label_embedded, or not a yes or no: %v", id, wide, tall, p.Properties[format.PropertyLabelEmbedded])
			}
			return embedded, p.Notes
		}

		if embedded, notes := claims(height.Min); embedded || !hasNote(notes, "label_omitted") {
			t.Errorf("%s at %dx%d says label_embedded %v with notes %v - its label cannot be whole in %d px",
				id, wide, height.Min, embedded, notes, height.Min)
		}
		if embedded, notes := claims(max(height.Min, 64)); !embedded {
			t.Errorf("%s at %dx%d says it carries no label (%v) - there is room for one", id, wide, max(height.Min, 64), notes)
		}
	}
	if asked < 11 {
		t.Fatalf("only %d format(s) declare a width and a height - the registry was not read", asked)
	}
}

// A label is readable from the first height that holds every row its text is
// drawn in, and not one row before - asked of the pixels Draw puts down rather
// than of the arithmetic beside it, at each of the four scales a label is
// drawn at. The guard above asks the lowest height and one with room, and an
// answer one row out at the boundary passed it (found by review, 2026-10-27).
func TestALabelIsReadableFromTheFirstHeightThatHoldsItsText(t *testing.T) {
	label := core.Label("bmp", 2048, 1)
	across := len(label)*4 + 4 // a glyph and its gap is four columns at scale 1, with the padding either side
	for scale := 1; scale <= 4; scale++ {
		width := across * scale
		img := image.NewRGBA(image.Rect(0, 0, width, 64))
		imagelabel.Draw(img, label)
		band := img.RGBAAt(0, 0)
		last := -1
		for y := 0; y < 64; y++ {
			for x := 0; x < width; x++ {
				if c := img.RGBAAt(x, y); c != band && c.A != 0 {
					last = y
				}
			}
		}
		if last < 0 {
			t.Fatalf("no text was drawn at %d px across - the guard did not reach a drawn label", width)
		}
		holds := last + 1
		if imagelabel.Readable(width, holds-1, len(label)) || !imagelabel.Readable(width, holds, len(label)) {
			t.Errorf("at %d px across the text is drawn in the top %d rows, and Readable says %v at %d rows and %v at %d",
				width, holds, imagelabel.Readable(width, holds-1, len(label)), holds-1, imagelabel.Readable(width, holds, len(label)), holds)
		}
	}
}

func hasNote(notes []format.Note, code string) bool {
	for _, n := range notes {
		if n.Code == code {
			return true
		}
	}
	return false
}
