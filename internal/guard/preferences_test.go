package guard

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// The Preferences screen, docs/USTAWIENIA-2026-09-29.md: which language the
// window speaks, and what it keeps between runs.

// polishName is what the list calls Polish, read from the build rather than
// typed here - a guard that wrote "Polski" would pass against a list that
// shows something else.
func polishName(t *testing.T) string {
	t.Helper()
	for _, l := range text.Languages() {
		if l.Tag == "pl" {
			return l.Name
		}
	}
	t.Fatal("this build carries no Polish, so there is no second language to choose")
	return ""
}

// Choosing a language saves it at once, and Restart now is offered exactly
// while a restart would change what the window speaks.
func TestChoosingALanguageSavesItAndOffersARestart(t *testing.T) {
	host := newFakeHost(t)
	window.Open(host)
	prefs := selectTab(t, host.content, text.TabPreferences())
	restart := buttonNamed(prefs, text.ButtonRestart())
	if restart == nil {
		t.Fatalf("there is no %q button. The screen has: %v", text.ButtonRestart(), buttonNames(prefs))
	}
	// In the state this is about: English on screen, nothing chosen.
	if text.Speaking() != text.English || host.Remembered().Language() != "" {
		t.Fatalf("the window speaks %q with %q chosen, not English with nothing chosen",
			text.Speaking(), host.Remembered().Language())
	}
	if !restart.Disabled() {
		t.Error("Restart now is offered with nothing chosen that a restart would change")
	}

	menu := chooserUnder(t, prefs, text.FieldLanguage())
	menu.SetSelected(polishName(t))
	if got := host.Remembered().Language(); got != "pl" {
		t.Errorf("Polish was chosen and %q was saved", got)
	}
	if restart.Disabled() {
		t.Error("Polish was chosen and Restart now stayed off, so the choice can only arrive by closing the window by hand")
	}

	// Back to the system, which here is English: nothing saved, and nothing a
	// restart would change.
	menu.SetSelected(text.ChoiceSameAsSystem(text.LanguageName()))
	if got := host.Remembered().Language(); got != "" {
		t.Errorf("the system was chosen and %q stayed saved", got)
	}
	if !restart.Disabled() {
		t.Error("the language on screen was chosen again and Restart now stayed on")
	}
}

// Opening the screen writes nothing - above all when the saved language is
// one this build does not carry. The list shows the system then, and saving
// what it shows would erase the person's choice without a press (untouchable
// rule 7) and the sentence saying it was missing along with it.
func TestOpeningPreferencesChangesNothingSaved(t *testing.T) {
	host := newFakeHost(t)
	// A real language this build does not carry, so it is parsed and matched
	// rather than rejected as a malformed tag.
	host.Remembered().RememberLanguage("de")
	window.Open(host)
	prefs := selectTab(t, host.content, text.TabPreferences())

	if got := host.Remembered().Language(); got != "de" {
		t.Errorf("a saved choice of de became %q by opening the window", got)
	}
	if said := textIn(prefs); !strings.Contains(said, text.PreferencesMissing("de")) {
		t.Errorf("the screen does not say the saved language is missing from this version:\n%s", said)
	}
	if got, want := chooserUnder(t, prefs, text.FieldLanguage()).Selected, text.ChoiceSameAsSystem(text.LanguageName()); got != want {
		t.Errorf("with the saved language missing the list shows %q, and what the window speaks for it is %q", got, want)
	}
}

// Restart now stands down while files are being made, stands up when they are
// done, and closes the window the way the close button does - the run
// stopped, the folder written down - before asking for the new one.
func TestRestartWaitsForTheRunAndClosesTheWayAPersonDoes(t *testing.T) {
	host, content, hold := heldScreen(t)
	prefs := tabNamed(t, host.content, text.TabPreferences())
	chooserUnder(t, prefs, text.FieldLanguage()).SetSelected(polishName(t))
	restart := buttonNamed(prefs, text.ButtonRestart())
	if restart == nil || restart.Disabled() {
		t.Fatal("Polish is chosen and Restart now is not on, so this guard cannot see it stand down")
	}

	dir := t.TempDir()
	fill(t, content, text.FieldOutputDir(), dir)
	fill(t, content, text.FieldCount(), "20000")
	press(t, content, text.ButtonPreview())
	hold.look(func() {
		if !restart.Disabled() {
			t.Error("Restart now stayed on while the screen was busy - pressing it would close the window and stop the work")
		}
		if said := textIn(prefs); !strings.Contains(said, text.PreferencesRestartBusy()) {
			t.Errorf("the screen does not say why Restart now is off:\n%s", said)
		}
	})
	join(host)
	if restart.Disabled() {
		t.Fatal("the work ended and Restart now stayed off")
	}

	press(t, prefs, text.ButtonRestart())
	if host.restarts != 1 {
		t.Errorf("Restart now asked for %d new windows", host.restarts)
	}
	if host.closed != 1 {
		t.Errorf("Restart now closed the window %d times", host.closed)
	}
	if got := host.Remembered().Directory(); got != dir {
		t.Errorf("the output folder written down at the restart is %q, not %q - the window did not close the way the close button closes it", got, dir)
	}
}

// Forget removes what the window keeps, says so, and the close that follows
// writes none of it back.
func TestForgetClearsWhatTheWindowKeepsAndTheCloseWritesNothingBack(t *testing.T) {
	host := newFakeHost(t)
	kept := host.Remembered()
	kept.RememberDirectory(`C:\before`)
	kept.RememberSize(fyne.NewSize(900, 700))
	kept.RememberLanguage("pl")
	window.Open(host)
	prefs := selectTab(t, host.content, text.TabPreferences())
	if host.kept.dir == "" || host.kept.language == "" {
		t.Fatal("nothing was kept before Forget, so this guard would pass on a button that does nothing")
	}

	press(t, prefs, text.ButtonForget())
	if host.kept.forgets != 1 {
		t.Errorf("Forget reached the store %d times", host.kept.forgets)
	}
	if host.kept.dir != "" || host.kept.size != (fyne.Size{}) || host.kept.language != "" {
		t.Errorf("after Forget the store still holds %q, %v and %q", host.kept.dir, host.kept.size, host.kept.language)
	}
	if forget := buttonNamed(prefs, text.ButtonForget()); forget == nil || !forget.Disabled() {
		t.Error("Forget can still be pressed after it was, with nothing left to forget")
	}
	if said := textIn(prefs); !strings.Contains(said, text.PreferencesForgotten()) {
		t.Errorf("the screen does not say it forgot:\n%s", said)
	}
	if got, want := chooserUnder(t, prefs, text.FieldLanguage()).Selected, text.ChoiceSameAsSystem(text.LanguageName()); got != want {
		t.Errorf("after Forget the list shows %q, and with nothing chosen the window speaks %q", got, want)
	}

	// The close button, as a person presses it.
	host.intercept()
	if host.kept.dir != "" {
		t.Errorf("closing the window after Forget wrote the folder %q back", host.kept.dir)
	}
	// And the size, which the real window writes after the close rather than
	// in it. A fresh store writes one - the control, without which a store
	// that writes nothing at all would pass - and the window's own store,
	// told to forget, does not.
	window.Forgetting(host.kept).RememberSize(fyne.NewSize(1, 1))
	if host.kept.size != fyne.NewSize(1, 1) {
		t.Fatalf("a store that was never told to forget did not write a size, so the next check proves nothing")
	}
	host.Remembered().RememberSize(fyne.NewSize(800, 600))
	if host.kept.size != fyne.NewSize(1, 1) {
		t.Errorf("after Forget the window's store wrote a size of %v", host.kept.size)
	}
}
