package guard

import (
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
)

// A closed set of values is in an order somebody can look a value up in.
//
// Asked for on 2026-08-12 and worth more than it sounds: a menu whose order is
// whatever the declaration happened to be written in is a menu somebody reads
// from top to bottom every time, and the pdf paper sizes were a4, a3, a5,
// letter, legal.
//
// The sort happens at registration rather than where the values are drawn, and
// that is what this guard is really about. The same set appears in a menu, in
// the line "tfg formats pdf" prints and in the wording of a refusal, so sorting
// it in the window would leave the other two in the declaration's order - two
// surfaces describing one format two ways, which is where D1 comes apart
// without anybody noticing.
func TestEveryClosedSetIsRegisteredInOrder(t *testing.T) {
	checked := 0
	for _, d := range format.All() {
		for _, p := range d.Properties {
			if len(p.Choices) == 0 {
				continue
			}
			checked++
			if got, want := p.Choices, inOrder(p.Choices); strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("%s.%s offers %v, and in order that is %v", d.ID, p.Name, got, want)
			}
		}
	}
	for _, p := range preset.All() {
		// Both kinds of setting a preset puts on a screen. Parameters are its
		// own and Globals are the flags it reads, and only the first go through
		// Register - so only the first are sorted by anything. The file kind
		// list on the preset screen is a Global, it was photographed for the
		// first time on 2026-08-18, and until that day nothing here looked at
		// it: it comes out in order because format.IDs happens to sort, which
		// is a coincidence rather than a rule, and a second global built any
		// other way would arrive unsorted with everything still green.
		for _, param := range append(append([]format.Property{}, p.Parameters...), p.Globals()...) {
			if len(param.Choices) == 0 {
				continue
			}
			checked++
			if got, want := param.Choices, inOrder(param.Choices); strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("preset %s.%s offers %v, and in order that is %v", p.ID, param.Name, got, want)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no closed set was examined - this guard would pass without checking anything")
	}
	t.Logf("%d closed set(s), each in the order it is registered in", checked)
}

// Numbers sort as numbers, which is the one place the plain answer is wrong.
//
// Named rather than left to the guard above, because that one compares the
// registry against the same function the registry used - so it proves the sort
// was applied and says nothing about what the sort does. This says what it
// does, and it is the case a plain string sort gets wrong: the WAV bit depths
// come out 16, 24, 32, 8, which is alphabetical and unreadable.
func TestAClosedSetOfNumbersIsInNumericOrder(t *testing.T) {
	depths := []string{"32", "8", "24", "16"}
	format.SortChoices(depths)
	if got := strings.Join(depths, ","); got != "8,16,24,32" {
		t.Errorf("a set of numbers came out as %s", got)
	}

	words := []string{"tone", "silence", "noise", "sweep"}
	format.SortChoices(words)
	if got := strings.Join(words, ","); got != "noise,silence,sweep,tone" {
		t.Errorf("a set of words came out as %s", got)
	}

	// A set that is only partly numbers has no numeric order to be put in, so
	// it goes alphabetically like any other words. Here because "all of them"
	// is the condition and a guard that never sees a mixed set cannot tell that
	// condition from "any of them".
	mixed := []string{"9", "auto", "1"}
	format.SortChoices(mixed)
	if got := strings.Join(mixed, ","); got != "1,9,auto" {
		t.Errorf("a mixed set came out as %s", got)
	}
}

// What the menu offers is what the declaration says, in that order.
//
// The other half of the sort, and the half that cannot be seen from either side
// alone: the registry is in order and a window is free to draw a menu from its
// own copy. Measured through the tree rather than by reading the code, because
// the failure this catches is a menu built from a list somebody typed.
func TestAMenuOffersItsChoicesInTheDeclaredOrder(t *testing.T) {
	_, content := screen(t)
	picker := controlUnder(content, text.FieldFormat).(*parts.Chooser)

	checked := 0
	for _, d := range format.All() {
		picker.SetSelected(d.ID)
		for _, p := range d.Properties {
			if len(p.Choices) == 0 {
				continue
			}
			menu, ok := controlUnder(content, p.Name).(*parts.Chooser)
			if !ok {
				t.Errorf("%s.%s is a closed set and the window draws %T for it",
					d.ID, p.Name, controlUnder(content, p.Name))
				continue
			}
			checked++
			if got, want := strings.Join(menu.Options, ","), strings.Join(p.Choices, ","); got != want {
				t.Errorf("the menu for %s.%s offers %s and the declaration says %s", d.ID, p.Name, got, want)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no menu was examined - this guard would pass without checking anything")
	}
	t.Logf("%d menu(s) drawn in the order their declaration lists", checked)
}

// The two lists somebody chooses a screen's subject from are in order too.
//
// Formats and presets come from registries keyed by id, and a map hands its
// keys back in whatever order it likes. Both registries sort already and
// neither says out loud that a screen depends on it.
func TestTheFormatAndPresetMenusAreInOrder(t *testing.T) {
	_, generate := screen(t)
	if got := controlUnder(generate, text.FieldFormat).(*parts.Chooser).Options; !sorted(got) {
		t.Errorf("the format menu offers %v", got)
	}

	_, presets := presetScreen(t)
	if got := controlUnder(presets, text.FieldPreset).(*parts.Chooser).Options; !sorted(got) {
		t.Errorf("the preset menu offers %v", got)
	}
}

func sorted(values []string) bool {
	return strings.Join(values, ",") == strings.Join(inOrder(values), ",")
}

// inOrder is a copy of a set, sorted. A copy because the sort works in place
// and the caller is holding the registry's own slice.
func inOrder(values []string) []string {
	out := append([]string{}, values...)
	format.SortChoices(out)
	return out
}
