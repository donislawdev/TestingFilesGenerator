package guard

import (
	"strings"
	"testing"

	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// The screen that says what this program is also says what to do with it.
//
// Counted off the stored screen on 2026-09-22: the thesis had one sentence and
// everything under it was the licence notice and the list of what the binary
// carries. Four fifths of the one screen somebody opens to find out what they
// have answered a question about redistribution - a real question, and not the
// one anybody has first. What to do with the program was on no screen in the
// window at all.

// TestTheAboutScreenSaysHowToUseTheProgram.
//
// Three questions, and the third is the one that makes the other two worth
// asking: the steps have to stand ABOVE the licence. A section carrying the
// same words at the bottom of a page of notices is the defect with a heading
// on it, and a guard that only asked whether the words were somewhere on the
// screen would be green for it.
func TestTheAboutScreenSaysHowToUseTheProgram(t *testing.T) {
	content, _ := laidOutWindow(t)
	about := tabContent(t, content, text.TabAbout())

	steps := text.HowToUseSteps()
	if len(steps) == 0 {
		t.Fatal("the window offers no steps at all, so there is nothing to look for")
	}

	said := shownText(about)
	for _, step := range steps {
		if !strings.Contains(said, step) {
			t.Errorf("the About screen does not say %q.\nIt says:\n%s", step, said)
		}
	}

	heading, ok := labelBox(about, text.SectionHowToUse())
	if !ok {
		t.Fatalf("the About screen has no section headed %q, so the steps are loose on a page of notices",
			text.SectionHowToUse())
	}
	licence, ok := labelBox(about, text.SectionLicence())
	if !ok {
		t.Fatalf("the About screen has no section headed %q, so this guard cannot ask which comes first",
			text.SectionLicence())
	}
	if heading.Y >= licence.Y {
		t.Errorf("%q starts at y=%.0f and %q at y=%.0f, so what to do with this program is under "+
			"the licence rather than over it",
			text.SectionHowToUse(), heading.Y, text.SectionLicence(), licence.Y)
	}

	// And the one sentence that was already there is still there. The steps
	// are an addition rather than a replacement, and a "fix" that swallowed
	// the thesis would be the screen losing the thing it is for.
	if !strings.Contains(said, text.AboutTagline()) {
		t.Errorf("the About screen no longer says what this program is:\n%s", said)
	}
}
