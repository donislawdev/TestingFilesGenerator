package guard

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// foldButtons are the controls that put a batch away, in the order the batches
// are drawn in.
//
// Found by shape rather than by words: a fold is the only button on this screen
// with an icon and nothing written on it. The button that opens a field's
// longer explanation looks the same from a distance and is a *parts.DetailButton,
// which embeds widget.Button rather than being one - so it does not answer this
// type assertion, and that is checked by the count below rather than assumed.
func foldButtons(o fyne.CanvasObject) []*widget.Button {
	var out []*widget.Button
	walk(o, func(obj fyne.CanvasObject) {
		if b, ok := obj.(*widget.Button); ok && b.Text == "" && b.Icon != nil {
			out = append(out, b)
		}
	})
	return out
}

func foldBatch(t *testing.T, o fyne.CanvasObject, position int) {
	t.Helper()
	all := foldButtons(o)
	if position < 1 || position > len(all) {
		t.Fatalf("batch %d has no fold, and there are %d of them", position, len(all))
	}
	all[position-1].OnTapped()
}

// TestAFoldedBatchSaysWhatIsInIt keeps a folded batch from being a bare title.
//
// A column of headings somebody opens one by one to find the one they meant is
// a worse screen than a long one. What the line carries is what tells two
// batches apart at a glance, and it has to come from what was typed rather than
// from a placeholder - so this fills the boxes in and reads the line back.
func TestAFoldedBatchSaysWhatIsInIt(t *testing.T) {
	batches := window.NewRecipe(&fakeHost{}).Object()

	entryUnder(t, batches, text.FieldTargetID()).SetText("invoices")
	entryUnder(t, batches, text.FieldCount()).SetText("7")
	chooseSizeWay(t, batches, text.SizeWayExact())
	entryUnder(t, batches, text.FieldSize()).SetText("2mb")

	foldBatch(t, batches, 1)

	said := shownText(batches)
	for _, want := range []string{"invoices", "7", "2mb"} {
		if !strings.Contains(said, want) {
			t.Errorf("a folded batch does not say %q anywhere, so nothing tells it from the one below:\n%s",
				want, said)
		}
	}
	// And the form really is away, or the line is decoration on a screen that
	// saved nothing.
	if strings.Contains(said, text.FieldNameTemplate()) {
		t.Errorf("the batch is folded and its fields are still on the screen:\n%s", said)
	}
}

// TestARefusalOpensTheBatchItIsAbout is the guard that lets this screen fold at
// all, and it is worth saying why in full.
//
// Putting batches away was rejected on 2026-08-18, with the reason written into
// the screen: a refusal names the batch it is about, a batch that is not on the
// screen has no box to mark, so refusals about it would fall back to the foot of
// the form - which is the defect the whole addressing effort removed. The
// rejection was of a list with one batch open at a time, and it was right.
//
// Folding is only different from that shape while this holds. Every box is
// registered whether or not it is on the screen, so a refusal finds it, and the
// screen opens what it has to before moving the form.
func TestARefusalOpensTheBatchItIsAbout(t *testing.T) {
	host := &fakeHost{}
	screen := window.NewRecipe(host)
	batches := screen.Object()

	// Two batches, the first one answerable and the second one refused.
	entryUnder(t, batches, text.FieldTargetID()).SetText("first")
	entryUnder(t, batches, text.FieldSize()).SetText("1kb")
	pressNamed(t, batches, text.ButtonAddBatch())

	fields := screen.Fields()
	setBox(t, fields, recipe.TargetAddress(2, recipe.KeyID), "second")
	setBox(t, fields, recipe.TargetAddress(2, recipe.KeySize), "not a size at all")

	// Counted by a field NAME rather than by what was typed, because what is
	// typed lives in an entry and the words on the screen are labels. Two
	// batches with one folded leave one of each name showing.
	foldBatch(t, batches, 2)
	if got := strings.Count(shownText(batches), text.FieldNameTemplate()); got != 1 {
		t.Fatalf("batch 2 was folded and %d boxes called %q are still on the screen, so this guard "+
			"cannot tell whether a refusal opened it", got, text.FieldNameTemplate())
	}

	pressNamed(t, batches, text.ButtonPreview())
	join(host)

	if got := strings.Count(shownText(batches), text.FieldNameTemplate()); got != 2 {
		t.Errorf("the run was refused because of a box inside a folded batch and the batch stayed shut, " +
			"so the screen refuses to run and marks nothing anybody can see")
	}
	// And the refusal itself is where somebody can read it, not just the box it
	// is about back on the screen.
	if said := shownText(batches); !strings.Contains(said, "not a size at all") {
		t.Errorf("the box that was refused is on the screen and what it says is not:\n%s", said)
	}
}

// TestACopiedBatchCarriesEverythingButItsName is what makes several batches
// quick to write.
//
// Batches usually differ in one setting, so writing the second from an empty
// form is typing the first one again. The name is the one thing NOT carried
// over: two targets with one id is a refusal the recipe reader already words,
// and a copy that arrives refused is a copy somebody has to repair before they
// can use it.
func TestACopiedBatchCarriesEverythingButItsName(t *testing.T) {
	screen := window.NewRecipe(&fakeHost{})
	batches := screen.Object()

	entryUnder(t, batches, text.FieldTargetID()).SetText("invoices")
	entryUnder(t, batches, text.FieldCount()).SetText("7")
	chooseSizeWay(t, batches, text.SizeWayRange())
	entryUnder(t, batches, text.FieldSizeRange()).SetText("1kb-8kb")

	pressNamed(t, batches, text.ButtonDuplicateBatch())

	if got := len(foldButtons(batches)); got != 2 {
		t.Fatalf("copying a batch left %d of them", got)
	}

	fields := screen.Fields()
	for _, want := range []struct {
		setting string
		value   string
	}{
		{recipe.KeyCount, "7"},
		{recipe.KeySizeRange, "1kb-8kb"},
	} {
		if got := boxText(t, fields, recipe.TargetAddress(2, want.setting)); got != want.value {
			t.Errorf("the copy has %q in its %s and the batch it came from has %q",
				got, want.setting, want.value)
		}
	}
	if got := boxText(t, fields, recipe.TargetAddress(2, recipe.KeyID)); got != "" {
		t.Errorf("the copy is named %q, so it arrives refused for sharing a name with the batch above it", got)
	}
	// And the way of stating a size came with it, or the value copied into the
	// range box is in a box nobody is looking at.
	if got := sizeWaySwitchIn(t, batches, 2).Selected; got != text.SizeWayRange() {
		t.Errorf("the copy states its size as %q where the batch it came from says %q",
			got, text.SizeWayRange())
	}
}
