package guard

import (
	"fmt"

	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// laidOutWindow opens the whole window into a test canvas and lays it out.
//
// The whole window rather than one screen, because reaching a control is a
// question about what is in front of it, and the tabs, the overlays and the
// other screens are all in the same canvas. A screen taken out of the window
// answers an easier question than the one a person asks.
func laidOutWindow(t *testing.T) (fyne.CanvasObject, fyne.Canvas) {
	t.Helper()
	host := &fakeHost{}
	window.Open(host)
	if host.content == nil {
		t.Fatal("opening the window put no screen in it")
	}
	w := test.NewWindow(host.content)
	t.Cleanup(w.Close)
	// Taller than the window opens, on purpose, and this is a separation of
	// questions rather than a convenience.
	//
	// Both screens sit in a vertical scroll, so at the opening size anything
	// past the fold is laid out below the canvas and a press at its own
	// coordinates lands nowhere. That is not a control being unreachable, it
	// is a control needing a scroll first, and conflating the two would make
	// this guard fail for a reason it is not asking about. Measured with
	// tools/probes/formheight on 2026-08-13: the preset form wants 1019 px and
	// the scroll offers 794, so it is 225 px too tall and the Choose button
	// falls off the bottom.
	//
	// Whether a form fits is already answered, with a number, by that probe.
	// What is left for this one is whether a control that HAS room can be
	// pressed - so it gets the room.
	w.Resize(fyne.NewSize(window.OpenSize.Width, 1600))
	return host.content, w.Canvas()
}

func allTabs() []string {
	return []string{text.TabOneTarget, text.TabPresets, text.TabAbout}
}

// What this defends. A button a person can see is a button a person can press.
//
// Why it needed a guard. Every window test in this package presses by calling
// the handler - b.OnTapped() - and one of them checks Disabled() first because
// somebody remembered to. Calling the handler is not pressing: it reaches a
// button that is disabled, a button covered by an overlay, a button of zero
// size and a button laid out past the edge of the window. Measured on
// 2026-08-13: seventeen call sites across eight files reach widgets that way,
// and none of them could fail for any of those four reasons.
//
// So this one goes through the canvas. test.TapCanvas resolves the target with
// FindObjectAtPositionMatching against Overlays().Top() first and Content()
// second, which is the same hit testing a mouse gets, and widget.Button.Tapped
// returns early when Disabled - both read in the toolkit source at v2.8.0.
//
// The handler is swapped for a sentinel before the tap and put back after, for
// two reasons. It records whether the press ARRIVED, which is the whole
// question, and it stops the real action running - pressing Generate for real
// would start a run inside a test that is asking about geometry.
//
// Both directions are asserted. An enabled button must receive the tap and a
// disabled one must not, because a guard that only checks the first would pass
// on a screen where nothing is ever disabled.
func TestEveryButtonAPersonCanSeeIsReallyPressable(t *testing.T) {
	for _, tab := range allTabs() {
		t.Run(tab, func(t *testing.T) {
			test.NewApp()
			defer test.NewApp()

			content, canvas := laidOutWindow(t)
			screen := selectTab(t, content, tab)

			checked, unreachable := 0, 0
			walk(screen, func(o fyne.CanvasObject) {
				button, ok := o.(*widget.Button)
				if !ok || !button.Visible() {
					return
				}
				checked++

				size := button.Size()
				if size.Width <= 0 || size.Height <= 0 {
					unreachable++
					t.Errorf("the %q button is %gx%g, so there is nothing to press",
						button.Text, size.Width, size.Height)
					return
				}

				at := fyne.CurrentApp().Driver().AbsolutePositionForObject(button)
				centre := at.Add(fyne.NewPos(size.Width/2, size.Height/2))

				arrived := false
				was := button.OnTapped
				button.OnTapped = func() { arrived = true }
				test.TapCanvas(canvas, centre)
				button.OnTapped = was

				switch {
				case button.Disabled() && arrived:
					t.Errorf("the %q button is disabled and a press still reached it", button.Text)
				case !button.Disabled() && !arrived:
					unreachable++
					t.Errorf("the %q button is on the screen at %v and a press at its centre %v "+
						"reached something else. Either something is drawn over it or it is laid "+
						"out where the canvas is not", button.Text, at, centre)
				}
			})

			if checked == 0 {
				t.Logf("no buttons on %q", tab)
				return
			}
			t.Logf("%s: %d button(s) checked, %d unreachable", tab, checked, unreachable)
		})
	}
}

// What this defends. UX9 asks that everything reachable with a mouse is
// reachable from the keyboard. Half of that was measured - a focused field
// draws a ring, a focused toggle draws a disc - and the half about ORDER was
// never asked at all. That is O84, open since the window was built.
//
// This walks the focus chain the way Tab does, through test.FocusNext, and
// reports what it finds rather than what somebody expected.
func TestTabbingReachesTheControlsAndSaysInWhatOrder(t *testing.T) {
	for _, tab := range allTabs() {
		t.Run(tab, func(t *testing.T) {
			test.NewApp()
			defer test.NewApp()

			content, canvas := laidOutWindow(t)
			screen := selectTab(t, content, tab)

			// What a person can see AND operate on this screen. Disabled
			// controls are left out on purpose: the focus chain skips them,
			// and that is right rather than a fault, so counting them here
			// would make this guard report a defect every time a screen
			// greys something out.
			onScreen := map[fyne.Focusable]bool{}
			walk(screen, func(o fyne.CanvasObject) {
				f, ok := o.(fyne.Focusable)
				if !ok || !o.Visible() {
					return
				}
				if off, ok := o.(fyne.Disableable); ok && off.Disabled() {
					return
				}
				onScreen[f] = true
			})

			// The chain, walked until it repeats. The ceiling exists because a
			// screen with nothing focusable would otherwise be walked forever,
			// and About is such a screen.
			var order []fyne.Focusable
			seen := map[fyne.Focusable]bool{}
			ceiling := len(onScreen)*2 + 8
			for range ceiling {
				// The canvas moves the focus itself. test.FocusNext does the
				// same thing and is deprecated in v2.8.0, which staticcheck
				// said before this ever ran anywhere but here.
				canvas.FocusNext()
				focused := canvas.Focused()
				if focused == nil || seen[focused] {
					break
				}
				seen[focused] = true
				order = append(order, focused)
			}

			t.Logf("%s: %d focusable on the screen, %d in the tab chain", tab, len(onScreen), len(order))
			for i, f := range order {
				at := fyne.Position{}
				if o, ok := f.(fyne.CanvasObject); ok {
					at = fyne.CurrentApp().Driver().AbsolutePositionForObject(o)
				}
				t.Logf("  %2d. y=%-7.1f x=%-7.1f %s%s",
					i+1, at.Y, at.X, describeFocusable(f), offScreenNote(onScreen, f))
			}
			for f := range onScreen {
				if !seen[f] {
					t.Errorf("%s cannot be reached with Tab, so this screen is not operable "+
						"from the keyboard (UX9)", describeFocusable(f))
				}
			}

			// There is deliberately NO assertion about the order here, and the
			// reason is a measurement rather than laziness.
			//
			// The first version of this test asserted that Tab follows the
			// order an eye reads: down the screen, then across. It failed on
			// the generate screen at step 4, and the failure was the test's.
			// The size box and the count box sit on ONE row a pixel apart
			// (y=299.3 and y=298.3), so an exact comparison called them two
			// rows and put them in the wrong order itself.
			//
			// Fixing that with a row tolerance would not have saved the idea.
			// The real chain runs help button, field, help button, field for
			// the LEFT column and then the right one - measured at y=373, 413,
			// 373, 413 - so it is grouped by field, not by row. For a two
			// column form that is arguably the better order, and no monotonic
			// rule describes it.
			//
			// So what the right order IS remains a judgement, and this project
			// has one place for judgements about how things look: the owner.
			// What is asserted above is what is not a matter of taste - that
			// everything can be reached, and that the chain stays on the
			// screen a person is looking at. The order is logged, so a change
			// to it is visible in a run rather than silent.
			for _, f := range order {
				if !onScreen[f] {
					t.Errorf("Tab reaches %s, which is not on the %q screen. Focus is leaving "+
						"the screen a person is looking at", describeFocusable(f), tab)
				}
			}
		})
	}
}

// describeFocusable names a control the way a person would, falling back to
// its type when it carries no words of its own.
func describeFocusable(f fyne.Focusable) string {
	switch control := f.(type) {
	case *widget.Entry:
		if control.PlaceHolder != "" {
			return fmt.Sprintf("a box hinting %q", control.PlaceHolder)
		}
		return fmt.Sprintf("a box holding %q", control.Text)
	case *parts.Toggle:
		return fmt.Sprintf("the toggle %q", control.Text)
	case *widget.Button:
		return fmt.Sprintf("the %q button", control.Text)
	}
	return fmt.Sprintf("%T", f)
}

// offScreenNote marks a control the chain reached that is not on the screen
// being asked about - which would mean Tab leaves the visible screen.
func offScreenNote(onScreen map[fyne.Focusable]bool, f fyne.Focusable) string {
	if onScreen[f] {
		return ""
	}
	return "   <- NOT ON THIS SCREEN"
}
