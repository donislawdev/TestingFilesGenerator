// Package textenc is the character encoding the text formats share.
//
// It lives here rather than inside one of them because TXT and MD ask the same
// question and have to answer it in the same words. A copy in each is how
// thirteen packages ended up carrying four different versions of one filler
// loop, and the same rule applies to a setting a person reads.
//
// What it holds is arithmetic as much as bytes. A file written in UTF-16 is a
// whole number of sixteen bit units, so its length is always even - which
// means half of all sizes stop being reachable, and the exact size promise
// turns them into refusals rather than into files of the wrong size.
// Measured 2026-09-07 on three independent readers: a UTF-16 file cut to an
// odd length is REJECTED by Python, by V8 and by .NET when each is asked
// strictly, and quietly repaired by all three when it is not. A file whose
// corruption only a strict reader can see is the silence this project bans.
package textenc

import (
	"fmt"
	"io"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// Setting names. Public names, so they are spelled once.
const (
	Setting    = "encoding"
	SettingBOM = "bom"
)

// The encodings on offer.
//
// Single byte encodings are deliberately absent, and that is a measurement
// rather than an omission. The filler vocabulary of every text format here is
// ASCII, so a file written as latin-1 comes out byte for byte identical to the
// same file written as UTF-8 - compared with cmp on 2026-09-07. Offering one
// would be offering a setting that changes nothing, which this project treats
// as a refusal rather than as a choice. They become real the day the content
// stops being English, and that is a different change with its own golden
// bytes.
const (
	UTF8    = "utf-8"
	UTF16LE = "utf-16le"
	UTF16BE = "utf-16be"
)

// Codec is one encoding, with or without a byte order mark.
type Codec struct {
	name string
	// wide is two bytes per character rather than one, which is what makes
	// odd sizes unreachable.
	wide bool
	// big is the byte order of a wide encoding, and means nothing without it.
	big bool
	bom bool
}

// Default is what a recipe that says nothing gets, and it has to stay the
// bytes these formats have always written: UTF-8 with no mark in front.
func Default() Codec { return Codec{name: UTF8} }

// Parse reads the two settings.
//
// A value outside the declared set has already been refused by the registry,
// which checks every format against its declaration in one place. These
// branches stay for the same reason the CSV dialect keeps its own: this
// function is callable directly, a guard is such a caller, and a generator
// that trusts its input is one registry change away from writing a file
// nobody ordered.
func Parse(formatID string, props map[string]string) (Codec, error) {
	c := Default()

	if v, ok := props[Setting]; ok && v != "" {
		switch v {
		case UTF8:
			c.name, c.wide, c.big = UTF8, false, false
		case UTF16LE:
			c.name, c.wide, c.big = UTF16LE, true, false
		case UTF16BE:
			c.name, c.wide, c.big = UTF16BE, true, true
		default:
			return Codec{}, &format.PropertyValueError{
				Format: formatID, Key: Setting, Value: v,
				Reason: "it has to be " + UTF8 + ", " + UTF16LE + " or " + UTF16BE,
			}
		}
	}

	if v, ok := props[SettingBOM]; ok && v != "" {
		switch v {
		case "true":
			c.bom = true
		case "false":
			c.bom = false
		default:
			return Codec{}, &format.PropertyValueError{
				Format: formatID, Key: SettingBOM, Value: v,
				Reason: "it has to be true or false",
			}
		}
	}

	return c, nil
}

// Name is the encoding as the manifest records it.
func (c Codec) Name() string { return c.name }

// HasBOM says whether a mark is written, for the manifest.
func (c Codec) HasBOM() bool { return c.bom }

// Preamble is the bytes in front of the content, empty when there is no mark.
func (c Codec) Preamble() []byte {
	if !c.bom {
		return nil
	}
	switch {
	case !c.wide:
		return []byte{0xEF, 0xBB, 0xBF}
	case c.big:
		return []byte{0xFE, 0xFF}
	default:
		return []byte{0xFF, 0xFE}
	}
}

// width is how many bytes one ASCII character costs in this encoding.
//
// The generators build their content as ASCII and this is what turns that
// into a byte count. It holds because the vocabulary and the label are ASCII,
// which a guard checks rather than this file assuming.
func (c Codec) width() int64 {
	if c.wide {
		return 2
	}
	return 1
}

// Source is how many ASCII bytes of content fit in a file of this size.
func (c Codec) Source(fileBytes int64) int64 {
	return (fileBytes - int64(len(c.Preamble()))) / c.width()
}

// Cost is what that many ASCII bytes take up once encoded.
func (c Codec) Cost(sourceBytes int64) int64 { return sourceBytes * c.width() }

// Check refuses a size this encoding cannot write exactly, in the four parts
// every refusal in this tool carries.
//
// Two things can be wrong and they are different sentences. A file smaller
// than its own mark cannot exist at all. A file of an odd length in a wide
// encoding cannot exist either, but the size ASKED FOR is not too small - the
// one below it is fine - so the reason says which sizes are reachable rather
// than pretending there is a floor.
func (c Codec) Check(formatName string, requested int64) error {
	mark := int64(len(c.Preamble()))
	if requested < mark {
		return &format.BelowMinimumError{
			Format: formatName, Requested: requested, Minimum: mark,
			Reason: fmt.Sprintf(
				"a byte order mark is %d B and the file has to hold it", mark),
			Hint: fmt.Sprintf("Ask for %d B or more, or turn the %s setting off.", mark, SettingBOM),
		}
	}
	if next, ok := c.fits(requested); !ok {
		return &format.BelowMinimumError{
			Format: formatName, Requested: requested, Minimum: next,
			Reason: fmt.Sprintf(
				"%s stores two bytes for every character, so a whole file always has an even number of them",
				c.name),
			Hint: fmt.Sprintf("Ask for %d B or %d B.", requested-1, next),
		}
	}
	return nil
}

// fits says whether a file of exactly this many bytes can be written, and
// names the next size that can when it cannot.
func (c Codec) fits(fileBytes int64) (next int64, ok bool) {
	if (fileBytes-int64(len(c.Preamble())))%c.width() == 0 {
		return fileBytes, true
	}
	return fileBytes + 1, false
}

// Writer wraps a writer so that UTF-8 written to it comes out in this
// encoding.
//
// UTF-8 gets the writer straight back rather than a wrapper that copies. That
// is not a micro optimisation: it is what keeps the default path producing the
// same bytes through the same calls it always did, so the pinned hashes and
// the allocation ceiling both stay where they were.
func (c Codec) Writer(w io.Writer) io.Writer {
	if !c.wide {
		return w
	}
	return &wideWriter{out: w, big: c.big}
}

// wideWriter turns UTF-8 into UTF-16 as it goes, holding no more than one
// incomplete character between calls.
type wideWriter struct {
	out io.Writer
	big bool
	// part is the tail of a character split across two writes. Our own
	// generators write whole words, so it stays empty for them - it is here
	// because a writer that only works when its caller is careful is a trap
	// for the next caller.
	part []byte
	buf  []byte
}

func (w *wideWriter) Write(p []byte) (int, error) {
	n := len(p)
	if len(w.part) > 0 {
		p = append(w.part, p...)
		w.part = w.part[:0]
	}

	w.buf = w.buf[:0]
	for len(p) > 0 {
		r, size := utf8.DecodeRune(p)
		if r == utf8.RuneError && size == 1 && !utf8.FullRune(p) {
			// An incomplete character at the end. Keep it for the next call
			// rather than encoding a replacement nobody asked for.
			w.part = append(w.part[:0], p...)
			break
		}
		w.buf = appendRune(w.buf, r, w.big)
		p = p[size:]
	}

	if _, err := w.out.Write(w.buf); err != nil {
		return 0, err
	}
	return n, nil
}

// appendRune writes one character as one or two sixteen bit units.
func appendRune(dst []byte, r rune, big bool) []byte {
	if r1, r2 := utf16.EncodeRune(r); r1 != utf8.RuneError {
		return appendUnit(appendUnit(dst, uint16(r1), big), uint16(r2), big)
	}
	return appendUnit(dst, uint16(r), big)
}

func appendUnit(dst []byte, u uint16, big bool) []byte {
	if big {
		return append(dst, byte(u>>8), byte(u))
	}
	return append(dst, byte(u), byte(u>>8))
}

// Properties is the declaration both text formats hand to the registry, so
// the two cannot describe the same setting differently.
func Properties() []format.Property {
	return []format.Property{
		{
			Name: Setting, Kind: format.PropertyChoice,
			Choices: []string{UTF16BE, UTF16LE, UTF8}, Default: UTF8,
			Detail: "Which encoding the characters are written in. UTF-16 stores two bytes per character, so a file in it always has an even number of bytes and an odd size is refused.",
		},
		{
			Name: SettingBOM, Kind: format.PropertyBool,
			Default: "false",
			Detail:  "Whether the file opens with a byte order mark. A reader that has to guess the encoding needs one, and a reader that does not expect it shows it as stray characters at the start of the file.",
		},
	}
}
