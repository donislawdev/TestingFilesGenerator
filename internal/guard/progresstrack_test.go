package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// The empty part of a progress bar is a groove and not a bar.
//
// The toolkit draws the unfilled part in the accent colour at half alpha -
// progressbar.go, progressBlendColor - so the groove came out the same hue as
// the fill and only lighter. Measured off two renders of one run on
// 2026-08-20: at 1.2 s the whole width was #4A6E8B, which is the empty track,
// and at 6 s the fill reached 24 per cent. A bar at nothing was a solid blue
// bar across the foot of the window.
//
// The numbers say it better than the picture. Before:
//
//	fill against track   2.49
//	track against panel  2.80
//
// The empty part stood out against its surroundings MORE than the full part
// stood out against the empty. That is the wrong way round for something read
// out of the corner of an eye, and telling them apart needed a comparison of
// two shades of one colour.
//
// So the assertion is the relationship rather than either number: what is
// filled has to be louder against the track than the track is against the
// panel behind it. A guard pinning the colours would pass the day somebody
// picks two different wrong ones.
func TestTheFilledPartOfAProgressBarIsLouderThanTheEmptyPart(t *testing.T) {
	bar := parts.NewProgress()
	bar.Resize(fyne.NewSize(800, parts.SlimHeight))
	bar.SetValue(50)
	rects := rectanglesOf(test.WidgetRenderer(bar).Objects())
	if len(rects) != 2 {
		t.Fatalf("a progress bar is drawn out of %d rectangles, and this guard reads two - the groove and the fill", len(rects))
	}
	track, fill := rects[0].FillColor, rects[1].FillColor
	panel := parts.PaletteColour(parts.ColorNamePanel, theme.VariantDark)

	fillOnTrack := contrast(fill, track)
	trackOnPanel := contrast(track, panel)

	if fillOnTrack <= trackOnPanel {
		t.Errorf("the fill stands against the track at %.2f and the track stands against the panel at %.2f."+
			" An empty bar louder than a full one is a bar nobody can read at a glance",
			fillOnTrack, trackOnPanel)
	}
	// Named as well as compared, because "louder than the groove" is satisfied
	// by two colours that are both nearly invisible.
	if fillOnTrack < 3 {
		t.Errorf("the fill stands against the track at only %.2f, so how far along a run is takes a second look", fillOnTrack)
	}
}

// rectanglesOf is the plain shapes an object list holds, in the order they are
// drawn.
//
// The colours are read off the control rather than out of the palette, and
// that distinction is the guard. The first version of this asked the palette
// what the two names resolve to - which is true whatever the widget does with
// them, so a mutation painting the groove in the accent colour left it green.
// Caught by the mutation runner and by nothing else.
func rectanglesOf(objects []fyne.CanvasObject) []*canvas.Rectangle {
	var out []*canvas.Rectangle
	for _, o := range objects {
		if rect, ok := o.(*canvas.Rectangle); ok {
			out = append(out, rect)
		}
	}
	return out
}

// Nothing done draws nothing, and everything done draws everything.
//
// The half of this a colour cannot answer. A track whose fill is laid out at
// full width whatever the value would pass every colour check above and would
// say a run had finished the moment it started.
func TestAProgressBarDrawsWhatItWasToldAndNotMore(t *testing.T) {
	bar := parts.NewProgress()
	bar.Resize(fyne.NewSize(800, parts.SlimHeight))

	widths := map[float64]float32{}
	for _, value := range []float64{0, 50, 100} {
		bar.SetValue(value)
		renderer := test.WidgetRenderer(bar)
		renderer.Layout(bar.Size())
		widths[value] = filledWidth(renderer.Objects())
	}

	if widths[0] != 0 {
		t.Errorf("a bar at nothing draws %.1f px of fill, so a run looks started before it is", widths[0])
	}
	if widths[100] < 799 {
		t.Errorf("a bar at its maximum draws %.1f px of 800, so a finished run never looks finished", widths[100])
	}
	if widths[50] <= widths[0] || widths[50] >= widths[100] {
		t.Errorf("half way draws %.1f px, against %.1f at nothing and %.1f at the end."+
			" The fill has to move with the value or it is a picture rather than a report",
			widths[50], widths[0], widths[100])
	}
}

// filledWidth is the width of the rectangle drawn in the fill colour, found
// by its colour rather than by its place in the list.
func filledWidth(objects []fyne.CanvasObject) float32 {
	fill := parts.PaletteColour(theme.ColorNamePrimary, theme.VariantDark)
	for _, o := range objects {
		rect, ok := o.(*canvas.Rectangle)
		if !ok || rect.FillColor != fill {
			continue
		}
		return rect.Size().Width
	}
	return -1
}
