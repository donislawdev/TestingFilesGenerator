// Package logfile generates log files.
//
// The package is not called "log" so that it cannot be confused with the
// standard library package of that name at a glance. The format id is "log".
//
// Six shapes since 2026-08-31, where there was one before: the Apache family,
// nginx, syslog, a plain application log and JSON lines. Every template was
// taken from a real file rather than from memory - see shapes.go, which says
// which two of them memory would have got wrong.
package logfile

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

// The padding channel is the content, like the rest of the text group. What
// the measurement did not settle, and this file does: a log is read line by
// line, so every line has to be a whole entry.
//
// Filling to an exact byte count by cutting the last line would leave a
// half entry that a parser rejects - and "the last line is truncated" is
// exactly what a real log looks like mid rotation, so the failure would read
// as realism rather than as a defect. Instead the last entry is built to the
// byte, with one stretchable field per shape taking up the difference. Every
// line parses.

const (
	generatorVersion = "1"

	// statusWidth and sizeWidth are why the ranges in shapes.go are picked - a
	// status is always three digits and a byte count always six, so neither
	// changes the length of a line.
	statusWidth = 3
	sizeWidth   = 6
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
		//
		// The number announced is the DEFAULT shape's, because a guard holds
		// this tool to accepting whatever minimum it prints. A shape that
		// needs more raises the floor when it is chosen, and says its own
		// number then - the same way a picture size named by hand does.
		MinBytes: minimumBytes(),

		Padding: format.PaddingChannel{
			Name:     "the request path or message of the last entry",
			Where:    format.PlacementEnd,
			Capacity: 0,
		},
		Label:      format.LabelVisible,
		Oracle:     format.OracleNone,
		Properties: properties(),

		GeneratorVersion: generatorVersion,
		Generator:        generator{},
	})
}

func properties() []format.Property {
	return []format.Property{
		{
			Name: "entry_format", Kind: format.PropertyChoice,
			Choices: shapeIDs, Default: defaultShape,
			Detail: "Which kind of log to write. Web server shapes carry a request and a status, the others carry a level and a message.",
		},
		{
			Name: "timestamps", Kind: format.PropertyChoice,
			Choices: []string{"advancing", "fixed"}, Default: "advancing",
			Detail: "Whether each entry happens later than the one before it. Fixed puts every entry at the same instant, which is what this format did before it could advance.",
		},
		{
			Name: "rate", Kind: format.PropertyInt,
			Min: minRate, Max: maxRate, Unit: "entries per second",
			Default: strconv.Itoa(defaultRate),
			Detail:  "How fast the entries arrive. Only means anything while timestamps advance.",
		},
		{
			Name: "methods", Kind: format.PropertyChoice,
			Choices: []string{"get", "read", "mixed"}, Default: "get",
			Detail: "Which request methods appear. Read is GET and HEAD, mixed adds POST, PUT, PATCH and DELETE.",
		},
		{
			Name: "status_mix", Kind: format.PropertyChoice,
			Choices: []string{"realistic", "success", "client-errors", "server-errors"}, Default: "realistic",
			Detail: "Which response codes appear. Realistic is mostly success with a tail of errors.",
		},
		{
			Name: "ip_version", Kind: format.PropertyChoice,
			Choices: []string{"v4", "v6", "mixed"}, Default: "v4",
			Detail: "Which kind of client address appears. Choose v6 to find out whether a reader handles it.",
		},
		{
			Name: "line_ending", Kind: format.PropertyChoice,
			Choices: []string{"lf", "crlf"}, Default: "lf",
			Detail: "How each line ends. Choose crlf for a log written by a Windows service.",
		},
	}
}

type generator struct{}

type memo struct {
	labelLine string // includes the terminator, empty when absent
	seed      uint64
	opt       options
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	opt, err := parseOptions(r.Properties)
	if err != nil {
		return format.Plan{}, err
	}

	min := opt.shape.shortest(opt)
	if r.Bytes < min {
		return format.Plan{}, &format.BelowMinimumError{
			Format:    "LOG",
			Requested: r.Bytes,
			Minimum:   min,
			Reason: fmt.Sprintf(
				"a log holds whole entries and one entry in the %s shape needs that much", opt.shape.id),
			Hint: fmt.Sprintf("Ask for %d B or more, or choose a shorter entry_format.", min),
		}
	}

	p := format.Plan{
		Bytes:       r.Bytes,
		Exact:       true,
		Determinism: format.DeterminismByte,
		// Only what is true OF THIS SHAPE. A syslog line carries no request,
		// so recording methods beside it would be the manifest stating a fact
		// about the file that is not one - and the manifest is the half of this
		// tool a test suite reads rather than a person, so a value nobody can
		// see in the file is worse there than anywhere.
		Properties: map[string]any{
			"encoding":     "utf-8",
			"line_ending":  opt.lineEnding,
			"entry_format": opt.shape.id,
			"timestamps":   opt.timestamps,
		},
	}
	if opt.advancing {
		p.Properties["rate"] = opt.rate
	}
	if opt.shape.web {
		p.Properties["methods"] = opt.methodMix
		p.Properties["ip_version"] = opt.ipVersion
	}
	if opt.shape.web || opt.shape.id == "json-lines" {
		p.Properties["status_mix"] = opt.statusMix
	}

	m := memo{seed: r.Seed, opt: opt}
	if r.Label {
		// A log has no comment syntax that every reader agrees on, so the
		// label is a line of its own. It is the one line that is not an entry,
		// and it says so in words rather than pretending to be one - except in
		// JSON lines, where a comment is not a line any reader would take.
		line := opt.shape.label(core.Label("log", r.Bytes, r.Seed), opt)
		// It has to leave room for at least one whole entry, or the file would
		// be a label and nothing else.
		if int64(len(line))+min <= r.Bytes {
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

	p.Properties[format.PropertyLabelEmbedded] = m.labelLine != ""
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
	rec := &entries{st: state{rng: rng, clock: newClock(m.opt), opt: m.opt}}
	return core.FillRecords(ctx, w, rng, remaining, rec)
}

// entries is the log seen as a stream of records.
type entries struct{ st state }

func (e *entries) Shortest() int64 { return e.st.opt.shape.shortest(e.st.opt) }

func (e *entries) Append(dst []byte, _ *rand.Rand) []byte {
	return e.st.opt.shape.appendTo(dst, &e.st, -1)
}

func (e *entries) AppendExact(dst []byte, _ *rand.Rand, n int64) []byte {
	return e.st.opt.shape.appendTo(dst, &e.st, n)
}

// Discard puts the clock back. The filler builds one entry past the end to
// measure it and throws it away, and without this the file would skip a tick -
// which nothing else here could see, because the size stays exact and every
// line still parses. csv counts rows and puts them back for the same reason.
func (e *entries) Discard() { e.st.clock.back() }

// minimumBytes is the floor the registry announces: the default shape with
// nothing set. Computed rather than written down, so it cannot drift away from
// the templates the way a number in a document would.
func minimumBytes() int64 {
	o := defaultOptions()
	return o.shape.shortest(o)
}

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

// syslogHost and tags are what the message shapes say produced the line. Taken
// from the shape of a real rsyslog file rather than invented.
const syslogHost = "app-01"

// The range a process id is drawn from. Named rather than written into
// the draw, because the widest of them is what the minimum entry has to
// leave room for and the two must not drift apart.
const (
	minPid = 100
	maxPid = 9998
)

var tags = []string{"sshd", "cron", "systemd", "kernel", "nginx", "dockerd"}

var levels = []string{"INFO", "INFO", "INFO", "WARN", "ERROR", "DEBUG"}
