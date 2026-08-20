package guard

import (
	"testing"

	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// Every registered format is sorted into a kind.
//
// The menu draws a picture in front of each format so that somebody looking for
// "a picture of some sort" does not have to recognise twenty three-letter
// abbreviations in one alphabetical run. That classification is a table kept by
// hand, and a table kept by hand is a table that goes stale in silence - O80
// names two of those in this package already, and a format registered without a
// kind would simply draw nothing while everything around it drew something.
//
// So it is compared against the registry rather than against a list written
// here. The twenty-first format joins this table on the day it is registered or
// the suite goes red.
func TestEveryRegisteredFormatHasAKind(t *testing.T) {
	known := parts.FileKinds()

	for _, d := range format.All() {
		if !known[d.ID] {
			t.Errorf("%s is registered and has no kind, so it draws no picture in a menu where every "+
				"other format draws one. Add it to fileKinds in internal/gui/parts/filekind.go", d.ID)
		}
		if parts.KindOfFile(d.ID) == nil {
			t.Errorf("%s has a kind and no picture for it", d.ID)
		}
	}
	for id := range known {
		if _, err := format.Get(id); err != nil {
			t.Errorf("%q has a kind and is not a registered format, so the table describes something "+
				"that is not there", id)
		}
	}
	t.Logf("%d format(s), each sorted into a kind", len(known))
}

// The pictures are pictures of different things.
//
// The toolkit's own file icons were tried first and are useless here: at the
// size a row draws them, FileImageIcon, FileTextIcon and DocumentIcon are the
// same sheet of paper with different creases, so six rows carried one icon.
// Worse than no icon, because it looks like information and is not.
//
// Compared by resource rather than by eye, which is the part a machine can do:
// two kinds that resolve to one picture say nothing that the alphabetical order
// was not already saying.
func TestTwoKindsOfFileAreNotDrawnWithOnePicture(t *testing.T) {
	seen := map[string]string{}
	for _, d := range format.All() {
		icon := parts.KindOfFile(d.ID)
		if icon == nil {
			continue
		}
		// Formats of one kind share a picture on purpose, so this remembers
		// the first format that used it and only complains when a SECOND kind
		// arrives at the same one.
		if first, taken := seen[icon.Name()]; taken {
			if parts.SameKind(first, d.ID) {
				continue
			}
			t.Errorf("%s and %s are different kinds of file and draw the same picture (%s)",
				first, d.ID, icon.Name())
			continue
		}
		seen[icon.Name()] = d.ID
	}
	if len(seen) < 2 {
		t.Fatalf("every format draws one picture, so the menu is telling nobody anything")
	}
	t.Logf("%d different pictures across the formats", len(seen))
}

// And the picture reaches the row a person looks at.
//
// The table and the row are two halves. A classification nothing draws is a
// table, and a row that draws whatever it is handed proves nothing about the
// table - this asks the list that opens under the menu what it put in front of
// its words.
func TestTheFormatMenuDrawsThePictureOfEachKind(t *testing.T) {
	content, w := screenInAWindow(t, text.TabOneTarget)

	menu, ok := controlUnder(content, text.FieldFormat).(*parts.Chooser)
	if !ok {
		t.Fatal("the format field is not a list to choose from, so this guard read the wrong tree")
	}
	menu.Tapped(&fyne.PointEvent{})
	if !listIsOpenOn(w.Canvas()) {
		t.Fatal("tapping the format menu put nothing on the canvas, so nothing opened")
	}

	list := menu.Opened()
	if list == nil {
		t.Fatal("the format menu never dropped a list down")
	}
	rows := list.DrawnRows()
	if len(rows) == 0 {
		t.Fatal("the list that dropped down is drawing no rows")
	}
	for _, row := range rows {
		if row.Kind() == nil {
			t.Errorf("the row for %q draws no picture, so the kinds stop at the table", row.Label())
		}
	}
}
