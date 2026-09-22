package guard

import (
	"image/color"
	"math"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// Every colour in the palette is the one the rule produces.
//
// The palette guard next door asks whether the values are READABLE. This one
// asks whether they are the values somebody would get by following the rule,
// and the two questions are not the same: a colour typed in by hand can pass
// every contrast threshold and still leave the window painted from two
// different systems. That is not hypothetical here. Measured on 2026-09-22, on
// the palette this one replaced: the surfaces stood at hue 291, the inks at
// 251 to 262, and the page and the main ink were pure neutral - three greys,
// every one of them above its threshold, none of them from the same idea.
//
// What this guard is really for is the edit nobody would call wrong. A value
// gets nudged for the screen it is being looked at on, the guards stay green
// because thresholds have room, and the ladder quietly stops being a ladder.
// It happened before this rule existed and it is written up beside the menu
// colour in parts/theme.go: a button's face was added at 31.7 L* while the
// menu - the surface that floats over everything - sat at 30.8, so the highest
// surface in the window was a button for a month. Nobody typed a wrong number.
// Two right numbers were chosen a month apart, against different neighbours,
// and nothing in the project compared them.
//
// The rule is stated here rather than in parts, and that is deliberate. The
// package keeps a flat table of colours a person can read and grep, and the
// arithmetic that produced it lives with the test that checks it - so the
// table is the artefact and this is the question asked of it. The same numbers
// drive tools/probes/palette/design.py, which is where a new palette is
// generated.
func TestThePaletteIsTheOneTheRuleProduces(t *testing.T) {
	for _, variant := range []struct {
		name   string
		v      fyne.ThemeVariant
		ladder map[fyne.ThemeColorName]float64
		accent float64
		sel    float64
	}{
		{"dark", theme.VariantDark, darkLadder, 70.0, 30.5},
		{"light", theme.VariantLight, lightLadder, 42.0, 88.0},
	} {
		// The neutrals: one hue, one curve, and lightness off the ladder.
		//
		// The question is asked of the BYTES rather than of the lightness, the
		// chroma and the hue read back separately, and the first cut of this
		// test got that wrong: it asked whether each colour still sat at hue
		// 291 and went red on three light values sitting at 295 to 297. They
		// were right. Eight bits per channel cannot hold a hue exactly at
		// chroma under about four - half a step in a* or b* is several degrees
		// there - so the tolerance would have had to grow with the tint until
		// it measured nothing. Comparing what the rule produces, rounded the
		// same way, has no such hole.
		for name, want := range variant.ladder {
			wanted := labToRGB(want, neutralChroma*math.Sin(math.Pi*want/100), neutralHue)
			check(t, variant.name, name, parts.PaletteColour(name, variant.v), wanted,
				"the ladder puts that rank at %.1f L* on hue %.0f", want, neutralHue)
		}

		// The accent and the three colours that carry meaning: one lightness,
		// and one SHARE of the chroma each hue can actually reach. A common
		// chroma is not available at one lightness - at 70 L* red reaches 49.5
		// where green reaches 80.1 - so what is shared is the share.
		for name, hue := range map[fyne.ThemeColorName]float64{
			theme.ColorNamePrimary:   accentHue,
			theme.ColorNameHyperlink: accentHue,
			theme.ColorNameError:     25,
			theme.ColorNameSuccess:   145,
			theme.ColorNameWarning:   80,
		} {
			chroma := accentShare * gamutChroma(variant.accent, hue)
			check(t, variant.name, name, parts.PaletteColour(name, variant.v),
				labToRGB(variant.accent, chroma, hue),
				"a colour carrying meaning stands at %.1f L* and %.0f%% of the %.1f chroma hue %.0f can reach",
				variant.accent, accentShare*100, gamutChroma(variant.accent, hue), hue)
		}

		// Selection is the accent's hue held down to a surface's lightness, so
		// a chosen row belongs to the same family as the thing that chose it.
		chroma := math.Min(selectionChroma, gamutChroma(variant.sel, accentHue))
		check(t, variant.name, theme.ColorNameSelection,
			parts.PaletteColour(theme.ColorNameSelection, variant.v),
			labToRGB(variant.sel, chroma, accentHue),
			"the selection is the accent at %.1f L*", variant.sel)
	}
}

// The ladder climbs in even steps.
//
// Separate from the test above, because that one would pass a ladder whose
// rungs are 2, 9 and 3 apart as long as the table and the palette agreed on
// it. This is the property the eye actually reads: a surface is told from the
// one under it by the same distance everywhere, so nothing in the window is
// nearly the same colour as its neighbour by accident.
//
// The old palette failed this and that is why the rule exists: its steps were
// 5.9, 6.5, 1.4, 0.9 and 4.7 in L*, so three surfaces near the top - the
// separator, the menu and a button's face - stood within 2.3 L* of each other
// and read as one colour in three names.
func TestTheLadderClimbsInEvenSteps(t *testing.T) {
	for _, variant := range []struct {
		name  string
		v     fyne.ThemeVariant
		ranks []fyne.ThemeColorName
	}{
		{"dark", theme.VariantDark, darkRanks},
		{"light", theme.VariantLight, lightRanks},
	} {
		steps := make([]float64, 0, len(variant.ranks)-1)
		for i := 1; i < len(variant.ranks); i++ {
			under, _, _ := lch(parts.PaletteColour(variant.ranks[i-1], variant.v))
			over, _, _ := lch(parts.PaletteColour(variant.ranks[i], variant.v))
			steps = append(steps, math.Abs(over-under))
		}

		least, most := steps[0], steps[0]
		for _, s := range steps {
			least, most = math.Min(least, s), math.Max(most, s)
		}
		if most-least > 1.0 {
			t.Errorf("%s: the rungs are %.1f apart at the tightest and %.1f at the widest, "+
				"so the ladder is not a ladder - %v", variant.name, least, most, steps)
		}
		if least < 5.0 {
			t.Errorf("%s: two surfaces stand %.1f L* apart, under the 5 that reads as a step at all",
				variant.name, least)
		}
		t.Logf("%s: rungs %v", variant.name, rounded(steps))
	}
}

// The rule, in numbers. Candidate A of the three rendered for the owner on
// 2026-09-22 - the one that keeps the window's character and puts its
// relationships in order.
const (
	neutralHue    = 291.0 // every surface and every ink
	neutralChroma = 5.0   // the peak of the curve, at the middle of the ladder
	accentHue     = 259.0 // the blue a person presses
	accentShare   = 0.85  // of the chroma the hue can reach, not of a number

	// How much colour a chosen row carries. A ceiling rather than a share: the
	// selection has to stay a SURFACE, and one at the accent's own chroma
	// would shout louder than the accent standing on it.
	selectionChroma = 22.0
)

var (
	darkLadder = map[fyne.ThemeColorName]float64{
		theme.ColorNameBackground:      11.3,
		parts.ColorNamePanel:           17.8,
		theme.ColorNameInputBackground: 24.3,
		theme.ColorNameButton:          30.8,
		theme.ColorNameSeparator:       30.8, // a line borrows the rank of a face
		theme.ColorNameMenuBackground:  37.3,
		theme.ColorNameInputBorder:     43.0,
		theme.ColorNamePlaceHolder:     66.7,
		theme.ColorNameDisabled:        80.3,
		parts.ColorNameLabel:           84.5,
		theme.ColorNameForeground:      91.3,
	}
	lightLadder = map[fyne.ThemeColorName]float64{
		theme.ColorNameBackground:      100.0,
		parts.ColorNamePanel:           94.0,
		theme.ColorNameInputBackground: 88.0,
		theme.ColorNameButton:          82.0,
		theme.ColorNameSeparator:       82.0,
		theme.ColorNameMenuBackground:  100.0, // on white, what floats is white
		theme.ColorNameInputBorder:     70.0,
		theme.ColorNamePlaceHolder:     37.8,
		theme.ColorNameDisabled:        25.8,
		parts.ColorNameLabel:           28.5,
		theme.ColorNameForeground:      9.3,
	}

	// The surfaces in order, for the step test. The menu is left out of the
	// light one on purpose: on a white page it is white, which is the page's
	// own rank rather than a rung above the face below it.
	darkRanks = []fyne.ThemeColorName{
		theme.ColorNameBackground, parts.ColorNamePanel, theme.ColorNameInputBackground,
		theme.ColorNameButton, theme.ColorNameMenuBackground, theme.ColorNameInputBorder,
	}
	lightRanks = []fyne.ThemeColorName{
		theme.ColorNameBackground, parts.ColorNamePanel, theme.ColorNameInputBackground,
		theme.ColorNameButton,
	}
)

// gamutChroma is how much chroma a hue has in sRGB at a lightness, found by
// halving - the same way tools/probes/palette/design.py finds it, because a
// requested chroma outside the gamut would be clipped into a different HUE,
// which is the one thing this palette holds constant.
func gamutChroma(L, hue float64) float64 {
	low, high := 0.0, 150.0
	for i := 0; i < 40; i++ {
		mid := (low + high) / 2
		if inGamut(L, mid, hue) {
			low = mid
		} else {
			high = mid
		}
	}
	_, c, _ := lch(labToRGB(L, low, hue))
	return c
}

// The colour arithmetic this file needs beyond what palette_test.go already
// has. That one asks about lightness and contrast, which both fall out of
// luminance alone. A hue and a chroma need the other two axes, so the
// conversion is written out here.
//
// CIE Lab, D65, the same white point and the same matrices as the probe in
// tools/probes/palette - and the two agreeing is not assumed: the values in
// the palette were produced by the probe and are checked here, so a
// disagreement between the two shows up as a red test rather than as a colour
// nobody questions.
func lch(c color.Color) (float64, float64, float64) {
	r, g, b := channels(c)
	x, y, z := xyz(linear(r), linear(g), linear(b))
	f := func(t float64) float64 {
		if t > 0.008856 {
			return math.Cbrt(t)
		}
		return 7.787*t + 16.0/116.0
	}
	fx, fy, fz := f(x/whiteX), f(y), f(z/whiteZ)
	lightness := 116*fy - 16
	a, bb := 500*(fx-fy), 200*(fy-fz)
	return lightness, math.Hypot(a, bb), math.Mod(math.Atan2(bb, a)*180/math.Pi+360, 360)
}

func xyz(r, g, b float64) (float64, float64, float64) {
	return 0.4124564*r + 0.3575761*g + 0.1804375*b,
		0.2126729*r + 0.7151522*g + 0.0721750*b,
		0.0193339*r + 0.1191920*g + 0.9503041*b
}

// labChannels is a lightness, chroma and hue back in linear sRGB, before
// anything is clamped - so inGamut can ask whether the colour exists.
func labChannels(l, c, h float64) (float64, float64, float64) {
	a := c * math.Cos(h*math.Pi/180)
	bb := c * math.Sin(h*math.Pi/180)
	fy := (l + 16) / 116
	fx, fz := fy+a/500, fy-bb/200
	back := func(t float64) float64 {
		if t*t*t > 0.008856 {
			return t * t * t
		}
		return (t - 16.0/116.0) / 7.787
	}
	x, y, z := back(fx)*whiteX, back(fy), back(fz)*whiteZ
	return gamma(3.2404542*x - 1.5371385*y - 0.4985314*z),
		gamma(-0.9692660*x + 1.8760108*y + 0.0415560*z),
		gamma(0.0556434*x - 0.2040259*y + 1.0572252*z)
}

func gamma(v float64) float64 {
	if v <= 0.0031308 {
		return 12.92 * v
	}
	return 1.055*math.Pow(v, 1/2.4) - 0.055
}

func inGamut(l, c, h float64) bool {
	r, g, b := labChannels(l, c, h)
	for _, v := range []float64{r, g, b} {
		if v < -0.0005 || v > 1.0005 {
			return false
		}
	}
	return true
}

func labToRGB(l, c, h float64) color.Color {
	r, g, b := labChannels(l, c, h)
	to8 := func(v float64) uint8 {
		return uint8(math.Round(math.Max(0, math.Min(1, v)) * 255))
	}
	return color.NRGBA{R: to8(r), G: to8(g), B: to8(b), A: 0xFF}
}

// angle is how far apart two hues are on a wheel, so 359 and 1 are two
// degrees apart rather than three hundred and fifty eight.
func angle(one, two float64) float64 {
	d := math.Mod(math.Abs(one-two), 360)
	if d > 180 {
		return 360 - d
	}
	return d
}

func rounded(values []float64) []float64 {
	out := make([]float64, len(values))
	for i, v := range values {
		out[i] = math.Round(v*10) / 10
	}
	return out
}

const (
	whiteX = 0.95047
	whiteZ = 1.08883
)

// check holds one colour of the palette against the one the rule produces.
//
// A byte either way is allowed and nothing more. The two conversions - this
// one and the probe's in tools/probes/palette - halve their way into the sRGB
// gamut in floating point and then round, so the last bit is not something
// either of them promises. Anything a person would call a different colour is
// several bytes away, and the whole point of the tolerance being this tight is
// that a nudge "just a shade lighter" lands outside it.
func check(t *testing.T, variant string, name fyne.ThemeColorName, got, want color.Color, why string, args ...any) {
	t.Helper()
	gotR, gotG, gotB, _ := got.RGBA()
	wantR, wantG, wantB, _ := want.RGBA()
	off := func(a, b uint32) bool { return math.Abs(float64(a>>8)-float64(b>>8)) > 1 }
	if off(gotR, wantR) || off(gotG, wantG) || off(gotB, wantB) {
		L, C, h := lch(got)
		wantL, wantC, wantH := lch(want)
		t.Errorf("%s: %s is #%02X%02X%02X (%.1f L*, %.2f C*, %.0f h) and the rule produces "+
			"#%02X%02X%02X (%.1f L*, %.2f C*, %.0f h) - "+why,
			append([]any{variant, name, gotR >> 8, gotG >> 8, gotB >> 8, L, C, h,
				wantR >> 8, wantG >> 8, wantB >> 8, wantL, wantC, wantH}, args...)...)
	}
}
