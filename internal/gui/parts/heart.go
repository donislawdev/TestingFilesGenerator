package parts

import (
	_ "embed"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// WithHeart puts the heart in front of a button's words, in red - the error
// colour, which is the red this palette already has and has measured, so the
// heart brings no colour of its own into the window. The owner's choice of
// 2026-09-24, for both Donate buttons (review UI-015). The words keep the
// look's ink, and a button switched off draws the heart in its own.
func (b *Button) WithHeart() *Button {
	b.Icon = heart
	b.iconInk = theme.ColorNameError
	b.Refresh()
	return b
}

// HeartIcon is the heart drawn in front of Donate.
//
// Drawn here, from two circles and a point, because the toolkit has none:
// fyne v2.8.1 ships 97 icons in theme/icons and not one is a heart or a
// "favourite", checked in the module on 2026-09-24. Taking one from an icon
// set would have brought that set's licence and its notice along with it, and
// the owner chose this instead - the route the application's own icon took
// (docs/LICENSING.md, section 11). So this is the project's own drawing, under
// the project's licence, with nobody's copyright to carry.
//
// The geometry, so it can be checked rather than trusted: two circles of
// radius 5 with centres at (7.5, 9) and (16.5, 9) meet at the notch (12,
// 6.821), and the lines from the tip at (12, 21) touch them at (3.874, 12.443)
// and (20.126, 12.443). The square is 24 by 24 like the toolkit's own icons,
// and the path is filled with no colour of its own, so it is tinted by name
// the way theirs are.
//
// The file carries no xmlns: the toolkit matches the svg element by its local
// name (fyne v2.8.1 internal/svg/svg.go), and an address in a shipped file is
// what the guard against a way out reads as one.
func HeartIcon() fyne.Resource { return heart }

//go:embed heart.svg
var heartSVG []byte

var heart = fyne.NewStaticResource("heart.svg", heartSVG)
