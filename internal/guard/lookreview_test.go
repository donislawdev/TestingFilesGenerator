package guard

import (
	"image"
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// Guards for points of the look review of 2026-09-24
// (docs/GUI-LOOK-REVIEW-2026-09-24.md) that nothing else holds. Each one reads
// what is drawn, because each of them was a thing correctly written in the code
// and wrong on the screen (GUI rule 10).

// TestAButtonBesideABoxIsDrawnAsTallAsTheBox lays a box to type in and a button
// at the same height and reads the rows each one paints. Until 2026-09-24 the
// button's edge was stroked across its bounds and the box's inside them, so
// Choose stood a pixel taller than the directory box at each end (UI-012).
func TestAButtonBesideABoxIsDrawnAsTallAsTheBox(t *testing.T) {
	ourTheme(t)
	e := parts.NewEntry()
	e.SetText("/tfg/out")
	boxed, _ := parts.WithRing(e)
	b := parts.NewButton(parts.Secondary, "Choose...", func() {})
	w := test.NewTempWindow(t, container.NewWithoutLayout(boxed, b))
	height := boxed.MinSize().Height
	boxed.Resize(fyne.NewSize(140, height))
	boxed.Move(fyne.NewPos(10, 10))
	b.Resize(fyne.NewSize(100, height))
	b.Move(fyne.NewPos(170, 10))
	w.Resize(fyne.NewSize(300, 60))
	picture := w.Canvas().Capture()

	page := parts.PaletteColour(theme.ColorNameBackground, theme.VariantDark)
	rows := func(x int) (first, last int) {
		first, last = -1, -1
		for y := 0; y < picture.Bounds().Dy(); y++ {
			if !sameColour(picture.At(x, y), page) {
				if first < 0 {
					first = y
				}
				last = y
			}
		}
		return first, last
	}
	boxTop, boxBottom := rows(60)
	buttonTop, buttonBottom := rows(220)
	if boxTop < 0 || buttonTop < 0 {
		t.Fatal("the box or the button drew nothing where it stands, so nothing was compared")
	}
	if boxTop != buttonTop || boxBottom != buttonBottom {
		t.Errorf("laid out at one height, the box is drawn over rows %d to %d and the button over %d to %d",
			boxTop, boxBottom, buttonTop, buttonBottom)
	}
}

// TestDonateDrawsARedHeartThatGoesOutWhenOff asks both Donate buttons - the
// bar's and the About card's - for the heart, and a Donate button drawn alone
// for red on the screen: there at rest, gone when it is switched off, since a
// control off is quieter than at rest (UI-015, the owner's choice of red).
func TestDonateDrawsARedHeartThatGoesOutWhenOff(t *testing.T) {
	for _, tab := range []string{text.TabOneTarget(), text.TabAbout()} {
		content, _ := screenInAWindow(t, tab)
		donate := buttonNamed(content, text.ButtonDonate())
		if donate == nil {
			t.Fatalf("the %s screen has no %q button", tab, text.ButtonDonate())
		}
		if donate.Icon != parts.HeartIcon() {
			t.Errorf("%q on the %s screen carries no heart", text.ButtonDonate(), tab)
		}
	}

	ourTheme(t)
	b := parts.NewButton(parts.Quiet, "Donate", func() {}).WithHeart()
	w := test.NewTempWindow(t, container.NewWithoutLayout(b))
	b.Resize(b.MinSize())
	w.Resize(b.MinSize().Add(fyne.NewSquareSize(20)))
	red := parts.PaletteColour(theme.ColorNameError, theme.VariantDark)
	if n := pixelsNear(w.Canvas().Capture(), red); n < 20 {
		t.Errorf("a Donate button with its heart draws %d pixels of the red the heart is filled with", n)
	}
	b.Disable()
	if n := pixelsNear(w.Canvas().Capture(), red); n != 0 {
		t.Errorf("switched off, a Donate button still draws %d pixels of red", n)
	}
}

// TestTheBarsRailStandsOnTheFormsLeftEdge reads where Donate's ink starts -
// the heart, the first thing it draws - and where the form's first field name
// does. The rail stood at the bar's own left edge from the owner's decision of
// 2026-08-19 until the owner reversed it on 2026-09-24 (UI-006): one left edge
// for everything in the bar that is not centred, and the form's.
//
// The ink and not the button's box: a quiet button draws a surface only under
// the pointer, so its box is invisible and its words start the room round
// them further in. The first version of this guard measured the box, passed,
// and the render showed the heart 16 px right of the line under it.
func TestTheBarsRailStandsOnTheFormsLeftEdge(t *testing.T) {
	for _, tab := range []string{text.TabOneTarget(), text.TabRecipe()} {
		content, _ := screenInAWindow(t, tab)
		donate := buttonNamed(content, text.ButtonDonate())
		if donate == nil {
			t.Fatalf("the %s screen has no %q button", tab, text.ButtonDonate())
		}
		rail, ok := objectBox(content, donate)
		name, found := labelBox(content, text.FieldFormat())
		if !ok || !found {
			t.Fatalf("on the %s screen, %q or the name %q is not laid out", tab, text.ButtonDonate(), text.FieldFormat())
		}
		ink := float32(-1)
		for _, o := range test.WidgetRenderer(donate).Objects() {
			if picture, is := o.(*canvas.Image); is && picture.Visible() {
				ink = rail.X + picture.Position().X
			}
		}
		if ink < 0 {
			t.Fatalf("%q on the %s screen draws no heart, so where its ink starts cannot be read", text.ButtonDonate(), tab)
		}
		if off := ink - name.X; off > 1 || off < -1 {
			t.Errorf("on the %s screen the ink of %q starts at x=%.1f and the form's names at x=%.1f", tab, text.ButtonDonate(), ink, name.X)
		}
	}
}

// pixelsNear counts the pixels of a picture within a small distance of one
// colour - the edge of a shape is blended, its middle is the colour itself.
func pixelsNear(picture image.Image, want color.Color) int {
	wr, wg, wb, _ := want.RGBA()
	near := func(a, b uint32) bool {
		d := int(a>>8) - int(b>>8)
		return d > -24 && d < 24
	}
	n := 0
	for y := picture.Bounds().Min.Y; y < picture.Bounds().Max.Y; y++ {
		for x := picture.Bounds().Min.X; x < picture.Bounds().Max.X; x++ {
			r, g, b, _ := picture.At(x, y).RGBA()
			if near(r, wr) && near(g, wg) && near(b, wb) {
				n++
			}
		}
	}
	return n
}
