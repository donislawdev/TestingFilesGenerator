package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/catalogue"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// No screen, and nothing in the catalogue of parts, stands in a theme override.
//
// An override is the toolkit's one public door into a subtree's theme, and it
// costs memory that never comes back. Fyne 2.8.1 gives an override a NEW scope
// at construction, at CreateRenderer and at every Refresh, and keys its parsed
// fonts by scope - so every rebuild of a screen followed by a new string drawn
// under an override parsed another ~7.4 MB set of fonts. Measured in the real
// window on 2026-09-23 (tools/probes/guilag, docs/GUI-MEMORY-2026-09-23.md
// section 4c): 26 sets after visiting the tabs, a live heap of 205 MB, and ten
// rebuilds each followed by a new size added 84 MB more. Every sentence, error
// area, caption and count of bytes was under one. Without them: 2 sets, 27 MB,
// and nothing added by the rebuilds.
//
// The open list of formats is asked too, since its rows left one on
// 2026-09-23 (section 4e): every opening was a new override and the letters
// its filter makes bold were new strings, so ten openings kept 158 MB. It is
// a popup rather than part of a screen, and it keeps its rows inside its
// renderer, where the walk of the screens does not go - so it is opened, typed
// into, and walked through what it draws.
func TestNoScreenStandsInAThemeOverride(t *testing.T) {
	content, _ := laidOutWindow(t)

	quiet, counts := 0, 0
	walk(content, func(o fyne.CanvasObject) {
		switch o.(type) {
		case *parts.QuietText:
			quiet++
		case *parts.ByteCount:
			counts++
		}
	})
	// The things that stood under an override until 2026-09-23 have to be on
	// the screen for their absence from one to mean anything: a subtitle on
	// each of three work screens, and the count of bytes beside a size.
	if quiet < 3 || counts < 1 {
		t.Fatalf("the window holds %d quiet line(s) and %d count(s) of bytes, so this guard is not "+
			"looking at the screens it asks about", quiet, counts)
	}

	cat := test.NewWindow(catalogue.Screen())
	t.Cleanup(cat.Close)

	_, _, list, filter := openFormatList(t)
	typeInto(filter, "p")
	rows := 0
	walkDrawn(list, func(o fyne.CanvasObject) {
		if _, is := o.(*parts.ListRow); is {
			rows++
		}
	})
	if rows == 0 {
		t.Fatal("the open list of formats draws no row, so its rows were not looked at")
	}

	for name, root := range map[string]fyne.CanvasObject{"the window": content, "the catalogue": cat.Content(), "the open list of formats": list} {
		overrides := 0
		walkDrawn(root, func(o fyne.CanvasObject) {
			if _, is := o.(*container.ThemeOverride); is {
				overrides++
			}
		})
		if overrides > 0 {
			t.Errorf("%s holds %d theme override(s), and each one parses the fonts again in a scope of its own - "+
				"take the room off with a layout (inkTight) and name the colour in the words (QuietText)", name, overrides)
		}
	}
}

// walkDrawn visits everything a tree draws: into a container's objects and into
// every widget's renderer, which is where a widget keeps what it built - the
// rows of an open list among them. walk stops at a widget it has no case for,
// and that is the half this guard needs.
func walkDrawn(o fyne.CanvasObject, visit func(fyne.CanvasObject)) {
	if o == nil {
		return
	}
	visit(o)
	switch v := o.(type) {
	case *fyne.Container:
		for _, child := range v.Objects {
			walkDrawn(child, visit)
		}
	case fyne.Widget:
		for _, child := range test.WidgetRenderer(v).Objects() {
			walkDrawn(child, visit)
		}
	}
}
