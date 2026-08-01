// Package logfile generates access log files.
//
// The package is not called "log" so that it cannot be confused with the
// standard library package of that name at a glance. The format id is "log".
package logfile

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// The padding channel is the content, like the rest of the text group. What
// the measurement did not settle, and this file does: a log is read line by
// line, so every line has to be a whole entry.
//
// Filling to an exact byte count by cutting the last line would leave a
// half entry that a parser rejects - and "the last line is truncated" is
// exactly what a real log looks like mid rotation, so the failure would read
// as realism rather than as a defect. Instead the last entry is built to the
// byte, with the request path taking up the difference. Every line parses.
//
// Same shape as the CSV finding on 2026-08-01: padding goes where the format
// has room for a long value, never into a truncated record.

const (
	generatorVersion = "1"

	// Every field except the path is fixed width, so the length of an entry
	// is known before it is built and the path absorbs the difference.
	timestamp = "01/Aug/2026:12:00:00 +0000"

	// statusWidth and sizeWidth are why the ranges below are picked - a status
	// is always three digits and a byte count always six, so neither changes
	// the length of a line.
	statusWidth = 3
	sizeWidth   = 6

	// longestAddress is 255.255.255.255. Used for the minimum, which has to
	// hold for every draw rather than for the lucky one.
	longestAddress = 15

	// fixedWidth is every byte of a line except the address, the path and the
	// user agent. A constant expression, so it costs nothing at run time.
	fixedWidth = len(" - - [") + len(timestamp) + len("] \"GET /") +
		len(" HTTP/1.1\" ") + statusWidth + len(" ") + sizeWidth + len(" \"-\" \"") + len("\"\n")
)

func init() {
	format.Register(format.Descriptor{
		ID:          "log",
		Extension:   ".log",
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,

		// Unlike text, a log file of nought bytes holds no entries and a log
		// with no entries is not a fixture anybody asked for. The minimum is
		// one whole entry, and asking for less is refused with the number.
		MinBytes: minimumBytes(),

		Padding: format.PaddingChannel{
			Name:     "the request path of the last entry",
			Where:    format.PlacementEnd,
			Capacity: 0,
		},
		Label:  format.LabelVisible,
		Oracle: format.OracleNone,
		// Entry format, rate, time range and level mix come later. Declaring
		// none now makes a recipe asking for them fail loudly.
		Properties:       nil,
		GeneratorVersion: generatorVersion,
		Generator:        generator{},
	})
}

type generator struct{}

type memo struct {
	labelLine string // includes the trailing newline, empty when absent
	seed      uint64
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	min := minimumBytes()
	if r.Bytes < min {
		return format.Plan{}, &format.BelowMinimumError{
			Format:    "LOG",
			Requested: r.Bytes,
			Minimum:   min,
			Reason:    "a log holds whole entries and one entry in the combined format needs that much",
			Hint:      fmt.Sprintf("Ask for %d B or more.", min),
		}
	}

	p := format.Plan{
		Bytes:       r.Bytes,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties: map[string]any{
			"encoding":     "utf-8",
			"line_ending":  "lf",
			"entry_format": "apache-combined",
		},
	}

	m := memo{seed: r.Seed}
	if r.Label {
		// A log has no comment syntax that every reader agrees on, so the
		// label is a line of its own. It is the one line that is not an entry,
		// and it says so in words rather than pretending to be one.
		line := "# " + core.Label("log", r.Bytes, r.Seed) + "\n"
		// It has to leave room for at least one whole entry, or the file would
		// be a label and nothing else.
		if int64(len(line))+minEntry() <= r.Bytes {
			m.labelLine = line
		} else {
			p.Notes = append(p.Notes, format.Note{
				Code: "label_omitted",
				Detail: fmt.Sprintf(
					"The label line needs %d B and this file has no room for it beside a whole entry. Its name and the manifest still identify it.",
					len(line)),
			})
		}
	}

	p.Properties["label_embedded"] = m.labelLine != ""
	p.Memo = m
	return p, nil
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(memo)
	if !ok {
		return fmt.Errorf("log: the plan was not produced by this generator")
	}

	remaining := p.Bytes
	if m.labelLine != "" {
		if err := core.WriteAll(w, []byte(m.labelLine)); err != nil {
			return err
		}
		remaining -= int64(len(m.labelLine))
	}

	// Whole entries while another one still leaves room for a final entry of
	// its own, then a closing entry built to the byte. That rule is the same
	// for every record based format here, so it lives in core rather than
	// being written out a fourth time.
	rng := core.NewRand(m.seed)
	return core.FillRecords(ctx, w, rng, remaining, entries{})
}

// entries is the log seen as a stream of records.
type entries struct{}

func (entries) Shortest() int64 { return minEntry() }

func (entries) Append(dst []byte, rng *rand.Rand) []byte {
	return appendEntry(dst, rng, -1)
}

func (entries) AppendExact(dst []byte, rng *rand.Rand, n int64) []byte {
	return appendEntry(dst, rng, n)
}

// Discard has nothing to put back. An entry carries no state from one to the
// next, so throwing one away leaves no trace to undo.
func (entries) Discard() {}

// appendEntry appends one line in the Apache combined format.
//
// want below zero means "whatever length it comes out". Any other value is the exact
// length the line must have, newline included, and the request path is
// stretched to reach it.
//
// It appends rather than returning a new slice because a log of any size is
// millions of entries, and one allocation per entry is a multiple of the file
// in garbage. The resource guard measures that.
func appendEntry(dst []byte, rng *rand.Rand, want int64) []byte {
	// Every field but the path is fixed width or drawn from a list, so the
	// length of the line is known before the path is chosen.
	a, b, c, d := 10+rng.IntN(240), rng.IntN(256), rng.IntN(256), 1+rng.IntN(254)
	status := statuses[rng.IntN(len(statuses))]
	size := 100000 + rng.IntN(899999)
	agent := agents[rng.IntN(len(agents))]
	path := paths[rng.IntN(len(paths))]

	// The length of everything but the path, as arithmetic rather than by
	// building a string and measuring it - that allocated once per entry,
	// about the size of the file again in garbage over a large log.
	base := int64(fixedWidth + len(agent) + digits(a) + digits(b) + digits(c) + digits(d) + 3)

	// No leading zeros. Padding octets to three digits made the line length
	// trivial to predict and produced addresses no real log contains - and a
	// leading zero is read as octal by some address parsers, where 069 is not
	// even valid octal. Untouchable rule 4: fidelity does not drop for the
	// convenience of the implementation.
	dst = strconv.AppendInt(dst, int64(a), 10)
	dst = append(dst, '.')
	dst = strconv.AppendInt(dst, int64(b), 10)
	dst = append(dst, '.')
	dst = strconv.AppendInt(dst, int64(c), 10)
	dst = append(dst, '.')
	dst = strconv.AppendInt(dst, int64(d), 10)
	dst = append(dst, " - - ["...)
	dst = append(dst, timestamp...)
	dst = append(dst, "] \"GET /"...)

	if want < 0 {
		dst = append(dst, path...)
	} else {
		dst = appendPath(dst, want-base)
	}

	dst = append(dst, " HTTP/1.1\" "...)
	dst = strconv.AppendInt(dst, int64(status), 10)
	dst = append(dst, ' ')
	dst = strconv.AppendInt(dst, int64(size), 10)
	dst = append(dst, " \"-\" \""...)
	dst = append(dst, agent...)
	return append(dst, "\"\n"...)
}

// digits is how many characters a byte sized number takes. The address is
// written the way a person sees it, so the length varies and the path has to
// know by how much.
func digits(n int) int {
	switch {
	case n < 10:
		return 1
	case n < 100:
		return 2
	default:
		return 3
	}
}

// appendPath writes a URL path of exactly n bytes out of readable segments, so
// a padded entry still looks like a request rather than a run of one letter.
func appendPath(dst []byte, n int64) []byte {
	if n < 1 {
		// Only reachable if the caller ignored the minimum. The check in Write
		// turns that into an error rather than a file of the wrong size.
		return append(dst, 'x')
	}
	start := len(dst)
	for int64(len(dst)-start) < n {
		if len(dst) > start {
			dst = append(dst, '/')
		}
		dst = append(dst, "segment"...)
	}
	return dst[:start+int(n)]
}

// minEntry is the length of the shortest line this generator can produce, and
// minimumBytes is the smallest file - one whole entry.
//
// Computed rather than written down, so it cannot drift away from the template
// above the way a number in a document would.
func minEntry() int64 {
	// The longest agent, because any entry may draw it and the minimum has to
	// hold for every draw rather than for the lucky one.
	longest := 0
	for _, a := range agents {
		if len(a) > longest {
			longest = len(a)
		}
	}
	// The longest address too, for the same reason, plus one character of
	// path - the shortest a path can be.
	return int64(fixedWidth + longestAddress + longest + 1)
}

func minimumBytes() int64 { return minEntry() }

var statuses = []int{200, 200, 200, 201, 204, 301, 302, 304, 400, 401, 403, 404, 409, 429, 500, 502, 503}

var paths = []string{
	"index.html", "api/v1/invoices", "api/v1/orders", "static/app.css",
	"static/app.js", "images/logo.png", "health", "login", "logout",
	"api/v1/customers/4711", "reports/monthly.pdf", "search?q=invoice",
}

var agents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) Gecko/20100101 Firefox/128.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Safari/605.1.15",
	"Mozilla/5.0 (X11; Linux x86_64) Chrome/126.0.0.0 Safari/537.36",
	"curl/8.7.1",
	"python-requests/2.32.3",
	"Go-http-client/2.0",
}
