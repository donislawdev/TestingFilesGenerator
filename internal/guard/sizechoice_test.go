package guard

import (
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// The rule about the three ways of stating a size is where all three of them
// can be read with it.
//
// The three stand side by side as equals and the engine refuses a batch that
// fills in more than one, so a person reading any of them has to be able to
// learn that. Until 2026-08-19 the rule was written under the FIRST of them
// only, and somebody reading the third had nothing to learn it from - the two
// beside it described themselves and said nothing about excluding each other.
// That is O114, and it was closed by putting the sentence under all three.
//
// This guard replaces the one that pinned that fix, because the fix was
// replaced. Three copies of a rule inside 800 px of one row is the rule turned
// into noise, measured off a render on 2026-08-20, and it cost two lines of a
// form that is 379 px too tall. The sentence stands above the row now.
//
// So what is asserted is the property rather than the arrangement: the rule is
// on the screen, and it is above the first of the three boxes and near it. The
// old guard walked the three hint strings, which was the arrangement - and an
// arrangement guard goes red when a better arrangement arrives, without
// anything having broken.
//
// TestTheRuleAboutTheThreeSizeBoxesIsStatedOnce and
// TestTheRuleAboutTheThreeSizeBoxesIsStillOnTheScreen are the other two halves.
func TestTheRuleAboutSizesIsWhereAllThreeBoxesCanBeSeenWithIt(t *testing.T) {
	ourTheme(t)
	content, _ := laidOutWindow(t)
	batches := tabContent(t, content, text.TabRecipe())

	rule, ok := labelBox(batches, text.OneSizeSettingOnly())
	if !ok {
		t.Fatalf("the batch screen never says %q", text.OneSizeSettingOnly())
	}

	for _, field := range []string{text.FieldSize(), text.FieldSizeRange(), text.FieldBoundary()} {
		name, ok := labelBox(batches, field)
		if !ok {
			t.Fatalf("the batch screen has no field called %q", field)
		}
		if rule.Y > name.Y {
			t.Errorf("the rule is at %.0f px and %q is at %.0f px, so somebody reading that box has already "+
				"passed the sentence that says only one of the three may be filled in", rule.Y, field, name.Y)
		}
		// One field's height. Further up than that and it is above something
		// else, which is where it was when O114 was written.
		if gap := name.Y - rule.Y; gap > 110 {
			t.Errorf("the rule is %.0f px above %q, which is far enough to belong to something else", gap, field)
		}
	}
}
