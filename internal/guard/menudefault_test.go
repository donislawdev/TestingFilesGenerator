package guard

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// A menu offers what the declaration lists, and opens on the declared default.
//
// This replaces TestAMenuLeftAloneSaysNothingWasChosen, which held the opposite
// promise: that a menu carries an extra first entry reading "not stated - a4",
// so a default nobody picked could be told from a value somebody chose. The
// reason written on that guard was untouchable rule 5 - the manifest records
// WHICH values were ours - and the reason was never checked against a manifest.
//
// Measured on 2026-08-27, at both ends, and neither end wanted it:
//
//   - A format setting. An ICO run leaving embed alone and one asking for
//     embed=bmp give the same bytes and the same manifest, which writes
//     embed: bmp in both of them. The word defaulted is in neither.
//   - A preset setting. Defaulted is built from the parameters a preset
//     declares, and the format a preset is built in is a global flag rather
//     than one of those, so a run that never states it records
//     defaulted: [spread] and never mentions format at all.
//
// So the entry was a third value between two that already did the same thing.
// Removed by the owner's decision. What is guarded instead is the promise that
// replaced it, and it has two halves that fail differently: a menu offering
// something the declaration does not list would let somebody ask for a value
// the engine refuses, and a menu opening on anything but the default would
// silently change what a fresh screen produces.
//
// Boxes somebody types in are a different case and keep the old promise -
// TestABoxYouMayLeaveAloneSaysSo is the guard for those.
func TestAMenuOpensOnItsDeclaredDefault(t *testing.T) {
	checked := 0
	for _, p := range everyDeclaredMenu() {
		p := p
		t.Run(p.Name, func(t *testing.T) {
			field := parts.FromProperty(p)
			menu, ok := field.Control.(*parts.Chooser)
			if !ok {
				t.Fatalf("%q is a closed set and the window draws %T for it", p.Name, field.Control)
			}
			checked++

			if got, want := strings.Join(menu.Options, ","), strings.Join(p.Choices, ","); got != want {
				t.Errorf("the menu for %q offers %s and the declaration lists %s", p.Name, got, want)
			}

			if p.Default == "" {
				// Nothing declared, so nothing chosen. The format works its own
				// answer out and the placeholder says so.
				if menu.Selected != "" {
					t.Errorf("%q declares no default and its menu opens on %q, which is a choice nobody made",
						p.Name, menu.Selected)
				}
				return
			}

			if menu.Selected != p.Default {
				t.Errorf("%q declares the default %q and its menu opens on %q",
					p.Name, p.Default, menu.Selected)
			}
			// Asked of the FIELD rather than read off the control, because the
			// value is the half that reaches the engine. A menu showing the
			// right word while sending something else onward would be a defect
			// wearing a label.
			if got := field.Value(); got != p.Default {
				t.Errorf("the menu for %q shows %q and sends %q", p.Name, menu.Selected, got)
			}
		})
	}
	if checked == 0 {
		t.Fatal("no menu was examined - this guard would pass without checking anything")
	}
	t.Logf("%d menu(s) open on their declared default", checked)
}

// everyDeclaredMenu is every closed set either surface draws, with and without
// a declared default.
//
// Both kinds, deliberately. The old guard looked only at settings that declare
// a default, because the entry it was about only existed for those - which left
// the menus that declare none unwatched by anything asking what they offer.
func everyDeclaredMenu() []format.Property {
	var out []format.Property
	seen := map[string]bool{}
	add := func(p format.Property) {
		if p.Kind != format.PropertyChoice || seen[p.Name] {
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

// Every menu offering the whole list of formats draws the file kind pictures.
//
// There were three such menus on 2026-08-27 and one of them had the pictures:
// the format of a batch. The list inside "Add files inside" and the format a
// preset is built in were drawn plain, so the same twenty values looked like
// different kinds of list depending on which tab somebody was standing on.
// Reported by the owner from the running window.
//
// Asked of the SCREENS rather than of the constructor, and that is the whole
// value of it. The rule now lives in parts.NewChooser, and a guard calling that
// directly would prove the rule and say nothing about whether a screen went
// through it - which is exactly the state this defect was in, since the batch
// screen assigned the pictures by hand and the other two forgot.
func TestEveryMenuOfferingEveryFormatDrawsTheKindPictures(t *testing.T) {
	host := newFakeHost(t)
	window.Open(host)

	look := func(where string, root fyne.CanvasObject) int {
		found := 0
		walk(root, func(o fyne.CanvasObject) {
			menu, ok := o.(*parts.Chooser)
			if !ok || !parts.IsEveryFormat(menu.Options) {
				return
			}
			found++
			if menu.KindOf == nil {
				t.Errorf("a menu of formats on %s draws no pictures, and the one on %s does",
					where, text.TabRecipe())
				return
			}
			for _, id := range menu.Options {
				if menu.KindOf(id) == nil {
					t.Errorf("the menu of formats on %s draws nothing for %q", where, id)
				}
			}
		})
		return found
	}

	// One per screen, counted per screen. A total would let a screen with none
	// hide behind a screen with two, which is the shape of the defect itself -
	// two menus were missed for twenty days while a third had the pictures.
	for _, tab := range []string{text.TabOneTarget(), text.TabRecipe(), text.TabPresets()} {
		if n := look(tab, tabNamed(t, host.content, tab)); n != 1 {
			t.Errorf("the %s screen has %d menu(s) offering every format and this guard expects 1", tab, n)
		}
	}

	// The fourth is not on any screen until somebody asks for it: the list of
	// what an archive holds is drawn by pressing a button, and since 2026-08-27
	// that button is only under a format that holds files. Two are expected
	// here, since this batch screen still has the batch's own format menu on it.
	screen := window.NewRecipe(newFakeHost(t))
	body := screen.Object()
	chooserIn(t, screen.Fields(), recipe.TargetAddress(1, recipe.KeyFormat)).SetSelected("zip")
	pressNamed(t, body, text.ButtonAddContents())
	if n := look(text.ContentsHeading(), body); n != 2 {
		t.Errorf("a batch holding files has %d menu(s) offering every format and this guard expects 2 - "+
			"the batch's own and the one row inside it", n)
	}
}
