package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// A list that opened downward is as tall as what its filter left, and one
// that opened upward keeps one height. The owner's report of 2026-09-24 from
// the running window was one format standing over a slab of empty grey: the
// list kept the height of all twenty six while somebody typed "jxl". Downward
// the box, the filter and every row stay put and only the foot moves. Upward
// the filter box at the top would move under the hands typing into it, which
// is why that list still does not.

// TestAListOpenedDownwardShrinksToWhatTheFilterLeft types into the format
// list on the first screen, which opens under its box.
func TestAListOpenedDownwardShrinksToWhatTheFilterLeft(t *testing.T) {
	cv, menu, list, filter := openFormatList(t)
	pop := popUpIn(cv.Overlays().Top())
	if pop == nil {
		t.Fatal("the list is not on the canvas")
	}
	box := fyne.CurrentApp().Driver().AbsolutePositionForObject(menu)
	if pop.Position().Y < box.Y+menu.Size().Height-0.5 {
		t.Fatalf("the format list opened at y=%.1f, above the foot of its box at y=%.1f - this guard is about a list that opened downward",
			pop.Position().Y, box.Y+menu.Size().Height)
	}
	top, full := pop.Position().Y, pop.Size().Height

	typeInto(filter, "jxl")
	want := list.HeadHeight() + 2*parts.ListRowHeight() // the heading and jxl
	if got := pop.Size().Height; got > want+0.5 || got < want-0.5 {
		t.Errorf("jxl typed leaves a heading and one format, %.1f px of list, and the list is %.1f px tall", want, got)
	}
	if pop.Position().Y != top {
		t.Errorf("the list's top moved from y=%.1f to y=%.1f while it shrank - the filter box moved under the hands", top, pop.Position().Y)
	}

	typeInto(filter, "")
	if got := pop.Size().Height; got != full {
		t.Errorf("the filter emptied, and the list is %.1f px tall where it opened at %.1f", got, full)
	}
}

// TestAListOpenedUpwardKeepsOneHeight puts the menu at the foot of a short
// window, where its list opens upward, and types into its filter.
func TestAListOpenedUpwardKeepsOneHeight(t *testing.T) {
	ourTheme(t)
	menu := parts.NewChooser(format.IDs(), nil)
	w := test.NewTempWindow(t, container.NewBorder(nil, menu, nil, nil, container.NewVBox()))
	w.Resize(fyne.NewSize(500, 400))
	menu.Tapped(&fyne.PointEvent{})
	list := menu.Opened()
	pop := popUpIn(w.Canvas().Overlays().Top())
	if list == nil || pop == nil || list.Filter() == nil {
		t.Fatal("the press opened no list with a filter")
	}
	box := fyne.CurrentApp().Driver().AbsolutePositionForObject(menu)
	if pop.Position().Y >= box.Y {
		t.Fatalf("the list opened at y=%.1f under a box at y=%.1f - this guard is about a list that opened upward", pop.Position().Y, box.Y)
	}
	top, height := pop.Position().Y, pop.Size().Height
	typeInto(list.Filter(), "jxl")
	if pop.Position().Y != top || pop.Size().Height != height {
		t.Errorf("typing moved an upward list from y=%.1f, %.1f px tall, to y=%.1f, %.1f px - its filter box moved under the hands",
			top, height, pop.Position().Y, pop.Size().Height)
	}
}
