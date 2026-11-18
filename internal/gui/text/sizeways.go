package text

import "strings"

// The ways a size can be given on the batch screens - one size, a range,
// around a limit, smallest - as the switch names them, the label over it, and
// the sentence explaining each.

// The three ways of saying how big, as one control that allows one of them.
//
// They were three boxes side by side until 2026-08-25, with a sentence above
// them saying that only one may be filled in - which is O114, and it was a
// sentence because the screen let somebody fill in two and find out at the
// press of a button. A switch removes the state instead of describing it.
//
// The words differ from the labels below them on purpose. A switch reading
// "Size" above a box reading "Size" is the same word twice in 40 px, and these
// say HOW the size is stated where the label says WHAT the box holds.
func SizeWayExact() string    { return say("SizeWayExact", "One size") }
func SizeWayRange() string    { return say("SizeWayRange", "A range") }
func SizeWayBoundary() string { return say("SizeWayBoundary", "Around a limit") }

// SizeWaySmallest is the fourth way, offered for pictures only since
// 2026-11-17: each picture as small as its format makes it. It was a word to
// type into the size box before that, and the owner could not find it.
func SizeWaySmallest() string { return say("SizeWaySmallest", "Smallest") }

// FieldSizeWay names that switch, and until 2026-09-23 it had no name at all.
//
// It was the one control on the batch screen standing between two named
// fields with nothing over it, so what it was about came from where it sat -
// which is guessing, and which puts it outside the rule that every element
// belongs to something. The words say HOW the size is stated, where the
// label under it says WHAT the box holds, for the reason written above the
// three words themselves.
func FieldSizeWay() string { return say("FieldSizeWay", "How the size is given") }

// What each way of giving the size does, one sentence each, behind the button
// beside the switch because it is read once.
//
// One sentence per way, deliberately: the line under a box explains only the
// way already chosen, so before choosing there is nothing on the screen that
// compares them - and "Around a limit" is both the least obvious of them and
// the one this tool is really for.
//
// Separate sentences rather than one paragraph since 2026-11-17, because the
// switch no longer offers the same ways everywhere: smallest is offered for
// pictures only, and the single batch screen offers two ways of the four. A
// paragraph naming a way the switch beside it does not have sends somebody
// looking for it. See DetailSizeWays.
func SaysSizeWayExact() string {
	return say("SaysSizeWayExact", "One size gives every file the same size.")
}
func SaysSizeWayRange() string {
	return say("SaysSizeWayRange", "A range draws a different size for each file.")
}
func SaysSizeWayBoundary() string {
	return say("SaysSizeWayBoundary", "Around a limit makes three files: one byte under the limit, one on it, one over.")
}
func SaysSizeWaySmallest() string {
	return say("SaysSizeWaySmallest", "Smallest makes each picture as small as it can be at its width and height, so the files come out at different sizes. It can be chosen once both are set in the format's settings.")
}

// SmallestNeedsTheSides is why Smallest cannot be chosen yet, opened by the
// pointer over it. The second sentence of SaysSizeWaySmallest says the same
// for somebody who reached the switch by keyboard and has no pointer to hover
// with.
func SmallestNeedsTheSides() string {
	return say("SmallestNeedsTheSides", "Smallest needs the picture's width and height. Set both in the format's settings first.")
}

// DetailSizeWays is the explanation of a switch offering these ways, from the
// sentences of the ways it offers.
func DetailSizeWays(sentences []string) string { return strings.Join(sentences, " ") }

// SmallestCaption stands under a size box holding smallest, where the count of
// bytes stands under a number. There is no number to count until the run has
// planned each picture, so it says when there will be one and what it comes
// from - the width and height, which sit folded away in the format's
// settings, and a picture without them is refused.
func SmallestCaption() string {
	return say("SmallestCaption", "Known after Preview, from the width and height in the format's settings.")
}
