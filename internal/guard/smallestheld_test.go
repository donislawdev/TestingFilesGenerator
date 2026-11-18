package guard

import (
	"slices"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// Smallest held back until the picture has a width and a height, on both
// screens (docs/SIZE-FIXES-2026-11-16.md section 3.9). Reported by the owner
// from the window on 2026-11-18: Smallest could be chosen with neither set,
// and the run then refused it - and nothing on the screen said that smallest
// needs them. The owner chose to keep it on the switch, greyed, with the
// reason under the pointer, rather than to hide it.

// TestSmallestCannotBeChosenUntilTheWidthAndHeightAreSet asks two things on
// both screens.
//
// First, for every picture format at its defaults, whether Smallest is held -
// against the engine asked by a PLAN at those defaults, not by the function
// the window calls, so a window holding it back where a run would take it (svg
// declares its width and height) or letting it go where a run refuses turns
// this red.
//
// Then, on a png, everything a person can do to it: press it, reach it with
// the keys, hover it for the reason, set one side and then the other, and
// empty a side again with Smallest chosen - which keeps it chosen, because a
// switch that moved itself would change what the run makes without a word.
func TestSmallestCannotBeChosenUntilTheWidthAndHeightAreSet(t *testing.T) {
	refused := map[string]bool{}
	for _, d := range format.All() {
		if engine.TakesSmallest(d) {
			refused[d.ID] = !engineTakesSmallestAtDefaults(t, d)
		}
	}
	// Both answers have to occur: svg declares its sides and png does not.
	if !slices.Contains(mapValues(refused), true) || !slices.Contains(mapValues(refused), false) {
		t.Fatalf("the engine refused smallest at the defaults for %v - one of the two answers never came", refused)
	}
	for _, s := range sizeScreens(t) {
		held := map[string]bool{}
		for id := range refused {
			chooserIn(t, s.fields, s.format).SetSelected(id)
			held[id] = s.way(t).Held.Has(text.SizeWaySmallest())
		}
		for _, wrong := range heldDisagreements(held, refused) {
			t.Errorf("%s: %s", s.name, wrong)
		}
		pressingSmallest(t, s)
	}

	// The canary: a switch holding Smallest for every picture has to be told
	// apart from the engine, which takes it for svg at its defaults.
	always := map[string]bool{}
	for id := range refused {
		always[id] = true
	}
	if len(heldDisagreements(always, refused)) == 0 {
		t.Error("a switch holding Smallest for every picture agreed with the engine, so this compares nothing")
	}
}

// pressingSmallest drives one screen on a png through every way a person
// reaches Smallest, before and after the width and height are set.
func pressingSmallest(t *testing.T, s sizeScreen) {
	t.Helper()
	chooserIn(t, s.fields, s.format).SetSelected("png")
	setBox(t, s.fields, s.width, "")
	setBox(t, s.fields, s.height, "")
	way := s.way(t)
	way.SetSelected(text.SizeWayExact())
	smallest := text.SizeWaySmallest()
	if !way.Held.Has(smallest) {
		t.Fatalf("%s: a png with no width and no height offers Smallest to be chosen", s.name)
	}

	// Each asked on its own: End steps back off a held last option, so a
	// press that chose it would be undone by the key after it and read as
	// held.
	reaches := []struct {
		how  string
		move func()
	}{
		{"a press", func() { way.Tapped(&fyne.PointEvent{Position: parts.SegmentMiddle(way, smallest)}) }},
		{"End", func() { way.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEnd}) }},
		{"the right arrow", func() {
			for range way.Options {
				way.TypedKey(&fyne.KeyEvent{Name: fyne.KeyRight})
			}
		}},
	}
	for _, r := range reaches {
		way.SetSelected(text.SizeWayExact())
		r.move()
		if way.Selected == smallest || boxText(t, s.fields, s.size) == core.SizeSmallest {
			t.Errorf("%s: Smallest held back was chosen by %s - the switch says %q, the box %q",
				s.name, r.how, way.Selected, boxText(t, s.fields, s.size))
		}
	}

	way.MouseMoved(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: parts.SegmentMiddle(way, smallest)}})
	if way.Held.Telling() == nil {
		t.Errorf("%s: the pointer over Smallest held back opens no reason", s.name)
	}
	way.MouseOut()
	if way.Held.Telling() != nil {
		t.Errorf("%s: the reason stays open after the pointer left", s.name)
	}

	setBox(t, s.fields, s.width, "160")
	if !way.Held.Has(smallest) {
		t.Errorf("%s: Smallest let go with a width and no height, which the run refuses", s.name)
	}
	setBox(t, s.fields, s.height, "120")
	if way.Held.Has(smallest) {
		t.Fatalf("%s: Smallest still held with a width and a height set", s.name)
	}
	way.Tapped(&fyne.PointEvent{Position: parts.SegmentMiddle(way, smallest)})
	if way.Selected != smallest || boxText(t, s.fields, s.size) != core.SizeSmallest {
		t.Errorf("%s: a press on Smallest with the sides set left the switch on %q and the box holding %q",
			s.name, way.Selected, boxText(t, s.fields, s.size))
	}

	setBox(t, s.fields, s.width, "")
	if !way.Held.Has(smallest) {
		t.Errorf("%s: Smallest not held again once the width was emptied", s.name)
	}
	if way.Selected != smallest || boxText(t, s.fields, s.size) != core.SizeSmallest {
		t.Errorf("%s: emptying the width moved the switch to %q and the box to %q - what the run makes changed without a word",
			s.name, way.Selected, boxText(t, s.fields, s.size))
	}
}

// engineTakesSmallestAtDefaults plans one file of d at its smallest with
// nothing set - the form as a screen opens on it.
func engineTakesSmallestAtDefaults(t *testing.T, d format.Descriptor) bool {
	t.Helper()
	target := engine.Target{ID: "smallest", Format: d.ID, Smallest: true, Sizes: engine.Uniform(1, 0), Properties: map[string]string{}}
	_, err := engine.Plan([]engine.Target{target},
		engine.Options{OutDir: t.TempDir(), Seed: 7, Command: "guard", ManifestName: "manifest.json"})
	return err == nil
}

// heldDisagreements is every picture format where a switch holding Smallest
// back and the engine refusing it differ.
func heldDisagreements(held, refused map[string]bool) []string {
	var out []string
	for id, no := range refused {
		switch {
		case held[id] && !no:
			out = append(out, id+" holds Smallest back and the engine takes smallest for it at its defaults")
		case !held[id] && no:
			out = append(out, id+" lets Smallest be chosen and the engine refuses smallest for it at its defaults")
		}
	}
	slices.Sort(out)
	return out
}
