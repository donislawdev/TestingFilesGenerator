package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

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
	bigger := fyne.NewSize(window.LargestOpening.Width+600, window.LargestOpening.Height+400)
	// With the screens wanting something else entirely, so that the answer
	// cannot be the want by coincidence.
	size, centre := window.HowToOpen(bigger, fyne.NewSize(window.LargestOpening.Width, 700))
	if size != bigger {
		t.Errorf("the window was %v when it was closed and opens at %v", bigger, size)
	}
	if centre {
		t.Errorf("a remembered %v is centred, and a window bigger than the screen it comes back on"+
			" is then placed with its title bar off the top - measured at -1215 px on 2026-08-25", bigger)
	}
}

// With nothing remembered it opens as tall as the screens want, no taller
// than the ceiling, in the middle.
//
// Three answers and each has to be here rather than assumed. The height is
// the screens' want, because until 2026-09-16 it was a number measured once
// against the forms of 2026-08-19 and typed in, and the forms moved 300 px
// under it (O202) - GUI rule 14, worked out and never measured. The ceiling
// holds, because it is the one fact about somebody else's screen this program
// has and a window taller than the screen cannot be reached at the bottom. And
// a first start with no opinion belongs in the middle of the screen - a
// version that never centred would leave every first start in whatever corner
// the system chose.
//
// A want with a nought in it is no want, and gets the ceiling: the same
// predicate that refuses a remembered nought, so a screen that could not be
// measured does not open as a window of nothing.
func TestAFirstStartOpensAsTallAsTheScreensWantInTheMiddle(t *testing.T) {
	ceiling := window.LargestOpening
	wanted := fyne.NewSize(ceiling.Width, ceiling.Height-183)
	for _, tc := range []struct {
		name         string
		remembered   fyne.Size
		wanted, want fyne.Size
	}{
		{"nothing remembered, the screens want less than the ceiling", fyne.Size{}, wanted, wanted},
		{"a remembered nought is nothing remembered", fyne.NewSize(0, 900), wanted, wanted},
		{"a remembered nought the other way", fyne.NewSize(1200, 0), wanted, wanted},
		{"the screens want more than the ceiling", fyne.Size{}, fyne.NewSize(ceiling.Width, ceiling.Height+500), ceiling},
		{"the screens want exactly the ceiling", fyne.Size{}, ceiling, ceiling},
		{"a want with a nought in it is no want", fyne.Size{}, fyne.NewSize(ceiling.Width, 0), ceiling},
		{"no want at all", fyne.Size{}, fyne.Size{}, ceiling},
	} {
		size, centre := window.HowToOpen(tc.remembered, tc.wanted)
		if size != tc.want {
			t.Errorf("%s: with %v remembered and %v wanted the window opens at %v rather than %v",
				tc.name, tc.remembered, tc.wanted, size, tc.want)
		}
		if !centre {
			t.Errorf("%s: the window is not centred, so a first start lands wherever the system put it", tc.name)
		}
	}
}

// And what the screens want is what shows every work screen whole: laid out
// at the size Open hands back, no work screen scrolls - unless the ceiling
// stopped the window growing, which is the one reason a form may be cut.
//
// Asked of the real screens through Open rather than of HowToOpen alone,
// because HowToOpen is arithmetic on two numbers and the number that matters
// is the one Open works out: a want that left out a screen, or forgot the
// strip above the screens, would pass every case above and still open a
// window whose batch screen scrolls from the first frame.
func TestTheFirstOpeningShowsEveryWorkScreenWhole(t *testing.T) {
	ourTheme(t)
	host := newFakeHost(t)
	wanted := window.Open(host)
	size, _ := window.HowToOpen(fyne.Size{}, wanted)
	w := test.NewWindow(host.content)
	t.Cleanup(w.Close)
	w.Resize(size)
	host.content.Refresh()
	w.Resize(size)

	if size.Height >= window.LargestOpening.Height {
		t.Logf("the screens want %.0f px and the ceiling is %.0f, so a screen is allowed to scroll today",
			wanted.Height, window.LargestOpening.Height)
	}
	checked := 0
	tightest := float32(-1)
	for _, tab := range []string{text.TabOneTarget(), text.TabPresets(), text.TabRecipe()} {
		screen := selectTab(t, host.content, tab)
		w.Resize(fyne.NewSize(size.Width, size.Height-1))
		w.Resize(size)
		scroll := scrollIn(screen)
		if scroll == nil {
			t.Fatalf("the %s screen has no scroll, so this guard cannot say whether it fits", tab)
		}
		form, room := scroll.Content.MinSize().Height, scroll.Size().Height
		if form > room && size.Height < window.LargestOpening.Height {
			t.Errorf("at the first opening of %v the %s screen's form needs %.0f px and gets %.0f, so it"+
				" scrolls from the first frame although the window had room to grow", size, tab, form, room)
		}
		if slack := room - form; tightest < 0 || slack < tightest {
			tightest = slack
		}
		checked++
	}
	if checked != 3 {
		t.Fatalf("checked %d screens, and there are three work screens", checked)
	}
	// And no taller than that. The tallest screen fits with nothing to spare,
	// because the height is worked out from it - a window that opened taller
	// would be the band of nothing under the form that O202 is about, back
	// under another number.
	if tightest > 1 && size.Height < window.LargestOpening.Height {
		t.Errorf("the first opening is %v and the tallest work screen still has %.0f px to spare under"+
			" its form, so the window opens taller than the screens want", size, tightest)
	}
	t.Logf("first opening %v: every work screen shows whole, the tallest with %.2f px to spare", size, tightest)
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
