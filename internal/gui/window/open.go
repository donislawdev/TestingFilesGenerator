package window

import (
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// Open fills the window with the first screen and wires the way between them.
//
// The window opens on the work rather than on a welcome, decided by the owner
// on 2026-08-05. The licence notice is a screen you go to and not one you are
// shown: docs/GUI.md section 7 asks that somebody using only the window be able
// to read what the licence means for the files they generate, which is a
// question about whether it can be reached rather than about greeting people
// with it at every start.
//
// Every screen is built once and kept. Rebuilding one on the way back would
// lose whatever was typed, and would lose a run in progress along with the only
// handle on stopping it.
func Open(h Host) {
	gen := NewGenerate(h)
	pre := NewPreset(h)
	rec := NewRecipe(h)

	// Tabs across the top rather than buttons at the foot, reported from use on
	// 2026-08-11. The way between the screens used to sit in the row of actions
	// under the last field, so changing screen meant scrolling past the whole
	// form to find it - and nobody scrolls to the bottom to navigate. Moving
	// between screens is not an action taken on the form, and it was reading
	// like one.
	//
	// About is a tab as well although only the two work screens were asked
	// about. Leaving it at the foot would have kept exactly the defect being
	// fixed, for the one screen somebody reaches least often and would look
	// hardest for. It also loses its Back button: a tab is its own way out.
	tabs := container.NewAppTabs(
		container.NewTabItem(text.TabOneTarget, gen.Object()),
		container.NewTabItem(text.TabPresets, pre.Object()),
		container.NewTabItem(text.TabRecipe, rec.Object()),
		container.NewTabItem(text.TabAbout, About(h)),
	)

	// The output directory follows whoever is looking, and that is a fix for a
	// real confusion rather than a convenience. Each screen used to hold its
	// own box, both starting at the working directory - so they agreed until
	// somebody changed one, and then silently disagreed. Reported from use on
	// 2026-08-11: files were written to a chosen directory, the preset screen
	// was opened, and its run went somewhere else entirely.
	//
	// Carried at the moment of moving rather than bound both ways, because
	// this is the only moment the difference can become visible - and one
	// direction of copying cannot loop.
	// Which screen was being looked at, so the directory can be carried from it
	// to the next one. Two screens could be written as a switch naming the other
	// one, and a third makes "the other one" ambiguous - the answer is where
	// somebody has just been, which is a thing to remember rather than derive.
	working := map[string]interface {
		OutDir() string
		SetOutDir(string)
	}{
		text.TabOneTarget: gen,
		text.TabPresets:   pre,
		text.TabRecipe:    rec,
	}
	showing := text.TabOneTarget

	tabs.OnSelected = func(item *container.TabItem) {
		from, leaving := working[showing]
		to, arriving := working[item.Text]
		if leaving && arriving {
			to.SetOutDir(from.OutDir())
		}
		// Remembered even when the new screen has no directory of its own, so
		// that About in the middle of two work screens does not strand the
		// value on the screen before it.
		if arriving {
			showing = item.Text
		}
	}

	// Closing the window during a run is a cancellation and not a kill, G7. The
	// invariant that the output directory never holds a half written file rests
	// on the signal handler in cmd/tfg, and closing a window is not a signal -
	// so without this the run would carry on with nobody watching it, or die in
	// the middle of a file.
	//
	// It asks every screen rather than the one on show, and that is the part
	// worth writing down: a run keeps going while somebody moves to another
	// screen, so the busy one is not necessarily the visible one. Stop does
	// nothing on a screen that is idle.
	h.SetCloseIntercept(func() {
		gen.Stop()
		pre.Stop()
		rec.Stop()
		h.Close()
	})

	// The window still opens on the work rather than on the notice, which is
	// the owner's decision of 2026-08-05 and is now a property of which tab is
	// first rather than of which screen is installed.
	h.SetContent(tabs)
}

// FirstScreen is what the window shows when it opens, without a window to put
// it in. It is what a guard renders, and it is the same tree Open installs.
func FirstScreen(h Host) fyne.CanvasObject {
	return NewGenerate(h).Object()
}

// donateButton asks for money towards the work, at the left of the bar a screen
// keeps its actions in.
//
// Moved there from a strip above the tabs on 2026-08-19, on the owner's
// decision. It is built per screen rather than once for the window, and that is
// what the move costs: every screen has to place it, including About, which has
// no run to start and gets a bar holding nothing else. The alternative was a
// second strip under the tabs, which is two bars at the foot of one window.
//
// It stays on every screen either way, which is not decoration - a button asking
// for money that appears on some screens and not others is a button people
// conclude they imagined.
//
// It opens the support page in whatever the desktop uses for the web. The
// program fetches nothing and sends nothing, which is what keeps untouchable
// rule 8 intact - see the carve out written into it on 2026-08-18.
func donateButton(h Host) fyne.CanvasObject {
	return widget.NewButton(text.ButtonDonate, func() { h.OpenLink(text.SupportURL) })
}

// chooserFor is the output directory box with a way to browse to one.
//
// Typing a path is fine when you have one to hand and hopeless when you do not,
// and the box beside it is the only field on either screen whose value is not
// something a person carries in their head. The button asks the window, which
// is the only thing that can open a picker.
//
// The box stays editable. A picker that replaces typing takes away pasting a
// path somebody sent you, which is how most of these get filled in.
func chooserFor(host Host, box *widget.Entry) fyne.CanvasObject {
	choose := widget.NewButton(text.ButtonChoose, func() {
		host.ChooseDirectory(func(dir string) {
			if dir != "" {
				box.SetText(dir)
			}
		})
	})
	return container.NewBorder(nil, nil, nil, choose, box)
}

// startingDirectory is where the output field points before anybody changes it.
//
// Spelled out in full rather than left as a dot, and that is the one place a
// window has to differ from the command line rather than merely look different.
// In a terminal "." is the directory you are standing in and you know which one
// that is, because you typed your way there. A window started from a desktop
// has a working directory nobody chose and nobody can see - so the same dot
// means "somewhere", and this is the one part of this tool that writes into
// other people's directories.
//
// The destination itself is unchanged. What changes is that it is legible
// before the button is pressed rather than after the files have appeared.
//
// A working directory we cannot read leaves the folder name on its own, which
// lands in the same place by a shorter route, because a relative name that
// means "here" is still better than a path that is wrong.
func startingDirectory() string {
	dir, err := os.Getwd()
	if err != nil {
		return OutputFolderName
	}
	return filepath.Join(dir, OutputFolderName)
}

// OutputFolderName is the folder the window offers to write into, under
// whatever directory the program was started from.
//
// A folder of our own rather than the working directory itself, and the reason
// is what a double click does. Started from a desktop, the working directory is
// the folder the program was unpacked into - so the offered destination was
// somebody's Downloads, and a set of ten thousand files went straight into it,
// mixed in with everything already there. Deleting them again means picking
// them out by hand, because nothing marks which ones arrived this way.
//
// One named folder makes that a single thing to delete, and it is visible in
// the field before anybody presses anything. Nothing is written until a run
// starts, and the engine makes the folder then - measured on 2026-08-19, a run
// and a preview into a directory that does not exist both succeed - so this
// costs an empty folder nobody asked for exactly never.
//
// The command line still defaults to the directory you are standing in, and
// that difference is deliberate for the same reason the full path above is:
// in a terminal you typed your way to that directory and you know which one it
// is (O103).
const OutputFolderName = "tfg-out"
