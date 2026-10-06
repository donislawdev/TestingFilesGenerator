package webm

import (
	"encoding/binary"
	"math"
)

// Element IDs, RFC 9559 and the WebM subset of it. Written with their marker
// bits, the way they appear in the file.
const (
	idEBML               = 0x1A45DFA3
	idEBMLVersion        = 0x4286
	idEBMLReadVersion    = 0x42F7
	idEBMLMaxIDLength    = 0x42F2
	idEBMLMaxSizeLength  = 0x42F3
	idDocType            = 0x4282
	idDocTypeVersion     = 0x4287
	idDocTypeReadVersion = 0x4285

	idSegment      = 0x18538067
	idSeekHead     = 0x114D9B74
	idSeek         = 0x4DBB
	idSeekID       = 0x53AB
	idSeekPosition = 0x53AC

	idInfo           = 0x1549A966
	idTimestampScale = 0x2AD7B1
	idDuration       = 0x4489
	idMuxingApp      = 0x4D80
	idWritingApp     = 0x5741

	idTracks          = 0x1654AE6B
	idTrackEntry      = 0xAE
	idTrackNumber     = 0xD7
	idTrackUID        = 0x73C5
	idTrackType       = 0x83
	idFlagLacing      = 0x9C
	idCodecID         = 0x86
	idCodecPrivate    = 0x63A2
	idDefaultDuration = 0x23E383
	idVideo           = 0xE0
	idPixelWidth      = 0xB0
	idPixelHeight     = 0xBA
	idColour          = 0x55B0
	idMatrix          = 0x55B1
	idRange           = 0x55B9
	idTransfer        = 0x55BA
	idPrimaries       = 0x55BB

	idCluster     = 0x1F43B675
	idTimestamp   = 0xE7
	idSimpleBlock = 0xA3

	idCues               = 0x1C53BB6B
	idCuePoint           = 0xBB
	idCueTime            = 0xB3
	idCueTrackPositions  = 0xB7
	idCueTrack           = 0xF7
	idCueClusterPosition = 0xF1

	idVoid = 0xEC
)

// idBytes is an element ID as it is written: its own length, marker included.
func idBytes(id uint32) []byte { return appendBigEndian(nil, uint64(id), idLen(id)) }

// sizeLen is how many bytes the shortest size field for n takes. The value
// whose bits are all ones in a length is reserved for "unknown size", which is
// why a length of k holds at most 2^(7k) - 2.
func sizeLen(n uint64) int {
	for k := 1; k < 8; k++ {
		if n < 1<<(7*uint(k))-1 {
			return k
		}
	}
	return 8
}

// sizeField writes n in exactly k bytes.
func sizeField(n uint64, k int) []byte {
	out := make([]byte, k)
	v := n | 1<<(7*uint(k))
	for i := k - 1; i >= 0; i-- {
		out[i] = byte(v)
		v >>= 8
	}
	return out
}

// appendHeader writes an element's ID and the shortest size field for its
// content onto dst, and allocates nothing when dst has the room - which is
// what lets the frames of a film be written without one allocation each.
func appendHeader(dst []byte, id uint32, content uint64) []byte {
	dst = appendBigEndian(dst, uint64(id), idLen(id))
	k := sizeLen(content)
	return appendBigEndian(dst, content|1<<(7*uint(k)), k)
}

// appendUint is an unsigned integer element in the fewest bytes, at least one,
// written onto dst.
func appendUint(dst []byte, id uint32, v uint64) []byte {
	n := uintLen(v)
	return appendBigEndian(appendHeader(dst, id, uint64(n)), v, n)
}

// appendBytes is an element holding b, written onto dst.
func appendBytes(dst []byte, id uint32, b []byte) []byte {
	return append(appendHeader(dst, id, uint64(len(b))), b...)
}

func appendString(dst []byte, id uint32, s string) []byte {
	return append(appendHeader(dst, id, uint64(len(s))), s...)
}

// appendFloat is a float element in eight bytes, which is how Matroska writes
// a duration.
func appendFloat(dst []byte, id uint32, f float64) []byte {
	return binary.BigEndian.AppendUint64(appendHeader(dst, id, 8), math.Float64bits(f))
}

// appendFixedUint is an unsigned integer in eight bytes whatever its value.
func appendFixedUint(dst []byte, id uint32, v uint64) []byte {
	return binary.BigEndian.AppendUint64(appendHeader(dst, id, 8), v)
}

// appendBigEndian writes the low n bytes of v, most significant first.
func appendBigEndian(dst []byte, v uint64, n int) []byte {
	for i := n - 1; i >= 0; i-- {
		dst = append(dst, byte(v>>(8*uint(i))))
	}
	return dst
}

// idLen is how many bytes an element ID takes - its own length, marker
// included, which is where its highest byte is.
func idLen(id uint32) int {
	switch {
	case id >= 1<<24:
		return 4
	case id >= 1<<16:
		return 3
	case id >= 1<<8:
		return 2
	}
	return 1
}

// uintLen is how many bytes the shortest unsigned integer for v takes, at
// least one.
func uintLen(v uint64) int {
	n := 1
	for n < 8 && v>>(8*uint(n)) != 0 {
		n++
	}
	return n
}

// elementLen is how many bytes a whole element takes.
func elementLen(id uint32, content uint64) uint64 {
	return uint64(idLen(id)+sizeLen(content)) + content
}

// uintElementLen is uintElement's length without building it.
func uintElementLen(id uint32, v uint64) uint64 {
	return elementLen(id, uint64(uintLen(v)))
}
