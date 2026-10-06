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

// cluster is one cluster's place in the file: the frames it holds, when it
// starts, where it is, and how big its content is.
type cluster struct {
	first, last int64 // frames [first, last)
	ts          int64
	position    uint64 // from the start of the segment's content
	content     uint64
}

// layout is the whole file worked out without writing it. Planning and writing
// both build one from a stream - planning from the bound of a rung, writing
// from the coded picture - so the two cannot come to disagree about where
// anything is.
//
// The work is per cluster, not per frame. A day at sixty frames a second is
// five million frames and at most ninety thousand clusters, because the key
// frame interval is at least a second.
type layout struct {
	stream   video.Stream
	head     []byte // the EBML header
	seekHead []byte
	info     []byte
	tracks   []byte
	clusters []cluster
	cuesBody uint64 // what the Cues element holds
	body     uint64 // the segment's content without the Void
}

func newLayout(s video.Stream) layout {
	l := layout{stream: s, head: ebmlHeader(), info: info(s), tracks: tracks(s)}
	l.seekHead = seekHead(0, 0, 0)
	pos := uint64(len(l.seekHead) + len(l.info) + len(l.tracks))

	var cues uint64
	for g := int64(0); g < s.Keys(); g++ {
		pos, cues = l.addGroup(g, pos, cues)
	}
	l.cuesBody = cues
	l.body = pos + elementLen(idCues, cues)
	l.seekHead = seekHead(uint64(len(l.seekHead)), uint64(len(l.seekHead)+len(l.info)), pos)
	return l
}

// addGroup lays out the clusters of one group of pictures - the frames from
// one key frame to the next - from pos on, and gives back where the next
// group starts and the Cues so far. The first cluster of a group opens with
// its key frame and gets the group's cue point.
func (l *layout) addGroup(g int64, pos, cues uint64) (uint64, uint64) {
	s := l.stream
	key, copied, shown := s.KindBytes()
	span := int64(s.FPS) * clusterSpanMs / 1000
	start, end := g*s.KeyEvery, min((g+1)*s.KeyEvery, s.Frames)
	opening := len(l.clusters)
	for first := start; first < end; first += span {
		c := cluster{first: first, last: min(first+span, end), ts: s.StartMs(first), position: pos}
		c.content = uintElementLen(idTimestamp, uint64(c.ts)) + blocks(c, s.KeyEvery, key, copied, shown)
		pos += elementLen(idCluster, c.content)
		l.clusters = append(l.clusters, c)
	}
	return pos, cues + cuePointLen(l.clusters[opening])
}

// blocks is the bytes a cluster's SimpleBlocks take. A cluster that starts a
// group of pictures opens with the key sample and the hidden copy, and the
// rest of every cluster is frames that show the copy again.
func blocks(c cluster, keyEvery int64, key, copied, shown int) uint64 {
	n := c.last - c.first
	var total uint64
	if c.first%keyEvery == 0 {
		total += blockLen(key)
		n--
		if n > 0 && keyEvery > 1 {
			total += blockLen(copied)
			n--
		}
	}
	return total + uint64(n)*blockLen(shown)
}

// blockLen is a SimpleBlock element carrying a sample: track number, a two
// byte time offset and the flags, then the sample.
func blockLen(sample int) uint64 { return elementLen(idSimpleBlock, uint64(4+sample)) }

func cuePointLen(c cluster) uint64 {
	positions := uintElementLen(idCueTrack, 1) + uintElementLen(idCueClusterPosition, c.position)
	return elementLen(idCuePoint, uintElementLen(idCueTime, uint64(c.ts))+elementLen(idCueTrackPositions, positions))
}

// fileBytes is the file without its Void.
func (l layout) fileBytes() int64 {
	return int64(len(l.head)) + int64(idLen(idSegment)) + segmentSizeField + int64(l.body)
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
