// The three ways of saying how big, as one switch and one box.
//
// Its own file because recipe.go was 426 lines of a 550 line ceiling when this
// arrived, and TestNothingIsQuietlyCreepingTowardsTheCeiling said so the same
// hour. The split is along a seam rather than at a line count: everything here
// answers one question that the rest of the screen does not ask.
package window

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// sizeWayFor is the switch that chooses how this batch says how big, and the
// three boxes it chooses between.
//
// All three are registered, and two of them are hidden. Registration is what
// gives a box the address a refusal is placed by, the star, the count of bytes
// under it and the check that runs while somebody types - building only the
// chosen one would mean building it again on every change of mind, and a box
// built again is a box that has forgotten what was typed into it.
//
// The switch is drawn above them rather than beside the name of one, because it
// belongs to all three and a control that belongs to three fields cannot sit
// inside one of their headings.
func (r *Recipe) sizeWayFor(b *batch, at func(string) string,
	add func(setting, label, hint string, detail parts.Detail, control fyne.CanvasObject) fyne.CanvasObject,
) fyne.CanvasObject {
	// Half a row wide, like every other short box on this screen. A size is
	// "10mb" and the three used to share one row between them, so a box across
	// the whole form would be the only wide box holding four characters - and
	// the eye reads width as how much is expected.
	half := func(field fyne.CanvasObject) fyne.CanvasObject {
		return r.fields.Row(field, layout.NewSpacer())
	}
	b.sizeBoxes = map[string]fyne.CanvasObject{
		recipe.KeySize: half(add(recipe.KeySize, text.FieldSize(), text.HintSizeExact(),
			r.tips.Say(text.DetailSize()), b.size)),
		recipe.KeySizeRange: half(add(recipe.KeySizeRange, text.FieldSizeRange(), text.HintSizeRange(),
			r.tips.Say(text.DetailSizeRange()), b.sizeRange)),
		recipe.KeyBoundary: half(add(recipe.KeyBoundary, text.FieldBoundary(), text.HintBoundary(),
			r.tips.Say(text.DetailBoundary()), b.boundary)),
	}

	names := sizeWayNames()
	// Assigned after the boxes exist, and on every rebuild, because a rebuild
	// makes new boxes for a switch that is kept.
	b.sizeWay.OnChanged = func(chosen string) {
		key := sizeWayKey(chosen)
		b.showSizeWay(key)
		// A refusal about a box that is no longer sent is a refusal about
		// nothing. Taken back rather than left on a hidden box, where it would
		// wait to reappear the next time somebody came back to that way of
		// saying how big - and read as a complaint about what they just typed.
		for _, other := range sizeWayKeys() {
			if other != key {
				r.fields.Clear(at(other))
			}
		}
		r.recheck(at(key))
	}
	b.showSizeWay(b.chosenSizeKey())

	boxes := make([]fyne.CanvasObject, 0, len(names))
	for _, key := range sizeWayKeys() {
		boxes = append(boxes, b.sizeBoxes[key])
	}
	return parts.Stacked(append([]fyne.CanvasObject{b.sizeWay}, boxes...)...)
}

// newSizeWaySwitch is the control itself, built with the batch rather than with
// the panel that draws it.
//
// Built early because copying a batch copies its controls, and a switch that
// only existed once the screen had been laid out was a switch a copy could not
// read - so the copy took the values of the other two ways and then showed the
// first one. Found by the guard on its first run.
func newSizeWaySwitch() *widget.RadioGroup {
	names := sizeWayNames()
	group := widget.NewRadioGroup(names, nil)
	group.Horizontal = true
	// Required, so there is no fourth state where nothing is chosen and no box
	// is shown. One of the three is always the answer.
	group.Required = true
	group.Selected = names[0]
	return group
}

// The three ways, in one order, named once. The window shows the words and the
// recipe writes the keys, and neither list may drift from the other.
func sizeWayNames() []string {
	return []string{text.SizeWayExact(), text.SizeWayRange(), text.SizeWayBoundary()}
}

func sizeWayKeys() []string {
	return []string{recipe.KeySize, recipe.KeySizeRange, recipe.KeyBoundary}
}

// sizeWayKey is the recipe key behind a word on the switch.
func sizeWayKey(chosen string) string {
	for i, name := range sizeWayNames() {
		if name == chosen {
			return sizeWayKeys()[i]
		}
	}
	return recipe.KeySize
}

// chosenSizeKey is which way this batch is using.
func (b *batch) chosenSizeKey() string {
	if b.sizeWay == nil {
		return recipe.KeySize
	}
	return sizeWayKey(b.sizeWay.Selected)
}

// showSizeWay leaves one box on the screen and takes the other two off it.
//
// Hidden rather than disabled. A disabled box holding a value still reads as
// something the run will use, which is the confusion the switch is here to end.
func (b *batch) showSizeWay(key string) {
	for at, box := range b.sizeBoxes {
		if at == key {
			box.Show()
			continue
		}
		box.Hide()
	}
}

// statedSize is what one of the three boxes contributes to the recipe, which is
// nothing at all unless the switch is on it.
//
// This is the whole of what makes the refusal unreachable. The box keeps what
// was typed into it so that changing your mind twice does not cost the value,
// and the recipe never hears about it.
func (b *batch) statedSize(key string) string {
	if b.chosenSizeKey() != key {
		return ""
	}
	switch key {
	case recipe.KeySizeRange:
		return b.sizeRange.Text
	case recipe.KeyBoundary:
		return b.boundary.Text
	default:
		return b.size.Text
	}
}

// readdressSizeWay puts a refusal about any of the three ways of stating a size
// onto the one this batch is actually using.
//
// The recipe reader words a batch with no size at all as "target 1 has no size"
// however the person meant to state it, because the size is the key it looks
// for first. With the switch on a range that refusal is addressed to a box
// nobody can see, so the form refuses to run and marks nothing. Found by
// TestAStarIsOnEveryBoxTheRunWillNotDoWithout on 2026-08-25, an hour after the
// switch arrived.
//
// Only the three move, and only onto each other. Anything else is handed back
// as it came, so a screen wide rule cannot quietly redirect a refusal about a
// setting that has one box like everything else.
func (r *Recipe) readdressSizeWay(address string) string {
	for index, b := range r.batches {
		for _, key := range sizeWayKeys() {
			if address != recipe.TargetAddress(index+1, key) {
				continue
			}
			return recipe.TargetAddress(index+1, b.chosenSizeKey())
		}
	}
	return address
}
