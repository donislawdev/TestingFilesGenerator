package guard

import (
	"testing"

	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// What this defends. The window opens where it was left, and nowhere else.
//
// Why it needed a guard. This is the first thing the program deliberately keeps
// on somebody's disk between runs - untouchable rule 7 and D16 are both next
// door - so what it keeps and what it does with it are worth stating in a test
// rather than in a comment. A defect here is not a wrong pixel: it is a set of
// files written to last week's directory because the window quietly offered it
// on a screen the person did not look at.
//
// Asked of all three screens, because the directory follows whoever is looking.
// Setting it on the screen that opens first and leaving the others at the
// default is the exact shape of the defect this window already had once, in
// August: two screens that agreed until somebody changed one.
func TestTheWindowOffersTheDirectoryItWasLastToldToUse(t *testing.T) {
	const last = "D:/somewhere/else"
	host := newFakeHost(t)
	host.Remembered().RememberDirectory(last)
	window.Open(host)
	if host.content == nil {
		t.Fatal("opening the window put no screen in it")
	}

	for _, tab := range []string{text.TabOneTarget(), text.TabPresets(), text.TabRecipe()} {
		screen := selectTab(t, host.content, tab)
		box := entryUnder(t, screen, text.FieldOutputDir())
		if box == nil {
			t.Fatalf("the %s screen has no output directory box", tab)
		}
		if box.Text != last {
			t.Errorf("the %s screen offers %q, and the window was last told to write to %q",
				tab, box.Text, last)
		}
	}
}

// And with nothing remembered it offers the folder it always offered.
//
// The half that is easy to leave out, and leaving it out is not harmless: it
// would mean a first start offering an empty box, which every screen refuses -
// so the first thing anybody saw would be a refusal about a field they had not
// touched. The measured default is a folder of our own under the working
// directory, and the reason it is a folder rather than the directory itself is
// ten thousand files landing in somebody's Downloads.
func TestAFirstStartOffersTheFolderItAlwaysDid(t *testing.T) {
	host := newFakeHost(t)
	window.Open(host)
	if host.content == nil {
		t.Fatal("opening the window put no screen in it")
	}

	screen := selectTab(t, host.content, text.TabOneTarget())
	box := entryUnder(t, screen, text.FieldOutputDir())
	if box == nil {
		t.Fatal("the generate screen has no output directory box")
	}
	if box.Text == "" {
		t.Fatal("a first start offers an empty directory, which every screen refuses")
	}
	if !hasSuffix(box.Text, window.OutputFolderName) {
		t.Errorf("a first start offers %q, which does not end in the folder this tool writes into, %q",
			box.Text, window.OutputFolderName)
	}
}

// Closing the window writes down where the files were going.
//
// It is taken from the screen somebody was last on rather than from a fixed
// one, and this asks for exactly that: the directory is changed on a screen
// that is NOT the one the window opened on. A version that read the first
// screen would pass a test that only ever looked at the first screen, and would
// remember the wrong value for everybody who changed tabs.
func TestClosingTheWindowRemembersWhereTheFilesWereGoing(t *testing.T) {
	const chosen = "D:/chosen/on/the/preset/screen"
	host := newFakeHost(t)
	window.Open(host)
	if host.intercept == nil {
		t.Fatal("the window has no close intercept, so closing it does nothing this can read")
	}

	screen := selectTab(t, host.content, text.TabPresets())
	box := entryUnder(t, screen, text.FieldOutputDir())
	if box == nil {
		t.Fatal("the preset screen has no output directory box")
	}
	box.SetText(chosen)

	host.intercept()
	if got := host.Remembered().Directory(); got != chosen {
		t.Errorf("the window was closed with %q in the box and remembered %q", chosen, got)
	}
	if host.closed == 0 {
		t.Error("the window remembered the directory and never actually closed")
	}
}

// A size that was left behind is used, and NOT centred.
//
// The two answers are one decision and this asks about both together, because
// the centring is the half that matters and the half nobody would think to
// check. Measured on 2026-08-25 with tools/probes/windowsize and
// tools/probes/windowrect.ps1, on a screen with 3840x2088 of usable area:
//
//	5000x3000, not centred : lands at 304,304 - the title bar is on the screen
//	5000x3000, centred     : lands at -1841,-1215 - the title bar is 1215 px
//	                         above the top, so the window cannot be moved or
//	                         resized with a mouse at all
//
// Centring works the middle out from the size ASKED for, so a window bigger
// than the screen it comes back on is put out of reach. Fyne cannot say how big
// the screen is - there is no such call in v2.8.0 - so leaving the window where
// the system puts it is the whole of the mitigation.
func TestARememberedSizeIsUsedAndNotRecentred(t *testing.T) {
	bigger := fyne.NewSize(window.OpenSize.Width+600, window.OpenSize.Height+400)
	size, centre := window.HowToOpen(bigger)
	if size != bigger {
		t.Errorf("the window was %v when it was closed and opens at %v", bigger, size)
	}
	if centre {
		t.Errorf("a remembered %v is centred, and a window bigger than the screen it comes back on"+
			" is then placed with its title bar off the top - measured at -1215 px on 2026-08-25", bigger)
	}
}

// With nothing remembered it opens at the measured size, in the middle.
//
// The other half, and it has to be here rather than assumed: a first start with
// no opinion belongs in the middle of the screen, and OpenSize is a measured
// number with its own reasoning. A version that never centred would leave every
// first start in whatever corner the system chose.
func TestAFirstStartOpensAtTheMeasuredSizeInTheMiddle(t *testing.T) {
	for _, nothing := range []fyne.Size{
		{},
		fyne.NewSize(0, 900),
		fyne.NewSize(1200, 0),
	} {
		size, centre := window.HowToOpen(nothing)
		if size != window.OpenSize {
			t.Errorf("with %v remembered the window opens at %v rather than the measured %v",
				nothing, size, window.OpenSize)
		}
		if !centre {
			t.Errorf("with %v remembered the window is not centred, so a first start lands"+
				" wherever the system put it", nothing)
		}
	}
}

// A size with a nought in it is refused at BOTH ends by one predicate.
//
// Two copies of "is this a size" is how a window comes back as nothing on one
// machine and not another: the end that writes and the end that reads have to
// agree, and the only way to be sure they do is for there to be one of them.
// This asks the exported predicate directly, because it is the thing both ends
// call.
func TestASizeWithANoughtInItIsNotWorthRemembering(t *testing.T) {
	if !window.WorthRemembering(fyne.NewSize(800, 600)) {
		t.Error("a real size is not worth remembering, so nothing would ever be kept")
	}
	for _, empty := range []fyne.Size{
		{},
		fyne.NewSize(0, 600),
		fyne.NewSize(800, 0),
	} {
		if window.WorthRemembering(empty) {
			t.Errorf("%v is treated as a size, so a window closed while minimised comes back as nothing", empty)
		}
	}
}

func hasSuffix(whole, end string) bool {
	return len(whole) >= len(end) && whole[len(whole)-len(end):] == end
}
