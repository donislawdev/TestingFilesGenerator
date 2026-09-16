package guard

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// The window is set in Inter, and this asks the painter rather than the
// theme: a face the theme names and the painter never shapes with is a font
// that ships and is not seen. The two faces give a sentence two different
// widths, so the width is the measurement, and a theme that fell back to the
// toolkit's Noto Sans for either weight would give the toolkit's width.
func TestTheWindowIsSetInInter(t *testing.T) {
	const sentence = "Files of one format and one size, as many as you need."
	size := parts.Theme().Size(theme.SizeNameText)

	app := test.NewApp()
	defer test.NewApp()
	app.Settings().SetTheme(theme.DefaultTheme())
	toolkit := fyne.MeasureText(sentence, size, fyne.TextStyle{})
	toolkitBold := fyne.MeasureText(sentence, size, fyne.TextStyle{Bold: true})

	app.Settings().SetTheme(parts.Theme())
	ours := fyne.MeasureText(sentence, size, fyne.TextStyle{})
	oursBold := fyne.MeasureText(sentence, size, fyne.TextStyle{Bold: true})

	if ours.Width == toolkit.Width {
		t.Errorf("a sentence is %.1f px wide under our theme and %.1f under the toolkit's, so the "+
			"painter is shaping the regular weight with the toolkit's face rather than Inter", ours.Width, toolkit.Width)
	}
	if oursBold.Width == toolkitBold.Width {
		t.Errorf("a bold sentence is %.1f px wide under our theme and %.1f under the toolkit's, so the "+
			"painter is shaping the bold weight with the toolkit's face rather than Inter", oursBold.Width, toolkitBold.Width)
	}
	if ours.Width == oursBold.Width {
		t.Errorf("regular and bold measure the same %.1f px, so one weight is standing in for both", ours.Width)
	}
	t.Logf("regular %.1f px (toolkit %.1f), bold %.1f px (toolkit %.1f)",
		ours.Width, toolkit.Width, oursBold.Width, toolkitBold.Width)
}

// The styles the window never draws in stay the toolkit's. Not a preference:
// Inter ships here in two weights and no italic, so an italic answered with
// the upright would be a style silently drawn as another, and a monospace
// answered with a proportional face would misalign the one thing monospace is
// for. The symbol face is the toolkit's own Inter Symbols and stays so.
func TestTheStylesTheWindowNeverDrawsKeepTheToolkitsFace(t *testing.T) {
	ours := parts.Theme()
	toolkit := theme.DefaultTheme()
	for _, style := range []fyne.TextStyle{
		{Italic: true},
		{Bold: true, Italic: true},
		{Monospace: true},
		{Symbol: true},
	} {
		got := ours.Font(style)
		want := toolkit.Font(style)
		if got.Name() != want.Name() {
			t.Errorf("for %+v our theme answers %s and the toolkit %s - a style Inter does not ship "+
				"has to fall through to the face the toolkit would have used", style, got.Name(), want.Name())
		}
	}
	for _, style := range []fyne.TextStyle{{}, {Bold: true}} {
		if name := ours.Font(style).Name(); !strings.HasPrefix(name, "Inter-") {
			t.Errorf("for %+v our theme answers %s, which is not Inter", style, name)
		}
	}
}
