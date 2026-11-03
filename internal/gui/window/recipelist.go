package window

import (
	"slices"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// The list of batches on the recipe screen, and the list of what each batch
// holds: what a press adds to them, copies and takes away. Out of recipe.go
// on 2026-11-03, when that file reached its ceiling - the screen builds the
// batches, and this is the part that only changes how many there are.

// addBatch puts another batch at the end of the list.
func (r *Recipe) addBatch() {
	newBatchAtTheEnd(r)
	afterAPress(r)
}

// newBatchAtTheEnd puts a batch after the last one, without saying what the
// form comes to. Apart from addBatch for the one caller that is not a press:
// the base switched off with no batch left brings one back, and the switch's
// own check says the line. Through addBatch it was said twice - found by the
// review of #133 (docs/REVIEW-133-2026-09-24.md).
func newBatchAtTheEnd(r *Recipe) {
	r.batches = append(r.batches, r.newBatch())
	// A new batch has no format until one is chosen, and its declared settings
	// come with that choice. Chosen here rather than left empty so that a batch
	// arrives looking like the one above it.
	r.batches[len(r.batches)-1].formatPick.SetSelected(format.IDs()[0])
}

// duplicateBatch copies one batch and puts the copy under it.
//
// Batches usually differ from each other in one setting - a size, a format, a
// count - so writing the second one from an empty form is typing the first one
// again. The copy is a new batch with the same values rather than a shared one,
// so changing it changes nothing else.
//
// The name is left empty on the copy, on purpose. Two targets with one id is a
// refusal the recipe reader already words and addresses, and handing somebody a
// form that is refused the moment they press a button would be a copy that has
// to be repaired before it can be used. Empty is refused too, and it is refused
// pointing at a box that is asking to be filled in.
func (r *Recipe) duplicateBatch(index int) {
	if index < 0 || index >= len(r.batches) {
		return
	}
	from := r.batches[index]
	to := r.newBatch()

	to.formatPick.SetSelected(from.formatPick.Selected)
	to.count.SetText(from.count.Text)
	to.size.SetText(from.size.Text)
	to.sizeRange.SetText(from.sizeRange.Text)
	to.boundary.SetText(from.boundary.Text)
	to.name.SetText(from.name.Text)
	to.notes.takeFrom(from.notes)
	if from.sizeWay != nil && to.sizeWay != nil {
		to.sizeWay.SetSelected(from.sizeWay.Selected)
	}

	rest := append([]*batch{to}, r.batches[index+1:]...)
	r.batches = append(r.batches[:index+1], rest...)
	afterAPress(r)
}

// removeBatch drops one batch. The last cannot go, unless the screen builds
// on a preset (base.carriesTheRun): a screen with no batches and no preset
// can produce nothing, and would answer a press with a refusal about a
// document rather than about anything anybody did.
func (r *Recipe) removeBatch(index int) {
	if index < 0 || index >= len(r.batches) {
		return
	}
	if len(r.batches) <= 1 && !r.base.carriesTheRun() {
		return
	}
	// slices.Delete rather than append of the two halves, because it clears the
	// place the last batch stood in: the halves glued together leave it
	// holding that batch, boxes and all, until something is written over it.
	r.batches = slices.Delete(r.batches, index, index+1)
	afterAPress(r)
}

// removeContent drops one entry from what a batch holds.
func (r *Recipe) removeContent(b *batch, index int) {
	if index < 0 || index >= len(b.contents) {
		return
	}
	b.contents = slices.Delete(b.contents, index, index+1)
	afterAPress(r)
}
