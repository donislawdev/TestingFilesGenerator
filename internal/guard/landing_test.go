package guard

import (
	"testing"

	"fyne.io/fyne/v2"

	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
)

// Exactly one preset declares itself the one a surface opens on.
//
// Before 2026-09-22 both screens opened on the first id in order, which is
// alphabetical - so what somebody saw when they opened the Presets tab was
// decided by whoever wrote a preset next. It changed on the day
// empty-and-minimal arrived, and nine window guards went red at once, none of
// them reporting anything more useful than a field being nil. The catalogue in
// docs/PRESETS.md has fourteen entries, so that was going to keep happening.
//
// Two presets claiming it is a panic at registration rather than a failure
// here, the same way two presets of one id are. What this catches is the other
// end: nobody claiming it, which is silent - preset.Landing falls back to the
// first id, and the fallback exists so that a window still opens rather than so
// that anybody relies on it.
func TestExactlyOnePresetOpensTheWindow(t *testing.T) {
	all := preset.All()
	if len(all) == 0 {
		t.Fatal("no preset is registered, so this guard checked nothing")
	}

	claimed := make([]string, 0, 1)
	for _, p := range all {
		if p.Landing {
			claimed = append(claimed, p.ID)
		}
	}
	switch len(claimed) {
	case 1:
	case 0:
		t.Errorf("no preset says it is the one to open on, so the window falls back to %q - "+
			"the first id in alphabetical order, which is whichever preset gets written next",
			preset.Landing())
	default:
		t.Errorf("%v all say they are the one to open on, and only one can", claimed)
	}

	if got := preset.Landing(); len(claimed) == 1 && got != claimed[0] {
		t.Errorf("%s declares itself the one to open on and the window is told to open on %q",
			claimed[0], got)
	}
}

// Both screens that offer presets open on the declared one.
//
// Asked of the SCREENS rather than of preset.Landing, which is the whole value
// of it: the declaration being right says nothing about whether a screen went
// through it, and the two screens reached for the first id separately. A guard
// calling Landing directly would agree with Landing and prove nothing.
func TestBothScreensOpenOnTheDeclaredPreset(t *testing.T) {
	want := preset.Landing()
	if want == "" {
		t.Fatal("this build registers no preset, so neither screen has one to open on")
	}

	host := newFakeHost(t)
	window.Open(host)
	if host.content == nil {
		t.Fatal("opening the window put no screen in it")
	}
	t.Cleanup(func() { join(host) })

	presets := selectTab(t, host.content, text.TabPresets())
	if got := chooserUnder(t, presets, text.FieldPreset()).Selected; got != want {
		t.Errorf("the presets screen opens on %q and %q is the declared one", got, want)
	}

	// The batch screen draws its preset menu only once the switch is on, so it
	// is turned on the way a press turns it on.
	batches := selectTab(t, host.content, text.TabRecipe())
	switchOn := checkNamed(batches, text.FieldBuildOnPreset())
	if switchOn == nil {
		t.Fatal("there is no switch to build on a preset on the batch screen")
	}
	switchOn.SetChecked(true)
	if got := basePresetOn(t, batches); got != want {
		t.Errorf("the batch screen opens on %q and %q is the declared one", got, want)
	}
}

// basePresetOn is the preset chosen in the batch screen's base section.
//
// Read through the tree, because the control registered under the recipe key is
// the menu inside its width wrapper.
func basePresetOn(t *testing.T, screen fyne.CanvasObject) string {
	t.Helper()
	control := controlUnder(screen, text.FieldBasePreset())
	menu, ok := control.(*parts.Chooser)
	if !ok {
		t.Fatalf("the base preset field is %T rather than a list to choose from", control)
	}
	return menu.Selected
}
