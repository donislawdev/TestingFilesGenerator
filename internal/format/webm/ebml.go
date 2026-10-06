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
func idBytes(id uint32) []byte {
	switch {
	case id >= 1<<24:
		return []byte{byte(id >> 24), byte(id >> 16), byte(id >> 8), byte(id)}
	case id >= 1<<16:
		return []byte{byte(id >> 16), byte(id >> 8), byte(id)}
	case id >= 1<<8:
		return []byte{byte(id >> 8), byte(id)}
	}
	return []byte{byte(id)}
}

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

// header is an element's ID and the shortest size field for its content.
func header(id uint32, content uint64) []byte {
	return append(idBytes(id), sizeField(content, sizeLen(content))...)
}

// elementLen is how many bytes a whole element takes.
func elementLen(id uint32, content uint64) uint64 {
	return uint64(len(idBytes(id))+sizeLen(content)) + content
}

func element(id uint32, content []byte) []byte {
	return append(header(id, uint64(len(content))), content...)
}

// uintBytes is an unsigned integer in the fewest bytes, at least one.
func uintBytes(v uint64) []byte {
	n := 1
	for v>>(8*uint(n)) != 0 && n < 8 {
		n++
	}
	out := make([]byte, n)
	for i := n - 1; i >= 0; i-- {
		out[i] = byte(v)
		v >>= 8
	}
	return out
}

func uintElement(id uint32, v uint64) []byte { return element(id, uintBytes(v)) }

// uintElementLen is uintElement's length without building it.
func uintElementLen(id uint32, v uint64) uint64 {
	return elementLen(id, uint64(len(uintBytes(v))))
}

func fixedUintElement(id uint32, v uint64) []byte {
	return element(id, binary.BigEndian.AppendUint64(nil, v))
}

func floatElement(id uint32, f float64) []byte {
	return element(id, binary.BigEndian.AppendUint64(nil, math.Float64bits(f)))
}

func stringElement(id uint32, s string) []byte { return element(id, []byte(s)) }

func cat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}
