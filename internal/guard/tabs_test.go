package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/version"
)

// The strip across the top stands on the edge the screen reads from.
//
// Measured off a render on 2026-09-15, before this held: the toolkit's tabs
// stood on the edge of the window at x=10 while the title under them stood on
// the column at x=157, so at the width the window opens at the way between
// screens and the screens themselves had no line in common. The strip is ours
// now and goes through the same column and the same indent as the title, so
// the two stand on one edge by construction - and this asks the laid out
// window whether they do, on every screen, because "by construction" is a
// sentence about the code and this is a sentence about the canvas.
func TestTheStripStandsOnTheEdgeTheScreenReadsFrom(t *testing.T) {
	ourTheme(t)
	content, _ := laidOutWindow(t)
	strip := tabsIn(content)
	if strip == nil || len(strip.Words()) == 0 {
		t.Fatal("the window has no strip, so this guard read the wrong tree")
	}
	first := fyne.CurrentApp().Driver().AbsolutePositionForObject(strip.Words()[0])

	for _, tab := range allTabs() {
		t.Run(tab, func(t *testing.T) {
			screen := tabContent(t, content, tab)
			title, ok := labelBox(screen, titleOf(tab))
			if !ok {
				t.Fatalf("the %q screen has no title reading %q", tab, titleOf(tab))
			}
			at, found := absoluteOf(content, screen)
			if !found {
				t.Fatal("the screen is not in the window it was found in")
			}
			// labelBox measures from the screen and the strip is outside it,
			// so the title is put in the window's frame before comparing.
			edge := at.X + title.X
			if off := edge - first.X; off > 1 || off < -1 {
				t.Errorf("the first word of the strip starts at %.1f px and the title of %q at %.1f px, %.1f px apart."+
					" The strip and the screen have to share one edge or the window reads as two things assembled",
					first.X, tab, edge, off)
			}
		})
	}
}

// A screen is titled with the word on its tab.
//
// Every work screen had two names until 2026-09-15 - "Single batch" on the
// tab and "Generate files" over the form - and a person moving between them
// had to hold both. One vocabulary now: the tab's word is the title, at the
// rank a title has, and what the title used to say stands under it as a quiet
// sentence. About keeps the product and its version as its title, because on
// an About screen the product IS the subject, and that exception is asserted
// here rather than tolerated so that it cannot widen.
//
// Both halves are asked. A guard that only looked for the tab's word on the
// screen would pass while the word stood there as a hint, and one that only
// checked the rank would pass for any title at all.
func TestAScreenIsTitledWithTheWordOnItsTab(t *testing.T) {
	ourTheme(t)
	content, _ := laidOutWindow(t)

	for _, tab := range allTabs() {
		t.Run(tab, func(t *testing.T) {
			screen := tabContent(t, content, tab)
			titles := 0
			walk(screen, func(o fyne.CanvasObject) {
				words, bold, size, ok := boldWordsAt(o)
				if !ok || !bold || size != parts.TextTitle {
					return
				}
				titles++
				if words != titleOf(tab) {
					t.Errorf("the %q screen is titled %q, and its title has to be %q", tab, words, titleOf(tab))
				}
			})
			if titles != 1 {
				t.Errorf("the %q screen draws %d titles at the title's rank, and a screen has exactly one", tab, titles)
			}
			if sentence, has := subtitleOf(tab); has {
				if _, ok := labelBox(screen, sentence); !ok {
					t.Errorf("the %q screen does not say %q under its title, so what the old title said is gone rather than moved",
						tab, sentence)
				}
			}
		})
	}
}

// titleOf is what a screen is titled: the word on its tab, except About,
// which is titled with the product and its version.
func titleOf(tab string) string {
	if tab == text.TabAbout() {
		return text.HeadingAbout(version.Version)
	}
	return tab
}

// subtitleOf is the sentence under a screen's title, for the screens that
// have one.
func subtitleOf(tab string) (string, bool) {
	switch tab {
	case text.TabOneTarget():
		return text.SubtitleGenerate(), true
	case text.TabPresets():
		return text.SubtitlePreset(), true
	case text.TabRecipe():
		return text.SubtitleRecipe(), true
	}
	return "", false
}

// Enter on a word of the strip chooses its screen and hands the keyboard on
// where it can be seen.
//
// The toolkit's strip answered the pointer and nothing else, so until
// 2026-09-15 the way between screens did not exist for the keyboard at all.
// Two things are asserted and the second is the one that would go wrong
// quietly: the screen changes, and the keyboard lands on the new screen's
// first field WITH its mark drawn - somebody who pressed Enter is using the
// keyboard, and a mark placed silently, the way a press places it, would leave
// them with no idea where the next key goes.
func TestEnterOnAWordChoosesItsScreenAndHandsTheKeyboardOnVisibly(t *testing.T) {
	_, content, canvas := keyedWindow(t)
	strip := tabsIn(content)
	if strip == nil {
		t.Fatal("the window has no strip")
	}
	presets := wordOn(t, strip, text.TabPresets())

	canvas.Focus(holdsTheKeyboard(t, presets))
	if !presets.Marked() {
		t.Fatal("the keyboard was put on a word and the word drew no mark, so this guard cannot tell a keyboard from a press")
	}
	presets.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})

	if chosen := strip.Selected(); chosen == nil || chosen.Text != text.TabPresets() {
		t.Fatalf("Enter on %q did not choose its screen. Chosen: %v", text.TabPresets(), chosen)
	}
	screen := tabNamed(t, content, text.TabPresets())
	if !screen.Visible() {
		t.Error("the chosen screen is not visible, so the strip named one screen and showed another")
	}
	if generate := tabNamed(t, content, text.TabOneTarget()); generate.Visible() {
		t.Error("the screen somebody left is still visible under the one they chose")
	}

	first := controlUnder(screen, text.FieldPreset())
	if first == nil {
		t.Fatalf("the preset screen has no field called %q", text.FieldPreset())
	}
	focused := canvas.Focused()
	if any, ok := focused.(fyne.CanvasObject); !ok || any != first {
		t.Fatalf("after Enter the keyboard is on %s, and it has to be on the first field of the chosen screen",
			describeFocusable(focused))
	}
	if menu, ok := first.(*parts.Chooser); !ok || !menu.Marked() {
		t.Error("the first field got the keyboard from a KEY and drew no mark, so somebody using the keyboard " +
			"cannot see where it went")
	}
}

// The arrows move the keyboard along the strip without choosing.
//
// Choosing hands the keyboard on to the screen's first field, so an arrow that
// chose would leave the strip on every press: somebody looking for the third
// word would find themselves on the second screen's form instead. Manual
// activation - the arrows move, Enter chooses - is the one pattern that fits a
// strip whose choice moves the keyboard. Both ends are asked as well, because
// wrapping round is the other thing an arrow could do and the strip is short
// enough to see the whole of.
func TestTheArrowsMoveTheKeyboardAlongTheStripWithoutChoosing(t *testing.T) {
	_, content, canvas := keyedWindow(t)
	strip := tabsIn(content)
	if strip == nil {
		t.Fatal("the window has no strip")
	}
	words := strip.Words()
	if len(words) < 3 {
		t.Fatalf("the strip has %d words, and this guard needs three to walk along", len(words))
	}
	was := strip.Selected()

	canvas.Focus(holdsTheKeyboard(t, words[0]))
	press := func(key fyne.KeyName, want *parts.TabWord) {
		t.Helper()
		if focused, ok := canvas.Focused().(*parts.TabWord); ok {
			focused.TypedKey(&fyne.KeyEvent{Name: key})
		} else {
			t.Fatalf("the keyboard left the strip before %s was pressed: it is on %s", key, describeFocusable(canvas.Focused()))
		}
		if got := canvas.Focused(); got != want {
			t.Errorf("after %s the keyboard is on %s and %q was expected", key, describeFocusable(got), want.Text())
		}
		if !want.Marked() {
			t.Errorf("after %s the word %q holds the keyboard and draws no mark", key, want.Text())
		}
		if strip.Selected() != was {
			t.Errorf("%s chose a screen. The arrows move the keyboard and Enter chooses", key)
		}
	}
	press(fyne.KeyRight, words[1])
	press(fyne.KeyRight, words[2])
	press(fyne.KeyLeft, words[1])
	press(fyne.KeyHome, words[0])
	press(fyne.KeyLeft, words[0])
	press(fyne.KeyEnd, words[len(words)-1])
	press(fyne.KeyRight, words[len(words)-1])
}

// A press on a word chooses its screen without drawing the keyboard mark.
//
// The same class as the menu and the switch (pointerfocus_test.go): the mark
// means "the keyboard is here and you are using it", and a press is not the
// keyboard. Asked of the word AND of the field the keyboard is handed on to,
// because after a press the window places the keyboard quietly on the new
// screen's first field, and a field marked by a press is the defect that was
// reported from the screen twice.
//
// The word pressed FIRST is the chosen one, and that order is what the first
// version of this guard got wrong. A press on another word hands the keyboard
// straight on to that screen's first field, so a mark drawn on the word for a
// frame is gone before anything reads it - broken by hand, a word that took the
// keyboard loudly passed. Pressing the chosen word moves nothing on, so the
// mark it draws is the mark that stays.
func TestAPressOnAWordChoosesWithoutDrawingTheKeyboardMark(t *testing.T) {
	_, content, canvas := keyedWindow(t)
	strip := tabsIn(content)
	if strip == nil {
		t.Fatal("the window has no strip")
	}
	chosen := wordOn(t, strip, text.TabOneTarget())
	if !chosen.Chosen() {
		t.Fatalf("the window did not open on %q, so the press below is not on the chosen word", text.TabOneTarget())
	}
	test.Tap(chosen)
	if chosen.Marked() {
		t.Error("the chosen word was pressed with the pointer and drew the keyboard mark - and since the press " +
			"moved nothing, the mark stays until something else is pressed")
	}
	if focused, ok := canvas.Focused().(*parts.TabWord); !ok || focused != chosen {
		t.Errorf("after a press on the chosen word the keyboard is on %s, and the press put it on the word",
			describeFocusable(canvas.Focused()))
	}

	presets := wordOn(t, strip, text.TabPresets())
	test.Tap(presets)

	if now := strip.Selected(); now == nil || now.Text != text.TabPresets() {
		t.Fatalf("a press on %q did not choose its screen", text.TabPresets())
	}
	if presets.Marked() {
		t.Error("the word was pressed with the pointer and drew the keyboard mark")
	}
	screen := tabNamed(t, content, text.TabPresets())
	first := controlUnder(screen, text.FieldPreset())
	if any, ok := canvas.Focused().(fyne.CanvasObject); !ok || any != first {
		t.Fatalf("after a press the keyboard is on %s rather than on the first field of the chosen screen",
			describeFocusable(canvas.Focused()))
	}
	if menu, ok := first.(*parts.Chooser); !ok || menu.Marked() {
		t.Error("the first field of the chosen screen drew the keyboard mark after a PRESS, which is the blue box " +
			"reported from the screen on 2026-08-12 and 2026-08-18")
	}
}

// A word under the pointer is drawn at full strength, and goes quiet again
// when the pointer leaves.
//
// Rule 10 of the GUI rules: a look is checked by measuring its effect, never
// by the presence of code. So this reads the colour of the text the word
// DRAWS after the pointer arrives - the object in its renderer - rather than
// asking the word whether it believes it is hovered. A widget that took the
// event and drew the same picture was measured once already in the sister
// project: zero pixels different across the whole window (O205 here).
func TestAWordUnderThePointerIsDrawnAtFullStrength(t *testing.T) {
	ourTheme(t)
	content, _ := laidOutWindow(t)
	strip := tabsIn(content)
	if strip == nil {
		t.Fatal("the window has no strip")
	}
	word := wordOn(t, strip, text.TabPresets())
	if word.Chosen() {
		t.Fatal("the word this guard hovers is the chosen one, which is at full strength anyway")
	}
	quiet := parts.PaletteColour(theme.ColorNamePlaceHolder, theme.VariantDark)
	loud := parts.PaletteColour(theme.ColorNameForeground, theme.VariantDark)

	if got := inkOf(t, word); got != quiet {
		t.Fatalf("at rest the word is drawn in %v and the quiet colour is %v, so this guard cannot see it change", got, quiet)
	}
	word.MouseIn(&desktop.MouseEvent{})
	if got := inkOf(t, word); got != loud {
		t.Errorf("under the pointer the word is drawn in %v and full strength is %v", got, loud)
	}
	word.MouseOut()
	if got := inkOf(t, word); got != quiet {
		t.Errorf("after the pointer left the word is still drawn in %v", got)
	}
}

// holdsTheKeyboard is a word as something the canvas can put the keyboard on.
//
// Asked at run time rather than written as the type, on purpose: whether the
// words can hold the keyboard at all is the thing this file exists for, and a
// guard that stated it as a type would stop compiling the day it was lost
// instead of going red. Compiling and red is what a guard is for.
func holdsTheKeyboard(t *testing.T, o fyne.CanvasObject) fyne.Focusable {
	t.Helper()
	f, ok := o.(fyne.Focusable)
	if !ok {
		t.Fatalf("%T cannot hold the keyboard, so the strip is back to answering the pointer alone", o)
	}
	return f
}

// wordOn is the word on the strip that leads to the named screen.
func wordOn(t *testing.T, strip *parts.Tabs, name string) *parts.TabWord {
	t.Helper()
	for _, word := range strip.Words() {
		if word.Text() == name {
			return word
		}
	}
	t.Fatalf("the strip has no word reading %q", name)
	return nil
}

// inkOf is the colour a word on the strip is drawing its letters in.
func inkOf(t *testing.T, word *parts.TabWord) any {
	t.Helper()
	for _, o := range test.WidgetRenderer(word).Objects() {
		if ink, ok := o.(*canvas.Text); ok {
			return ink.Color
		}
	}
	t.Fatalf("the word %q draws no text", word.Text())
	return nil
}
