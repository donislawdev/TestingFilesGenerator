package core

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// Shown is s the way a person should read it: every character HoldsUnseen
// finds, and every byte that is not UTF-8, written as the escape %q would use
// for it, and nothing else changed. No quotes are added.
//
// For every line this tool prints about a name or a path that came from a
// recipe, a preset, a manifest or a directory listing (O241). Measured on
// 2026-09-24: verify reported a missing "photo", right to left override,
// "gpj.txt" as the terminal drew it, which is "phototxt.jpg", and an extra
// "in", zero width space, "voice.txt" as "invoice.txt" - a report naming files
// other than the ones on the disk, two of which could not be told apart.
//
// Without quotes, because a name holding nothing of the kind comes out byte
// for byte as it always did, and every report line of every run that never
// met such a name stays what scripts and people already read. The escape is
// not ambiguous inside a file name: a backslash is refused in one on every
// system (engine/filename.go). In a Windows path it reads as a separator
// followed by a letter and a number, which a person does not mistake for one.
//
// Never for what a program reads. The manifest and every --json report carry
// the name exactly, because a program compares it byte for byte.
func Shown(s string) string {
	if !HoldsUnseen(s) && utf8.ValidString(s) {
		return s
	}
	return shown(s, false)
}

// ShownText is Shown for a whole message rather than one name: the line
// breaks and tabs it is laid out with stay as they are, and everything else
// nobody can see is escaped.
//
// For the places a message is turned into words for a person - the command
// line's describeError and the window's refusals - because an error wrapped
// from the operating system repeats the path it failed on in its own words,
// after this tool's sentence has already shown it. Measured on 2026-09-25,
// from a review: "cannot create the output directory" showed the folder
// escaped and the "mkdir" part after it showed it raw.
func ShownText(s string) string {
	return shown(s, true)
}

func shown(s string, layout bool) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			q := strconv.Quote(s[i : i+1])
			b.WriteString(q[1 : len(q)-1])
		case layout && (r == '\n' || r == '\t'):
			b.WriteRune(r)
		case !strconv.IsPrint(r):
			q := strconv.QuoteRune(r)
			b.WriteString(q[1 : len(q)-1])
		default:
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	return b.String()
}

// ShownEach is Shown for every name of a list, for the lines that name a few
// files one after another.
func ShownEach(names []string) []string {
	out := make([]string, len(names))
	for i, name := range names {
		out[i] = Shown(name)
	}
	return out
}

// HoldsUnseen reports whether s holds a character a person reading it cannot
// see: a character that changes the direction of the text around it, one of
// no width, a byte order mark, a separator that breaks a line without being a
// line break, a space that is not the space bar's, a tag character, and every
// other one Go does not count as printable.
//
// It exists because file names are exactly where such characters are put on
// purpose. A name with a right to left override shows its extension in the
// wrong place, one with a zero width space prints as a name it is not, and
// both are test cases this tool writes (docs/NAMES-PRESET-2026-09-24.md). The
// file keeps its name. What a person reads about it has to show the character
// rather than let it act (O241), and a recipe has to carry it in a form that
// can be read and edited (O244).
//
// The class is strconv.IsPrint turned around, and that is a choice: it is the
// class %q escapes, and the refusals of this tool have quoted names with %q
// all along. One rule means one name looks the same in a refusal, in a report
// and in a recipe. A combining mark is printable and stays as it is. The
// space is printable, every other space is not.
func HoldsUnseen(s string) bool {
	for _, r := range s {
		if !strconv.IsPrint(r) {
			return true
		}
	}
	return false
}
