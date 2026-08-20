package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// The form does not move when a run starts.
//
// A hidden widget takes no room in this toolkit, so the progress bar and the
// status line used to cost nothing at rest and their full height on the press
// of Generate. The bar at the foot went from 48 px to 116 px and the form above
// it lost the same 68 px, which slid every field upward - at the one moment
// somebody is looking at the buttons and not at the form, and twice per run,
// because Preview adds a line and Generate then adds the bar. The owner
// reported it as the bottom bar expanding oddly.
//
// This measures the room the form is left with rather than the height of the
// bar, because the room is the half a person sees moving. It asks the scroll
// how tall it ended up rather than working it out from the window, for the
// reason the height probe records: subtracting a bar from a window forgets the
// tab strip, and a guard that does its own arithmetic about a layout is
// checking the arithmetic rather than the layout.
//
// The state is reached by showing the two widgets rather than by starting a
// run. A real run would make this guard depend on ten thousand files being
// written while it holds a stopwatch, and it would measure the same two Show
// calls at the end of it.
//
// It is asked twice per screen, and the second way is the one that carries the
// weight. Measured on 2026-08-20 with the reserve deliberately taken out:
//
//	resting with a destination on the line   849 px -> 849 px   no jump
//	resting with nothing to say              886 px -> 849 px   jumps 37 px
//
// So for two weeks this guard was green against a build with the reserve
// removed, and it was green honestly - it never reached the state the reserve
// exists for. The window proposes an output folder of its own (O102), the
// status line carries that destination while nothing louder is there, and a
// label with text in it never collapsed - so the case the wrapper was written
// for stopped being the case the guard set up. Clearing the output box is what
// puts the screen back into it: runner.sayDestination hides the line when there
// is no destination to name, and a hidden widget takes no room in this toolkit.
// See observation O118.
func TestTheFormDoesNotMoveWhenARunStarts(t *testing.T) {
	restingStates := []struct {
		name          string
		clearTheBox   bool
		wantResting   bool
		whyItIsWorthA string
	}{
		{
			name:          "with a destination on the line",
			whyItIsWorthA: "the ordinary path, where the status line already carries the output folder",
		},
		{
			name:        "with nothing to say",
			clearTheBox: true,
			wantResting: true,
			whyItIsWorthA: "the state the reserve exists for - no destination, so the line is hidden " +
				"and costs nothing until a run speaks",
		},
	}

	for _, tab := range []string{text.TabOneTarget, text.TabPresets, text.TabRecipe} {
		for _, state := range restingStates {
			t.Run(tab+"/"+state.name, func(t *testing.T) {
				content, w := screenInAWindow(t, tab)

				scroll := scrollIn(content)
				if scroll == nil {
					t.Fatal("this screen has no scrolling area, so this guard read the wrong tree")
				}
				bar, status := runMessages(content)
				if bar == nil || status == nil {
					t.Fatal("this screen has no progress bar and status line, so this guard read the wrong tree")
				}

				if state.clearTheBox {
					box := entryUnder(t, content, text.FieldOutputDir)
					if box == nil {
						t.Fatalf("the %s screen has no output directory box, so this guard read the wrong tree", tab)
					}
					box.SetText("")
					settle(content, w)
				}

				// Whether the line is actually hidden is asserted rather than
				// assumed. If a later change keeps something on it at rest,
				// this state stops being the state the reserve is for, and
				// this guard has to say so instead of quietly measuring the
				// other one twice - which is exactly how it went blind before.
				if state.wantResting && status.Visible() {
					t.Fatalf("the status line still says %q with no destination, so this guard is not in "+
						"the state it means to check (%s)", status.Text, state.whyItIsWorthA)
				}

				atRest := scroll.Size().Height
				if atRest <= 0 {
					t.Fatal("the scrolling area has no height, so this guard would pass without checking anything")
				}

				// What a run says on the ordinary path: one line. A preview says
				// what it would cost, a run in flight says how far it has got, and
				// a run that finished says how many files it wrote.
				status.SetText("Wrote 10000 files.")
				status.Show()
				bar.Show()
				settle(content, w)

				duringRun := scroll.Size().Height
				if duringRun != atRest {
					t.Errorf("resting %s, the form has %.0f px and once a run is talking it has %.0f px, "+
						"so it jumps %.0f px under the pointer.\n"+
						"What to do: the run's messages are wrapped in parts.WithRoomForARun, which keeps "+
						"their height whether or not there is a run. Either that room is no longer big "+
						"enough for what the status line says here, or the wrapper was dropped.",
						state.name, atRest, duringRun, atRest-duringRun)
				}
			})
		}
	}
}

// settle lays the screen out again after something on it changed size.
//
// Resizing to the size the window is already at does nothing, so the two calls
// are to a different size and back. A guard that skips this reads the size from
// before its own change and compares it with itself.
func settle(content fyne.CanvasObject, w fyne.Window) {
	content.Refresh()
	w.Resize(fyne.NewSize(window.OpenSize.Width, window.OpenSize.Height-1))
	content.Refresh()
	w.Resize(window.OpenSize)
}

// The room kept for a run's messages is real room, not a number that happens to
// match.
//
// The guard above compares a screen with itself, so it would be satisfied by a
// wrapper that reserved nothing on a screen where nothing is ever shown. This
// one asks the wrapper directly: it has to be at least as tall as the bar and
// a line of label together, which is what it exists to hold.
func TestTheRoomKeptForARunHoldsTheBarAndALine(t *testing.T) {
	held := parts.WithRoomForARun(container.NewVBox()).MinSize().Height
	// The track is measured wrapped, because wrapped is how the screen puts it
	// there. Measured bare, this asked for the toolkit's own 31 px and would
	// have failed a reserve that is right - which is a guard reporting a defect
	// in itself.
	needed := container.NewVBox(
		parts.Slim(widget.NewProgressBar()), widget.NewLabel("one")).MinSize().Height

	if held < needed {
		t.Errorf("the bar keeps %.0f px for a run's messages and they need %.0f px, so the form "+
			"still moves when a run starts.", held, needed)
	}
}

// screenInAWindow lays one tab out at the size the window really opens at.
//
// Resized twice with a refresh between, for the reason the other screen helpers
// give: a wrapping label reports its height for the width it knows about, and
// on the first pass that is not the width it ends up with.
func screenInAWindow(t *testing.T, tab string) (fyne.CanvasObject, fyne.Window) {
	t.Helper()

	app := test.NewApp()
	app.Settings().SetTheme(parts.Theme())
	t.Cleanup(func() { test.NewApp() })

	host := &fakeHost{}
	window.Open(host)
	if host.content == nil {
		t.Fatal("opening the window put no screen in it")
	}
	content := selectTab(t, host.content, tab)

	w := test.NewWindow(host.content)
	t.Cleanup(w.Close)
	w.Resize(window.OpenSize)
	content.Refresh()
	w.Resize(window.OpenSize)
	content.Refresh()
	w.Resize(window.OpenSize)

	return content, w
}

// scrollIn is the scrolling area a screen puts its form in.
func scrollIn(o fyne.CanvasObject) *container.Scroll {
	var found *container.Scroll
	walk(o, func(obj fyne.CanvasObject) {
		if scroll, ok := obj.(*container.Scroll); ok && found == nil {
			found = scroll
		}
	})
	return found
}

// runMessages is the progress bar and the status line beside it.
//
// Found as a pair rather than separately, because the status line is an
// ordinary label and every screen has several - the one that belongs to a run
// is the one sharing a container with the bar.
func runMessages(o fyne.CanvasObject) (*widget.ProgressBar, *widget.Label) {
	var bar *widget.ProgressBar
	var status *widget.Label

	walk(o, func(obj fyne.CanvasObject) {
		box, ok := obj.(*fyne.Container)
		if !ok || bar != nil {
			return
		}
		var foundBar *widget.ProgressBar
		var foundLabel *widget.Label
		for _, child := range box.Objects {
			// The label has to be a child of this box, because that is what
			// says this is the row a run talks in. The track is looked for
			// underneath the child instead: it is wrapped in parts.Slim since
			// 2026-08-19, and a guard that insisted on a bare widget here read
			// the wrapper and declared the screen had no progress bar.
			if it, ok := child.(*widget.Label); ok {
				foundLabel = it
				continue
			}
			if it := progressUnder(child); it != nil {
				foundBar = it
			}
		}
		if foundBar != nil && foundLabel != nil {
			bar, status = foundBar, foundLabel
		}
	})
	return bar, status
}

// progressUnder finds the progress track at or beneath an object.
func progressUnder(o fyne.CanvasObject) *widget.ProgressBar {
	var found *widget.ProgressBar
	walk(o, func(obj fyne.CanvasObject) {
		if it, ok := obj.(*widget.ProgressBar); ok && found == nil {
			found = it
		}
	})
	return found
}
