package guard

import (
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// A rule about three boxes is stated once, not once per box.
//
// The three ways of asking for a size are alternatives, and each of them said
// so: "Fill in one of these three." was the tail of all three hints, so one row
// of the screen carried the sentence three times. Measured off a render on
// 2026-08-20 - three copies within 800 px of each other, two lines of height on
// a form that is 379 px too tall, and a rule repeated three times reads as
// noise rather than as a rule.
//
// What stays under each box is what THAT box does, which is the only part of
// the three sentences that differed.
//
// It counts on the screen rather than in the strings, because the defect was a
// screen showing one string three times.
func TestTheRuleAboutTheThreeSizeBoxesIsStatedOnce(t *testing.T) {
	content, _ := laidOutWindow(t)
	batches := tabContent(t, content, text.TabRecipe)

	shown := allText(batches)
	if seen := strings.Count(shown, text.OneSizeSettingOnly); seen != 1 {
		t.Errorf("%q is on the batch screen %d time(s). A rule said once is a rule and said three times is noise",
			text.OneSizeSettingOnly, seen)
	}
}

// And it is still said.
//
// The other half, because deleting the sentence would also make the count
// above come out at zero and read as fixed. O114 is why it exists at all: the
// three boxes described themselves and said nothing about excluding each
// other, so a person could fill in two and find out at the press of a button.
func TestTheRuleAboutTheThreeSizeBoxesIsStillOnTheScreen(t *testing.T) {
	content, _ := laidOutWindow(t)
	batches := tabContent(t, content, text.TabRecipe)

	if !strings.Contains(allText(batches), text.OneSizeSettingOnly) {
		t.Errorf("the batch screen never says %q, so three boxes that exclude each other say nothing about it",
			text.OneSizeSettingOnly)
	}
}
