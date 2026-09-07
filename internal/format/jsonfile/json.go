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
	// D11 promises the same bytes from the same seed, so a deliberate,
	// reproducible generator is the product rather than a weakness. Nothing
	// here ever makes a secret.
	// nosemgrep: go.lang.security.audit.crypto.math_random.math-random-used
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

	emailDomain = "@example.com"

	// Fixed widths, so the parts that are not the note stay predictable.
	amountWidth = 9 // six digits, a dot, two more
	zipWidth    = 5

	// maxIDDigits bounds the width of the record number. A record is at least
	// one byte, so a document can never hold more records than it has bytes,
	// and a size is an int64 - nineteen digits covers every file this tool can
	// be asked for.
	maxIDDigits = 19

	// widestBool is "false", the longer of the two.
	widestBool = 5
)

// The literal parts of a record live on the style, because there are three
// layouts of them and the arithmetic has to measure whichever one is in use.
// See style.go.

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
		//
		// The DEFAULT layout's minimum, the way CSV declares its default
		// dialect's. Every other layout answers for itself, through the same
		// refusal, and SmallestAccepted asks the generator rather than reading
		// this number.
		MinBytes: minimumBytes(defaultStyle()),

		Padding: format.PaddingChannel{
			Name:     "the note value of the last record",
			Where:    format.PlacementEnd,
			Capacity: 0,
		},

		// The label never reaches the content. An extra field changes the very
		// structure under test. The file name and the manifest carry it instead.
		Label:  format.LabelExternalOnly,
		Oracle: "node-json",
		// Nesting depth, key counts, value types and NDJSON come later.
		// Declaring only what is here makes a recipe asking for them fail
		// loudly rather than quietly producing something else.
		Properties:       properties(),
		GeneratorVersion: generatorVersion,
		Generator:        generator{},
	})
}

type generator struct{}

type memo struct {
	seed uint64
	s    style
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	s, err := parseStyle(r.Properties)
	if err != nil {
		return format.Plan{}, err
	}

	min := minimumBytes(s)
	if r.Bytes < min {
		return format.Plan{}, &format.BelowMinimumError{
			Format:    "JSON",
			Requested: r.Bytes,
			Minimum:   min,
			Reason: fmt.Sprintf(
				"a document holds whole records, and one %s record with every value type needs that much",
				s.name),
			Hint: fmt.Sprintf("Ask for %d B or more.", min),
		}
	}

	return format.Plan{
		Bytes:       r.Bytes,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties: map[string]any{
			"encoding": "utf-8",
			Formatting: s.name,
			"root":     "array",
			"depth":    3,
			// Stated even though it is always false here, so a test can assert
			// on it without knowing which formats carry a label internally.
			format.PropertyLabelEmbedded: false,
		},
		Memo: memo{seed: r.Seed, s: s},
	}, nil
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(memo)
	if !ok {
		return fmt.Errorf("json: the plan was not produced by this generator")
	}

	if err := core.WriteAll(w, []byte(m.s.prologue)); err != nil {
		return err
	}

	rng := core.NewRand(m.seed)
	return core.FillRecords(ctx, w, rng, p.Bytes-int64(len(m.s.prologue)), &records{s: m.s})
}

// records builds the objects inside the array. It carries the record number, so
// the id counts up the way a real export does, and the layout the document is
// being written in.
type records struct {
	next int64
	s    style
}

// Shortest is the smallest record this builder can close a document with: the
// widest record number, the longest word in all five places a word appears, the
// longer of the two booleans, and an empty note. It has to hold for every draw
// rather than for the lucky one.
func (r *records) Shortest() int64 {
	return int64(maxIDDigits + 5*longestWord + r.s.fixed())
}

func (r *records) Append(dst []byte, rng *rand.Rand) []byte {
	return r.append(dst, rng, -1)
}

func (r *records) AppendExact(dst []byte, rng *rand.Rand, n int64) []byte {
	return r.append(dst, rng, n)
}

// Discard hands back the number the thrown away record took with it, so the
// closing record carries it instead and the ids read 1..N with nothing missing.
func (r *records) Discard() { r.next-- }

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

	dst = append(dst, r.s.openID...)
	dst = strconv.AppendInt(dst, r.next, 10)
	dst = append(dst, r.s.openName...)
	dst = append(dst, name...)
	dst = append(dst, r.s.openMail...)
	dst = append(dst, name...)
	dst = append(dst, emailDomain...)
	dst = append(dst, r.s.openAmt...)
	dst = strconv.AppendInt(dst, int64(whole), 10)
	dst = append(dst, '.')
	if cents < 10 {
		dst = append(dst, '0')
	}
	dst = strconv.AppendInt(dst, int64(cents), 10)
	dst = append(dst, r.s.openAct...)
	if active {
		dst = append(dst, "true"...)
	} else {
		dst = append(dst, "false"...)
	}
	dst = append(dst, r.s.nullPart...)
	dst = append(dst, r.s.openTags...)
	dst = append(dst, tagA...)
	dst = append(dst, r.s.tagSep...)
	dst = append(dst, tagB...)
	dst = append(dst, r.s.openAddr...)
	dst = append(dst, city...)
	dst = append(dst, r.s.openZip...)
	dst = strconv.AppendInt(dst, int64(zip), 10)
	dst = append(dst, r.s.openNote...)

	if want < 0 {
		dst = appendPhrase(dst, rng, 3+rng.IntN(5))
		return append(dst, r.s.tailMore...)
	}

	// Everything written so far, plus the bytes that close the record and the
	// array.
	used := int64(len(dst)-start) + int64(len(r.s.tailLast))
	dst = appendFiller(dst, want-used)
	return append(dst, r.s.tailLast...)
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
// appendFiller stretches the note to the byte. Plain words with one space
// between them, because a note is prose.
func appendFiller(dst []byte, n int64) []byte {
	return core.AppendFiller(dst, words, n, nil)
}

// minimumBytes is the opening bracket and one whole record, computed rather
// than written down so it cannot drift away from the template the way a number
// in a document would.
func minimumBytes(s style) int64 {
	r := records{s: s}
	return int64(len(s.prologue)) + r.Shortest()
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
