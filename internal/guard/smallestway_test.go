package guard

import (
	"slices"
	"testing"

	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// Smallest as a way of giving the size, on both screens since 2026-11-17
// (docs/SIZE-FIXES-2026-11-16.md section 3.5). The owner could not find it: it
// was a word to type into the size box, named only behind the button beside
// it. These ask two things of the switch that offers it now - that it offers
// it where a run would take it and nowhere else, and that it never says one
// thing while the box beside it sends another.

// TestSmallestIsOfferedForExactlyTheFormatsTheEngineTakesItFor chooses every
// format on both screens and asks the switch whether it offers Smallest, and
// asks the engine, by planning, whether it takes smallest for that format.
//
// The engine is asked by a plan and not by the function the window calls, so
// a format the window offers it for and the engine then refuses - a picture
// that needs something else first, say - turns this red rather than passing
// because both sides read one rule.
func TestSmallestIsOfferedForExactlyTheFormatsTheEngineTakesItFor(t *testing.T) {
	takes := map[string]bool{}
	for _, d := range format.All() {
		takes[d.ID] = engineTakesSmallest(t, d)
	}
	// Both answers have to occur, or the comparison below holds nothing to
	// tell apart.
	if !slices.Contains(mapValues(takes), true) || !slices.Contains(mapValues(takes), false) {
		t.Fatalf("the engine took smallest for %v - one of the two answers never came, so nothing here is compared", takes)
	}

	single := window.NewGenerate(newFakeHost(t))
	several := window.NewRecipe(newFakeHost(t))
	offered := map[string]map[string]bool{"the single batch screen": {}, "the batch screen": {}}
	for _, d := range format.All() {
		chooserIn(t, single.Fields(), engine.SettingFormat).SetSelected(d.ID)
		offered["the single batch screen"][d.ID] = offersSmallest(single.Object())
		chooserIn(t, several.Fields(), recipe.TargetAddress(1, recipe.KeyFormat)).SetSelected(d.ID)
		offered["the batch screen"][d.ID] = offersSmallest(several.Object())
	}
	for screen, there := range offered {
		for _, wrong := range smallestDisagreements(there, takes) {
			t.Errorf("%s: %s", screen, wrong)
		}
	}

	// The canary: a window offering smallest for every format has to be told
	// apart from the engine.
	everywhere := map[string]bool{}
	for id := range takes {
		everywhere[id] = true
	}
	if len(smallestDisagreements(everywhere, takes)) == 0 {
		t.Error("a window offering smallest for every format agreed with the engine, so this compares nothing")
	}
}

// engineTakesSmallest plans one file of d at its smallest, with a width and a
// height where d declares them - 160x120, because ico stops at 256 pixels.
func engineTakesSmallest(t *testing.T, d format.Descriptor) bool {
	t.Helper()
	props := map[string]string{}
	for _, p := range d.Properties {
		if p.Name == "width" {
			props["width"] = "160"
		}
		if p.Name == "height" {
			props["height"] = "120"
		}
	}
	target := engine.Target{ID: "smallest", Format: d.ID, Smallest: true, Sizes: engine.Uniform(1, 0), Properties: props}
	_, err := engine.Plan([]engine.Target{target},
		engine.Options{OutDir: t.TempDir(), Seed: 7, Command: "guard", ManifestName: "manifest.json"})
	return err == nil
}

// smallestDisagreements is every format where a screen offering smallest and
// the engine taking it differ.
func smallestDisagreements(offered, takes map[string]bool) []string {
	var out []string
	for id, taken := range takes {
		switch {
		case offered[id] && !taken:
			out = append(out, id+" offers Smallest and the engine refuses smallest for it")
		case !offered[id] && taken:
			out = append(out, id+" does not offer Smallest and the engine takes smallest for it")
		}
	}
	slices.Sort(out)
	return out
}

func mapValues(m map[string]bool) []bool {
	out := make([]bool, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

// offersSmallest says a switch a person can see offers Smallest.
func offersSmallest(o fyne.CanvasObject) bool {
	for _, s := range shownSwitches(o) {
		if slices.Contains(s.Options, text.SizeWaySmallest()) {
			return true
		}
	}
	return false
}

// shownSwitches is every switch on the screen that nothing hides - built like
// shownText, with the same walk twice: everything, then everything under
// something hidden.
func shownSwitches(o fyne.CanvasObject) []*parts.Segments {
	var hidden []fyne.CanvasObject
	walk(o, func(obj fyne.CanvasObject) {
		if obj != nil && !obj.Visible() {
			hidden = append(hidden, obj)
		}
	})
	under := map[fyne.CanvasObject]bool{}
	for _, root := range hidden {
		walk(root, func(obj fyne.CanvasObject) { under[obj] = true })
	}
	var out []*parts.Segments
	walk(o, func(obj fyne.CanvasObject) {
		if s, ok := obj.(*parts.Segments); ok && !under[s] {
			out = append(out, s)
		}
	})
	return out
}

// sizeScreen is one of the two screens as the steps below drive it: the
// addresses of its format and size, and its one switch.
type sizeScreen struct {
	name   string
	object fyne.CanvasObject
	fields *parts.Fields
	format string
	size   string
	width  string
	height string
	settle func()
}

func (s sizeScreen) way(t *testing.T) *parts.Segments {
	t.Helper()
	all := sizeWaySwitches(s.object)
	if len(all) != 1 {
		t.Fatalf("%s: %d switches for the way of giving the size, where one was due", s.name, len(all))
	}
	return all[0]
}

func sizeScreens(t *testing.T) []sizeScreen {
	t.Helper()
	single := window.NewGenerate(newFakeHost(t))
	several := window.NewRecipe(newFakeHost(t))
	return []sizeScreen{
		{name: "the single batch screen", object: single.Object(), fields: single.Fields(),
			format: engine.SettingFormat, size: core.SettingSize, width: "width", height: "height", settle: single.Settled},
		{name: "the batch screen", object: several.Object(), fields: several.Fields(),
			format: recipe.TargetAddress(1, recipe.KeyFormat), size: recipe.TargetAddress(1, recipe.KeySize),
			width:  recipe.TargetAddress(1, core.KeyProperties+".width"),
			height: recipe.TargetAddress(1, core.KeyProperties+".height"), settle: several.Settled},
	}
}

// TestTheSizeSwitchAndTheSizeBoxNeverSayDifferentThings walks every way the two
// can move on both screens and asks both after each step.
//
// The box is what the run reads, so a switch on Smallest over a box holding a
// number - or the other way round - is a screen telling somebody one thing and
// sending another. Each step is one way a person moves one of them: the switch,
// the box typed into, the format changed under them, and the run accepting
// what Smallest sent. The button under a refusal is the fourth way, and
// TestBothButtonsUnderABatchOfSmallPicturesAreAcceptedInTheWindow presses it.
func TestTheSizeSwitchAndTheSizeBoxNeverSayDifferentThings(t *testing.T) {
	for _, s := range sizeScreens(t) {
		chooserIn(t, s.fields, s.format).SetSelected("png")
		setBox(t, s.fields, s.size, "4kb")
		way := s.way(t)

		agree := func(step, wantWay, wantBox string) {
			t.Helper()
			if got := way.Selected; got != wantWay {
				t.Errorf("%s, %s: the switch is on %q, where %q was due", s.name, step, got, wantWay)
			}
			if got := boxText(t, s.fields, s.size); got != wantBox {
				t.Errorf("%s, %s: the size box holds %q, where %q was due", s.name, step, got, wantBox)
			}
		}

		way.SetSelected(text.SizeWaySmallest())
		agree("Smallest chosen", text.SizeWaySmallest(), core.SizeSmallest)
		way.SetSelected(text.SizeWayExact())
		agree("One size chosen again", text.SizeWayExact(), "4kb")
		setBox(t, s.fields, s.size, core.SizeSmallest)
		agree("smallest typed", text.SizeWaySmallest(), core.SizeSmallest)
		setBox(t, s.fields, s.size, "8kb")
		agree("a size typed over it", text.SizeWayExact(), "8kb")

		// A format that is not a picture cannot be the smallest of anything:
		// the switch loses Smallest and the box takes back its size, rather
		// than keeping a word the run refuses for it.
		way.SetSelected(text.SizeWaySmallest())
		chooserIn(t, s.fields, s.format).SetSelected("csv")
		if offersSmallest(s.object) {
			t.Errorf("%s: csv is chosen and Smallest is still offered", s.name)
		}
		agree("csv chosen under Smallest", text.SizeWayExact(), "8kb")

		// Without a width and a height the run refuses smallest, and the
		// refusal lands on the size box - which Smallest leaves on the screen
		// so that there is a box for it to land on. Asked first, so the
		// acceptance below is known to come from a preview that can refuse.
		chooserIn(t, s.fields, s.format).SetSelected("png")
		way.SetSelected(text.SizeWaySmallest())
		pressNamed(t, s.object, text.ButtonPreview())
		s.settle()
		if saidBy(t, s.fields, s.size) == "" {
			t.Errorf("%s: a png at Smallest with no width or height was not refused at the size box.\n%s",
				s.name, allSaid(s.fields))
		}

		// And the run takes what Smallest sends, for a picture with its width
		// and height set.
		setBox(t, s.fields, s.width, "160")
		setBox(t, s.fields, s.height, "120")
		way.SetSelected(text.SizeWaySmallest())
		pressNamed(t, s.object, text.ButtonPreview())
		s.settle()
		if refusal := anyRefusal(s.object); refusal != "" {
			t.Errorf("%s: a png of 160x120 at Smallest was refused: %s\n%s", s.name, refusal, allSaid(s.fields))
		}
	}
}
