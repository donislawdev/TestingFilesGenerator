// The ways of giving a size, and the word smallest kept in step with the switch
// that offers it.
//
// Its own file because both screens draw the switch since 2026-11-17 - the
// batch screen with up to four ways, the single batch screen with two - and a
// way described in one screen's file would be described again in the other's.
package window

import (
	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// sizeWay is one way of giving the size: the word on the switch, the sentence
// explaining it, and the box it leaves on the screen.
//
// Smallest leaves the box of one size, because what it sends is that box's
// key holding the word smallest - the value a recipe and --size take. It is a
// way of filling the box, not a box of its own. See smallestWord.
type sizeWay struct {
	name     func() string
	says     func() string
	box      string
	smallest bool
}

var (
	wayExact    = sizeWay{name: text.SizeWayExact, says: text.SaysSizeWayExact, box: recipe.KeySize}
	wayRange    = sizeWay{name: text.SizeWayRange, says: text.SaysSizeWayRange, box: recipe.KeySizeRange}
	wayBoundary = sizeWay{name: text.SizeWayBoundary, says: text.SaysSizeWayBoundary, box: recipe.KeyBoundary}
	waySmallest = sizeWay{name: text.SizeWaySmallest, says: text.SaysSizeWaySmallest, box: recipe.KeySize, smallest: true}
)

// waysFor is the ways a screen offering all of these offers for one format:
// all of them for a picture, and all but smallest for anything else.
//
// Asked of the engine (G1), by the rule it refuses smallest by, so the window
// holds no list of formats of its own. A format the menu holds and the
// registry does not know offers what a format that is not a picture does - the
// menu was filled from the registry, so that is a build where the two came
// apart, and the screen says so elsewhere.
func waysFor(all []sizeWay, formatID string) []sizeWay {
	picture := takesSmallest(formatID)
	offered := make([]sizeWay, 0, len(all))
	for _, way := range all {
		if way.smallest && !picture {
			continue
		}
		offered = append(offered, way)
	}
	return offered
}

// takesSmallest says a batch of this format can ask for smallest.
func takesSmallest(formatID string) bool {
	desc, err := format.Get(formatID)
	return err == nil && engine.TakesSmallest(desc)
}

// wayNames is the words on a switch offering these ways, in their order.
func wayNames(ways []sizeWay) []string {
	names := make([]string, 0, len(ways))
	for _, way := range ways {
		names = append(names, way.name())
	}
	return names
}

// waysExplained is what the button beside a switch offering these ways says:
// one sentence for each of them, and none for a way it does not offer.
func waysExplained(ways []sizeWay) string {
	sentences := make([]string, 0, len(ways))
	for _, way := range ways {
		sentences = append(sentences, way.says())
	}
	return text.DetailSizeWays(sentences)
}

// wayNamed is the way behind a word on the switch, or the first of them for a
// word none of them has.
func wayNamed(ways []sizeWay, chosen string) sizeWay {
	for _, way := range ways {
		if way.name() == chosen {
			return way
		}
	}
	if len(ways) == 0 {
		return wayExact
	}
	return ways[0]
}

// smallestWord keeps one size box and the switch offering smallest saying the
// same thing, with the box as the one that is right.
//
// The box is what reaches the run - draft and settle read it and nothing else
// - so the switch is a view of it rather than a second record. Choosing
// Smallest writes the word into the box. Leaving it puts back the last size
// that was typed there. Typing smallest moves the switch onto Smallest, and
// typing anything else while it is there moves it back to One size. The
// button under a size refusal writes smallest into the box the same way any
// value goes in, and the switch follows - so there is no way of filling the
// box that leaves the two disagreeing, and no second rule for the button.
//
// Kept in the box rather than hidden behind a box of its own because a
// refusal about the size has to land on a box that is on the screen. Smallest
// without a width and a height is refused about the size, and a fourth box
// holding no value would have needed an address of its own and a rule moving
// refusals onto it - the shape that left a run refusing and nothing marked on
// 2026-08-25 (see readdressSizeWay).
type smallestWord struct {
	box *parts.Entry
	way *parts.Segments
	// last is the last thing the box held that was not smallest, which is
	// what leaving Smallest puts back.
	last string
}

// newSmallestWord ties a box to a switch. Called before the box is registered
// as a field, because registering chains onto the box's own handler, and a
// handler assigned after that would replace the chain.
func newSmallestWord(box *parts.Entry, way *parts.Segments) *smallestWord {
	w := &smallestWord{box: box, way: way, last: startingSize}
	if box.Text != core.SizeSmallest {
		w.last = box.Text
	}
	box.OnChanged = w.typed
	return w
}

// typed follows the box with the switch.
func (w *smallestWord) typed(value string) {
	if value == core.SizeSmallest {
		// Nothing happens where the switch does not offer it - see
		// parts.Segments.SetSelected - and the run then refuses smallest for
		// that format in the engine's own words.
		w.way.SetSelected(text.SizeWaySmallest())
		return
	}
	w.last = value
	if w.way.Selected == text.SizeWaySmallest() {
		w.way.SetSelected(text.SizeWayExact())
	}
}

// chosen follows the switch with the box. Called first from the switch's own
// handler, so whatever the screen then reads is read off the box as it is now.
func (w *smallestWord) chosen(name string) {
	if name == text.SizeWaySmallest() {
		if w.box.Text != core.SizeSmallest {
			w.box.SetText(core.SizeSmallest)
		}
		return
	}
	if w.box.Text == core.SizeSmallest {
		w.box.SetText(w.last)
	}
}

// offer puts these ways on the switch and the switch where the box says.
//
// A switch that loses Smallest while on it moves to the first way and the box
// takes back its last size - a format that is not a picture cannot be the
// smallest of anything, and leaving the word would leave a form the run
// refuses. A switch that gains it goes onto it when the box already holds the
// word, which is how a batch copied with smallest, or a box somebody typed
// smallest into before choosing a picture, reads.
func (w *smallestWord) offer(ways []sizeWay) {
	w.way.SetOptions(wayNames(ways))
	if w.box.Text == core.SizeSmallest {
		w.way.SetSelected(text.SizeWaySmallest())
	}
}
