package guard

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/driver/desktop"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// A setting a format declares is named the way every other field is named.
//
// A format declares its settings under the key a recipe writes - width,
// bit_depth, entry_size - and until 2026-08-20 the window put that key on the
// screen as the label. Two naming systems sharing one visual style: Format,
// Size, How many, Group name on one side and settings for bmp, width,
// bit_depth on the other, in the same weight at the same size. Half the labels
// on the screen looked written and half looked leaked, and nothing said which
// was which.
//
// It looks for the key ON THE SCREEN rather than checking what SettingLabel
// returns. Asking the function proves the function, and the defect was never in
// the function - it was a screen calling something else.
func TestNoSettingWearsItsRecipeKeyAsALabel(t *testing.T) {
	generate, choose, _ := formatsLaidOut(t)

	checked := 0
	for _, d := range format.All() {
		choose.to(d.ID)
		for _, p := range d.Properties {
			label := text.SettingLabel(p.Name)
			if label == p.Name {
				t.Fatalf("%s.%s is spelled the same either way, so this guard cannot tell the two apart", d.ID, p.Name)
			}
			if controlUnder(generate, label) == nil {
				t.Errorf("%s declares %q and no field on the screen is called %q", d.ID, p.Name, label)
			}
			if controlUnder(generate, p.Name) != nil {
				t.Errorf("%s.%s is on the screen under its recipe key rather than under %q", d.ID, p.Name, label)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no format declares a setting, so this guard checked nothing")
	}
	t.Logf("%d declared setting(s), none of them showing a recipe key as its label", checked)
}

// And the key is still one hover away.
//
// This is what makes the label change a translation rather than a loss. The
// window and the recipe file are two ways into one engine, so somebody who
// finds a setting on the screen has to be able to write it down - and the key
// is the only spelling that works there. Before this, the key was on the screen
// because it WAS the label, so nothing had to carry it.
func TestTheRecipeKeyOfASettingIsStillReachable(t *testing.T) {
	generate, choose, _ := formatsLaidOut(t)

	checked := 0
	for _, d := range format.All() {
		choose.to(d.ID)
		for _, p := range d.Properties {
			label := text.SettingLabel(p.Name)
			button := detailButtonBeside(generate, label)
			if button == nil {
				t.Errorf("%s.%s has no way to open its longer explanation, so its recipe key is nowhere", d.ID, p.Name)
				continue
			}
			// Pointed at rather than read out of the widget, so what is
			// asserted is that the words reach the screen.
			button.MouseIn(&desktop.MouseEvent{})
			shown := allText(generate)
			button.MouseOut()
			if !strings.Contains(shown, text.SettingKey(p.Name)) {
				t.Errorf("hovering the button beside %q did not put the recipe key %q on the screen",
					label, p.Name)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no format declares a setting, so this guard checked nothing")
	}
}
