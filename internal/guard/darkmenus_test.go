package guard

import (
	"runtime"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui"
)

// The program asks Windows to draw its menus dark, and the ask lands.
//
// Reported from use on 2026-08-18, with a picture: the window is dark and the
// menu Windows puts over it when you right click the title bar came up white.
//
// This guard exists because the thing it watches cannot be photographed. Every
// other window guard here compares a tree or a picture of what this program
// draws, and that menu is drawn by Windows in the non client area - outside the
// canvas, outside the toolkit, outside anything a stored screenshot can reach.
// What can be checked is whether the request got as far as the system, and that
// is what this asks.
//
// It is also the guard on an UNDOCUMENTED call. SetPreferredAppMode is not
// exported by name and is reached by ordinal, so the failure that matters is not
// "the menu looks wrong" but "the ordinal stopped resolving" - which is silent,
// and which this turns red.
//
// What it deliberately does NOT claim: that the menu came out dark. That is
// Windows' decision after being asked, and no test in this project can see it.
// Somebody has to look, once, and the code says so too.
func TestTheProgramAsksWindowsForDarkMenus(t *testing.T) {
	if runtime.GOOS != "windows" {
		// Not a gap. There is nothing to ask for: other desktops draw their own
		// menus from their own theme.
		if gui.PreferDarkMenus() {
			t.Errorf("something answered the Windows only question on %s", runtime.GOOS)
		}
		t.Skipf("menus in the non client area are a Windows matter, and this is %s", runtime.GOOS)
	}

	if !gui.PreferDarkMenus() {
		t.Error("Windows did not take the request for dark menus.\n" +
			"Either uxtheme.dll would not load, or ordinal 135 no longer resolves - and\n" +
			"the second is the one to expect, because that entry point is undocumented\n" +
			"and reached by number. The window still works. Its title bar menu goes back\n" +
			"to being white against a dark program, which is what this was for.")
	}

	// Twice, because it is called once per run today and a future caller might
	// reasonably call it again - and a call that only works the first time would
	// be a trap laid for that person.
	if !gui.PreferDarkMenus() {
		t.Error("the second request was refused, so this only works once per process")
	}
}
