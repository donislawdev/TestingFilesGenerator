package guard

import (
	"testing"

	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// Add a batch does not read as a pair with the button before it.
//
// They stand side by side at the left of the action bar in the same style, and
// they are the least related controls in the window: Add a batch adds to the
// form in front of you, and the buttons before it - Donate, and GitHub since
// 2026-10-14 - hand an address to your browser. Side by side in one style,
// proximity says they belong together - which is the only thing proximity
// ever says.
//
// Asked of whatever stands nearest before it, found by where it is drawn, and
// not of Donate by name. Until 2026-10-14 this measured from Donate, and when
// GitHub arrived between the two the gap it measured held a whole button - so
// it stayed green with the boundary deleted, asking a question the screen no
// longer posed.
//
// Answered with a boundary rather than by moving the button. Under the last
// batch is where the button belongs by proximity, and a guard already keeps it
// reachable without scrolling - at three batches it would be off the screen, so
// somebody would have to scroll to add a fourth. That guard is the recorded
// decision and this change does not touch it.
//
// The comparison is with two buttons that ARE a pair. Preview and Generate do
// the same kind of thing to the same form, they sit together on purpose, and
// whatever gap they use is what "these belong together" looks like on this
// screen. Anything claiming to separate has to be wider than that.
func TestAddABatchIsNotDrawnAsAPairWithTheButtonBeforeIt(t *testing.T) {
	content, w := screenInAWindow(t, text.TabRecipe())
	settle(content, w)

	add := buttonNamed(content, text.ButtonAddBatch())
	preview := buttonNamed(content, text.ButtonPreview())
	generate := buttonNamed(content, text.ButtonGenerate())
	for name, b := range map[string]fyne.CanvasObject{
		text.ButtonAddBatch(): add, text.ButtonPreview(): preview, text.ButtonGenerate(): generate,
	} {
		if b == nil {
			t.Fatalf("the batch screen has no %q button, so this guard read the wrong tree", name)
		}
	}

	before, name := buttonJustBefore(content, add)
	if before == nil {
		t.Fatalf("nothing stands before %q in its row, so this guard measured nothing. The screen carries: %v",
			text.ButtonAddBatch(), buttonNames(content))
	}

	apart := horizontalGap(t, content, before, add)
	together := horizontalGap(t, content, preview, generate)

	if apart <= together {
		t.Errorf("%q and %q are %.1f px apart and two buttons that do belong together are %.1f px apart."+
			" Two unrelated controls drawn at the spacing of a pair read as a pair",
			name, text.ButtonAddBatch(), apart, together)
	}
}

// buttonJustBefore is the button drawn nearest to the left of this one in the
// same row, and its words.
func buttonJustBefore(screen fyne.CanvasObject, of *parts.Button) (*parts.Button, string) {
	target, ok := objectBox(screen, of)
	if !ok {
		return nil, ""
	}
	var found *parts.Button
	edge := float32(-1)
	atAbsolute(screen, func(o fyne.CanvasObject, at fyne.Position) {
		b, isButton := o.(*parts.Button)
		if !isButton || b == of || !b.Visible() {
			return
		}
		right := at.X + o.Size().Width
		sameRow := at.Y < target.Y+target.Height && target.Y < at.Y+o.Size().Height
		if sameRow && right <= target.X && right > edge {
			found, edge = b, right
		}
	})
	if found == nil {
		return nil, ""
	}
	return found, found.Text
}

// horizontalGap is the empty space between the right edge of one thing and the
// left edge of the next.
func horizontalGap(t *testing.T, screen fyne.CanvasObject, left, right fyne.CanvasObject) float32 {
	t.Helper()
	a, ok := objectBox(screen, left)
	if !ok {
		t.Fatal("the button on the left is not laid out")
	}
	b, ok := objectBox(screen, right)
	if !ok {
		t.Fatal("the button on the right is not laid out")
	}
	return b.X - (a.X + a.Width)
}
