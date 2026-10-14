package htmlfile

import (
	// D11 promises the same bytes from the same seed, so a deliberate,
	// reproducible generator is the product rather than a weakness. Nothing
	// here ever makes a secret.
	// nosemgrep: go.lang.security.audit.crypto.math_random.math-random-used
	"math/rand/v2"
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// The blocks that fill a page, split out of html.go on 2026-10-14 with the
// settings. Nothing here changed in that move - these are the bytes every page
// has been made of since 2026-08-01, and the golden hashes hold them there.

// blocks builds the body. A natural record is one complete block element and
// the closing record is a paragraph stretched to land the byte count.
//
// Whole blocks only, for the same reason Markdown writes whole blocks: a table
// or a list cut in the middle still renders, and it says something other than
// it meant.
type blocks struct {
	// tail is what the closing record ends with, which is the only thing the
	// shape of the file changes down here.
	tail string
	// m is how the blocks are spelled. The words drawn are the same in every
	// spelling, so one seed fills a page of one line and a page of many with
	// the same blocks.
	m *markup
}

// Shortest is the smallest closing record: an empty paragraph plus whatever
// this shape closes after it.
func (b blocks) Shortest() int64 { return int64(len(b.m[paraOpen]) + len(b.tail)) }

func (b blocks) Append(dst []byte, rng *rand.Rand) []byte {
	m := b.m
	switch rng.IntN(5) {
	case 0:
		dst = append(dst, m[h2Open]...)
		dst = appendPhrase(dst, rng, 2+rng.IntN(3))
		return append(dst, m[h2Close]...)
	case 1:
		dst = append(dst, m[ulOpen]...)
		for i, n := 0, 2+rng.IntN(3); i < n; i++ {
			dst = append(dst, m[liOpen]...)
			dst = appendPhrase(dst, rng, 3+rng.IntN(4))
			dst = append(dst, m[liClose]...)
		}
		return append(dst, m[ulClose]...)
	case 2:
		dst = append(dst, m[tableOpen]...)
		for i, n := 0, 2+rng.IntN(3); i < n; i++ {
			dst = append(dst, m[rowOpen]...)
			dst = strconv.AppendInt(dst, int64(i+1), 10)
			dst = append(dst, m[cellSep]...)
			dst = append(dst, words[rng.IntN(len(words))]...)
			dst = append(dst, m[cellSep]...)
			dst = append(dst, words[rng.IntN(len(words))]...)
			dst = append(dst, m[rowClose]...)
		}
		return append(dst, m[tableClose]...)
	case 3:
		// A real page carries entities, and an ampersand left raw is the
		// classic way a document stops being well formed. The fixture carries
		// one on purpose.
		dst = append(dst, m[quoteOpen]...)
		dst = appendPhrase(dst, rng, 4+rng.IntN(4))
		dst = append(dst, " &amp; "...)
		dst = appendPhrase(dst, rng, 2)
		return append(dst, m[quoteClose]...)
	default:
		dst = append(dst, m[paraOpen]...)
		dst = appendPhrase(dst, rng, 12+rng.IntN(12))
		return append(dst, m[paraClose]...)
	}
}

// Discard has nothing to put back. A block carries no state from one to the
// next, so throwing one away leaves no trace to undo.
func (blocks) Discard() {}

func (b blocks) AppendExact(dst []byte, rng *rand.Rand, n int64) []byte {
	start := len(dst)
	dst = append(dst, b.m[paraOpen]...)
	used := int64(len(dst)-start) + int64(len(b.tail))
	dst = appendFiller(dst, n-used)
	return append(dst, b.tail...)
}

func appendPhrase(dst []byte, rng *rand.Rand, n int) []byte {
	for i := 0; i < n; i++ {
		if i > 0 {
			dst = append(dst, ' ')
		}
		dst = append(dst, words[rng.IntN(len(words))]...)
	}
	return dst
}

// appendFiller writes exactly n bytes of paragraph text out of readable words.
//
// It never emits an ampersand or an angle bracket, the two characters that
// would have to be escaped - an escape would make the text longer than the
// count asked for.
// appendFiller stretches the closing paragraph to the byte.
func appendFiller(dst []byte, n int64) []byte {
	return core.AppendFiller(dst, words, n, nil)
}

// words is the filler vocabulary. English by default, like the rest of the text
// group.
var words = []string{
	"account", "anchor", "banner", "browser", "caption", "content", "control",
	"default", "element", "feature", "footer", "gallery", "handler", "header",
	"heading", "inline", "layout", "listing", "margin", "marker", "message",
	"module", "navigation", "notice", "option", "padding", "preview", "profile",
	"section", "selector", "sidebar", "summary", "template", "toolbar",
	"tooltip", "viewport", "widget", "wrapper",
}
