// Package imagedim is the width and height that every picture format shares.
//
// It lives here rather than inside one of them because ten formats ask the
// same question and used to answer it ten times. Measured on 2026-09-08: the
// two declarations were written out in full in all ten packages, and the
// function reading them - the same twelve lines, differing only in the format
// named in its message - stood in all ten as well. That is the shape thirteen
// packages had before one filler loop replaced their four, and the shape two
// containers had before the archive package replaced their two.
//
// What is shared is what cannot legitimately differ: the name, that it is a
// whole number, that the smallest picture is one pixel on a side, and that the
// number counts pixels. What each format supplies is what genuinely differs
// between them, which was measured before this package existed rather than
// guessed:
//
//   - The largest side. Five formats reach 20000, AVIF and JXL 16384, WEBP
//     16383 and ICO 256, and each of those is the ceiling of the thing itself
//     rather than a number somebody picked.
//   - The sentence. An icon is not a picture and an SVG only SAYS how wide it
//     is, so four different sentences are correct rather than four copies of
//     one. A shared sentence would have made three of them wrong.
//   - The default, which only SVG declares, because it is the only one that
//     cannot work a size out from the bytes asked for.
//
// The shape is enforced by construction rather than by a check: a format
// cannot declare a width of a different kind or in a different unit, because
// it does not write those fields. What stops an ELEVENTH format declaring
// width its own way without using this package at all is
// TestOneSettingNameMeansOneKindOfSetting.
package imagedim

import (
	"fmt"
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// Setting names. Public names, so they are spelled once.
const (
	SettingWidth  = "width"
	SettingHeight = "height"
)

const (
	// Smallest is the shortest side any picture can have. Zero is not a
	// picture, and every format that has ever declared these agreed on one.
	Smallest = 1
	// Unit is what the number counts, for the refusal and for a window's field.
	Unit = "pixels"
)

// Side is one dimension of a picture as one format takes it.
type Side struct {
	// Largest is the longest side this format can encode. It is the ceiling of
	// the format or of its encoder, not a limit chosen for comfort.
	Largest int64
	// Default is what the format uses when nothing says otherwise, written the
	// way a person writes it. Empty means the format works it out from the
	// bytes it was asked for, which is what nine of the ten do.
	Default string
	// Detail is the one sentence a person reads, beside the field in a window
	// and under the name in "tfg formats". It is per format on purpose - see
	// the note above this package.
	Detail string
}

// Width is the declaration of the width setting for a format taking this side.
func Width(s Side) format.Property { return s.property(SettingWidth) }

// Height is the declaration of the height setting for a format taking this side.
func Height(s Side) format.Property { return s.property(SettingHeight) }

func (s Side) property(name string) format.Property {
	return format.Property{
		Name: name, Kind: format.PropertyInt,
		Min: Smallest, Max: s.Largest, Unit: Unit,
		Default: s.Default,
		Detail:  s.Detail,
	}
}

// Value reads one side out of what a recipe stated, falling back to what the
// format works out for itself when nothing was said.
//
// The range is checked here as well as in the registry, and that is deliberate
// rather than left over. The registry refuses first on both surfaces - measured
// on 2026-09-08, where a width of 999999 comes back as exit 4 with the sentence
// built from the declaration, and nothing in internal/guard reaches a generator
// with a value outside its range. But this function is callable directly, a
// guard is such a caller, and a generator that trusts its input is one registry
// change away from encoding a picture nobody ordered. Same reasoning as the
// branches textenc.Parse keeps, and it costs one copy now rather than ten.
//
// The refusal is the structured one rather than a sentence, and that is the
// ten being unified on the BEST of them rather than on the most common. Nine
// built a string with fmt.Errorf and named the format inside it by hand. SVG
// returned format.PropertyValueError - the same type the registry itself
// raises, so what reports it can ask for the parts separately instead of
// parsing a sentence. The wording is SVG's, unchanged, because that is the one
// of the ten a reader could actually reach.
func Value(formatID, key string, props map[string]string, largest, fallback int) (int, error) {
	raw, ok := props[key]
	if !ok || raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, &format.PropertyValueError{
			Format: formatID, Key: key, Value: raw,
			Reason: "it has to be a whole number of " + Unit,
		}
	}
	if n < Smallest || n > largest {
		return 0, &format.PropertyValueError{
			Format: formatID, Key: key, Value: raw,
			Reason: fmt.Sprintf("it has to be between %d and %d", Smallest, largest),
		}
	}
	return n, nil
}
