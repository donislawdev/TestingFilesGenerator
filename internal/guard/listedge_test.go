package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// An open list stays inside the window.
//
// It used to go under the box at its full height, always. That is right until
// the box is near the foot of the form, and every form here is taller than its
// window - so a menu low on one opened straight through the bottom edge. The
// format menu showed four of its twenty values that way, with the rest past the
// edge and the run buttons hidden underneath it (O113).
//
// The menu is put at the foot of a short window rather than found on a screen,
// and that is the point rather than a shortcut: what decides this is how much
// room is left under the box, so the test has to be able to say how much there
// is. A screen where everything happens to fit proves nothing about a screen
// where it does not.
func TestAnOpenListStaysInsideTheWindow(t *testing.T) {
	app := test.NewApp()
	app.Settings().SetTheme(parts.Theme())
	t.Cleanup(func() { test.NewApp() })

	values := format.IDs()
	if len(values) < 8 {
		t.Skipf("this build has %d values, which is fewer than the list shows at once", len(values))
	}
	menu := parts.NewChooser(values, nil)

	// Hard against the bottom edge, which is where a form taller than its
	// window puts a control as soon as somebody scrolls.
	const height = 240
	w := test.NewWindow(container.NewBorder(nil, menu, nil, nil, container.NewVBox()))
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(400, height))
	w.Resize(fyne.NewSize(400, height))

	menu.Tapped(&fyne.PointEvent{})
	if menu.Opened() == nil {
		t.Fatal("the press opened no list")
	}

	// The popup itself, not the overlay holding it. The overlay covers the
	// whole canvas, so measuring THAT says the list fits however far past the
	// edge it goes - which is how the first version of this guard passed
	// against the very defect it was written for.
	pop := popUpIn(w.Canvas().Overlays().Top())
	if pop == nil {
		t.Fatal("the list is not on the canvas, so where it went cannot be read")
	}

	top := pop.Position().Y
	bottom := top + pop.Size().Height
	switch {
	case bottom > height:
		t.Errorf("the open list runs from %.0f to %.0f in a window %d px tall, so %.0f px of it is "+
			"past the bottom edge - along with the buttons underneath it.\n"+
			"What to do: parts.roomForList cuts the list to the room that is left and opens it "+
			"upward when there is more room above the box than below.",
			top, bottom, height, bottom-height)
	case top < 0:
		t.Errorf("the open list starts at %.0f, above the top of the window", top)
	case pop.Size().Height <= 0:
		t.Error("the open list has no height at all, which is not a list that fits - it is a list nobody can use")
	}

	// Upward, in this case. With the box against the foot there is no room
	// below it and the whole window above it, so a list that still opened
	// downward could only satisfy the check above by being almost nothing tall.
	if boxTop := menu.Position().Y; top >= boxTop {
		t.Errorf("the list opened at %.0f with the box at %.0f, so it went downward into the %.0f px "+
			"left under the box rather than upward into the room above it.",
			top, boxTop, height-boxTop)
	}
}

// An open list is cut to the room there is, when there is not much anywhere.
//
// Opening upward is only half of it. In a window with fewer than eight rows of
// room on either side, a list at its row ceiling goes past an edge whichever
// way it opens - so it has to be shorter than its ceiling, and the shortening
// has to reach the LIST rather than only the popup around it. Measured on
// 2026-08-19: a popup is never laid out smaller than its content's minimum, so
// resizing it alone left the list its full height and did nothing at all
// (O113).
func TestAnOpenListIsCutToTheRoomThereIs(t *testing.T) {
	app := test.NewApp()
	app.Settings().SetTheme(parts.Theme())
	t.Cleanup(func() { test.NewApp() })

	values := format.IDs()
	if len(values) < 8 {
		t.Skipf("this build has %d values, which is fewer than the list shows at once", len(values))
	}
	menu := parts.NewChooser(values, nil)

	// Shorter than eight rows either side of the box, so no whole list fits
	// whichever way it opens.
	const height = 120
	w := test.NewWindow(container.NewBorder(nil, menu, nil, nil, container.NewVBox()))
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(400, height))
	w.Resize(fyne.NewSize(400, height))

	menu.Tapped(&fyne.PointEvent{})
	pop := popUpIn(w.Canvas().Overlays().Top())
	if pop == nil {
		t.Fatal("the list is not on the canvas, so where it went cannot be read")
	}

	top, tall := pop.Position().Y, pop.Size().Height
	if top < 0 || top+tall > height {
		t.Errorf("the open list runs from %.0f to %.0f in a window %d px tall, so it is not cut to "+
			"the room there is.\n"+
			"What to do: the room has to be told to the LIST - parts.OpenList.LimitTo - because a "+
			"popup is never laid out smaller than its content's minimum.",
			top, top+tall, height)
	}
	if tall <= 0 {
		t.Error("the open list has no height at all, which is not a list cut to fit - it is a list nobody can use")
	}
}

// popUpIn is the popup inside the overlay that holds it.
func popUpIn(o fyne.CanvasObject) *widget.PopUp {
	var found *widget.PopUp
	walk(o, func(obj fyne.CanvasObject) {
		if pop, ok := obj.(*widget.PopUp); ok && found == nil {
			found = pop
		}
	})
	return found
}
