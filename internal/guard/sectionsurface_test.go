package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// A section draws a surface of its own, so grouping is something you can see.
//
// This guard exists because the obvious way to group things was measured and
// found to do nothing. Until 2026-08-12 a section was a widget.Card, and a card
// fills itself with ColorNameBackground - the toolkit's own card.go, line 44 -
// which is the same name the page is painted with. Rendered and sampled, the
// panel and the page were both #1E1E1E: 0.00 L* apart, with nothing at the edge
// but a shadow going darker than the page it lay on. Three sections on the
// generate screen and the form read as one wall.
//
// What makes that worth a guard rather than a fix is how it hid. Every
// structural check passed, because the cards were there and the fields were
// inside them. The only thing wrong was a colour nobody had measured, and the
// only way to find it was to render the screen and sample two pixels.
//
// So this asks the tree rather than the pixels, on purpose. A pixel sample
// needs a coordinate, and the coordinate moves with every font and every
// toolkit release - the file next door says why this project keeps no image
// goldens. What cannot drift is the question of whether a section puts a filled
// shape behind its content at all, and what colour that shape is.
func TestASectionDrawsItsOwnSurface(t *testing.T) {
	for _, subject := range []struct {
		what string
		tree fyne.CanvasObject
	}{
		{"a section", parts.Section("Output", widget.NewLabel("a field"))},
		// The action bar had the identical defect for the identical reason, and
		// it matters more there: the bar is pinned over a form that scrolls
		// underneath it, so a bar with no surface is a bar with text sliding
		// through its buttons. That is what it was built to stop.
		{"the action bar", parts.ActionBar(widget.NewButton("Generate", nil))},
	} {
		var found []*canvas.Rectangle
		walk(subject.tree, func(o fyne.CanvasObject) {
			if rect, ok := o.(*canvas.Rectangle); ok {
				found = append(found, rect)
			}
		})

		if len(found) != 1 {
			t.Errorf("%s draws %d filled shapes and should draw exactly one - the surface it stands on",
				subject.what, len(found))
			continue
		}
		surface := found[0]

		page := parts.PaletteColour(theme.ColorNameBackground, theme.VariantDark)
		if surface.FillColor == page {
			t.Errorf("%s is filled with the page colour, so it is not a surface at all", subject.what)
		}
		if want := parts.PaletteColour(parts.ColorNamePanel, theme.VariantDark); surface.FillColor != want {
			t.Errorf("%s is filled with %v and the palette says the panel is %v",
				subject.what, surface.FillColor, want)
		}

		// The edge carries the visibility, because the fill on its own is four
		// L* and that is a surface you sense rather than see. A stroke of zero
		// width is a panel with no boundary, which is where this started.
		if surface.StrokeWidth <= 0 {
			t.Errorf("%s has no line round it, so its edge is left to a shadow", subject.what)
		}
		if surface.StrokeColor == surface.FillColor {
			t.Errorf("%s has a line the same colour as its fill, which draws nothing", subject.what)
		}
	}
}

// The three surfaces that stack on one another are each told from the one
// beneath.
//
// A page holds panels and a panel holds input boxes, so this is a stack of
// three and every step of it has to be visible. The trap is specific and was
// nearly walked into on 2026-08-12: docs/UX.md section 8.2 already records a
// surface colour, #2E2E30, and reaching for it would have been the obvious
// move. That value is what an input box is painted with - a panel painted with
// it swallows every field standing on it, and the screen would have looked
// grouped while the controls disappeared.
//
// The threshold is a share of the distance rather than a number of L*, and that
// is deliberate. The two variants have different room to work in - 7.7 L*
// between page and input on the dark one, 5.6 on the light - so a fixed
// threshold would be generous in one and impossible in the other. What matters
// in both is the same thing: the panel sits between its neighbours and hugs
// neither. Painting it with either neighbour's colour scores zero.
func TestEachSurfaceIsToldFromTheOneUnderIt(t *testing.T) {
	for _, variant := range []struct {
		name string
		v    fyne.ThemeVariant
	}{{"dark", theme.VariantDark}, {"light", theme.VariantLight}} {
		page := parts.PaletteColour(theme.ColorNameBackground, variant.v)
		panel := parts.PaletteColour(parts.ColorNamePanel, variant.v)
		input := parts.PaletteColour(theme.ColorNameInputBackground, variant.v)

		total := lightnessGap(input, page)
		toPage := lightnessGap(panel, page)
		toInput := lightnessGap(input, panel)

		// A third each way. Measured on 2026-08-12 the real split is close to
		// half and half in both variants, so this leaves room for the palette to
		// be adjusted without leaving room for a panel to merge with a
		// neighbour.
		least := total / 3
		if toPage < least {
			t.Errorf("%s: the panel is %.1f L* from the page out of %.1f available, so it barely lifts off it",
				variant.name, toPage, total)
		}
		if toInput < least {
			t.Errorf("%s: an input box is %.1f L* from the panel out of %.1f available, so the fields sink into it",
				variant.name, toInput, total)
		}

		// Text moved house when the panel arrived. Section 8.2 computed every
		// ratio in this palette against the page, and almost none of that text
		// is on the page any more - it is on a panel, which is lighter. That is
		// the same shape as the defect the palette guard caught on its first
		// day: a colour measured against the wrong thing.
		for _, name := range []fyne.ThemeColorName{theme.ColorNameForeground, theme.ColorNamePlaceHolder} {
			if got := contrast(parts.PaletteColour(name, variant.v), panel); got < 4.5 {
				t.Errorf("%s: %s is %.2f:1 on a panel, under the 4.5 a reader needs",
					variant.name, name, got)
			}
		}

		t.Logf("%s: page to panel %.1f L*, panel to input %.1f L*, of %.1f available",
			variant.name, toPage, toInput, total)
	}
}
