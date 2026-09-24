// Package tomlfile generates TOML documents.
//
// The package is not called "toml" so that it cannot be confused with the
// parser the toolkit already pulls into this module, the same reason jsonfile
// is not called json. The format id is "toml".
package tomlfile

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
	"github.com/donislawdev/TestingFilesGenerator/internal/format/textenc"
)

// Measured on 2026-09-22 across five candidate channels and three readers -
// Python's tomllib, tomlkit 0.15.1 and BurntSushi/toml 1.6.0. Comments, basic
// strings and multi line strings all hold to 10 MB, and odd sizes are
// reachable. Written up in docs/YAML-TOML-2026-09-22.md.
//
// tomllib and tomli are the same code vendored twice, so they count as one
// reader rather than two. The second one is tomlkit.
//
// A comment wins that measurement and loses the design. A parser throws
// comments away, so a document padded with one has the right size and gives
// the system under test nothing to do. The filler goes into the note value of
// the last record, where the reader has to carry it, and the comment takes the
// label instead.
const (
	generatorVersion = "1"

	emailDomain = "@example.com"

	// An array of tables rather than one inline array, because that is the
	// shape a person meets in a real TOML file. There is no root key: a TOML
	// document is a table already.
	openID   = "[[records]]\nid = "
	openName = "\nname = \""
	openMail = "\"\nemail = \""
	openAmt  = "\"\namount = "
	openAct  = "\nactive = "
	openTags = "\ntags = [\""
	tagSep   = "\", \""
	openAddr = "\"]\naddress = { city = \""
	openZip  = "\", zip = "
	openNote = " }\nnote = \""
	tail     = "\"\n"

	// maxIDDigits bounds the width of the record number, so the shortest whole
	// record holds for every draw rather than for the lucky one.
	//
	// A constant rather than the width of the next id, for the three reasons
	// written out beside the same name in internal/format/yamlfile: Shortest is
	// a worst case bound on three axes, core.FillRecords reads it once so a
	// growing width goes stale in the loop, and json and xml stand on the same
	// constant with their minimums published.
	maxIDDigits = 19

	amountDigits = 9
	zipDigits    = 5
	// "false" is the longer of the two, and the minimum has to hold for both.
	longestBool = 5
)

func init() {
	format.Register(format.Descriptor{
		ID:          "toml",
		Name:        "TOML",
		Extension:   ".toml",
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,

		// An empty file is legal TOML - measured, it reads back as an empty
		// table. It is still not what a byte count orders, the same answer
		// JSON gives about an empty array. The minimum is one whole record.
		MinBytes: minimumBytes(),

		Padding: format.PaddingChannel{
			Name:  "the note value of the last record",
			Where: format.PlacementEnd,
			// No ceiling found to 10 MB, which is where the measurement
			// stopped.
			Capacity: 0,
		},

		Label:  format.LabelInternal,
		Oracle: "python-toml",
		// Record counts and table layouts come later. Declaring only what is
		// here makes a recipe asking for them fail loudly.
		Properties: nil,

		// Encoding is not a gap here, it is the format. TOML 1.0 says a
		// document is UTF-8, and measured on 2026-09-22 both readers refuse a
		// byte order mark even in front of UTF-8 - so there is nothing to
		// offer rather than something not built yet. Saying so with the reason
		// beats the generic "no such property", which reads as a hole in this
		// build.
		Unsupported: []format.UnsupportedSetting{
			{
				Name: textenc.Setting,
				Why: "TOML is UTF-8 by its own specification, so there is no other " +
					"encoding for a document to be in",
				Instead: "Use yaml, xml, txt or md for a file in another encoding.",
			},
			{
				Name: textenc.SettingBOM,
				Why: "both readers on this machine refuse a TOML file that opens with a " +
					"byte order mark, even in UTF-8",
				Instead: "Use txt or md for a file that opens with a byte order mark.",
			},
		},
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
			Format:    "TOML",
			Requested: r.Bytes,
			Minimum:   min,
			Reason:    "a document holds whole records, and one table of them with every value type needs that much",
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
			"style":       "array-of-tables",
			// A TOML document has no null. The record here carries one value
			// of every type the format does have, and the absence is stated
			// rather than left for somebody to notice it is shorter than the
			// JSON one.
			"null_supported": false,
		},
	}

	m := memo{seed: r.Seed}
	if r.Label {
		line := "# " + core.Label("toml", r.Bytes, r.Seed) + "\n"
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
		return fmt.Errorf("toml: the plan was not produced by this generator")
	}

	// The comment is the whole of the prologue, so with no label there is
	// nothing in front of the first table and the document opens with it.
	if err := core.WriteAll(w, []byte(m.comment)); err != nil {
		return err
	}

	rng := core.NewRand(m.seed)
	return core.FillRecords(ctx, w, rng, p.Bytes-int64(len(m.comment)), &records{})
}

// minimumBytes is one shortest whole record, with no label.
func minimumBytes() int64 {
	r := &records{}
	return r.Shortest()
}

// records builds the tables. It carries the record number, so the id counts up
// the way a real export does.
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
// ids read 1..N with nothing missing.
func (r *records) Discard() { r.next-- }

// fixed is every byte of a record that does not depend on a draw, measured
// from the literals beside it rather than written out as a number.
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
// file in garbage.
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

	used := int64(len(dst)-start) + int64(len(tail))
	dst = core.AppendFiller(dst, words, want-used, nil)
	return append(dst, tail...)
}

// appendPhrase writes a readable note. Words and single spaces only - a basic
// string cannot take a quote or a backslash raw, and neither appears in the
// vocabulary. Measured 2026-09-22: every other byte from 20 to 7E goes in as
// itself, so an escape never costs a byte the arithmetic did not budget for.
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
// bytes per format, so one shared list would move every file at once the day a
// word changed.
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
