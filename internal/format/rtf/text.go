package rtf

import "strconv"

// The paragraphs characters=international adds, and the four ways
// text_encoding writes their letters. Each was read by five readers on
// 2026-10-15 before a line of this was written (docs/RTF-2026-10-15.md section
// 6.1 c), and the file this writes is the file they read: the same fonts, the
// same code pages, the same fallback.
//
// Nothing here spells an escape that starts with a backslash and a u followed
// by digits. Those are built from the code point when the file is written, and
// the letters outside ASCII are runes given by number where they would turn a
// line around or vanish (the Arabic, the joiner, the accent), so no tool
// between this file and the compiler can turn an escape into something else.

// script is one paragraph in another script, in the font that names its code
// page.
type script struct {
	font int
	text string
	// rtl puts the paragraph right to left.
	rtl bool
	// page is how the font's code page writes each letter of text outside
	// ASCII. A letter missing from it is one the code page does not hold.
	page map[rune]string
}

// fontTable is the font table of a document with these paragraphs: Arial in
// four code pages and a Japanese font, each saying its character set the way
// Word does - 0 Western, 238 Central European, 204 Cyrillic, 128 Shift JIS,
// 178 Arabic.
const fontTable = `{\fonttbl{\f0\fswiss\fcharset0 Arial;}{\f1\fswiss\fcharset238 Arial;}{\f2\fswiss\fcharset204 Arial;}` +
	`{\f3\fmodern\fcharset128 MS Gothic;}{\f4\fswiss\fcharset178 Arial;}}`

// plainFontTable is the one font of a document in ASCII.
const plainFontTable = `{\fonttbl{\f0\fswiss Arial;}}`

func runes(r ...rune) string { return string(r) }

// scripts are the paragraphs, in the order they are written. The code page
// bytes were taken from Python's codecs cp1250, cp1251, cp932, cp1256 and
// cp1252 on 2026-10-15 rather than from memory, and the structural checker
// decodes every file written in codepage or both with those codecs and holds
// the result to the sentence - so a wrong byte here is a refused file there
// (proven by mutation the same day).
var scripts = []script{
	{font: 1, text: "Zażółć gęślą jaźń.", page: map[rune]string{
		'ż': "\xbf", 'ó': "\xf3", 'ł': "\xb3", 'ć': "\xe6", 'ę': "\xea", 'ś': "\x9c", 'ą': "\xb9", 'ź': "\x9f", 'ń': "\xf1",
	}},
	{font: 2, text: "Это текст на русском языке.", page: map[rune]string{
		'Э': "\xdd", 'т': "\xf2", 'о': "\xee", 'е': "\xe5", 'к': "\xea", 'с': "\xf1", 'н': "\xed",
		'а': "\xe0", 'р': "\xf0", 'у': "\xf3", 'м': "\xec", 'я': "\xff", 'з': "\xe7", 'ы': "\xfb",
	}},
	{font: 3, text: "これは日本語のテキストです。", page: map[rune]string{
		'こ': "\x82\xb1", 'れ': "\x82\xea", 'は': "\x82\xcd", '日': "\x93\xfa", '本': "\x96\x7b", '語': "\x8c\xea",
		'の': "\x82\xcc", 'テ': "\x83\x65", 'キ': "\x83\x4c", 'ス': "\x83\x58", 'ト': "\x83\x67", 'で': "\x82\xc5",
		'す': "\x82\xb7", '。': "\x81\x42",
	}},
	{font: 4, rtl: true,
		text: runes(0x647, 0x630, 0x627, ' ', 0x646, 0x635, ' ', 0x628, 0x627, 0x644, 0x644, 0x63A, 0x629, ' ',
			0x627, 0x644, 0x639, 0x631, 0x628, 0x64A, 0x629, '.'),
		page: map[rune]string{
			0x647: "\xe5", 0x630: "\xd0", 0x627: "\xc7", 0x646: "\xe4", 0x635: "\xd5", 0x628: "\xc8",
			0x644: "\xe1", 0x63A: "\xdb", 0x629: "\xc9", 0x639: "\xda", 0x631: "\xd1", 0x64A: "\xed",
		}},
	{font: 0, text: "Emoji " + runes(0x1F600) + " and a sequence " + runes(0x1F469, 0x200D, 0x1F4BB) +
		", a letter in two code points e" + runes(0x301) + ", the characters RTF escapes " + `\ { }` +
		" and the ones it names " + runes(0x2014, ' ', 0x2013, ' ', 0x2018) + "q" + runes(0x2019, ' ', 0x201C) + "qq" +
		runes(0x201D, ' ', 0x2022) + ", a space that does not break" + runes(0xA0) + "and the euro " + runes(0x20AC) + ".",
		page: map[rune]string{0x20AC: "\x80"}},
}

// named are the characters RTF has a word for. They are written as that word
// whatever text_encoding says, which is what Word does and what the readers
// were measured on.
var named = map[rune]string{
	0x2014: `\emdash `, 0x2013: `\endash `, 0x2018: `\lquote `, 0x2019: `\rquote `,
	0x201C: `\ldblquote `, 0x201D: `\rdblquote `, 0x2022: `\bullet `, 0xA0: `\~`,
}

// international is the paragraphs, written the way encoding says, each in a
// group of its own so the font and the count of fallback bytes end with it.
func international(encoding string) string {
	var dst []byte
	for _, sc := range scripts {
		dst = append(dst, `{\pard`...)
		if sc.rtl {
			dst = append(dst, `\rtlpar`...)
		}
		dst = append(dst, `\f`...)
		dst = strconv.AppendInt(dst, int64(sc.font), 10)
		if sc.rtl {
			dst = append(dst, `\rtlch`...)
		}
		dst = append(dst, ' ')
		dst = appendEncoded(dst, sc.text, sc.page, encoding)
		dst = append(dst, `\par}`+"\r\n"...)
	}
	return string(dst)
}

// appendEncoded writes text in one of the four encodings.
func appendEncoded(dst []byte, text string, page map[rune]string, encoding string) []byte {
	// The count of fallback bytes a reader skips after a number. Each paragraph
	// starts at the 1 the header declares.
	uc := 1
	for _, r := range text {
		switch {
		case r == '\\' || r == '{' || r == '}':
			dst = append(dst, '\\', byte(r))
		case r < 0x80:
			dst = append(dst, byte(r))
		case named[r] != "":
			dst = append(dst, named[r]...)
		default:
			dst, uc = appendLetter(dst, r, page[r], encoding, uc)
		}
	}
	return dst
}

// appendLetter writes one letter outside ASCII, and says what the count of
// fallback bytes is after it.
func appendLetter(dst []byte, r rune, inPage, encoding string, uc int) ([]byte, int) {
	switch {
	case encoding == Codepage && inPage != "":
		return appendHex(dst, inPage), uc
	case encoding == Literal:
		for _, u := range units(r) {
			dst = appendNumber(dst, u)
			dst = append(dst, '?')
		}
		return dst, uc
	case encoding == Both && inPage != "" && r < 0x10000:
		dst, uc = appendCount(dst, len(inPage), uc)
		return appendHex(appendNumber(dst, r), inPage), uc
	}
	// unicode, and every letter no code page holds: the number and a question
	// mark written as a byte, which is what LibreOffice writes itself.
	dst, uc = appendCount(dst, 1, uc)
	for _, u := range units(r) {
		dst = appendHex(appendNumber(dst, u), "?")
	}
	return dst, uc
}

// appendCount says a new count of fallback bytes, when it changes.
func appendCount(dst []byte, want, uc int) ([]byte, int) {
	if want == uc {
		return dst, uc
	}
	dst = append(dst, `\uc`...)
	return strconv.AppendInt(dst, int64(want), 10), want
}

// appendNumber is the backslash, the u and the 16 bit unit as a signed
// decimal - the specification says a value above 32767 is written negative.
func appendNumber(dst []byte, u rune) []byte {
	if u > 32767 {
		u -= 65536
	}
	dst = append(dst, '\\', 'u')
	return strconv.AppendInt(dst, int64(u), 10)
}

// appendHex writes bytes as a backslash, an apostrophe and two lower case hex
// digits each.
func appendHex(dst []byte, b string) []byte {
	const digits = "0123456789abcdef"
	for i := 0; i < len(b); i++ {
		dst = append(dst, '\\', '\'', digits[b[i]>>4], digits[b[i]&0xf])
	}
	return dst
}

// units is a code point as UTF-16 units: one, or a surrogate pair for a code
// point past the basic plane, which is how RTF writes an emoji.
func units(r rune) []rune {
	if r < 0x10000 {
		return []rune{r}
	}
	r -= 0x10000
	return []rune{0xD800 + r>>10, 0xDC00 + r&0x3FF}
}
