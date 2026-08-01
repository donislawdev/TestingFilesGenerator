// Package jsonfile generates JSON documents.
//
// The package is not called "json" so that it cannot be confused with the
// standard library package of that name at a glance, the same reason logfile is
// not called log. The format id is "json".
package jsonfile

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// Measured on 2026-08-01, three padding channels all hold to 1 MiB: whitespace
// after the closing token, whitespace between tokens, and text inside a string
// value. That says where the format tolerates arbitrary bytes. It does not say
// where the filling should go, and those are different questions.
//
// Whitespace is the wrong answer here. A five megabyte document built as two
// kilobytes of structure and five megabytes of spaces is the right size and
// useless as a fixture - a parser skips whitespace, so it would be tested on
// almost nothing. The filling goes through whole records instead, and only the
// remainder to the exact byte lands in the text of the last one, where it stays
// smaller than a single record.
//
// Same shape as LOG and CSV: padding goes where the format has room for a long
// value, never into a truncated record.

const (
	generatorVersion = "1"

	// The document is an array with one record per line. Minified on one line
	// and indented forms come later as a property.
	prologue = "[\n"

	emailDomain = "@example.com"

	// Fixed widths, so the parts that are not the note stay predictable.
	amountWidth = 9 // six digits, a dot, two more
	zipWidth    = 5

	// maxIDDigits bounds the width of the record number. A record is at least
	// one byte, so a document can never hold more records than it has bytes,
	// and a size is an int64 - nineteen digits covers every file this tool can
	// be asked for.
	maxIDDigits = 19

	// The literal parts of a record, named so the arithmetic below is a
	// constant expression rather than a number somebody has to keep in step.
	openID   = `{"id":`
	openName = `,"name":"`
	openMail = `","email":"`
	openAmt  = `","amount":`
	openAct  = `,"active":`
	nullPart = `,"retired":null`
	openTags = `,"tags":["`
	tagSep   = `","`
	openAddr = `"],"address":{"city":"`
	openZip  = `","zip":"`
	openNote = `"},"note":"`
	closeRec = `"}`

	// A record either has another one after it or closes the array.
	tailMore = closeRec + ",\n"
	tailLast = closeRec + "\n]\n"

	// widestBool is "false", the longer of the two.
	widestBool = 5

	// fixedWidth is every literal byte of a closing record - everything except
	// the record number, the name, the two tags, the city and the note.
	fixedWidth = len(openID) + len(openName) + len(openMail) + len(emailDomain) +
		len(openAmt) + amountWidth + len(openAct) + widestBool + len(nullPart) +
		len(openTags) + len(tagSep) + len(openAddr) + len(openZip) + zipWidth +
		len(openNote) + len(tailLast)
)

func init() {
	format.Register(format.Descriptor{
		ID:          "json",
		Extension:   ".json",
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,

		// An empty array is legal JSON, and it is not something anybody orders
		// by naming a byte count - that is a shape request, and it arrives with
		// the record count property. The minimum here is the array and one whole
		// record.
		MinBytes: minimumBytes(),

		Padding: format.PaddingChannel{
			Name:     "the note value of the last record",
			Where:    format.PlacementEnd,
			Capacity: 0,
		},

		// The label never reaches the content. An extra field changes the very
		// structure under test. The file name and the manifest carry it instead.
		Label:  format.LabelExternalOnly,
		Oracle: "node-json",
		// Nesting depth, key counts, value types, indentation and NDJSON come
		// later. Declaring none now makes a recipe asking for them fail loudly.
		Properties:       nil,
		GeneratorVersion: generatorVersion,
		Generator:        generator{},
	})
}

type generator struct{}

type memo struct{ seed uint64 }

func (generator) Plan(r format.Request) (format.Plan, error) {
	min := minimumBytes()
	if r.Bytes < min {
		return format.Plan{}, &format.BelowMinimumError{
			Format:    "JSON",
			Requested: r.Bytes,
			Minimum:   min,
			Reason:    "a document holds whole records and one record with every value type needs that much",
			Hint:      fmt.Sprintf("Ask for %d B or more.", min),
		}
	}

	return format.Plan{
		Bytes:       r.Bytes,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties: map[string]any{
			"encoding":   "utf-8",
			"formatting": "record-per-line",
			"root":       "array",
			"depth":      3,
			// Stated even though it is always false here, so a test can assert
			// on it without knowing which formats carry a label internally.
			"label_embedded": false,
		},
		Memo: memo{seed: r.Seed},
	}, nil
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(memo)
	if !ok {
		return fmt.Errorf("json: the plan was not produced by this generator")
	}

	if err := core.WriteAll(w, []byte(prologue)); err != nil {
		return err
	}

	rng := core.NewRand(m.seed)
	return core.FillRecords(ctx, w, rng, p.Bytes-int64(len(prologue)), &records{})
}

// records builds the objects inside the array. It carries the record number, so
// the id counts up the way a real export does.
type records struct{ next int64 }

// Shortest is the smallest record this builder can close a document with: the
// widest record number, the longest word in all five places a word appears, the
// longer of the two booleans, and an empty note. It has to hold for every draw
// rather than for the lucky one.
func (r *records) Shortest() int64 {
	return int64(maxIDDigits + 5*longestWord + fixedWidth)
}

func (r *records) Append(dst []byte, rng *rand.Rand) []byte {
	return r.append(dst, rng, -1)
}

func (r *records) AppendExact(dst []byte, rng *rand.Rand, n int64) []byte {
	return r.append(dst, rng, n)
}

// append writes one record. A want below zero means whatever length it comes
// out, any other value is the exact length the record must have, including the
// bytes that close the array.
//
// The length of everything before the note is measured rather than worked out
// in parallel arithmetic. Arithmetic that has to agree with the bytes beside it
// is a defect waiting for the day somebody adds a field and updates one of the
// two.
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
	active := rng.IntN(2) == 0
	tagA := words[rng.IntN(len(words))]
	tagB := words[rng.IntN(len(words))]
	city := words[rng.IntN(len(words))]
	zip := 10000 + rng.IntN(90000)

	dst = append(dst, openID...)
	dst = strconv.AppendInt(dst, r.next, 10)
	dst = append(dst, openName...)
	dst = append(dst, name...)
	dst = append(dst, openMail...)
	dst = append(dst, name...)
	dst = append(dst, emailDomain...)
	dst = append(dst, openAmt...)
	dst = strconv.AppendInt(dst, int64(whole), 10)
	dst = append(dst, '.')
	if cents < 10 {
		dst = append(dst, '0')
	}
	dst = strconv.AppendInt(dst, int64(cents), 10)
	dst = append(dst, openAct...)
	if active {
		dst = append(dst, "true"...)
	} else {
		dst = append(dst, "false"...)
	}
	dst = append(dst, nullPart...)
	dst = append(dst, openTags...)
	dst = append(dst, tagA...)
	dst = append(dst, tagSep...)
	dst = append(dst, tagB...)
	dst = append(dst, openAddr...)
	dst = append(dst, city...)
	dst = append(dst, openZip...)
	dst = strconv.AppendInt(dst, int64(zip), 10)
	dst = append(dst, openNote...)

	if want < 0 {
		dst = appendPhrase(dst, rng, 3+rng.IntN(5))
		return append(dst, tailMore...)
	}

	// Everything written so far, plus the bytes that close the record and the
	// array.
	used := int64(len(dst)-start) + int64(len(tailLast))
	dst = appendFiller(dst, want-used)
	return append(dst, tailLast...)
}

// appendPhrase writes a readable note. Words and single spaces only - a JSON
// string would otherwise need escaping, and an escape costs a byte the size
// arithmetic did not budget for.
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
// It never emits a quote, a backslash or a control character - the characters a
// JSON string has to escape, and an escape would make the value longer than the
// count asked for.
func appendFiller(dst []byte, n int64) []byte {
	if n <= 0 {
		// An empty note is a legal string, and it is what the smallest files
		// get. Anything below zero would mean the minimum was ignored, and
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

// minimumBytes is the opening bracket and one whole record, computed rather
// than written down so it cannot drift away from the template the way a number
// in a document would.
func minimumBytes() int64 {
	var r records
	return int64(len(prologue)) + r.Shortest()
}

// longestWord is the widest draw, because the minimum has to hold for every
// draw rather than for the lucky one.
var longestWord = func() int {
	longest := 0
	for _, w := range words {
		if len(w) > longest {
			longest = len(w)
		}
	}
	return longest
}()

// words is the vocabulary for names, tags, cities and notes. English by
// default, like the rest of the text group.
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
