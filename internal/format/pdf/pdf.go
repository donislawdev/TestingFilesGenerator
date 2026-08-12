// Package pdf generates PDF documents.
//
// The highest fidelity bar in Tier 1 - the file has to open in Adobe, not
// merely satisfy a parser.
package pdf

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

const (
	generatorVersion = "1"

	// padChunkSize is how much padding is built before each write, and how
	// often cancellation is noticed.
	padChunkSize = 32 * 1024

	// A comment is at least a per cent sign and a newline, so one byte of
	// padding is the single amount that cannot be produced.
	minComment = 2

	defaultPages = 1
	maxPages     = 5000
)

// The padding channel is a comment block placed after the trailer and before
// startxref.
//
// The obvious place, and the one this project's own notes assumed, is between
// startxref and %%EOF. Measured, that is wrong twice over.
//
// A reader finds the cross reference table by scanning backwards from the end
// of the file for the startxref keyword, and how far back it looks is up to
// the reader. Xpdf 4.06 reads a comment of 1004 bytes there and fails at
// 1005. The Windows renderer reads any size. So that placement produces files
// that open for one tester and not for another, which is worse than a
// placement that fails for everybody.
//
// After the trailer nothing downstream refers to a position: the objects and
// the cross reference table both sit earlier, and startxref still points at
// the same offset. Measured at 64 bytes, 4 KB, 100 KB, 1 MB, 2 MB and 10 MB
// against Xpdf, exiftool and the Windows renderer - all read the document and
// report one page.

func init() {
	format.Register(format.Descriptor{
		ID:          "pdf",
		Extension:   ".pdf",
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,
		MinBytes:    minimumBytes(),

		Padding: format.PaddingChannel{
			Name:     "comment block after the trailer",
			Where:    format.PlacementEnd,
			Capacity: 0,
		},
		Label:  format.LabelVisible,
		Oracle: "pdftotext",
		Properties: []format.Property{
			{
				Name: "pages", Kind: format.PropertyInt,
				Min: 1, Max: maxPages,
				Default: strconv.Itoa(defaultPages),
				Detail:  "How many pages the document has.",
			},
			{
				Name: "page_size", Kind: format.PropertyChoice,
				// Written out rather than read from the map, so one build
				// cannot offer a different set from the next. The ORDER is no
				// longer decided here: registration sorts every closed set, so
				// the menu in the window, "tfg formats pdf" and the wording of
				// a refusal all list them the same way round.
				Choices: []string{"a4", "a3", "a5", "letter", "legal"},
				Default: "a4",
				Detail:  "The paper size every page uses.",
			},
		},
		GeneratorVersion: generatorVersion,
		Generator:        generator{},
	})
}

type generator struct{}

type memo struct {
	pages    int
	pageSize pageSize
	seed     uint64
	label    string
	// prefix is everything up to and including the trailer. Small - a few
	// kilobytes per page - so holding it costs nothing next to the padding.
	prefix []byte
	// suffix is startxref, the offset and the closing marker. Its length does
	// not depend on the padding, because the offset it carries points at the
	// cross reference table, which sits before the padding.
	suffix []byte
	padLen int64
}

type pageSize struct {
	name          string
	width, height int
}

var pageSizes = map[string]pageSize{
	"a4":     {"A4", 595, 842},
	"a3":     {"A3", 842, 1191},
	"a5":     {"A5", 420, 595},
	"letter": {"Letter", 612, 792},
	"legal":  {"Legal", 612, 1008},
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	pages, err := pageCount(r.Properties)
	if err != nil {
		return format.Plan{}, err
	}
	size, err := paperSize(r.Properties)
	if err != nil {
		return format.Plan{}, err
	}

	label := ""
	if r.Label {
		label = core.Label("pdf", r.Bytes, r.Seed)
	}

	m := memo{pages: pages, pageSize: size, seed: r.Seed, label: label}
	m.prefix, m.suffix = document(m)

	// One number answers both questions: how much padding this file needs, and
	// what is too small to make. Every line of body text is the same width
	// whatever the seed drew, so the smallest document this format can produce
	// is the same for everybody - see lineWidth.
	bare := int64(len(m.prefix) + len(m.suffix))
	floor := bare

	p := format.Plan{
		Bytes:       r.Bytes,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties: map[string]any{
			"pages":           pages,
			"page_size":       size.name,
			"pdf_version":     "1.7",
			"fonts_embedded":  false,
			"label_embedded":  r.Label,
			"compressed":      false,
			"content_streams": pages,
		},
	}

	switch {
	case r.Bytes == bare:
		m.padLen = 0
	case r.Bytes < floor:
		return format.Plan{}, &format.BelowMinimumError{
			Format:    "PDF",
			Requested: r.Bytes,
			Minimum:   floor,
			Reason: fmt.Sprintf("a %d page %s document%s already needs that much before any padding",
				pages, size.name, labelCost(r.Label)),
			Hint: fmt.Sprintf("Ask for %d B or more%s.", floor, cleanHint(r.Label)),
		}
	case r.Bytes < bare+minComment:
		// A comment is a per cent sign and a newline at the very least, so
		// exactly one byte above the bare document cannot be produced.
		return format.Plan{}, &format.BelowMinimumError{
			Format:    "PDF",
			Requested: r.Bytes,
			Minimum:   bare + minComment,
			Reason: fmt.Sprintf(
				"this document is exactly %d B and the shortest comment that could pad it is %d B, so one byte more than the document is the single size in between that cannot be reached",
				bare, minComment),
			Hint: fmt.Sprintf("Ask for exactly %d B or for %d B or more.", bare, bare+minComment),
		}
	default:
		m.padLen = r.Bytes - bare
	}

	p.Memo = m
	return p, nil
}

// labelCost and cleanHint keep the message about the minimum honest. The
// figure in the registry is the smallest document with no label, so a user
// who left the label on and hits the limit needs to be told why the number
// they were shown is not the number they got.
func labelCost(label bool) string {
	if label {
		return " carrying the self describing label"
	}
	return ""
}

func cleanHint(label bool) string {
	if label {
		return ", ask for fewer pages by setting pages to 1, or drop the label"
	}
	return " or ask for fewer pages by setting pages to 1"
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(memo)
	if !ok {
		return fmt.Errorf("pdf: the plan was not produced by this generator")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if _, err := w.Write(m.prefix); err != nil {
		return err
	}
	if err := writeComment(ctx, w, m.seed, m.padLen); err != nil {
		return err
	}
	_, err := w.Write(m.suffix)
	return err
}

// writeComment emits the padding as a comment block without holding it in
// memory. Every line starts with a per cent sign, so a reader skips the lot.
func writeComment(ctx context.Context, w io.Writer, seed uint64, n int64) error {
	if n <= 0 {
		return nil
	}
	if n < minComment {
		return fmt.Errorf("pdf: %d B of padding cannot be written as a comment", n)
	}

	// Line length is fixed so the block stays readable in an editor. The last
	// line takes whatever is left.
	const lineLen = 64
	rng := core.NewRand(seed)
	buf := make([]byte, 0, padChunkSize+lineLen)

	remaining := n
	for remaining > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		buf = buf[:0]
		for len(buf) < padChunkSize && remaining > int64(len(buf)) {
			left := remaining - int64(len(buf))
			size := int64(lineLen)
			switch {
			case left <= lineLen:
				// Everything left goes on one line. The caller guaranteed
				// this is never exactly one byte.
				size = left
			case left-lineLen < minComment:
				// A full line here would leave one byte behind, and one byte
				// cannot be a comment. Shorten this line so the last one has
				// room. Found by the property test, not by reasoning.
				size = lineLen - minComment
			}
			if size < minComment {
				return fmt.Errorf("pdf: %d B left over, which cannot form a comment line", size)
			}
			buf = append(buf, '%')
			for i := int64(0); i < size-2; i++ {
				buf = append(buf, padAlphabet[rng.IntN(len(padAlphabet))])
			}
			buf = append(buf, '\n')
		}

		if _, err := w.Write(buf); err != nil {
			return err
		}
		remaining -= int64(len(buf))
	}
	return nil
}

// padAlphabet keeps the padding printable, so opening the file in an editor
// shows comment lines rather than binary noise.
var padAlphabet = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

// document builds everything up to the trailer, and the closing lines.
//
// The two are returned apart because the padding goes between them. The
// closing lines carry the offset of the cross reference table, which sits in
// the first part and therefore does not move.
func document(m memo) (prefix, suffix []byte) {
	var body bytes.Buffer
	body.WriteString("%PDF-1.7\n")
	// A comment of high bytes tells any tool handling the file that it is
	// binary, which stops a transfer from mangling the line endings.
	body.Write([]byte{'%', 0xe2, 0xe3, 0xcf, 0xd3, '\n'})

	var objects []string

	kids := make([]string, 0, m.pages)
	for i := 0; i < m.pages; i++ {
		// Objects: 1 catalog, 2 pages, 3 font, 4 info, then per page a page
		// object and a content stream.
		kids = append(kids, fmt.Sprintf("%d 0 R", 5+i*2))
	}

	objects = append(objects, "<</Type/Catalog/Pages 2 0 R>>")
	objects = append(objects, fmt.Sprintf("<</Type/Pages/Kids[%s]/Count %d>>",
		strings.Join(kids, " "), m.pages))
	objects = append(objects, "<</Type/Font/Subtype/Type1/BaseFont/Helvetica/Encoding/WinAnsiEncoding>>")
	objects = append(objects, infoObject(m))

	for i := 0; i < m.pages; i++ {
		content := pageContent(m, i)
		objects = append(objects, fmt.Sprintf(
			"<</Type/Page/Parent 2 0 R/MediaBox[0 0 %d %d]/Contents %d 0 R/Resources<</Font<</F1 3 0 R>>>>>>",
			m.pageSize.width, m.pageSize.height, 6+i*2))
		objects = append(objects, fmt.Sprintf("<</Length %d>>\nstream\n%sendstream", len(content), content))
	}

	offsets := make([]int, 0, len(objects))
	for i, obj := range objects {
		offsets = append(offsets, body.Len())
		fmt.Fprintf(&body, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}

	xrefPos := body.Len()
	fmt.Fprintf(&body, "xref\n0 %d\n", len(objects)+1)
	body.WriteString("0000000000 65535 f \n")
	for _, off := range offsets {
		fmt.Fprintf(&body, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&body, "trailer\n<</Size %d/Root 1 0 R/Info 4 0 R>>\n", len(objects)+1)

	suffix = []byte(fmt.Sprintf("startxref\n%d\n%%%%EOF\n", xrefPos))
	return body.Bytes(), suffix
}

func infoObject(m memo) string {
	title := m.label
	if title == "" {
		title = "Testing Files Generator"
	}
	// A fixed date, because a timestamp taken from the clock would make two
	// runs of the same recipe differ.
	return fmt.Sprintf("<</Title(%s)/Producer(Testing Files Generator)/CreationDate(D:20200101000000Z)>>",
		escapeString(title))
}

func pageContent(m memo, index int) string {
	var b strings.Builder
	top := m.pageSize.height - 72

	b.WriteString("BT\n/F1 18 Tf\n")
	fmt.Fprintf(&b, "72 %d Td\n(%s) Tj\n", top, escapeString(fmt.Sprintf("Page %d of %d", index+1, m.pages)))
	b.WriteString("ET\n")

	b.WriteString("BT\n/F1 11 Tf\n")
	rng := core.NewRand(core.FileSeed(m.seed, index))
	y := top - 36
	for line := 0; line < 24 && y > 108; line++ {
		fmt.Fprintf(&b, "1 0 0 1 72 %d Tm\n(%s) Tj\n", y, escapeString(bodyLine(rng)))
		y -= 16
	}
	b.WriteString("ET\n")

	// The label lives in the footer, where it does not sit on top of the
	// content a person is looking at.
	if m.label != "" {
		b.WriteString("BT\n/F1 8 Tf\n")
		fmt.Fprintf(&b, "72 48 Td\n(%s) Tj\n", escapeString(m.label))
		b.WriteString("ET\n")
	}
	return b.String()
}

// lineWidth is how many characters every line of body text comes to.
//
// Fixed, and that is the whole of the point. The words are still drawn from the
// seed, so two seeds give different text - only the length is settled, the same
// way every record based format here settles the length of its closing record.
//
// Why it has to be fixed was measured rather than argued. A line used to be
// eight to thirteen words of four to nine characters, so the smallest document
// one seed could produce was not the smallest another could:
//
//	the floor across 200 seeds   3090 B to 3499 B
//	--size 3300                  accepted for 6 seeds out of 10
//
// An error that appears and disappears when the seed changes is the one thing a
// tool built on "the same seed gives the same run" cannot have, and the engine
// says exactly that about a size drawn from a range. Nobody had applied it to
// the minimum itself.
//
// The first repair kept the ragged lines and took the theoretical worst case as
// the floor. That was honest and consistent, and it cost about 1400 B of range
// that no request could reach any more. This is the answer that costs the user
// nothing, and it pays for it by shifting every byte this format produces -
// a breaking change, taken deliberately while nothing depends on those bytes.
//
// Eighty four characters is close to what the old lines averaged, so a page
// still looks like a page. Justified rather than ragged, which if anything
// reads more like a real document than the random lengths did.
const lineWidth = 84

// bodyLine is one line of body text: drawn from the seed, always lineWidth
// characters long, and never ending in the middle of a word.
//
// core.AppendFiller does almost this and is not used, which is worth saying
// because sharing a primitive is usually the right answer here. That one cuts
// at the byte and makes the difference up with spaces, which is exactly right
// where it is used - inside a field of a CSV row or a JSON value, where a
// clipped word is padding nobody reads. This text is the visible content of a
// page. A document whose every line ends in "refere" reads as broken rather
// than as filler, and the fidelity bar for this format is that it looks like a
// document in Adobe.
//
// So: whole words while the next one still fits, then spaces. The line is the
// same length either way, which is the property the floor depends on.
func bodyLine(rng interface{ IntN(int) int }) string {
	var b strings.Builder
	b.Grow(lineWidth)

	// Started at a word the seed chose and walked in order, so two seeds read
	// differently without needing a draw per word.
	at := rng.IntN(len(words))
	for {
		w := words[at%len(words)]
		need := len(w)
		if b.Len() > 0 {
			need++ // the space in front of it
		}
		if b.Len()+need > lineWidth {
			break
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(w)
		at++
	}
	// Justified to the fixed width. Every vocabulary here is ASCII, so a byte
	// and a character are the same thing and the count is the width.
	return b.String() + strings.Repeat(" ", lineWidth-b.Len())
}

// escapeString protects the three characters that end or nest a PDF string.
// Without this a label containing a bracket would produce a file no reader
// can parse.
func escapeString(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`)
	return r.Replace(s)
}

func pageCount(props map[string]string) (int, error) {
	raw, ok := props["pages"]
	if !ok || raw == "" {
		return defaultPages, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("pdf: pages must be a whole number, got %q", raw)
	}
	if n < 1 || n > maxPages {
		return 0, fmt.Errorf("pdf: pages must be between 1 and %d, got %d", maxPages, n)
	}
	return n, nil
}

func paperSize(props map[string]string) (pageSize, error) {
	raw, ok := props["page_size"]
	if !ok || raw == "" {
		return pageSizes["a4"], nil
	}
	s, ok := pageSizes[strings.ToLower(raw)]
	if !ok {
		names := make([]string, 0, len(pageSizes))
		for k := range pageSizes {
			names = append(names, k)
		}
		return pageSize{}, fmt.Errorf("pdf: page_size %q is not one of: %s", raw, strings.Join(sorted(names), ", "))
	}
	return s, nil
}

func sorted(in []string) []string {
	out := append([]string(nil), in...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// minimumBytes is the smallest document this generator can produce: one A4
// page with no label. Measured at start up rather than guessed.
func minimumBytes() int64 {
	m := memo{pages: 1, pageSize: pageSizes["a4"]}
	prefix, suffix := document(m)
	return int64(len(prefix) + len(suffix))
}

var words = []string{
	"account", "amount", "balance", "branch", "client", "column", "contract",
	"customer", "delivery", "document", "invoice", "item", "ledger", "note",
	"order", "payment", "period", "product", "quantity", "receipt", "record",
	"reference", "region", "report", "sample", "service", "shipment",
	"statement", "summary", "supplier", "total", "transfer", "unit", "value",
}
