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

	// tailFragment is what a fragment's closing record ends with. A fragment
	// closes its last paragraph and stops, because it has no body and no
	// document to close.
	tailFragment = paraClose
)

// The shape of the file: a whole page, or only the blocks that would sit in one.
const (
	settingStructure  = "structure"
	structureDocument = "document"
	structureFragment = "fragment"
)

// structureOf reads the setting.
//
// A value outside the declared set has already been refused by the registry,
// which checks every format against its declaration in one place. This branch
// stays for the same reason the CSV dialect and the text encoding keep theirs:
// this function is callable directly, a guard is such a caller, and a generator
// that trusts its input is one registry change away from writing a file nobody
// ordered.
func structureOf(props map[string]string) (string, error) {
	v, ok := props[settingStructure]
	if !ok || v == "" {
		return structureDocument, nil
	}
	switch v {
	case structureDocument, structureFragment:
		return v, nil
	}
	return "", &format.PropertyValueError{
		Format: "html", Key: settingStructure, Value: v,
		Reason: "it has to be " + structureDocument + " or " + structureFragment,
	}
}

// prologueFor is the skeleton down to the body, empty for a fragment.
func prologueFor(shape string) string {
	if shape == structureFragment {
		return ""
	}
	return prologue
}

// blocksFor is the body builder for one shape.
//
// The shape has to reach the BUILDER and not only the prologue, and that is the
// part of this easy to miss: the bytes that close the body and the document sit
// in the last RECORD rather than in a footer. A change that swapped only the
// prologue would end a fragment with </body></html> - the right size,
// deterministic, and nonsense.
func blocksFor(shape string) blocks {
	if shape == structureFragment {
		return blocks{tail: tailFragment}
	}
	return blocks{tail: tailLast}
}

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
		// Element counts, inline CSS and JS, images, forms and the "every HTML5
		// tag" variant come later. Declaring only what is here is what makes a
		// recipe asking for them fail loudly instead of quietly producing
		// something else.
		Properties: []format.Property{{
			Name: settingStructure, Kind: format.PropertyChoice,
			Choices: []string{structureDocument, structureFragment},
			Default: structureDocument,
			Detail:  "Whether the file is a whole page or only the blocks that would sit inside one. A fragment has no doctype, no html element and no body, which is what a content field or the body of an email really holds. It is far smaller, so the smallest fragment sits well below the smallest page.",
		}},
		GeneratorVersion: generatorVersion,
		Generator:        generator{},
	})
}

type generator struct{}

type memo struct {
	head      string // the whole skeleton down to <body>, title included
	labelLine string // the visible heading, empty when absent
	seed      uint64
	shape     string
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	shape, err := structureOf(r.Properties)
	if err != nil {
		return format.Plan{}, err
	}

	min := minimumFor(shape)
	if r.Bytes < min {
		reason := "a page holds a head, a body and whole blocks, and one of each needs that much"
		if shape == structureFragment {
			reason = "a fragment holds whole blocks, and one of them needs that much"
		}
		return format.Plan{}, &format.BelowMinimumError{
			Format:    "HTML",
			Requested: r.Bytes,
			Minimum:   min,
			Reason:    reason,
			Hint:      fmt.Sprintf("Ask for %d B or more.", min),
		}
	}

	// A fragment has no doctype, and saying it has one would be the manifest
	// describing a file that is not there.
	doctype := "html"
	if shape == structureFragment {
		doctype = "none"
	}

	p := format.Plan{
		Bytes:       r.Bytes,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties: map[string]any{
			"encoding":       "utf-8",
			"line_ending":    "lf",
			"doctype":        doctype,
			"language":       "en",
			settingStructure: shape,
		},
	}

	m := memo{seed: r.Seed, head: prologueFor(shape), shape: shape}
	if r.Label {
		var note *format.Note
		m.head, m.labelLine, note = labelledHead(shape, r, min)
		if note != nil {
			p.Notes = append(p.Notes, *note)
		}
	}

	p.Properties[format.PropertyLabelEmbedded] = m.labelLine != ""
	p.Memo = m
	return p, nil
}

// labelledHead works out what a labelled file carries: the head, the visible
// heading, and a note instead of both when there is no room beside a whole
// block.
//
// Split out of Plan when that function reached the crowding threshold. The line
// is what a part does rather than how long it is: this answers "does the label
// fit and what does it cost", and Plan answers "is this request askable at all".
func labelledHead(shape string, r format.Request, min int64) (head, heading string, note *format.Note) {
	label := core.Label("html", r.Bytes, r.Seed)
	heading = "<h1>" + label + "</h1>\n"

	head, extra := prologueFor(shape), int64(0)
	if shape == structureDocument {
		// The title and the heading say the same thing, which is what a page
		// does - one for the tab and one for the reader. A fragment has no head
		// to put a title in, so it carries only the heading, and the label is
		// still visible because a heading is.
		head = strings.Replace(prologue, emptyTitle, "<title>"+label+"</title>", 1)
		extra = int64(len(head) - len(prologue))
	}
	if extra+int64(len(heading))+min <= r.Bytes {
		return head, heading, nil
	}
	return prologueFor(shape), "", &format.Note{
		Code: "label_omitted",
		Detail: fmt.Sprintf(
			"The label needs %d B and this file has no room for it beside a whole block. Its name and the manifest still identify it.",
			len(heading)),
	}
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
	return core.FillRecords(ctx, w, rng, p.Bytes-int64(len(head)), blocksFor(m.shape))
}

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
}

// Shortest is the smallest closing record: an empty paragraph plus whatever
// this shape closes after it.
func (b blocks) Shortest() int64 { return int64(len(paraOpen) + len(b.tail)) }

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

func (b blocks) AppendExact(dst []byte, rng *rand.Rand, n int64) []byte {
	start := len(dst)
	dst = append(dst, paraOpen...)
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

// minimumBytes is the smallest whole page, which is what the registry declares.
// Every shape answers for its own, the way the JSON layouts do.
func minimumBytes() int64 { return minimumFor(structureDocument) }

// minimumFor is the skeleton of one shape and one whole block, computed rather
// than written down so it cannot drift away from the template.
func minimumFor(shape string) int64 {
	return int64(len(prologueFor(shape))) + blocksFor(shape).Shortest()
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
