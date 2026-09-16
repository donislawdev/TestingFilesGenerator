package guard

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// The face of the segmented switch, after the owner looked at it (O219), and
// the two halves of what a frozen form did not do to it (O223).

// The switch between the three ways of stating a size freezes with the rest
// of the form, and thaws with it.
//
// Measured on a render of the batch screen mid run on 2026-09-16: every box
// frozen, the switch above them live. It went on the form through
// Fields.Unlabelled, which built its row and registered nothing, and Freeze
// walked the registry - so a press on "A range" during a run rebuilt the size
// boxes under a form drawn as frozen. The guard for a frozen form asks the
// single batch screen, which has no such switch (O223).
func TestTheSizeWaySwitchFreezesWithTheRestOfTheForm(t *testing.T) {
	dir := t.TempDir()
	batches, _, _ := screenInAWindowWithHost(t, text.TabRecipe())
	entryUnder(t, batches, text.FieldTargetID()).SetText("invoices")
	entryUnder(t, batches, text.FieldOutputDir()).SetText(dir)
	entryUnder(t, batches, text.FieldSize()).SetText("64kb")
	entryUnder(t, batches, text.FieldCount()).SetText("400")
	press(t, batches, "Generate")

	box := entryUnder(t, batches, text.FieldSize())
	if !box.Disabled() {
		t.Fatalf("the size box is not frozen during the run, so this guard is asking about a form that never froze. Refusal: %q",
			anyRefusal(batches))
	}
	if !sizeWaySwitch(t, batches).Disabled() {
		t.Error("the size box is frozen and the switch above it is not - a press on another way of stating " +
			"a size rebuilds the boxes under a form drawn as frozen")
	}

	cancel := buttonNamed(batches, "Cancel")
	if cancel == nil {
		t.Fatalf("there is no Cancel button. The screen has: %v", buttonNames(batches))
	}
	cancel.OnTapped()
	if sizeWaySwitch(t, batches).Disabled() {
		t.Error("the switch is still frozen after the run stopped, so it never comes back")
	}
}

// A frozen switch still shows which way is chosen.
//
// Until 2026-09-16 every fill went transparent when the switch was disabled,
// so a form frozen for a run would have said nothing about which of the three
// ways it was running with. Nobody saw it, because the switch was not being
// frozen at all - the other half of O223.
func TestAFrozenSwitchStillShowsWhichWayIsChosen(t *testing.T) {
	ways := []string{"one", "two", "three"}
	s := parts.NewSegments(ways, nil)
	s.SetSelected("two")
	s.Disable()
	filled := segmentFills(t, s)
	if len(filled) != 1 || filled[0] != 1 {
		t.Errorf("the switch is frozen on %q and the filled segments are %v (by position), so it no longer says which way is chosen",
			"two", filled)
	}
	s.Enable()
	if filled := segmentFills(t, s); len(filled) != 1 || filled[0] != 1 {
		t.Errorf("thawed, the switch fills segments %v rather than the chosen one", filled)
	}
}

// The chosen segment's fill stays inside the border, and no rule touches it.
//
// What the owner saw on 2026-09-16 was three geometries at once: a sharp
// rectangle standing out past the arc of a rounded border, a hairline against
// its edge, and the ring outside. The fill is rounded and a border's width
// inside the switch now, and a rule stands only between two segments neither
// of which is the chosen one. Read off the renderer's objects by what they
// are, not by their position in the list - a guard reading by position is
// green the day the order changes.
func TestTheChosenSegmentStaysInsideTheBorderAndNoRuleTouchesIt(t *testing.T) {
	ways := []string{"one", "two", "three"}
	s := parts.NewSegments(ways, nil)
	s.SetSelected("two")
	s.Resize(s.MinSize())

	var fill *canvas.Rectangle
	var rules []*canvas.Rectangle
	for _, o := range test.WidgetRenderer(s).Objects() {
		rect, is := o.(*canvas.Rectangle)
		if !is {
			continue
		}
		switch {
		case rect.FillColor == parts.PaletteColour(theme.ColorNameSelection, theme.VariantDark):
			fill = rect
		case rect.Size().Width == parts.Hairline:
			rules = append(rules, rect)
		}
	}
	if fill == nil {
		t.Fatal("no segment is filled with the selection colour, so the chosen way is not drawn at all")
	}
	if len(rules) != len(ways)-1 {
		t.Fatalf("%d rules drawn for %d segments, so this guard read the wrong objects", len(rules), len(ways))
	}

	edge := float32(parts.EdgeWidth())
	pos, size, whole := fill.Position(), fill.Size(), s.Size()
	if pos.X < edge || pos.Y < edge || pos.X+size.Width > whole.Width-edge || pos.Y+size.Height > whole.Height-edge {
		t.Errorf("the chosen fill at %v size %v reaches under the border of a %v switch, so its corner stands out past the border's arc",
			pos, size, whole)
	}
	if fill.CornerRadius <= 0 {
		t.Error("the chosen fill has square corners inside a rounded border")
	}
	// The chosen segment is the second, so the first rule and the second both
	// touch it. A third would stand clear, and there is none with three ways.
	for i, rule := range rules {
		if rule.Visible() {
			t.Errorf("rule %d is drawn against the chosen segment's edge - the fill is the boundary there", i)
		}
	}
	s.SetSelected("one")
	if !rules[1].Visible() {
		t.Error("with the first way chosen the rule between the second and third is hidden, and it stands between two unchosen segments")
	}
}

// segmentFills is the position of every segment whose fill is not
// transparent. A fill is told from the border, the ring and the rules by what
// it is: the one rectangle rounded to sit inside the border's own radius.
func segmentFills(t *testing.T, s *parts.Segments) []int {
	t.Helper()
	var filled []int
	at := 0
	for _, o := range test.WidgetRenderer(s).Objects() {
		rect, is := o.(*canvas.Rectangle)
		if !is || rect.CornerRadius != parts.RadiusField-parts.EdgeWidth() {
			continue
		}
		if rect.FillColor != color.Transparent {
			filled = append(filled, at)
		}
		at++
	}
	if at != len(s.Options) {
		t.Fatalf("read %d fills for %d segments, so this guard is not reading the fills", at, len(s.Options))
	}
	return filled
}
