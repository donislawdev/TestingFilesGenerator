package guard

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/catalogue"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// The palette page shows every colour the palette has, and says how to read
// each one.
//
// The page builds its rows from parts.PaletteNames rather than from a list
// beside it, so a colour cannot be missing from the page - it can only arrive
// with nothing said about it, which is what this guard is really about. A row
// that says "no reading written for this one yet" is a colour somebody added
// without deciding what it is measured against, and against WHAT is the half
// of a palette that gets forgotten: every threshold in this project is a pair.
//
// Both variants are asked about. The light one is computed and not installed,
// and a page that quietly showed only the installed half would leave that work
// exactly as invisible as it was before this page existed.
func TestThePalettePageShowsEveryColourAndHowToReadIt(t *testing.T) {
	shown := textIn(catalogue.Page())

	names := parts.PaletteNames()
	if len(names) < 20 {
		t.Fatalf("the palette answers for %d names, too few to be the real palette", len(names))
	}

	for _, name := range names {
		if !strings.Contains(shown, string(name)) {
			t.Errorf("the palette page never names %s", name)
		}
		if !parts.PaletteRanked(name) {
			t.Errorf("%s has no place in the order colours are read in, so it lands at the end of the page "+
				"where nobody decided it should be - give it a rank in parts/palette.go", name)
		}
		for _, variant := range []fyne.ThemeVariant{theme.VariantDark, theme.VariantLight} {
			if value := hexOf(parts.PaletteColour(name, variant)); !strings.Contains(shown, value) {
				t.Errorf("the palette page never prints %s, which is what %s holds in the %s variant",
					value, name, variantName(variant))
			}
		}
	}

	// The two sentences that say a colour arrived without anybody saying how
	// it is read. Asked about as text rather than by reaching into the
	// catalogue's own table, because what matters is what a person opening
	// the page is told.
	for _, silence := range []string{"Not placed yet", "no reading written for this one yet"} {
		if strings.Contains(shown, silence) {
			t.Errorf("the palette page carries %q, so a colour is on it with nothing said about "+
				"what it is drawn on - the reading is the point of the page", silence)
		}
	}

	// And the light variant is actually there, under a heading that says what
	// it is. It has never been on a screen and the whole reason for showing it
	// is that "computed, not installed" is a state somebody has to be told.
	for _, said := range []string{"Palette, as this window draws it", "not installed"} {
		if !strings.Contains(shown, said) {
			t.Errorf("the palette page never says %q", said)
		}
	}
}

// hexOf is how the page writes a colour, which is the string this guard has to
// look for. Alpha and all: half the palette is translucent, and a page that
// printed those as if they were opaque would be describing colours nobody can
// edit.
//
// A SECOND formatter beside the catalogue's, on purpose, and not a thing to
// tidy away into one shared helper. Calling the page's own function here would
// ask the page whether it agrees with itself, and this project has a whole
// entry in CLAUDE.md about guards that compare a result with the same result:
// the site guard was green for a month while both halves said "twenty
// formats" at twenty-one. The source of truth is parts.PaletteColour, and the
// only way to ask whether the page prints THAT is to render the value here.
// If the two formats ever diverge this goes red, which is the correct outcome
// for a page changing how it writes a colour.
func hexOf(c color.Color) string {
	r, g, b, a := c.RGBA()
	switch {
	case a == 0:
		return "nothing - drawn transparent on purpose"
	case a < 0xFFFF:
		return fmt.Sprintf("#%02X%02X%02X at 0x%02X", r*0xFFFF/a>>8, g*0xFFFF/a>>8, b*0xFFFF/a>>8, a>>8)
	}
	return fmt.Sprintf("#%02X%02X%02X", r>>8, g>>8, b>>8)
}

func variantName(v fyne.ThemeVariant) string {
	if v == theme.VariantLight {
		return "light"
	}
	return "dark"
}
