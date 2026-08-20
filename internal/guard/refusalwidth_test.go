package guard

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// A refusal gets the width of the form to say what it has to say.
//
// A refusal in this tool has four parts - what happened, why, what is allowed,
// what to do instead - which is a sentence and not a word. Two fields share a
// row, so a message about one of them used to be laid out in half the form.
// Measured off a render on 2026-08-20: a size below what BMP can make wrapped
// onto four lines in the left column while the right half of the panel was
// empty, and those four lines pushed everything under them down by three.
//
// The controls share the row and the messages do not, now. Asserted against
// the row rather than against a number of pixels: what has to hold is that a
// message is not confined to the column its field is in.
func TestARefusalIsAsWideAsTheFormRatherThanItsColumn(t *testing.T) {
	content, w := screenInAWindow(t, text.TabOneTarget)

	// A size no format can produce, so the refusal lands on the size box - and
	// the size box shares its row with how many.
	fill(t, content, text.FieldSize, "1")
	press(t, content, text.ButtonGenerate)
	settle(content, w)

	said := refusalLabelSaying(content, "BMP")
	if said == nil {
		t.Fatalf("nothing on the screen is complaining about the size. It says:\n%s", textIn(content))
	}
	box := controlUnder(content, text.FieldSize)
	if box == nil {
		t.Fatal("the screen has no size box, so this guard read the wrong tree")
	}

	// Halfway between one column and the whole form, so the assertion holds
	// whatever the padding does and fails the moment the message goes back
	// into a column.
	half := float32(parts.ColumnWidth) / 2
	if said.Size().Width <= half {
		t.Errorf("the refusal about the size is %.0f px wide, which is no more than the %.0f px column it sits in."+
			" A message with four parts in half a form is a message that wraps four times",
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
	generate, choose := formatsLaidOut(t)
	choose.to("bmp")

	box := entryUnder(t, generate, text.SettingLabel("width"))
	if box == nil {
		t.Fatal("bmp declares width and the screen has no box for it")
	}
	box.SetText("99999")
	press(t, generate, text.ButtonGenerate)

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
