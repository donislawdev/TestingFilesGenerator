package guard

import (
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// One change of a box reads the form once, on every screen.
//
// Reading the form is settle, and on the preset screen settle expands the
// preset. Until 2026-09-23 every change read it twice - once for the line under
// the buttons and once to mark the box - and with upload-validation chosen that
// was 271-295 ms of the window's thread for every key typed, half of it working
// out an answer the line had just been given (docs/GUI-MEMORY-2026-09-23.md
// section 4h).
//
// Counted rather than timed, because a time says how fast this machine is and
// the count is the defect. Asked as exactly one rather than at most one: a
// screen whose readings stopped reaching the host would count nought and pass
// a ceiling, so nought is this guard not being in the state it asks about.
//
// An emptied box is asked about as well, because it takes the other way out of
// the live check - the line is said and the box is left alone - and that early
// return is where a second reading was sitting.
func TestOneChangeOfABoxReadsTheFormOnce(t *testing.T) {
	host := newFakeHost(t)
	gen, pre, rec := window.NewGenerate(host), window.NewPreset(host), window.NewRecipe(host)
	chooserIn(t, pre.Fields(), settingPresetBox).SetSelected("size-boundaries")

	for _, b := range []struct {
		screen string
		fields *parts.Fields
		at     string
		values []string
	}{
		{"single batch", gen.Fields(), engine.SettingSeed, []string{"7", "", "42"}},
		{"single batch", gen.Fields(), format.SettingSize, []string{"3kb", "", "5kb"}},
		{"presets", pre.Fields(), engine.SettingSeed, []string{"7", "", "42"}},
		{"presets", pre.Fields(), "limit", []string{"20mb", "", "30mb"}},
		{"several batches", rec.Fields(), recipe.KeySeed, []string{"7", "", "42"}},
		{"several batches", rec.Fields(), recipe.TargetAddress(1, recipe.KeySize), []string{"3kb", "", "5kb"}},
	} {
		for _, v := range b.values {
			box := entryIn(t, b.fields, b.at)
			if box.Text == v {
				t.Fatalf("%q already holds %q on the %s screen, so typing it would change nothing and this guard would ask about no change",
					b.at, v, b.screen)
			}
			host.settles = 0
			box.SetText(v)
			switch {
			case host.settles == 0:
				t.Fatalf("typing %q into %q on the %s screen told the host of no reading of the form - "+
					"either the box reports no change or the screen reads its form without watchedSettle, and this guard sees nothing either way",
					v, b.at, b.screen)
			case host.settles > 1:
				t.Errorf("typing %q into %q on the %s screen read the form %d times, expected once - "+
					"on the preset screen every reading expands the preset",
					v, b.at, b.screen, host.settles)
			}
		}
	}
}

// A menu, a switch or a press that lays the batch screen out again reads the
// form once.
//
// The batch screen lays itself out again when a batch's format changes, when
// the base is switched or chosen, and when a press adds, copies or takes away
// a batch or what an archive holds. The layout said what the form comes to,
// and until 2026-09-24 a menu or a switch was then followed by the live check,
// which said it again: two readings, and with a base preset two expansions -
// switching the base to upload-validation took 278-321 ms in the real window
// (docs/GUI-MEMORY-2026-09-23.md section 4j).
//
// Exactly one, for the reason the guard above gives, and it matters here in
// the other direction too: a press is followed by no check, so nought is a
// press after which the line under the buttons still describes the old form.
func TestAChangeThatLaysTheBatchScreenOutAgainReadsTheFormOnce(t *testing.T) {
	host := newFakeHost(t)
	rec := window.NewRecipe(host)
	body := rec.Object()
	once := func(what string, act func()) {
		t.Helper()
		host.settles = 0
		act()
		switch {
		case host.settles == 0:
			t.Fatalf("%s told the host of no reading of the form - either nothing changed or the line under the buttons was not said, and this guard cannot tell which",
				what)
		case host.settles > 1:
			t.Errorf("%s read the form %d times, expected once", what, host.settles)
		}
	}
	press := func(name string) func() {
		return func() {
			t.Helper()
			b := buttonNamed(body, name)
			if b == nil {
				t.Fatalf("the batch screen has no %q button, so this guard cannot press it", name)
			}
			b.OnTapped()
		}
	}
	base := func() *parts.Chooser {
		for _, c := range reportingControls(findField(rec.Fields(), recipe.KeyExtends).Control) {
			if pick, is := c.(*parts.Chooser); is {
				return pick
			}
		}
		t.Fatal("the base is switched on and there is no menu of presets under it")
		return nil
	}

	once("choosing png for the first batch", func() {
		chooserIn(t, rec.Fields(), recipe.TargetAddress(1, recipe.KeyFormat)).SetSelected("png")
	})
	once("switching the base on", func() { toggleIn(t, rec.Fields(), "start_from_preset").SetChecked(true) })
	once("choosing text-encoding as the base", func() { base().SetSelected("text-encoding") })
	once("switching the base off", func() { toggleIn(t, rec.Fields(), "start_from_preset").SetChecked(false) })

	once("adding a batch", press(text.ButtonAddBatch()))
	once("duplicating a batch", press(text.ButtonDuplicateBatch()))
	once("removing a batch", press(text.ButtonRemoveBatch()))
	once("choosing zip for the first batch", func() {
		chooserIn(t, rec.Fields(), recipe.TargetAddress(1, recipe.KeyFormat)).SetSelected("zip")
	})
	once("adding what an archive holds", press(text.ButtonAddContents()))
	once("removing what an archive holds", press(text.ButtonRemoveContents()))

	// The base switched off with no batch left, which brings a batch back and
	// is the one way a switch and a press meet. Raised by the review of #133
	// (docs/REVIEW-133-2026-09-24.md): the batch came back through the press's
	// path, which says the line, and the switch's check then said it again.
	once("switching the base on again", func() { toggleIn(t, rec.Fields(), "start_from_preset").SetChecked(true) })
	for findField(rec.Fields(), recipe.TargetAddress(1, recipe.KeyID)) != nil {
		once("removing a batch with the base on", press(text.ButtonRemoveBatch()))
	}
	once("switching the base off with no batch left", func() { toggleIn(t, rec.Fields(), "start_from_preset").SetChecked(false) })
	if findField(rec.Fields(), recipe.TargetAddress(1, recipe.KeyID)) == nil {
		t.Error("the base was switched off with no batch left and no batch came back, so the screen can produce nothing")
	}
}

// Typing into a box a preset is not given does not expand the preset again.
//
// The seed and the output directory are read with the preset's values but are
// not among them, so what the preset expands to cannot depend on either. The
// screen expanded it on every key anyway until 2026-09-23 - 157-180 ms and
// 60 MB each time for upload-validation, which encodes images to find its
// sizes. The screen remembers its last expansion now (lastExpansion in
// internal/gui/window/preset.go).
//
// upload-validation because it is the one that cost the most, though the
// count does not depend on which preset is chosen.
func TestTypingWhatAPresetIsNotGivenDoesNotExpandItAgain(t *testing.T) {
	host := newFakeHost(t)
	pre := window.NewPreset(host)
	chooserIn(t, pre.Fields(), settingPresetBox).SetSelected("upload-validation")
	if host.expansions == 0 {
		t.Fatal("building the screen and choosing a preset expanded nothing, so either the preset screen no longer expands through lastExpansion or this guard is not in the state it asks about")
	}

	host.settles, host.expansions = 0, 0
	seed := entryIn(t, pre.Fields(), engine.SettingSeed)
	for _, v := range []string{"1", "12", "123"} {
		seed.SetText(v)
	}
	dir := entryIn(t, pre.Fields(), engine.SettingOutDir)
	dir.SetText(dir.Text + "-x")

	// The half that shows the keys were heard at all. Without it, a screen that
	// stopped reading its form would expand nothing and pass.
	if host.settles != 4 {
		t.Fatalf("four keys read the form %d time(s), so this guard is not counting what it thinks it is", host.settles)
	}
	if host.expansions != 0 {
		t.Errorf("typing into the seed and the output directory expanded the preset %d time(s), and neither is something a preset is given",
			host.expansions)
	}
}

// A changed preset value, or another preset chosen, is expanded again.
//
// The other half of the one above, and the half without which a screen that
// remembers one expansion forever would pass it. Asked twice: through the
// count, and through what the line under the buttons says - an expansion
// worked out again and then not used is the same defect with the count right.
func TestAChangedPresetValueIsExpandedAgain(t *testing.T) {
	host := newFakeHost(t)
	pre := window.NewPreset(host)
	chooserIn(t, pre.Fields(), settingPresetBox).SetSelected("size-boundaries")
	before := statusLine(t, pre.Object())
	if before == "" {
		t.Fatal("the line under the buttons says nothing with size-boundaries chosen, so there is nothing to compare a change against")
	}

	host.expansions = 0
	entryIn(t, pre.Fields(), "limit").SetText("20mb")
	if host.expansions != 1 {
		t.Errorf("a new limit was typed and the preset was expanded %d time(s), expected once", host.expansions)
	}
	if after := statusLine(t, pre.Object()); after == before {
		t.Errorf("a new limit was typed and the line still says what the old one came to:\n  %q", after)
	}

	// Another preset, with nothing given to either, so only the name tells the
	// two apart. Asked of the screen rather than assumed: size-boundaries was
	// the first choice here, and its format menu arrives with a value chosen,
	// so the two differed by what they were given and a screen that ignored
	// the name passed - measured by mutation on 2026-09-23.
	fresh := newFakeHost(t)
	other := window.NewPreset(fresh)
	chooserIn(t, other.Fields(), settingPresetBox).SetSelected("empty-and-minimal")
	nothingGiven(t, other.Fields(), "empty-and-minimal")
	first := statusLine(t, other.Object())
	fresh.expansions = 0
	chooserIn(t, other.Fields(), settingPresetBox).SetSelected("text-encoding")
	nothingGiven(t, other.Fields(), "text-encoding")
	if fresh.expansions != 1 {
		t.Errorf("another preset was chosen and it was expanded %d time(s), expected once", fresh.expansions)
	}
	if second := statusLine(t, other.Object()); second == first {
		t.Errorf("another preset was chosen and the line still says what the first came to:\n  %q", second)
	}
}

// nothingGiven stops a guard whose preset is given a value: every box and menu
// the preset screen draws for the chosen preset has to be empty.
func nothingGiven(t *testing.T, fields *parts.Fields, id string) {
	t.Helper()
	for _, f := range fields.All() {
		switch f.Setting {
		case settingPresetBox, engine.SettingSeed, engine.SettingOutDir:
			continue
		}
		if e := firstEntryIn(f.Control); e != nil && e.Text != "" {
			t.Fatalf("%s is given %s=%q, so it is not only the name that tells it from another preset", id, f.Setting, e.Text)
		}
		if c, is := f.Control.(*parts.Chooser); is && c.Selected != "" {
			t.Fatalf("%s is given %s=%q, so it is not only the name that tells it from another preset", id, f.Setting, c.Selected)
		}
	}
}

// settingPresetBox is the address the preset screen registers its menu of
// presets under - a key of that screen rather than of a recipe, see
// settingPreset in internal/gui/window/preset.go.
const settingPresetBox = "preset"
