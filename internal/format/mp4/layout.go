package mp4

import (
	"encoding/binary"
	"math"

	"github.com/donislawdev/TestingFilesGenerator/internal/format/isobmff"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/video"
)

// fullBoxHeader is a box header with the version and flags a full box adds.
const fullBoxHeader = isobmff.BoxHeader + 4

// The sample tables' entries, in bytes.
const (
	syncEntry   = 4  // stss: a sample number
	chunkRun    = 12 // stsc: first chunk, samples per chunk, description
	sizeEntry   = 4  // stsz
	narrowChunk = 4  // stco
	wideChunk   = 8  // co64
)

// layout is a film's file before its pictures are coded: every box of moov
// except the sample tables as bytes, and the lengths of those tables. The
// tables have an entry per frame, key frame or chunk - never anything that
// depends on what a picture codes to - so moov is the same length whatever the
// pictures come to, planning counts it without coding any, and writing puts it
// before them, where a browser reads it first (docs/MP4-2026-10-07.md
// sections 2 and 3).
//
// A chunk opens at every key frame and every change of picture, the places
// WebM opens a cluster, so a chunk carries at most one picture. ISO base media
// lets one chunk hold a whole track, which the probe of 2026-10-06 played, but
// files from a real muxer are cut into chunks, and a test file is meant to
// look like one.
type layout struct {
	stream video.Stream
	counts [video.SampleKinds]int64
	chunks int64
	// runs is how many entries stsc has: one for each run of chunks that hold
	// the same number of samples.
	runs int64
	// wide is whether the chunks' places take eight bytes each (co64), and
	// bigData whether the media data's length does - each only when a film
	// whose pictures take their whole reserve would need it, so a smaller film
	// never has a field it could not fill.
	wide, bigData bool

	ftyp, mvhd, tkhd, mdhd, hdlr, vmhd, dinf, stsd, stts []byte
}

func newLayout(s video.Stream) layout {
	l := layout{stream: s, counts: s.SampleCounts()}
	last := int64(-1)
	l.eachChunk(func(_, samples int64) {
		l.chunks++
		l.runs += boolCount(samples != last)
		last = samples
	})
	l.ftyp = fileType()
	l.mvhd, l.tkhd, l.mdhd = movieHeader(s), trackHeader(s), mediaHeader(s)
	l.hdlr, l.vmhd, l.dinf = handler(), videoHeader(), dataInformation()
	l.stsd, l.stts = sampleDescription(s), timeToSample(s)

	content := l.boundContent()
	l.bigData = isobmff.BoxHeader+content > math.MaxUint32
	l.wide = l.front()+content > math.MaxUint32
	return l
}

// eachChunk walks the chunks in order, each by its first frame and how many
// frames it holds: one opens at every key frame and every change, so the walk
// is over the key frames and the changes rather than the frames.
func (l layout) eachChunk(chunk func(first, samples int64)) {
	t := l.stream.Timeline
	for first := int64(0); first < t.Frames; {
		last := min(t.Frames, (first/t.KeyEvery+1)*t.KeyEvery, (first/t.ChangeEvery+1)*t.ChangeEvery)
		chunk(first, last-first)
		first = last
	}
}

// boundContent is the media data when every picture takes its whole reserve -
// what planning promises, because no film of this stream comes to more.
func (l layout) boundContent() int64 {
	bound := l.stream.BoundBytes()
	var total int64
	for k, n := range l.counts {
		total += n * int64(bound[k])
	}
	return total
}

// front is where the media data's content starts: the file type, the movie
// box and the media data's header.
func (l layout) front() int64 {
	head := int64(isobmff.BoxHeader)
	if l.bigData {
		head += 8
	}
	return int64(len(l.ftyp)) + l.moovLen() + head
}

// need is the file without its padding when every picture takes its whole
// reserve, and with the smallest free box, which every one of these carries.
func (l layout) need() int64 { return l.front() + l.boundContent() + minFree }

// The sample tables, each a full box.
func (l layout) stssLen() int64 { return fullBoxHeader + 4 + syncEntry*l.counts[video.KeySample] }
func (l layout) sdtpLen() int64 { return fullBoxHeader + l.stream.Frames }
func (l layout) stscLen() int64 { return fullBoxHeader + 4 + chunkRun*l.runs }
func (l layout) stszLen() int64 { return fullBoxHeader + 8 + sizeEntry*l.stream.Frames }

func (l layout) stcoLen() int64 {
	entry := int64(narrowChunk)
	if l.wide {
		entry = wideChunk
	}
	return fullBoxHeader + 4 + entry*l.chunks
}

func (l layout) stblLen() int64 {
	return isobmff.BoxHeader + int64(len(l.stsd)+len(l.stts)) + l.stssLen() + l.sdtpLen() + l.stscLen() + l.stszLen() + l.stcoLen()
}

func (l layout) minfLen() int64 {
	return isobmff.BoxHeader + int64(len(l.vmhd)+len(l.dinf)) + l.stblLen()
}

func (l layout) mdiaLen() int64 {
	return isobmff.BoxHeader + int64(len(l.mdhd)+len(l.hdlr)) + l.minfLen()
}

func (l layout) trakLen() int64 { return isobmff.BoxHeader + int64(len(l.tkhd)) + l.mdiaLen() }
func (l layout) moovLen() int64 { return isobmff.BoxHeader + int64(len(l.mvhd)) + l.trakLen() }

// compressor is the sample entry's compressor name, the one the AV1 binding
// RECOMMENDS (v1.3.0 section 2.2.4): a length, then the name, in thirty two
// bytes. MP4 has no field that has to name the program, and WebM names it
// only because Matroska requires a MuxingApp.
const compressor = "AOM Coding"

func boolCount(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func fileType() []byte {
	b := boxStart("ftyp")
	b = append(b, "isom"...)
	b = binary.BigEndian.AppendUint32(b, 0x200)
	// av01 SHALL be among the compatible brands (AV1 binding section 2.1).
	b = append(b, "isomiso6av01mp41"...)
	return boxEnd(b)
}

// unity is the identity transformation matrix both headers carry.
func appendUnity(b []byte) []byte {
	for _, v := range [9]uint32{0x00010000, 0, 0, 0, 0x00010000, 0, 0, 0, 0x40000000} {
		b = binary.BigEndian.AppendUint32(b, v)
	}
	return b
}

// movieHeader counts in milliseconds, which every length a film has is a
// whole number of. Its times are nought, so a file made today and the same
// one made tomorrow are the same bytes (D11).
func movieHeader(s video.Stream) []byte {
	b := fullBoxStart("mvhd", 0, 0)
	b = appendU32s(b, 0, 0, 1000, uint32(s.DurationMs), 0x00010000)
	b = binary.BigEndian.AppendUint16(b, 0x0100)
	b = append(b, make([]byte, 10)...)
	b = appendUnity(b)
	b = append(b, make([]byte, 24)...)
	return boxEnd(binary.BigEndian.AppendUint32(b, 2))
}

// trackHeader is the one track, enabled and in the movie, with the picture's
// size in sixteen dot sixteen.
func trackHeader(s video.Stream) []byte {
	b := fullBoxStart("tkhd", 0, 3)
	b = appendU32s(b, 0, 0, 1, 0, uint32(s.DurationMs), 0, 0)
	b = append(b, make([]byte, 8)...)
	b = appendUnity(b)
	return boxEnd(appendU32s(b, uint32(s.Width)<<16, uint32(s.Height)<<16))
}

// mediaHeader counts in frames - the time scale is the frame rate, a whole
// number (video.Properties), so every frame lasts one tick. The language is
// und, packed as ISO 639-2 three times five bits.
func mediaHeader(s video.Stream) []byte {
	b := fullBoxStart("mdhd", 0, 0)
	b = appendU32s(b, 0, 0, uint32(s.FPS), uint32(s.Frames))
	b = binary.BigEndian.AppendUint16(b, 0x55C4)
	return boxEnd(binary.BigEndian.AppendUint16(b, 0))
}

func handler() []byte {
	b := fullBoxStart("hdlr", 0, 0)
	b = binary.BigEndian.AppendUint32(b, 0)
	b = append(b, "vide"...)
	b = append(b, make([]byte, 12)...)
	return boxEnd(append(b, "VideoHandler\x00"...))
}

func videoHeader() []byte {
	return boxEnd(append(fullBoxStart("vmhd", 0, 1), make([]byte, 8)...))
}

// dataInformation says the media data is in this file.
func dataInformation() []byte {
	url := boxEnd(fullBoxStart("url ", 0, 1))
	dref := boxEnd(append(binary.BigEndian.AppendUint32(fullBoxStart("dref", 0, 0), 1), url...))
	return boxEnd(append(boxStart("dinf"), dref...))
}

// sampleDescription is the one AV1 sample entry: the picture's size, which
// SHALL be the sequence header's (AV1 binding section 2.2.4), the codec
// configuration with the sequence header in it, and the colour - the numbers
// the sequence header uses, said again for a player that reads the container's
// description rather than the stream's, as the binding says the entry SHOULD
// (section 2.3.4).
func sampleDescription(s video.Stream) []byte {
	e := boxStart("av01")
	e = append(e, make([]byte, 6)...)
	e = binary.BigEndian.AppendUint16(e, 1) // data_reference_index
	e = append(e, make([]byte, 16)...)
	e = binary.BigEndian.AppendUint16(e, uint16(s.Width))
	e = binary.BigEndian.AppendUint16(e, uint16(s.Height))
	e = appendU32s(e, 0x00480000, 0x00480000, 0)
	e = binary.BigEndian.AppendUint16(e, 1) // frame_count
	name := make([]byte, 32)
	name[0] = byte(len(compressor))
	copy(name[1:], compressor)
	e = append(e, name...)
	e = binary.BigEndian.AppendUint16(e, 0x0018)
	e = binary.BigEndian.AppendUint16(e, 0xFFFF)
	e = append(e, boxEnd(append(boxStart("av1C"), s.Config()...))...)
	colr := append(boxStart("colr"), "nclx"...)
	colr = binary.BigEndian.AppendUint16(colr, 1) // BT.709 primaries
	colr = binary.BigEndian.AppendUint16(colr, 1) // BT.709 transfer
	colr = binary.BigEndian.AppendUint16(colr, 1) // BT.709 matrix
	e = append(e, boxEnd(append(colr, 0))...)     // studio range
	e = boxEnd(e)
	return boxEnd(append(binary.BigEndian.AppendUint32(fullBoxStart("stsd", 0, 0), 1), e...))
}

// timeToSample is one run: every frame lasts one tick of the frame rate.
func timeToSample(s video.Stream) []byte {
	b := binary.BigEndian.AppendUint32(fullBoxStart("stts", 0, 0), 1)
	return boxEnd(appendU32s(b, uint32(s.Frames), 1))
}

// boxStart begins a box whose length boxEnd fills in once its content is
// there - for the small boxes built whole. The tables are written as a stream
// and their length counted instead.
func boxStart(kind string) []byte {
	b := make([]byte, 4, 64)
	return append(b, kind...)
}

func fullBoxStart(kind string, version byte, flags uint32) []byte {
	return append(boxStart(kind), version, byte(flags>>16), byte(flags>>8), byte(flags))
}

func boxEnd(b []byte) []byte {
	binary.BigEndian.PutUint32(b, uint32(len(b)))
	return b
}

func appendU32s(b []byte, vs ...uint32) []byte {
	for _, v := range vs {
		b = binary.BigEndian.AppendUint32(b, v)
	}
	return b
}
