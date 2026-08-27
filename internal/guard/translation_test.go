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

// childMarker puts the second half of this guard in its own process.
const childMarker = "TFG_TRANSLATION_CHILD"

// TestTheWindowSpeaksWhateverTheCatalogueSays is the proof that the seam
// carries, rather than that it compiles.
//
// Every word this window shows became a lookup on 2026-08-25 - see
// text.Load. From inside the build that change is invisible: the English is
// still written beside each entry and answers when no catalogue is loaded, so
// a wiring mistake anywhere between the entry and the catalogue leaves a window
// that looks exactly right and can never be translated. Nothing about English
// can catch that, which is why this guard puts a language in and looks.
//
// The language is asked for by name rather than left to the machine, so this
// guard answers the same on every machine. A guard that added Polish and hoped
// the machine was Polish would pass by doing nothing on every other one, which
// is the shape this project has written down as "a test passes without reaching
// the code".
//
// It runs in a child process because loading a catalogue changes it for
// everything after it. Twenty five screens are compared against stored pictures
// in this same package, and a guard that translated the window for them would
// turn every one of them red - or, worse, be reordered one day into passing.
func TestTheWindowSpeaksWhateverTheCatalogueSays(t *testing.T) {
	if os.Getenv(childMarker) == "1" {
		theWindowSpeaksTheCatalogue(t)
		return
	}

	run := exec.Command(os.Args[0],
		"-test.run=^TestTheWindowSpeaksWhateverTheCatalogueSays$", "-test.v")
	run.Env = append(os.Environ(), childMarker+"=1")
	out, err := run.CombinedOutput()
	if err != nil {
		t.Fatalf("the window did not take the words it was given:\n%s", out)
	}
	if !strings.Contains(string(out), "PASS") {
		t.Fatalf("the child said nothing about passing, which means it never ran:\n%s", out)
	}
}

// madeUpCatalogue holds words that are not English and are not Polish either,
// so a word from it appearing in the window cannot have come from anywhere but
// the catalogue.
//
// A real language tag, because go-i18n refuses one it has no plural rules for -
// measured on 2026-08-25, "no plural rule registered for qaa". Polish is the
// one a made up catalogue may as well be written in here: it is the language
// this build is expected to gain first, so the shape being proven is the shape
// that will arrive.
//
// Three entries rather than one, and each is a different kind of place: the
// name above a box, a heading, and the words on a button. A seam wired for one
// kind and not the others would otherwise pass.
const madeUpCatalogue = `{
  "FieldSize": { "other": "ROZMIAREK" },
  "HeadingGenerate": { "other": "NAGLOWEK" },
  "ButtonPreview": { "other": "PODGLAD" }
}`

func theWindowSpeaksTheCatalogue(t *testing.T) {
	t.Helper()

	// English first, so what changes is measured rather than assumed. A guard
	// that only looked afterwards would pass against a build where these words
	// were always the made up ones.
	if got := text.FieldSize(); got != "Size" {
		t.Fatalf("before any catalogue the size box is called %q, and the English beside the entry says Size", got)
	}

	made := fstest.MapFS{
		"locale/pl.json": &fstest.MapFile{Data: []byte(madeUpCatalogue)},
	}
	if err := text.Load(made, "locale", "pl"); err != nil {
		t.Fatalf("the made up catalogue would not load: %v", err)
	}

	for _, want := range []struct {
		what string
		got  func() string
		say  string
	}{
		{"the name above the size box", text.FieldSize, "ROZMIAREK"},
		{"the heading of the generate screen", text.HeadingGenerate, "NAGLOWEK"},
		{"the words on the preview button", text.ButtonPreview, "PODGLAD"},
	} {
		if got := want.got(); got != want.say {
			t.Errorf("%s says %q where the catalogue says %q", want.what, got, want.say)
		}
	}

	// The entries are one thing and a built screen is another: a screen holding
	// a word it read once at package level would still show the English.
	screen := window.NewGenerate(newFakeHost(t))
	said := textIn(screen.Object())
	for _, word := range []string{"ROZMIAREK", "NAGLOWEK", "PODGLAD"} {
		if !strings.Contains(said, word) {
			t.Errorf("the screen does not say %q anywhere, so it is not reading the catalogue:\n%s",
				word, said)
		}
	}
	if strings.Contains(said, "Size") {
		t.Errorf("the screen still says Size, so something holds the English rather than asking for it:\n%s", said)
	}
}
