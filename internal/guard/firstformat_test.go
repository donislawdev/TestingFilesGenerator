package guard

import (
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// The format a screen opens on, asked of the registry rather than written down.
//
// Three guards used to name bmp here, because bmp sorted first for as long as
// anybody had looked. AVIF arrived on 2026-08-29, sorted ahead of it, and all
// three went red at once - not because the window broke, but because they were
// describing a fact about the alphabet as though it were a fact about the
// screen. The window itself has never named a format: it takes the first the
// registry hands it, on the reasoning that picking one by name would be a
// preference nothing else in this tool holds.
func firstFormat() string {
	ids := format.IDs()
	if len(ids) == 0 {
		panic("guard: the registry has no formats, so no screen can open on one")
	}
	return ids[0]
}

// And the fact those three guards lean on, asserted where they can see it: the
// screen opens on the first format rather than on one chosen by name.
func TestTheFormatAScreenOpensOnIsTheFirstOneRegistered(t *testing.T) {
	ids := format.IDs()
	if len(ids) < 2 {
		t.Fatalf("the registry has %d format(s), which is too few for first to mean anything", len(ids))
	}

	for i := 1; i < len(ids); i++ {
		if ids[i] < ids[i-1] {
			t.Fatalf("the registry hands its formats back in no order - %q comes after %q - so first is not a thing a guard can lean on",
				ids[i], ids[i-1])
		}
	}

	screen := window.NewGenerate(newFakeHost(t))
	shown := textIn(screen.Object())
	if !strings.Contains(shown, firstFormat()) {
		t.Errorf("the registry starts with %q and the screen does not show it. The screen says:\n%s",
			firstFormat(), shown)
	}
}
