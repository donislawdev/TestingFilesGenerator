package core

import "strconv"

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
