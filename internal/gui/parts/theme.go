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
// ColorNamePanel is the surface a section is drawn on.
//
// A name of ours rather than one of the toolkit's, because the toolkit has
// none: every colour it offers for a background is either the page itself or
// something belonging to a control. Naming it here keeps it in the one table
// the guard measures, which is the whole point of having a palette rather than
// colours at call sites.
const ColorNamePanel fyne.ThemeColorName = "panel"

// ColorNameLabel is the ink of a field's name, a step quieter than the value
// in the box under it, so the value is the brightest thing in the field.
// Owner's decision of 2026-09-21 - see docs/GUI-STRUCTURE-2026-09-21.md.
const ColorNameLabel fyne.ThemeColorName = "label"

// ColorNameLift is what the pointer does to a face that is itself light - the
// filled primary button - and ColorNameShade is what a press does to it.
//
// Two names of ours rather than the toolkit's Hover and Pressed, and the
// reason is arithmetic rather than taste. L* is not linear: the palette's hover
// of white at 0x22 moves a dark face by 12.7 L* and the primary face by 3.7,
// which is the 1.12 contrast O205 measured on Generate - a hover drawn and not
// seen. To move the primary face by the 10 L* this palette calls noticeable
// takes white at 0x66, and that same alpha on a dark face would move it by
// twenty five. One name cannot be right for both faces, so the light face has
// its own two. Measured on 2026-09-15: 83.0 L* under the pointer and 58.5
// pressed, against 71.9 at rest, with the ink on every one of them above 4.5.
const (
	ColorNameLift  fyne.ThemeColorName = "lift"
	ColorNameShade fyne.ThemeColorName = "shade"
)

var (
	darkColours = map[fyne.ThemeColorName]color.Color{
		theme.ColorNameBackground:  hex(0x1E, 0x1E, 0x1E),
		theme.ColorNameForeground:  hex(0xE6, 0xE6, 0xE6),
		theme.ColorNamePlaceHolder: hex(0x9D, 0xA3, 0xA8),
		// A step of its own between a value and a hint, since 2026-08-20.
		//
		// It was the placeholder colour exactly, so a box switched off for the
		// length of a run drew the value somebody typed in the same grey as the
		// hint in an empty box beside it - "you cannot edit this right now" and
		// "there is nothing here yet" said with one colour. Measured off a
		// render of a run in flight: the format box read #9DA3A8 for "txt" and
		// the width box read #9DA3A8 for "worked out from the size".
		//
		// Three steps now, and the gaps are what was picked rather than the
		// values: 91.3, 80.3 and 66.7 in L*, which is 11.0 and 13.6 apart. The
		// palette's own yardstick for "noticeable" is 10.
		//
		// Brighter rather than dimmer, because a disabled value is content
		// somebody typed and a hint is not - and dimming it further would have
		// made the one thing a person wants to re-read during a run the hardest
		// thing on the screen to read. A button loses its fill when it is
		// disabled, so it does not need the text to carry the whole message.
		theme.ColorNameDisabled:  hex(0xC2, 0xC8, 0xCD),
		theme.ColorNameError:     hex(0xF1, 0x70, 0x7A),
		theme.ColorNameSuccess:   hex(0x6F, 0xCF, 0x7F),
		theme.ColorNameWarning:   hex(0xE8, 0xB3, 0x3E),
		theme.ColorNamePrimary:   hex(0x6F, 0xB7, 0xF0),
		theme.ColorNameHyperlink: hex(0x6F, 0xB7, 0xF0),
		// Translucent, for the reason the hover below it is - and this one was
		// left opaque when that was corrected on 2026-08-11, so the same defect
		// stayed on the screen in a second place for a day.
		//
		// The toolkit does not draw this as a ring despite the name every
		// palette gives it. A menu fills its WHOLE background with it -
		// select.go, bgColor - and a button blends it over its own colour, so
		// an opaque value replaced a chosen format with a solid blue bar and
		// wrote the format's name across it. Measured off the render on
		// 2026-08-12: #E6E6E6 on #4C9DF0 is 2.28:1, against the 4.5 a reader
		// needs. Reported from the screen as a field that stays lit after the
		// choice has been made.
		//
		// The value is 0x66 rather than the toolkit's own 0x2a because ours has
		// to work over an input box rather than over the page: blue at 0x66
		// over #2E2E30 comes out at #3A5A7D, which leaves the value on it at
		// 5.73:1. What it can no longer do is say WHERE the keyboard is - a
		// wash that text survives is a wash nothing else can see either, and
		// that is arithmetic rather than a compromise. See parts.Ring, which is
		// what carries the state instead.
		theme.ColorNameFocus: overlay(0x4C, 0x9D, 0xF0, 0x66),
		// Translucent, and that is the fix for a defect somebody saw rather
		// than a preference. The toolkit does not paint this colour, it BLENDS
		// it over whatever is underneath - so an opaque grey replaced the blue
		// of the Generate button the moment the pointer touched it, reported on
		// 2026-08-11 as an ugly colour on hover.
		//
		// White at 0x22 over the page comes out at #3C3C3E, which is the row
		// hover of docs/UX.md section 8.2 to the byte. The measured look is
		// kept and the behaviour is corrected, because section 8 computed this
		// as the background of a row and the toolkit uses it on anything.
		theme.ColorNameHover: overlay(0xFF, 0xFF, 0xFF, 0x22),
		// What a press does to a dark face: more of the same white, because a
		// press has to be told from the hover it follows and black over
		// #2A2A2D moves it by 3.9 L*, which nobody sees. Measured 2026-09-15.
		theme.ColorNamePressed: overlay(0xFF, 0xFF, 0xFF, 0x40),
		// The two for the light face, see ColorNameLift.
		ColorNameLift:            overlay(0xFF, 0xFF, 0xFF, 0x66),
		ColorNameShade:           overlay(0x00, 0x00, 0x00, 0x33),
		theme.ColorNameSelection: hex(0x2C, 0x4A, 0x6B),
		// A box to type in has to be findable without reading a word, and until
		// 2026-08-23 it was not. Measured on the palette as it stood: a field
		// sat 8 of 255 above the panel it is drawn on, a ratio of 1.11, and its
		// border came from the toolkit's default 11 above the fill. Three
		// surfaces within sixteen steps of each other, so the eye had nothing
		// to count fields by.
		//
		// The step is 18 now against the panel and 29 from fill to border,
		// which is 2.3 and 2.6 times what it was. Deliberately still a raised
		// surface rather than a sunken one: a well reads well on a card and
		// disappears where there is no card, and a field measured at 1.06
		// against the window background would be worse than what it replaced.
		theme.ColorNameInputBackground: hex(0x38, 0x38, 0x3D),
		theme.ColorNameInputBorder:     hex(0x55, 0x55, 0x5C),
		// A button is no longer the same colour as a box to type in. It was,
		// exactly, and that is half of why a menu and a field were one object
		// with an arrow on the end of it - see the note on Chooser.
		theme.ColorNameButton: hex(0x4A, 0x4A, 0x52),
		ColorNameLabel:        hex(0xCF, 0xD3, 0xD8),

		// A menu floats over everything, so it is the LIGHTEST surface rather
		// than another one at the height of an input box.
		//
		// It was #2E2E30 until 2026-08-12, which is the input colour to the
		// byte, and it opens over a panel at #262628 - 3.8 L* apart, with no
		// border and no shadow anybody could see. Reported from a screenshot as
		// a list where everything runs together, and it was: the list, the form
		// behind it and the box it came from were three surfaces within four
		// L* of each other.
		//
		// It moved again on 2026-08-23, and that is the interesting part: making
		// a field visible pushed the input to 23.7 L*, which left the menu 0.9
		// above it. The value had not changed and the relationship it was
		// chosen for had - one number in a stack cannot be moved on its own.
		// Exactly the defect described above, reintroduced by a change aimed
		// somewhere else, and caught by recomputing the stack rather than by
		// looking at it.
		//
		// The first attempt at the new value aimed at the 5.6 the paragraph
		// above records and landed at 5.8 - and went red, because the guard for
		// an open list asks for 6.2. Worth keeping: the prose here carried the
		// distance that WAS measured, the guard carries the distance that is
		// REQUIRED, and reading the prose for the threshold got it wrong by a
		// margin no one would see by looking.
		//
		// #48484F is 30.8 L*, which puts it 7.1 above an input and 15.6 above
		// the panel. The stack reads page 11.3, panel 15.2, input 23.7, menu
		// 30.8: each one told from the one under it, and the one that floats
		// furthest from all of them.
		theme.ColorNameMenuBackground: hex(0x48, 0x48, 0x4F),

		// And what it casts, which is the other half of floating. The toolkit
		// draws a shadow under every popup and asks the theme for its colour -
		// so a shadow nobody set is the toolkit's default over a page darker
		// than the one it was chosen for. Black at two thirds reads as depth
		// against #1E1E1E rather than as a smudge.
		// NOTHING, since 2026-08-24, and it is a decision off the screen
		// rather than off a measurement. Black at two thirds under a popup read
		// as a hard dark band along the bottom edge of the open format list -
		// reported as the thing the eye keeps landing on.
		//
		// What is lost is nothing, because the shadow was never what told the
		// list from the form. The menu surface does that on its own and the
		// numbers are already in this file: 30.8 L* against a panel at 17.2,
		// which is 13.6 apart against the 10 this palette calls noticeable.
		// The shadow was the second mark and the second mark was the loud one.
		//
		// It reaches the format list and nothing else that matters. The longer
		// explanation behind a field's button is drawn on our own sheet rather
		// than as a popup - see parts.Tips - so it never asked the theme for a
		// shadow and does not lose one.
		theme.ColorNameShadow: color.Transparent,

		// The surface a section is drawn on, and the line round its edge.
		//
		// These exist because the toolkit has no colour for a panel. Measured on
		// 2026-08-12: widget.Card fills itself with ColorNameBackground - the
		// same name the page uses - so a card came out at exactly the page
		// colour, 0.00 L* apart, and the only thing marking its edge was a
		// shadow going the wrong way, down to #151515. Grouping was not faint,
		// it was absent, and no value put in the palette could have fixed it
		// because there was no name to put it under.
		//
		// The fill splits the gap between the page and an input box rather than
		// taking the surface value section 8.2 already records. That value is
		// what an input box is - a panel painted with it would swallow every
		// field standing on it. Three surfaces stack here and each has to be
		// told from the one under it: page 11.3, panel 17.2, input 23.7 in L*,
		// which is +5.9 and +6.5.
		//
		// It used to be 15.2, and the line round the edge did the work the fill
		// could not - four L* is a surface you sense rather than see. That line
		// is gone as of 2026-08-23 and the fill was lifted to replace it,
		// because the same one pixel border was what every box to type in used
		// to say "your value goes here". A mark meaning two things means
		// neither, and a container can group by being a surface where a field
		// cannot: a field has to say where the typing goes.
		//
		// So the numbers above are load bearing now in a way they were not.
		// TestASectionDrawsItsOwnSurface holds the fill against the page with
		// no edge to fall back on.
		//
		// The colour is 29.4 L* and what gets drawn is not. Measured off the
		// render on 2026-08-12: a one pixel stroke lands between pixels and is
		// anti-aliased, so the brightest pixel of the edge comes out at 22.3 -
		// which is +7.1 from the panel and +11.0 from the page. It reads, and
		// it reads because of the gap to the page rather than the one to the
		// panel. Worth keeping straight, because the number in a palette is a
		// claim about a colour and only the render is a claim about a line.
		//
		// It is no longer the panel's edge - see the note above - so the only
		// thing drawn with it now is parts.Divider. The measurement stays
		// because it is about a one pixel line on this page, which is what a
		// divider is.
		theme.ColorNameSeparator: hex(0x45, 0x45, 0x49),
		ColorNamePanel:           hex(0x2A, 0x2A, 0x2D),

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
		theme.ColorNameBackground:  hex(0xFF, 0xFF, 0xFF),
		theme.ColorNameForeground:  hex(0x1A, 0x1A, 0x1A),
		theme.ColorNamePlaceHolder: hex(0x59, 0x59, 0x59),
		// The same three steps the other way up: on a white page "closer to
		// the value" means darker. 9.3, 25.8 and 37.8 in L*, 16.5 and 12.1
		// apart.
		theme.ColorNameDisabled:  hex(0x3D, 0x3D, 0x3D),
		theme.ColorNameError:     hex(0xB3, 0x12, 0x1F),
		theme.ColorNameSuccess:   hex(0x10, 0x6B, 0x2E),
		theme.ColorNameWarning:   hex(0x8A, 0x5A, 0x00),
		theme.ColorNamePrimary:   hex(0x0F, 0x5F, 0xA8),
		theme.ColorNameHyperlink: hex(0x0F, 0x5F, 0xA8),
		// The same correction as the dark variant above, at the same alpha:
		// this is blended over whatever it lands on rather than painted.
		theme.ColorNameFocus: overlay(0x0F, 0x62, 0xFE, 0x66),
		// Black rather than white here: this page is white, so its hover is
		// DARKER than what is under it. 0x20 over white comes out at #DFDFDF,
		// which is what section 8.3 measured.
		theme.ColorNameHover: overlay(0x00, 0x00, 0x00, 0x20),
		// The same way up as the hover: a light face is pressed darker. The
		// light primary is dark blue, so it is lifted with white like the dark
		// palette's - written the same way, measured only for the dark one.
		theme.ColorNamePressed:   overlay(0x00, 0x00, 0x00, 0x33),
		ColorNameLift:            overlay(0xFF, 0xFF, 0xFF, 0x66),
		ColorNameShade:           overlay(0x00, 0x00, 0x00, 0x33),
		theme.ColorNameSelection: hex(0xCF, 0xE4, 0xF7),
		// The same step, worked out the same way against a white page. Here a
		// field is the sunken one - on white there is nowhere lighter to go -
		// so it sits below the panel rather than above it, 16 of 255 down
		// instead of the 8 it was, with a border 63 below the fill.
		theme.ColorNameInputBackground: hex(0xE7, 0xE7, 0xEA),
		theme.ColorNameInputBorder:     hex(0xA8, 0xA8, 0xB0),
		theme.ColorNameButton:          hex(0xEF, 0xEF, 0xEF),
		ColorNameLabel:                 hex(0x44, 0x44, 0x48),
		// The same reasoning the other way up: on a white page a floating
		// surface is the lightest thing there is, so it is white and the shadow
		// is what separates it.
		theme.ColorNameMenuBackground: hex(0xFF, 0xFF, 0xFF),
		theme.ColorNameShadow:         overlay(0x00, 0x00, 0x00, 0x40),

		// The same two, worked out the same way against a white page. The gap
		// between page and input is narrower here - 5.6 L* against 7.7 - so the
		// panel splits it at 2.8 either side, and the line again carries the
		// edge at 11.5 L*. Computed rather than installed, like the rest of this
		// palette.
		theme.ColorNameSeparator: hex(0xD6, 0xD6, 0xD9),
		ColorNamePanel:           hex(0xF4, 0xF4, 0xF5),

		// The other way round here, for the same reason: these are dark enough
		// to read on a white page, so what is written on them is white.
		theme.ColorNameForegroundOnPrimary: hex(0xFF, 0xFF, 0xFF),
		theme.ColorNameForegroundOnError:   hex(0xFF, 0xFF, 0xFF),
		theme.ColorNameForegroundOnSuccess: hex(0xFF, 0xFF, 0xFF),
		theme.ColorNameForegroundOnWarning: hex(0xFF, 0xFF, 0xFF),
	}
)

func hex(r, g, b uint8) color.Color { return color.NRGBA{R: r, G: g, B: b, A: 0xFF} }

// overlay is a colour the toolkit blends over whatever is beneath it, rather
// than one it paints. The distinction is invisible in a palette table and
// decides what a hovered button looks like.
func overlay(r, g, b, a uint8) color.Color { return color.NRGBA{R: r, G: g, B: b, A: a} }

// blended is an overlay laid over a face, worked out here so that a control
// whose face IS the fill - the primary button - can paint one opaque colour
// rather than stacking a translucent rectangle on an opaque one.
//
// The same arithmetic as the toolkit's blendColor (widget/button.go), the
// over operator on premultiplied 16 bit channels, kept in step by hand
// because the toolkit's is unexported: a guard measures the pixels a button
// comes out as, so the two cannot quietly disagree without something going
// red.
func blended(under, over color.Color) color.Color {
	dstR, dstG, dstB, dstA := under.RGBA()
	srcR, srcG, srcB, srcA := over.RGBA()
	blend := func(src, dst, alpha uint32) uint16 {
		return uint16((src + dst - (dst * alpha / 0xFFFF)) & 0xFFFF)
	}
	return color.RGBA64{
		R: blend(srcR, dstR, srcA),
		G: blend(srcG, dstG, srcA),
		B: blend(srcB, dstB, srcA),
		A: blend(srcA, dstA, srcA),
	}
}

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

// Size is our type scale and our spacing, over the toolkit's.
//
// The scale is the point rather than the individual numbers: a screen title,
// a section title, a field name and an explanation are four ranks, and until
// 2026-08-11 three of them were one size in one weight. Nothing led the eye,
// which is the first question the UX section 7 checklist asks.
//
// The two heading names are swapped from what they sound like, on purpose. The
// toolkit draws a card's title at SizeNameHeadingText, and a card is a section
// INSIDE a screen - so the name a screen's own title uses has to end up bigger
// than the one its cards use, whatever the names suggest. Measured on screen:
// with the toolkit's 24 and 18 the sections shouted over the page they were on.
func (o ours) Size(name fyne.ThemeSizeName) float32 {
	// Every answer is a token, so the ladder of type and the scale of
	// distances have one home (parts/tokens.go). A size not named here is the
	// toolkit's own, and the point of naming one is that somebody chose it.
	//
	// The room inside a control is the one that was argued over: the
	// toolkit's 8 makes a row of a menu 41 px tall for 13 px of text -
	// measured off the open list on 2026-08-12 - and moving it here made the
	// whole form denser, not the list. The list got a control of its own
	// instead (parts/openlist.go), and 6 stays on its own merits: it is the
	// room inside every box and button on the form, and it went in against a
	// render of the form.
	switch name {
	case theme.SizeNameSubHeadingText:
		return TextTitle
	case theme.SizeNameHeadingText:
		return TextHeading
	case theme.SizeNameText:
		return TextBody
	case theme.SizeNameCaptionText:
		return TextCaption
	case theme.SizeNamePadding:
		return ThemePadding
	case theme.SizeNameInnerPadding:
		return ControlInset
	case theme.SizeNameCardRadius:
		return RadiusPanel
	case theme.SizeNameInputRadius, theme.SizeNameButtonRadius:
		return RadiusField
	}
	return o.Theme.Size(name)
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
