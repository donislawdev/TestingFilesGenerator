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
)

// The form stops at a readable width however wide the window is.
//
// O72, measured twice: maximised to 3862 px, every box on the screen was 3848
// to 3854 px of it, so the seed field holding "0" was nearly four thousand
// pixels wide. UX6 asks it as a question - run your eye along a row to the
// right edge, and if you got lost the row is too long.
//
// This guard exists because the comment beside parts.ColumnWidth calls it a
// ratchet that can be tightened and never widened, and a ratchet nothing turns
// is a sentence. A VBox stretches its children to whatever it is handed, so
// the old behaviour is one wrapper away at any time and nothing else would
// notice: every other guard here asks what a control holds, not how wide it is.
func TestTheFormDoesNotRunToTheEdgeOfTheWindow(t *testing.T) {
	for _, screenName := range []string{"generate", "preset"} {
		host := newFakeHost(t)
		window.Open(host)
		if screenName == "preset" {
			selectTab(t, host.content, text.TabPresets())
		}
		content := host.content

		w := test.NewWindow(content)
		// Far wider than any window a person opens, which is the point: the
		// defect only shows when there is space to stretch into.
		w.Resize(fyne.NewSize(3862, 1200))
		content.Refresh()

		widest := float32(0)
		var worst fyne.CanvasObject
		walk(content, func(obj fyne.CanvasObject) {
			switch obj.(type) {
			case *parts.Entry, *parts.Chooser, *parts.Toggle:
			default:
				return
			}
			if size := obj.Size().Width; size > widest {
				widest, worst = size, obj
			}
		})
		w.Close()

		if widest == 0 {
			t.Fatalf("%s: no control was measured, so this guard read the wrong tree", screenName)
		}
		if widest > parts.ColumnWidth {
			t.Errorf("%s: a %T is %.0f px wide in a 3862 px window, over the %d px this form allows.\n"+
				"A row that long cannot be followed from its label to its value.",
				screenName, worst, widest, parts.ColumnWidth)
		}
		t.Logf("%s: widest control %.0f px in a 3862 px window, ceiling %d", screenName, widest, parts.ColumnWidth)
	}
}

// A box holding a number is as wide as a number, not as wide as the column.
//
// The other half of the stretching defect, and the half the guard above cannot
// see: capping the column stopped a field at 820 px, which is still a box the
// width of a paragraph offered for the digit 0. Measured on 2026-08-12 before
// this was fixed - the seed was 397 px and the count 399, because a column
// split in two hands each half to whatever stands in it.
//
// It asks about the three fields by name rather than about every box on the
// screen, because that is the actual rule. A path and a name template take the
// column on purpose: nobody can predict how long those are, and a short box for
// a long path is the same defect pointing the other way.
func TestABoxForANumberIsTheWidthOfANumber(t *testing.T) {
	for _, screenName := range []string{"generate", "preset"} {
		host := newFakeHost(t)
		window.Open(host)

		// Through the tab rather than across the window. Both work screens
		// carry a field called seed, and a lookup over the whole window
		// answers with the last one it walked past without saying so.
		content := tabNamed(t, host.content, text.TabOneTarget())
		if screenName == "preset" {
			content = selectTab(t, host.content, text.TabPresets())
		}

		w := test.NewWindow(content)
		w.Resize(fyne.NewSize(1000, 760))
		content.Refresh()

		numeric := []string{text.FieldSeed()}
		if screenName == "generate" {
			numeric = []string{text.FieldSize(), text.FieldCount(), text.FieldSeed()}
		}
		for _, label := range numeric {
			box := entryUnder(t, content, label)
			if got := box.Size().Width; got > parts.NumericWidth {
				t.Errorf("%s: the box for %q is %.0f px wide, over the %d a number needs.\n"+
					"A box that size promises a value the field does not take.",
					screenName, label, got, parts.NumericWidth)
			}
		}
		w.Close()
	}
}

// What the run says about itself lines up with the form it is talking about.
//
// The action bar draws its surface across the whole window on purpose - that is
// what makes it read as a bar - and until 2026-08-12 everything standing on it
// did the same. Measured then: the form stopped at 822 px and a refusal about a
// field in it ran to 1099, so the longest line on the screen was the one nobody
// wants to have to read.
//
// Only the contents are asked about here. A guard that also demanded the
// surface stop at the column would be pinning down the one thing that is
// deliberately different, and the next person to widen the bar would delete it.
// A button standing in a row of fields is the size of a button.
//
// Reported by the owner on 2026-08-28 from the running window, and the numbers
// are off the laid out screen rather than off the code: the Remove button
// ending a row of an archive's contents was 197.50 x 63.16 px for a word the
// toolkit says needs 67.92 x 32. That is a quarter of the form wide and as tall
// as a label and a control together, so it read as a grey panel with a word in
// the middle of it. The Duplicate button at the head of the same batch, which
// stands in no row, is 78.97 x 35.16.
//
// The cause is that parts.Row shares the width out in equal columns, which is
// what a field wants and what anything else gets whether it wants it or not.
// See parts.BesideFields.
//
// Asked against MinSize, which is the widget's own answer for the room its word
// needs, so nothing here repeats a layout's arithmetic. Both directions, because
// a button smaller than its own minimum is a word cut in half.
func TestAButtonInARowOfFieldsIsTheSizeOfAButton(t *testing.T) {
	ourTheme(t)
	content, _ := laidOutWindow(t)
	screen := selectTab(t, content, text.TabRecipe())

	// An archive first. The only row in this window that ends in a button is
	// the one saying what an archive holds, and it is not on the screen until a
	// batch says it holds anything.
	chooseFormat(t, screen, "zip")
	pressNamed(t, screen, text.ButtonAddContents())

	remove := buttonNamed(screen, text.ButtonRemoveContents())
	if remove == nil {
		t.Fatalf("no %q button after asking a zip what it holds, so this guard checked nothing",
			text.ButtonRemoveContents())
	}
	got, needs := remove.Size(), remove.MinSize()
	if got.Width == 0 || got.Height == 0 {
		t.Fatal("the button was never laid out, so its size says nothing")
	}
	const slack = 0.5
	if got.Width > needs.Width+slack || got.Height > needs.Height+slack {
		t.Errorf("the %q button in a row of an archive's contents is %.2f x %.2f px and the word"+
			" in it needs %.2f x %.2f, so the row is drawing it as a panel rather than as a"+
			" button.\n"+
			"What to do: parts.BesideFields keeps something that is not a field out of the"+
			" column arithmetic.",
			remove.Text, got.Width, got.Height, needs.Width, needs.Height)
	}
	if got.Width+slack < needs.Width || got.Height+slack < needs.Height {
		t.Errorf("the %q button is %.2f x %.2f px and needs %.2f x %.2f, so its word is cut off.",
			remove.Text, got.Width, got.Height, needs.Width, needs.Height)
	}
	t.Logf("the %q button is %.2f x %.2f, and it needs %.2f x %.2f",
		remove.Text, got.Width, got.Height, needs.Width, needs.Height)
}

func TestTheRunSpeaksInsideTheSameColumnAsTheForm(t *testing.T) {
	host := newFakeHost(t)
	window.Open(host)
	content := tabNamed(t, host.content, text.TabOneTarget())

	w := test.NewWindow(content)
	// Wide, because the defect is stretching and a narrow window hides it.
	w.Resize(fyne.NewSize(1600, 900))
	content.Refresh()

	// A run that SUCCEEDS, and that is a correction made on 2026-08-12 after a
	// mutation run rather than a preference. This used to ask for one byte and
	// measure the refusal, which stopped landing here the day refusals moved
	// under the fields they are about - so the bar had nothing on it, every
	// label the walk found belonged to the form, and removing the width cap
	// from the bar changed nothing this guard could see. It stayed green
	// through its own mutation.
	//
	// The preview line is the longest thing the bar ever holds: how many, what
	// kind, how big, and how much room is left, with a path on the end of it.
	entryUnder(t, content, text.FieldSize()).SetText("1mb")
	entryUnder(t, content, text.FieldOutputDir()).SetText(t.TempDir())
	preview := buttonNamed(content, "Preview")
	if preview == nil {
		t.Fatal("the generate screen has no Preview button, so this guard read the wrong tree")
	}
	preview.OnTapped()
	// The answer comes from a worker now, so it has to be here before the tree
	// is read. See join.
	join(host)
	content.Refresh()

	spoke := false
	walk(content, func(obj fyne.CanvasObject) {
		label, ok := obj.(*widget.Label)
		if !ok || label.Text == "" || !label.Visible() {
			return
		}
		// The line the run wrote, told from the form's own labels by what it
		// says. Counting every visible label instead is what let this guard
		// pass while the bar was empty.
		if strings.Contains(label.Text, text.AndNothingWrittenYet()) {
			spoke = true
		}
		if got := label.Size().Width; got > parts.ColumnWidth {
			t.Errorf("a line on the action bar is %.0f px wide in a 1600 px window, over the %d the form uses.\n"+
				"Text: %q", got, parts.ColumnWidth, label.Text)
		}
	})
	w.Close()

	if !spoke {
		t.Fatal("the run said nothing about itself, so this guard measured the form and not the bar")
	}
}

// A switch stands under its name in the column of names.
//
// This is O72 turned the other way up, on 2026-09-15. The switch used to carry
// its own words, because a heading ABOVE it left a bare square with nothing to
// read - but in a grid the name stands BESIDE the square, in the column every
// other name is in, so the words to read are there without the switch being
// the one control that breaks the grid (GUI rule 13). What the two guards here
// used to protect - that a switch is not a nameless square, and that its words
// are clear of it - is now that the switch is a field found by the name beside
// it, like every other. TestTheWordsOfASwitchStandClearOfItsSquare (O95) went
// with the words: there are none to stand clear of.
//
// Found through checkNamed, which is controlUnder - so this passes only while
// the name is a real heading in the column, level with the square.
func TestTheLabelSwitchStandsUnderItsName(t *testing.T) {
	_, content := screen(t)

	if checkNamed(content, text.FieldLabel()) == nil {
		t.Errorf("no switch stands under the name %q in the column of names, so either the switch "+
			"carries its own words again or its name is not beside it", text.FieldLabel())
	}
}
