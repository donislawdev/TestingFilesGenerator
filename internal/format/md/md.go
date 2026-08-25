// Package md generates Markdown documents.
//
// The point of a Markdown fixture is the structure - headings, lists, tables,
// fenced code - because that is what a renderer or a converter under test has
// to cope with. A file of prose with a .md extension tests nothing that TXT
// does not.
package md

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"strconv"
	"unicode"
	"unicode/utf8"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// The padding channel is the content, measured 2026-08-01 like the rest of the
// text group. It has no ceiling.
//
// What the measurement did not settle, and this file does: where the cut
// lands. Filling to an exact byte count means the last thing written is
// truncated, and truncating a fenced code block leaves the fence unclosed -
// still renderable, but a document that says something other than it meant.
// So structure is written only while a whole block fits, and the remainder is
// prose. A paragraph cut anywhere is still a paragraph.

const (
	generatorVersion = "1"

	lineWidth = 72
	chunkSize = 32 * 1024
)

func init() {
	format.Register(format.Descriptor{
		ID:          "md",
		Extension:   ".md",
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,

		// An empty file is a valid Markdown document, the same as for text.
		MinBytes: 0,

		Padding: format.PaddingChannel{
			Name:     "document content",
			Where:    format.PlacementEnd,
			Capacity: 0,
		},
		Label:  format.LabelVisible,
		Oracle: format.OracleNone,
		// Heading depth, table size and which elements appear come later.
		// Declaring none now is what makes a recipe asking for them fail
		// loudly rather than quietly producing something else.
		Properties:       nil,
		GeneratorVersion: generatorVersion,
		Generator:        generator{},
	})
}

type generator struct{}

type memo struct {
	labelLine string // includes the trailing blank line, empty when absent
	seed      uint64
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	if r.Bytes < 0 {
		return format.Plan{}, &format.BelowMinimumError{
			Format:    "MD",
			Requested: r.Bytes,
			Minimum:   0,
			Reason:    "a file cannot hold fewer than zero bytes",
			Hint:      "Ask for 0 B or more.",
		}
	}

	p := format.Plan{
		Bytes:       r.Bytes,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties: map[string]any{
			"encoding":    "utf-8",
			"line_ending": "lf",
			"flavour":     "commonmark",
		},
	}

	m := memo{seed: r.Seed}
	if r.Label {
		// A plain line followed by a blank one is a paragraph, which renders
		// visibly and cannot break the document wherever it sits.
		line := core.Label("md", r.Bytes, r.Seed) + "\n\n"
		if int64(len(line)) <= r.Bytes {
			m.labelLine = line
		} else {
			p.Notes = append(p.Notes, format.Note{
				Code: "label_omitted",
				Detail: fmt.Sprintf(
					"The label needs %d B and the file is %d B, so this file carries no label. Its name and the manifest still identify it.",
					len(line), r.Bytes),
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
		return fmt.Errorf("md: the plan was not produced by this generator")
	}

	remaining := p.Bytes
	if m.labelLine != "" {
		if err := writeAll(w, []byte(m.labelLine)); err != nil {
			return err
		}
		remaining -= int64(len(m.labelLine))
	}

	rng := core.NewRand(m.seed)

	// One buffer, reused. A block built into its own allocation costs a
	// multiple of the file size in garbage over a large document, and the
	// resource guard measures exactly that.
	buf := make([]byte, 0, chunkSize+1024)

	// Whole blocks while one still fits. The moment one does not, the tail
	// becomes prose, which is the element that survives being cut anywhere.
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		mark := len(buf)
		buf = appendBlock(buf, rng)
		if int64(len(buf)-mark) > remaining {
			buf = buf[:mark]
			break
		}
		remaining -= int64(len(buf) - mark)

		if len(buf) >= chunkSize {
			if err := writeAll(w, buf); err != nil {
				return err
			}
			buf = buf[:0]
		}
	}
	if len(buf) > 0 {
		if err := writeAll(w, buf); err != nil {
			return err
		}
	}

	return fillProse(ctx, w, rng, remaining)
}

// appendBlock appends one complete Markdown element, newline terminated and
// followed by a blank line so the next block starts cleanly.
func appendBlock(dst []byte, rng *rand.Rand) []byte {
	switch rng.IntN(6) {
	case 0:
		dst = append(dst, "## "...)
		dst = appendTitle(dst, rng, 3)
		return append(dst, "\n\n"...)
	case 1:
		for i, n := 0, 3+rng.IntN(3); i < n; i++ {
			dst = append(dst, "- "...)
			dst = appendPhrase(dst, rng, 4+rng.IntN(4))
			dst = append(dst, '\n')
		}
		return append(dst, '\n')
	case 2:
		dst = append(dst, "| id | field | value |\n|---|---|---|\n"...)
		for i, n := 0, 2+rng.IntN(3); i < n; i++ {
			dst = append(dst, "| "...)
			dst = strconv.AppendInt(dst, int64(i+1), 10)
			dst = append(dst, " | "...)
			dst = append(dst, word(rng)...)
			dst = append(dst, " | "...)
			dst = append(dst, word(rng)...)
			dst = append(dst, " |\n"...)
		}
		return append(dst, '\n')
	case 3:
		dst = append(dst, "```text\n"...)
		for i, n := 0, 2+rng.IntN(3); i < n; i++ {
			dst = appendPhrase(dst, rng, 5)
			dst = append(dst, '\n')
		}
		return append(dst, "```\n\n"...)
	case 4:
		dst = append(dst, "> "...)
		dst = appendPhrase(dst, rng, 8)
		return append(dst, "\n\n"...)
	default:
		dst = appendWrapped(dst, rng, 20+rng.IntN(20))
		return append(dst, "\n\n"...)
	}
}

func appendPhrase(dst []byte, rng *rand.Rand, n int) []byte {
	for i := 0; i < n; i++ {
		if i > 0 {
			dst = append(dst, ' ')
		}
		dst = append(dst, word(rng)...)
	}
	return dst
}

// appendTitle is a phrase with each word capitalised, for a heading.
func appendTitle(dst []byte, rng *rand.Rand, n int) []byte {
	for i := 0; i < n; i++ {
		if i > 0 {
			dst = append(dst, ' ')
		}
		w := word(rng)
		// The first character, not the first byte, and appended without
		// building a string to throw away.
		//
		// strings.ToUpper(w[:1]) allocated once per word of every heading -
		// measured at 91% of everything this generator allocates, 163842
		// objects for a 16 MiB document. It also took the first BYTE, so a
		// vocabulary that is not ASCII would silently stop being capitalised.
		r, size := utf8.DecodeRuneInString(w)
		dst = utf8.AppendRune(dst, unicode.ToUpper(r))
		dst = append(dst, w[size:]...)
	}
	return dst
}

// appendWrapped is a paragraph broken at the same width the prose filler uses,
// so a paragraph written as a block and one written as filler look alike.
func appendWrapped(dst []byte, rng *rand.Rand, n int) []byte {
	col := 0
	for i := 0; i < n; i++ {
		w := word(rng)
		switch {
		case col == 0:
		case col+len(w)+1 > lineWidth:
			dst = append(dst, '\n')
			col = 0
		default:
			dst = append(dst, ' ')
			col++
		}
		dst = append(dst, w...)
		col += len(w)
	}
	return dst
}

// fillProse writes plain paragraph text up to the exact byte still owed. A
// paragraph is the one element that survives being cut anywhere.
func fillProse(ctx context.Context, w io.Writer, rng *rand.Rand, remaining int64) error {
	buf := make([]byte, 0, chunkSize+16)
	col := 0
	for remaining > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		buf = buf[:0]
		for len(buf) < chunkSize && remaining > int64(len(buf)) {
			word := words[rng.IntN(len(words))]
			if col+len(word)+1 > lineWidth {
				buf = append(buf, '\n')
				col = 0
			} else if col > 0 {
				buf = append(buf, ' ')
				col++
			}
			buf = append(buf, word...)
			col += len(word)
		}

		if int64(len(buf)) > remaining {
			buf = buf[:remaining]
		}
		// A round emitting nothing would spin for ever, and a run that hangs
		// cannot be told apart from a very large file.
		if len(buf) == 0 {
			return fmt.Errorf("md: made no progress with %d B still owed", remaining)
		}

		if err := writeAll(w, buf); err != nil {
			return err
		}
		remaining -= int64(len(buf))
	}
	return nil
}

func word(rng *rand.Rand) string { return words[rng.IntN(len(words))] }

func writeAll(w io.Writer, b []byte) error {
	for len(b) > 0 {
		n, err := w.Write(b)
		if err != nil {
			return err
		}
		b = b[n:]
	}
	return nil
}

// words is the filler vocabulary, English by default like the rest of the
// text group.
var words = []string{
	"account", "after", "amount", "answer", "before", "between", "branch",
	"buffer", "build", "cache", "change", "client", "column", "config",
	"content", "create", "default", "delete", "detail", "device", "domain",
	"effect", "engine", "entry", "error", "export", "field", "filter",
	"folder", "format", "handle", "header", "import", "index", "input",
	"insert", "invoice", "layer", "length", "level", "limit", "loader",
	"market", "member", "method", "module", "notice", "number", "object",
	"option", "output", "packet", "parent", "parser", "period", "policy",
	"prefix", "process", "public", "query", "reason", "record", "region",
	"report", "result", "return", "sample", "schema", "script", "search",
	"sender", "server", "signal", "single", "source", "status", "stream",
	"string", "switch", "system", "target", "thread", "ticket", "toggle",
	"token", "update", "upload", "vendor", "verify", "version", "volume",
	"window", "worker", "writer",
}
