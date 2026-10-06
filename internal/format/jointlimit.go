package format

import (
	"fmt"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// JointLimit is a rule binding two settings that neither of them can state
// alone.
//
// A Property bounds one value. PNG needs more than that: each side of a picture
// may go up to twenty thousand pixels and the two multiplied may not pass forty
// megapixels, because the picture is held in memory while it is encoded. That
// rule had nowhere to live, so it lived in the generator and in a sentence of
// prose - and "tfg formats png" offered a pair it then refused.
//
// A sentence is enough for a person and not for anything else. AR9 has the
// registry as the one place a consumer asks what a format accepts, and a window
// drawing two number fields from Min and Max would offer twenty thousand in
// both and produce a request the run rejects. Declaring it is what lets the
// refusal, the printed description and a future field all come from one place.
//
// Kept to a product of two because that is the shape every case in Tier 1 has -
// pixels, samples times channels, pages times page size. A general expression
// would be a language to learn, and this is a line to read.
type JointLimit struct {
	// Of and By are the two settings multiplied together.
	Of, By string
	// Max is the largest their product may be, counted in the same units the
	// settings themselves use.
	Max int64
	// Unit is what the product is reported in, and Per is how many of the
	// settings' own units make one of it. Pixels are counted in millions when
	// spoken about, so Unit is "megapixels" and Per is a million - "400
	// megapixels and the limit is 40" is a sentence somebody can act on and
	// "400000000 and the limit is 40000000" is not. Per of nought means one.
	Unit string
	Per  int64
	// Base is what the product itself counts, for the sentence that has to be
	// exact: "pixels", "cells".
	//
	// It exists because the readable form above cannot always be used. Rounding
	// two counts that differ can land them on one number, and then the refusal
	// says "they come to 40 megapixels and the limit is 40" - measured on
	// 2026-09-22 for a picture of 20000x2001, which is 40 020 000 pixels
	// against a limit of 40 000 000. A person reading that has been told
	// nothing. See Allows.
	Base string
	// Why is the reason, in the words the refusal uses.
	Why string
}

// Allows reports whether a pair of values satisfies the limit, and says what is
// wrong when it does not.
//
// Both values are expected to have passed their own Property first, and this
// does not check that again. of*by is not guarded against overflow because the
// declarations that reach it cap each side at twenty thousand, so the product
// is nine orders of magnitude below the range of the type. A guard here would
// be a branch nothing could ever reach, and an unreachable branch reads as a
// protection somebody is relying on.
//
// The format is whose limit it is, so a window finds the reason in its own
// words under the key it keeps it by.
func (j JointLimit) Allows(format string, of, by int64) (bad core.Said) {
	got := of * by
	if got <= j.Max {
		return core.Said{}
	}
	asked, allowed := j.readably(got)
	return core.Says("format.JointTooMuch", "together they come to %s and the limit is %s, because %s",
		core.A("Asked", asked), core.A("Limit", allowed),
		core.A("Why", core.Term{Key: core.JointKey(core.FormatOwner(format), j.Of, j.By), Text: j.Why}))
}

// Subject is the pair of settings this limit binds, named for a refusal.
func (j JointLimit) Subject() core.Said {
	return core.Says("format.TwoSettings", "%s and %s", core.A("Of", core.LabelTerm(j.Of)), core.A("By", core.LabelTerm(j.By)))
}

// readably is the pair of counts as a person reads them, and it never puts two
// different counts on one number.
//
// The rounded form is offered first, because that is the one worth reading:
// "400 megapixels and the limit is 40 megapixels" is a sentence somebody can
// act on. It is stood down when both counts round to the same text, which is
// not a corner case - it is what a request just over the limit looks like.
// Measured on 2026-09-22 (O232): 20000x2001 is 40 020 000 pixels, the limit is
// 40 000 000, and the sentence read "they come to 40 megapixels and the limit
// is 40". Two identical numbers, and no way to tell how far over it was.
//
// Both halves carry the unit now. Only the first one did, so the limit was a
// bare number taking its noun from four words earlier.
//
// Adding decimal places was the other way out and it does not work: 40 000 001
// against 40 000 000 collides at every fixed number of places.
func (j JointLimit) readably(got int64) (asked, allowed core.Said) {
	if j.per() > 1 && got/j.per() != j.Max/j.per() {
		return core.Says("format.CountOf", "%d %s", core.A("Number", got/j.per()), core.A("Unit", core.UnitTerm(j.Unit))),
			core.Says("format.CountOf", "%d %s", core.A("Number", j.Max/j.per()), core.A("Unit", core.UnitTerm(j.Unit)))
	}
	return core.Says("format.ExactlyOf", "%s %s", core.A("Number", core.Exactly(got)), core.A("Unit", core.UnitTerm(j.Base))),
		core.Says("format.ExactlyOf", "%s %s", core.A("Number", core.Exactly(j.Max)), core.A("Unit", core.UnitTerm(j.Base)))
}

// Describe is the rule as one sentence, for the format list and for a window.
func (j JointLimit) Describe() string {
	return fmt.Sprintf("%s times %s cannot pass %d %s, because %s",
		j.Of, j.By, j.Most(), j.Unit, j.Why)
}

// Most is the ceiling counted in Unit - 40 for forty million pixels in
// megapixels - which is the number a sentence about this limit says. Here
// rather than worked out by whoever words the sentence, so the window saying it
// in another language divides the same way.
func (j JointLimit) Most() int64 { return j.Max / j.per() }

func (j JointLimit) per() int64 {
	if j.Per == 0 {
		return 1
	}
	return j.Per
}
