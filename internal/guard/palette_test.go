package guard

import (
	"image/color"
	"math"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// The palette meets the thresholds it was computed against, in both variants.
//
// docs/UX.md section 8 worked these out before the first widget existed, and
// until 2026-08-11 nothing installed them: the window ran on the toolkit's
// default and the tables described a screen nobody had. O70. Measured then:
// the focus ring was 1.16:1 against a threshold of 3.0, so the one thing that
// says which box the keyboard is in was very nearly invisible.
//
// The numbers are computed here rather than copied from the document. A test
// that repeats the table checks the table against itself - the whole reason
// this project keeps saying that a number in prose is not a measurement.
//
// Three thresholds and not one, which is the part worth keeping straight:
// text at 4.5, something carrying state at 3.0, and a surface that only has to
// be noticed at a difference in L* rather than at a ratio at all. A ratio
// compresses against a light background, so one ratio threshold on subtle
// surfaces measures something different in each variant.
func TestThePaletteMeetsTheContrastItWasComputedFor(t *testing.T) {
	for _, variant := range []struct {
		name string
		v    fyne.ThemeVariant
	}{{"dark", theme.VariantDark}, {"light", theme.VariantLight}} {
		background := parts.PaletteColour(theme.ColorNameBackground, variant.v)

		// Read: 4.5, from WCAG 1.4.3.
		for _, name := range []fyne.ThemeColorName{
			theme.ColorNameForeground,
			theme.ColorNamePlaceHolder,
			theme.ColorNameError,
			theme.ColorNameSuccess,
			theme.ColorNameWarning,
			theme.ColorNamePrimary,
		} {
			if got := contrast(parts.PaletteColour(name, variant.v), background); got < 4.5 {
				t.Errorf("%s: %s is %.2f:1 against the background, under the 4.5 a reader needs",
					variant.name, name, got)
			}
		}

		// Recognised as a state: 3.0, from WCAG 1.4.11. The focus ring is the
		// only thing that says where the keyboard is.
		if got := contrast(parts.PaletteColour(theme.ColorNameFocus, variant.v), background); got < 3.0 {
			t.Errorf("%s: the focus ring is %.2f:1 against the background, under the 3.0 that makes it a visible state",
				variant.name, got)
		}

		// Noticed rather than read: 10 in L*, which is a judgement recorded as
		// a judgement in section 8.1 rather than borrowed from a standard.
		for _, name := range []fyne.ThemeColorName{theme.ColorNameHover, theme.ColorNameSelection} {
			if got := lightnessGap(parts.PaletteColour(name, variant.v), background); got < 10 {
				t.Errorf("%s: %s differs from the background by %.1f L*, under the 10 that makes it noticeable",
					variant.name, name, got)
			}
		}

		// What is written ON a colour, which is a different question from what
		// that colour contrasts with, and the one this guard missed on its
		// first day. The toolkit fills a button with the primary and writes on
		// top: measured white on #6FB7F0 at 2.16:1, under a threshold of 4.5,
		// while the toolkit's own blue had been 4.47:1. Installing a measured
		// palette had made one thing worse, and the guard beside it was green
		// because it only ever compared a colour with the page.
		for _, pair := range []struct{ on, fill fyne.ThemeColorName }{
			{theme.ColorNameForegroundOnPrimary, theme.ColorNamePrimary},
			{theme.ColorNameForegroundOnError, theme.ColorNameError},
			{theme.ColorNameForegroundOnSuccess, theme.ColorNameSuccess},
			{theme.ColorNameForegroundOnWarning, theme.ColorNameWarning},
		} {
			got := contrast(parts.PaletteColour(pair.on, variant.v), parts.PaletteColour(pair.fill, variant.v))
			if got < 4.5 {
				t.Errorf("%s: %s is %.2f:1 on %s, under the 4.5 a reader needs",
					variant.name, pair.on, got, pair.fill)
			}
		}

		t.Logf("%s: foreground %.2f:1, focus %.2f:1, error %.2f:1, selection %.1f L*, on primary %.2f:1",
			variant.name,
			contrast(parts.PaletteColour(theme.ColorNameForeground, variant.v), background),
			contrast(parts.PaletteColour(theme.ColorNameFocus, variant.v), background),
			contrast(parts.PaletteColour(theme.ColorNameError, variant.v), background),
			lightnessGap(parts.PaletteColour(theme.ColorNameSelection, variant.v), background),
			contrast(parts.PaletteColour(theme.ColorNameForegroundOnPrimary, variant.v),
				parts.PaletteColour(theme.ColorNamePrimary, variant.v)))
	}
}

// The window looks the same whatever the desktop is set to.
//
// Decision of the owner, 2026-08-11: this program has one look, the dark one.
// Guarded rather than left to the theme's shape, because the toolkit hands the
// system's variant to every colour lookup and the obvious way to write a theme
// is to branch on it - so a screen that quietly went half light on somebody
// else's desktop is one plausible edit away, and it is the kind of edit that
// looks correct.
//
// The light palette is still measured by the test above. It is computed and
// not installed, which is a different thing from absent.
func TestTheWindowKeepsOneLookWhateverTheDesktopSays(t *testing.T) {
	shipped := parts.Theme()
	names := []fyne.ThemeColorName{
		theme.ColorNameBackground, theme.ColorNameForeground, theme.ColorNameError,
		theme.ColorNameFocus, theme.ColorNameSelection, theme.ColorNameInputBackground,
		theme.ColorNameForegroundOnPrimary,
	}
	for _, name := range names {
		light := shipped.Color(name, theme.VariantLight)
		dark := shipped.Color(name, theme.VariantDark)
		if light != dark {
			t.Errorf("%s answers %v on a light desktop and %v on a dark one, so the window follows the system setting",
				name, light, dark)
		}
		if want := parts.PaletteColour(name, theme.VariantDark); dark != want {
			t.Errorf("%s is %v and the dark palette says %v", name, dark, want)
		}
	}
}

// The two variants are actually different colours.
//
// A theme that answers with one palette whatever it is asked passes every
// threshold above twice and is still half a product: the light variant is what
// somebody gets from a system setting this tool does not offer to change.
func TestTheLightAndDarkPalettesAreNotTheSame(t *testing.T) {
	same := 0
	names := []fyne.ThemeColorName{
		theme.ColorNameBackground, theme.ColorNameForeground, theme.ColorNameError,
		theme.ColorNameFocus, theme.ColorNameSelection,
	}
	for _, name := range names {
		if parts.PaletteColour(name, theme.VariantDark) == parts.PaletteColour(name, theme.VariantLight) {
			same++
			t.Errorf("%s is the same colour in both variants", name)
		}
	}
	if same == len(names) {
		t.Fatal("every colour matched, so the theme is ignoring the variant entirely")
	}
}

// contrast is the WCAG ratio between two colours, from 1 to 21.
func contrast(a, b color.Color) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func luminance(c color.Color) float64 {
	r, g, b := channels(c)
	return 0.2126*linear(r) + 0.7152*linear(g) + 0.0722*linear(b)
}

func linear(v float64) float64 {
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

// lightnessGap is the difference in CIE L*, which is perceptually even - so
// one number means the same thing against a dark background and a light one.
func lightnessGap(a, b color.Color) float64 {
	return math.Abs(lightness(a) - lightness(b))
}

func lightness(c color.Color) float64 {
	y := luminance(c)
	if y > 0.008856 {
		return 116*math.Cbrt(y) - 16
	}
	return 903.3 * y
}

func channels(c color.Color) (float64, float64, float64) {
	r, g, b, _ := c.RGBA()
	return float64(r) / 65535, float64(g) / 65535, float64(b) / 65535
}
