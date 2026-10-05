package parts

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// Tone is which way a verdict came out.
type Tone int

// The two ways a check comes out.
const (
	// Agrees is a check that found what it was told to expect: in the
	// palette's success colour.
	Agrees Tone = iota + 1
	// Disagrees is one that did not: in its error colour.
	Disagrees
)

// Verdict is the sentence that answers what a check was asked, in the colour
// of the answer.
//
// Green for a match since 2026-10-05, the owner's list after the first look at
// the folder tools: "Matches" was drawn in the grey of every other line under
// it while "Does not match" was red, so the answer somebody ran the tool for
// was the quietest line of the result. The same choice the run screen made
// for "3 files written" on 2026-08-20 (window.toneOfOutcome), named here so
// the two colours are one part rather than a setting repeated beside each
// sentence. The words differ as well, so the colour is never the only carrier
// (UX1).
func Verdict(tone Tone, said string) fyne.CanvasObject {
	label := widget.NewLabel(said)
	label.Wrapping = fyne.TextWrapWord
	label.Importance = widget.DangerImportance
	if tone == Agrees {
		label.Importance = widget.SuccessImportance
	}
	return inkTight(label)
}
