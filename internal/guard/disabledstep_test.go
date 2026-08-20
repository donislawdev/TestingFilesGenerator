package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// A box switched off does not look like a box that is empty.
//
// The palette gave ColorNameDisabled and ColorNamePlaceHolder the same value.
// Measured off a render of a run in flight on 2026-08-20: the format box drew
// "txt" - a value somebody chose - at #9DA3A8, and the width box beside it
// drew "worked out from the size" at #9DA3A8 as well. "You cannot edit this
// right now" and "there is nothing here yet" said in one colour, on one screen,
// at the same moment.
//
// Three ranks of text live in a box and each has to be told from the other two:
// a value, a value that is switched off, and a hint about a box with nothing in
// it. So this asks for two gaps rather than one - a disabled value that had
// crept up to the ordinary foreground would be as wrong as one that had sunk
// back to the placeholder, and a guard checking one end would let the other
// through.
//
// Measured in L* rather than as a contrast ratio, for the reason the palette
// guard records: the ratio compresses against a light background, so one
// threshold on it means two different things in the two variants. Ten is what
// this palette already calls noticeable.
func TestAValueSwitchedOffIsItsOwnStepBetweenAValueAndAHint(t *testing.T) {
	for _, variant := range []struct {
		name string
		v    fyne.ThemeVariant
	}{{"dark", theme.VariantDark}, {"light", theme.VariantLight}} {
		t.Run(variant.name, func(t *testing.T) {
			value := parts.PaletteColour(theme.ColorNameForeground, variant.v)
			off := parts.PaletteColour(theme.ColorNameDisabled, variant.v)
			hint := parts.PaletteColour(theme.ColorNamePlaceHolder, variant.v)

			fromValue := lightnessGap(value, off)
			fromHint := lightnessGap(off, hint)

			if fromValue < 10 {
				t.Errorf("a switched off value is %.1f L* from an ordinary one, under the 10 this palette calls"+
					" noticeable - so a form frozen for a run looks like a form somebody can still type into", fromValue)
			}
			if fromHint < 10 {
				t.Errorf("a switched off value is %.1f L* from a hint, under the 10 this palette calls noticeable"+
					" - so the value somebody typed and the hint in an empty box read as the same thing", fromHint)
			}
		})
	}
}

// The step goes towards the value rather than away from it.
//
// Which side it falls on is a decision and not an accident. A disabled value is
// content somebody typed, and a hint is not - so dimming it further would have
// made the one thing a person wants to re-read while a run is going the hardest
// thing on the screen to read. A button already loses its fill when it is
// disabled, so the text does not have to carry that message on its own.
func TestASwitchedOffValueIsEasierToReadThanAHint(t *testing.T) {
	for _, variant := range []struct {
		name string
		v    fyne.ThemeVariant
	}{{"dark", theme.VariantDark}, {"light", theme.VariantLight}} {
		t.Run(variant.name, func(t *testing.T) {
			box := parts.PaletteColour(theme.ColorNameInputBackground, variant.v)
			off := contrast(parts.PaletteColour(theme.ColorNameDisabled, variant.v), box)
			hint := contrast(parts.PaletteColour(theme.ColorNamePlaceHolder, variant.v), box)

			if off <= hint {
				t.Errorf("a switched off value stands at %.2f against its box and a hint stands at %.2f."+
					" The value is the one somebody typed, so it cannot be the harder of the two to read", off, hint)
			}
		})
	}
}
