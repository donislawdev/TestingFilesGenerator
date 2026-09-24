package guard

import (
	"testing"
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// The window gives memory back once it has been left alone, and only then.
//
// After a spell of work the process kept 189-206 MB for as long as the window
// stood idle: the toolkit lets go of the renderers of what left the screen only
// while drawing a frame, and an idle window draws none. The window now waits
// out a quiet, asks for one frame, and gives the memory back a little later -
// 252-263 MB to 117-119 MB in the real window (docs/GUI-MEMORY-2026-09-23.md
// section 4j).
//
// What this can hold is the timing and the order. That the memory really comes
// back is a property of the toolkit and of Go, and only the real window shows
// it - tools/probes/guilag -quiet -nudge.
//
// The waits are asked about as well as the order. The first has to outlast the
// minute the toolkit keeps a renderer, or the frame comes while they are still
// valid and nothing is let go. The second has to outlast the ten seconds between
// two of the toolkit's cleans, or the memory goes back before the renderers do.
func TestTheWindowGivesMemoryBackOnceItHasBeenLeftAlone(t *testing.T) {
	host := newFakeHost(t)
	window.Open(host)
	host.quiet = &quietClock{}
	content := tabNamed(t, host.content, text.TabOneTarget())

	fill(t, content, text.FieldSeed(), "5")
	quiet := host.quiet.waiting()
	if len(quiet) != 1 {
		t.Fatalf("typing into a box left %d wait(s) for quiet, expected one - the window cannot tell when it has been left alone", len(quiet))
	}
	if quiet[0].after <= time.Minute {
		t.Errorf("the window waits %v of quiet, and the toolkit keeps a renderer for a minute - the frame would come while they are still valid", quiet[0].after)
	}

	host.quiet.fireAll()
	if host.releases != 0 {
		t.Fatal("memory was given back at the end of the quiet, before the frame that lets the renderers go")
	}
	frame := host.quiet.waiting()
	if len(frame) != 1 {
		t.Fatalf("after the quiet the window waits for %d thing(s), expected the one release", len(frame))
	}
	if frame[0].after <= 10*time.Second {
		t.Errorf("memory goes back %v after the frame, and the toolkit cleans at most once in ten seconds - it would sometimes go back before the renderers", frame[0].after)
	}

	host.quiet.fireAll()
	if host.releases != 1 {
		t.Fatalf("the quiet and the frame were waited out and memory was given back %d time(s), expected once", host.releases)
	}
	host.quiet.fireAll()
	if host.releases != 1 || len(host.quiet.waiting()) != 0 {
		t.Errorf("with nothing done since, memory was given back %d time(s) and %d wait(s) are left - it has to happen once per quiet, not keep going",
			host.releases, len(host.quiet.waiting()))
	}
}

// Anything done during the wait starts the quiet over, and takes back a
// release already waiting.
func TestSomethingDoneDuringTheWaitStartsTheQuietOver(t *testing.T) {
	host := newFakeHost(t)
	window.Open(host)
	host.quiet = &quietClock{}
	content := tabNamed(t, host.content, text.TabOneTarget())

	fill(t, content, text.FieldSeed(), "5")
	host.quiet.fireAll() // the quiet waited out, so the release is waiting
	if len(host.quiet.waiting()) != 1 {
		t.Fatal("after the quiet no release is waiting, so this guard is not in the state it asks about")
	}

	fill(t, content, text.FieldSeed(), "6")
	host.quiet.fireAll() // the new quiet, which has to come first
	if host.releases != 0 {
		t.Error("something was typed while the release was waiting and memory was given back anyway, in the middle of somebody working")
	}
	host.quiet.fireAll()
	if host.releases != 1 {
		t.Errorf("after the new quiet and its frame memory was given back %d time(s), expected once", host.releases)
	}
}

// No memory goes back while work owns the window - the run is what is using it.
func TestNoMemoryIsGivenBackWhileWorkIsGoing(t *testing.T) {
	host, content, hold := heldScreen(t)
	host.quiet = &quietClock{}
	fill(t, content, text.FieldOutputDir(), t.TempDir())
	press(t, content, text.ButtonGenerate())

	hold.look(func() {
		if len(host.quiet.waiting()) == 0 {
			t.Fatal("the run started and no wait for quiet was asked for, so this guard asks about nothing")
		}
		host.quiet.fireAll() // the quiet ends during the run
		host.quiet.fireAll()
		if host.releases != 0 {
			t.Error("memory was given back while a run owned the window")
		}
		if len(host.quiet.waiting()) == 0 {
			t.Error("the quiet ended during a run and nothing waits for the next one, so memory is never given back after it")
		}
	})
	join(host)

	host.quiet.fireAll()
	host.quiet.fireAll()
	if host.releases != 1 {
		t.Errorf("the run ended, the quiet was waited out, and memory was given back %d time(s), expected once", host.releases)
	}
}
