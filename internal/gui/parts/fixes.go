package parts

import (
	"errors"
	"strconv"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// fixFor is the button a refusal can carry under a field, when the refusal
// itself holds the value that would put it right - or nothing.
//
// One refusal today, and it is the one the owner reported from the running
// window on 2026-09-23: a size below what a format can make says the smallest
// it CAN make as an exact count of bytes, "14707223 B", and the only way to
// act on it was to copy eight digits into the box by hand. The refusal is a
// format.BelowMinimumError, which carries the minimum as a number rather than
// only as words, so the button is worked out from the number and not read out
// of the sentence.
//
// Nothing is filled in by itself. Untouchable rule 1 says a size is exact or
// refused, never quietly rounded, and a button somebody presses is the
// opposite of quiet: the value goes in the box, the box is checked again like
// any typed value, and the byte count under it says what it is.
//
// Only on a box holding one size. A range of two sizes ("1kb-8kb") refused
// for its lower end would be replaced by a single number, which is a
// different request, so it gets the sentence and no button.
func fixFor(f *Field, err error) (string, func()) {
	var below *format.BelowMinimumError
	if !errors.As(err, &below) || below.Minimum <= 0 {
		return "", nil
	}
	boxes := boxesIn(f.Control)
	if len(boxes) != 1 || strings.Contains(boxes[0].Text, "-") {
		return "", nil
	}
	box, value := boxes[0], strconv.FormatInt(below.Minimum, 10)
	return text.UseSmallestSize(core.ExactBytes(below.Minimum)), func() { box.SetText(value) }
}
