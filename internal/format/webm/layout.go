package webm

import (
	"github.com/donislawdev/TestingFilesGenerator/internal/format/video"
)

// application is what the file says made it, without a version, so a new
// release does not move the bytes of a file nobody changed (D11). The PDF
// producer says the same.
const application = "Testing Files Generator"

// clusterSpanMs is the longest a cluster runs. A block's time is a 16 bit
// offset from its cluster's, so a cluster cannot pass 32 767 ms, and thirty
// seconds stays clear of that at every frame rate.
const clusterSpanMs = 30_000

// segmentSizeField is how long the segment's size is written, always. Fixed,
// so a file one byte longer is a Void one byte longer and nothing before it
// moves - measured with the probe of docs/WIDEO-2026-10-06.md section 9.6.
const segmentSizeField = 8

// minVoid is the smallest Void: its ID and a one byte size of nothing. Every
// file carries one, which is what makes every size above the minimum reachable
// a byte at a time.
const minVoid = 2

// layout is what a film's file is made of before its pictures are coded: the
// parts that do not depend on them, and the arithmetic of the clusters that do.
//
// A cluster's length is known only once its picture is coded, because the
// pictures of a film differ (docs/WIDEO-2026-10-06.md section 15), so the
// clusters' places are found while writing. What is fixed in advance is where
// the Cues go: last in the segment, at its end less their length - which does
// not move, because each cluster position in them is written in eight bytes.
// So the SeekHead points at them before anything after it is written, the
// clusters are written as their pictures are coded, and the Void between the
// last cluster and the Cues takes up whatever the pictures left.
//
// A cluster opens at every key frame, at every change of picture and thirty
// seconds into a run of neither, so a cluster carries at most one picture and
// writing holds no more than that. The work is per cluster, not per frame: a
// day at sixty frames a second is five million frames, and the settings bound
// the clusters to a hundred thousand key frames, a hundred thousand changes and
// one every thirty seconds.
type layout struct {
	stream      video.Stream
	head        []byte // the EBML header
	info        []byte
	tracks      []byte
	seekHeadLen uint64
	cuesBody    uint64 // what the Cues element holds
}

func newLayout(s video.Stream) layout {
	l := layout{stream: s, head: ebmlHeader(), info: info(s), tracks: tracks(s)}
	l.seekHeadLen = uint64(len(seekHead(0, 0, 0)))
	for g := int64(0); g < s.Keys(); g++ {
		l.cuesBody += cuePointLen(uint64(s.StartMs(g * s.KeyEvery)))
	}
	return l
}

// front is where the first cluster starts, counted from the start of the
// segment's content.
func (l layout) front() uint64 { return l.seekHeadLen + uint64(len(l.info)+len(l.tracks)) }

// cuesLen is the whole Cues element.
func (l layout) cuesLen() uint64 { return elementLen(idCues, l.cuesBody) }

// outside is the bytes of the file before the segment's content: the EBML
// header, the segment's ID and its size.
func (l layout) outside() int64 {
	return int64(len(l.head)) + int64(idLen(idSegment)) + segmentSizeField
}

// next is where the cluster that opens at frame first ends: at the next key
// frame, the next change of picture or thirty seconds on, whichever comes
// first, and never past the film.
func (l layout) next(first int64) int64 {
	s := l.stream
	span := int64(s.FPS) * clusterSpanMs / 1000
	return min(s.Frames, (first/s.KeyEvery+1)*s.KeyEvery, (first/s.ChangeEvery+1)*s.ChangeEvery, first+span)
}

// clusterContent is what the cluster of frames [first, last) holds, given how
// long each kind of sample is (video.Timeline.SampleAt), so the length written
// before a cluster and the blocks written in it come from one answer.
//
// Only a cluster's first two frames can carry a picture. A key frame and a
// change each open a cluster, and the frame after a key frame - the only
// other one that carries a picture - is the second of the key frame's cluster
// or opens a change and its own, because a cluster runs thirty seconds and a
// second is at least one frame.
func (l layout) clusterContent(first, last int64, lens [video.SampleKinds]int) uint64 {
	total := uintElementLen(idTimestamp, uint64(l.stream.StartMs(first)))
	for i := first; i < min(first+2, last); i++ {
		total += blockLen(lens[l.stream.SampleAt(i)])
	}
	if rest := last - first - 2; rest > 0 {
		total += uint64(rest) * blockLen(lens[video.ShowSample])
	}
	return total
}

// boundBytes is the file without its Void when every picture takes its whole
// reserve - what planning promises, because no film of this stream comes to
// more.
func (l layout) boundBytes() int64 {
	lens := l.stream.BoundBytes()
	body := l.front() + l.cuesLen()
	for first := int64(0); first < l.stream.Frames; {
		last := l.next(first)
		body += elementLen(idCluster, l.clusterContent(first, last, lens))
		first = last
	}
	return l.outside() + int64(body)
}

// blockLen is a SimpleBlock element carrying a sample: track number, a two
// byte time offset and the flags, then the sample.
func blockLen(sample int) uint64 { return elementLen(idSimpleBlock, uint64(4+sample)) }

// cuePointLen is one cue point, with the cluster's position in eight bytes
// whatever it is - so the Cues are the same length before the clusters are
// written as after.
func cuePointLen(ts uint64) uint64 {
	positions := uintElementLen(idCueTrack, 1) + fixedUintElementLen(idCueClusterPosition)
	return elementLen(idCuePoint, uintElementLen(idCueTime, ts)+elementLen(idCueTrackPositions, positions))
}

func ebmlHeader() []byte {
	b := make([]byte, 0, 48)
	b = appendUint(b, idEBMLVersion, 1)
	b = appendUint(b, idEBMLReadVersion, 1)
	b = appendUint(b, idEBMLMaxIDLength, 4)
	b = appendUint(b, idEBMLMaxSizeLength, 8)
	b = appendString(b, idDocType, "webm")
	b = appendUint(b, idDocTypeVersion, 4)
	b = appendUint(b, idDocTypeReadVersion, 2)
	return appendBytes(make([]byte, 0, len(b)+8), idEBML, b)
}

// seekHead points at Info, Tracks and Cues. The positions are written in
// eight bytes each, so the SeekHead is the same length whatever they are and
// can be built before anything after it is known.
func seekHead(infoPos, tracksPos, cuesPos uint64) []byte {
	b := make([]byte, 0, 96)
	for _, s := range [3]struct {
		id  uint32
		pos uint64
	}{{idInfo, infoPos}, {idTracks, tracksPos}, {idCues, cuesPos}} {
		b = appendHeader(b, idSeek, elementLen(idSeekID, uint64(idLen(s.id)))+elementLen(idSeekPosition, 8))
		b = appendBigEndian(appendHeader(b, idSeekID, uint64(idLen(s.id))), uint64(s.id), idLen(s.id))
		b = appendFixedUint(b, idSeekPosition, s.pos)
	}
	return appendBytes(make([]byte, 0, len(b)+8), idSeekHead, b)
}

func info(s video.Stream) []byte {
	b := make([]byte, 0, 80)
	b = appendUint(b, idTimestampScale, 1_000_000)
	b = appendFloat(b, idDuration, float64(s.DurationMs))
	b = appendString(b, idMuxingApp, application)
	b = appendString(b, idWritingApp, application)
	return appendBytes(make([]byte, 0, len(b)+8), idInfo, b)
}

// tracks is the one video track. The colour is said again here, in the
// numbers the sequence header uses, for a player that reads the container's
// description rather than the stream's.
func tracks(s video.Stream) []byte {
	v := make([]byte, 0, 48)
	v = appendUint(v, idPixelWidth, uint64(s.Width))
	v = appendUint(v, idPixelHeight, uint64(s.Height))
	v = appendHeader(v, idColour, 4*uintElementLen(idMatrix, 1))
	v = appendUint(v, idMatrix, 1)
	v = appendUint(v, idRange, 1)
	v = appendUint(v, idTransfer, 1)
	v = appendUint(v, idPrimaries, 1)

	e := make([]byte, 0, 64+len(s.Config())+len(v))
	e = appendUint(e, idTrackNumber, 1)
	e = appendUint(e, idTrackUID, 1)
	e = appendUint(e, idTrackType, 1)
	e = appendUint(e, idFlagLacing, 0)
	e = appendString(e, idCodecID, "V_AV1")
	e = appendBytes(e, idCodecPrivate, s.Config())
	e = appendUint(e, idDefaultDuration, uint64(1_000_000_000/s.FPS))
	e = appendBytes(e, idVideo, v)

	t := appendBytes(make([]byte, 0, len(e)+16), idTrackEntry, e)
	return appendBytes(make([]byte, 0, len(t)+8), idTracks, t)
}
