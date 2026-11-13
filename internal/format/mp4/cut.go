package mp4

import (
	"math"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// splitForACut settles how much of the padding goes between the movie box and
// the media data of a film a damage will cut after its first cutAt bytes, so
// that the cut keeps the share of the media data it keeps of the whole file
// (format.Request.CutAt), and gives back the padding left behind the media
// data. content is the media data's samples, as the first pass measured them.
//
// With all of the padding behind the media data, as an uncut film has it, a
// large film cut at any share kept every picture and lost only padding. A free
// box may stand anywhere among the top level boxes, and readers skip it in
// front of the media data as they do behind it - ffmpeg's faststart writes one
// there (docs/TRUNCATE-2026-11-11.md section 14.4). The one thing a box there
// changes is where every chunk starts, which stco says in bytes from the start
// of the file, so every place moves by the gap: front counts it, and the places
// are written from front.
//
// The places take four bytes unless the layout already asked for eight. One
// that the gap takes past 4 GiB widens the table to co64, as a film whose
// pictures take their whole reserve past 4 GiB has it, and the movie box grows
// by four bytes a chunk - only then. When the padding cannot give up those
// bytes and still hold a free box, the film is laid out as one nothing cuts,
// and the manifest's cut says where the cut fell. That takes a film whose
// pictures came within four bytes a chunk of their whole reserve, which the
// guard cannot reach (section 14.3).
//
// A film whose share wants nothing in front - and every film nothing cuts - is
// laid out as one nothing cuts. It sets l.gap, and l.wide when it widens.
func splitForACut(l *layout, content, total, cutAt int64) (back int64) {
	before := int64(len(l.ftyp)) + l.moovLen()
	mdat := l.front() - before + content
	pad := total - before - mdat
	l.gap = format.SplitFront(pad, before, mdat, cutAt, total, minFree)
	if l.gap > 0 && !l.wide && l.front()+content > math.MaxUint32 {
		grown := (wideChunk - narrowChunk) * l.chunks
		l.gap = 0
		if pad-grown >= minFree {
			l.wide = true
			l.gap = format.SplitFront(pad-grown, before+grown, mdat, cutAt, total, minFree)
			l.wide = l.gap > 0
		}
	}
	return total - l.front() - content
}

// mediaData is how long the samples of the media data come to, chunk by
// chunk.
func (z sizes) mediaData(l layout) int64 {
	var n int64
	l.eachChunk(func(first, samples int64) { n += z.chunk(first, samples) })
	return n
}
