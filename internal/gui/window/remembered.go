package window

import "fyne.io/fyne/v2"

// Remembered is the little this window keeps between runs.
//
// Two things it notices, by the owner's decision of 2026-08-23: where the
// files go, and how big the window is. Deliberately NOT the format, the tab or
// the preset - a setting from last week is worse than a start you can predict,
// because the person who opens this tool tomorrow is answering a different
// question than the one they answered today.
//
// And since 2026-09-29 one thing it is told: the language chosen on the
// Preferences screen. That is not last week's state coming back uninvited, it
// is a choice somebody made in order to keep it, and it is the one value here
// that nothing writes but a person.
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
// our values to a file that was already there. Nothing is ever deleted from it
// by us except on the press of Forget, and then only our values - rule 7 - and
// nothing leaves the machine, which is D16.
type Remembered interface {
	// Directory is where the files went last time, or empty when nobody has
	// said yet.
	Directory() string
	RememberDirectory(string)

	// Size is how big the window was when it was last closed, or a size with a
	// nought in it when nobody has said yet.
	Size() fyne.Size
	RememberSize(fyne.Size)

	// Language is the language somebody chose on the Preferences screen, as a
	// tag, or empty when nobody has - which means the language of the system
	// (docs/PRODUCT.md D9, since 2026-09-29). RememberLanguage with an empty
	// tag goes back to that by removing the value rather than writing an
	// empty one.
	Language() string
	RememberLanguage(tag string)

	// Forget removes everything above, on the press of the Forget button and
	// at no other time - untouchable rule 7. For the rest of this run the
	// folder and the size are not written down again either, or closing the
	// window would put back what the person just asked to be forgotten. A
	// language chosen after it is written: that is a new choice, not the
	// old memory.
	Forget()
}

// Forgetting is a store that keeps its word after Forget: for the rest of this
// run, the folder and the size are not written down again, or closing the
// window would put back what the person just asked to be forgotten. The store
// under it only removes and writes values - this is the promise, in the one
// place a guard can reach it, and the real window and the guards' stand in both
// go through it. A language chosen after Forget is written: that is a new
// choice, not the old memory.
func Forgetting(store Remembered) Remembered { return &forgetting{Remembered: store} }

type forgetting struct {
	Remembered
	forgot bool
}

func (f *forgetting) RememberDirectory(d string) {
	if !f.forgot {
		f.Remembered.RememberDirectory(d)
	}
}

func (f *forgetting) RememberSize(size fyne.Size) {
	if !f.forgot {
		f.Remembered.RememberSize(size)
	}
}

func (f *forgetting) Forget() {
	f.forgot = true
	f.Remembered.Forget()
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
