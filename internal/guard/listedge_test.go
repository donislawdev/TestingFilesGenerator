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
// Opening upward is only half of it. In a window with less room on either side
// of the box than the list's ceiling - a share of the window, so a few rows in
// a window this short - the list goes past an edge whichever way it opens. So
// it has to be shorter than its ceiling, and the shortening has to reach the
// LIST rather than only the popup around it. Measured on
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

	// Too short for the list either side of the box - its ceiling here is two
	// rows and the room on the roomier side is under three - so no whole list
	// fits whichever way it opens.
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

// A list opens downward whenever a few whole rows fit under its box, and
// upward only when fewer fit there than above it.
//
// Decision of the owner, 2026-09-21, from the running window: the format list
// on the preset screen opened UPWARD - over the question the preset asks -
// while the same list on the other screens opened downward, because the rule
// turned upward as soon as the list did not fit below and there was more room
// above. One control behaving two ways for a reason nobody could see. Now a
// list that has room for a few rows under its box goes there, shorter and
// scrolling, and upward is kept for a box standing just over the bar.
//
// Asked of the arithmetic directly, in rows, because the rule is about rows.
// The two guards above open real lists and hold the emergency half - a box at
// the foot still goes upward, and a cramped window still cuts the list.
func TestAListOpensDownwardWheneverAFewRowsFitUnderTheBox(t *testing.T) {
	app := test.NewApp()
	app.Settings().SetTheme(parts.Theme())
	t.Cleanup(func() { test.NewApp() })

	row := parts.ListRowHeight()
	const box = 31
	for _, tc := range []struct {
		name          string
		canvas, top   float32
		wantedRows    float32
		wantRows      float32
		wantDownward  bool
		whyItIsWorthA string
	}{
		{"the preset screen's format box in the owner's window", 870, 557, 24, 9, true,
			"274 px under the box holds nine rows, and that is where the list goes now"},
		{"a box with the whole list's room under it", 1300, 200, 10, 10, true,
			"the ordinary case, unchanged"},
		{"a box against the foot of the window", 240, 240 - box, 24, 4, false,
			"no row fits below, so upward is the only place (O113), and the ceiling of a 240 px window is four rows"},
		{"a few rows below and fewer above", 200, 60, 24, 3, true,
			"three rows below beat one above, whichever side has more"},
		{"two rows below and sixteen above", 600, 480, 24, 10, false,
			"fewer than the threshold below, and the ceiling of a 600 px window is ten rows"},
	} {
		height, at := parts.RoomForList(tc.canvas, tc.top, box, tc.wantedRows*row, 0)
		downward := at >= tc.top+box
		if downward != tc.wantDownward {
			t.Errorf("%s: the list opens %s and should open %s (%s)", tc.name,
				direction(downward), direction(tc.wantDownward), tc.whyItIsWorthA)
		}
		if height != tc.wantRows*row {
			t.Errorf("%s: the list is %.0f px tall, which is %.2f rows, and should be %.0f rows (%s)",
				tc.name, height, height/row, tc.wantRows, tc.whyItIsWorthA)
		}
		if !downward && at+height != tc.top {
			t.Errorf("%s: a list opening upward ends at %.0f and the box starts at %.0f", tc.name, at+height, tc.top)
		}
	}
}

func direction(downward bool) string {
	if downward {
		return "downward"
	}
	return "upward"
}
