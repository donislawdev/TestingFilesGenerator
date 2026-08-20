package guard

import (
	"testing"

	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// The action bar is inset once, not twice.
//
// It owns about an eighth of the window for three buttons and a line, and the
// height is not free: the form above it is 151 px short of fitting on one
// screen and 410 on another. So every pixel it holds has to be doing something.
//
// Twelve of them were not. The bar keeps its own padding around what stands on
// it, and on 2026-08-20 a second padding went inside that - put there to move
// the bar's words onto the same left edge as the form's, which is what it did
// horizontally. Vertically it was the same inset applied a second time, and it
// took the bar from 134 px to 146.
//
// Asserted as arithmetic on the part rather than as a number: what has to hold
// is that the bar costs its content plus one inset, whatever the inset is.
func TestTheActionBarCostsItsContentPlusOneInset(t *testing.T) {
	app := test.NewApp()
	app.Settings().SetTheme(parts.Theme())
	t.Cleanup(func() { test.NewApp() })

	inside := widget.NewLabel("what a run has to say")
	bar := parts.ActionBar(nil, inside)

	want := inside.MinSize().Height + theme.Padding()*2
	got := bar.MinSize().Height

	// Half a pixel, because a rounded corner and a stroke are drawn on the
	// surface behind and neither is padding.
	if diff := got - want; diff > 0.5 || diff < -0.5 {
		t.Errorf("the bar asks for %.1f px around a %.1f px line, and one inset at each end is %.1f px."+
			" A bar inset twice costs the form the difference and gives nothing back",
			got, inside.MinSize().Height, want)
	}
}

// And it still keeps the room a run needs.
//
// The half that must not be traded away for the pixels above. The reserve is
// why the form does not jump when Generate is pressed, and taking it out is
// the cheapest way to make a bar shorter - see O101 and O118.
func TestTheActionBarStillReservesRoomForARun(t *testing.T) {
	app := test.NewApp()
	app.Settings().SetTheme(parts.Theme())
	t.Cleanup(func() { test.NewApp() })

	// A HIDDEN child, which is the whole point: a hidden widget costs no height
	// in this toolkit, so the bar shrinks the moment a run stops talking and
	// grows the moment it starts. What the wrapper does is hold that room open.
	//
	// The first version of this asked an empty label instead, and an empty
	// label is still a label - it has a height of its own, so the wrapper
	// could be removed entirely and the guard stayed green. Caught by the
	// mutation runner.
	quiet := widget.NewLabel("")
	quiet.Hide()

	bare := container.NewVBox(quiet).MinSize().Height
	held := parts.WithRoomForARun(container.NewVBox(quiet)).MinSize().Height

	if held <= bare {
		t.Errorf("a bar with nothing to say asks for %.1f px and the same thing unwrapped asks for %.1f px,"+
			" so no room is being held open and the form moves when a run starts", held, bare)
	}
}
