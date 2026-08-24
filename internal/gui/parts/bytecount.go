package parts

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// ByteCount says what the size in a box comes to, counted out in bytes.
//
// "10mb" is two different numbers depending on who reads it, and which one this
// tool means was settled long ago and written down - units count in 1024s,
// RECIPE.md section 9. What had never happened is anybody being TOLD on the
// screen. The sentence behind the little i said it, in an example about 10mb
// rather than about whatever is in the box, and only to somebody who clicked.
//
// It is not a second opinion about the units. It asks core.ParseSize, which is
// the same function the run parses the box with, so it cannot come to a
// different number than the files do - and if the box holds something that is
// not a size it says nothing at all rather than guessing.
//
// It lives on the field's NAME line, at the far right, and that placement is
// the whole reason this was cheap. A line of its own under the box would have
// cost about eighteen pixels per size field, and the single batch screen fits
// its window today with about seventeen to spare - so the honest version of
// "put it under the box" was "make the screen scroll". The name line has the
// rest of the column empty.
type ByteCount struct {
	widget.Label
}

func newByteCount() *ByteCount {
	c := &ByteCount{}
	c.ExtendBaseWidget(c)
	// The same rank as the line explaining a field, because it is the same kind
	// of thing: something quiet beside a control that the eye should skip until
	// it wants it.
	c.SizeName = theme.SizeNameCaptionText
	c.Importance = widget.LowImportance
	c.Alignment = fyne.TextAlignTrailing
	return c
}

// show puts the count for one size on screen, or takes it away.
//
// Empty for anything that is not a size, which covers the box somebody is
// halfway through typing. A count that guessed at a half-typed value would
// change three times per keystroke and be wrong twice.
func (c *ByteCount) show(size string) {
	bytes, err := core.ParseSize(size)
	if err != nil {
		c.SetText("")
		return
	}
	c.SetText(text.ExactBytes(bytes))
}
