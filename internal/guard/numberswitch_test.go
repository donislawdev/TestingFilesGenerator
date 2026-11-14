package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// The switch beside the output directory on the three work screens: a run into
// a directory holding an earlier one is added beside it under a number, or
// refused when the switch is off (docs/RUN-FOLDERS-2026-11-14.md).

// toggleUnder is the switch of a labelled field.
func toggleUnder(t *testing.T, o fyne.CanvasObject, label string) *parts.Toggle {
	t.Helper()
	toggle, ok := controlUnder(o, label).(*parts.Toggle)
	if !ok {
		t.Fatalf("the field %q is not a switch", label)
	}
	return toggle
}

// openOnTab opens the window on the fake host, on tab, with the two settings
// the batch screen cannot run without answered.
func openOnTab(t *testing.T, tab string) (*fakeHost, fyne.CanvasObject) {
	t.Helper()
	host := newFakeHost(t)
	window.Open(host)
	if host.content == nil {
		t.Fatal("opening the window put no screen in it")
	}
	t.Cleanup(func() { join(host) })
	content := selectTab(t, host.content, tab)
	if tab == text.TabRecipe() {
		fill(t, content, text.FieldTargetID(), "files")
		fill(t, content, text.FieldSize(), "1kb")
	}
	return host, content
}

// On every work screen the switch is there, on, and it decides what a run into
// a directory holding an earlier one does: added beside it under a number, or
// refused under the output directory with nothing written.
func TestTheSwitchBesideTheFolderDecidesWhatASecondRunDoesOnEveryScreen(t *testing.T) {
	for _, tab := range []string{text.TabOneTarget(), text.TabPresets(), text.TabRecipe()} {
		t.Run(tab, func(t *testing.T) {
			host, content := openOnTab(t, tab)
			dir := t.TempDir()
			seedDirectory(t, dir)
			before := contentsOf(t, dir)
			entryUnder(t, content, text.FieldOutputDir()).SetText(dir)

			numbers := toggleUnder(t, content, text.FieldNumberIfTaken())
			if !numbers.Checked {
				t.Fatalf("%s opens with the switch off, and nobody turned it off", tab)
			}
			press(t, content, text.ButtonGenerate())
			join(host)
			if _, err := os.Stat(filepath.Join(dir, "manifest-2.json")); err != nil {
				t.Fatalf("with the switch on, the run into a directory holding one wrote no manifest-2.json: %v", err)
			}
			if changed := changedFrom(before, contentsOf(t, dir)); len(changed) > 0 {
				t.Errorf("the run beside the earlier one changed %v", changed)
			}
			// Said in the window as a sentence of its own (rule 6), opening
			// with a capital - CodeRabbit on #246.
			if said := statusLine(t, content); !strings.Contains(said, "The output directory already holds names this run would write") {
				t.Errorf("the run took a number and the window did not say so as a sentence. It said:\n%s", said)
			}

			numbers.SetChecked(false)
			// The refused run is held to the whole directory, not only to the
			// manifest it would have taken - CodeRabbit on #246.
			beforeRefusal := contentsOf(t, dir)
			press(t, content, text.ButtonGenerate())
			join(host)
			if edgeOf(t, content, text.FieldOutputDir()).StrokeWidth <= 0 {
				t.Errorf("with the switch off, a run into a directory holding others went unremarked under the output directory")
			}
			after := contentsOf(t, dir)
			if changed := changedFrom(beforeRefusal, after); len(changed) > 0 || len(after) != len(beforeRefusal) {
				t.Errorf("with the switch off, the refused run left the directory different: %d files before, %d after, changed %v",
					len(beforeRefusal), len(after), changed)
			}
		})
	}
}

// A note a run leaves about itself stands in the window as a line of its own,
// so it opens with a capital - unless it opens with a name, which is shown as
// it is spelt (text.Line).
func TestARunNoteOpensWithACapitalUnlessItOpensWithAName(t *testing.T) {
	word := text.Line(core.Says("guard.NoteOfOurs", "the folder holds %d runs", core.A("Count", 2)))
	if !strings.HasPrefix(word, "The folder") {
		t.Errorf("a note opening with a word of ours stands as %q, with a small letter at the start of a line", word)
	}
	name := text.Line(core.Says("guard.NoteOnAName", "%s was written past the memory setting", core.A("Name", "files_0001.txt")))
	if !strings.HasPrefix(name, "files_0001.txt") {
		t.Errorf("a note opening with a file name stands as %q - the name is no longer the file's", name)
	}
}

// The switch is one answer with the directory: carried to the screen somebody
// moves to, written down when the window closes, given back at the next start,
// and on again after Forget.
func TestTheSwitchTravelsAndIsRememberedWithTheFolder(t *testing.T) {
	host, content := openOnTab(t, text.TabOneTarget())
	toggleUnder(t, content, text.FieldNumberIfTaken()).SetChecked(false)

	presets := selectTab(t, host.content, text.TabPresets())
	if toggleUnder(t, presets, text.FieldNumberIfTaken()).Checked {
		t.Error("the switch was turned off on one screen and is on on the next")
	}
	host.intercept()
	if host.Remembered().NumbersIfTaken() {
		t.Error("the window closed with the switch off and remembered it on")
	}

	again := newFakeHost(t)
	again.kept = host.kept
	window.Open(again)
	t.Cleanup(func() { join(again) })
	if toggleUnder(t, selectTab(t, again.content, text.TabRecipe()), text.FieldNumberIfTaken()).Checked {
		t.Error("the switch was left off and the next start turned it back on")
	}

	again.Remembered().Forget()
	if !again.Remembered().NumbersIfTaken() {
		t.Error("Forget left the switch remembered off, and nobody has said anything since")
	}
}
