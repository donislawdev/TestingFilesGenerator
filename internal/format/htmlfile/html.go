// Package htmlfile generates HTML documents.
//
// The package is not called "html" so that it cannot be confused with the
// standard library package of that name at a glance, the same reason logfile is
// not called log. The format id is "html".
package htmlfile

import (
	"context"
	"fmt"
	"io"
	// D11 promises the same bytes from the same seed, so a deliberate,
	// reproducible generator is the product rather than a weakness. Nothing
	// here ever makes a secret.
	// nosemgrep: go.lang.security.audit.crypto.math_random.math-random-used
	"math/rand/v2"
	"strconv"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// Measured on 2026-08-01, a comment holds arbitrary bytes to 1 MiB both after
// the closing tag and inside the body. That says where the format tolerates
// arbitrary bytes, and the filling still does not go there - a five megabyte
// page that is one comment renders as an empty page.
//
// Whole blocks instead, and the remainder to the exact byte in the text of a
// closing paragraph. Fifth format built this way.
//
// HTML is the weakest case in the group for checking. There is one tolerant
// reader on this machine and it is lenient by design - the format itself says
// a parser must recover from almost anything, so "it parsed" says very little.
// The guards written against this format carry more of the weight than they do
// for SVG, which has a renderer behind it.

const (
	generatorVersion = "1"

	prologue = `<!DOCTYPE html>` + "\n" +
		`<html lang="en">` + "\n" +
		`<head>` + "\n" +
		`<meta charset="utf-8">` + "\n" +
		`<title></title>` + "\n" +
		`</head>` + "\n" +
		`<body>` + "\n"

	// The title is filled in per file when a label is wanted, so the empty pair
	// above is what the smallest document carries.
	emptyTitle = `<title></title>`

	paraOpen  = "<p>"
	paraClose = "</p>\n"
	bodyClose = "</body>\n</html>\n"

	tailLast = paraClose + bodyClose

	// fixedWidth is every literal byte of the closing record.
	fixedWidth = len(paraOpen) + len(tailLast)
)

func init() {
	format.Register(format.Descriptor{
		ID:          "html",
		Extension:   ".html",
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,

		// A document with an empty body is legal HTML and renders as a blank
		// page. That is a shape request rather than a byte count. The minimum
		// here is the skeleton and one whole block.
		MinBytes: minimumBytes(),

		Padding: format.PaddingChannel{
			Name:     "the text of the closing paragraph",
			Where:    format.PlacementEnd,
			Capacity: 0,
		},

		// A page shows what it is, so the label is a visible heading rather
		// than a comment.
		Label:  format.LabelVisible,
		Oracle: "python-html",
		// Fragment mode, element counts, inline CSS and JS, images, forms and
		// the "every HTML5 tag" variant come later. Declaring none now makes a
		// recipe asking for them fail loudly.
		Properties:       nil,
		GeneratorVersion: generatorVersion,
		Generator:        generator{},
	})
}

type generator struct{}

type memo struct {
	head      string // the whole skeleton down to <body>, title included
	labelLine string // the visible heading, empty when absent
	seed      uint64
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	min := minimumBytes()
	if r.Bytes < min {
		return format.Plan{}, &format.BelowMinimumError{
			Format:    "HTML",
			Requested: r.Bytes,
			Minimum:   min,
			Reason:    "a page holds a head, a body and whole blocks, and one of each needs that much",
			Hint:      fmt.Sprintf("Ask for %d B or more.", min),
		}
	}

	p := format.Plan{
		Bytes:       r.Bytes,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties: map[string]any{
			"encoding":    "utf-8",
			"line_ending": "lf",
			"doctype":     "html",
			"language":    "en",
		},
	}

	m := memo{seed: r.Seed, head: prologue}
	if r.Label {
		// The title and the heading say the same thing, which is what a page
		// does - one for the tab and one for the reader.
		label := core.Label("html", r.Bytes, r.Seed)
		withTitle := strings.Replace(prologue, emptyTitle, "<title>"+label+"</title>", 1)
		heading := "<h1>" + label + "</h1>\n"
		if int64(len(withTitle)-len(prologue)+len(heading))+minimumBytes() <= r.Bytes {
			m.head = withTitle
			m.labelLine = heading
		} else {
			p.Notes = append(p.Notes, format.Note{
				Code: "label_omitted",
				Detail: fmt.Sprintf(
					"The label needs %d B and this file has no room for it beside a whole block. Its name and the manifest still identify it.",
					len(heading)),
			})
		}
	}

	p.Properties[format.PropertyLabelEmbedded] = m.labelLine != ""
	p.Memo = m
	return p, nil
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(memo)
	if !ok {
		return fmt.Errorf("html: the plan was not produced by this generator")
	}

	head := m.head + m.labelLine
	if err := core.WriteAll(w, []byte(head)); err != nil {
		return err
	}

	rng := core.NewRand(m.seed)
	return core.FillRecords(ctx, w, rng, p.Bytes-int64(len(head)), blocks{})
}

// blocks builds the body. A natural record is one complete block element and
// the closing record is a paragraph stretched to land the byte count.
//
// Whole blocks only, for the same reason Markdown writes whole blocks: a table
// or a list cut in the middle still renders, and it says something other than
// it meant.
type blocks struct{}

// Shortest is the smallest closing record: an empty paragraph plus the bytes
// that close the body and the document.
func (blocks) Shortest() int64 { return int64(fixedWidth) }

func (blocks) Append(dst []byte, rng *rand.Rand) []byte {
	switch rng.IntN(5) {
	case 0:
		dst = append(dst, "<h2>"...)
		dst = appendPhrase(dst, rng, 2+rng.IntN(3))
		return append(dst, "</h2>\n"...)
	case 1:
		dst = append(dst, "<ul>\n"...)
		for i, n := 0, 2+rng.IntN(3); i < n; i++ {
			dst = append(dst, "<li>"...)
			dst = appendPhrase(dst, rng, 3+rng.IntN(4))
			dst = append(dst, "</li>\n"...)
		}
		return append(dst, "</ul>\n"...)
	case 2:
		dst = append(dst, "<table>\n<tr><th>id</th><th>field</th><th>value</th></tr>\n"...)
		for i, n := 0, 2+rng.IntN(3); i < n; i++ {
			dst = append(dst, "<tr><td>"...)
			dst = strconv.AppendInt(dst, int64(i+1), 10)
			dst = append(dst, "</td><td>"...)
			dst = append(dst, words[rng.IntN(len(words))]...)
			dst = append(dst, "</td><td>"...)
			dst = append(dst, words[rng.IntN(len(words))]...)
			dst = append(dst, "</td></tr>\n"...)
		}
		return append(dst, "</table>\n"...)
	case 3:
		// A real page carries entities, and an ampersand left raw is the
		// classic way a document stops being well formed. The fixture carries
		// one on purpose.
		dst = append(dst, "<blockquote><p>"...)
		dst = appendPhrase(dst, rng, 4+rng.IntN(4))
		dst = append(dst, " &amp; "...)
		dst = appendPhrase(dst, rng, 2)
		return append(dst, "</p></blockquote>\n"...)
	default:
		dst = append(dst, paraOpen...)
		dst = appendPhrase(dst, rng, 12+rng.IntN(12))
		return append(dst, paraClose...)
	}
}

// Discard has nothing to put back. A block carries no state from one to the
// next, so throwing one away leaves no trace to undo.
func (blocks) Discard() {}

func (blocks) AppendExact(dst []byte, rng *rand.Rand, n int64) []byte {
	start := len(dst)
	dst = append(dst, paraOpen...)
	used := int64(len(dst)-start) + int64(len(tailLast))
	dst = appendFiller(dst, n-used)
	return append(dst, tailLast...)
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

// minimumBytes is the skeleton and one whole block, computed rather than
// written down so it cannot drift away from the template.
func minimumBytes() int64 {
	var b blocks
	return int64(len(prologue)) + b.Shortest()
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
