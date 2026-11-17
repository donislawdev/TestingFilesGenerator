package guard

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// A list shows its values by names in the language of the window, and English
// is one of those languages - docs/SIZE-FIXES-2026-11-16.md section 6.1 (M1).
// TestNoTranslationNamesAListValueTheListCallsOtherwise holds the sentences
// under a list to its names. These hold the names themselves.

// TestEveryNameWrittenByHandBelongsToADeclaredValue asks each English name
// written by hand to stand on a value something still declares, and to be the
// name a translator is given for it.
//
// The names are keyed by setting and value, so renaming either leaves the
// entry behind: the list goes back to showing the value, and nothing turns red
// because nothing is wrong with either half on its own.
func TestEveryNameWrittenByHandBelongsToADeclaredValue(t *testing.T) {
	declared := map[string]string{}
	for _, w := range text.RegistryWords() {
		declared[w.Key] = w.English
	}
	named := text.NamedChoices()
	if len(named) < 30 {
		t.Fatalf("%d value(s) are named by hand, where 30 or more were on 2026-11-17 - the names "+
			"have moved, and this guard asks less than it did", len(named))
	}
	for _, wrong := range namesLeftBehind(named, declared) {
		t.Error(wrong)
	}
	// The canary: a name for a value nothing declares has to be found.
	if len(namesLeftBehind([]text.NamedChoice{{Of: "content", Value: "no-such-value"}}, declared)) == 0 {
		t.Error("canary: a name for a value nothing declares was taken")
	}
}

// TestTheEnglishWindowShowsAListValueByItsName asks the English window, not
// the catalogue: registry/en.json is written from the names, so a window that
// stopped asking for them would leave every guard reading the file green.
//
// Every value of every list is asked of the function each list and sentence
// draws through, and one value is read off the screen twice - in the closed
// menu, and in the line a folded section of settings says about itself.
func TestTheEnglishWindowShowsAListValueByItsName(t *testing.T) {
	if got := text.AllowedWholeNumber(); got != "whole number" {
		t.Fatalf("this process speaks another language (%q), so nothing here is English to ask", got)
	}
	english := registryFiles(t)[text.English]
	asked := 0
	for _, l := range declaredLists() {
		for _, v := range l.values {
			want := english[text.ChoiceKey(l.setting, v)]["other"]
			if got := text.ChoiceName(l.setting, v); got != want {
				t.Errorf("%s %s: the window calls %s %q and registry/en.json %q", l.owner, l.setting, v, got, want)
			}
			asked++
		}
	}
	if asked < 200 {
		t.Fatalf("asked about %d value(s), so the lists were not read", asked)
	}

	wav, err := format.Get("wav")
	if err != nil {
		t.Fatal(err)
	}
	screen := window.NewGenerate(newFakeHost(t))
	chooserIn(t, screen.Fields(), engine.SettingFormat).SetSelected(wav.ID)
	settings := text.SettingsFor(wav.ID)
	openFold(t, screen.Object(), "", settings)
	content := chooserIn(t, screen.Fields(), "content")
	test.NewTempWindow(t, content)
	content.SetSelected("sweep")
	name := text.ChoiceName("content", "sweep")
	if name == "sweep" {
		t.Fatal("sweep is named as it is written, so nothing here tells a name from a value")
	}
	if shown := wordsInTheBox(t, content).Text; shown != name {
		t.Errorf("the menu of what a wav sounds like shows %q for sweep, where its name is %q", shown, name)
	}
	if content.Selected != "sweep" {
		t.Errorf("the menu hands back %q, and a recipe needs sweep", content.Selected)
	}
	foldTitled(t, screen.Object(), "", settings).Tapped(nil)
	said := shownText(screen.Object())
	if !strings.Contains(said, name) || valueWord("sweep").MatchString(said) {
		t.Errorf("the folded settings of a wav do not say %q for sweep:\n%s", name, said)
	}
}

// namesLeftBehind is what is wrong with each named value, read against the
// words the catalogue is written from.
func namesLeftBehind(named []text.NamedChoice, declared map[string]string) []string {
	var wrong []string
	for _, n := range named {
		english, ok := declared[text.ChoiceKey(n.Of, n.Value)]
		switch {
		case !ok:
			wrong = append(wrong, fmt.Sprintf("%s of %s is named by hand and no list declares it", n.Value, n.Of))
		case english != text.EnglishChoice(n.Of, n.Value):
			wrong = append(wrong, fmt.Sprintf("%s of %s is named %q by hand and the catalogue gives a translator %q",
				n.Value, n.Of, text.EnglishChoice(n.Of, n.Value), english))
		}
	}
	return wrong
}

// keyShaped is a value written the way a recipe writes a key: words of letters
// joined by _ or -.
var keyShaped = regexp.MustCompile(`^[a-z]+(?:[_-][a-z]+)+$`)

// standardTokens are the values shaped like a key that are names somebody
// looks up as they are: a header of a message and a transfer encoding, both
// spelled by RFC 2045.
var standardTokens = []string{"content-type", "quoted-printable"}

// TestNoListShowsAValueSpelledAsARecipeKey holds every list in every language
// to showing a name, not the key, for a value made of joined words. The
// review of 2026-11-17 found the English lists showing every one of them as a
// recipe writes it - record-per-line, out-of-order, size_zero - and a format
// added tomorrow would bring its own without a word.
func TestNoListShowsAValueSpelledAsARecipeKey(t *testing.T) {
	lists := declaredLists()
	asked := 0
	for tag, entries := range registryFiles(t) {
		wrong, n := valuesShownAsKeys(lists, entries)
		asked += n
		for _, w := range wrong {
			t.Errorf("%s %s", tag, w)
		}
	}
	if asked < 40 {
		t.Fatalf("asked about %d value(s) shaped like a key, so the lists were not read", asked)
	}
	for _, token := range standardTokens {
		if !slices.ContainsFunc(lists, func(l declaredList) bool { return slices.Contains(l.values, token) }) {
			t.Errorf("%s is let through as a standard token and no list declares it any more", token)
		}
	}
	// The canary: a list of one value shaped like a key and no name for it.
	planted := []declaredList{{owner: "format/x", setting: "layout", values: []string{"record-per-line"}}}
	if wrong, _ := valuesShownAsKeys(planted, nil); len(wrong) == 0 {
		t.Error("canary: a list showing record-per-line as it is was taken")
	}
}

// valuesShownAsKeys is every value shaped like a key that one catalogue shows
// as it is, and how many such values it asked about.
func valuesShownAsKeys(lists []declaredList, entries map[string]map[string]string) ([]string, int) {
	var wrong []string
	asked := 0
	for _, l := range lists {
		for _, v := range l.values {
			if !keyShaped.MatchString(v) || slices.Contains(standardTokens, v) {
				continue
			}
			asked++
			shown := entries[text.ChoiceKey(l.setting, v)]["other"]
			if shown == "" || shown == v {
				wrong = append(wrong, fmt.Sprintf("%s %s shows %s as it is written in a recipe", l.owner, l.setting, v))
			}
		}
	}
	return wrong, asked
}
