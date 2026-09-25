package guard

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// repeatedRounds is how many times each screen does its round. One round is
// the one every later round is compared with, so three is the fewest that
// shows growth twice rather than once.
const repeatedRounds = 4

// The same work done again leaves the window as it was.
//
// A person sits in this window for an afternoon and does the same few things
// over and over, and on 2026-09-23 that is what the window could not survive.
// The batch screen wired every kept control once more on every rebuild, so the
// preset switch took 0.7 s at its first press and 6.1 s at its twentieth
// (docs/GUI-MEMORY-2026-09-23.md section 2.2). That one place has a guard of
// its own, TestAControlRegisteredAgainReportsOnceUnderItsLatestAddress. This
// one asks about the class rather than the place: every screen does one round
// of the work people repeat, several times over, and after every round three
// counts have to come out the same.
//
//   - How many times each change read the form and expanded a preset. A chain
//     of listeners anywhere, not only where the last one was, makes the same
//     change cost more with every round.
//   - How many things the screen can reach through its own fields, rather
//     than through what is drawn. A rebuild that leaves the old panels in a
//     box and a map that keeps the controls of a removed batch look the same
//     from here.
//   - How many goroutines started during this guard are still running our code
//     once a round's run has finished. Nothing else asks: the wait for a run is
//     a channel the worker closes before it returns, and a worker stuck after
//     that point holds nothing up, not even the end of the test binary.
//
// Counts rather than memory, because the toolkit's test driver never clears
// its cache of renderers - only the loop of a real window does - so the heap
// of a test grows with every rebuild of a healthy window. Memory is asked of
// the real window once per release, in phase 11 of the release check. What
// this guard cannot see is listed in docs/GUI-LEAK-TEST-2026-09-25.md section 7.
func TestTheSameWorkDoneAgainLeavesTheWindowAsItWas(t *testing.T) {
	before := goroutineIDs()
	for _, s := range []struct {
		name  string
		start func(t *testing.T) *repetition
	}{
		{"single batch", singleBatchRound},
		{"presets", presetRound},
		{"several batches", batchesRound},
	} {
		t.Run(s.name, func(t *testing.T) {
			r := s.start(t)
			r.before = before
			for n := 1; n <= repeatedRounds; n++ {
				r.round(n)
			}
			r.compare()
		})
	}
}

// singleBatchRound is every format in turn, a damage and none, a size typed
// and typed back, and one small run into a directory of its own.
//
// The run is held just before it reports, which is where the count of
// goroutines is shown to see the worker at all - a count that saw nothing
// while one was plainly running would pass every round with one left behind.
func singleBatchRound(t *testing.T) *repetition {
	host := newFakeHost(t)
	g := window.NewGenerate(host)
	hold := newHold()
	g.HoldBeforeFinishing(hold.enter)
	// In this order because cleanups run last first: a round that failed
	// before looking leaves the worker parked, and waiting for it before
	// letting it go would wait for ever.
	t.Cleanup(g.Settled)
	t.Cleanup(hold.free)

	fields := g.Fields()
	formats := chooserIn(t, fields, engine.SettingFormat)
	damages := chooserIn(t, fields, recipe.KeyDamage)
	size := entryIn(t, fields, format.SettingSize)
	// Text, and a size nobody has to wait for, so the run costs what writing
	// one small file costs. Set once, before the first round, so every round
	// starts from it and ends on it.
	formats.SetSelected("txt")
	size.SetText("1kb")

	r := newRepetition(t, host, g)
	r.work = func(r *repetition) {
		for _, id := range inTurnFrom(t, format.IDs(), formats.Selected) {
			r.change("choosing "+id, func() { formats.SetSelected(id) })
		}
		r.change("choosing a damage", func() { damages.SetSelected(damage.Names()[0]) })
		r.change("choosing no damage", func() { damages.SetSelected(text.DamageNone()) })
		r.change("typing a size", func() { size.SetText("3kb") })
		r.change("typing the size back", func() { size.SetText("1kb") })

		dir := t.TempDir()
		g.SetOutDir(dir)
		r.change("a run", func() {
			press(t, g.Object(), text.ButtonGenerate())
			hold.look(r.lookAtTheWorker)
			g.Settled()
		})
		hold.again()
		if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err != nil {
			t.Fatalf("the run of this round wrote no manifest into %s (%v), so it was refused rather than run and the goroutines it would have left are not asked about", dir, err)
		}
	}
	return r
}

// presetRound is every preset in turn, with its seed typed and typed back,
// and the one parameter of size-boundaries - the preset screen expands the
// preset on every change, so this is where an expansion too many would show.
func presetRound(t *testing.T) *repetition {
	host := newFakeHost(t)
	p := window.NewPreset(host)
	fields := p.Fields()
	pick := chooserIn(t, fields, settingPresetBox)

	r := newRepetition(t, host, p)
	r.work = func(r *repetition) {
		for _, id := range inTurnFrom(t, preset.IDs(), pick.Selected) {
			r.change("choosing "+id, func() { pick.SetSelected(id) })
			r.typeAndTypeBack(id+" seed", entryIn(t, fields, engine.SettingSeed), "7")
			if id == "size-boundaries" {
				r.typeAndTypeBack(id+" limit", entryIn(t, fields, "limit"), "20mb")
			}
		}
	}
	return r
}

// batchesRound is what the batch screen is for: a base switched on, changed
// and switched off, a batch added and taken away, a batch copied and the copy
// taken away, an archive given contents and back, and the name of a batch
// typed into. Every one of them but the typing lays the screen out again, and
// the kept controls are registered again every time.
func batchesRound(t *testing.T) *repetition {
	host := newFakeHost(t)
	rec := window.NewRecipe(host)
	body, fields := rec.Object(), rec.Fields()
	pressing := func(name string) func() {
		return func() { press(t, body, name) }
	}

	r := newRepetition(t, host, rec)
	r.work = func(r *repetition) {
		base := toggleIn(t, fields, settingBuildOnPreset)
		r.change("switching the base on", func() { base.SetChecked(true) })
		bases := baseMenuOf(t, fields)
		r.change("choosing upload-validation as the base", func() { bases.SetSelected("upload-validation") })
		r.change("choosing tabular-import as the base", func() { bases.SetSelected("tabular-import") })
		r.change("switching the base off", func() { base.SetChecked(false) })

		r.change("adding a batch", pressing(text.ButtonAddBatch()))
		r.change("removing it", pressing(text.ButtonRemoveBatch()))
		r.change("copying a batch", pressing(text.ButtonDuplicateBatch()))
		r.change("removing the copy", pressing(text.ButtonRemoveBatch()))

		kind := chooserIn(t, fields, recipe.TargetAddress(1, recipe.KeyFormat))
		was := kind.Selected
		if was == "zip" {
			t.Fatal("the first batch starts as a zip, so choosing zip would change nothing and the contents are not asked about")
		}
		r.change("choosing zip", func() { kind.SetSelected("zip") })
		r.change("adding what the archive holds", pressing(text.ButtonAddContents()))
		r.change("removing what the archive holds", pressing(text.ButtonRemoveContents()))
		r.change("choosing the format back", func() { kind.SetSelected(was) })

		name := entryIn(t, fields, recipe.TargetAddress(1, recipe.KeyID))
		r.typeAndTypeBack("the batch name", name, name.Text+"x")

		if findField(fields, recipe.TargetAddress(2, recipe.KeyID)) != nil {
			t.Fatal("a second batch is still on the screen after the round, so the next round does not start where this one did")
		}
	}
	return r
}

// settingBuildOnPreset is the switch that makes the batch screen start from a
// preset. The screen keeps the name to itself.
const settingBuildOnPreset = "start_from_preset"

// baseMenuOf is the menu of presets under the base, which is on the screen
// only while the switch is on.
func baseMenuOf(t *testing.T, fields *parts.Fields) *parts.Chooser {
	t.Helper()
	f := findField(fields, recipe.KeyExtends)
	if f == nil {
		t.Fatal("the base is switched on and nothing is registered for the preset it starts from")
	}
	for _, c := range reportingControls(f.Control) {
		if pick, is := c.(*parts.Chooser); is {
			return pick
		}
	}
	t.Fatal("the base is switched on and there is no menu of presets under it")
	return nil
}

// inTurnFrom is every value once, starting after from and ending on it, so
// that each is a change from the one before and the round ends where it began.
func inTurnFrom(t *testing.T, values []string, from string) []string {
	t.Helper()
	for i, v := range values {
		if v == from {
			return append(append([]string{}, values[i+1:]...), values[:i+1]...)
		}
	}
	t.Fatalf("%q is not among %v, so the round cannot end where it began", from, values)
	return nil
}
