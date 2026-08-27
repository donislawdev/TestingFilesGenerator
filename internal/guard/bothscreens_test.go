package guard

import (
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// One refusal marks its box on both screens, whichever way that screen names
// its boxes.
//
// The two screens do not share a vocabulary and cannot: the batch screen draws
// twenty batches and has to tell them apart, so it registers targets[2].size,
// while the single batch screen draws one and registers size. Measured
// 2026-08-25 by listing what each screen registers.
//
// Until that day the engine spoke only the second of those, and the first was
// left with nothing. A batch asking for a 10 B PDF and a batch with a name the
// host cannot store each refused the run and marked NOTHING on the batch
// screen - measured, both of them - while marking their box on the single one.
// Those are the two most ordinary refusals this tool has.
//
// So the engine names the position now and the single batch screen drops it
// again. That is two changes that can each be wrong on their own, and wrong in
// opposite directions, which is why this asks both screens the same question
// rather than each of them a question of its own. Getting the hook backwards
// would leave the single screen silent about a size below the minimum, which
// is the refusal people meet most.
func TestOneRefusalMarksItsBoxOnBothScreens(t *testing.T) {
	cases := []struct {
		name string
		// format and the box to spoil, in the vocabulary of each screen.
		format  string
		setting string
		value   string
		// onBatchScreen is where the same box lives over there.
		onBatchScreen string
	}{
		{
			name:          "a size the format cannot deliver",
			format:        "pdf",
			setting:       "size",
			value:         "10",
			onBatchScreen: recipe.TargetAddress(1, recipe.KeySize),
		},
		{
			name:          "a name the host cannot store",
			format:        "txt",
			setting:       "name",
			value:         "a<b.txt",
			onBatchScreen: recipe.TargetAddress(1, recipe.KeyName),
		},
		{
			name:          "a value a format setting will not take",
			format:        "bmp",
			setting:       "width",
			value:         "99999",
			onBatchScreen: recipe.TargetAddress(1, recipe.KeyProperties+".width"),
		},
	}

	for _, c := range cases {
		t.Run(c.name+", single batch screen", func(t *testing.T) {
			screen := window.NewGenerate(newFakeHost(t))
			body := screen.Object()
			fields := screen.Fields()
			chooserIn(t, fields, "format").SetSelected(c.format)
			if c.setting != "size" {
				setBox(t, fields, "size", "1mb")
			}
			setBox(t, fields, c.setting, c.value)

			pressNamed(t, body, text.ButtonPreview())
			screen.Settled()

			if saying := saidBy(t, fields, c.setting); saying == "" {
				t.Errorf("nothing is marked at %q on the screen that draws one batch.\n"+
					"Reason: this screen names its boxes without a position, so a refusal that\n"+
					"arrives carrying one has to have it dropped again. Getting that backwards is\n"+
					"silent - the run still refuses and the form says nothing.\n%s",
					c.setting, allSaid(fields))
			}
		})

		t.Run(c.name+", batch screen", func(t *testing.T) {
			screen := window.NewRecipe(newFakeHost(t))
			body := screen.Object()
			fields := screen.Fields()
			setBox(t, fields, recipe.TargetAddress(1, recipe.KeyID), "batch1")
			chooserIn(t, fields, recipe.TargetAddress(1, recipe.KeyFormat)).SetSelected(c.format)
			if c.setting != "size" {
				setBox(t, fields, recipe.TargetAddress(1, recipe.KeySize), "1mb")
			}
			setBox(t, fields, c.onBatchScreen, c.value)

			pressNamed(t, body, text.ButtonPreview())
			screen.Settled()

			if saying := saidBy(t, fields, c.onBatchScreen); saying == "" {
				t.Errorf("nothing is marked at %q on the screen that draws several batches.\n"+
					"Reason: with twenty batches on screen a refusal that names only the setting\n"+
					"points at all of them, so the run stops and the form says nothing about which\n"+
					"one to change.\n%s", c.onBatchScreen, allSaid(fields))
			}
		})
	}
}

// A refusal about the name of the manifest marks the manifest box, not the box
// for the name of a file.
//
// One function checks both names, and until 2026-08-25 it called both of them
// "name". On the batch screen that marked nothing, because the box there is
// called targets[1].name and the manifest box is output.manifest. On a screen
// that had both it would have marked the wrong one, which is worse: a value
// that is fine, pointed at, while the one that is wrong sits clean.
func TestARefusalAboutTheManifestNameMarksTheManifestBox(t *testing.T) {
	screen := window.NewRecipe(newFakeHost(t))
	body := screen.Object()
	fields := screen.Fields()
	setBox(t, fields, recipe.TargetAddress(1, recipe.KeyID), "batch1")
	setBox(t, fields, recipe.TargetAddress(1, recipe.KeySize), "1kb")
	setBox(t, fields, recipe.KeyOutputManifest, "report|1.json")

	pressNamed(t, body, text.ButtonPreview())
	screen.Settled()

	if saying := saidBy(t, fields, recipe.KeyOutputManifest); saying == "" {
		t.Errorf("nothing is marked at %q.\nReason: the manifest name is checked by the same\n"+
			"function as a file name, and calling both of them \"name\" leaves this refusal with\n"+
			"no box on a screen that has one for it.\n%s", recipe.KeyOutputManifest, allSaid(fields))
	}
	if saying := saidBy(t, fields, recipe.TargetAddress(1, recipe.KeyName)); saying != "" {
		t.Errorf("the box for a file name is marked and the manifest name is what is wrong.\n"+
			"It says: %s", saying)
	}
}

// A refusal about the run itself keeps its address whole on the screen that
// drops positions.
//
// The single batch screen takes the position off an address, because it draws
// one target and names its boxes without one. output.dir names no target and
// must come through untouched - taking its last segment would leave "dir",
// which is a box nothing draws, and the refusal would land at the foot of the
// form with nothing marked.
//
// Written because the hook is the half of this that can be wrong in a way
// nothing else notices: every other guard here asks about a setting of a
// target, and those all have a position to drop.
func TestARefusalAboutTheRunItselfKeepsItsWholeAddress(t *testing.T) {
	screen := window.NewGenerate(newFakeHost(t))
	body := screen.Object()
	fields := screen.Fields()
	setBox(t, fields, "size", "1kb")
	setBox(t, fields, "output.dir", "")

	pressNamed(t, body, text.ButtonPreview())
	screen.Settled()

	if saying := saidBy(t, fields, "output.dir"); saying == "" {
		t.Errorf("nothing is marked at %q.\nReason: this screen takes the position off an address\n"+
			"so that a refusal about its one target lands on the right box. An address that names\n"+
			"no target has no position to take off, and cutting it anyway leaves a key nothing\n"+
			"draws.\n%s", "output.dir", allSaid(fields))
	}
}
