package guard

import (
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// Both screens answer "where do the files go" with the same directory.
//
// Reported from use on 2026-08-11, and it is the kind of defect a screenshot
// does not show: each screen held its own box, both starting at the working
// directory, so they agreed until somebody changed one. Files were written to
// a chosen directory, the preset screen was opened, and its run went somewhere
// else - with nothing on screen saying so, because both boxes looked filled in
// and neither looked wrong.
//
// It is checked by moving between the screens the way a person does, through
// the buttons, rather than by calling the accessors. Calling them would prove
// the getter and the setter agree, which nobody doubted.
func TestBothScreensAgreeWhereTheFilesGo(t *testing.T) {
	host := &fakeHost{}
	window.Open(host)
	if host.content == nil {
		t.Fatal("opening the window put no screen in it")
	}

	const chosen = "C:\\somewhere\\else"
	generate := host.content
	fill(t, generate, text.FieldOutputDir, chosen)

	press(t, generate, text.ButtonPresets)
	preset := host.content
	if got := entryUnder(t, preset, text.FieldOutputDir).Text; got != chosen {
		t.Errorf("the generate screen was pointed at %q and the preset screen says %q.\n"+
			"Two boxes that start the same and drift apart is worse than one that is wrong, "+
			"because both look filled in.", chosen, got)
	}

	// And back, because a fix that carries the value one way leaves the other
	// direction exactly as confusing as before.
	const second = "C:\\third\\place"
	fill(t, preset, text.FieldOutputDir, second)
	press(t, preset, text.ButtonOneTarget)
	if got := entryUnder(t, host.content, text.FieldOutputDir).Text; got != second {
		t.Errorf("the preset screen was pointed at %q and the generate screen says %q", second, got)
	}
}
