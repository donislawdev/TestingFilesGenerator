package parts

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// The palette, computed before the first widget existed and wired on
// 2026-08-11. Until then the window ran on the toolkit's default theme and the
// numbers in docs/UX.md sections 8.2 and 8.3 described nothing - O70.
//
// Both variants are here because a user switches them with a system setting
// rather than with anything this tool offers, so a light theme nobody measured
// is half the product without evidence.
//
// The four roles are measured against three different thresholds and that
// distinction matters more than the values: text at 4.5, anything carrying
// state at 3.0, and a surface that only has to be noticed at a difference in
// L* rather than at a contrast ratio at all. A single ratio threshold on
// subtle surfaces measures something different in each variant, because the
// ratio compresses against a light background.
var (
	darkColours = map[fyne.ThemeColorName]color.Color{
		theme.ColorNameBackground:      hex(0x1E, 0x1E, 0x1E),
		theme.ColorNameForeground:      hex(0xE6, 0xE6, 0xE6),
		theme.ColorNamePlaceHolder:     hex(0x9D, 0xA3, 0xA8),
		theme.ColorNameDisabled:        hex(0x9D, 0xA3, 0xA8),
		theme.ColorNameError:           hex(0xF1, 0x70, 0x7A),
		theme.ColorNameSuccess:         hex(0x6F, 0xCF, 0x7F),
		theme.ColorNameWarning:         hex(0xE8, 0xB3, 0x3E),
		theme.ColorNamePrimary:         hex(0x6F, 0xB7, 0xF0),
		theme.ColorNameHyperlink:       hex(0x6F, 0xB7, 0xF0),
		theme.ColorNameFocus:           hex(0x4C, 0x9D, 0xF0),
		theme.ColorNameHover:           hex(0x3C, 0x3C, 0x3E),
		theme.ColorNameSelection:       hex(0x2C, 0x4A, 0x6B),
		theme.ColorNameInputBackground: hex(0x2E, 0x2E, 0x30),
		theme.ColorNameButton:          hex(0x2E, 0x2E, 0x30),
		theme.ColorNameMenuBackground:  hex(0x2E, 0x2E, 0x30),

		// What is written ON one of those colours, which is a different
		// question from what they contrast with. Section 8 computed them as
		// text against the page, and the toolkit also fills a button with one
		// and writes on top - so the dark variant's colours, chosen to be
		// light enough to read on a dark page, need dark writing on them.
		//
		// Measured 2026-08-11 and it was a real regression on the way in: the
		// Generate button came out white on #6FB7F0, which is 2.16:1 against a
		// threshold of 4.5. The toolkit's own blue had been 4.47:1, so
		// installing a measured palette had made one thing worse.
		theme.ColorNameForegroundOnPrimary: hex(0x1A, 0x1A, 0x1A),
		theme.ColorNameForegroundOnError:   hex(0x1A, 0x1A, 0x1A),
		theme.ColorNameForegroundOnSuccess: hex(0x1A, 0x1A, 0x1A),
		theme.ColorNameForegroundOnWarning: hex(0x1A, 0x1A, 0x1A),
	}

	lightColours = map[fyne.ThemeColorName]color.Color{
		theme.ColorNameBackground:      hex(0xFF, 0xFF, 0xFF),
		theme.ColorNameForeground:      hex(0x1A, 0x1A, 0x1A),
		theme.ColorNamePlaceHolder:     hex(0x59, 0x59, 0x59),
		theme.ColorNameDisabled:        hex(0x59, 0x59, 0x59),
		theme.ColorNameError:           hex(0xB3, 0x12, 0x1F),
		theme.ColorNameSuccess:         hex(0x10, 0x6B, 0x2E),
		theme.ColorNameWarning:         hex(0x8A, 0x5A, 0x00),
		theme.ColorNamePrimary:         hex(0x0F, 0x5F, 0xA8),
		theme.ColorNameHyperlink:       hex(0x0F, 0x5F, 0xA8),
		theme.ColorNameFocus:           hex(0x0F, 0x62, 0xFE),
		theme.ColorNameHover:           hex(0xDF, 0xDF, 0xDF),
		theme.ColorNameSelection:       hex(0xCF, 0xE4, 0xF7),
		theme.ColorNameInputBackground: hex(0xEF, 0xEF, 0xEF),
		theme.ColorNameButton:          hex(0xEF, 0xEF, 0xEF),
		theme.ColorNameMenuBackground:  hex(0xEF, 0xEF, 0xEF),

		// The other way round here, for the same reason: these are dark enough
		// to read on a white page, so what is written on them is white.
		theme.ColorNameForegroundOnPrimary: hex(0xFF, 0xFF, 0xFF),
		theme.ColorNameForegroundOnError:   hex(0xFF, 0xFF, 0xFF),
		theme.ColorNameForegroundOnSuccess: hex(0xFF, 0xFF, 0xFF),
		theme.ColorNameForegroundOnWarning: hex(0xFF, 0xFF, 0xFF),
	}
)

func hex(r, g, b uint8) color.Color { return color.NRGBA{R: r, G: g, B: b, A: 0xFF} }

// ours is the palette laid over the toolkit's theme.
//
// Everything not named above falls through to the default rather than being
// invented here. A theme that answers every name is a theme that has to be
// kept in step with a toolkit that adds them, and the ones we have opinions
// about are the ones somebody measured.
type ours struct{ fyne.Theme }

// Color answers dark whatever the system asks for.
//
// Decision of the owner, 2026-08-11: this program has one look. The variant is
// deliberately ignored rather than absent - Fyne passes whichever the desktop
// is set to, and returning the dark palette for both is what "one look" means
// here.
//
// The light palette below is kept, measured and guarded, and is not installed.
// That is not an oversight to tidy away later: it is the half of docs/UX.md
// section 8 that was computed at the same time as the other, and deleting it
// would throw away work whose whole point was that it is cheapest to do before
// the widgets exist. If this program ever follows the system setting, the
// colours are already worked out and already meet their thresholds.
func (o ours) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if c, ok := darkColours[name]; ok {
		return c
	}
	return o.Theme.Color(name, theme.VariantDark)
}

// Theme is the look of this tool, for the app to install and for a probe or a
// guard to render with. A picture taken under a different theme is a picture
// of a screen nobody has.
func Theme() fyne.Theme { return ours{theme.DefaultTheme()} }

// PaletteColour is one colour of either palette, for a guard to measure.
//
// It reads the palette for the variant asked about rather than going through
// Theme, which answers dark for everything. Both are still measured, because
// the numbers are the thing that took the work and they are what makes the
// light half usable the day somebody wants it.
func PaletteColour(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	palette := darkColours
	if variant == theme.VariantLight {
		palette = lightColours
	}
	if c, ok := palette[name]; ok {
		return c
	}
	return theme.DefaultTheme().Color(name, variant)
}
