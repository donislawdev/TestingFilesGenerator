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
	// The tab and not the window. Both work screens have a field of this name,
	// and controlUnder answers with the LAST one it walks past rather than the
	// first - so asking the window filled the preset screen's box while reading
	// as though it had filled this one.
	generate := tabNamed(t, host.content, text.TabOneTarget)
	fill(t, generate, text.FieldOutputDir, chosen)

	preset := selectTab(t, host.content, text.TabPresets)
	if got := entryUnder(t, preset, text.FieldOutputDir).Text; got != chosen {
		t.Errorf("the generate screen was pointed at %q and the preset screen says %q.\n"+
			"Two boxes that start the same and drift apart is worse than one that is wrong, "+
			"because both look filled in.", chosen, got)
	}

	// And back, because a fix that carries the value one way leaves the other
	// direction exactly as confusing as before.
	const second = "C:\\third\\place"
	fill(t, preset, text.FieldOutputDir, second)
	generate = selectTab(t, host.content, text.TabOneTarget)
	if got := entryUnder(t, generate, text.FieldOutputDir).Text; got != second {
		t.Errorf("the preset screen was pointed at %q and the generate screen says %q", second, got)
	}
}
