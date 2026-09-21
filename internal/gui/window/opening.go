package window

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// LargestOpening is the most a first start opens at: as wide as the window
// ever opens, and as tall as it is allowed to be on a screen nobody has
// measured.
//
// The width is a decision of the owner from 2026-08-12. At 720 the form was
// narrower than the 820 it is allowed, so the column cap did nothing and every
// screen scrolled from the moment it opened - and a form that has to be
// scrolled before it can be read is a form nobody sees the shape of.
//
// The height is a ceiling and not the size the window opens at - see
// HowToOpen for what it opens at. It is the one number here that is about
// somebody else's screen rather than about this program: a window taller than
// the screen it opens on cannot be reached at the bottom at all, which is
// worse than one that scrolls, and this toolkit offers no portable way to ask
// how big the screen is - checked in the driver interface on 2026-08-19 and
// again in v2.8.0 on 2026-08-25, there is none. A 1080p screen leaves about
// 1040 px once the taskbar has taken its share, so 1000 fits it with room to
// spare, and the owner's own screen, measured the same day at 3840x2088 of
// usable area, is not the constraint.
//
// Until 2026-09-16 this was called OpenSize and WAS the size the window opened
// at, measured against the forms of 2026-08-19 and typed in. The forms moved
// - the grid of 2026-09-14 alone took 300 px off the single batch screen - and
// the number did not, so a first start opened with 161 px of nothing under
// the form (O202). A size measured once is a size that is wrong the moment
// what it measured moves, and GUI rule 14 says as much: worked out, never
// measured.
var LargestOpening = fyne.NewSize(1000, 1000)

// HowToOpen says what size to open the window at, and whether to put it in the
// middle of the screen.
//
// The two answers travel together because they are one decision. Measured on
// 2026-08-25 with tools/probes/windowsize and tools/probes/windowrect.ps1, on a
// screen with 3840x2088 of usable area:
//
//	asked for 5000x3000, not centred : the window lands at 304,304 and its
//	                                   title bar is on the screen
//	asked for 5000x3000, centred     : the window lands at -1841,-1215 and its
//	                                   title bar is 1215 px above the top
//
// Centring works out the middle from the size that was ASKED for, so a window
// bigger than the screen it comes back on is placed with its title bar off the
// top - and a window whose title bar cannot be reached cannot be moved or
// resized with a mouse at all. That is the whole reason a remembered size is
// not centred, and it is the state a person reaches by carrying a laptop from a
// large monitor to its own screen.
//
// A first start opens in the middle, because there is nothing to restore and
// the middle is where a window belongs when nobody has an opinion yet. It
// opens as tall as the first screen WANTS - the height at which the screen
// the window opens on shows its whole form without scrolling, worked out by
// Open from that screen as it is - and no taller than LargestOpening, which
// is the one fact about somebody else's screen this program knows. A want
// with a nought in it is no want, and the ceiling stands in for it.
func HowToOpen(remembered, wanted fyne.Size) (size fyne.Size, centre bool) {
	if WorthRemembering(remembered) {
		return remembered, false
	}
	size = LargestOpening
	if WorthRemembering(wanted) {
		size.Height = fyne.Min(wanted.Height, LargestOpening.Height)
	}
	return size, true
}

// unscrolled is a screen that can say how tall it has to be for its form to
// show whole.
type unscrolled interface{ Unscrolled() float32 }

// firstOpening is the size the window wants on a first start: the room the
// tab strip keeps above a screen, plus the screen the window opens on shown
// whole, at the width the window always opens at.
//
// The screen it opens on, and not the tallest of the three - the owner's
// decision of 2026-09-21. Until then the height came from the tallest work
// screen, the batch screen, so that no work screen scrolled from the first
// frame. What that bought was a window that opened on the single batch
// screen with a band of nothing between its form and the bar - 70 px in the
// forms of that day, the same band O202 was about under another number -
// because the screen a person actually sees first is the shortest. Now the
// window fits what it shows, and the batch screen scrolls a little when
// somebody goes there, which it does anyway from the second batch on.
//
// The About screen is left out for the same reason it always was. It carries
// the notices for every module in the build, which is a length that belongs
// to the dependencies and not to us, and a window sized for it would be sized
// for nothing else.
//
// The content is laid out at the opening width before anything is measured,
// because a sentence that wraps reports a height for the width it currently
// knows about - and before the first layout it knows none. tools/probes/
// guirender learnt this the hard way and resizes twice for the same reason.
//
// The window's own padding is added on top, because the size asked of a window
// is the size of its canvas and the canvas keeps the theme's padding round the
// content on every side - fyne v2.8.1 internal/driver/glfw/canvas.go line 223,
// padded by default and never switched off here. Measured before it was added:
// the batch screen's form needed 732 px and got 724 at the size worked out
// without it, eight pixels of padding short of showing whole.
func firstOpening(tabbed *fyne.Container, first unscrolled) fyne.Size {
	tabbed.Resize(LargestOpening)
	return fyne.NewSize(LargestOpening.Width, parts.AboveTheScreens(tabbed)+first.Unscrolled()+2*theme.Padding())
}

// unscrolledHeight is how tall a screen has to be for the form inside its
// scroll to show whole: the screen as it is, with the scroll's own claim on
// height swapped for the form's.
//
// Read off the screen's own objects rather than added up from a list of
// parts, for the reason tools/probes/formheight gives: subtracting the action
// bar and forgetting the tab strip said "fits" for a screen whose last field
// was cut off. Whatever stands above and below the scroll has taken its share
// in the screen's MinSize already.
func unscrolledHeight(body fyne.CanvasObject, scroll *container.Scroll) float32 {
	return body.MinSize().Height - scroll.MinSize().Height + scroll.Content.MinSize().Height
}
