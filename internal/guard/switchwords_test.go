package guard

import (
	"regexp"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// A switch is said as on or off wherever the window puts one into words -
// docs/SIZE-FIXES-2026-11-16.md section 6.3. A recipe writes true and false,
// and the folded settings of a placeholder png said "Guides true" over a
// switch that shows neither word, with "true or false, default false" behind
// its button.

// recipeBool is true or false standing as a word of its own, the way a recipe
// writes a switch. Asked only of lines made of labels and values, which have
// no other reason to carry either word.
var recipeBool = regexp.MustCompile(`\b(true|false)\b`)

// TestTheWindowSaysASwitchAsOnOrOff asks every switch anything declares, in the
// two places the window says one in words: the explanation behind its button,
// which says what it starts on, and the line its folded section shows once it
// has been pressed. Then the one screen the defect was seen on, read off the
// screen, so a window that stopped asking parts.SettingsSaid is caught too.
func TestTheWindowSaysASwitchAsOnOrOff(t *testing.T) {
	if got := text.AllowedWholeNumber(); got != "whole number" {
		t.Fatalf("this process speaks another language (%q), so nothing here is English to ask", got)
	}
	on, off := text.SwitchState(true), text.SwitchState(false)
	if on == off || recipeBool.MatchString(on+" "+off) {
		t.Fatalf("a switch is said as %q and %q, so there is no on and off to look for", on, off)
	}
	// The canary: the check has to see the line this guard was written against,
	// and has to let the line it asks for through.
	if !recipeBool.MatchString(text.FoldedSummary("Content placeholder", "Guides true")) {
		t.Fatal("the check does not see \"Guides true\", so a green below would mean nothing")
	}
	if recipeBool.MatchString(text.SettingSaid("Guides", on)) {
		t.Fatalf("the check refuses %q, the line it asks for", text.SettingSaid("Guides", on))
	}

	asked := 0
	for _, d := range declarations() {
		fields := make([]parts.PropertyField, 0, len(d.props))
		for _, p := range d.props {
			fields = append(fields, parts.FromProperty(p))
		}
		for i, p := range d.props {
			if p.Kind != format.PropertyBool {
				continue
			}
			where := string(d.owner) + " " + p.Name
			detail := parts.PropertyDetail(d.owner, p)
			if strings.Contains(detail, text.AllowedTrueOrFalse()) {
				t.Errorf("%s: the explanation of a switch says %q:\n%s", where, text.AllowedTrueOrFalse(), detail)
			}
			if starts := text.AllowedDefault(text.SwitchState(p.Default == "true")); p.Default != "" && !strings.Contains(detail, starts) {
				t.Errorf("%s: the explanation does not say %q:\n%s", where, starts, detail)
			}

			toggle, isSwitch := fields[i].Control.(*parts.Toggle)
			if !isSwitch {
				t.Errorf("%s: declared true or false and drawn as %T rather than a switch", where, fields[i].Control)
				continue
			}
			meetCondition(t, where, fields, p.OnlyWhen)
			toggle.SetChecked(!toggle.Checked)
			said := parts.SettingsSaid(fields)
			want := text.SettingSaid(text.SettingLabel(p.Name), text.SwitchState(toggle.Checked))
			if !strings.Contains(said, want) || recipeBool.MatchString(said) {
				t.Errorf("%s: pressed, the folded section says %q, where it should say %q", where, said, want)
			}
			toggle.SetChecked(!toggle.Checked)
			asked++
		}
	}
	if asked < 15 {
		t.Fatalf("asked about %d switch(es), so the registries were not read", asked)
	}

	png, err := format.Get("png")
	if err != nil {
		t.Fatal(err)
	}
	screen := window.NewGenerate(newFakeHost(t))
	chooserIn(t, screen.Fields(), engine.SettingFormat).SetSelected(png.ID)
	settings := text.SettingsFor(png.ID)
	openFold(t, screen.Object(), "", settings)
	chooserIn(t, screen.Fields(), "content").SetSelected("placeholder")
	guides := toggleIn(t, screen.Fields(), "guides")
	guides.SetChecked(true)
	foldTitled(t, screen.Object(), "", settings).Tapped(nil)
	said := shownText(screen.Object())
	want := text.SettingSaid(text.SettingLabel("guides"), on)
	if !strings.Contains(said, want) {
		t.Errorf("the folded settings of a placeholder png with its guides on do not say %q:\n%s", want, said)
	}
}

// meetCondition sets the setting a switch depends on to the first value under
// which it applies, so the folded line has a reason to name it.
func meetCondition(t *testing.T, where string, fields []parts.PropertyField, c format.Condition) {
	t.Helper()
	if c.IsZero() {
		return
	}
	for _, f := range fields {
		if f.Name != c.Setting {
			continue
		}
		menu, isMenu := f.Control.(*parts.Chooser)
		if !isMenu || len(c.Values) == 0 {
			t.Fatalf("%s applies only when %s, which this guard cannot set", where, c.Describe())
		}
		menu.SetSelected(c.Values[0])
		return
	}
	t.Fatalf("%s applies only when %s, and nothing beside it declares %s", where, c.Describe(), c.Setting)
}
