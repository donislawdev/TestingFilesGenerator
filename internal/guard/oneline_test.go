package guard

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
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
		walk(screen, func(o fyne.CanvasObject) {
			var words string
			var need, room float32
			switch v := o.(type) {
			case *widget.Label:
				if v.Truncation != fyne.TextTruncateEllipsis {
					return
				}
				size := fyne.CurrentApp().Settings().Theme().Size(sizeNameOf(v))
				words, need, room = v.Text, fyne.MeasureText(v.Text, size, v.TextStyle).Width, v.Size().Width
			case *canvas.Text:
				// A section's name is still canvas words - it also heads a
				// folded section, in a row that hands it the width of its own
				// text, where an ellipsis could never be asked for - so it is
				// measured here rather than left to run past a panel unseen.
				if !v.TextStyle.Bold || v.TextSize != parts.TextHeading {
					return
				}
				words, need, room = v.Text, v.MinSize().Width, v.Size().Width
			default:
				return
			}
			if room == 0 {
				return
			}
			if need > room {
				t.Errorf("on the %s screen%s, %q needs %.2f px and has %.2f, so it is drawn cut - a word"+
					" of ours never reaches a person with an ellipsis in it or past the edge of its panel",
					tab, under, words, need, room)
			}
			checked++
		})
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
