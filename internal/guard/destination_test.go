package guard

import (
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// The window offers a folder of its own to write into, not the directory it was
// started from.
//
// Double clicked from a file manager, the working directory is the folder the
// program was unpacked into - so the destination the window offered was
// somebody's Downloads, and a set of ten thousand files went into it mixed with
// everything already there, with nothing marking which ones were ours. This is
// the only part of the tool that writes into other people's directories, and
// the only field on these forms that decides where (O103).
//
// The command line still offers the directory you are standing in, and that
// difference is deliberate: in a terminal you typed your way there and you know
// which one it is. This guard is about the window only.
func TestTheWindowOffersAFolderOfItsOwnToWriteInto(t *testing.T) {
	for _, tab := range []string{text.TabOneTarget(), text.TabPresets(), text.TabRecipe()} {
		t.Run(tab, func(t *testing.T) {
			content, _ := screenInAWindow(t, tab)

			box := entryUnder(t, content, text.FieldOutputDir())
			if box == nil {
				t.Fatalf("the %s screen has no output directory box, so this guard read the wrong tree", tab)
			}
			offered := box.Text
			if offered == "" {
				t.Fatal("the output directory box is empty, so the window offers nothing and this guard checks nothing")
			}

			working, err := os.Getwd()
			if err != nil {
				t.Skipf("this system will not say what the working directory is: %v", err)
			}
			if filepath.Clean(offered) == filepath.Clean(working) {
				t.Errorf("the window offers the working directory itself (%s).\n"+
					"Started by double click that is the folder the program was unpacked into, so a "+
					"run of ten thousand files lands in it mixed with whatever is already there.\n"+
					"What to do: offer a folder of our own under it - window.OutputFolderName.", offered)
			}
			if base := filepath.Base(offered); base != window.OutputFolderName {
				t.Errorf("the window offers %q, whose last part is %q rather than %q.\n"+
					"One named folder is what makes a run a single thing to delete again.",
					offered, base, window.OutputFolderName)
			}
		})
	}
}

// Where the files will go is readable without scrolling.
//
// Every one of these forms is taller than the window it opens in, and the
// output directory is at the foot of the last section - so the one field that
// decides where somebody else's disk gets written to was the one field nobody
// saw before pressing Generate (O102). The bar at the foot never scrolls away
// and it keeps a line clear for a run whether or not there is one, so saying it
// there costs no room at all.
//
// The test is that the line is OUTSIDE the scrolling area rather than that it
// exists. A label saying the right thing in a part of the screen you have to
// scroll to reach is the defect, not the fix.
func TestWhereTheFilesGoIsSaidOutsideTheScrollingPart(t *testing.T) {
	for _, tab := range []string{text.TabOneTarget(), text.TabPresets(), text.TabRecipe()} {
		t.Run(tab, func(t *testing.T) {
			content, _ := screenInAWindow(t, tab)

			box := entryUnder(t, content, text.FieldOutputDir())
			if box == nil {
				t.Fatalf("the %s screen has no output directory box, so this guard read the wrong tree", tab)
			}
			want := text.WritingTo(box.Text)

			said := labelSaying(content, want)
			if said == nil {
				t.Fatalf("nothing on the %s screen says %q.\n"+
					"The status line carries it while a run has said nothing - see runner.sayDestination.",
					tab, want)
			}

			scroll := scrollIn(content)
			if scroll == nil {
				t.Fatal("this screen has no scrolling area, so this guard read the wrong tree")
			}
			if holds(scroll.Content, said) {
				t.Errorf("the %s screen says where the files go from inside the scrolling area, so it "+
					"is only readable after scrolling to it - which is the whole defect.", tab)
			}
		})
	}
}

// labelSaying is the label carrying one exact sentence.
func labelSaying(o fyne.CanvasObject, want string) *widget.Label {
	var found *widget.Label
	walk(o, func(obj fyne.CanvasObject) {
		label, ok := obj.(*widget.Label)
		if ok && found == nil && label.Text == want {
			found = label
		}
	})
	return found
}

// holds says whether one object is anywhere inside another.
func holds(parent fyne.CanvasObject, wanted fyne.CanvasObject) bool {
	found := false
	walk(parent, func(obj fyne.CanvasObject) {
		if obj == wanted {
			found = true
		}
	})
	return found
}

// Clearing the output directory refuses the run instead of writing beside the
// window, on EVERY screen.
//
// O125. The batch screen composes a recipe and runs it, and the recipe reader
// answers "." for a recipe with no output.dir - which is right for a file,
// because leaving the key out is how somebody says "put them here". On a screen
// it was wrong: the box arrives filled in, so an empty one is somebody clearing
// it rather than somebody never saying, and the window ran into whatever
// directory it happened to be started from without a word.
//
// Measured 2026-08-24: a recipe with no output section writes its files beside
// the working directory, and the window took that path. The other two screens
// hand the engine an empty string and it refuses. This holds all three to the
// same answer.
//
// It presses Generate rather than Preview on purpose. A preview writes nothing
// whatever the directory says, so it could not tell the two behaviours apart -
// which is exactly the shape of guard this project has recorded as passing
// without reaching the code.
func TestClearingTheOutputDirectoryRefusesTheRunOnEveryScreen(t *testing.T) {
	for _, tab := range []string{text.TabOneTarget(), text.TabPresets(), text.TabRecipe()} {
		t.Run(tab, func(t *testing.T) {
			host := &fakeHost{}
			window.Open(host)
			if host.content == nil {
				t.Fatal("opening the window put no screen in it")
			}
			t.Cleanup(func() { join(host) })
			content := selectTab(t, host.content, tab)

			// The batch screen opens with nothing answered, so give it the two
			// settings it cannot run without - otherwise the refusal under test
			// is lost among others and this would pass for the wrong reason.
			if tab == text.TabRecipe() {
				fill(t, content, text.FieldTargetID(), "files")
				fill(t, content, text.FieldSize(), "1kb")
			}

			// Asserted rather than assumed: if the box were already empty this
			// guard would be proving nothing about clearing it.
			box := entryUnder(t, content, text.FieldOutputDir())
			if box.Text == "" {
				t.Fatalf("%s opens with an empty output directory, so there is nothing to clear", tab)
			}
			box.SetText("")

			press(t, content, text.ButtonGenerate())
			join(host)

			if edgeOf(t, content, text.FieldOutputDir()).StrokeWidth <= 0 {
				t.Errorf("%s: the output directory was cleared, Generate was pressed, and the box says nothing - "+
					"so the run went somewhere nobody named", tab)
			}
		})
	}
}
