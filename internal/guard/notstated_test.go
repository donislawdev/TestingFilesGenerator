package guard

import (
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
)

// A menu can say that nobody stated anything, and can be put back to saying it.
//
// Being able to say nothing is what lets the manifest record a value as
// defaulted rather than chosen, which this window promises and the command line
// gets for free by the flag simply being absent. A text box says it by being
// empty. A menu had no way to: the toolkit paints its placeholder in the
// ordinary foreground colour, so a default nobody picked was drawn exactly like
// a choice somebody made, and once the menu had been moved off it there was no
// entry to move back to (O104).
//
// The value is asked of the field rather than read off the control, because the
// value is the half that reaches the manifest. A menu showing the right words
// while sending them onward as a chosen value would be the same defect wearing
// a label.
func TestAMenuLeftAloneSaysNothingWasChosen(t *testing.T) {
	checked := 0
	for _, p := range everyClosedSetWithADefault(t) {
		t.Run(p.Name, func(t *testing.T) {
			checked++
			field := parts.FromProperty(p)

			menu, ok := field.Control.(*parts.Chooser)
			if !ok {
				t.Fatalf("%q is a closed set and the window draws %T for it", p.Name, field.Control)
			}

			notStated := text.ChoiceLeftAlone(p.Default)
			if len(menu.Options) == 0 || menu.Options[0] != notStated {
				t.Fatalf("the menu for %q does not offer %q as its first entry, so there is no way to "+
					"say the setting was left alone.\nIt offers %v", p.Name, notStated, menu.Options)
			}
			if menu.Selected != notStated {
				t.Errorf("the menu for %q starts on %q rather than on %q, so a fresh screen claims a "+
					"choice nobody made.", p.Name, menu.Selected, notStated)
			}
			if got := field.Value(); got != "" {
				t.Errorf("the menu for %q was left alone and reports %q, so the manifest would record "+
					"a value somebody never chose.", p.Name, got)
			}

			// Chosen, and then put back. The way back is the half a menu never
			// had.
			menu.SetSelected(p.Default)
			if got := field.Value(); got != p.Default {
				t.Errorf("the menu for %q was set to %q and reports %q", p.Name, p.Default, got)
			}
			menu.SetSelected(notStated)
			if got := field.Value(); got != "" {
				t.Errorf("the menu for %q was put back to %q and still reports %q, so the setting "+
					"cannot be unstated once it has been touched.", p.Name, notStated, got)
			}
		})
	}
	if checked == 0 {
		t.Fatal("no closed set with a default was examined - this guard would pass without checking anything")
	}
}

// everyClosedSetWithADefault is every menu on either screen that has a declared
// default, gathered from the registries rather than from a list written here.
//
// From the source rather than by hand because that is this project's standing
// lesson about completeness audits: a list somebody typed cannot find what they
// left out of it. The preset screen's format setting is included and is the one
// the interface audit was about.
func everyClosedSetWithADefault(t *testing.T) []format.Property {
	t.Helper()

	var out []format.Property
	seen := map[string]bool{}
	add := func(p format.Property) {
		if p.Kind != format.PropertyChoice || p.Default == "" || seen[p.Name] {
			return
		}
		seen[p.Name] = true
		out = append(out, p)
	}

	for _, d := range format.All() {
		for _, p := range d.Properties {
			add(p)
		}
	}
	for _, p := range preset.All() {
		for _, param := range append(append([]format.Property{}, p.Parameters...), p.Globals()...) {
			add(param)
		}
	}
	return out
}
