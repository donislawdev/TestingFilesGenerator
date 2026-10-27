package guard

import (
	"strconv"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
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
			embedded, _ := p.Properties[format.PropertyLabelEmbedded].(bool)
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

func hasNote(notes []format.Note, code string) bool {
	for _, n := range notes {
		if n.Code == code {
			return true
		}
	}
	return false
}
