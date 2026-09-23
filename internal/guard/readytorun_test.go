package guard

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"

	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// Generate means the same thing on every screen it is on.
//
// It did not until 2026-09-23. The single batch screen opened with a name and
// a size typed in and ran on the first press. The batch screen opened with
// both of them empty under a red star each, so the same button under the same
// mark turned somebody down on the third tab after working on the first - and
// what they had learned on the first screen was that Generate simply works.

// TestEveryWorkScreenOpensReadyToRun presses the button on a screen nobody has
// typed into, and asks the DISK whether anything came of it.
//
// Both screens, from one list, so the two cannot come apart again: a screen
// added tomorrow is held to the same promise by being added to it. The output
// directory is the one thing filled in, because a guard writing into whatever
// the window offers would write into the working directory of whoever runs the
// suite.
//
// Asked of the files rather than of the refusal area, and the difference
// matters: a screen can be silent and still have done nothing. The manifest on
// the disk is the only answer that cannot be arranged by the screen.
func TestEveryWorkScreenOpensReadyToRun(t *testing.T) {
	for _, tab := range []string{text.TabOneTarget(), text.TabRecipe()} {
		t.Run(tab, func(t *testing.T) {
			dir := t.TempDir()
			host, content, _ := keyedWindow(t)
			screen := selectTab(t, content, tab)

			entryUnder(t, screen, text.FieldOutputDir()).SetText(dir)
			press(t, screen, text.ButtonGenerate())
			// Waiting first, so that a refusal reported below is the screen's
			// answer rather than a run still going.
			join(host)

			if refusal := anyRefusal(screen); refusal != "" {
				t.Fatalf("%s opens on a form Generate refuses: %q.\n"+
					"Somebody who has typed nothing has no way to know which box it means", tab, refusal)
			}
			manifestAfter(t, func() { join(host) }, dir, "manifest.json")
		})
	}
}

// TestOnlyTheFirstBatchArrivesFilledIn is the other half, and without it the
// guard above would be satisfied by filling every batch in.
//
// A batch is filled in because a form nobody has typed into has to be able to
// run. The SECOND one is a different question: two batches carrying one name
// is a refusal the recipe reader already words, so a copy arriving with the
// first one's name would be a form that has to be repaired before it can be
// used - which is why duplicating a batch leaves the copy's name empty too.
func TestOnlyTheFirstBatchArrivesFilledIn(t *testing.T) {
	batches, _, _ := screenInAWindowWithHost(t, text.TabRecipe())

	first := entryUnder(t, batches, text.FieldTargetID())
	if strings.TrimSpace(first.Text) == "" {
		t.Fatal("the first batch opens with no name, so this guard is asking about a screen " +
			"that never filled one in")
	}
	opening := first.Text

	press(t, batches, text.ButtonAddBatch())

	names := entriesUnder(batches, text.FieldTargetID())
	if len(names) != 2 {
		t.Fatalf("after adding a batch the screen draws %d boxes called %q", len(names), text.FieldTargetID())
	}
	if got := strings.TrimSpace(names[1].Text); got != "" {
		t.Errorf("a batch added to the screen arrives named %q, and the batch above it is named %q - "+
			"two batches with one name is a refusal, and this hands it to somebody who typed nothing",
			got, opening)
	}
}

// entriesUnder is every box on a screen drawn under one name, in the order the
// screen draws them.
//
// entryUnder answers with ONE and is the right shape for a screen holding one
// of each. This screen repeats a whole block, so "the box called Batch name"
// is a question with as many answers as there are batches - and the defect
// this guard is about lives in the second answer.
func entriesUnder(o fyne.CanvasObject, label string) []*parts.Entry {
	var found []*parts.Entry
	walk(o, func(obj fyne.CanvasObject) {
		field, ok := obj.(*fyne.Container)
		if !ok || !parts.IsField(field) || len(field.Objects) < 2 {
			return
		}
		if head, named := headingOf(field.Objects[0]); !named || head != label {
			return
		}
		var box *parts.Entry
		walk(field.Objects[1], func(inner fyne.CanvasObject) {
			if e, is := inner.(*parts.Entry); is && box == nil {
				box = e
			}
		})
		if box != nil {
			found = append(found, box)
		}
	})
	return found
}
