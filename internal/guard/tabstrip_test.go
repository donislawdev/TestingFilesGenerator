package guard

import (
	"image"
	"image/color"
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// The tab somebody is on is the one that stands out.
//
// The toolkit draws the selected tab in the accent colour and every other tab
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
// preference: the strip is built inside the toolkit's own renderer, so the
// walk this package uses does not reach it. A guard on the theme object would
// prove the theme was written and say nothing about which colour the strip
// took - that mistake was made twice in this run of changes and the mutation
// runner caught both.
func TestTheTabSomebodyIsOnIsTheOneThatStandsOut(t *testing.T) {
	app := test.NewApp()
	app.Settings().SetTheme(parts.Theme())
	t.Cleanup(func() { test.NewApp() })

	host := &fakeHost{}
	window.Open(host)
	if host.content == nil {
		t.Fatal("opening the window put no screen in it")
	}
	w := test.NewWindow(host.content)
	t.Cleanup(w.Close)
	w.Resize(window.OpenSize)

	picture := w.Canvas().Capture()
	page := parts.PaletteColour(theme.ColorNameBackground, theme.VariantDark)

	// The strip runs across the top. The first tab is the one selected when the
	// window opens, and the rest follow it along the same band.
	chosen := boldestIn(picture, image.Rect(0, 8, 130, 40), page)
	rest := boldestIn(picture, image.Rect(140, 8, 560, 40), page)

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
