package guard

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// The keyboard, and the one thing about it that is not obvious.
//
// A shortcut in this toolkit is delivered to the FOCUSED widget when that
// widget can take shortcuts, and only reaches the canvas when nothing is
// focused - measured in internal/driver/glfw window.go on 2026-08-25. A
// widget.Entry can take them and drops the ones it does not know without a
// word. These screens are forms, so the keyboard is nearly always in a box:
// a shortcut registered on the canvas and pressed the way a person presses it
// would arrive nowhere, and every test that called the handler directly would
// stay green.
//
// So these press through the BOX, which is the road a real keystroke takes.

// keyedWindow opens the window on a canvas a guard can press shortcuts on.
//
// The canvas is handed to the host BEFORE Open, because Open is what registers
// the shortcuts and they have to land on the canvas the press will come from. A
// guard that built its window afterwards would press an empty table.
func keyedWindow(t *testing.T) (*fakeHost, fyne.CanvasObject, fyne.Canvas) {
	t.Helper()
	w := test.NewWindow(nil)
	t.Cleanup(w.Close)

	host := &fakeHost{canvas: w.Canvas()}
	window.Open(host)
	if host.content == nil {
		t.Fatal("opening the window put no screen in it")
	}
	// NOT SetContent again: the host has already put the tree on this canvas,
	// and setting it a second time takes the keyboard off whatever Open put it
	// on - which is the very thing some of these guards ask about.
	w.Resize(window.OpenSize)
	t.Cleanup(func() { join(host) })
	return host, host.content, w.Canvas()
}

// pressInABox sends a shortcut the way one arrives when somebody has just been
// typing: to the box that has the keyboard.
func pressInABox(t *testing.T, box *parts.Entry, key fyne.KeyName, mod fyne.KeyModifier) {
	t.Helper()
	if box == nil {
		t.Fatal("no box to press in")
	}
	box.TypedShortcut(&desktop.CustomShortcut{KeyName: key, Modifier: mod})
}

// TestTheKeyboardStartsARunFromInsideABox is the guard that makes the shortcuts
// real rather than registered.
func TestTheKeyboardStartsARunFromInsideABox(t *testing.T) {
	dir := t.TempDir()
	_, content, _ := keyedWindow(t)
	screen := selectTab(t, content, text.TabOneTarget())

	entryUnder(t, screen, text.FieldOutputDir()).SetText(dir)
	entryUnder(t, screen, text.FieldSize()).SetText("1kb")
	entryUnder(t, screen, text.FieldTargetID()).SetText("keys")

	// The keyboard is in a box, which is where it is after somebody types a
	// size - and it is the case a canvas shortcut cannot reach.
	pressInABox(t, entryUnder(t, screen, text.FieldSize()), fyne.KeyReturn, fyne.KeyModifierControl)
	waitForManifest(t, dir)

	// And the run really happened, read off the disk rather than off the screen.
	if got := len(filesIn(t, dir)); got == 0 {
		t.Error("Ctrl+Enter was pressed with the keyboard in a box and nothing was written, " +
			"so the shortcut is registered somewhere the keystroke never reaches")
	}
}

// TestThePreviewShortcutAsksWithoutWriting is the other half: the same road,
// and nothing on the disk at the end of it.
func TestThePreviewShortcutAsksWithoutWriting(t *testing.T) {
	dir := t.TempDir()
	host, content, _ := keyedWindow(t)
	screen := selectTab(t, content, text.TabOneTarget())

	entryUnder(t, screen, text.FieldOutputDir()).SetText(dir)
	entryUnder(t, screen, text.FieldSize()).SetText("1kb")
	entryUnder(t, screen, text.FieldTargetID()).SetText("keys")

	before := shownText(content)
	pressInABox(t, entryUnder(t, screen, text.FieldSize()), fyne.KeyP, fyne.KeyModifierControl)
	join(host)

	if got := len(filesIn(t, dir)); got != 0 {
		t.Errorf("Ctrl+P wrote %d file(s), and a preview writes none", got)
	}
	// It did something, and this is asked of what the PREVIEW says rather than
	// of the screen holding any text at all. The first version of this guard
	// compared shownText with "" - which every screen fails, because the labels
	// are text - so it passed against a shortcut wired to nothing. Caught by the
	// sibling guard failing while this one stayed green.
	if shownText(content) == before {
		t.Error("Ctrl+P was pressed and nothing on the screen changed, so the shortcut reached nothing")
	}
}

// TestEscapeIsOnlyLiveWhileThereIsSomethingToStop.
//
// Escape takes a different road from the other two and that is measured rather
// than assumed: the toolkit turns a key into a shortcut only when a modifier is
// held, so Escape alone arrives at a box as an ordinary key. parts.Entry sends
// it on as a shortcut so the window has one table rather than two.
//
// What this asks is that it does NOTHING when there is no run - because Cancel
// is hidden and disabled then, and a shortcut that worked on a button nobody
// can see would reach a state the screen does not offer.
func TestEscapeIsOnlyLiveWhileThereIsSomethingToStop(t *testing.T) {
	dir := t.TempDir()
	host, content, _ := keyedWindow(t)
	screen := selectTab(t, content, text.TabOneTarget())

	entryUnder(t, screen, text.FieldOutputDir()).SetText(dir)
	entryUnder(t, screen, text.FieldSize()).SetText("1kb")
	entryUnder(t, screen, text.FieldTargetID()).SetText("keys")

	size := entryUnder(t, screen, text.FieldSize())
	before := shownText(content)
	// Nothing is running. Escape has to be harmless.
	size.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})
	join(host)
	if shownText(content) != before {
		t.Error("Escape changed the screen while nothing was running, so it reaches a button " +
			"that is meant to be out of use")
	}
	if got := len(filesIn(t, dir)); got != 0 {
		t.Errorf("Escape with nothing running wrote %d file(s)", got)
	}
}

// TestTheKeyboardStartsOnTheFirstFieldOfTheScreen records the owner's decision
// of 2026-08-25.
//
// The mark that says the keyboard is here is deliberately NOT drawn by it: this
// window draws that mark only for somebody using the keyboard (O90), and nobody
// has pressed a key when a window opens. What is asked here is where the
// keyboard IS, not what is painted.
func TestTheKeyboardStartsOnTheFirstFieldOfTheScreen(t *testing.T) {
	_, content, canvas := keyedWindow(t)

	for _, want := range []struct {
		tab   string
		label string
	}{
		{text.TabOneTarget(), text.FieldFormat()},
		{text.TabPresets(), text.FieldPreset()},
		{text.TabRecipe(), text.FieldFormat()},
	} {
		t.Run(want.tab, func(t *testing.T) {
			screen := selectTab(t, content, want.tab)
			focused := canvas.Focused()
			if focused == nil {
				t.Fatalf("nothing has the keyboard on %s, so the first Tab starts nowhere", want.tab)
			}
			control := controlUnder(screen, want.label)
			if control == nil {
				t.Fatalf("this screen has no field called %q", want.label)
			}
			if any, ok := focused.(fyne.CanvasObject); !ok || any != control {
				t.Errorf("the keyboard is on %s and %q is the first field of %s",
					describeFocusable(focused), want.label, want.tab)
			}
		})
	}
}

// TestAFinishedRunOffersTheFolderItWroteInto is point 20 of the rebuild, and
// O85 closed.
//
// The button is on the bar only while there is something to open. A button
// leading to a directory that does not exist yet is a button that does nothing,
// and this window has spent a month getting rid of those.
func TestAFinishedRunOffersTheFolderItWroteInto(t *testing.T) {
	dir := t.TempDir()
	host, content, _ := keyedWindow(t)
	screen := selectTab(t, content, text.TabOneTarget())

	// Nothing has run, so there is nothing to open. Asked about what is SHOWN
	// rather than what is in the tree: the button is built with the bar and
	// hidden, so a guard that only looked for it would find it every time.
	if shownButton(screen, text.ButtonOpenFolder()) != nil {
		t.Fatal("the folder button is on the screen before anything was written, " +
			"so it points at a directory that need not exist")
	}

	entryUnder(t, screen, text.FieldOutputDir()).SetText(dir)
	entryUnder(t, screen, text.FieldSize()).SetText("1kb")
	entryUnder(t, screen, text.FieldTargetID()).SetText("done")
	press(t, screen, text.ButtonGenerate())
	waitForManifest(t, dir)
	join(host)

	button := shownButton(screen, text.ButtonOpenFolder())
	if button == nil {
		t.Fatal("the run wrote files and there is no way to open the folder they went into")
	}
	button.OnTapped()
	if host.folderCount != 1 {
		t.Errorf("the folder button was pressed and the desktop was asked %d times", host.folderCount)
	}
	// The directory the run actually used, not whatever the box says now.
	if host.folder != dir {
		t.Errorf("the button opens %q and the files went to %q", host.folder, dir)
	}
}

// TestTheFolderOfferGoesAwayWhenTheNextRunStarts.
//
// The offer belongs to one run. Left standing it would point at the results of
// the run before this one, which is worse than not being there - somebody would
// open it, see the old files and believe them.
func TestTheFolderOfferGoesAwayWhenTheNextRunStarts(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	host, content, _ := keyedWindow(t)
	screen := selectTab(t, content, text.TabOneTarget())

	entryUnder(t, screen, text.FieldSize()).SetText("1kb")
	entryUnder(t, screen, text.FieldTargetID()).SetText("one")
	entryUnder(t, screen, text.FieldOutputDir()).SetText(first)
	press(t, screen, text.ButtonGenerate())
	waitForManifest(t, first)
	join(host)

	if shownButton(screen, text.ButtonOpenFolder()) == nil {
		t.Fatal("the first run wrote files and offered no folder, so this guard cannot ask its question")
	}

	entryUnder(t, screen, text.FieldOutputDir()).SetText(second)
	press(t, screen, text.ButtonGenerate())
	waitForManifest(t, second)
	join(host)

	button := shownButton(screen, text.ButtonOpenFolder())
	if button == nil {
		t.Fatal("the second run wrote files and offered no folder")
	}
	button.OnTapped()
	if host.folder != second {
		t.Errorf("after a second run the button opens %q, which is where the FIRST run wrote", host.folder)
	}
}

// shownButton is a button somebody can actually see, which is not the same as
// one that is in the tree - this window builds several and hides them until
// they mean something.
func shownButton(o fyne.CanvasObject, name string) *widget.Button {
	button := buttonNamed(o, name)
	if button == nil || !button.Visible() {
		return nil
	}
	return button
}

// TestAShortcutCannotStartASecondRunDuringTheFirst is what the check in
// pressIfLive is really for.
//
// Ctrl+Enter presses the BUTTON, and the button is disabled while a run is
// going - so the shortcut cannot reach a state the screen does not offer. This
// is asked with a run in flight rather than with none, and the difference
// matters: with nothing running, Cancel does nothing when pressed anyway, so a
// guard built round Escape stays green however the check is broken. Found by
// the mutation runner on 2026-08-25, which is the second time this project has
// caught itself defending something that could not be broken.
func TestAShortcutCannotStartASecondRunDuringTheFirst(t *testing.T) {
	dir := t.TempDir()
	host, content, _ := keyedWindow(t)
	screen := selectTab(t, content, text.TabRecipe())

	// Enough files that the run is still going when the second press lands.
	entryUnder(t, screen, text.FieldOutputDir()).SetText(dir)
	entryUnder(t, screen, text.FieldTargetID()).SetText("many")
	entryUnder(t, screen, text.FieldCount()).SetText("400")
	chooseSizeWay(t, screen, text.SizeWayExact())
	entryUnder(t, screen, text.FieldSize()).SetText("4kb")

	press(t, screen, text.ButtonGenerate())
	// While it is going, the button is off. A shortcut has to respect that.
	pressInABox(t, entryUnder(t, screen, text.FieldSize()), fyne.KeyReturn, fyne.KeyModifierControl)
	join(host)

	// A second run into the same directory is refused by the engine, so the
	// sign of one having started is a refusal on the screen where a finished
	// run should be reported.
	if said := shownText(content); strings.Contains(said, text.RefusedBeforeWriting()) {
		t.Error("Ctrl+Enter started a second run while the first was going, so the shortcut " +
			"presses a button the screen has taken out of use")
	}
}
