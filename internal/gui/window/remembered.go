package window

import "fyne.io/fyne/v2"

// Remembered is the little this window keeps between runs.
//
// Two things and no more, by the owner's decision of 2026-08-23: where the
// files go, and how big the window is. Deliberately NOT the format, the tab or
// the preset - a setting from last week is worse than a start you can predict,
// because the person who opens this tool tomorrow is answering a different
// question than the one they answered today.
//
// It is an interface on the Host rather than a call to the toolkit's global
// preferences, for the reason every other seam here exists: the screens build
// and run with no window, no canvas and no C compiler, and a guard has to be
// able to say what was remembered without writing to anybody's disk. The
// toolkit's test driver does hand out in memory preferences, so the global
// would have been testable - but it would also have been reachable from
// anywhere, and this is the one part of the program that writes outside the
// output directory.
//
// What it costs, and it is worth saying plainly because docs/PRODUCT.md D16 and
// untouchable rule 7 are both nearby: this puts a file on somebody's disk.
// It is not a new one. Measured on 2026-08-25: the toolkit's folder picker has
// been writing dev.donislaw.tfg/preferences.json since the Choose button
// arrived on 2026-08-05, with its own last folder and view layout in it, and
// the note beside appID said no file was written into that directory. This adds
// two keys to a file that was already there. Nothing is ever deleted from it by
// us - rule 7 - and nothing leaves the machine, which is D16.
type Remembered interface {
	// Directory is where the files went last time, or empty when nobody has
	// said yet.
	Directory() string
	RememberDirectory(string)

	// Size is how big the window was when it was last closed, or a size with a
	// nought in it when nobody has said yet.
	Size() fyne.Size
	RememberSize(fyne.Size)
}

// WorthRemembering says whether a size is a real one.
//
// A window being closed while minimised reports nothing useful, and a nought
// written down now is a nought read back at the next start. Exported so that
// the one predicate is used at BOTH ends - the real window asks it before
// writing, HowToOpen asks it before restoring - because two copies of "is this
// a size" is how a window comes back as nothing on one machine and not another.
func WorthRemembering(size fyne.Size) bool {
	return size.Width > 0 && size.Height > 0
}

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
// Fyne cannot help here: there is no way to ask how big the screen is, checked
// in the driver interface on 2026-08-19 and again in v2.8.0 on 2026-08-25. So
// the size cannot be checked against the screen, and what is done instead is to
// leave the window where the system puts it.
//
// A first start still opens in the middle at the measured OpenSize, because
// there is nothing to restore and the middle is where a window belongs when
// nobody has an opinion yet.
func HowToOpen(remembered fyne.Size) (size fyne.Size, centre bool) {
	if !WorthRemembering(remembered) {
		return OpenSize, true
	}
	return remembered, false
}
