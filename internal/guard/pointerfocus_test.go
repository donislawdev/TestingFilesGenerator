package guard

import (
	"fmt"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// The mark that says "the keyboard is here" is drawn for the keyboard.
//
// Reported from the screen twice, which is what makes it a guard rather than a
// tidy-up. On 2026-08-12 a format menu stayed painted blue after a value was
// chosen with the mouse, and on 2026-08-18 the same thing again plus the disc
// behind the label switch, called ugly both times it appeared - on checking and
// on unchecking.
//
// The cause is one thing and it is in the toolkit. Both controls take the
// keyboard when they are pressed, widget/select.go tapped and widget/check.go
// Tapped by way of focusIfNotMobile, and both draw that as a fill in the focus
// colour that nothing takes off again. So the answer is not a third shape for
// the disc - it is that a press moves the keyboard silently and only the
// keyboard says so. Every desktop platform does this and the web calls it
// focus-visible.
//
// Asked of both kinds of control in one test on purpose. The first fix went in
// for menus alone on 2026-08-12 and the switch came back six days later as a
// separate report, which is docs/UX.md section 7.0 gate 2 in one sentence:
// naming the class is what stops occurrence N plus one.
func TestAPressMovesTheKeyboardWithoutDrawingItsMark(t *testing.T) {
	c, content := screenOnACanvas(t)

	menu := chooserUnder(t, content, text.FieldFormat())
	menu.Tapped(&fyne.PointEvent{})
	if menu.Marked() {
		t.Error("the format menu was pressed with the pointer and drew the keyboard mark, " +
			"so the value chosen sits on a blue box until something else is clicked")
	}
	// And the keyboard IS there. The name of this guard promised that from
	// the start and until 2026-09-16 only the second half was asked, so a
	// control that took the keyboard nowhere at all passed it - which is what
	// the switch below did: the driver unfocuses whatever had the keyboard on
	// a press and leaves focusing to the widget, and the switch left it.
	// On the list the press opened, which is where the arrows have to work.
	// Closing it hands the keyboard back to the menu (see Chooser.giveBack).
	//
	// "On the list" is where the list says the keyboard goes, since the list
	// of 2026-09-23 took a filter: the box at its top has the keyboard and
	// hands the arrows on (FilterBox.TypedKey). Asked of OpenList.Keyboard
	// rather than of a type, and then the arrow is pressed on whatever has the
	// keyboard - so the guard holds the promise, not the shape it took.
	list := menu.Opened()
	if list == nil {
		t.Fatal("the format menu was pressed and opened no list")
	}
	if c.Focused() != list.Keyboard() {
		t.Errorf("the format menu was pressed and the keyboard is on %T, not where the list it opened takes it (%T)",
			c.Focused(), list.Keyboard())
	}
	if focused := c.Focused(); focused != nil {
		before := list.Active()
		focused.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})
		if list.Active() == before {
			t.Errorf("Down pressed on %T moved nothing in the list the press opened - the keyboard is somewhere the arrows do not work", focused)
		}
	}

	// The list the press opened is taken away first. A real press respects what
	// covers it - which is the point of pressing for real - so leaving an open
	// menu over the form would aim the next press at the menu and report a
	// switch that cannot be pressed.
	if top := c.Overlays().Top(); top != nil {
		c.Overlays().Remove(top)
	}

	// A real press through the canvas rather than a call, because widget.Check
	// changes its own value inside Tapped - so calling anything else would
	// reach a state a person cannot reach. It is also the only way to find out
	// that the press landed at all.
	toggle := checkNamed(content, text.FieldLabel())
	if toggle == nil {
		t.Fatalf("there is no switch labelled %q, so this guard read the wrong tree", text.FieldLabel())
	}
	before := toggle.Checked
	at := fyne.CurrentApp().Driver().AbsolutePositionForObject(toggle)
	active := toggle.MinSize()
	test.TapCanvas(c, at.Add(fyne.NewPos(active.Width/2, toggle.Size().Height/2)))
	if toggle.Checked == before {
		t.Fatalf("the press on the switch did not change it, so this guard never reached the state it is about")
	}
	if toggle.Marked() {
		t.Error("the switch was pressed with the pointer and drew the keyboard mark, " +
			"which is the blue disc behind the square that was reported on 2026-08-18")
	}
	if c.Focused() != toggle {
		t.Errorf("the switch was pressed and the keyboard is on %T, not on the switch - so the next "+
			"Space flips nothing and the next Tab starts from the top of the screen", c.Focused())
	}
}

// A press on a segmented switch puts the keyboard on it quietly, and the
// first arrow says so.
//
// The same rule as the two above, on the third control that can hold the
// keyboard. Asked on a bare canvas rather than on a screen, because a press
// aimed at one segment needs a position, and the segment widths are the
// control's own to know.
func TestAPressOnASegmentedSwitchPutsTheKeyboardOnItQuietly(t *testing.T) {
	app := test.NewApp()
	app.Settings().SetTheme(parts.Theme())
	t.Cleanup(func() { test.NewApp() })

	s := parts.NewSegments([]string{"one", "two", "three"}, nil)
	w := test.NewWindow(s)
	t.Cleanup(w.Close)
	w.Resize(s.MinSize())

	test.TapAt(s, fyne.NewPos(s.Size().Width-1, s.Size().Height/2))
	if s.Selected != "three" {
		t.Fatalf("the press on the last segment chose %q, so this guard never reached the state it is about", s.Selected)
	}
	if w.Canvas().Focused() != s {
		t.Errorf("the switch was pressed and the keyboard is on %T, not on the switch", w.Canvas().Focused())
	}
	if s.Marked() {
		t.Error("the switch was pressed with the pointer and drew the keyboard mark")
	}
	s.TypedKey(&fyne.KeyEvent{Name: fyne.KeyLeft})
	if !s.Marked() {
		t.Error("an arrow was pressed on the switch and the mark that says the keyboard is here did not come on")
	}
	if s.Selected != "two" {
		t.Errorf("the arrow after the press moved to %q, not on from the segment that was pressed", s.Selected)
	}
}

// The other half, and it is the half that makes the first one safe.
//
// Without this a window that never draws the mark at all passes, and that
// window breaks UX9: everything the mouse can do the keyboard can, and the
// place the keyboard IS has to be visible or the rest of that rule buys nothing.
func TestTheKeyboardDrawsItsMarkWhenTheKeyboardIsWhatArrived(t *testing.T) {
	c, content := screenOnACanvas(t)

	menu := chooserUnder(t, content, text.FieldFormat())
	c.Focus(menu)
	if !menu.Marked() {
		t.Error("the keyboard was moved into the format menu and nothing on it says so")
	}

	toggle := checkNamed(content, text.FieldLabel())
	if toggle == nil {
		t.Fatalf("there is no switch labelled %q, so this guard read the wrong tree", text.FieldLabel())
	}
	c.Focus(toggle)
	if !toggle.Marked() {
		t.Error("the keyboard was moved into the label switch and nothing on it says so")
	}
}

// Reaching for the keyboard after pressing with the mouse turns the mark on.
//
// The state in between, and the one a rule written as "pointer means never"
// would get wrong: somebody opens a list with the mouse and then drives it with
// the arrows, and from that moment they need to see which control is listening.
func TestReachingForTheKeyboardTurnsTheMarkOn(t *testing.T) {
	c, content := screenOnACanvas(t)

	menu := chooserUnder(t, content, text.FieldFormat())
	menu.Tapped(&fyne.PointEvent{})
	if menu.Marked() {
		t.Fatal("the press already drew the mark, so this guard cannot tell what the key did")
	}
	c.Focus(menu)
	menu.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})
	if !menu.Marked() {
		t.Error("a key was pressed on the format menu and it still does not say the keyboard is in it")
	}
}

// Every control on either screen that can hold the keyboard is one of ours.
//
// This is the guard that closes the class rather than the case. Counted from
// the TREE rather than from a list of the controls somebody remembered to
// convert - the same argument as the field registry and as
// TestEveryFormatIsClassifiedAsTextOrBinary: walking the entries already
// written down cannot find what is missing from them.
//
// It found one on the day it was written. The label switch had been converted
// and the switch a format or a preset DECLARES had not, so the first format to
// declare a bool setting would have shipped with the defect still in it and
// nothing would have said so - there is no such format today, which is exactly
// why nobody would have noticed.
func TestEverySwitchAndMenuOnScreenKnowsWhoFocusedIt(t *testing.T) {
	host := newFakeHost(t)
	window.Open(host)
	if host.content == nil {
		t.Fatal("opening the window put no screen in it")
	}

	seen := 0
	var raw []string
	walk(host.content, func(obj fyne.CanvasObject) {
		switch control := obj.(type) {
		case *parts.Toggle, *parts.Chooser:
			seen++
		case *widget.Check:
			raw = append(raw, "the switch")
		case *widget.Select:
			raw = append(raw, fmt.Sprintf("the menu showing %q", control.Selected))
		}
	})

	if seen == 0 {
		t.Fatal("no switch and no menu was found, so this guard would pass without checking anything")
	}
	for _, one := range raw {
		t.Errorf("%s is the toolkit's own control, so a press on it draws the keyboard mark.\n"+
			"Reason: widget.Select and widget.Check both fill themselves in the focus colour when they are\n"+
			"pressed and neither takes it off again - reported from the screen on 2026-08-12 and 2026-08-18.\n"+
			"What to do: build it with parts.NewToggle or parts.NewChooser, which draw that mark for the\n"+
			"keyboard only.", one)
	}
	t.Logf("%d control(s) that can hold the keyboard, all of them ours", seen)

	// And the kinds nothing has declared yet, asked of the builder rather than
	// of the screen. The walk above cannot see a control no format asks for -
	// there is no bool setting in the registry today, so reverting the switch a
	// declaration produces left every guard green. Measured on 2026-08-18 by
	// the mutation runner, which is the only thing that would have said so.
	for _, declared := range []format.Property{
		{Name: "a switch", Kind: format.PropertyBool, Default: "true"},
		{Name: "a list", Kind: format.PropertyChoice, Choices: []string{"one", "two"}, Default: "one"},
	} {
		built := parts.FromProperty(declared).Control
		switch built.(type) {
		case *parts.Toggle, *parts.Chooser:
		default:
			t.Errorf("a declared %s is drawn as %T, which is the toolkit's own control.\n"+
				"Reason: it fills itself in the focus colour when it is pressed and never takes it off.\n"+
				"What to do: build it with parts.NewToggle or parts.NewChooser.", declared.Kind, built)
		}
	}
}

// screenOnACanvas is the generate screen in a window, laid out.
//
// A canvas rather than the bare tree, because everything this file asks about
// needs one: focus belongs to a canvas, and a press has to land somewhere.
func screenOnACanvas(t *testing.T) (fyne.Canvas, fyne.CanvasObject) {
	t.Helper()
	return screenOnACanvasOfHeight(t, referenceHeight)
}

// screenOnACanvasOfHeight is the same screen in a window this tall, for a
// guard asking about something that follows the window - an open list is as
// tall as a share of it, so one height proves nothing about the rule.
func screenOnACanvasOfHeight(t *testing.T, height float32) (fyne.Canvas, fyne.CanvasObject) {
	t.Helper()
	// A fresh application and our own theme, the same two lines renderScene
	// opens with. Without them the sizes are the test driver's rather than the
	// window's, and a press aimed with one set of numbers at a screen laid out
	// with the other lands beside the control.
	app := test.NewApp()
	app.Settings().SetTheme(parts.Theme())
	t.Cleanup(func() { test.NewApp() })

	host := newFakeHost(t)
	window.Open(host)
	if host.content == nil {
		t.Fatal("opening the window put no screen in it")
	}
	content := selectTab(t, host.content, text.TabOneTarget())

	w := test.NewWindow(host.content)
	t.Cleanup(w.Close)
	// Laid out twice, for the reason renderScene gives: a wrapping label
	// reports its height for the width it knows about, and on the first pass
	// that is not the width it ends up with. A press aimed at a control that
	// has not settled lands somewhere else, which reads as a control that
	// cannot be pressed.
	size := fyne.NewSize(referenceWidth, height)
	w.Resize(size)
	w.Resize(size)
	host.content.Refresh()
	return w.Canvas(), content
}

// chooserUnder is the menu under a labelled field, on whichever screen.
func chooserUnder(t *testing.T, o fyne.CanvasObject, label string) *parts.Chooser {
	t.Helper()
	chooser, ok := controlUnder(o, label).(*parts.Chooser)
	if !ok {
		t.Fatalf("there is no menu under %q, so this guard read the wrong tree", label)
	}
	return chooser
}

// The window coming back to the front draws no mark nobody asked for, and
// draws again the one that was there.
//
// Reported by the owner on 2026-09-21 as one menu wearing a different colour
// from the others: on the single batch screen the format menu opened blue
// with a ring round it, and no other menu anywhere did. It was the keyboard
// mark, drawn by a call the control could not tell from the keyboard
// arriving - when the system gives the window the front, the driver calls
// FocusGained on whatever holds the keyboard (internal/driver/glfw/window.go
// processFocused, internal/app/focus_manager.go FocusGained), on the very
// first activation too, right after Open has put the keyboard on the first
// field quietly. So every start marked that one field, and every Alt-Tab
// back marked whichever control held the keyboard.
//
// The window says WindowReturning just before that call, from the toolkit's
// foreground hook - see internal/gui/run_cgo.go - and this guard does what
// the hook and the driver do, in that order, because the hook itself is
// behind cgo where no guard reaches it. Three arrivals, on the same control:
// the first activation after a quiet focus, a return after the keyboard had
// been used, and a plain FocusGained with no window in it, which is the
// keyboard moving here and has to draw as it always did.
func TestTheWindowComingBackDrawsOnlyTheMarkThatWasThere(t *testing.T) {
	c, content := screenOnACanvas(t)
	menu := chooserUnder(t, content, text.FieldFormat())

	// The first activation: the window put the keyboard here quietly, and
	// then the system handed it the front.
	parts.FocusQuietly(c, menu)
	if menu.Marked() {
		t.Fatal("a quiet focus drew the mark, so this guard is not starting from the state it is about")
	}
	menu.FocusLost()
	menu.WindowReturning()
	menu.FocusGained()
	if menu.Marked() {
		t.Error("the window came to the front and the first menu drew the keyboard mark although nobody had pressed a key - " +
			"the blue box the owner saw on one menu and no other")
	}

	// The keyboard moving here, with no window in it, draws the mark as it
	// always did - a return is not a licence to stay quiet afterwards.
	// Asked while nothing is marked, so a return that was never forgotten
	// answers with the old state and goes red here.
	menu.FocusLost()
	menu.FocusGained()
	if !menu.Marked() {
		t.Error("the keyboard arrived by a plain FocusGained and nothing was drawn, so the window's return has muted an ordinary arrival")
	}

	// The keyboard has been used - the mark is drawn - and the window went
	// behind and came back: what was drawn is drawn again.
	menu.TypedKey(&fyne.KeyEvent{Name: fyne.KeyRight})
	if !menu.Marked() {
		t.Fatal("an arrow key did not draw the mark, so the last part of this guard cannot start")
	}
	menu.FocusLost()
	if menu.Marked() {
		t.Fatal("losing the focus left the mark on")
	}
	menu.WindowReturning()
	menu.FocusGained()
	if !menu.Marked() {
		t.Error("the window came back and the mark a keyboard user had was gone, so they no longer know which control is listening")
	}

	// The same rule on the other controls that know who focused them, so the
	// class is held and not the one instance the owner saw.
	for _, control := range []interface {
		fyne.Focusable
		parts.Returnable
		Marked() bool
	}{
		checkNamed(content, text.FieldLabel()),
		parts.NewSegments([]string{"one", "two"}, func(string) {}),
	} {
		control.FocusLost()
		control.WindowReturning()
		control.FocusGained()
		if control.Marked() {
			t.Errorf("%T drew its mark when the window came back although it had none before", control)
		}
	}
}
