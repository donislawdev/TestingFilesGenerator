package guard

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// What this defends. A title too long for its room ends in an ellipsis at the
// room's edge, and a title that fits is drawn whole.
//
// Why it needed a guard. A title and a block's name were canvas words until
// 2026-09-16 - one line however much room it had - and the catalogue's long
// line state showed a title running 60 px past its panel (O214). No screen
// shows one that long today, so the defect lived only where somebody looks
// least, and a translation would have brought it to every screen at once.
//
// How it is asked. Off the DRAWN text rather than off the label's field: a
// label keeps its whole text and the toolkit draws what fits, so the field
// says nothing about what a person sees. The drawn text is the canvas text
// under the label's own renderer. Both halves are asked, because a widget that
// truncated everything would pass the first alone.
func TestATitleTooLongForItsRoomEndsInAnEllipsisInsideIt(t *testing.T) {
	ourTheme(t)
	long := "Write a label inside each generated file, including the ones that are far too small to hold it"
	room := float32(300)

	for _, tc := range []struct {
		name string
		make func(string) fyne.CanvasObject
	}{
		{"a title", parts.Title},
		{"a block's name", parts.Subheading},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cut := tc.make(long)
			w := test.NewWindow(cut)
			t.Cleanup(w.Close)
			w.Resize(fyne.NewSize(room, 60))
			label := labelIn(t, cut)
			drawn, width := drawnText(label)
			if drawn == long || !strings.HasSuffix(drawn, "…") {
				t.Errorf("%s of %d characters in %.0f px is drawn as %q, and a line short of room ends in an"+
					" ellipsis rather than running past its panel", tc.name, len(long), room, drawn)
			}
			if width > label.Size().Width {
				t.Errorf("%s is drawn %.2f px wide in a room of %.2f, so it runs past its edge",
					tc.name, width, label.Size().Width)
			}

			// And one that fits is whole - the control that cut everything
			// would have passed the half above.
			short := tc.make("Single batch")
			w2 := test.NewWindow(short)
			t.Cleanup(w2.Close)
			w2.Resize(fyne.NewSize(room, 60))
			if got, _ := drawnText(labelIn(t, short)); got != "Single batch" {
				t.Errorf("%s that fits its room is drawn as %q rather than whole", tc.name, got)
			}
		})
	}
}

// And on the screens we ship, nothing is ever cut. Every title and every
// block's name, on every screen, under every format, fits the room the
// layout gives it at the largest first opening - so the ellipsis above is the
// state of the control for a window somebody made narrow, never the way a
// word of ours reaches a person.
//
// This is the guard on the dictionary, asked where the words stand rather
// than against a length typed into a test: the room a title gets is a fact of
// the layout and the width a word needs is a fact of the typeface, and both
// change without anybody editing this file. The formats are walked because
// one block's name carries a format id - "Settings for avif" - and the
// catalogue screen is left out because its long line state exists to show
// the cut.
func TestEveryTitleAndBlockNameFitsWhereItStands(t *testing.T) {
	ourTheme(t)
	content, cv := laidOutWindow(t)

	checked := 0
	measure := func(tab, under string, screen fyne.CanvasObject) {
		found, refused := headingsOn(screen)
		for _, r := range refused {
			t.Errorf("on the %s screen%s, %s", tab, under, r)
		}
		for _, h := range found {
			if h.need > h.room {
				t.Errorf("on the %s screen%s, %q needs %.2f px and has %.2f, so it is drawn cut - a word"+
					" of ours never reaches a person with an ellipsis in it or past the edge of its panel",
					tab, under, h.words, h.need, h.room)
			}
			checked++
		}
	}
	for _, tab := range allTabs() {
		screen := selectTab(t, content, tab)
		cv.Content().Resize(cv.Size())
		measure(tab, "", screen)
		if tab == text.TabOneTarget() {
			for _, id := range format.IDs() {
				chooseFormat(t, screen, id)
				cv.Content().Resize(cv.Size())
				measure(tab, " under "+id, screen)
			}
		}
		if tab == text.TabRecipe() {
			// The table of files inside an archive has a block name of its own,
			// and it is on the screen only once a batch holds files.
			menusWithAnArchiveOpened(t, screen, tab, cv)
			measure(tab, " with an archive opened", screen)
		}
	}
	if checked < 4 {
		t.Fatalf("only %d titles and block names were laid out, and there are four screens", checked)
	}
	t.Logf("%d titles, section and block names measured at %.0f px wide, every one whole", checked, window.LargestOpening.Width)
}

// heading is one title, section name or block name a person can see: what it
// says, the width its words need and the room the layout gave it.
type heading struct {
	words      string
	need, room float32
}

// headingsOn is every heading a person can see on a screen, and a refusal for
// every one the layout gave no room - see widthShown for why that is a
// refusal and not a skip. Every label that ends in an ellipsis is a heading,
// and so is every bold canvas text at the heading size: a section's name is
// still canvas words, where an ellipsis could never be asked for - so it is
// measured here rather than left to run past a panel unseen.
//
// The name of a FOLDED section is measured as its whole head row - title,
// arrow and line against the width the row was given - and not as its own
// words. The row hands each of them the width of its own text (an HBox lays
// every child out at its MinSize width, layout/boxlayout.go), so the title
// measured alone is whole by construction and this guard was green with the
// arrow drawn 6 px outside the row it marks, in the catalogue's long title
// state, until 2026-09-17 - an outside review of #110 counted the pixels.
// Every head row on the screen has to be found, and a head the walk could
// not pair with its words is a refusal rather than a row stepped over: the
// pairing is the row's shape, and a guard that assumed the shape would be
// green the day it changed (O118).
func headingsOn(screen fyne.CanvasObject) (found []heading, refused []string) {
	buried := underSomethingHidden(screen)
	inRow := insideAHeadRow(screen)
	heads, rows := 0, 0
	walk(screen, func(o fyne.CanvasObject) {
		var words string
		var need float32
		switch v := o.(type) {
		case *widget.Label:
			if v.Truncation != fyne.TextTruncateEllipsis {
				return
			}
			size := fyne.CurrentApp().Settings().Theme().Size(sizeNameOf(v))
			words, need = v.Text, fyne.MeasureText(v.Text, size, v.TextStyle).Width
		case *canvas.Text:
			if !v.TextStyle.Bold || v.TextSize != parts.TextHeading || inRow[v] {
				return
			}
			words, need = v.Text, v.MinSize().Width
		case *parts.FoldHead:
			heads++
			return
		case *fyne.Container:
			head, row, ok := foldHeadRow(v)
			if !ok {
				return
			}
			rows++
			words, need, o = fmt.Sprintf("the head row of %q", head.Title()), row.MinSize().Width, row
		default:
			return
		}
		room, shown, err := widthShown(o, buried)
		if err != nil {
			refused = append(refused, fmt.Sprintf("%q %v", words, err))
			return
		}
		if shown {
			found = append(found, heading{words, need, room})
		}
	})
	if heads != rows {
		refused = append(refused, fmt.Sprintf("%d head row(s) of folds stand on the screen and %d were paired with"+
			" their words, so a fold's title is being measured on its own text again", heads, rows))
	}
	return found, refused
}

// foldHeadRow is the pairing a fold's head is built as: the head control and
// the words it lies under, the two children of one stack (folding.go). The
// words are whichever child is not the head, so the order of the two is not
// something this guard assumes.
func foldHeadRow(c *fyne.Container) (head *parts.FoldHead, row fyne.CanvasObject, ok bool) {
	if len(c.Objects) != 2 {
		return nil, nil, false
	}
	for i, child := range c.Objects {
		if h, isHead := child.(*parts.FoldHead); isHead {
			return h, c.Objects[1-i], true
		}
	}
	return nil, nil, false
}

// insideAHeadRow is every object standing in the words of a fold's head row,
// so that the title's own canvas text is not measured a second time on its
// own width.
func insideAHeadRow(screen fyne.CanvasObject) map[fyne.CanvasObject]bool {
	inside := map[fyne.CanvasObject]bool{}
	walk(screen, func(o fyne.CanvasObject) {
		c, isContainer := o.(*fyne.Container)
		if !isContainer {
			return
		}
		if _, row, ok := foldHeadRow(c); ok {
			walk(row, func(within fyne.CanvasObject) { inside[within] = true })
		}
	})
	return inside
}

// widthShown is the width the layout gave a control a person can see, and
// whether it is one.
//
// Under something hidden a control is not shown whatever its size - Visible()
// answers for one object and not for its ancestors, so this asks through
// underSomethingHidden. Shown and given no width, a control is a refusal
// rather than a skip: it is drawn as nothing, and a guard that stepped over it
// would pass on the strength of the controls beside it. Three guards stepped
// over zero until 2026-09-16, each for the hidden case, and measured that day
// nothing on any screen reached the skip - so it was dead for the state it
// was written for and live for the collapse it would have hidden. Hidden is
// asked by hiddenness now, and zero is what it is.
func widthShown(o fyne.CanvasObject, buried map[fyne.CanvasObject]bool) (width float32, shown bool, err error) {
	if buried[o] {
		return 0, false, nil
	}
	if width = o.Size().Width; width == 0 {
		return 0, true, errors.New("was laid out to nothing, so it is drawn as nothing - a collapse no skip may pass")
	}
	return width, true, nil
}

// widthShown's red path and its two green ones: a heading a person can see
// that the layout gave nothing is refused, one under something hidden is not
// measured whatever its size, and one laid out is measured - asked through
// headingsOn, because that is where the answer is used. The third half is the
// control: a refusal that refused everything would pass the first two.
func TestAHeadingLaidOutToNothingIsRefusedAndAHiddenOneIsNotMeasured(t *testing.T) {
	ourTheme(t)
	nothing := container.NewWithoutLayout(parts.Title("Single batch"))
	if found, refused := headingsOn(nothing); len(refused) != 1 || len(found) != 0 {
		t.Errorf("a title the layout gave no room is measured as %v and refused as %v - it is drawn"+
			" as nothing, which is a cut, not a state to step over", found, refused)
	}

	hidden := container.NewWithoutLayout(parts.Title("Single batch"))
	hidden.Hide()
	if found, refused := headingsOn(hidden); len(refused) != 0 || len(found) != 0 {
		t.Errorf("a title under something hidden is measured as %v and refused as %v - nobody sees"+
			" it, so the room it has is not a room", found, refused)
	}

	shown := parts.Title("Single batch")
	w := test.NewWindow(shown)
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(300, 60))
	if found, refused := headingsOn(shown); len(refused) != 0 || len(found) != 1 || found[0].room == 0 {
		t.Errorf("a title laid out in a window is measured as %v and refused as %v", found, refused)
	}
}

// A fold's head row is measured as a whole against the width it was given,
// and its title is not measured a second time on its own. Asked of a fold
// too narrow for its title - the catalogue's long title, in the room the
// catalogue gives it - and of one with room, so a measure that refused every
// row would pass the first half alone. The narrow fold is laid out below its
// MinSize on purpose: a real window refuses that (the driver sets the
// window's minimum from the content, window_desktop.go fitContent), so this
// is the state only the catalogue reaches, and the one the arrow was drawn
// outside the row in.
func TestAFoldsHeadRowIsMeasuredAsAWholeAgainstItsWidth(t *testing.T) {
	ourTheme(t)
	long := "Write a label inside each generated file, including the ones that are far too small to hold it"
	for _, tc := range []struct {
		name  string
		title string
		room  float32
		cut   bool
	}{
		{"a title wider than its row", long, 300, true},
		{"a title with room", "Single batch", 600, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fold := parts.NewFolding(tc.title, nil, parts.Prose("inside"))
			w := test.NewWindow(fold.Object())
			t.Cleanup(w.Close)
			w.Resize(fyne.NewSize(tc.room, 120))
			found, refused := headingsOn(fold.Object())
			if len(refused) != 0 {
				t.Fatalf("a fold laid out in a window is refused: %v", refused)
			}
			if len(found) != 1 {
				t.Fatalf("a fold with one title is measured as %d heading(s): %v - the row once, and the title"+
					" not again on its own", len(found), found)
			}
			row := found[0]
			if !strings.Contains(row.words, tc.title) {
				t.Errorf("the one heading measured is %q, and the fold's title is %q", row.words, tc.title)
			}
			if cut := row.need > row.room; cut != tc.cut {
				t.Errorf("%s needs %.2f px in %.2f: measured as cut %v, and it is cut %v", tc.name, row.need, row.room, cut, tc.cut)
			}
		})
	}

	// And a head the walk finds with no words beside it is a refusal, not a
	// row stepped over - the assertion that every fold on a screen was
	// measured, for the day the row is built another way.
	t.Run("a head without its row", func(t *testing.T) {
		fold := parts.NewFolding("Single batch", nil, parts.Prose("inside"))
		alone := container.NewWithoutLayout(fold.Head())
		if found, refused := headingsOn(alone); len(refused) != 1 || len(found) != 0 {
			t.Errorf("a head row with no words is measured as %v and refused as %v - a fold this guard"+
				" cannot pair is a fold it is not measuring", found, refused)
		}
	})
}

// labelIn is the one toolkit label under an object - a title is a label wrapped
// in the theme that takes its inner padding away.
func labelIn(t *testing.T, o fyne.CanvasObject) *widget.Label {
	t.Helper()
	var found *widget.Label
	walk(o, func(obj fyne.CanvasObject) {
		if l, ok := obj.(*widget.Label); ok && found == nil {
			found = l
		}
	})
	if found == nil {
		t.Fatal("no label under this object, so there is no drawn text to read")
	}
	return found
}

// drawnText is what a label actually draws: the canvas text under its
// renderer, joined, with the width it takes. The label's Text field is what it
// was GIVEN, which is a different thing once the toolkit has cut it.
func drawnText(label *widget.Label) (string, float32) {
	var parts []string
	var width float32
	var rec func(o fyne.CanvasObject)
	rec = func(o fyne.CanvasObject) {
		switch v := o.(type) {
		case *canvas.Text:
			parts = append(parts, v.Text)
			width += v.MinSize().Width
		case fyne.Widget:
			for _, c := range test.WidgetRenderer(v).Objects() {
				rec(c)
			}
		case *fyne.Container:
			for _, c := range v.Objects {
				rec(c)
			}
		}
	}
	rec(label)
	return strings.Join(parts, ""), width
}

// sizeNameOf is the theme size a label draws at: the one it names, or the
// body text when it names none.
func sizeNameOf(label *widget.Label) fyne.ThemeSizeName {
	if label.SizeName == "" {
		return "text"
	}
	return label.SizeName
}
