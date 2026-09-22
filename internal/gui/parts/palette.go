package parts

import (
	"image/color"
	"math"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
)

// What this file adds to theme.go: a way to LOOK at the palette.
//
// The values have been measured, guarded and written about since 2026-08-11
// and until 2026-09-22 there was nowhere in the running program to see them.
// That is the same shape as the defect the catalogue of parts was built to
// end - a control existed in four states and only three were ever drawn
// anywhere - one floor down. A palette nobody can open is a palette argued
// about from memory.

// Swatch is a square of one colour, for the palette page of the catalogue.
//
// It wears a line round it, and that line is not decoration: half of this
// palette is within a few L* of the page it would be drawn on, so a swatch of
// the panel colour on a panel is an invisible square in a row that claims to
// show a colour. The line is the one a divider uses, so the mark is one this
// window already makes rather than a second kind of edge.
func Swatch(c color.Color) fyne.CanvasObject {
	chip := canvas.NewRectangle(c)
	chip.CornerRadius = RadiusMark
	chip.StrokeColor = PaletteColour(theme.ColorNameSeparator, theme.VariantDark)
	chip.StrokeWidth = edgeWidth
	chip.SetMinSize(fyne.NewSize(SwatchSide, SwatchSide))
	return chip
}

// PaletteNames is every colour this palette answers for, in the order a person
// reads them: the ladder from the page up to the edge of a field, then the
// inks, then what carries meaning, then what the pointer and the keyboard do,
// and last what is written on a filled face.
//
// Read off the palette itself rather than listed here, which is the whole
// point. A list typed out beside a map is a list that goes stale the first
// time somebody adds a colour and does not think of the page that shows it -
// this project has the receipts on that, and they are in CLAUDE.md. A name
// nobody has ranked still comes back, at the end, where it is obvious.
func PaletteNames() []fyne.ThemeColorName {
	names := make([]fyne.ThemeColorName, 0, len(darkColours))
	for name := range darkColours {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		oneRank, oneKnown := paletteOrder[names[i]]
		twoRank, twoKnown := paletteOrder[names[j]]
		if oneKnown != twoKnown {
			return oneKnown
		}
		if oneRank != twoRank {
			return oneRank < twoRank
		}
		return names[i] < names[j]
	})
	return names
}

// PaletteRanked says whether a colour has a place in the order above. It is
// false for a colour somebody added without deciding where it is read, which
// is a question for a person rather than something to guess at here.
func PaletteRanked(name fyne.ThemeColorName) bool {
	_, ok := paletteOrder[name]
	return ok
}

// PaletteSeen is what a colour LOOKS like, which is a different question from
// what it is - and the difference is the one this palette has been caught by
// twice. The toolkit blends the hover, the press and the focus over whatever
// is underneath rather than painting them, so the raw value of the hover is a
// colour nobody has ever seen on this screen. Given the surface it lands on,
// this returns what is drawn.
func PaletteSeen(name fyne.ThemeColorName, over color.Color, variant fyne.ThemeVariant) color.Color {
	c := PaletteColour(name, variant)
	if _, _, _, alpha := c.RGBA(); alpha < 0xFFFF {
		return blended(over, c)
	}
	return c
}

// Contrast is the WCAG ratio between two colours, from 1 to 21 - the measure
// for anything that is read, at 4.5, and for anything carrying state, at 3.0.
func Contrast(one, two color.Color) float64 {
	a, b := relativeLuminance(one), relativeLuminance(two)
	if a < b {
		a, b = b, a
	}
	return (a + 0.05) / (b + 0.05)
}

// Lightness is CIE L*, which is perceptually even - so one number means the
// same thing against a dark background and a light one. It is the measure for
// a surface, where a contrast ratio would compress against a light page and
// quietly mean something different in each variant.
func Lightness(c color.Color) float64 {
	y := relativeLuminance(c)
	if y > 0.008856 {
		return 116*math.Cbrt(y) - 16
	}
	return 903.3 * y
}

func relativeLuminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	straight := func(v uint32) float64 {
		f := float64(v) / 65535
		if f <= 0.04045 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	return 0.2126*straight(r) + 0.7152*straight(g) + 0.0722*straight(b)
}

// The order colours are read in. Ranks rather than a sorted list of names, so
// that adding a colour is a decision about WHERE IT BELONGS and not about
// where it lands alphabetically.
var paletteOrder = map[fyne.ThemeColorName]int{
	// The ladder, bottom to top.
	theme.ColorNameBackground:      10,
	ColorNamePanel:                 11,
	theme.ColorNameInputBackground: 12,
	theme.ColorNameButton:          13,
	theme.ColorNameSeparator:       14,
	theme.ColorNameMenuBackground:  15,
	theme.ColorNameInputBorder:     16,

	// The inks, brightest first, which is the order they are met in a field.
	theme.ColorNameForeground:  20,
	ColorNameLabel:             21,
	theme.ColorNameDisabled:    22,
	theme.ColorNamePlaceHolder: 23,

	// What carries meaning.
	theme.ColorNamePrimary:   30,
	theme.ColorNameHyperlink: 31,
	theme.ColorNameError:     32,
	theme.ColorNameSuccess:   33,
	theme.ColorNameWarning:   34,
	theme.ColorNameSelection: 35,

	// What the pointer, the press and the keyboard do - every one of these is
	// blended rather than painted.
	theme.ColorNameHover:   40,
	theme.ColorNamePressed: 41,
	ColorNameLift:          42,
	ColorNameShade:         43,
	theme.ColorNameFocus:   44,
	ColorNameTipShade:      45,
	theme.ColorNameShadow:  46,

	// What is written on a filled face.
	theme.ColorNameForegroundOnPrimary: 50,
	theme.ColorNameForegroundOnError:   51,
	theme.ColorNameForegroundOnSuccess: 52,
	theme.ColorNameForegroundOnWarning: 53,
}
