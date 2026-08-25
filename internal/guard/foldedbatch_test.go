package guard

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// foldRow is one fold as this file finds it: the words in its head and the
// button that puts it away.
type foldRow struct {
	title  string
	toggle *widget.Button
}

// foldRows is every fold on a screen, in the order the tree holds them.
//
// Found by shape AND by the title beside it, which is a change of 2026-08-25
// and the reason is worth stating. A fold used to be the only button on this
// screen with an icon and nothing written on it, so counting those was enough
// and the batches came out in order. Then batches gained folds INSIDE them -
// settings and manifest notes - and the same count returned six buttons for two
// batches, so "the fold of batch 2" was the settings of batch 1. That is O118
// again: nothing broke in the guard or in the screen, the shape it identified
// its subject by stopped identifying it.
//
// The title is read out of the same row as the button rather than by position
// in it. Positions in that row have moved before - a star, a byte count and a
// detail button have all been added to a field's name row, and each time three
// walks and a probe that read position 1 stopped reading what they meant.
//
// The button that opens a field's longer explanation looks the same from a
// distance and is a *parts.DetailButton, which embeds widget.Button rather than
// being one, so it does not answer this type assertion. It also has no title
// beside it, which is the second reason it cannot be mistaken for a fold here.
func foldRows(o fyne.CanvasObject) []foldRow {
	var out []foldRow
	walk(o, func(obj fyne.CanvasObject) {
		row, ok := obj.(*fyne.Container)
		if !ok {
			return
		}
		var toggle *widget.Button
		title := ""
		for _, item := range row.Objects {
			switch found := item.(type) {
			case *widget.Button:
				if found.Text == "" && found.Icon != nil {
					toggle = found
				}
			case *widget.Label:
				if title == "" {
					title = found.Text
				}
			}
		}
		if toggle != nil && title != "" {
			out = append(out, foldRow{title: title, toggle: toggle})
		}
	})
	return out
}

// foldTitled is the fold with these words in its head, counting from the one
// named. Titles repeat - every batch has a section called "Settings for bmp" -
// so a section is asked for as the first one after the batch it belongs to.
func foldTitled(t *testing.T, o fyne.CanvasObject, after, title string) *widget.Button {
	t.Helper()
	rows := foldRows(o)
	from := 0
	if after != "" {
		from = -1
		for i, row := range rows {
			if row.title == after {
				from = i + 1
				break
			}
		}
		if from < 0 {
			t.Fatalf("there is no fold headed %q, and the screen has %v", after, foldTitles(rows))
		}
	}
	for _, row := range rows[from:] {
		if row.title == title {
			return row.toggle
		}
	}
	t.Fatalf("there is no fold headed %q after %q, and the screen has %v", title, after, foldTitles(rows))
	return nil
}

func foldTitles(rows []foldRow) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.title)
	}
	return out
}

// batchFolds is how many batches are drawn, counted by their headings rather
// than by how many folds there are - a batch holds folds of its own.
//
// Counted by asking for the exact heading of batch 1, then batch 2, and so on,
// rather than by matching the word in front of the number. The heading comes
// out of the catalogue and a second language is free to put the number first.
func batchFolds(o fyne.CanvasObject) int {
	titles := map[string]bool{}
	for _, row := range foldRows(o) {
		titles[row.title] = true
	}
	count := 0
	for titles[text.BatchHeading(count+1)] {
		count++
	}
	return count
}

func foldBatch(t *testing.T, o fyne.CanvasObject, position int) {
	t.Helper()
	foldTitled(t, o, "", text.BatchHeading(position)).OnTapped()
}

// openFold opens a fold that is shut, and says so if it was open already.
//
// Asserting the state rather than assuming it, which is the lesson O118 keeps
// teaching in this package: a guard that presses a toggle blindly SHUTS a fold
// somebody has since made open by default, and then measures a screen it
// believes it opened. The state is read off the arrow, which is the same thing
// a person reads it off.
func openFold(t *testing.T, o fyne.CanvasObject, after, title string) {
	t.Helper()
	toggle := foldTitled(t, o, after, title)
	if toggle.Icon == nil || toggle.Icon.Name() != theme.MenuExpandIcon().Name() {
		t.Fatalf("the section headed %q is already open, so this guard is not asking what it thinks", title)
	}
	toggle.OnTapped()
}

// assertFoldOpen says the fold is open without touching it.
//
// The pair to openFold, for a guard that opens one section and then meets the
// same section again - a fold is remembered across a change of format, so
// pressing the toggle a second time would shut it and leave the guard measuring
// a screen with nothing on it.
func assertFoldOpen(t *testing.T, o fyne.CanvasObject, after, title string) {
	t.Helper()
	toggle := foldTitled(t, o, after, title)
	if toggle.Icon != nil && toggle.Icon.Name() == theme.MenuExpandIcon().Name() {
		t.Fatalf("the section headed %q is shut, so whatever is measured next is not on the screen", title)
	}
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

	if got := batchFolds(batches); got != 2 {
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
