package guard

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// sameWordChild puts the second half of this guard in its own process, for the
// reason translation_test.go gives: loading a catalogue changes the words for
// everything that runs after it.
const sameWordChild = "TFG_SAME_WORD_CHILD"

// TestTwoTabsCalledTheSameAreStillTwoScreens holds the window to knowing its
// screens by what they are rather than by what they are called.
//
// The window carried the output directory and the keyboard between screens
// through maps keyed by the word on each tab until 2026-09-29. In English every
// word is different and nothing could tell. A translation is free to call two
// screens by one word, and then the two became one entry: moving between them
// carried the directory from a screen to itself, and the box on the other
// screen stayed where it was without a word.
//
// So this gives two tabs the same word and moves between them the way a person
// does, by the tab rather than by its name - a lookup by name cannot tell them
// apart, which is the whole point.
func TestTwoTabsCalledTheSameAreStillTwoScreens(t *testing.T) {
	if os.Getenv(sameWordChild) == "1" {
		twoTabsCalledTheSame(t)
		return
	}

	run := exec.Command(os.Args[0],
		"-test.run=^TestTwoTabsCalledTheSameAreStillTwoScreens$", "-test.v")
	run.Env = append(os.Environ(), sameWordChild+"=1")
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("two tabs called the same were not two screens:\n%s", out)
	}
	if !strings.Contains(string(out), "PASS") {
		t.Fatalf("the child said nothing about passing, which means it never ran:\n%s", out)
	}
}

// sameWordCatalogue calls the first two screens by one word.
const sameWordCatalogue = `{
  "TabOneTarget": { "other": "EKRAN" },
  "TabPresets": { "other": "EKRAN" }
}`

func twoTabsCalledTheSame(t *testing.T) {
	t.Helper()

	made := fstest.MapFS{
		"locale/pl.json": &fstest.MapFile{Data: []byte(sameWordCatalogue)},
	}
	if err := text.Load(made, "locale", "pl"); err != nil {
		t.Fatalf("the catalogue would not load: %v", err)
	}

	host := newFakeHost(t)
	window.Open(host)
	tabs := tabsIn(host.content)
	if tabs == nil || len(tabs.Items()) < 2 {
		t.Fatal("the window has fewer than two tabs")
	}
	generate, preset := tabs.Items()[0], tabs.Items()[1]
	// In the state this guard is about, or it proves nothing: two tabs whose
	// words are the same, and the ones the catalogue named.
	if generate.Text != "EKRAN" || preset.Text != "EKRAN" {
		t.Fatalf("the first two tabs are called %q and %q, not the one word the catalogue gave both",
			generate.Text, preset.Text)
	}

	const chosen = "C:\\somewhere\\else"
	fill(t, generate.Content, text.FieldOutputDir(), chosen)
	tabs.Select(preset)
	if got := entryUnder(t, preset.Content, text.FieldOutputDir()).Text; got != chosen {
		t.Errorf("the first screen was pointed at %q and the second, called by the same word, says %q",
			chosen, got)
	}

	const second = "C:\\third\\place"
	fill(t, preset.Content, text.FieldOutputDir(), second)
	tabs.Select(generate)
	if got := entryUnder(t, generate.Content, text.FieldOutputDir()).Text; got != second {
		t.Errorf("the second screen was pointed at %q and the first, called by the same word, says %q",
			second, got)
	}
}
