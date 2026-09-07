package guard

import (
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// The address the Donate button leads to is also READABLE, in words, on a
// screen.
//
// This is not decoration and it is not a second way to say the button works.
// Handing an address to the desktop can fail - a machine with no browser
// registered is the ordinary case - and internal/gui swallows that refusal on
// purpose, with the reason written above OpenLink: there is nothing useful to
// say to somebody who pressed a button out of curiosity, and no harm done,
// because the address is on the About screen for anybody who wants to type it.
//
// That last clause was FALSE from the day the button was written until
// 2026-09-07. text.SupportURL had exactly one use in the whole tree, as the
// button's destination, so it appeared on no screen at all: a person whose
// desktop did nothing pressed Donate, saw nothing happen, and had nowhere to
// go. The decision to stay quiet was sound and rested on a fallback nobody had
// built. Found by tools/probes/guardonly, which reports exported definitions
// nothing names, not by reading the code.
//
// So the silence and the readable address are ONE decision, and this guard is
// what keeps them together. Delete the Support section and this goes red, which
// is the point: taking the address off the screen turns a considered silence
// back into a dead end.
func TestTheSupportAddressIsReadableOnAScreen(t *testing.T) {
	shown := textIn(window.About(newFakeHost(t)))

	if !strings.Contains(shown, text.SupportURL) {
		t.Errorf("the About screen does not carry %q, so somebody whose desktop cannot open\n"+
			"the Donate button has no way to reach the page - and internal/gui stays quiet\n"+
			"about that failure precisely because this screen is supposed to carry it.\n"+
			"The screen says:\n%s", text.SupportURL, shown)
	}
}

// The address is on the screen as its own line rather than buried in a
// sentence.
//
// Somebody typing it into a browser by hand is exactly the reader this exists
// for, and an address wrapped inside a paragraph is one they have to pick out
// of prose. Asked as "a line that is the address and nothing else", which is
// the shape a person can select in one gesture.
func TestTheSupportAddressStandsOnItsOwnLine(t *testing.T) {
	shown := textIn(window.About(newFakeHost(t)))

	for _, line := range strings.Split(shown, "\n") {
		if strings.TrimSpace(line) == text.SupportURL {
			return
		}
	}
	t.Errorf("the About screen names %q only inside a longer line, so somebody copying it\n"+
		"has to pick it out of a sentence. The screen says:\n%s", text.SupportURL, shown)
}
