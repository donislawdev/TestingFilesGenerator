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
