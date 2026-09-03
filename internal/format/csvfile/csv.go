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

	// quoteMark wraps a field. It is the one character RFC 4180 gives a value
	// for holding a separator inside it, and how many of them a row carries is
	// what quote_style decides.
	quoteMark = '"'

	// maxRowDigits bounds the width of the row number. A row is at least one
	// byte, so a file can never hold more rows than it has bytes, and a size is
	// an int64 - nineteen digits covers every file this tool can be asked for.
	// Bounding it rather than guessing is what lets the closing row reach its
	// length whatever row number it lands on.
	maxRowDigits = 19
)

// widestColumn is the most bytes the column at this position can take, for any
// draw. The description is not here: it is always last and is empty in the row
// the floor is made of.
//
// Measured rather than guessed only in the sense that every number in it is
// read off the template above. The positions match columnNamesFor, and a
// column past the ones this format started with holds a word.
func widestColumn(at int) int {
	switch at {
	case 0:
		return maxRowDigits
	case 1:
		return longestWord
	case 2:
		return longestWord + len(emailDomain)
	case 3:
		return amountWidth
	case 4:
		return len(createdDate)
	default:
		return longestWord
	}
}

// fixedWidth is the whole row except the description: every leading column at
// its widest, the separators between all of them, the quotes and the row
// ending.
//
// Three settings move it. A CRLF row costs one byte more than an LF one on
// every row, quoting every field costs two bytes a column, and the number of
// columns moves every term at once - which is why the minimum moves with any
// of the three.
//
// The separators count one byte each, which is a fact about the separators
// offered rather than an assumption: every one of them is a single byte, and
// dialect.go says so where they are declared.
func fixedWidth(d dialect) int64 {
	total := int64(d.columns-1) /* separators */ + int64(len(d.eol)) +
		int64(d.quotes.quoteBytes(d.columns))
	for at := 0; at < d.columns-1; at++ {
		total += int64(widestColumn(at))
	}
	return total
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
		// Column count and column types come later. Declaring neither of them
		// now makes a recipe asking for one fail loudly.
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
			"quote_style": d.quotes.id,
			"columns":     d.columns,
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

	// scratch holds the description while it is being asked whether it needs
	// quotes. It cannot be written straight into the row, because the answer
	// decides whether a quote goes in FRONT of it.
	//
	// Reused rather than allocated per row. A table of any size is millions of
	// rows and the resource guard measures exactly that. It stays small: the
	// closing row is the longest and is bounded by twice the shortest row.
	scratch []byte
}

// Shortest is the smallest row this builder can close a file with: the widest
// row number, the longest word wherever a word goes, and an empty description.
// It has to hold for every draw rather than for the lucky one.
func (r *rows) Shortest() int64 {
	return fixedWidth(r.dia)
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
	q := r.dia.quotes

	// Every column but the description, in the order columnNamesFor names
	// them. The draws above happen once and outside this loop, which is what
	// keeps a six column table byte for byte what it always was: a narrower
	// table draws exactly the same values and writes fewer of them, and a wider
	// one draws its extra words only when it reaches them.
	for at := 0; at < r.dia.columns-1; at++ {
		dst = q.mark(dst)
		dst = r.appendValue(dst, rng, at, name, whole, cents)
		dst = q.mark(dst)
		dst = append(dst, sep)
	}

	return r.appendDescription(dst, rng, want, int64(len(dst)-start))
}

// appendValue writes the value of the column at this position.
//
// The name is drawn once by the caller and used twice, in the name column and
// inside the address, which is how this table has always read. The positions
// match widestColumn above, and the two are the pair that has to stay in step -
// a value wider than its column would make the closing row overshoot a length
// it was handed.
func (r *rows) appendValue(dst []byte, rng *rand.Rand, at int, name string, whole, cents int) []byte {
	switch at {
	case 0:
		return strconv.AppendInt(dst, r.next, 10)
	case 1:
		return append(dst, name...)
	case 2:
		dst = append(dst, name...)
		return append(dst, emailDomain...)
	case 3:
		dst = strconv.AppendInt(dst, int64(whole), 10)
		dst = append(dst, '.')
		if cents < 10 {
			dst = append(dst, '0')
		}
		return strconv.AppendInt(dst, int64(cents), 10)
	case 4:
		return append(dst, createdDate...)
	default:
		return append(dst, words[rng.IntN(len(words))]...)
	}
}

// appendDescription writes the last field and ends the row.
//
// want below zero means a natural row, any other value is the exact length the
// whole row must have. used is what the row has spent already, measured rather
// than worked out beside the bytes.
func (r *rows) appendDescription(dst []byte, rng *rand.Rand, want, used int64) []byte {
	if want < 0 {
		r.scratch = appendPhrase(r.scratch[:0], rng, 3+rng.IntN(5),
			r.dia.sep, r.dia.quotes.separatorsInDescription)
		return r.closeRow(dst, r.scratch)
	}

	// What is left for the description AND its quotes together. Which of the
	// two it is comes out of fill below.
	r.scratch = r.fill(r.scratch[:0], want-used-int64(len(r.dia.eol)))
	return r.closeRow(dst, r.scratch)
}

// fill builds the description of the closing row so the row lands on exactly
// the length it was asked for.
//
// room is the description and its quotes together, and how it divides between
// them is the whole of this function. With "all" the quotes are certain. With
// "none" there are none. With "minimal" it depends on the description itself,
// so the quoted length is built first and MEASURED: if it carries the
// separator the quotes are earned and that is the answer, and if it does not,
// the description is built to the full room with the separator withheld, which
// leaves nothing for a quote to be needed for.
//
// Measured on 2026-09-03: the filler first carries a separator at 30 B of
// description, so the second branch is reached by the two lengths either side
// of that. It is a narrow band and it is the only place the two halves of this
// setting could have disagreed.
func (r *rows) fill(dst []byte, room int64) []byte {
	q, sep := r.dia.quotes, r.dia.sep

	if q.everyField {
		return appendFiller(dst, room-2, sep, true)
	}
	if q.separatorsInDescription && room >= 2 {
		dst = appendFiller(dst, room-2, sep, true)
		if q.wraps(dst, sep) {
			return dst
		}
		dst = dst[:0]
	}
	return appendFiller(dst, room, sep, false)
}

// closeRow puts the description into the row and ends it.
//
// It asks the setting whether these bytes carry quotes, and fill above asked
// the same question to work the length out - one question, one answer, so the
// arithmetic and the bytes cannot part company.
func (r *rows) closeRow(dst, description []byte) []byte {
	if r.dia.quotes.wraps(description, r.dia.sep) {
		dst = append(dst, quoteMark)
		dst = append(dst, description...)
		dst = append(dst, quoteMark)
	} else {
		dst = append(dst, description...)
	}
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
//
// carries is false only under quote_style none, where an unquoted field cannot
// hold a separator without ending early.
func appendPhrase(dst []byte, rng *rand.Rand, n int, sep byte, carries bool) []byte {
	for i := 0; i < n; i++ {
		if i > 0 {
			if carries && i%3 == 0 {
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
// It follows the dialect for the reason appendPhrase gives, and it withholds
// the separator for the reason appendPhrase gives too.
func appendFiller(dst []byte, n int64, sep byte, carries bool) []byte {
	both := string(sep) + " "
	return core.AppendFiller(dst, words, n, func(i int) string {
		if carries && i%4 == 0 {
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
