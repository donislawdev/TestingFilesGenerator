package catalogue

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// The palette, as a page somebody can open.
//
// Every row is built from parts.PaletteNames rather than from a list written
// out here, so a colour added to the palette appears on this page by itself.
// The reading beside each one is computed when the page is built, not copied
// from a document: a number in prose is a claim about the day it was written,
// and this one is a claim about the binary it is printed by.
//
// Both variants are here. The light one is computed, measured by the same
// guards and NOT installed - the owner's decision of 2026-08-11 is that this
// program has one look - and showing it is the only way anybody ever sees the
// half of the work that has never been on a screen.

// paletteSections is the palette as the catalogue shows it: one section per
// variant, each grouped the way the colours are read.
func paletteSections() []fyne.CanvasObject {
	return []fyne.CanvasObject{
		paletteSection("Palette, as this window draws it", theme.VariantDark,
			"Every colour the window paints with, measured against what it is drawn on."),
		paletteSection("Palette, light - computed and not installed", theme.VariantLight,
			"Worked out and guarded to the same thresholds. The window does not use it: this program has one look."),
	}
}

func paletteSection(title string, variant fyne.ThemeVariant, sentence string) fyne.CanvasObject {
	rows := []fyne.CanvasObject{parts.Caption(sentence)}
	group := ""
	for _, name := range parts.PaletteNames() {
		role, known := palettePlan[name]
		if !known {
			// A colour nobody has said how to read. It still shows, because
			// the alternative is a page that quietly omits it.
			role = paletteRole{group: unplaced, what: "no reading written for this one yet"}
		}
		if role.group != group {
			group = role.group
			rows = append(rows, parts.Subheading(group))
		}
		rows = append(rows, paletteRow(name, role, variant))
	}
	return parts.Section(title, rows...)
}

// paletteRow is one colour: the square, the name it is asked for by, the value
// it holds and the number that says whether it is doing its job.
func paletteRow(name fyne.ThemeColorName, role paletteRole, variant fyne.ThemeVariant) fyne.CanvasObject {
	return container.NewHBox(
		parts.Swatch(shown(name, role, variant)),
		parts.Column(parts.GapInline,
			parts.Caption(string(name)+" - "+role.what),
			parts.Caption(hexOf(parts.PaletteColour(name, variant))+reading(name, role, variant))),
	)
}

// shown is the colour to put in the square, which for anything translucent is
// what it comes to over the surface it lands on rather than the value itself.
func shown(name fyne.ThemeColorName, role paletteRole, variant fyne.ThemeVariant) color.Color {
	if role.against == "" {
		return parts.PaletteColour(name, variant)
	}
	return parts.PaletteSeen(name, parts.PaletteColour(role.against, variant), variant)
}

// reading is the measurement that decides whether this colour works: a
// contrast ratio for anything read, a distance in L* for a surface, and for
// something the toolkit blends, both what it comes to and how far it moved.
func reading(name fyne.ThemeColorName, role paletteRole, variant fyne.ThemeVariant) string {
	if role.against == "" {
		return ""
	}
	against := parts.PaletteColour(role.against, variant)
	switch role.measure {
	case byNothing:
		// A colour that is not measured against anything names no surface
		// either, so this is unreachable rather than empty - and it is written
		// out because the linter asks every switch on this type to say what it
		// does with every value, which is the correct thing to ask.
		return ""
	case byContrast:
		return fmt.Sprintf(" - %.2f:1 on %s", parts.Contrast(parts.PaletteColour(name, variant), against), role.on)
	case byStep:
		step := parts.Lightness(parts.PaletteColour(name, variant)) - parts.Lightness(against)
		return fmt.Sprintf(" - %.1f L*, %+.1f from %s",
			parts.Lightness(parts.PaletteColour(name, variant)), step, role.on)
	case byMove:
		seen := parts.PaletteSeen(name, against, variant)
		return fmt.Sprintf(" - over %s it comes to %s, %.1f L*, a move of %+.1f",
			role.on, hexOf(seen), parts.Lightness(seen), parts.Lightness(seen)-parts.Lightness(against))
	}
	return ""
}

// hexOf is the value as it stands in the palette, alpha and all.
//
// The alpha is printed rather than dropped, and the row beside it says what
// the colour comes to over the surface it lands on, because those are two
// different facts and this palette has been bitten by treating them as one:
// the toolkit BLENDS half of these names rather than painting them, so the
// raw value of the hover is a colour that has never been on the screen and
// the composited one is not what anybody can edit.
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

type measure int

const (
	byNothing measure = iota
	byContrast
	byStep
	byMove
)

type paletteRole struct {
	group   string
	what    string
	against fyne.ThemeColorName
	on      string
	measure measure
}

const (
	ladder   = "The ladder of surfaces"
	inks     = "The inks"
	meaning  = "What carries meaning"
	pointer  = "What the pointer, a press and the keyboard do"
	onFace   = "What is written on a filled face"
	unplaced = "Not placed yet"
)

// How each colour is read. The group and the sentence are for a person, and
// the pair at the end is the whole point: a colour is never measured on its
// own, it is measured against the thing it is drawn on, and getting THAT wrong
// is how this palette shipped a button nobody could read the word on.
var palettePlan = map[fyne.ThemeColorName]paletteRole{
	theme.ColorNameBackground: {ladder, "the page the window stands on", "", "", byNothing},
	parts.ColorNamePanel: {ladder, "the surface a section is drawn on",
		theme.ColorNameBackground, "the page", byStep},
	theme.ColorNameInputBackground: {ladder, "the fill of a box to type in",
		parts.ColorNamePanel, "the panel", byStep},
	theme.ColorNameButton: {ladder, "the face of a button that is not the main one",
		theme.ColorNameInputBackground, "a box to type in", byStep},
	theme.ColorNameSeparator: {ladder, "a line that separates and says nothing else",
		parts.ColorNamePanel, "the panel", byStep},
	theme.ColorNameMenuBackground: {ladder, "what the toolkit's own menus float on - the one a box opens on a right press. This window's lists float on a card since 2026-09-24",
		theme.ColorNameInputBackground, "a box to type in", byStep},
	theme.ColorNameInputBorder: {ladder, "the edge that says where the typing goes",
		theme.ColorNameInputBackground, "its own fill", byStep},

	// Each ink is measured against the LIGHTEST surface it is actually drawn
	// on, which for three of these four is not the panel. A ratio taken
	// against the wrong surface is the one mistake this page exists to stop
	// somebody making, and it was made here first: measured 2026-09-22, the
	// hint in an empty box reads 5.56:1 against a panel and 4.52:1 against the
	// box it is really in, which is 0.02 over the 4.5 a reader needs. The
	// comfortable number was the wrong number.
	//
	// The surface each one is drawn on is read from the code rather than
	// assumed: the words of a secondary button stand on its face, the
	// lightest surface Foreground is drawn on since the lists moved to a box's
	// surface on 2026-09-24 (parts/button.go, parts/parts.go floatingCard), a
	// value and a hint sit inside a box, and a field's name stands on the panel
	// beside it.
	theme.ColorNameForeground: {inks, "a value, and everything read at rest",
		theme.ColorNameButton, "a button's face", byContrast},
	parts.ColorNameLabel: {inks, "the name of a field, a step quieter than its value",
		parts.ColorNamePanel, "a panel", byContrast},
	theme.ColorNameDisabled: {inks, "a value in a box switched off for the length of a run",
		theme.ColorNameInputBackground, "a box to type in", byContrast},
	theme.ColorNamePlaceHolder: {inks, "a hint in a box nobody has typed in",
		theme.ColorNameInputBackground, "a box to type in", byContrast},

	theme.ColorNamePrimary: {meaning, "the action a screen is for, and the line round a focused control",
		theme.ColorNameBackground, "the page", byContrast},
	theme.ColorNameHyperlink: {meaning, "a link, the same blue as the action",
		theme.ColorNameBackground, "the page", byContrast},
	theme.ColorNameError: {meaning, "a refusal, and a file that failed",
		theme.ColorNameBackground, "the page", byContrast},
	theme.ColorNameSuccess: {meaning, "a run that wrote everything it promised",
		theme.ColorNameBackground, "the page", byContrast},
	theme.ColorNameWarning: {meaning, "a run that wrote files and skipped some",
		theme.ColorNameBackground, "the page", byContrast},
	theme.ColorNameSelection: {meaning, "the row or the segment somebody chose",
		theme.ColorNameBackground, "the page", byStep},

	theme.ColorNameHover: {pointer, "what the pointer does to a dark face",
		theme.ColorNameBackground, "the page", byMove},
	theme.ColorNamePressed: {pointer, "what a press does to a dark face",
		theme.ColorNameButton, "a button's face", byMove},
	parts.ColorNameLift: {pointer, "what the pointer does to the filled main button",
		theme.ColorNamePrimary, "the main button", byMove},
	parts.ColorNameShade: {pointer, "what a press does to the filled main button",
		theme.ColorNamePrimary, "the main button", byMove},
	theme.ColorNameFocus: {pointer, "the wash the toolkit lays over a control the keyboard is in",
		theme.ColorNameInputBackground, "a box to type in", byMove},
	parts.ColorNameTipShade: {pointer, "the shade an explanation casts on the form under it",
		parts.ColorNamePanel, "a panel", byMove},
	theme.ColorNameShadow: {pointer, "what a popup casts - nothing here, on purpose", "", "", byNothing},

	theme.ColorNameForegroundOnPrimary: {onFace, "the word on the main button",
		theme.ColorNamePrimary, "the main button", byContrast},
	theme.ColorNameForegroundOnError: {onFace, "what is written on a filled red",
		theme.ColorNameError, "a filled red", byContrast},
	theme.ColorNameForegroundOnSuccess: {onFace, "what is written on a filled green",
		theme.ColorNameSuccess, "a filled green", byContrast},
	theme.ColorNameForegroundOnWarning: {onFace, "what is written on a filled amber",
		theme.ColorNameWarning, "a filled amber", byContrast},
}

// swatch is the square itself, in the catalogue like every other part.
//
// The states are the ones that decide whether it works: the page colour, which
// is a black square on a black screen and is only a square at all because of
// the line round it, and a colour the toolkit blends, which is transparent and
// would be an empty frame if the page did not lay it over a surface first.
func swatch() Entry {
	return Entry{Name: "Swatch", Natural: true, States: []State{
		{"a surface near the bottom of the ladder", func() fyne.CanvasObject {
			return parts.Swatch(parts.PaletteColour(theme.ColorNameBackground, theme.VariantDark))
		}},
		{"a surface near the top of it", func() fyne.CanvasObject {
			return parts.Swatch(parts.PaletteColour(theme.ColorNameInputBorder, theme.VariantDark))
		}},
		{"a colour that carries meaning", func() fyne.CanvasObject {
			return parts.Swatch(parts.PaletteColour(theme.ColorNamePrimary, theme.VariantDark))
		}},
		{"an ink, which is nearly white", func() fyne.CanvasObject {
			return parts.Swatch(parts.PaletteColour(theme.ColorNameForeground, theme.VariantDark))
		}},
		{"something the toolkit blends, laid over a panel first", func() fyne.CanvasObject {
			return parts.Swatch(parts.PaletteSeen(theme.ColorNameHover,
				parts.PaletteColour(parts.ColorNamePanel, theme.VariantDark), theme.VariantDark))
		}},
	}}
}
