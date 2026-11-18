package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// The title bar is asked for dark again after every change of settings, for
// the window it belongs to.
//
// The toolkit answers each change by setting the title bar from the desktop's
// mode for apps, so a bar darkened once at start turns white again the first
// time somebody switches the desktop's mode with the window open (O360). Two
// changes, two asks - and none before anything changed, so an ask made once
// at registration does not pass for the listener.
func TestTheTitleBarIsAskedForAgainAfterEveryChangeOfSettings(t *testing.T) {
	a := test.NewTempApp(t)
	w := test.NewTempWindow(t, nil)
	var asked []fyne.Window
	gui.KeepTitleBarDark(a.Settings(), w, func(x fyne.Window) bool {
		asked = append(asked, x)
		return true
	})
	if len(asked) != 0 {
		t.Fatalf("the title bar was asked for %d time(s) before anything changed", len(asked))
	}
	a.Settings().SetTheme(parts.Theme())
	a.Settings().SetTheme(parts.Theme())
	if len(asked) != 2 || asked[0] != w || asked[1] != w {
		t.Errorf("two changes of settings asked for the title bar %d time(s), of %v.\n"+
			"What to do: KeepTitleBarDark has to ask again after every change, because the toolkit "+
			"sets the bar from the desktop's mode on each of them.", len(asked), asked)
	}
	// A window of the toolkit's test driver has no native handle, and is left
	// alone rather than asked about - on every system.
	if gui.DarkTitleBar(w) {
		t.Error("a window with no native handle answered that its title bar is dark")
	}
}
