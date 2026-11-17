// How big, on the single batch screen: one size, or smallest for a picture.
package window

import (
	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// singleWays is what the single batch screen offers. Two of the four, because
// this screen is one batch of one size: a range and a limit belong to the
// batch screen, which says how each of them is written.
var singleWays = []sizeWay{wayExact, waySmallest}

// singleSizing is the switch between one size and smallest, with its name, and
// what keeps it in step with the size box.
//
// The switch is shown for pictures only. One way on a switch is not a choice,
// so for any other format the box stands alone the way it always has.
//
// Added on 2026-11-17 because the owner could not find smallest: it was a word
// to type into the size box, named only in the explanation behind the button
// beside it.
type singleSizing struct {
	way   *parts.Segments
	word  *smallestWord
	named fyne.CanvasObject
}

// build makes the switch and ties it to the box, before the box is registered
// as a field - see newSmallestWord.
func (s *singleSizing) build(box *parts.Entry) {
	s.way = parts.NewSegments(wayNames(singleWays), nil)
	s.word = newSmallestWord(box, s.way)
	// Nothing else to do on a change: the box changes with it, and the box is
	// a field the live check and the line at the foot already listen to.
	s.way.OnChanged = s.word.chosen
}

// namedIn puts the switch on the form under its name.
func (s *singleSizing) namedIn(fields *parts.Fields, tips *parts.Tips) fyne.CanvasObject {
	s.named = fields.Named(text.FieldSizeWay(), tips.Say(waysExplained(singleWays)), s.way)
	return s.named
}

// offerFor shows the switch for a format that takes smallest and hides it for
// one that does not. A switch hidden while on Smallest moves to one size and
// the box takes back its last size, for the reason smallestWord.offer gives.
func (s *singleSizing) offerFor(formatID string) {
	s.word.offer(waysFor(singleWays, formatID))
	if s.named == nil {
		return
	}
	if takesSmallest(formatID) {
		s.named.Show()
		return
	}
	s.named.Hide()
}
