package guard

import (
	"fmt"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// EVERY box on a screen can be told it is the one that was refused.
//
// This is the guard that answers "what about the fiftieth field", and it exists
// because the answer used to be "somebody has to remember". Marking a refused
// box arrived on 2026-08-12 wired field by field: a screen called one function
// for the fields it thought could be refused and another for the rest, so
// whether a box could be marked was a property of what somebody typed rather
// than of what a field IS. Measured that day on the generate screen - five
// fields could carry a refusal and three could not, and every setting a format
// declares was in the second group. PNG declares two of those, TAR.GZ three.
//
// It compares the tree against the registry rather than checking the registry
// against itself. That direction is the whole guard: walking the fields
// somebody registered cannot find the control they never registered, which is
// the same lesson as TestEveryFormatIsClassifiedAsTextOrBinary and is written
// down in docs/UX.md section 7.0.
func TestEveryBoxOnAScreenIsOneThatCanBeMarked(t *testing.T) {
	host := &fakeHost{}
	generate := window.NewGenerate(host)
	presets := window.NewPreset(host)

	for _, s := range []struct {
		name    string
		object  fyne.CanvasObject
		fields  *parts.Fields
		screens int
	}{
		{name: "the generate screen", object: generate.Object(), fields: generate.Fields()},
		{name: "the preset screen", object: presets.Object(), fields: presets.Fields()},
	} {
		// Everything a registered field holds, not only the object it was given.
		// A field's control is sometimes a wrapper - a box held to the width of
		// a number, or a path box with a Choose button beside it - and the thing
		// somebody types into is inside it.
		known := map[fyne.CanvasObject]string{}
		for _, f := range s.fields.All() {
			if f.Setting == "" {
				t.Errorf("%s: the field %q has no setting, so no refusal can ever name it", s.name, f.Label)
			}
			walk(f.Control, func(obj fyne.CanvasObject) { known[obj] = f.Setting })
		}

		onScreen := inputsIn(s.object)
		if len(onScreen) == 0 {
			t.Fatalf("%s: no control was found - this guard would pass without checking anything", s.name)
		}
		for _, control := range onScreen {
			if _, ok := known[control]; !ok {
				t.Errorf("%s: %s is on the screen and not in the registry, so a refusal about it "+
					"lands at the foot of the form with nothing marked", s.name, controlName(control))
			}
		}
		t.Logf("%s: %d control(s) on screen, %d field(s) registered", s.name, len(onScreen), len(s.fields.All()))
	}
}

// A registered field is one a refusal can actually reach.
//
// The guard above proves every box is in the registry. This proves the registry
// does something: a screen could register every field and lose the message on
// the way, which looks identical from outside and is the shape a window ends up
// in when the wiring is right and the lookup is not.
//
// Every field of both screens, rather than a sample, because the failure this
// catches is per field.
func TestEveryRegisteredFieldCanBeMarkedAndUnmarked(t *testing.T) {
	host := &fakeHost{}
	for _, s := range []struct {
		name   string
		fields *parts.Fields
	}{
		{"the generate screen", window.NewGenerate(host).Fields()},
		{"the preset screen", window.NewPreset(host).Fields()},
	} {
		for _, f := range s.fields.All() {
			const refusal = "this is the message"
			if !s.fields.Mark(f.Setting, refusal) {
				t.Errorf("%s: %q is registered under %q and marking it found nothing",
					s.name, f.Label, f.Setting)
				continue
			}
			if f.Saying() != refusal {
				t.Errorf("%s: marking %q put %q under a different field", s.name, f.Setting, refusal)
			}
			s.fields.ClearAll()
			if f.Saying() != "" {
				t.Errorf("%s: %q still says %q after everything was cleared",
					s.name, f.Setting, f.Saying())
			}
		}
	}
}

// The settings a format declares are among the fields that can be marked, for
// every format this build has.
//
// Separate from the two above because these fields come and go: choosing a
// format throws the previous one's away and draws the new one's, so the
// registry is rebuilt on every change. A screen that registered them once and
// then forgot to on the second change would pass both guards above, which only
// ever look at the format the screen opens on.
func TestEveryFormatsOwnSettingsCanBeMarked(t *testing.T) {
	host := &fakeHost{}
	generate := window.NewGenerate(host)
	picker, ok := controlUnder(generate.Object(), "Format").(*parts.Chooser)
	if !ok {
		t.Fatal("there is no format menu, so this guard read the wrong tree")
	}

	checked := 0
	for _, d := range format.All() {
		picker.SetSelected(d.ID)
		known := map[string]bool{}
		for _, f := range generate.Fields().All() {
			known[f.Setting] = true
		}
		for _, p := range d.Properties {
			checked++
			if !known[p.Name] {
				t.Errorf("%s declares %s and the screen showing it cannot mark that box", d.ID, p.Name)
			}
		}
		// And the previous format's settings are gone rather than left behind as
		// places a refusal could still be sent.
		for _, other := range format.All() {
			if other.ID == d.ID {
				continue
			}
			for _, p := range other.Properties {
				if declaredBy(d, p.Name) {
					continue
				}
				if known[p.Name] {
					t.Errorf("%s is showing and %s.%s is still registered, so a refusal could be "+
						"sent to a box that is not on the screen", d.ID, other.ID, p.Name)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no declared setting was examined - this guard would pass without checking anything")
	}
	t.Logf("%d declared setting(s) across %d formats, each one markable", checked, len(format.All()))
}

func declaredBy(d format.Descriptor, name string) bool {
	for _, p := range d.Properties {
		if p.Name == name {
			return true
		}
	}
	return false
}

// inputsIn is every control somebody puts a value into.
//
// By type rather than by position, and the list is the three kinds this window
// builds. A fourth kind arrives as a compile error here rather than as a field
// nobody can mark, which is the point.
func inputsIn(o fyne.CanvasObject) []fyne.CanvasObject {
	var out []fyne.CanvasObject
	walk(o, func(obj fyne.CanvasObject) {
		switch obj.(type) {
		case *widget.Entry, *parts.Chooser, *widget.Check:
			out = append(out, obj)
		}
	})
	return out
}

func controlName(o fyne.CanvasObject) string {
	switch v := o.(type) {
	case *widget.Entry:
		return fmt.Sprintf("the box holding %q", v.Text)
	case *parts.Chooser:
		return fmt.Sprintf("the menu showing %q", v.Selected)
	case *widget.Check:
		return fmt.Sprintf("the switch %q", v.Text)
	}
	return fmt.Sprintf("a %T", o)
}
