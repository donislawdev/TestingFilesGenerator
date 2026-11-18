package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// The title bar is asked for dark again after every change of settings, for
// the window it belongs to, and the ask lands after the toolkit's own answer.
//
// The toolkit answers each change by setting the title bar from the desktop's
// mode for apps, so a bar darkened once at start turns white again the first
// time somebody switches the desktop's mode with the window open (O360). Its
// listener is added when the loop starts (fyne v2.8.2, internal/driver/glfw,
// runGL), after the one KeepTitleBarDark adds, so an ask made straight from
// our listener is the one overwritten. The test driver runs fyne.Do at once,
// so this guard holds it back the way the loop does, until every listener of
// the change has run.
//
// The canary is that ask made straight from the listener: under the same two
// changes it has to leave the bar light.
func TestTheTitleBarIsAskedForAgainAfterEveryChangeOfSettings(t *testing.T) {
	straight := func(s fyne.Settings, w fyne.Window, darken func(fyne.Window) bool) {
		s.AddListener(func(fyne.Settings) { darken(w) })
	}
	if _, asked, dark := titleBarAfterTwoChanges(t, straight); dark {
		t.Fatalf("canary: an ask made straight from the listener left the bar dark after %d ask(s), "+
			"so the toolkit's answer was not the last word here and this guard cannot see the order", len(asked))
	}

	w, asked, dark := titleBarAfterTwoChanges(t, gui.KeepTitleBarDark)
	if len(asked) != 2 || asked[0] != w || asked[1] != w {
		t.Errorf("two changes of settings asked for the title bar %d time(s), of %v.\n"+
			"What to do: KeepTitleBarDark has to ask again after every change, because the toolkit "+
			"sets the bar from the desktop's mode on each of them.", len(asked), asked)
	}
	if !dark {
		t.Error("the toolkit's answer to a change came after the ask, so the bar ended light.\n" +
			"What to do: KeepTitleBarDark has to ask through fyne.Do, which the loop runs once every " +
			"listener of the change, the toolkit's among them, has run.")
	}
	// A window of the toolkit's test driver has no native handle, and is left
	// alone rather than asked about - on every system.
	if gui.DarkTitleBar(w) {
		t.Error("a window with no native handle answered that its title bar is dark")
	}
}

// titleBarAfterTwoChanges has keep look after a window, adds the toolkit's
// answer after it, as the loop does, and changes the settings twice, each
// followed by the work fyne.Do queued. It says the window, every ask, and
// whether the bar ended dark. Nothing may be asked before the first change.
func titleBarAfterTwoChanges(t *testing.T, keep func(fyne.Settings, fyne.Window, func(fyne.Window) bool)) (
	w fyne.Window, asked []fyne.Window, dark bool) {
	t.Helper()
	a := newTempTestApp(t)
	w = test.NewTempWindow(t, nil)
	loop := &heldDriver{Driver: a.Driver()}
	// Put back by the clean-up of the application, which makes a new one.
	fyne.SetCurrentApp(heldApp{App: a, driver: loop})
	keep(a.Settings(), w, func(x fyne.Window) bool {
		asked = append(asked, x)
		dark = true
		return true
	})
	// The toolkit's answer: the desktop's mode for apps, light here.
	a.Settings().AddListener(func(fyne.Settings) { dark = false })
	if len(asked) != 0 {
		t.Fatalf("the title bar was asked for %d time(s) before anything changed", len(asked))
	}
	for range 2 {
		a.Settings().SetTheme(parts.Theme())
		loop.drain()
	}
	return w, asked, dark
}

// heldDriver is the test driver with fyne.Do held back until drain, as the
// toolkit's loop holds it until the work in hand is done.
type heldDriver struct {
	fyne.Driver
	queued []func()
}

func (d *heldDriver) DoFromGoroutine(fn func(), _ bool) { d.queued = append(d.queued, fn) }

func (d *heldDriver) drain() {
	for len(d.queued) > 0 {
		fn := d.queued[0]
		d.queued = d.queued[1:]
		fn()
	}
}

// heldApp is the test application handing out heldDriver, the driver fyne.Do
// asks the current application for.
type heldApp struct {
	fyne.App
	driver *heldDriver
}

func (a heldApp) Driver() fyne.Driver { return a.driver }
