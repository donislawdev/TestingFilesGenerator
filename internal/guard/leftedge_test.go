package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// ourTheme installs the look this program ships before a screen is laid out.
//
// Needed rather than tidy, and found by this guard being 2 px out. A test
// canvas runs on the toolkit's own theme, which pads by 4 where ours pads by
// 6 - so a layout measured without this is a layout nobody has, which is the
// same sentence parts.Theme carries about pictures. See observation O121.
func ourTheme(t *testing.T) {
	t.Helper()
	app := test.NewApp()
	app.Settings().SetTheme(parts.Theme())
	t.Cleanup(func() { test.NewApp() })
}

// Everything a person reads on a screen starts on one left edge.
//
// Measured off a render on 2026-08-20, before this held. Four ranks of text,
// three different edges:
//
//	card border                       89
//	screen title "Generate files"     97
//	status line under the buttons     97
//	section title, field name        103
//
// Six pixels is the worst of the three distances a title can stand at. Aligned
// reads as deliberate and a real indent reads as deliberate. Six pixels reads
// as nobody having looked, and alignment is what separates a screen that was
// designed from one that was assembled.
//
// The cause was two of them, both invisible from either side alone. A panel
// carries its padding inside itself, so its fields start 6 px in from its
// edge, while the screen title sits on the column at the panel's edge. The
// action bar looked like it had the same padding and did not: the padding was
// applied around the bar and the column inside it is centred in what was left,
// which puts the column back where the panel's edge is.
//
// It asks for the words rather than the boxes. Type carries its own bearing
// and a heading at 17 px carries more of it than a name at 14, which is where
// the one pixel of slack below goes - measured at 103 against 104 for the same
// left edge. That pixel is the type and not the layout, and a guard demanding
// zero would be a guard against the font.
func TestEverythingAPersonReadsStartsOnOneLeftEdge(t *testing.T) {
	ourTheme(t)
	content, _ := laidOutWindow(t)

	screens := []struct {
		tab   string
		words []string
	}{
		// The deepest rank used to be the line under a field. That line moved
		// behind the button beside the field's name on 2026-08-24, so what is
		// left standing on this edge is the heading a format's settings get.
		// Four ranks either way, which is what this is about.
		{text.TabOneTarget, []string{
			text.HeadingGenerate,
			text.SectionConfiguration,
			text.FieldFormat,
			text.SettingsFor("bmp"),
		}},
		// The preset screen names its section and its first field with the
		// same word, so only one of the two can be found by its text. Asked
		// about the section, which is the rank the heading above it has to
		// line up with.
		{text.TabPresets, []string{
			text.HeadingPreset,
			text.SectionPreset,
			text.PresetCatchesHeading,
		}},
		{text.TabRecipe, []string{
			text.HeadingRecipe,
			text.FieldFormat,
		}},
	}

	for _, screen := range screens {
		t.Run(screen.tab, func(t *testing.T) {
			laid := tabContent(t, content, screen.tab)

			var edge float32
			var edgeOf string
			for _, words := range screen.words {
				box, ok := labelBox(laid, words)
				if !ok {
					t.Fatalf("no label reading %q on this screen", words)
				}
				if edgeOf == "" {
					edge, edgeOf = box.X, words
					continue
				}
				if off := box.X - edge; off > 1 || off < -1 {
					t.Errorf("%q starts at %.1f px and %q at %.1f px, %.1f px apart."+
						" Text a person reads has to start on one edge or the screen reads as assembled rather than laid out",
						edgeOf, edge, words, box.X, off)
				}
			}
		})
	}
}

// The bar's own words stand on the same edge as the form's.
//
// Named apart from the screens above because it is the one that was wrong for
// a reason nobody could see: the action bar does have padding, and the column
// inside it is centred in what the padding left - so the padding moved the
// bar's edge and not its content. A guard reading the code would have found
// the padding and stopped there.
func TestTheActionBarSpeaksOnTheSameEdgeAsTheForm(t *testing.T) {
	ourTheme(t)
	content, _ := laidOutWindow(t)
	generate := tabContent(t, content, text.TabOneTarget)

	field, ok := labelBox(generate, text.FieldFormat)
	if !ok {
		t.Fatal("the generate screen has no Format field")
	}
	// What the bar says at rest, before anything has been pressed.
	status, ok := labelBox(generate, text.WritingTo(destinationShownAtRest(t, generate)))
	if !ok {
		t.Skip("the bar is not naming a destination at rest, so there is nothing on it to line up")
	}
	if off := status.X - field.X; off > 1 || off < -1 {
		t.Errorf("a field name starts at %.1f px and the bar's own line at %.1f px, %.1f px apart",
			field.X, status.X, off)
	}
}

// destinationShownAtRest is the folder the bar is naming, read off the box
// that holds it rather than worked out again here.
func destinationShownAtRest(t *testing.T, screen fyne.CanvasObject) string {
	t.Helper()
	box := entryUnder(t, screen, text.FieldOutputDir)
	if box == nil {
		t.Fatal("the screen has no output directory box")
	}
	return box.Text
}
