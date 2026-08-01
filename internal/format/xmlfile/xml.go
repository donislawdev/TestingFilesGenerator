// Package xmlfile generates XML documents.
//
// The package is not called "xml" so that it cannot be confused with the
// standard library package of that name at a glance, the same reason logfile is
// not called log. The format id is "xml".
package xmlfile

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// Measured on 2026-08-01, a comment holds arbitrary bytes to 1 MiB both in the
// epilogue and inside the root element. That says where the format tolerates
// arbitrary bytes. It does not say where the filling should go, and those are
// different questions.
//
// A comment is the wrong answer here. A five megabyte document built as a small
// tree plus a five megabyte comment is the right size and useless as a fixture,
// because a parser discards comments and would be tested on almost nothing. The
// filling goes through whole elements instead, and only the remainder to the
// exact byte lands in the text of the last one, where it stays smaller than a
// single record.
//
// The comment channel is still used, for the label - which is what the format
// document assigns it.
//
// Same shape as LOG, CSV and JSON: padding goes where the format has room for a
// long value, never into a truncated record.

const (
	generatorVersion = "1"

	declaration = `<?xml version="1.0" encoding="UTF-8"?>` + "\n"
	rootOpen    = "<records>\n"
	rootClose   = "</records>\n"

	emailDomain = "@example.com"
	createdDate = "2026-08-01"

	amountWidth = 9 // six digits, a dot, two more

	// maxIDDigits bounds the width of the record number. A record is at least
	// one byte, so a document can never hold more records than it has bytes,
	// and a size is an int64 - nineteen digits covers every file this tool can
	// be asked for.
	maxIDDigits = 19

	// The literal parts of a record, named so the arithmetic below is a
	// constant expression rather than a number somebody has to keep in step.
	openRecord = `<record id="`
	openName   = `"><name>`
	openMail   = `</name><email>`
	openAmount = `</email><amount>`
	openCreate = `</amount><created>`
	openVendor = `</created><vendor>`
	openNote   = `</vendor><note>`
	closeRec   = "</note></record>\n"

	// A record either has another one after it or closes the root element.
	tailMore = closeRec
	tailLast = closeRec + rootClose

	// fixedWidth is every literal byte of a closing record - everything except
	// the record number, the name, the vendor and the note.
	fixedWidth = len(openRecord) + len(openName) + len(openMail) + len(emailDomain) +
		len(openAmount) + amountWidth + len(openCreate) + len(createdDate) +
		len(openVendor) + len(openNote) + len(tailLast)
)

func init() {
	format.Register(format.Descriptor{
		ID:          "xml",
		Extension:   ".xml",
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,

		// A root element with nothing in it is legal XML, and it is not
		// something anybody orders by naming a byte count - that is a shape
		// request, and it arrives with the element count property. The minimum
		// here is the declaration, the root and one whole record.
		MinBytes: minimumBytes(),

		Padding: format.PaddingChannel{
			Name:     "the note element of the last record",
			Where:    format.PlacementEnd,
			Capacity: 0,
		},

		// The label rides in a comment, out of the way of the content, which is
		// what the format document assigns XML.
		Label:  format.LabelInternal,
		Oracle: "python-xml",
		// Depth, element counts, namespaces, CDATA and an internal DTD come
		// later. Declaring none now makes a recipe asking for them fail loudly.
		Properties:       nil,
		GeneratorVersion: generatorVersion,
		Generator:        generator{},
	})
}

type generator struct{}

type memo struct {
	comment string // includes the trailing newline, empty when absent
	seed    uint64
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	min := minimumBytes()
	if r.Bytes < min {
		return format.Plan{}, &format.BelowMinimumError{
			Format:    "XML",
			Requested: r.Bytes,
			Minimum:   min,
			Reason:    "a document holds a declaration, a root element and whole records, and one of each needs that much",
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
			"root":        "records",
			"declaration": true,
		},
	}

	m := memo{seed: r.Seed}
	if r.Label {
		// A comment carries the label without touching the content, and it can
		// sit anywhere after the declaration. The label text uses spaced
		// hyphens and never a double one, which a comment may not contain.
		line := "<!-- " + core.Label("xml", r.Bytes, r.Seed) + " -->\n"
		// It has to leave room for a whole document beside it, or the file would
		// be a comment and an empty root.
		if int64(len(line))+minimumBytes() <= r.Bytes {
			m.comment = line
		} else {
			p.Notes = append(p.Notes, format.Note{
				Code: "label_omitted",
				Detail: fmt.Sprintf(
					"The label comment needs %d B and this file has no room for it beside a whole record. Its name and the manifest still identify it.",
					len(line)),
			})
		}
	}

	p.Properties["label_embedded"] = m.comment != ""
	p.Memo = m
	return p, nil
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(memo)
	if !ok {
		return fmt.Errorf("xml: the plan was not produced by this generator")
	}

	head := declaration + m.comment + rootOpen
	if err := core.WriteAll(w, []byte(head)); err != nil {
		return err
	}

	rng := core.NewRand(m.seed)
	return core.FillRecords(ctx, w, rng, p.Bytes-int64(len(head)), &records{})
}

// records builds the record elements. It carries the record number, so the id
// attribute counts up the way a real export does.
type records struct{ next int64 }

// Shortest is the smallest record this builder can close a document with: the
// widest record number, the longest name in both the name and the address, the
// longest vendor once escaped, and an empty note. It has to hold for every draw
// rather than for the lucky one.
func (r *records) Shortest() int64 {
	return int64(maxIDDigits + 2*longestWord + longestVendor + fixedWidth)
}

func (r *records) Append(dst []byte, rng *rand.Rand) []byte {
	return r.append(dst, rng, -1)
}

func (r *records) AppendExact(dst []byte, rng *rand.Rand, n int64) []byte {
	return r.append(dst, rng, n)
}

// append writes one record element. A want below zero means whatever length it
// comes out, any other value is the exact length the record must have,
// including the bytes that close the root element.
//
// The length of everything before the note is measured rather than worked out
// in parallel arithmetic. Arithmetic that has to agree with the bytes beside it
// is a defect waiting for the day somebody adds an element and updates one of
// the two - and here the escaping makes the two differ on purpose.
//
// It appends rather than returning a new slice because a document of any size
// is millions of records, and one allocation per record is a multiple of the
// file in garbage. The resource guard measures that.
func (r *records) append(dst []byte, rng *rand.Rand, want int64) []byte {
	r.next++
	start := len(dst)

	name := words[rng.IntN(len(words))]
	whole := 100000 + rng.IntN(899999)
	cents := rng.IntN(100)
	vendor := vendors[rng.IntN(len(vendors))]

	dst = append(dst, openRecord...)
	dst = strconv.AppendInt(dst, r.next, 10)
	dst = append(dst, openName...)
	dst = append(dst, name...)
	dst = append(dst, openMail...)
	dst = append(dst, name...)
	dst = append(dst, emailDomain...)
	dst = append(dst, openAmount...)
	dst = strconv.AppendInt(dst, int64(whole), 10)
	dst = append(dst, '.')
	if cents < 10 {
		dst = append(dst, '0')
	}
	dst = strconv.AppendInt(dst, int64(cents), 10)
	dst = append(dst, openCreate...)
	dst = append(dst, createdDate...)
	dst = append(dst, openVendor...)
	dst = appendEscaped(dst, vendor)
	dst = append(dst, openNote...)

	if want < 0 {
		dst = appendPhrase(dst, rng, 3+rng.IntN(5))
		return append(dst, tailMore...)
	}

	// Everything written so far, plus the bytes that close the record and the
	// root element.
	used := int64(len(dst)-start) + int64(len(tailLast))
	dst = appendFiller(dst, want-used)
	return append(dst, tailLast...)
}

// appendEscaped writes text as XML character data. A real export carries names
// like "Smith & Sons", and an ampersand left raw makes the document not well
// formed - so the fixture carries one on purpose and escapes it properly.
func appendEscaped(dst []byte, s string) []byte {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&':
			dst = append(dst, "&amp;"...)
		case '<':
			dst = append(dst, "&lt;"...)
		case '>':
			dst = append(dst, "&gt;"...)
		default:
			dst = append(dst, s[i])
		}
	}
	return dst
}

// appendPhrase writes a readable note out of words and single spaces.
func appendPhrase(dst []byte, rng *rand.Rand, n int) []byte {
	for i := 0; i < n; i++ {
		if i > 0 {
			dst = append(dst, ' ')
		}
		dst = append(dst, words[rng.IntN(len(words))]...)
	}
	return dst
}

// appendFiller writes exactly n bytes of note out of readable words.
//
// It never emits an ampersand or an angle bracket - the characters that would
// have to be escaped, and an escape would make the text longer than the count
// asked for.
func appendFiller(dst []byte, n int64) []byte {
	if n <= 0 {
		// An empty element is legal, and it is what the smallest files get.
		// Anything below zero would mean the minimum was ignored, and
		// FillRecords turns that into an error rather than a wrong size.
		return dst
	}
	start := len(dst)
	for i := 0; int64(len(dst)-start) < n; i++ {
		if len(dst) > start {
			dst = append(dst, ' ')
		}
		dst = append(dst, words[i%len(words)]...)
	}
	return dst[:start+int(n)]
}

// minimumBytes is the declaration, the root element and one whole record,
// computed rather than written down so it cannot drift away from the template
// the way a number in a document would.
func minimumBytes() int64 {
	var r records
	return int64(len(declaration)+len(rootOpen)) + r.Shortest()
}

// longestWord and longestVendor are the widest draws, because the minimum has
// to hold for every draw rather than for the lucky one. The vendor is measured
// after escaping, which is what actually reaches the file.
var (
	longestWord = func() int {
		longest := 0
		for _, w := range words {
			if len(w) > longest {
				longest = len(w)
			}
		}
		return longest
	}()

	longestVendor = func() int {
		longest := 0
		for _, v := range vendors {
			n := len(appendEscaped(nil, v))
			if n > longest {
				longest = n
			}
		}
		return longest
	}()
)

// vendors carry an ampersand on purpose, so every document exercises entity
// escaping rather than only plain text.
var vendors = []string{
	"Baker & Sons", "Northwind Traders", "Clarke & Reid", "Riverside Supply",
	"Hale & Company", "Meridian Logistics", "Foster & Wren", "Union Freight",
}

// words is the vocabulary for names and notes. English by default, like the
// rest of the text group.
var words = []string{
	"account", "amount", "balance", "branch", "broker", "budget", "buyer",
	"carrier", "charge", "client", "column", "contact", "contract", "credit",
	"customer", "delivery", "deposit", "discount", "dispatch", "district",
	"invoice", "ledger", "manager", "market", "member", "monthly", "order",
	"partner", "payment", "pending", "product", "profile", "project", "quarter",
	"receipt", "record", "refund", "region", "report", "reseller", "revenue",
	"sample", "seller", "service", "shipment", "status", "storage", "summary",
	"supplier", "support", "tariff", "ticket", "transfer", "vendor", "voucher",
	"warehouse", "weekly", "wholesale",
}
