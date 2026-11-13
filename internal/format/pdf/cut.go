package pdf

// The layout of a document a damage will cut: its padding split around the
// objects.

import "github.com/donislawdev/TestingFilesGenerator/internal/format"

// largestOffset is the largest offset a cross reference entry can carry. ISO
// 32000-1 section 7.5.4 writes each one in ten digits.
const largestOffset = 9_999_999_999

// splitForACut is how much of the padding m has behind its trailer goes in
// front of its objects instead, for a document a damage will cut after its
// first cutAt bytes of whole, so that the cut keeps the share of the objects it
// keeps of the whole file (format.Request.CutAt). It returns the padding in
// front and the padding left behind, and none in front for a document nothing
// cuts.
//
// With all of the padding behind the trailer, as a document nothing cuts has
// it, a large document cut at any share kept every object and lost only
// padding. A comment may stand between the header lines and the first object as
// well, and the readers measured take it there as they take it behind the
// trailer (docs/TRUNCATE-2026-11-11.md section 12).
//
// The padding in front moves every object, so the offset startxref carries
// grows with it and can gain a digit, which the padding behind gives up. Each
// side is nothing or a comment of at least minComment bytes. Where the sides
// cannot land exactly the front takes a byte more, which cuts a byte more of
// the objects - the "at least" missing promises - or, where a digit gained
// leaves that no room, a byte less. Where the end is longer than the part the
// cut takes, all of the padding goes in front, and the cut falls in the end.
// An object can start no later than largestOffset, so in a document of more
// than about 9.3 GiB the front stops there and the cut keeps more of the
// objects than the share.
func splitForACut(m memo, whole, cutAt int64) (front, back int64) {
	if m.padLen == 0 || cutAt <= 0 || cutAt >= whole {
		return 0, m.padLen
	}
	want := format.PaddingInFront(m.at.objects, m.at.xref-m.at.objects, cutAt, whole)
	if want <= 0 {
		return 0, m.padLen
	}
	// padding is all of the padding with front of it in front: the closing
	// lines carry the offset front moves.
	padding := func(front int64) int64 {
		return whole - int64(len(m.prefix)) - int64(len(closing(m.at.xref+front)))
	}
	fits := func(front int64) bool {
		back := padding(front) - front
		return front >= minComment && m.at.xref+front <= largestOffset && (back == 0 || back >= minComment)
	}
	front = min(max(want, minComment), largestOffset-m.at.xref)
	if padding(front) < front {
		front = padding(front)
	}
	for _, f := range []int64{front, front + 1, front - 1} {
		if fits(f) {
			return f, padding(f) - f
		}
	}
	return 0, m.padLen
}
