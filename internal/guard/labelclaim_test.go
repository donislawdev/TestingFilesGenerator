package guard

import (
	"image"
	"strconv"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/imagelabel"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/video"
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

// A film says it carries its label only when one of its pictures shows the
// text whole under the square.
//
// The square steps across a film in ten places and is drawn over the label in
// the label's own ink. On a low wide film it rides across the text, and until
// 2026-11-03 a film of 1920x64 and one change drew it over "tfg -" in its only
// picture and said label_embedded true (O288 of docs/OBSERVATIONS.md). Asked
// of the pixels: the text rows of every picture are compared with the same
// picture of the film made tall, where the square passes under the label, over
// the columns the label's ink covers - measured from the label drawn on its
// own. Across heights at three scales and widths around the one where the
// second of two pictures first clears the text, so the answer is held at both
// edges it can move at, and each series has to give both answers.
func TestAFilmSaysItCarriesItsLabelOnlyWhenAPictureShowsItWhole(t *testing.T) {
	const seed, tall = 7, 256
	label := core.Label("webm", 1<<20, seed)
	across := len(label)*4 + 4 // the width that draws it at scale 1, as in the guard above
	timeline := func(changes int64) video.Timeline {
		tl, err := video.NewTimeline("webm", changes*1000, 60_000, 1000, 30)
		if err != nil {
			t.Fatal(err)
		}
		return tl
	}
	// shown says whether picture c of the film leaves the label's ink as the
	// tall film shows it, over every row and column the ink reaches. The tall
	// picture is the same at every height, so it is painted once.
	talls := map[[2]int64]video.Planes{}
	shown := func(w, h int, tl video.Timeline, ink image.Rectangle, c int64) bool {
		got := video.WholePicture(w, h, seed, label, tl, c)
		key := [2]int64{int64(w)*1000 + tl.Changes(), c}
		want, ok := talls[key]
		if !ok {
			want = video.WholePicture(w, tall, seed, label, tl, c)
			talls[key] = want
		}
		for y := ink.Min.Y; y < ink.Max.Y; y++ {
			for x := ink.Min.X; x < ink.Max.X; x++ {
				if got.Y[y*w+x] != want.Y[y*w+x] {
					return false
				}
			}
		}
		return true
	}
	ask := func(w, h int, tl video.Timeline) (said, seen bool, compared bool) {
		said = video.Choice{Width: w, Height: h, Seed: seed, Label: label}.Labelled(tl)
		if !video.Labelled(w, label) || !imagelabel.Readable(w, h, len(label)) {
			return said, false, false
		}
		ink := labelInkBounds(w, label)
		if text := imagelabel.TextRect(w, len(label)); ink != text {
			t.Fatalf("at %d px across the label's ink covers %v and TextRect says %v - this guard reads the ink and the film asks TextRect, so the two have to agree on its label", w, ink, text)
		}
		for c := range tl.Changes() {
			if shown(w, h, tl, ink, c) {
				return said, true, true
			}
		}
		return said, false, true
	}
	type series struct {
		name  string
		cases [][2]int
		tl    video.Timeline
	}
	var all []series
	for _, changes := range []int64{1, 2, 10} {
		for _, scale := range []int{1, 2, 4} {
			s := series{name: "scale " + strconv.Itoa(scale) + ", " + strconv.FormatInt(changes, 10) + " change(s), heights", tl: timeline(changes)}
			for h := 1; h <= 120; h++ {
				s.cases = append(s.cases, [2]int{across * scale, h})
			}
			all = append(all, s)
		}
	}
	// The square of the second of two pictures stands a ninth of the way along
	// the room it leaves, so at 64 px tall, where it is 64 px and rides over
	// the text, a film 9 x + 64 px wide has it at column x. Walked over the
	// last two characters and past the text's right edge, which is where it
	// stops hiding anything - not at the edge, because the last column of a 7
	// is ink all the way down.
	right := imagelabel.TextRect(across*4, len(label)).Max.X
	step := imagelabel.CellsOf(across*4, len(label)).Step
	edge := series{name: "2 changes at 64 px, widths", tl: timeline(2)}
	for x := right - 2*step; x <= right+2; x++ {
		edge.cases = append(edge.cases, [2]int{9*x + 64, 64})
	}
	all = append(all, edge)

	for _, s := range all {
		answers := map[bool]int{}
		for _, wh := range s.cases {
			said, seen, compared := ask(wh[0], wh[1], s.tl)
			if !compared {
				if said {
					t.Errorf("%s: a film of %dx%d says it carries its label, and the label is not whole in its height", s.name, wh[0], wh[1])
				}
				continue
			}
			answers[seen]++
			if said != seen {
				t.Errorf("%s: a film of %dx%d says label_embedded %v, and a picture of it shows the label whole: %v", s.name, wh[0], wh[1], said, seen)
			}
		}
		if answers[true] == 0 || answers[false] == 0 {
			t.Errorf("%s: the pictures showed the label whole %d time(s) and covered %d - the series never reached the edge it is there for", s.name, answers[true], answers[false])
		}
	}
}

// And the plan of both film formats says it, with a note when the square is
// what hides the label: the film of O288, and the same film at the default
// ten changes, whose last picture shows the label whole.
func TestAFilmWhoseSquareCoversItsLabelSaysSoInItsPlan(t *testing.T) {
	asked := 0
	for _, id := range format.IDs() {
		d, err := format.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := settingNamed(d, video.SettingChangeInterval); !ok {
			continue
		}
		asked++
		for _, c := range []struct {
			duration string
			want     bool
		}{{"1s", false}, {"10s", true}} {
			props := map[string]string{"width": "1920", "height": "64", video.SettingDuration: c.duration}
			r := format.Request{Seed: 1, Label: true, Properties: props}
			r.Bytes = format.SmallestRemembered(d, r) + 64<<10
			p, err := d.Generator.Plan(r)
			if err != nil {
				t.Fatalf("%s 1920x64 for %s: %v", id, c.duration, err)
			}
			embedded, _ := p.Properties[format.PropertyLabelEmbedded].(bool)
			omitted := 0
			for _, n := range p.Notes {
				if n.Code == "label_omitted" {
					omitted++
				}
			}
			if embedded != c.want || omitted != map[bool]int{true: 0, false: 1}[c.want] {
				t.Errorf("%s 1920x64 for %s says label_embedded %v with %d label_omitted note(s), want %v with %d - notes %v",
					id, c.duration, embedded, omitted, c.want, map[bool]int{true: 0, false: 1}[c.want], p.Notes)
			}
		}
	}
	if asked < 2 {
		t.Fatalf("only %d format(s) declare %s - the registry was not read", asked, video.SettingChangeInterval)
	}
}

// labelInkBounds is the rectangle the pixels of a label drawn alone across
// width cover, read from the pixels.
func labelInkBounds(width int, label string) image.Rectangle {
	img := image.NewRGBA(image.Rect(0, 0, width, 64))
	imagelabel.Draw(img, label)
	band := img.RGBAAt(0, 0)
	var ink image.Rectangle
	for y := range 64 {
		for x := range width {
			if c := img.RGBAAt(x, y); c != band && c.A != 0 {
				ink = ink.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return ink
}

func hasNote(notes []format.Note, code string) bool {
	for _, n := range notes {
		if n.Code == code {
			return true
		}
	}
	return false
}
