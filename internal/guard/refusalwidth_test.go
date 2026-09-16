package guard

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// A refusal gets the width of the form to say what it has to say.
//
// A refusal in this tool has four parts - what happened, why, what is allowed,
// what to do instead - which is a sentence and not a word. Where fields share
// a row, a message about one of them used to be laid out in a fraction of the
// form. Measured off a render on 2026-08-20: a size below what BMP can make
// wrapped onto four lines in the left column while the right half of the
// panel was empty, and those four lines pushed everything under them down by
// three.
//
// The controls share the row and the messages do not, now. Asserted against
// the row rather than against a number of pixels: what has to hold is that a
// message is not confined to the column its field is in.
//
// Asked on the batch screen, because that is where fields still share a row -
// the format, count and size of a file inside an archive stand in cells of
// one row (parts.Table.Row). The generate screen has had one field to a row since
// the form became a grid on 2026-09-14, and this guard read that screen until
// 2026-09-16: the full mutation run found it green while the table's Row put every
// message back into its cell, because nothing it measured went through
// a table's Row any more.
func TestARefusalIsAsWideAsTheFormRatherThanItsColumn(t *testing.T) {
	screen := window.NewRecipe(newFakeHost(t))
	body := screen.Object()
	w := test.NewWindow(body)
	t.Cleanup(w.Close)
	fields := screen.Fields()

	// A batch that is fine on its own, holding one file with a count and no
	// size, so the refusal from the reader lands on the size cell of the
	// contents row - one of its four columns, 185 px of the 788 the row has.
	setBox(t, fields, recipe.TargetAddress(1, recipe.KeyID), "filled")
	setBox(t, fields, recipe.TargetAddress(1, recipe.KeySize), "10kb")
	chooserIn(t, fields, recipe.TargetAddress(1, recipe.KeyFormat)).SetSelected("zip")
	pressNamed(t, body, text.ButtonAddContents())
	setBox(t, fields, recipe.ContentAddress(1, 1, recipe.KeyCount), "1")
	pressNamed(t, body, text.ButtonPreview())
	settle(body, w)

	at := recipe.ContentAddress(1, 1, recipe.KeySize)
	saying := saidBy(t, fields, at)
	if saying == "" {
		t.Fatalf("nothing is marked at %q, so there is no refusal to measure.\n%s", at, allSaid(fields))
	}
	said := refusalLabelSaying(body, saying)
	if said == nil {
		t.Fatalf("the size cell says %q and no red label on the screen carries it", saying)
	}

	// Halfway between one cell and the whole form, so the assertion holds
	// whatever the padding does and fails the moment the message goes back
	// into a cell.
	half := float32(parts.ColumnWidth) / 2
	if said.Size().Width <= half {
		t.Errorf("the refusal about the size is %.0f px wide, which is no more than the %.0f px cell it sits in."+
			" A message with four parts in a quarter of a form is a message that wraps four times",
			said.Size().Width, half)
	}
}

// And it names the box in the words above the box.
//
// The engine words a refusal once for both surfaces and names the setting by
// the key a recipe writes - "bmp: width cannot be ...". On the command line
// that is the only name there is. In this window the label above that box
// reads Width, so the refusal was naming something the screen does not have.
//
// The defect arrived WITH the labels, which is why it is guarded beside them:
// before those, the key and the label were the same string and this could not
// happen.
func TestARefusalNamesTheBoxTheWayTheScreenNamesIt(t *testing.T) {
	generate, choose, host := formatsLaidOut(t)
	choose.to("bmp")

	box := entryUnder(t, generate, text.SettingLabel("width"))
	if box == nil {
		t.Fatal("bmp declares width and the screen has no box for it")
	}
	box.SetText("99999")
	press(t, generate, text.ButtonGenerate())
	// Joined before anything is read, and without this the guard asked its
	// question of a screen that had not been answered yet.
	//
	// Planning went onto a worker on 2026-08-26, so pressing Generate no longer
	// means the refusal is on the screen. Measured on 2026-08-27, when the full
	// mutation run reported this guard as a hole: before the join the screen
	// holds the single word "Width", which is the LABEL of the box. Both checks
	// below then passed on nothing - the first is satisfied by that label, and
	// the second finds no line saying "cannot be" to look inside. After the
	// join the refusal is there and reads "bmp: Width cannot be ...", which is
	// what this guard exists to demand.
	join(host)

	shown := textIn(generate)
	label := text.SettingLabel("width")
	if !strings.Contains(shown, label) {
		t.Errorf("the screen never says %q after refusing that value. It says:\n%s", label, shown)
	}
	// The key, on its own, in the sentence that was refused. Searched for as a
	// whole word so that Width itself is not read as a hit.
	for _, line := range strings.Split(shown, "\n") {
		if !strings.Contains(line, "cannot be") {
			continue
		}
		if strings.Contains(line, " width ") || strings.HasPrefix(line, "width ") {
			t.Errorf("the refusal reads %q, which names the box by the key a recipe writes"+
				" rather than by the words above the box", line)
		}
	}
}

// refusalLabelSaying is the red label carrying a message with these words in it.
func refusalLabelSaying(o fyne.CanvasObject, words string) *widget.Label {
	var found *widget.Label
	walk(o, func(obj fyne.CanvasObject) {
		label, is := obj.(*widget.Label)
		if !is || found != nil {
			return
		}
		if label.Importance == widget.DangerImportance && strings.Contains(label.Text, words) {
			found = label
		}
	})
	return found
}
