package parts

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
)

// Scroll is every scrolling area of the window: the toolkit's own, with one
// notch of a mouse wheel moving it as far as the system says a notch moves.
//
// The toolkit moves every area by a fixed step whatever the system is set to
// (ToolkitNotch in wheel_other.go), and on Windows and Linux that step is half
// of the three lines of text a notch moves by default. Measured on 2026-11-17 with
// tools/probes/guilag: the Single batch screen of otlp with its settings open
// took 32 notches top to bottom, and a notch cost less than a frame to draw -
// the window was not slow, the steps were short (docs/SIZE-FIXES-2026-11-16.md
// section 3.8).
//
// An extension of the toolkit's scroll rather than a copy of it: everything a
// scroll does - the bars, the offset, the shadows, a drag on the bar - stays
// the toolkit's, and only the distance a wheel asks for is answered here. One
// type for every area, so a change of how the wheel feels is a change in one
// place.
type Scroll struct {
	container.Scroll
}

// VScroll is a scrolling area that moves up and down only.
func VScroll(content fyne.CanvasObject) *Scroll {
	s := &Scroll{}
	s.Direction = container.ScrollVerticalOnly
	s.Content = content
	// Before the toolkit's own constructor would, because the first call
	// wins: the area has to be drawn and refreshed as this type, or the wheel
	// would reach the toolkit's scroll underneath and never this one.
	s.ExtendBaseWidget(s)
	return s
}

// Scrolled moves the area by the system's distance for the notches turned.
func (s *Scroll) Scrolled(ev *fyne.ScrollEvent) {
	moved := *ev
	moved.Scrolled.DY = wheelDistance(ev.Scrolled.DY, s.Size().Height)
	s.Scroll.Scrolled(&moved)
}

// WheelLines is how many lines one notch of the wheel moves, as the system
// is set: a negative number is a whole view at a time and nought is not at
// all. Package gui answers it from the system where the system has the
// setting - Windows - and the default stands everywhere else.
var WheelLines = func() int { return WheelLinesDefault }

// wheelLine is one line of the window's text, measured rather than written,
// so a notch keeps its meaning when the text grows or the screen is scaled.
func wheelLine() float32 {
	return fyne.MeasureText("Ag", TextBody, fyne.TextStyle{}).Height
}

// notchDistance is how far one notch moves an area showing view units.
func notchDistance(view float32) float32 {
	lines := WheelLines()
	if lines < 0 {
		return view
	}
	return float32(lines) * wheelLine()
}
