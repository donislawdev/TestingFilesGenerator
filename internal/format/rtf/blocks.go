package rtf

import (
	// D11 promises the same bytes from the same seed, so a deliberate,
	// reproducible generator is the product rather than a weakness. Nothing
	// here ever makes a secret.
	// nosemgrep: go.lang.security.audit.crypto.math_random.math-random-used
	"math/rand/v2"
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// The blocks a document is filled with, and the words of the paragraph that
// closes it to the byte.
//
// Every block leaves the document the way it found it: its paragraph closed,
// the paragraph properties reset with \pard, and every change to the letters
// inside a group. So whatever follows starts from the defaults, and the closing
// paragraph can be words and nothing else - a paragraph without \par at the end
// of the document is shown by all five readers measured (docs/RTF-2026-10-15.md
// section 3), which is what lets the closing record be as short as nothing.
//
// A line ends only after a space or after a control word, never between two
// letters. RTF ignores a line end, so one between two letters joins two words
// in every reader - measured in the same section.

const (
	// lineWidth is where a line of words is broken, well inside the 255
	// characters the specification suggests.
	lineWidth = 72
	// endBlock closes a paragraph and resets its properties.
	endBlock = `\par\pard` + "\r\n"
	// lineBreak is what goes between two words where a line ends.
	lineBreak = " \r\n"
)

// blocks is the record the document is filled with.
type blocks struct{}

// Append adds one whole block.
func (blocks) Append(dst []byte, rng *rand.Rand) []byte {
	switch rng.IntN(6) {
	case 0:
		// A heading by its outline level, which pandoc and LibreOffice both
		// read as a heading without a style sheet (measured 2026-10-15), so the
		// header stays as small as it can be.
		dst = append(dst, `\outlinelevel1{\b\fs28 `...)
		dst = appendTitle(dst, rng, 3)
		return append(dst, "}"+endBlock...)
	case 1:
		return appendList(dst, rng)
	case 2:
		return appendTable(dst, rng)
	case 3:
		dst = append(dst, `\li720 `...)
		dst = appendWords(dst, rng, 8, false)
		return append(dst, endBlock...)
	case 4:
		dst = appendWords(dst, rng, 20+rng.IntN(20), true)
		return append(dst, endBlock...)
	default:
		dst = appendWords(dst, rng, 20+rng.IntN(20), false)
		return append(dst, endBlock...)
	}
}

// AppendExact adds the words of the closing paragraph, exactly n bytes of them.
func (blocks) AppendExact(dst []byte, _ *rand.Rand, n int64) []byte { return appendProse(dst, n) }

// Shortest is nothing at all: the closing paragraph can be empty.
func (blocks) Shortest() int64 { return 0 }

// Discard has nothing to undo, since a block counts nothing.
func (blocks) Discard() {}

// appendList is three to five items, each with the bullet Word writes in front
// of it for a reader that does not know lists.
func appendList(dst []byte, rng *rand.Rand) []byte {
	for i, n := 0, 3+rng.IntN(3); i < n; i++ {
		dst = append(dst, `{\pntext\bullet\tab}{\*\pn\pnlvlblt\pnindent360{\pntxtb\bullet}}\fi-360\li720 `...)
		dst = appendWords(dst, rng, 4+rng.IntN(4), false)
		dst = append(dst, `\par`+"\r\n"...)
	}
	return append(dst, `\pard`+"\r\n"...)
}

// appendTable is a row of headings and two to four rows under it.
func appendTable(dst []byte, rng *rand.Rand) []byte {
	dst = appendRow(dst, "id", "field", "value")
	for i, n := 0, 2+rng.IntN(3); i < n; i++ {
		dst = appendRow(dst, strconv.Itoa(i+1), word(rng), word(rng))
	}
	return append(dst, `\pard`+"\r\n"...)
}

func appendRow(dst []byte, cells ...string) []byte {
	dst = append(dst, `\trowd\cellx3000\cellx6000\cellx9000\pard\intbl `...)
	for _, c := range cells {
		dst = append(dst, c...)
		dst = append(dst, `\cell `...)
	}
	return append(dst, `\row`+"\r\n"...)
}

// appendWords is n words, broken into lines at lineWidth, and with styled some
// of them bold or italic, each in a group of its own.
func appendWords(dst []byte, rng *rand.Rand, n int, styled bool) []byte {
	col := 0
	for i := 0; i < n; i++ {
		w := word(rng)
		if i > 0 {
			dst, col = appendSpace(dst, col, len(w))
		}
		open, closing := styleOf(rng, styled)
		dst = append(dst, open...)
		dst = append(dst, w...)
		dst = append(dst, closing...)
		col += len(w)
	}
	return dst
}

// styleOf is the group around a styled word, one word in five: bold or italic.
func styleOf(rng *rand.Rand, styled bool) (open, closing string) {
	if !styled {
		return "", ""
	}
	switch rng.IntN(10) {
	case 0:
		return `{\b `, "}"
	case 1:
		return `{\i `, "}"
	}
	return "", ""
}

// appendSpace is what goes in front of a word of length next at column col:
// a space, or a space and a line end once the line is full.
func appendSpace(dst []byte, col, next int) ([]byte, int) {
	if col+1+next > lineWidth {
		return append(dst, lineBreak...), 0
	}
	return append(dst, ' '), col + 1
}

// appendTitle is a phrase with each word capitalised, for a heading.
func appendTitle(dst []byte, rng *rand.Rand, n int) []byte {
	for i := 0; i < n; i++ {
		if i > 0 {
			dst = append(dst, ' ')
		}
		w := word(rng)
		// The vocabulary is lower case ASCII, so the first byte is the first
		// letter.
		dst = append(dst, w[0]-'a'+'A')
		dst = append(dst, w[1:]...)
	}
	return dst
}

// appendProse is n bytes of words, the lines broken the way appendWords breaks
// them. The cut can land on a space or inside a line end, and both are fine in
// front of the closing braces: RTF ignores the line end, and the space is the
// end of the last word.
func appendProse(dst []byte, n int64) []byte {
	col := len(words[0])
	return core.AppendFiller(dst, words, n, func(i int) string {
		next := len(words[i%len(words)])
		var sep []byte
		sep, col = appendSpace(nil, col, next)
		col += next
		return string(sep)
	})
}

func word(rng *rand.Rand) string { return words[rng.IntN(len(words))] }

// words is the vocabulary: lower case ASCII, so no word holds a character RTF
// escapes and no word starts a control word.
var words = []string{
	"account", "after", "amount", "answer", "before", "between", "branch",
	"buffer", "build", "cache", "change", "client", "column", "config",
	"content", "create", "default", "delete", "detail", "device", "domain",
	"effect", "engine", "entry", "error", "export", "field", "filter",
	"folder", "format", "handle", "header", "import", "index", "input",
	"insert", "invoice", "layer", "length", "level", "limit", "loader",
	"market", "member", "method", "module", "notice", "number", "object",
	"option", "output", "packet", "parent", "parser", "period", "policy",
	"prefix", "process", "public", "query", "reason", "record", "region",
	"report", "result", "return", "sample", "schema", "script", "search",
	"sender", "server", "signal", "single", "source", "status", "stream",
	"string", "switch", "system", "target", "thread", "ticket", "toggle",
	"token", "update", "upload", "vendor", "verify", "version", "volume",
	"window", "worker", "writer",
}
