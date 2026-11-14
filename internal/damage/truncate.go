package damage

import (
	"io"
	"math"
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// Truncate is the damage that cuts the end off a file. A public name under
// untouchable rule 10, and so is its setting.
const (
	Truncate = "truncate"

	SettingMissing = "missing"
)

const (
	// smallestCut is the fewest bytes worth cutting, and it is a measurement.
	//
	// Measured 2026-11-09 across 31 formats at 20 kB and 1 MiB: cutting one
	// byte leaves pdf, svg and xml intact, because each ends with a line
	// break, and from four bytes up the set of formats that notice stays the
	// same at every share tried, from 1% to 90% (docs/DAMAGE-ETAP-2-2026-11-09.md
	// section 2.1). Below four the cut lands in whitespace the format allows.
	smallestCut = 4

	// fewestMissing and mostMissing bound the share, in percent.
	//
	// The top is chosen rather than measured: at 90% the generator is asked
	// for ten times the file, and past it the plan grows for no new test. The
	// bottom is the smallest whole share.
	fewestMissing  = 1
	mostMissing    = 90
	defaultMissing = 50
)

func init() {
	Register(Descriptor{
		ID: Truncate,
		Detail: "Cuts the end off the file and keeps exactly the size you asked for, " +
			"the way an interrupted upload or copy leaves it.",
		Parameters: []format.Property{
			{
				Name: SettingMissing, Kind: format.PropertyInt,
				// No unit: "whole number of percent from 1 to 90" is not how
				// anyone says it, and the sentence below names the percent.
				Min: fewestMissing, Max: mostMissing,
				Default: strconv.Itoa(defaultMissing),
				// The last two sentences are what a person cannot see from the
				// outside. Every format lays its padding out for the cut, so the
				// cut removes the same percentage of the content
				// (docs/TRUNCATE-2026-11-11.md sections 7 to 15, and
				// TestACutTakesTheShareOfTheContentItIsAskedFor). In a file of
				// a few kilobytes it can take less: a WebP with less than 26 B
				// of padding cannot put any in front of its picture and keeps up
				// to that much more of it, and where what a format puts after
				// its content - the central directory of a ZIP or an Office
				// document, that WebP's padding - is longer than a cut of 1% or
				// 2%, the cut falls in it and leaves the content whole
				// (docs/TRUNCATE-2026-11-11.md section 15.4, measured in
				// docs/REVIEW-244-2026-11-14.md). The manifest says per file
				// where the cut fell.
				Detail: "How much of the whole file is gone, as a percentage. " +
					"At least this much is cut - a few bytes more where the format cannot end exactly there. " +
					"The cut removes the same percentage of the content - of the picture, the sound, the pages or the text. " +
					"A file of a few kilobytes cut by 1% or 2% can lose less of its content than that, or none of it when the cut falls in what the format puts after the content, such as a ZIP's list of its files - " +
					"the manifest says what each cut left.",
			},
		},
		Floor:    func(v Values) int64 { return smallestKept(missingShare(v)) },
		Length:   func(v Values, out int64) int64 { return withCut(out, missingShare(v)) },
		CutsTail: true,
		Policy:   truncatePolicy,
		Open: func(_ Values, f File, out io.Writer) (Stream, error) {
			return &truncation{out: out, keep: f.Bytes}, nil
		},
	})
}

// truncatePolicy is where a file cut short is still a valid file of its
// format, or is refused only for the shape this tool writes.
//
// Measured on 2026-11-10 with every setting of every format, reading the
// reason of each refusal rather than counting them
// (docs/DAMAGE-ETAP-2-2026-11-09.md section 12.1). Everywhere else the cut
// leaves something the format's rules forbid: a box or chunk longer than the
// file, a missing end marker, an element never closed.
var truncatePolicy = []PolicyCase{
	{Format: "txt", Why: "A text cut short is still text, and nothing in the file says it was cut."},
	{Format: "md", Why: "A Markdown document cut short is still a valid document."},
	{Format: "eml", Why: "A message cut short can still be a valid message, because its body is optional."},
	{Format: "csv", Why: "A table cut short ends in a row with no line break, which RFC 4180 allows."},
	{Format: "log", Why: "A log cut short ends part way through an entry, which most readers keep or skip."},
	{Format: "yaml", Why: "A YAML document cut short can still be valid YAML with fewer or shorter values."},
	{Format: "toml", Why: "A TOML document cut short can still be valid TOML, for example when the cut falls between two lines or inside a number."},
	{Format: "html", Why: "HTML readers are required to finish a page that is cut short."},
	// One layout only, measured 2026-11-11 on the OpenTelemetry Collector
	// 0.165.0 (docs/OTLP-2026-11-10.md section 12.3): its file receiver read
	// every whole line and said nothing of the cut one, and over HTTP it
	// answered 200. Cut short as one request, the same data is refused with
	// 400.
	{Format: "otlp", When: map[string][]string{"formatting": {"record-per-line"}},
		Why: "An OTLP file cut short still holds whole lines, and a reader of lines keeps those and drops the cut one without a word."},
}

// missingShare is the share this run cuts, in percent.
//
// The value has been past the declaration by the time it arrives. It falls
// back rather than failing for the reason headBytes gives: this is callable
// directly, and code that trusts its input is one registry change away from
// writing a file nobody ordered.
func missingShare(v Values) int64 {
	n, err := strconv.ParseInt(v[SettingMissing], 10, 64)
	if err != nil || n < fewestMissing || n > mostMissing {
		return defaultMissing
	}
	return n
}

// cutFor is how many bytes go missing from a file that keeps out bytes, so
// that they are the stated share of the whole: out * m / (100 - m), rounded
// up so the share is never less than stated.
//
// Divided before it is multiplied, so the remainder cannot wrap whatever the
// size. The product still can, once the cut is larger than what is kept: nine
// tenths missing is nine times the size, which passes the top of an int64
// above about 1e18 B. It saturates there rather than wrapping to a smaller
// number, which a format would have been asked for and a refusal would have
// named.
func cutFor(out, m int64) int64 {
	rest := 100 - m
	q, r := out/rest, out%rest
	if q > (math.MaxInt64-m)/m {
		return math.MaxInt64
	}
	return q*m + (r*m+rest-1)/rest
}

// withCut is the whole file for out bytes kept, saturating rather than
// wrapping - the format refuses the size then, in its own words.
func withCut(out, m int64) int64 {
	cut := cutFor(out, m)
	if out > math.MaxInt64-cut {
		return math.MaxInt64
	}
	return out + cut
}

// smallestKept is the fewest bytes a file can keep so that at least
// smallestCut bytes are cut: out * m / (100 - m) has to pass smallestCut - 1.
func smallestKept(m int64) int64 {
	return (smallestCut-1)*(100-m)/m + 1
}

// truncation passes on the first bytes of a file and drops the rest.
//
// Streaming like zero-head, so the regression line "a generator does not hold
// the whole file in memory" is untouched: it holds a count, never the file.
type truncation struct {
	out io.Writer
	// keep is how many bytes are still to be passed on.
	keep int64
	// cut records whether any byte was dropped. A file the generator ended at
	// exactly the bytes kept was not cut, and is a damage that did nothing.
	cut bool
}

func (t *truncation) Write(p []byte) (int, error) {
	if int64(len(p)) <= t.keep {
		n, err := t.out.Write(p)
		t.keep -= int64(n)
		return n, err
	}
	t.cut = true
	head := p[:t.keep]
	n, err := t.out.Write(head)
	t.keep -= int64(n)
	if err != nil {
		return n, err
	}
	// Fewer bytes taken than offered, with the reason - which is what the
	// io.Writer contract asks of a short write, and what stops the generator.
	return n, ErrKeptEnough
}

func (t *truncation) Touched() bool { return t.cut }

// Finish has nothing to add. What was cut is gone.
func (t *truncation) Finish() error { return nil }
