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
//
// What this guard does NOT cover, named because a control that is quiet
// about its limits is worse than no control: it asks whether a measurement is
// THERE, never whether it was taken against the right surface. Proved by
// mutation on 2026-09-23 - putting the hint's ratio back against a panel,
// which is the very mistake the review of #124 found, leaves this green. That
// question needs to know where each ink is drawn, which is a fact about the
// window rather than about this page, and it is asked in
// TestEachSurfaceIsToldFromTheOneUnderIt, where the pairs are named.
func TestThePalettePageShowsEveryColourAndHowToReadIt(t *testing.T) {
	shown := textIn(catalogue.Page())

	names := parts.PaletteNames()
	if len(names) < 20 {
		t.Fatalf("the palette answers for %d names, too few to be the real palette", len(names))
	}

	// Line by line rather than over the whole page, and that correction came
	// from the review of #124. Asking whether a name and a value appear
	// SOMEWHERE on the page is a question a broken page answers just as well:
	// proved by mutation on 2026-09-23, deleting the purpose from every row
	// and then deleting the measurement from every row both left this guard
	// green, because the names and the values were still there. A row is two
	// lines - "name - what it is for" and "value - what it measures" - and
	// both halves are what the page is for.
	lines := strings.Split(shown, "\n")
	lineWith := func(prefix string) (string, bool) {
		for _, line := range lines {
			if strings.HasPrefix(line, prefix) {
				return strings.TrimPrefix(line, prefix), true
			}
		}
		return "", false
	}

	for _, name := range names {
		if !strings.Contains(shown, string(name)) {
			t.Errorf("the palette page never names %s", name)
		}

		// What the colour is for, on the row that names it.
		if what, found := lineWith(string(name) + " - "); !found || len(strings.TrimSpace(what)) < 10 {
			t.Errorf("the palette page shows %s with nothing saying what it is for - "+
				"a swatch and a name is a colour chart, and the reason this page exists "+
				"is the sentence beside it", name)
		}
		if !parts.PaletteRanked(name) {
			t.Errorf("%s has no place in the order colours are read in, so it lands at the end of the page "+
				"where nobody decided it should be - give it a rank in parts/palette.go", name)
		}
		for _, variant := range []fyne.ThemeVariant{theme.VariantDark, theme.VariantLight} {
			value := hexOf(parts.PaletteColour(name, variant))
			if !strings.Contains(shown, value) {
				t.Errorf("the palette page never prints %s, which is what %s holds in the %s variant",
					value, name, variantName(variant))
				continue
			}
			if measuredAgainstNothing[name] {
				continue
			}
			// And the measurement, on the row that prints the value. Either a
			// ratio or a distance in lightness, because those are the two
			// units this palette is held to - a surface is asked for L* and
			// anything read is asked for a ratio.
			reading, found := lineWith(value + " - ")
			if !found || (!strings.Contains(reading, ":1") && !strings.Contains(reading, "L*")) {
				t.Errorf("the palette page prints %s for %s in the %s variant with no measurement beside it - "+
					"a value without the number that decides whether it works is the half of this page "+
					"that a document could have carried", value, name, variantName(variant))
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

// The two colours that are measured against nothing, and why each is right to
// be. Written out rather than inferred, so that a THIRD colour losing its
// measurement is a red test rather than a quiet exemption.
//
//   - The page is the floor of the ladder. Every surface above it is quoted as
//     a distance from the one under it, and the floor has nothing under it.
//   - The shadow is transparent on purpose since 2026-08-24 - it read as a
//     hard band under the open format list - so there is no colour to measure.
//     The page says that in words where a value would be.
var measuredAgainstNothing = map[fyne.ThemeColorName]bool{
	theme.ColorNameBackground: true,
	theme.ColorNameShadow:     true,
}
