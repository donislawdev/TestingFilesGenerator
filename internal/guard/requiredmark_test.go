package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// The star on a field means what the run does, rather than what somebody typed.
//
// This is the half that matters. A screen names the settings it cannot run
// without in one line, and a line like that is a second opinion about the
// engine - the copy that drifts, and drifts silently, because a star is not a
// thing any other test looks at. So nothing here reads that line and believes
// it: each box is emptied in turn, the run is asked, and the star that is
// ACTUALLY DRAWN is compared against whether the run actually refused. Neither
// side of the comparison is the declaration.
//
// The definition is deliberately "empty THIS box, with everything else
// answered". It is what makes the three ways of saying how big come out
// unmarked without an exception being written for them: emptying the size while
// a size range is filled leaves a run the engine is happy with, so no one of
// the three is required on its own, and three stars would tell somebody to fill
// in all three. The sentence above them carries that rule instead.
//
// What this cannot see is a setting no screen draws a box for. That is the
// parity guard's question rather than this one's.
func TestAStarIsOnEveryBoxTheRunWillNotDoWithout(t *testing.T) {
	for _, s := range screensWithBoxes(t) {
		for _, b := range s.boxes {
			host, content := s.open(t)
			s.baseline(t, content)
			// Where a rule binds several boxes together, emptying one of them
			// is only a fair question once the same question has been answered
			// another way. Without this the size box of the batch screen came
			// out required - correctly, because the size range and the boundary
			// beside it were empty too, so what the run refused was the three of
			// them rather than that one. Found by this guard on its first run.
			if b.answeredInstead != nil {
				b.answeredInstead(t, content)
			}

			entryUnder(t, content, b.label).SetText("")
			press(t, content, text.ButtonPreview())
			// A refusal never starts a worker and an accepted plan does, so the
			// second case leaves one to wait for. O124: under the test driver
			// fyne.Do runs on the calling goroutine, so a preview still in
			// flight when this iteration ends shapes text inside the next one.
			join(host)

			refused := edgeOf(t, content, b.label).StrokeWidth > 0
			starred := starsOnScreen(content)[b.label]

			switch {
			case refused && !starred:
				t.Errorf("%s: %q is empty and the run refuses it, and its name carries no star - "+
					"so the only way to learn it had to be filled in is to press a button",
					s.name, b.label)
			case !refused && starred:
				t.Errorf("%s: %q carries a star saying it must be filled in, and the run is happy "+
					"with it empty - so the star asks for work the tool does not need",
					s.name, b.label)
			}
		}
	}
}

// The star a screen declared is the star that is drawn.
//
// The guard above holds what is drawn against the engine. This holds it against
// the declaration, and both halves are needed for one reason: a screen names
// these BEFORE it builds its fields, so naming them afterwards would draw no
// star at all while the line declaring them still read correctly. That ordering
// is exactly the kind of thing that is right the day it is written.
//
// It walks the whole registry rather than a list, so a field added later is
// held by this without anybody remembering to add it anywhere.
func TestTheStarsDrawnAreTheOnesTheScreenDeclared(t *testing.T) {
	host := newFakeHost(t)
	generate := window.NewGenerate(host)
	presets := window.NewPreset(host)
	batches := window.NewRecipe(host)

	for _, s := range []struct {
		name   string
		object fyne.CanvasObject
		fields *parts.Fields
	}{
		{"the generate screen", generate.Object(), generate.Fields()},
		{"the preset screen", presets.Object(), presets.Fields()},
		{"the batch screen", batches.Object(), batches.Fields()},
	} {
		drawn := starsOnScreen(s.object)

		marked := 0
		for _, f := range s.fields.All() {
			want := s.fields.Required(f.Setting)
			if want {
				marked++
			}
			if got := drawn[f.Label]; got != want {
				t.Errorf("%s: %q is declared required=%v and drawn with a star=%v",
					s.name, f.Label, want, got)
			}
		}
		if marked == 0 {
			t.Errorf("%s: nothing on it is declared required, so this compared nothing", s.name)
		}
		t.Logf("%s: %d field(s) carry a star", s.name, marked)
	}
}

// starsOnScreen is the name of every field whose heading line carries the mark.
//
// Found by type in a flat row, which is the shape headingRow builds: the name
// first, then whatever qualifies it. Searched rather than indexed, for the
// reason detailButtonIn gives.
func starsOnScreen(o fyne.CanvasObject) map[string]bool {
	stars := map[string]bool{}
	walk(o, func(obj fyne.CanvasObject) {
		row, ok := obj.(*fyne.Container)
		if !ok || len(row.Objects) < 2 {
			return
		}
		head, isLabel := row.Objects[0].(*widget.Label)
		if !isLabel {
			return
		}
		for _, item := range row.Objects[1:] {
			if _, star := item.(*parts.RequiredMark); star {
				stars[head.Text] = true
			}
		}
	})
	return stars
}

// boxUnderTest is one box to empty, and what else has to be said first.
type boxUnderTest struct {
	label string
	// answeredInstead answers the same question through a different box, for
	// the settings where a rule binds several of them together. Nil where the
	// box stands on its own, which is nearly all of them.
	answeredInstead func(*testing.T, fyne.CanvasObject)
}

// box is one box that answers for itself.
func box(label string) boxUnderTest { return boxUnderTest{label: label} }

// screenUnderTest is one screen, how to open a fresh one, and what has to be
// answered on it before an empty box is a fair question.
type screenUnderTest struct {
	name  string
	boxes []boxUnderTest
	open  func(*testing.T) (*fakeHost, fyne.CanvasObject)
	// baseline fills in whatever the screen needs to be runnable. Without it
	// the batch screen refuses several boxes at once and every one of them
	// looks required.
	baseline func(*testing.T, fyne.CanvasObject)
}

// screensWithBoxes is the three screens and the boxes on them worth emptying.
//
// Opened through window.Open rather than by building a screen directly, and
// that is not a detail: Open is what sets the close intercept, and the close
// intercept is what join waits on. Built directly, a preview left running walks
// into the next iteration and panics inside the font shaper - O124, measured
// again here on the first run of this guard.
func screensWithBoxes(t *testing.T) []screenUnderTest {
	t.Helper()
	out := t.TempDir()

	onTab := func(name string) func(*testing.T) (*fakeHost, fyne.CanvasObject) {
		return func(t *testing.T) (*fakeHost, fyne.CanvasObject) {
			t.Helper()
			host := newFakeHost(t)
			window.Open(host)
			if host.content == nil {
				t.Fatal("opening the window put no screen in it")
			}
			t.Cleanup(func() { join(host) })
			return host, selectTab(t, host.content, name)
		}
	}

	return []screenUnderTest{
		{
			name: "the generate screen",
			boxes: []boxUnderTest{box(text.FieldSize()), box(text.FieldCount()),
				box(text.FieldTargetID()), box(text.FieldNameTemplate()), box(text.FieldSeed()),
				box(text.FieldOutputDir())},
			open:     onTab(text.TabOneTarget()),
			baseline: func(t *testing.T, c fyne.CanvasObject) { fill(t, c, text.FieldOutputDir(), out) },
		},
		{
			name:     "the preset screen",
			boxes:    []boxUnderTest{box(text.FieldSeed()), box(text.FieldOutputDir())},
			open:     onTab(text.TabPresets()),
			baseline: func(t *testing.T, c fyne.CanvasObject) { fill(t, c, text.FieldOutputDir(), out) },
		},
		{
			name: "the batch screen",
			boxes: []boxUnderTest{
				box(text.FieldTargetID()), box(text.FieldCount()),
				// The three ways of saying how big are one question with one
				// box on the screen at a time, so each of them is asked about
				// with the switch on it. Before 2026-08-25 they stood side by
				// side and the awkward one was the size, which had to be
				// answered another way first - now the awkward thing is that
				// two of the three are not on the screen at all, and a box
				// nobody can see is not a box a star can speak for.
				{label: text.FieldSize(), answeredInstead: func(t *testing.T, c fyne.CanvasObject) {
					chooseSizeWay(t, c, text.SizeWayExact())
				}},
				{label: text.FieldSizeRange(), answeredInstead: func(t *testing.T, c fyne.CanvasObject) {
					chooseSizeWay(t, c, text.SizeWayRange())
				}},
				{label: text.FieldBoundary(), answeredInstead: func(t *testing.T, c fyne.CanvasObject) {
					chooseSizeWay(t, c, text.SizeWayBoundary())
				}},
				box(text.FieldNameTemplate()), box(text.FieldGroup()),
				box(text.FieldManifest()), box(text.FieldSeed()), box(text.FieldOutputDir())},
			open: onTab(text.TabRecipe()),
			baseline: func(t *testing.T, c fyne.CanvasObject) {
				fill(t, c, text.FieldOutputDir(), out)
				fill(t, c, text.FieldTargetID(), "files")
				fill(t, c, text.FieldSize(), "1kb")
			},
		},
	}
}
