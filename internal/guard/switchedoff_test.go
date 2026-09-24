package guard

import (
	"image"
	"image/color"
	"math"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// A control switched off is never drawn brighter than the same control at
// rest. The form is switched off for the length of every run, so this is what
// a person sees each time they press Generate.
//
// It was, in three controls at once, found by measuring the catalogue on
// 2026-09-24 (docs/GUI-LOOK-REVIEW-2026-09-24.md, UI-001 to UI-003): the quiet
// button and the i beside a field name rest in the hint's ink and went to the
// brighter disabled ink when off, the switch's edge went from the edge of a
// box to that same bright ink, and the toolkit draws a switched off box's
// border in it too. A frozen form read as more there than a live one.
//
// Measured on the pixels, because every one of the three was a colour
// correctly named in the code and wrong on the screen (GUI rule 10). A box to
// type in is measured along its top edge only: its value is the brightest
// thing in it on or off, and a comparison of the whole box would be decided by
// the words and blind to the edge.
func TestAControlSwitchedOffIsNeverBrighterThanAtRest(t *testing.T) {
	app := test.NewApp()
	app.Settings().SetTheme(parts.Theme())
	t.Cleanup(func() { test.NewApp() })

	type control struct {
		name  string
		build func() (fyne.CanvasObject, fyne.Disableable)
		edge  bool
	}
	for _, c := range []control{
		{"a quiet button", func() (fyne.CanvasObject, fyne.Disableable) {
			b := parts.NewButton(parts.Quiet, "Donate", func() {})
			return b, b
		}, false},
		{"the i beside a field name", func() (fyne.CanvasObject, fyne.Disableable) {
			b := parts.NewGlyphButton(theme.InfoIcon(), func() {})
			return b, b
		}, false},
		{"a switch", func() (fyne.CanvasObject, fyne.Disableable) {
			s := parts.NewToggle(nil)
			return s, s
		}, false},
		{"a box to type in", func() (fyne.CanvasObject, fyne.Disableable) {
			e := parts.NewEntry()
			e.SetText("2048")
			boxed, _ := parts.WithRing(e)
			return boxed, e
		}, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			obj, off := c.build()
			rest := brightestOf(t, obj, c.edge)
			off.Disable()
			switched := brightestOf(t, obj, c.edge)
			if switched > rest+0.002 {
				t.Errorf("%s switched off is drawn at a relative luminance of %.3f and at rest at %.3f - off reads as more there than on",
					c.name, switched, rest)
			}
		})
	}
}

// TestASwitchThatIsOffStillShowsItsValue compares a ticked switch and an
// unticked one, both switched off. Until 2026-09-24 a switch that was off hid
// its tick whatever it held, so "Label in each file" ticked read as unticked
// for the length of every run.
func TestASwitchThatIsOffStillShowsItsValue(t *testing.T) {
	app := test.NewApp()
	app.Settings().SetTheme(parts.Theme())
	t.Cleanup(func() { test.NewApp() })

	picture := func(on bool) image.Image {
		s := parts.NewToggle(nil)
		s.SetChecked(on)
		s.Disable()
		w := test.NewWindow(container.NewWithoutLayout(s))
		t.Cleanup(w.Close)
		s.Resize(s.MinSize())
		w.Resize(fyne.NewSize(60, 60))
		return w.Canvas().Capture()
	}
	ticked, unticked := picture(true), picture(false)
	differ := 0
	for y := 0; y < parts.GlyphButton; y++ {
		for x := 0; x < parts.GlyphButton; x++ {
			if ticked.At(x, y) != unticked.At(x, y) {
				differ++
			}
		}
	}
	if differ < parts.GlyphButton {
		t.Errorf("a ticked switch and an unticked one, both switched off, differ in %d pixels - a run's frozen form does not show what it was frozen with", differ)
	}
}

// brightestOf draws a control alone on the page and gives the relative
// luminance of its brightest pixel - along its top edge only, away from the
// rounded corners, when edge is set.
func brightestOf(t *testing.T, obj fyne.CanvasObject, edge bool) float64 {
	t.Helper()
	w := test.NewWindow(container.NewWithoutLayout(obj))
	defer w.Close()
	size := obj.MinSize().Max(fyne.NewSize(parts.NumericWidth, 0))
	obj.Resize(size)
	obj.Move(fyne.NewPos(8, 8))
	w.Resize(size.Add(fyne.NewSize(16, 16)))
	picture := w.Canvas().Capture()

	band := image.Rect(8, 8, 8+int(size.Width), 8+int(size.Height))
	if edge {
		// The toolkit draws a box's border inside the box rather than on its
		// edge - measured on 2026-09-24 at 4 px in - so the band is the room
		// between the box's edge and where its words can start, and it keeps
		// clear of the rounded corners.
		inset := int(parts.RadiusField + parts.ControlInset)
		band = image.Rect(8+inset, 8, 8+int(size.Width)-inset, 8+int(parts.ControlInset))
	}
	if band.Empty() {
		t.Fatalf("the control is %.0fx%.0f, which leaves nothing to measure", size.Width, size.Height)
	}
	var brightest float64
	for y := band.Min.Y; y < band.Max.Y; y++ {
		for x := band.Min.X; x < band.Max.X; x++ {
			brightest = math.Max(brightest, luminanceOf(picture.At(x, y)))
		}
	}
	if brightest == luminanceOf(parts.PaletteColour(theme.ColorNameBackground, theme.VariantDark)) {
		t.Fatal("nothing but the page was drawn where the control stands, so this guard is measuring nothing")
	}
	return brightest
}

// luminanceOf is the relative luminance WCAG defines, of one colour.
func luminanceOf(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	channel := func(v uint32) float64 {
		s := float64(v) / 0xffff
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(r) + 0.7152*channel(g) + 0.0722*channel(b)
}
