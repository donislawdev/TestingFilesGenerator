package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// The window cannot be made narrower than the two columns it is laid out in.
//
// Fyne has no SetMinSize on a window. Checked in the pinned source rather than
// assumed: internal/driver/glfw/window_desktop.go line 297 hands the window
// manager view.SetSizeLimits(minWidth, minHeight, ...) and takes both numbers
// from minSizeOnScreen, which is the CONTENT's MinSize. So the smallest a
// window can be dragged to is not something anybody declares - it falls out of
// whatever the layout happens to ask for.
//
// That makes it exactly the kind of number observation O118 is about. Nothing
// states it, so nothing notices the day a layout change quietly lowers it, and
// what a person would meet is a window they can squeeze until the report column
// is a sliver and the form's boxes are narrower than the values in them. Which
// is defect 2 from the redesign coming back from the other side.
//
// The floor asserted here is the form's own smallest width plus the report
// column, and it is asked of the laid out screen rather than added up from the
// constants - adding up the constants would be a copy of the layout's own
// arithmetic and would agree with it however wrong both were.
//
// Every screen, because the window is one window: whichever tab somebody is on
// when they drag the edge is the one that decides.
func TestTheWindowCannotBeMadeTooNarrowToUse(t *testing.T) {
	ourTheme(t)
	content, _ := laidOutWindow(t)

	// Below this a Windows path in the report column wraps to four lines and
	// the panel stops being a summary. It is a judgement rather than a
	// threshold from anywhere, and it is recorded as one in
	// docs/GUI-REDESIGN-2026-09-08.md.
	const floor = 900

	for _, tab := range []string{
		text.TabOneTarget(), text.TabPresets(), text.TabRecipe(), text.TabAbout(),
	} {
		screen := tabContent(t, content, tab)
		wide := screen.MinSize().Width
		if wide < floor {
			t.Errorf("the %s screen asks for %.0f px at its smallest and the window will let itself "+
				"be dragged to that. Below about %d px the report column stops fitting a path on "+
				"one line. What to do: this is not a number to lower - find the layout that stopped "+
				"asking for the room it needs.", tab, wide, floor)
		}
		if wide < parts.ReportWidth {
			t.Errorf("the %s screen asks for %.0f px, which is less than the %0.f px the report "+
				"column alone is drawn at - so the column is being squeezed rather than the form",
				tab, wide, float32(parts.ReportWidth))
		}
	}
}

// The four surfaces of this window stay in the order they were measured in.
//
// Page, panel, raised panel, box - each one lighter than the last, and that
// ordering is the whole of what makes them tell apart. docs/UX.md section 8.1
// puts a surface in the "decoration" role, which carries no contrast threshold
// at all, so there is no number here to assert. What there is instead is the
// sequence: a raised panel darker than the panel it stands beside, or lighter
// than a box, is not a subtler version of the design - it is a different one.
//
// It matters because the two panels stand SIDE BY SIDE. The form's panel holds
// nothing but boxes and the raised one holds no boxes at all, so if the raised
// surface drifts up to meet the box colour, a box on the panel next to it reads
// as the same surface. That was measured while choosing the value: the obvious
// lighter candidate was 4.75 L* clear of the panel and only 1.78 from a box,
// and it was rejected on the second number.
//
// In L* rather than in a contrast ratio, for the reason section 8.1 gives: the
// ratio compresses against a light background, so one threshold would mean two
// different things in the two variants. Both variants are checked, because the
// light one is measured and guarded even though it is not installed.
func TestTheSurfacesOfThisWindowStayInOrder(t *testing.T) {
	for _, variant := range []struct {
		name string
		v    fyne.ThemeVariant
	}{{"dark", theme.VariantDark}, {"light", theme.VariantLight}} {
		page := lightness(parts.PaletteColour(theme.ColorNameBackground, variant.v))
		panel := lightness(parts.PaletteColour(parts.ColorNamePanel, variant.v))
		raised := lightness(parts.PaletteColour(parts.ColorNamePanelRaised, variant.v))
		box := lightness(parts.PaletteColour(theme.ColorNameInputBackground, variant.v))

		// Dark runs upwards from the page and light runs downwards from it, so
		// the rule is stated as distance from the page rather than as "lighter".
		// Written the other way it would hold in one variant and be nonsense in
		// the other.
		steps := []struct {
			name string
			at   float64
		}{{"page", page}, {"panel", panel}, {"raised panel", raised}, {"box", box}}
		for i := 1; i < len(steps); i++ {
			from := abs(steps[i-1].at - page)
			to := abs(steps[i].at - page)
			if to <= from {
				t.Errorf("%s: %s is %.2f L* from the page and %s is %.2f - so %s is not the further "+
					"of the two and the stack has lost a rung. The four surfaces have to step away "+
					"from the page in order or two of them read as one.",
					variant.name, steps[i-1].name, from, steps[i].name, to, steps[i].name)
			}
		}
	}
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
