package guard

import (
	"encoding/binary"
	"fmt"
	"math"
)

// A WebM film read back from its bytes into what the film guards ask
// (film_test.go): its elements and blocks as far as those questions need, and
// no further. What it does not understand it refuses, so a change in the writer
// that this reader cannot follow fails loudly rather than being read as
// something it is not. filmmp4_test.go reads an MP4 into the same shape.

// ebmlVint reads an element ID (keeping its marker) or a size (without it).
func ebmlVint(b []byte, keepMarker bool) (value uint64, length int, err error) {
	if len(b) == 0 {
		return 0, 0, fmt.Errorf("the file ends where an element should start")
	}
	length = 1
	for mask := byte(0x80); length <= 8 && b[0]&mask == 0; mask >>= 1 {
		length++
	}
	if length > 8 || length > len(b) {
		return 0, 0, fmt.Errorf("a variable length number runs past the end")
	}
	value = uint64(b[0])
	if !keepMarker {
		value &= uint64(0xFF >> length)
	}
	for _, c := range b[1:length] {
		value = value<<8 | uint64(c)
	}
	return value, length, nil
}

// ebmlElements walks a run of elements, each exactly covering the next, and
// refuses one whose size runs past the run.
func ebmlElements(b []byte, each func(id uint64, body []byte) error) error {
	for pos := 0; pos < len(b); {
		id, n, err := ebmlVint(b[pos:], true)
		if err != nil {
			return err
		}
		size, m, err := ebmlVint(b[pos+n:], false)
		if err != nil {
			return err
		}
		start := pos + n + m
		if uint64(len(b)-start) < size {
			return fmt.Errorf("element %#x at %d says %d B and %d remain", id, pos, size, len(b)-start)
		}
		if err := each(id, b[start:start+int(size)]); err != nil {
			return err
		}
		pos = start + int(size)
	}
	return nil
}

func beUint(b []byte) uint64 {
	var v uint64
	for _, c := range b {
		v = v<<8 | uint64(c)
	}
	return v
}

func walkWebM(b []byte) (filmRead, error) {
	var f filmRead
	sawSegment := false
	err := ebmlElements(b, func(id uint64, body []byte) error {
		switch id {
		case 0x1A45DFA3:
			return nil
		case 0x18538067:
			sawSegment = true
			return walkSegment(&f, body)
		}
		return fmt.Errorf("a top level element %#x that is neither the EBML header nor the segment", id)
	})
	if err == nil && !sawSegment {
		err = fmt.Errorf("no segment")
	}
	if f.tail != [2]uint64{0xEC, 0x1C53BB6B} {
		f.ends = fmt.Sprintf("the segment ends in elements %#x and %#x, and it ends in the padding and then the Cues", f.tail[0], f.tail[1])
	}
	return f, err
}

func walkSegment(f *filmRead, body []byte) error {
	return ebmlElements(body, func(id uint64, el []byte) error {
		f.tail = [2]uint64{f.tail[1], id}
		switch id {
		case 0x1549A966: // Info
			return ebmlElements(el, func(id uint64, v []byte) error {
				if id == 0x4489 {
					f.durationMs = math.Float64frombits(binary.BigEndian.Uint64(v))
				}
				return nil
			})
		case 0x1654AE6B: // Tracks
			return walkTracks(f, el)
		case 0x1F43B675: // Cluster
			return walkCluster(f, el)
		case 0x1C53BB6B: // Cues
			return ebmlElements(el, func(id uint64, _ []byte) error {
				if id == 0xBB {
					f.index++
				}
				return nil
			})
		}
		return nil
	})
}

func walkTracks(f *filmRead, el []byte) error {
	return ebmlElements(el, func(id uint64, entry []byte) error {
		return ebmlElements(entry, func(id uint64, v []byte) error {
			switch id {
			case 0x63A2:
				f.codecPrivate = v
			case 0xE0:
				return ebmlElements(v, func(id uint64, d []byte) error {
					switch id {
					case 0xB0:
						f.width = int(beUint(d))
					case 0xBA:
						f.height = int(beUint(d))
					}
					return nil
				})
			}
			return nil
		})
	})
}

func walkCluster(f *filmRead, el []byte) error {
	var clusterTs int64
	return ebmlElements(el, func(id uint64, v []byte) error {
		switch id {
		case 0xE7:
			clusterTs = int64(beUint(v))
		case 0xA3:
			if len(v) < 4 || v[0] != 0x81 {
				return fmt.Errorf("a block that is not on track 1")
			}
			rel := int64(int16(binary.BigEndian.Uint16(v[1:3])))
			f.blocks = append(f.blocks, filmBlock{ts: clusterTs + rel, key: v[3]&0x80 != 0, data: v[4:]})
		default:
			return fmt.Errorf("a cluster holds element %#x", id)
		}
		return nil
	})
}
