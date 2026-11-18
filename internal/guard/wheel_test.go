package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// A notch of the mouse wheel moves every scrolling area of the window as far
// as the system says a notch moves - three lines of text out of the box, as
// many as Windows is set to, a whole view, or nothing - rather than the
// toolkit's own fixed step. Reported by the owner on 2026-11-17 as scrolling
// that "goes slowly": 32 notches down one screen (docs/SIZE-FIXES-2026-11-16.md
// section 3.8).
//
// Asked of every screen, because a screen built around the toolkit's scroll
// instead of parts.Scroll would scroll at the old step and look no different
// in any picture. Each screen is made short, so each has something below its
// fold, and the wheel is turned the way the toolkit's driver turns it.
func TestEveryScrollingAreaMovesAsFarAsTheSystemSays(t *testing.T) {
	was := parts.WheelLines
	t.Cleanup(func() { parts.WheelLines = was })

	checked := 0
	for i, tab := range []string{text.TabOneTarget(), text.TabPresets(), text.TabRecipe(), text.TabTools(), text.TabPreferences(), text.TabAbout()} {
		content, w := screenInAWindow(t, tab)
		w.Resize(fyne.NewSize(window.LargestOpening.Width, 320))
		content.Refresh()
		// A line of the window's text, measured once the window's theme is
		// in place - measured before it, the toolkit's own font answers, and
		// this guard first did exactly that.
		line := fyne.MeasureText("Ag", parts.TextBody, fyne.TextStyle{}).Height
		if i == 0 {
			bareScrollIsCaught(t, line)
		}

		var areas []*parts.Scroll
		bare := 0
		walk(content, func(o fyne.CanvasObject) {
			switch s := o.(type) {
			case *parts.Scroll:
				areas = append(areas, s)
			case *container.Scroll:
				bare++
			}
		})
		if bare > 0 {
			t.Errorf("%s draws %d scrolling area(s) of the toolkit's own, which move at its step and not the system's - use parts.VScroll", tab, bare)
		}
		form := tallest(areas)
		if form == nil {
			t.Errorf("%s has no scrolling area", tab)
			continue
		}
		view := form.Size().Height
		room := form.Content.MinSize().Height - view
		if room <= 0 {
			t.Errorf("%s fits a window %0.f high, so the wheel was not tried on it", tab, view)
			continue
		}
		for _, lines := range []int{3, 5, -1, 0} {
			parts.WheelLines = func() int { return lines }
			form.ScrollToTop()
			form.Scrolled(&fyne.ScrollEvent{Scrolled: fyne.NewDelta(0, -driverNotch)})
			want := notchMoves(lines, line, view)
			if !movedRight(form.Offset.Y, want, room) {
				t.Errorf("%s moved %.1f for a notch with the system set to %d line(s), and %.1f was asked for",
					tab, form.Offset.Y, lines, min(want, room))
			}
		}
		form.ScrollToTop()
		checked++
	}
	if checked < 6 {
		t.Fatalf("the wheel was turned on %d screen(s) of six", checked)
	}
}

// bareScrollIsCaught is the canary: the toolkit's own scroll, turned one
// notch, has to be told apart from what the system asks for - or the guard
// above passes with every area back on the toolkit's step. On macOS the two
// are the same by design (parts/wheel_darwin.go), so there is nothing to tell
// apart there.
func bareScrollIsCaught(t *testing.T, line float32) {
	t.Helper()
	expected := notchMoves(3, line, 1000)
	if expected == driverNotch {
		return
	}
	bare := container.NewVScroll(container.NewGridWrap(fyne.NewSize(10, 5000)))
	bare.Resize(fyne.NewSize(100, 200))
	bare.Scrolled(&fyne.ScrollEvent{Scrolled: fyne.NewDelta(0, -driverNotch)})
	if movedRight(bare.Offset.Y, expected, 5000) {
		t.Fatalf("canary: the toolkit's own scroll moved %.1f for a notch and the check took it for %.1f", bare.Offset.Y, expected)
	}
}

// movedRight says an area turned one notch moved by want, or to its end when
// want is further than there is room for.
func movedRight(got, want, room float32) bool {
	want = min(want, room)
	return got > want-0.5 && got < want+0.5
}

// tallest is the area holding the most, which on every screen is its form.
func tallest(areas []*parts.Scroll) *parts.Scroll {
	var found *parts.Scroll
	for _, a := range areas {
		if found == nil || a.Content.MinSize().Height > found.Content.MinSize().Height {
			found = a
		}
	}
	return found
}
