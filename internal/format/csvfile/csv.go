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

	header      = "id,name,email,amount,created,description\n"
	emailDomain = "@example.com"
	createdDate = "2026-08-01"

	// amountWidth is six digits, a dot and two more. The range the amount is
	// drawn from below guarantees it.
	amountWidth = 9

	// rowTail is what follows the description: the closing quote and the
	// newline.
	rowTail = `"` + "\n"

	// maxRowDigits bounds the width of the row number. A row is at least one
	// byte, so a file can never hold more rows than it has bytes, and a size is
	// an int64 - nineteen digits covers every file this tool can be asked for.
	// Bounding it rather than guessing is what lets the closing row reach its
	// length whatever row number it lands on.
	maxRowDigits = 19

	// fixedWidth is every byte of a row except the row number, the name (which
	// also forms the address) and the description. A constant expression, so it
	// cannot drift away from the template above.
	fixedWidth = 5 /* separators */ + len(emailDomain) + amountWidth +
		len(createdDate) + 1 /* the opening quote */ + len(rowTail)
)

func init() {
	format.Register(format.Descriptor{
		ID:          "csv",
		Extension:   ".csv",
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,

		// A file with a header and no rows is legal CSV, and it is not something
		// anybody orders by naming a byte count - that is a shape request, and
		// it arrives with the row count property. The minimum here is the header
		// and one whole row.
		MinBytes: minimumBytes(),

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
		// Separator, quoting, column count and column types come later.
		// Declaring none now makes a recipe asking for them fail loudly.
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
			Format:    "CSV",
			Requested: r.Bytes,
			Minimum:   min,
			Reason:    "a table holds a header and whole rows, and one of each needs that much",
			Hint:      fmt.Sprintf("Ask for %d B or more.", min),
		}
	}

	return format.Plan{
		Bytes:       r.Bytes,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties: map[string]any{
			"encoding":    "utf-8",
			"line_ending": "lf",
			"separator":   ",",
			"header":      true,
			"columns":     6,
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
		return fmt.Errorf("csv: the plan was not produced by this generator")
	}

	if err := core.WriteAll(w, []byte(header)); err != nil {
		return err
	}

	rng := core.NewRand(m.seed)
	return core.FillRecords(ctx, w, rng, p.Bytes-int64(len(header)), &rows{})
}

// rows builds the data rows. It carries the row number, so the id column counts
// up the way a real export does.
type rows struct{ next int64 }

// Shortest is the smallest row this builder can close a file with: the widest
// row number, the longest name in both the name and the address, and an empty
// description. It has to hold for every draw rather than for the lucky one.
func (r *rows) Shortest() int64 {
	return int64(maxRowDigits + 2*longestWord + fixedWidth)
}

func (r *rows) Append(dst []byte, rng *rand.Rand) []byte {
	return r.append(dst, rng, -1)
}

func (r *rows) AppendExact(dst []byte, rng *rand.Rand, n int64) []byte {
	return r.append(dst, rng, n)
}

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

	dst = strconv.AppendInt(dst, r.next, 10)
	dst = append(dst, ',')
	dst = append(dst, name...)
	dst = append(dst, ',')
	dst = append(dst, name...)
	dst = append(dst, emailDomain...)
	dst = append(dst, ',')
	dst = strconv.AppendInt(dst, int64(whole), 10)
	dst = append(dst, '.')
	if cents < 10 {
		dst = append(dst, '0')
	}
	dst = strconv.AppendInt(dst, int64(cents), 10)
	dst = append(dst, ',')
	dst = append(dst, createdDate...)
	dst = append(dst, ',', '"')

	if want < 0 {
		dst = appendPhrase(dst, rng, 3+rng.IntN(5))
	} else {
		// Everything written so far, plus what still has to follow.
		used := int64(len(dst)-start) + int64(len(rowTail))
		dst = appendFiller(dst, want-used)
	}

	return append(dst, rowTail...)
}

// appendPhrase writes a readable description. Every few words it drops a comma,
// which is the case a CSV reader has to get right and the reason the column is
// quoted at all.
func appendPhrase(dst []byte, rng *rand.Rand, n int) []byte {
	for i := 0; i < n; i++ {
		if i > 0 {
			if i%3 == 0 {
				dst = append(dst, ',')
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
func appendFiller(dst []byte, n int64) []byte {
	if n <= 0 {
		// An empty description is a legal field, and it is what the smallest
		// files get. Anything below zero would mean the minimum was ignored,
		// and FillRecords turns that into an error rather than a wrong size.
		return dst
	}
	start := len(dst)
	for i := 0; int64(len(dst)-start) < n; i++ {
		if len(dst) > start {
			if i%4 == 0 {
				dst = append(dst, ',')
			}
			dst = append(dst, ' ')
		}
		dst = append(dst, words[i%len(words)]...)
	}
	return dst[:start+int(n)]
}

// minimumBytes is the header and one whole row, computed rather than written
// down so it cannot drift away from the template the way a number in a document
// would.
func minimumBytes() int64 {
	var r rows
	return int64(len(header)) + r.Shortest()
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
