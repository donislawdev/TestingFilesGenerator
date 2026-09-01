// Package csvfile generates comma separated value files.
//
// The package is not called "csv" so that it cannot be confused with the
// standard library package of that name at a glance, the same reason logfile
// is not called log. The format id is "csv".
package csvfile

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

// The padding channel is the content, like the rest of the text group, but with
// a limit the measurement of 2026-08-01 found and the documents did not have:
// padding pushed into a single field topples the Python csv module above
// 131 072 B, which is the default of the reader a tester is most likely to
// have. So the filling goes through rows, and only the remainder to the exact
// byte lands in the last field of the last row, where it stays small.
//
// A row is never truncated either. A CSV is read a record at a time and a short
// last row is a ragged row - a defect this tool offers deliberately elsewhere
// (Chaos Lab), so producing one by accident would be indistinguishable from the
// feature. Whole rows, then a closing row built to the byte.

const (
	generatorVersion = "1"

	emailDomain = "@example.com"
	createdDate = "2026-08-01"

	// amountWidth is six digits, a dot and two more. The range the amount is
	// drawn from below guarantees it.
	amountWidth = 9

	// closingQuote ends the description. What follows it is the row ending,
	// which the dialect decides, so the two are no longer one constant.
	closingQuote = `"`

	// maxRowDigits bounds the width of the row number. A row is at least one
	// byte, so a file can never hold more rows than it has bytes, and a size is
	// an int64 - nineteen digits covers every file this tool can be asked for.
	// Bounding it rather than guessing is what lets the closing row reach its
	// length whatever row number it lands on.
	maxRowDigits = 19

	// fixedBeforeEnding is every byte of a row except the row number, the name
	// (which also forms the address), the description and the row ending. A
	// constant expression, so it cannot drift away from the template above.
	//
	// The five separators count one byte each, which is a fact about the
	// separators offered rather than an assumption: every one of them is a
	// single byte, and dialect.go says so where they are declared.
	fixedBeforeEnding = 5 /* separators */ + len(emailDomain) + amountWidth +
		len(createdDate) + 1 /* the opening quote */ + len(closingQuote)
)

// fixedWidth is fixedBeforeEnding plus the row ending, which the dialect
// decides. A CRLF row costs one byte more than an LF one, on every row, which
// is why the minimum moves with this setting.
func fixedWidth(d dialect) int64 {
	return int64(fixedBeforeEnding + len(d.eol))
}

func init() {
	format.Register(format.Descriptor{
		ID:          "csv",
		Extension:   ".csv",
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,

		// A file with a header and no rows is legal CSV, and it is not something
		// anybody orders by naming a byte count - that is a shape request, and
		// it would arrive with a row count setting, which this format does not
		// offer yet.
		//
		// The floor announced here is for the settings left alone. The dialect
		// moves the real one - a CRLF row costs a byte more and a file with no
		// header has one fewer line to pay for - so Plan works that one out and
		// names it. The log format is arranged the same way.
		MinBytes: minimumBytes(defaultDialect()),

		Padding: format.PaddingChannel{
			Name:     "the description field of the last row",
			Where:    format.PlacementEnd,
			Capacity: 0,
		},

		// The label never reaches the content. A comment row breaks half the
		// parsers and an extra column changes the very data under test. The file
		// name and the manifest carry it instead.
		Label:  format.LabelExternalOnly,
		Oracle: "python-csv",
		// Quoting, column count and column types come later. Declaring none of
		// them now makes a recipe asking for one fail loudly.
		Properties:       properties(),
		GeneratorVersion: generatorVersion,
		Generator:        generator{},
	})
}

type generator struct{}

type memo struct {
	seed uint64
	dia  dialect
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	d, err := parseDialect(r.Properties)
	if err != nil {
		return format.Plan{}, err
	}

	min := minimumBytes(d)
	if r.Bytes < min {
		return format.Plan{}, &format.BelowMinimumError{
			Format:    "CSV",
			Requested: r.Bytes,
			Minimum:   min,
			Reason:    reasonForMinimum(d),
			Hint:      fmt.Sprintf("Ask for %d B or more.", min),
		}
	}

	return format.Plan{
		Bytes:       r.Bytes,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties: map[string]any{
			"encoding": "utf-8",
			// The manifest carries the separator as the CHARACTER, where the
			// recipe names it as a word. That difference is deliberate and is
			// the same one the contract already draws between size, which is an
			// intention, and bytes, which is a fact. Changing it would break
			// every script reading this field.
			"line_ending": d.lineEndingID,
			"separator":   string(d.sep),
			"header":      d.header,
			"columns":     len(columnNames),
			// Stated even though it is always false here, so a test can assert
			// on it without knowing which formats carry a label internally.
			format.PropertyLabelEmbedded: false,
		},
		Memo: memo{seed: r.Seed, dia: d},
	}, nil
}

// reasonForMinimum says what the floor is made of, which changes with the
// dialect. A file with no header pays for rows alone, and saying "a header and
// whole rows" there would name something the file does not have.
func reasonForMinimum(d dialect) string {
	if !d.header {
		return "a table holds whole rows, and one of them needs that much"
	}
	return "a table holds a header and whole rows, and one of each needs that much"
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(memo)
	if !ok {
		return fmt.Errorf("csv: the plan was not produced by this generator")
	}

	if m.dia.header {
		if err := core.WriteAll(w, []byte(m.dia.headerLine())); err != nil {
			return err
		}
	}

	rng := core.NewRand(m.seed)
	return core.FillRecords(ctx, w, rng, p.Bytes-m.dia.headerBytes(), &rows{dia: m.dia})
}

// rows builds the data rows. It carries the row number, so the id column counts
// up the way a real export does.
type rows struct {
	next int64
	dia  dialect
}

// Shortest is the smallest row this builder can close a file with: the widest
// row number, the longest name in both the name and the address, and an empty
// description. It has to hold for every draw rather than for the lucky one.
func (r *rows) Shortest() int64 {
	return int64(maxRowDigits+2*longestWord) + fixedWidth(r.dia)
}

func (r *rows) Append(dst []byte, rng *rand.Rand) []byte {
	return r.append(dst, rng, -1)
}

func (r *rows) AppendExact(dst []byte, rng *rand.Rand, n int64) []byte {
	return r.append(dst, rng, n)
}

// Discard hands back the number the thrown away row took with it, so the closing
// row carries it instead and the column reads 1..N with nothing missing.
func (r *rows) Discard() { r.next-- }

// append writes one row. A want below zero means whatever length it comes out,
// any other value is the exact length the row must have, newline included, and
// the description is stretched to reach it.
//
// The length of everything before the description is measured rather than
// worked out in parallel arithmetic. Arithmetic that has to agree with the
// bytes beside it is a defect waiting for the day somebody adds a column and
// updates only one of the two.
//
// It appends rather than returning a new slice because a table of any size is
// millions of rows, and one allocation per row is a multiple of the file in
// garbage. The resource guard measures that.
func (r *rows) append(dst []byte, rng *rand.Rand, want int64) []byte {
	r.next++
	start := len(dst)

	name := words[rng.IntN(len(words))]
	// Six digits before the dot and two after, always.
	whole := 100000 + rng.IntN(899999)
	cents := rng.IntN(100)

	sep := r.dia.sep

	dst = strconv.AppendInt(dst, r.next, 10)
	dst = append(dst, sep)
	dst = append(dst, name...)
	dst = append(dst, sep)
	dst = append(dst, name...)
	dst = append(dst, emailDomain...)
	dst = append(dst, sep)
	dst = strconv.AppendInt(dst, int64(whole), 10)
	dst = append(dst, '.')
	if cents < 10 {
		dst = append(dst, '0')
	}
	dst = strconv.AppendInt(dst, int64(cents), 10)
	dst = append(dst, sep)
	dst = append(dst, createdDate...)
	dst = append(dst, sep, '"')

	if want < 0 {
		dst = appendPhrase(dst, rng, 3+rng.IntN(5), sep)
	} else {
		// Everything written so far, plus what still has to follow.
		used := int64(len(dst)-start) + int64(len(closingQuote)) + int64(len(r.dia.eol))
		dst = appendFiller(dst, want-used, sep)
	}

	dst = append(dst, closingQuote...)
	return append(dst, r.dia.eol...)
}

// appendPhrase writes a readable description. Every few words it drops the
// SEPARATOR, which is the case a CSV reader has to get right and the reason the
// column is quoted at all.
//
// The separator rather than always a comma, and that is the point of the
// setting rather than a detail of it. A comma inside a semicolon separated file
// needs no quoting, so a description that kept dropping commas would leave a
// semicolon file never exercising the quoted path at all - the file would be
// the right size, parse everywhere, and quietly test less than the comma one.
func appendPhrase(dst []byte, rng *rand.Rand, n int, sep byte) []byte {
	for i := 0; i < n; i++ {
		if i > 0 {
			if i%3 == 0 {
				dst = append(dst, sep)
			}
			dst = append(dst, ' ')
		}
		dst = append(dst, words[rng.IntN(len(words))]...)
	}
	return dst
}

// appendFiller writes exactly n bytes of description out of readable words, so
// a padded row still looks like a record rather than a run of one letter.
//
// It never emits a quote or a newline, the two characters that would end the
// field early.
// appendFiller stretches the description to the byte.
//
// A separator every fourth word, unlike every other format here, and on
// purpose: the description is a quoted field, so the padding is what makes a
// long file keep exercising the quoting rather than turning into plain words.
// It follows the dialect for the reason appendPhrase gives.
func appendFiller(dst []byte, n int64, sep byte) []byte {
	both := string(sep) + " "
	return core.AppendFiller(dst, words, n, func(i int) string {
		if i%4 == 0 {
			return both
		}
		return " "
	})
}

// minimumBytes is the header, when there is one, and one whole row. Computed
// rather than written down so it cannot drift away from the template the way a
// number in a document would.
//
// It takes the dialect because the floor moves with it: a CRLF row costs a byte
// more, and a file with no header has one fewer line to pay for. The registry
// announces the floor for the settings left alone, and Plan works out the real
// one for the settings that arrived - the same arrangement the log format uses,
// where the entry shape moves the floor too.
func minimumBytes(d dialect) int64 {
	r := rows{dia: d}
	return d.headerBytes() + r.Shortest()
}

// longestWord is the widest draw, because the minimum has to hold for every
// draw rather than for the lucky one. The name appears twice in a row, once on
// its own and once inside the address.
var longestWord = func() int {
	longest := 0
	for _, w := range words {
		if len(w) > longest {
			longest = len(w)
		}
	}
	return longest
}()

// words is the vocabulary for names and descriptions. English by default, like
// the rest of the text group.
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
