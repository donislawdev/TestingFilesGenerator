package guard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// The window offers every damage the registry holds.
//
// Asked of the registry rather than of a list written here, so a second damage
// appears in this menu on the day it is registered - the same bargain the
// format menu makes, and the reason D1 is answered by declaring rather than by
// remembering.
func TestTheWindowOffersEveryDamageThereIs(t *testing.T) {
	_, content := screen(t)

	control := controlUnder(content, text.FieldDamage())
	picker, ok := control.(*parts.Chooser)
	if !ok {
		t.Fatalf("the damage field is %T rather than a list to choose from", control)
	}

	offered := append([]string{}, picker.Options...)
	want := append([]string{text.DamageNone()}, damage.Names()...)
	sort.Strings(offered)
	sort.Strings(want)
	if strings.Join(offered, ",") != strings.Join(want, ",") {
		t.Errorf("the window offers %v and the registry has %v", offered, want)
	}

	// It opens on none, because damage is the exception rather than the
	// ordinary run - and because a menu cannot be empty, so "not damaged" has
	// to be one of its values rather than the absence of a choice.
	if picker.Selected != text.DamageNone() {
		t.Errorf("the window opens with %q chosen rather than %q", picker.Selected, text.DamageNone())
	}
}

// Choosing a damage draws the fields it declares, and choosing none takes them
// away again.
//
// The second half is the one worth having. A field left behind under a menu
// set back to none is a value that reaches the engine from a control nobody
// can see, which is how a preset parameter once travelled without a widget.
func TestTheWindowDrawsAFieldForEveryDamageParameter(t *testing.T) {
	_, content := screen(t)
	picker := controlUnder(content, text.FieldDamage()).(*parts.Chooser)

	checked := 0
	for _, d := range damage.All() {
		picker.SetSelected(d.ID)
		for _, p := range d.Parameters {
			control := controlUnder(content, text.SettingLabel(p.Name))
			if control == nil {
				t.Errorf("%s declares %q and the window draws no field for it", d.ID, p.Name)
				continue
			}
			if bad := wrongKindOfControl(p, control); bad != "" {
				t.Errorf("%s.%s is %s", d.ID, p.Name, bad)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no damage declares a parameter, so this proved nothing")
	}

	picker.SetSelected(text.DamageNone())
	for _, d := range damage.All() {
		for _, p := range d.Parameters {
			if controlUnder(content, text.SettingLabel(p.Name)) != nil {
				t.Errorf("%q is still on the screen with no damage chosen", p.Name)
			}
		}
	}
	t.Logf("%d damage parameter(s) drawn from the registry", checked)
}

// Choosing a damage again, or a format, leaves the registry of fields holding
// what the screen draws and nothing else.
//
// The registry is what a refusal is addressed against and what a run freezes,
// so a field the screen no longer draws is a place a refusal could still go,
// under a box nobody can see. Every choice of a damage used to leave one
// behind - its parameters were added and never taken away - so the count is
// read after many choices rather than one: one stale field reads like the cost
// of the screen, twenty read like what they are (R18 of the stability review).
// Then the same with the format changed in between, because the parameters sit
// after the format's fields, and a mark that did not move with the format
// would cut the format's fields instead of the damage's.
func TestChoosingADamageAgainLeavesNoFieldBehind(t *testing.T) {
	host := newFakeHost(t)
	gen := window.NewGenerate(host)
	content := gen.Object()
	t.Cleanup(func() { join(host) })
	fields := gen.Fields()
	picker, ok := controlUnder(content, text.FieldDamage()).(*parts.Chooser)
	if !ok {
		t.Fatal("the damage field is not a list to choose from")
	}
	var chosen damage.Descriptor
	for _, d := range damage.All() {
		if len(d.Parameters) > 0 {
			chosen = d
			break
		}
	}
	if chosen.ID == "" {
		t.Fatal("no damage declares a parameter, so this proved nothing")
	}

	// The formats that declare the fewest and the most settings, asked of the
	// registry: two formats with as many fields each would pass a mark that
	// stayed where the first one left it.
	formats := chooserIn(t, fields, "format")
	declared := map[string]int{}
	var few, many string
	for _, id := range formats.Options {
		d, err := format.Get(id)
		if err != nil {
			t.Fatalf("the format menu offers %q: %v", id, err)
		}
		declared[id] = len(d.Properties)
		if few == "" || declared[id] < declared[few] {
			few = id
		}
		if many == "" || declared[id] > declared[many] {
			many = id
		}
	}
	if declared[few] == declared[many] {
		t.Fatalf("every format declares %d settings, so a mark that stayed with the first would pass", declared[few])
	}

	counts := map[string]int{}
	for _, f := range []string{few, many} {
		// None before the format rather than after it: a change of format
		// takes every parameter away, so this is a count no stale field can
		// be part of, whatever the screen does with a choice of none.
		picker.SetSelected(text.DamageNone())
		formats.SetSelected(f)
		without := fields.Len()
		picker.SetSelected(chosen.ID)
		counts[f] = fields.Len()
		if counts[f] <= without {
			t.Fatalf("choosing %s with %s added no field (%d before and after), so this proved nothing", chosen.ID, f, without)
		}
	}
	for i := 0; i < 20; i++ {
		picker.SetSelected(text.DamageNone())
		picker.SetSelected(chosen.ID)
	}
	if got := fields.Len(); got != counts[many] {
		t.Errorf("after twenty more choices of %s the registry holds %d fields, and %d after the first", chosen.ID, got, counts[many])
	}
	for i := 0; i < 20; i++ {
		formats.SetSelected([]string{few, many}[i%2])
		picker.SetSelected(text.DamageNone())
		picker.SetSelected(chosen.ID)
	}
	if got := fields.Len(); got != counts[many] {
		t.Errorf("after twenty changes of format the registry holds %d fields, and %d after the first", got, counts[many])
	}
	drawn := map[fyne.CanvasObject]bool{}
	walk(content, func(o fyne.CanvasObject) { drawn[o] = true })
	for i, control := range fields.Controls() {
		if !drawn[control] {
			t.Errorf("the registry holds a field for %q the screen does not draw", fields.All()[i].Setting)
		}
	}
	// And the other way round, for what the two choices declare: a box on the
	// screen with no field behind it is a box whose refusal has nowhere to go,
	// which is what a mark that cut the format's fields would leave.
	d, err := format.Get(many)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range settingNames(d.Properties, chosen.Parameters) {
		field := fields.Lookup(name)
		if field == nil || !drawn[field.Control] {
			t.Errorf("%q of %s with %s is drawn without a field in the registry behind it", name, many, chosen.ID)
		}
	}
}

// settingNames is the name of every setting in the lists given, in order.
func settingNames(lists ...[]format.Property) []string {
	var out []string
	for _, list := range lists {
		for _, p := range list {
			out = append(out, p.Name)
		}
	}
	return out
}

// A run started from the window is damaged, and the bytes say so.
//
// Pressed rather than settled, because settle is the window's own and a guard
// that called it would prove the screen agrees with itself. Read from the file
// rather than from the screen: a window that draws the menu and drops the
// choice on the way to the engine looks exactly like one that works, which is
// the defect that once let the window produce PDFs while the command line
// produced anything.
func TestARunFromTheWindowIsReallyDamaged(t *testing.T) {
	dir := t.TempDir()

	host := newFakeHost(t)
	gen := window.NewGenerate(host)
	content := gen.Object()
	t.Cleanup(func() { join(host) })

	fields := gen.Fields()
	chooserIn(t, fields, "format").SetSelected("png")
	setBox(t, fields, "size", "20kb")

	chooser, ok := controlUnder(content, text.FieldDamage()).(*parts.Chooser)
	if !ok {
		t.Fatalf("the damage field is not a list to choose from")
	}
	chooser.SetSelected(damage.ZeroHead)
	setBox(t, fields, damage.SettingBytes, "16")

	entryUnder(t, content, text.FieldOutputDir()).SetText(dir)
	press(t, content, text.ButtonGenerate())
	join(host)

	made, err := filepath.Glob(filepath.Join(dir, "*.png"))
	if err != nil || len(made) != 1 {
		t.Fatalf("the window wrote %v (err %v) and this guard needs exactly one file", made, err)
	}
	body, err := os.ReadFile(made[0])
	if err != nil {
		t.Fatal(err)
	}
	// Sixteen, because that is what was typed into the parameter. Eight would
	// pass for a window that drew the field and sent the default.
	for i := 0; i < 16; i++ {
		if body[i] != 0 {
			t.Fatalf("byte %d is %#x, so the choice or its setting never reached the engine", i, body[i])
		}
	}
	if int64(len(body)) != 20<<10 {
		t.Errorf("the file is %d B and the size asked for was %d B", len(body), 20<<10)
	}

	f := damageManifest(t, filepath.Join(dir, engine.DefaultManifestName)).Files[0]
	if len(f.Damage) != 1 || f.Damage[0].Type != damage.ZeroHead {
		t.Fatalf("the manifest records %v rather than the damage that was chosen", f.Damage)
	}
	if got := f.Damage[0].Settings[damage.SettingBytes]; got != "16" {
		t.Errorf("the manifest records bytes=%q rather than what was typed", got)
	}
}

// damageManifest reads a manifest as the manifest package spells it.
//
// Its own reader rather than the manifestShape the recipe guards share: that
// one is a narrow view written for what those guards ask, and adding a field
// to it for one caller would make every other guard carry it.
func damageManifest(t *testing.T, path string) manifest.Manifest {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the manifest: %v", err)
	}
	var m manifest.Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("parsing the manifest: %v", err)
	}
	return m
}
