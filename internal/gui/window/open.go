package window

import (
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
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
	// Every screen goes back to the ordinary theme, because the strip they hang
	// under is drawn quieter and a theme reaches everything below it.
	tabs := container.NewAppTabs(
		container.NewTabItem(text.TabOneTarget(), parts.AtFullStrength(gen.Object())),
		container.NewTabItem(text.TabPresets(), parts.AtFullStrength(pre.Object())),
		container.NewTabItem(text.TabRecipe(), parts.AtFullStrength(rec.Object())),
		container.NewTabItem(text.TabAbout(), parts.AtFullStrength(About(h))),
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
		text.TabOneTarget(): gen,
		text.TabPresets():   pre,
		text.TabRecipe():    rec,
	}
	showing := text.TabOneTarget()

	offerWhereItLastWrote(h, working)

	// The keyboard, wired once for the window rather than per screen. Which
	// screen an action lands on is asked at the moment it is pressed, because a
	// shortcut belongs to the window and the answer is whichever screen is
	// being looked at.
	keyed := map[string]keyboardScreen{
		text.TabOneTarget(): gen,
		text.TabPresets():   pre,
		text.TabRecipe():    rec,
	}

	// The keyboard starts on the first field of the screen somebody is looking
	// at, and moves with them. Owner's decision of 2026-08-25.
	//
	// The mark that says "the keyboard is here" is NOT drawn by this, and that
	// is deliberate rather than a gap: this window draws that mark only for
	// somebody using the keyboard (O90), so a focus placed by the program is
	// silent until a key is pressed. Placing it quietly is what makes the first
	// Tab land on the second field rather than the first.
	focusFirst := func(name string) {
		screen, ok := keyed[name]
		if !ok {
			return
		}
		if first := screen.FirstField(); first != nil {
			parts.FocusQuietly(h.Canvas(), first)
		}
	}

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
		// The keyboard follows the person to the screen they moved to. Without
		// this it stays on a control of the screen they left, which is a Tab
		// that starts somewhere nobody can see.
		focusFirst(item.Text)
	}

	// Closing the window during a run is a cancellation and not a kill, G7. The
	// invariant that the output directory never holds a half written file rests
	// on the signal handler in cmd/tfg, and closing a window is not a signal -
	// so without this the run would carry on with nobody watching it, or die in
	// the middle of a file.
	closeCleanly(h, []interface{ Stop() }{gen, pre, rec}, working, &showing)
	offerSettling(h, []interface{ Settled() }{gen, pre, rec})

	// One table for the window, handed to the boxes of every screen. Wired here
	// rather than in each constructor because the table belongs to the window
	// and the screens are built before it exists.
	shortcuts := parts.NewShortcuts()
	for _, screen := range []interface{ Fields() *parts.Fields }{gen, pre, rec} {
		screen.Fields().PassShortcutsTo(shortcuts.Deliver)
	}
	wireKeyboard(h, keyed, &showing, shortcuts)

	// The window still opens on the work rather than on the notice, which is
	// the owner's decision of 2026-08-05 and is now a property of which tab is
	// first rather than of which screen is installed.
	// The strip reads with one point of focus: the tab somebody is on is in the
	// accent colour and the others are quiet. Until 2026-08-20 it was the other
	// way round by contrast - the chosen one was the dimmest label there.
	h.SetContent(parts.QuietUnlessChosen(tabs))
	// Last, once there is something on the canvas to focus.
	focusFirst(showing)
}

// closeCleanly stops whatever is running, writes down where the files were
// going, and then lets the window go.
//
// It asks every screen to stop rather than the one on show, and that is the part
// worth writing down: a run keeps going while somebody moves to another screen,
// so the busy one is not necessarily the visible one. Stop does nothing on a
// screen that is idle.
//
// The directory is taken from the screen somebody was last on rather than from a
// fixed one - the three carry it between themselves, so the screen being looked
// at is the one holding the answer. showing is a pointer for that reason: it
// changes as somebody moves, and what matters is where they were when they shut
// the window.
//
// One moment rather than "whenever it changes", so what comes back is
// describable in a sentence: the window opens where you left it. Writing on
// every keystroke would also put a half typed path on somebody's disk.
//
// Split out of Open on 2026-08-25, when that function went past three quarters
// of the ceiling. The ceiling is a ratchet, so the answer is a split and never a
// higher number.
func closeCleanly(h Host, running []interface{ Stop() }, working map[string]interface {
	OutDir() string
	SetOutDir(string)
}, showing *string) {
	h.SetCloseIntercept(func() {
		for _, screen := range running {
			screen.Stop()
		}
		if screen, ok := working[*showing]; ok {
			h.Remembered().RememberDirectory(screen.OutDir())
		}
		h.Close()
	})
}

// offerWhereItLastWrote puts the directory of the last run on every screen.
//
// Every screen rather than the one that opens first: the three carry the
// directory between themselves as somebody moves, so setting it on one and not
// the others would leave the old value on any screen reached without passing
// through that one.
//
// Not the same question as startingDirectory, which the three constructors
// call. That one answers what to OFFER when nobody has said anything, and this
// is somebody having said. Keeping them apart is what leaves a first start
// showing the measured folder - see OutputFolderName - rather than an empty box
// every screen would refuse.
//
// Split out of Open on 2026-08-25, when that function went past three quarters
// of the ceiling for the second time in a day. The ceiling is a ratchet, so the
// answer is a split and never a higher number.
func offerWhereItLastWrote(h Host, working map[string]interface {
	OutDir() string
	SetOutDir(string)
}) {
	last := h.Remembered().Directory()
	if last == "" {
		return
	}
	for _, screen := range working {
		screen.SetOutDir(last)
	}
}

// wireKeyboard puts this window's three shortcuts on the canvas.
//
// Split out of Open on 2026-08-25, when that function went past three quarters
// of the ceiling. The ceiling is a ratchet, so the answer is a split and never
// a higher number.
//
// showing is a pointer because the answer changes as somebody moves between
// screens, and a shortcut is about the screen being looked at WHEN IT IS
// PRESSED rather than when it was registered.
func wireKeyboard(h Host, keyed map[string]keyboardScreen, showing *string, table *parts.Shortcuts) {
	on := func(act func(keyboardScreen)) func(fyne.Shortcut) {
		return func(fyne.Shortcut) {
			if screen, ok := keyed[*showing]; ok {
				act(screen)
			}
		}
	}
	// Ctrl+Enter and Ctrl+P, which is what a form with two buttons at the foot
	// is expected to answer to. They press the BUTTON rather than call what it
	// does, so a run cannot be started twice and a preview cannot be started
	// during one - see pressIfLive.
	// Into our own table AND onto the canvas: the table is what a box delivers
	// into when the keyboard is in one, the canvas is what the toolkit reaches
	// when nothing has the keyboard at all. Same entries either way, so there is
	// one answer to what a key means.
	wire := func(shortcut fyne.Shortcut, act func(keyboardScreen)) {
		table.On(shortcut, func() { on(act)(shortcut) })
		h.Canvas().AddShortcut(shortcut, on(act))
	}
	wire(&desktop.CustomShortcut{KeyName: fyne.KeyReturn, Modifier: fyne.KeyModifierControl},
		keyboardScreen.PressGenerate)
	wire(&desktop.CustomShortcut{KeyName: fyne.KeyEnter, Modifier: fyne.KeyModifierControl},
		keyboardScreen.PressGenerate)
	wire(&desktop.CustomShortcut{KeyName: fyne.KeyP, Modifier: fyne.KeyModifierControl},
		keyboardScreen.PressPreview)
	// Escape stops a run, and does nothing when there is none - PressCancel
	// presses a button that is hidden and disabled until one starts.
	//
	// It arrives here by a different road from the two above and that is worth
	// knowing: the toolkit turns a key into a shortcut only when a modifier is
	// held, so Escape reaches a box as an ordinary key and parts.Entry sends it
	// on as this. One table either way - see parts.Entry.TypedKey.
	//
	// It does NOT collide with the open format list, which takes Escape while it
	// is open and never lets it out: the list has the keyboard then, so nothing
	// here is reached.
	wire(&desktop.CustomShortcut{KeyName: fyne.KeyEscape}, keyboardScreen.PressCancel)

}

// keyboardScreen is what a screen has to offer for the keyboard to reach it.
//
// An interface rather than a switch over the three screens, so that a fourth
// one is a compile error at the map above rather than a shortcut that quietly
// does nothing on it.
type keyboardScreen interface {
	PressGenerate()
	PressPreview()
	PressCancel()
	FirstField() fyne.Focusable
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
	return widget.NewButton(text.ButtonDonate(), func() { h.OpenLink(text.SupportURL) })
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
func chooserFor(host Host, box *parts.Entry) fyne.CanvasObject {
	choose := widget.NewButton(text.ButtonChoose(), func() {
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

// offerSettling hands a host a way to wait for work in flight, if it wants one.
//
// An optional interface, checked rather than required, so the Host a real
// window implements does not grow a method for something only the guards ask
// for. Nothing in the shipped program calls it.
//
// It exists because of what changed on 2026-08-26. Before then a preview could
// not be cancelled - preflight took no context - so Stop only waited, and the
// guards used the close intercept as their way of waiting for an answer. Now
// closing cancels, which is the point of the change, so a guard that closed the
// window to read a preview would be cancelling the preview it wanted to read.
func offerSettling(h Host, screens []interface{ Settled() }) {
	w, ok := h.(interface{ SetWaitForWork(func()) })
	if !ok {
		return
	}
	w.SetWaitForWork(func() {
		for _, screen := range screens {
			screen.Settled()
		}
	})
}
