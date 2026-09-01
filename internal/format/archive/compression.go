package archive

import (
	"compress/flate"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// How hard the archive is squeezed, in one vocabulary for both containers.
//
// The words are deliberately not the mechanism. ZIP picks a method per entry
// and TAR.GZ compresses the whole stream, so "deflate level 6" and "gzip level
// 6" are the same intent said two ways - and a person choosing a setting is
// choosing how they want to trade time for size, not which library runs.
//
// none is the default and has to stay the default: every archive this tool has
// written so far is stored, so any other default would move the bytes of all of
// them, which is untouchable rule 3.
const Compression = "compression"

const (
	CompressNone    = "none"
	CompressFast    = "fast"
	CompressDefault = "default"
	CompressBest    = "best"
)

// Levels are the flate and gzip levels the words mean. Measured 2026-09-01 on
// a repeating text payload: level 1 came to 1304 B where levels 5, 6 and 9 all
// came to 616 B, so fast really is a different answer rather than a label. On
// a 10 MB archive one pass costs 25 ms at level 1 against 140 ms at level 6.
var levels = map[string]int{
	CompressNone:    flate.NoCompression,
	CompressFast:    1,
	CompressDefault: 6,
	CompressBest:    9,
}

// Squeeze is what a container should do with the bytes.
type Squeeze struct {
	// Name is the word the person asked for, for the manifest.
	Name string
	// Level is the flate or gzip level it means.
	Level int
}

// On reports whether anything is actually compressed. The zero value is off,
// which is what every archive written before this existed did.
func (s Squeeze) On() bool { return s.Name != "" && s.Name != CompressNone }

// ReadCompression works out how hard to squeeze, and refuses the two
// combinations that cannot mean what they say.
//
// The refusals are not tidiness, and each has a measurement behind it.
//
// Compression with a size that comes from the CONTENTS cannot be planned. The
// archive's size would then be whatever the contents compress to, and that is
// knowable only by compressing them - which is exactly what the guard on
// planning forbids. Measured 2026-09-01: our content compresses at about
// 50 MB/s, and that guard plans three gigabytes of declared contents in
// milliseconds against a twenty second ceiling. Compressing to find the answer
// would take about a minute.
//
// Compression with a PASSWORD cannot be streamed. A locked entry goes through
// CreateRaw, which needs the compressed length in the header before any of the
// data is written, so the entry would have to be deflated into memory first -
// and a generator holding a whole entry in memory is the other rule this
// project holds. Both halves are named in each message, because from "this is
// not allowed" nobody can tell which of the two to change.
func ReadCompression(id string, r format.Request, locked bool) (Squeeze, error) {
	raw, ok := r.Properties[Compression]
	if !ok || raw == "" {
		return Squeeze{Name: CompressNone, Level: flate.NoCompression}, nil
	}
	level, known := levels[raw]
	if !known {
		return Squeeze{}, &format.PropertyValueError{
			Format: id,
			Key:    Compression,
			Value:  raw,
			Reason: "it takes one of: " + CompressBest + ", " + CompressDefault + ", " + CompressFast + ", " + CompressNone,
			Remedy: "Ask for " + CompressNone + " to store the files as they are.",
		}
	}
	s := Squeeze{Name: raw, Level: level}
	if !s.On() {
		return s, nil
	}

	if r.SizeFromContents {
		return Squeeze{}, &format.PropertyValueError{
			Format: id,
			Key:    Compression,
			Value:  raw,
			Reason: "the size is being left to the contents, and how far they compress is only known once they have been compressed",
			Remedy: "Give the archive an explicit size, or ask for " + Compression + ": " + CompressNone + ".",
		}
	}
	if locked {
		return Squeeze{}, &format.PropertyValueError{
			Format: id,
			Key:    Compression,
			Value:  raw,
			Reason: "the archive is locked with a " + Password + ", and a locked entry states its length before its data is written - so a compressed one would have to be held in memory whole",
			Remedy: "Ask for " + Compression + ": " + CompressNone + ", or take the " + Password + " off.",
		}
	}
	return s, nil
}
