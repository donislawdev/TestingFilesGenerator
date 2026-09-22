// Package yamlfile generates YAML documents.
//
// The package is not called "yaml" so that it cannot be confused with the
// parser this module already links, the same reason jsonfile is not called
// json and logfile is not called log. The format id is "yaml".
package yamlfile

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

// Measured on 2026-09-22 across twelve candidate channels and four readers -
// PyYAML 6.0.3, ruamel.yaml 0.19.1, goccy/go-yaml 1.19.2 and gopkg.in/yaml.v3
// 3.0.1. Comments, quoted scalars, block scalars and blank lines all hold to
// 10 MB, and odd sizes are reachable everywhere. Written up in
// docs/YAML-TOML-2026-09-22.md.
//
// A comment wins that measurement and loses the design, which is the second
// half of the lesson rather than a footnote. A parser throws comments away, so
// a five megabyte document built as two kilobytes of records and five
// megabytes of comment is the right size and a useless fixture - the system
// under test gets two kilobytes of work. It is the same trap JSON turned down
// with whitespace.
//
// So the filler goes where the reader has to carry it: the note value of the
// last record. The comment carries the label instead, and that is worth
// something on its own - a comment is not data, so YAML is the first record
// format here whose label rides inside the file without changing the structure
// under test. CSV and JSON label from the outside for exactly that reason.
const (
	generatorVersion = "1"

	emailDomain = "@example.com"

	// The document is a block mapping holding a block sequence, rather than
	// the flow style that would also be legal. YAML is a superset of JSON, so
	// a flow document would BE a JSON document with a different extension -
	// which is the one thing this format must not produce if it is to be worth
	// having beside jsonfile.
	rootOpen = "records:\n"

	openID   = "  - id: "
	openName = "\n    name: "
	openMail = "\n    email: "
	openAmt  = "\n    amount: "
	openAct  = "\n    active: "
	openTags = "\n    tags:\n      - "
	tagSep   = "\n      - "
	openAddr = "\n    address:\n      city: "
	openZip  = "\n      zip: "
	// The note is quoted where every other scalar is plain, and the reason is
	// the smallest file rather than taste. The filler in a closing record can
	// come to nought bytes, and a plain scalar with nothing after the colon is
	// null - so the type of this value would depend on the size asked for,
	// which is a defect no size guard could see. Quoted, it is the empty
	// string at every size.
	openNote = "\n    note: \""
	tail     = "\"\n"

	// maxIDDigits bounds the width of the record number, so the shortest whole
	// record holds for every draw rather than for the lucky one. A document
	// cannot carry more records than this many digits allow.
	//
	// It is a constant rather than the width of the next id, and that is a
	// decision rather than an oversight. Measured 2026-09-22: at the floor the
	// closing record carries 27 B of slack, 18 of which is this reserve - so a
	// yaml floor of 223 B would be reachable. Three things are wrong with
	// taking it.
	//
	// Shortest is a worst case bound on three axes - the id width, the five
	// word draws and the longer boolean - and dropping one of the three while
	// keeping two is arbitrary. core.FillRecords reads it ONCE, so a width
	// that changes as the ids grow goes stale inside the loop: a closing record
	// with a wide id and five long words can then need more than the cached
	// bound promised, and AppendExact is asked for a record shorter than the
	// one it must write. That window is a handful of bytes wide and needs an
	// unlucky draw beside it - 401 sizes were swept without hitting it, which
	// says it is rare rather than that it is absent. And json and xml stand on
	// this same constant with their minimums published, so moving it is a D11
	// breaking change to two released formats to save 18 B on this one.
	maxIDDigits = 19

	// The amount is always six digits, a point and two more, and the postcode
	// is always five. Fixed widths, so the arithmetic does not have to ask.
	amountDigits = 9
	zipDigits    = 5
	// "false" is the longer of the two, and the minimum has to hold for both.
	longestBool = 5
)

func init() {
	format.Register(format.Descriptor{
		ID:          "yaml",
		Extension:   ".yaml",
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,

		// An empty file is legal YAML - measured, all four readers take it and
		// return a null document. It is still not what a byte count orders:
		// that is a shape request, and it arrives with a record count property
		// this build does not have yet. The minimum here is the root and one
		// whole record, the same answer JSON gives to the same question about
		// an empty array.
		MinBytes: minimumBytes(),

		Padding: format.PaddingChannel{
			Name:  "the note value of the last record",
			Where: format.PlacementEnd,
			// No ceiling found to 10 MB, which is where the measurement
			// stopped. Above that is not knowledge this project has.
			Capacity: 0,
		},

		Label:  format.LabelInternal,
		Oracle: "python-yaml",
		// Record counts, flow style, multiple documents and encodings other
		// than UTF-8 come later. Declaring only what is here makes a recipe
		// asking for them fail loudly rather than quietly producing something
		// else.
		//
		// UTF-16 is a measured possibility rather than an oversight: with a
		// byte order mark in front of it, all four readers took utf-16le and
		// utf-16be, and refused both without one. That is a joint rule between
		// two settings, so it arrives with them or not at all.
		Properties:       nil,
		GeneratorVersion: generatorVersion,
		Generator:        generator{},
	})
}

type generator struct{}

type memo struct {
	seed    uint64
	comment string
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	min := minimumBytes()
	if r.Bytes < min {
		return format.Plan{}, &format.BelowMinimumError{
			Format:    "YAML",
			Requested: r.Bytes,
			Minimum:   min,
			Reason:    "a document holds a root key and whole records, and one of each needs that much",
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
			"style":       "block",
			"depth":       3,
		},
	}

	m := memo{seed: r.Seed}
	if r.Label {
		// A comment carries the label without touching the content, and it
		// sits in front of the document where a person opening the file reads
		// it first. Nothing in the label needs escaping: it is plain ASCII and
		// a comment runs to the end of the line.
		line := "# " + core.Label("yaml", r.Bytes, r.Seed) + "\n"
		// It has to leave room for a whole document beside it, or the file
		// would be a comment and a root key with nothing under it.
		if int64(len(line))+min <= r.Bytes {
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

	p.Properties[format.PropertyLabelEmbedded] = m.comment != ""
	p.Memo = m
	return p, nil
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(memo)
	if !ok {
		return fmt.Errorf("yaml: the plan was not produced by this generator")
	}

	head := m.comment + rootOpen
	if err := core.WriteAll(w, []byte(head)); err != nil {
		return err
	}

	rng := core.NewRand(m.seed)
	return core.FillRecords(ctx, w, rng, p.Bytes-int64(len(head)), &records{})
}

// minimumBytes is the root key and one shortest whole record, with no label.
func minimumBytes() int64 {
	r := &records{}
	return int64(len(rootOpen)) + r.Shortest()
}

// records builds the sequence items under the root key. It carries the record
// number, so the id counts up the way a real export does.
type records struct {
	next int64
}

// Shortest is the smallest record this builder can close a document with: the
// widest record number, the longest word in all five places a word appears,
// the longer of the two booleans, and an empty note.
func (r *records) Shortest() int64 {
	return int64(maxIDDigits + 5*longestWord + fixed())
}

func (r *records) Append(dst []byte, rng *rand.Rand) []byte {
	return r.append(dst, rng, -1)
}

func (r *records) AppendExact(dst []byte, rng *rand.Rand, n int64) []byte {
	return r.append(dst, rng, n)
}

// Discard hands back the number the thrown away record took with it, so the
// closing record carries it instead and the ids read 1..N with nothing
// missing. The gap that shape leaves was a real defect in three record
// formats, found by looking at a file rather than by any guard.
func (r *records) Discard() { r.next-- }

// fixed is every byte of a record that does not depend on a draw.
//
// Measured from the literals beside it rather than written out as a number.
// Arithmetic that has to agree with the bytes next to it is a defect waiting
// for the day somebody adds a field and updates one of the two.
func fixed() int {
	return len(openID) + len(openName) + len(openMail) + len(emailDomain) +
		len(openAmt) + amountDigits + len(openAct) + longestBool +
		len(openTags) + len(tagSep) + len(openAddr) + len(openZip) + zipDigits +
		len(openNote) + len(tail)
}

// append writes one record. A want below zero means whatever length it comes
// out, any other value is the exact length the record must have.
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
		return append(dst, tail...)
	}

	// Everything written so far, plus the bytes that close the record.
	used := int64(len(dst)-start) + int64(len(tail))
	dst = core.AppendFiller(dst, words, want-used, nil)
	return append(dst, tail...)
}

// appendPhrase writes a readable note. Words and single spaces only - the one
// thing a quoted scalar cannot take raw is a quote or a backslash, and neither
// appears in the vocabulary. Measured 2026-09-22: every other byte from 20 to
// 7E goes in as itself, so an escape never costs a byte the arithmetic did not
// budget for.
func appendPhrase(dst []byte, rng *rand.Rand, n int) []byte {
	for i := 0; i < n; i++ {
		if i > 0 {
			dst = append(dst, ' ')
		}
		dst = append(dst, words[rng.IntN(len(words))]...)
	}
	return dst
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

// words is the vocabulary for names, tags, cities and notes. Its own copy
// rather than a shared one, like every other format here: D11 freezes these
// bytes per format, so one shared list would move twenty four files at once
// the day a word changed.
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
