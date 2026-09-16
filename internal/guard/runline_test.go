package guard

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// The line at the foot of a form says what the form comes to, and says it
// LIVE.
//
// G6: the window says what a run will cost before anything is pressed. Until
// 2026-09-14 the bar said one thing at rest - the directory - and gave it up
// at the first press for good, so after a preview the line carried a cost
// worked out for values somebody had typed over since. The line is worked
// out from the form on every change, which is what these guards hold it to:
// the count and the total follow the boxes, the kind follows the MENU, a
// form that does not settle falls back to naming the destination, and a
// change of the form puts the summary back over whatever a press said.
//
// Read off the status line - the label that shares a box with the progress
// track - rather than searched for anywhere on the screen, so words in the
// wrong place would not pass.

// The line's count and total follow what is typed, to the byte.
func TestTheLineSaysWhatTheFormComesTo(t *testing.T) {
	_, content := screen(t)
	fill(t, content, text.FieldSize(), "4kb")
	fill(t, content, text.FieldCount(), "3")

	format := chooserUnder(t, content, text.FieldFormat()).Selected
	dir := entryUnder(t, content, text.FieldOutputDir()).Text
	if format == "" || dir == "" {
		t.Fatal("the format menu or the output directory box is empty, so this guard has nothing to hold the line to")
	}
	// 3 x 4096, said the way the line says a size: readable, then exact.
	want := text.RunLine(3, text.SizeAndBytes(core.HumanBytes(12288), core.ExactBytes(12288)),
		[]string{format}, dir)
	if got := statusLine(t, content); got != want {
		t.Errorf("three files of 4kb make the line say\n  %q\nand they add up to\n  %q", got, want)
	}
}

// The kind follows the menu, not only the boxes.
//
// Boxes reported every keystroke to the runner from the start, and menus
// reported nothing - the only listener was the live check, and a menu has
// no value the check could refuse. A line fed by that mechanism named the
// format chosen when the screen was built, whatever was chosen since.
func TestTheLineFollowsTheMenu(t *testing.T) {
	_, content := screen(t)
	choose(t, content, text.FieldFormat(), "png")
	if got := statusLine(t, content); !strings.Contains(got, text.Formats([]string{"png"})) {
		t.Fatalf("png was chosen and the line says %q, so this guard cannot tell whether the next choice reaches it", got)
	}
	choose(t, content, text.FieldFormat(), "txt")
	got := statusLine(t, content)
	if !strings.Contains(got, text.Formats([]string{"txt"})) || strings.Contains(got, "png") {
		t.Errorf("txt was chosen from the menu and the line still says %q", got)
	}
}

// A form that does not settle falls back to naming the destination, rather
// than keeping the summary of the last form that did. The destination is
// read off its own box, and that box is fine.
func TestTheLineFallsBackToTheDestinationWhenTheFormDoesNotSettle(t *testing.T) {
	_, content := screen(t)
	fill(t, content, text.FieldCount(), "3")
	if got := statusLine(t, content); !strings.HasPrefix(got, "3 files") {
		t.Fatalf("the line says %q before the box is spoiled, so this guard is not in the state it means to check", got)
	}

	fill(t, content, text.FieldCount(), "three")

	dir := entryUnder(t, content, text.FieldOutputDir()).Text
	if got, want := statusLine(t, content), text.WritingTo(dir); got != want {
		t.Errorf("the count box says \"three\" and the line says %q - the honest line is %q", got, want)
	}
}

// A form drawing its sizes from a range says so, with both ends, instead of
// adding up numbers that have not been drawn yet.
func TestTheLineSaysBetweenForARange(t *testing.T) {
	batches := window.NewRecipe(newFakeHost(t)).Object()
	// A batch settles only once it has a name - the name anchors its seed -
	// so the screen opens naming the destination alone and this fills it in.
	entryUnder(t, batches, text.FieldTargetID()).SetText("spread")
	entryUnder(t, batches, text.FieldCount()).SetText("2")
	chooseSizeWay(t, batches, text.SizeWayRange())
	entryUnder(t, batches, text.FieldSizeRange()).SetText("1kb-8kb")

	want := text.SizeBetween(core.HumanBytes(2*1024), core.HumanBytes(2*8192))
	got := statusLine(t, batches)
	if !strings.Contains(got, want) {
		t.Errorf("two files of 1kb-8kb make the line say %q, and the honest total is %q", got, want)
	}
	if !strings.HasPrefix(got, "2 files") {
		t.Errorf("two files were asked for and the line says %q", got)
	}
}

// After a preview the range is drawn, so the line says one number - and the
// disk has been asked, so the destination carries the room left on it - and
// the line says that nothing was written.
func TestAPreviewMakesTheLineExact(t *testing.T) {
	host := newFakeHost(t)
	batches := window.NewRecipe(host).Object()
	t.Cleanup(func() { join(host) })
	dir := t.TempDir()
	entryUnder(t, batches, text.FieldOutputDir()).SetText(dir)
	entryUnder(t, batches, text.FieldTargetID()).SetText("spread")
	entryUnder(t, batches, text.FieldCount()).SetText("2")
	chooseSizeWay(t, batches, text.SizeWayRange())
	entryUnder(t, batches, text.FieldSizeRange()).SetText("1kb-8kb")
	if got := statusLine(t, batches); !strings.Contains(got, "between") {
		t.Fatalf("before the preview the line says %q, so this guard is not in the state it means to check", got)
	}

	press(t, batches, text.ButtonPreview())
	join(host)

	got := statusLine(t, batches)
	if strings.Contains(got, "between") {
		t.Errorf("the preview drew every size and the line still says %q", got)
	}
	if !strings.Contains(got, text.AndNothingWrittenYet()) {
		t.Errorf("after the preview the line says %q, which does not say that nothing was written", got)
	}
	// The room on the disk, beside the destination.
	if !strings.Contains(got, dir+" (") || !strings.Contains(got, "free)") {
		t.Errorf("after the preview the line says %q, which does not carry the room left in %q", got, dir)
	}
}

// A change of the form puts the summary back over what a press said.
//
// The line used to be one way: the destination until the first press, then
// whatever the press said, for good - so after a preview, editing a box left
// a cost on the line worked out for values that were no longer on the form.
// Now the form owns the line at rest, and a press borrows it.
func TestAChangeOfTheFormPutsTheLineBackOverWhatAPressSaid(t *testing.T) {
	host, content := screen(t)
	fill(t, content, text.FieldOutputDir(), t.TempDir())
	fill(t, content, text.FieldSize(), "4kb")
	fill(t, content, text.FieldCount(), "3")
	press(t, content, text.ButtonPreview())
	join(host)
	if got := statusLine(t, content); !strings.Contains(got, text.AndNothingWrittenYet()) {
		t.Fatalf("the preview left the line saying %q, so this guard is not in the state it means to check", got)
	}

	fill(t, content, text.FieldCount(), "4")

	got := statusLine(t, content)
	if strings.Contains(got, text.AndNothingWrittenYet()) {
		t.Errorf("the count was changed after the preview and the line still says %q - a cost worked out for a form that no longer exists", got)
	}
	if !strings.HasPrefix(got, "4 files") {
		t.Errorf("the count was changed to four and the line says %q", got)
	}
}

// statusLine is what the line under the buttons says.
func statusLine(t *testing.T, screen fyne.CanvasObject) string {
	t.Helper()
	_, status := runMessages(screen)
	if status == nil {
		t.Fatal("this screen has no status line, so this guard read the wrong tree")
	}
	if !status.Visible() {
		return ""
	}
	return status.Text
}

// absoluteOf is where an object stands on the screen, from the screen's
// origin rather than its parent's.
func absoluteOf(root, target fyne.CanvasObject) (fyne.Position, bool) {
	var at fyne.Position
	found := false
	atAbsolute(root, func(o fyne.CanvasObject, pos fyne.Position) {
		if o == target && !found {
			at, found = pos, true
		}
	})
	return at, found
}
