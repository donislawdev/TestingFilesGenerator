package imagelabel

// A 3 by 5 pixel font, drawn here rather than taken from a font file.
//
// Two reasons. A font file is a dependency with a licence to check, and this
// project answers that question often enough already. And a bitmap font is
// exact - the same label renders to the same pixels on every machine and
// every version, which the byte stability promise needs.
//
// Letters are drawn in upper case only. At three pixels wide, lower case is
// not legible, and the label is metadata rather than content.
//
// Each glyph is five rows of three columns. A '#' is a lit pixel.

const (
	glyphWidth  = 3
	glyphHeight = 5
)

var glyphs = map[rune]string{
	'A': "###" + "#.#" + "###" + "#.#" + "#.#",
	'B': "##." + "#.#" + "##." + "#.#" + "##.",
	'C': "###" + "#.." + "#.." + "#.." + "###",
	'D': "##." + "#.#" + "#.#" + "#.#" + "##.",
	'E': "###" + "#.." + "##." + "#.." + "###",
	'F': "###" + "#.." + "##." + "#.." + "#..",
	'G': "###" + "#.." + "#.#" + "#.#" + "###",
	'H': "#.#" + "#.#" + "###" + "#.#" + "#.#",
	'I': "###" + ".#." + ".#." + ".#." + "###",
	'J': "..#" + "..#" + "..#" + "#.#" + "###",
	'K': "#.#" + "#.#" + "##." + "#.#" + "#.#",
	'L': "#.." + "#.." + "#.." + "#.." + "###",
	'M': "#.#" + "###" + "###" + "#.#" + "#.#",
	'N': "#.#" + "###" + "###" + "###" + "#.#",
	'O': "###" + "#.#" + "#.#" + "#.#" + "###",
	'P': "###" + "#.#" + "###" + "#.." + "#..",
	'Q': "###" + "#.#" + "#.#" + "###" + "..#",
	'R': "###" + "#.#" + "##." + "#.#" + "#.#",
	'S': "###" + "#.." + "###" + "..#" + "###",
	'T': "###" + ".#." + ".#." + ".#." + ".#.",
	'U': "#.#" + "#.#" + "#.#" + "#.#" + "###",
	'V': "#.#" + "#.#" + "#.#" + "#.#" + ".#.",
	'W': "#.#" + "#.#" + "###" + "###" + "#.#",
	'X': "#.#" + "#.#" + ".#." + "#.#" + "#.#",
	'Y': "#.#" + "#.#" + ".#." + ".#." + ".#.",
	'Z': "###" + "..#" + ".#." + "#.." + "###",

	'0': "###" + "#.#" + "#.#" + "#.#" + "###",
	'1': ".#." + "##." + ".#." + ".#." + "###",
	'2': "###" + "..#" + "###" + "#.." + "###",
	'3': "###" + "..#" + "###" + "..#" + "###",
	'4': "#.#" + "#.#" + "###" + "..#" + "..#",
	'5': "###" + "#.." + "###" + "..#" + "###",
	'6': "###" + "#.." + "###" + "#.#" + "###",
	'7': "###" + "..#" + "..#" + "..#" + "..#",
	'8': "###" + "#.#" + "###" + "#.#" + "###",
	'9': "###" + "#.#" + "###" + "..#" + "###",

	' ': "..." + "..." + "..." + "..." + "...",
	'-': "..." + "..." + "###" + "..." + "...",
	'.': "..." + "..." + "..." + "..." + ".#.",
	':': "..." + ".#." + "..." + ".#." + "...",
	'_': "..." + "..." + "..." + "..." + "###",
	'/': "..#" + "..#" + ".#." + "#.." + "#..",
	'+': "..." + ".#." + "###" + ".#." + "...",
	'#': "#.#" + "###" + "#.#" + "###" + "#.#",
	// The label of a damaged file names its settings - missing=50,
	// bytes=16,other=2 - and these two are how it writes them.
	'=': "..." + "###" + "..." + "###" + "...",
	',': "..." + "..." + "..." + ".#." + "#..",
}

// unknown stands in for any character with no glyph, so a label never
// silently loses part of itself. It is drawn the same as 0 and O, which is why
// HasGlyph asks the table rather than comparing with this.
const unknown = "###" + "#.#" + "#.#" + "#.#" + "###"

func glyph(r rune) string {
	if g, ok := glyphs[upper(r)]; ok {
		return g
	}
	return unknown
}

// HasGlyph says whether r is drawn as itself rather than as the stand-in, for
// the guard that asks it of every character a label can hold.
func HasGlyph(r rune) bool {
	_, ok := glyphs[upper(r)]
	return ok
}

// upper is r in the case the table is drawn in - see the top of this file.
func upper(r rune) rune {
	if r >= 'a' && r <= 'z' {
		return r - ('a' - 'A')
	}
	return r
}
