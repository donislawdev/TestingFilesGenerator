package guard

import (
	"image"
	"image/color"
	"math"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// The tab somebody is on is the one that stands out.
//
// The toolkit drew the selected tab in the accent colour and every other tab
// in the ordinary foreground - tabs.go, either side of line 716. Measured off a
// render on 2026-08-20: the SELECTED tab stood at 7.71 against the page and the
// three nobody was on stood at 13.36. The chosen one was the dimmest label in
// the strip, and four names competed at full strength for a strip that has one
// answer.
//
// Not an accessibility fault. The underline carries the same meaning without
// the colour, so UX1 held either way. It is a question of what the eye lands on
// first, which nothing but a picture can answer.
//
// It reads the pixels rather than the widget tree, and that is not a
// preference: what the strip DRAWS is the question, and a guard on the colour
// a word says it has would prove the field was written and say nothing about
// what reached the canvas - that mistake was made twice in this run of changes
// and the mutation runner caught both. The bands it reads are taken from where
// the words ended up rather than from numbers written here: the strip moved
// from the edge of the window to the column on 2026-09-15, and a band pinned
// to the old place would have measured the page and gone red for the wrong
// reason - or, worse, caught half of the chosen word in both bands and gone
// green for no reason, which is what it did that morning.
func TestTheTabSomebodyIsOnIsTheOneThatStandsOut(t *testing.T) {
	app := test.NewApp()
	app.Settings().SetTheme(parts.Theme())
	t.Cleanup(func() { test.NewApp() })

	host := newFakeHost(t)
	window.Open(host)
	if host.content == nil {
		t.Fatal("opening the window put no screen in it")
	}
	w := test.NewWindow(host.content)
	t.Cleanup(w.Close)
	w.Resize(window.LargestOpening)

	strip := tabsIn(host.content)
	if strip == nil {
		t.Fatal("the window has no strip")
	}
	picture := w.Canvas().Capture()
	page := parts.PaletteColour(theme.ColorNameBackground, theme.VariantDark)

	// Each band is the word's own box on the canvas, so a word that moved is
	// still the word that is measured. Which word is chosen is asked of the
	// word, and a strip that marked none would put the chosen strength at
	// zero, which is red rather than quietly green.
	var chosen, rest float64
	for _, word := range strip.Words() {
		at := fyne.CurrentApp().Driver().AbsolutePositionForObject(word)
		size := word.Size()
		band := image.Rect(int(at.X), int(at.Y), int(at.X+size.Width), int(at.Y+size.Height))
		if band.Empty() {
			t.Fatalf("the %q word has no box on the canvas, so this guard is measuring nothing", word.Text())
		}
		got := boldestIn(picture, band, page)
		if word.Chosen() {
			chosen = got
			continue
		}
		rest = math.Max(rest, got)
	}

	if chosen <= rest {
		t.Errorf("the tab somebody is on stands at %.2f against the page and the ones they are not on stand"+
			" at %.2f. A strip where the chosen tab is the quietest label has four answers to a question with one",
			chosen, rest)
	}
}

// boldestIn is the strongest contrast any pixel in a band reaches against the
// page behind it - which is what the eye lands on.
func boldestIn(picture image.Image, band image.Rectangle, page color.Color) float64 {
	best := 0.0
	for y := band.Min.Y; y < band.Max.Y; y++ {
		for x := band.Min.X; x < band.Max.X; x++ {
			if got := contrast(picture.At(x, y), page); got > best {
				best = got
			}
		}
	}
	return best
}
