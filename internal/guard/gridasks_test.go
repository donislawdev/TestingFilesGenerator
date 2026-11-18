package guard

import (
	"fmt"
	"image/color"
	"slices"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// What a form grid asks its fields for, counted (O356).
//
// The toolkit asks every shown object for its smallest size before each
// frame it draws after a change - a notch of the wheel is one - and every
// answer asks everything inside it again. So a layout that asks a child
// twice makes every level above it pay twice. The grid asked a field five
// times a pass, and on 2026-11-18 forty batches on the batch screen cost
// 19.5 ms of the window's time per notch, past a frame. Asked once, the
// same forty cost 11 ms (docs/PLAN-AND-WHEEL-2026-11-18.md section 2).
//
// Nothing a person sees changes when a grid asks twice - the stored
// screens draw the same - so this is the one thing that would say so.

// sizeAsked stands in a grid and counts how often it is asked for its size.
type sizeAsked struct {
	*canvas.Rectangle
	times int
}

func newSizeAsked() *sizeAsked {
	return &sizeAsked{Rectangle: canvas.NewRectangle(color.Transparent)}
}

func (s *sizeAsked) MinSize() fyne.Size {
	s.times++
	return fyne.NewSize(60, 24)
}

// askingTwice is a layout that asks its child twice - the defect, planted,
// for the check to find.
type askingTwice struct{}

func (askingTwice) MinSize(objects []fyne.CanvasObject) fyne.Size {
	_ = objects[0].MinSize()
	return objects[0].MinSize()
}

func (askingTwice) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	objects[0].Resize(size)
}

// countedGrid is a grid with something counted in every kind of place a grid
// asks about: the box of a field and what stands after it, a field whose
// refusal is on the screen, a field in a group of settings - a grid inside
// the grid - and a sentence across the row. A box to tick stands among them
// for the way the grid asks it, uncounted, since its parts are the
// toolkit's. sentence wraps what stands across the row, so the canary can
// plant a layout there.
func countedGrid(t *testing.T, sentence func(fyne.CanvasObject) fyne.CanvasObject) (fyne.CanvasObject, map[string]*sizeAsked) {
	t.Helper()
	counted := map[string]*sizeAsked{}
	leaf := func(name string) *sizeAsked {
		counted[name] = newSizeAsked()
		return counted[name]
	}
	plain := parts.FieldSaying("Plain", parts.NoDetail, false, nil, leaf("the box of a field"))
	trailed := parts.FieldSaying("Trailed", parts.NoDetail, false, leaf("what stands after a box"), leaf("the box before it"))
	refused := parts.FieldSaying("Refused", parts.NoDetail, true, nil, leaf("the box of a refused field"))
	refused.Area.Say("this field is refused, so its refusal stands under the row")
	if !refused.Area.Object().Visible() {
		t.Fatal("the refusal under a field is not shown, so the grid is not asked about one")
	}
	group := parts.NewInnerFolding("Group", parts.FieldSaying("Inside", parts.NoDetail, false, nil, leaf("a box in a group of settings")).Object)
	if !group.IsOpen() {
		t.Fatal("the group of settings is folded, so the grid inside it is not asked")
	}
	tick := parts.ToggleSaying("Tick", parts.NoDetail, parts.NewToggle(nil))
	grid := parts.Grid(plain.Object, trailed.Object, tick.Object, refused.Object, group.Object(),
		parts.Wide(sentence(leaf("a sentence across the row"))))
	return grid, counted
}

// askedOnce asks a grid for its size once and says every counted thing that
// was not asked exactly once.
func askedOnce(grid fyne.CanvasObject, counted map[string]*sizeAsked) []string {
	for _, c := range counted {
		c.times = 0
	}
	grid.MinSize()
	var wrong []string
	for name, c := range counted {
		if c.times != 1 {
			wrong = append(wrong, fmt.Sprintf("%s was asked %d time(s)", name, c.times))
		}
	}
	slices.Sort(wrong)
	return wrong
}

// TestAGridAsksEveryPartOfAFieldForItsSizeOnce asks a grid for its size once
// and requires every counted thing in it to have been asked exactly once -
// not twice, which is the cost, and not never, which would be a field the
// grid did not reach.
func TestAGridAsksEveryPartOfAFieldForItsSizeOnce(t *testing.T) {
	ourTheme(t)

	planted, plantedCounted := countedGrid(t, func(o fyne.CanvasObject) fyne.CanvasObject {
		return container.New(askingTwice{}, o)
	})
	if wrong := askedOnce(planted, plantedCounted); len(wrong) == 0 {
		t.Fatal("canary: a sentence behind a layout that asks it twice passed, so this check cannot see a grid that asks twice")
	}

	grid, counted := countedGrid(t, func(o fyne.CanvasObject) fyne.CanvasObject { return o })
	for _, w := range askedOnce(grid, counted) {
		t.Errorf("one pass of a grid's size: %s, wanted once. A layout that asks a child more than once a pass is paid again at every level above it, every frame after a change (O356)", w)
	}
}
