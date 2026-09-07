// Package txt generates plain text files.
//
// TXT is the baseline of the whole tool. No compression, no container, no
// encoder - if the exact size promise does not hold here it holds nowhere.
package txt

import (
	"context"
	"fmt"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/textenc"
)

// The padding channel is the content itself, and it has no limit. A text file
// takes as many bytes as you give it, which is why this format is the first
// one built - it isolates the size arithmetic from every other difficulty.

const (
	generatorVersion = "1"

	// lineWidth is where filler text wraps. Long enough to look like prose,
	// short enough that an editor does not scroll sideways.
	lineWidth = 72

	// chunkSize is how much is built before a write. It also sets how often
	// cancellation is noticed, so it stays small enough that interrupting a
	// multi gigabyte file feels immediate.
	chunkSize = 32 * 1024
)

func init() {
	format.Register(format.Descriptor{
		ID:          "txt",
		Extension:   ".txt",
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,

		// An empty text file is a valid text file. TXT is one of the two
		// formats in Tier 1 that can legitimately be zero bytes.
		MinBytes: 0,

		Padding: format.PaddingChannel{
			Name:     "file content",
			Where:    format.PlacementEnd,
			Capacity: 0,
		},
		Label: format.LabelVisible,
		// No reference tool: a reader is handed a path and nothing else, so it
		// would have to GUESS which encoding the file claims - and a checker
		// that guesses agrees with a file written in the wrong one. The layer
		// that can be TOLD is the structural check, and that is where this
		// format's encoding is verified.
		Oracle: format.OracleNone,
		// Line endings and line length come later. Until they do, declaring
		// only what is here is what makes a recipe asking for them fail loudly
		// instead of quietly producing something else.
		Properties:       textenc.Properties(),
		GeneratorVersion: generatorVersion,
		Generator:        generator{},
	})
}

type generator struct{}

// memo carries what writing needs to know, worked out once during planning.
type memo struct {
	labelLine string // includes the trailing newline, empty when absent
	seed      uint64
	codec     textenc.Codec
	// source is how many bytes of ASCII content the file holds, which is the
	// ordered size less the mark and divided by the width of a character.
	// Everything below counts in these rather than in file bytes, so the
	// filling loop is the same loop it always was.
	source int64
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	if r.Bytes < 0 {
		return format.Plan{}, &format.BelowMinimumError{
			Format:    "TXT",
			Requested: r.Bytes,
			Minimum:   0,
			Reason:    "a file cannot hold fewer than zero bytes",
			Hint:      "Ask for 0 B or more.",
		}
	}

	codec, err := textenc.Parse("txt", r.Properties)
	if err != nil {
		return format.Plan{}, err
	}
	if err := codec.Check("TXT", r.Bytes); err != nil {
		return format.Plan{}, err
	}

	p := format.Plan{
		Bytes:       r.Bytes,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties: map[string]any{
			textenc.Setting:    codec.Name(),
			textenc.SettingBOM: codec.HasBOM(),
			"line_ending":      "lf",
			"content":          "english",
		},
	}

	m := memo{seed: r.Seed, codec: codec, source: codec.Source(r.Bytes)}
	if r.Label {
		line := core.Label("txt", r.Bytes, r.Seed) + "\n"
		if int64(len(line)) <= m.source {
			m.labelLine = line
		} else {
			// Silence is banned. The label did not fit, so that has to be
			// visible rather than quietly absent from a file the user
			// believes carries one.
			//
			// The number is what the label COSTS in this encoding, not how
			// long it is to read. In UTF-16 those differ by a factor of two,
			// and a note off by half is worse than no note.
			p.Notes = append(p.Notes, format.Note{
				Code: "label_omitted",
				Detail: fmt.Sprintf(
					"The label needs %d B and the file is %d B, so this file carries no label. Its name and the manifest still identify it.",
					codec.Cost(int64(len(line))), r.Bytes),
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
		return fmt.Errorf("txt: the plan was not produced by this generator")
	}

	// The mark is bytes rather than text, so it goes out as itself. Everything
	// after it is characters, so it goes through the encoder.
	if err := writeAll(w, m.codec.Preamble()); err != nil {
		return err
	}
	w = m.codec.Writer(w)

	remaining := m.source

	if m.labelLine != "" {
		if err := writeAll(w, []byte(m.labelLine)); err != nil {
			return err
		}
		remaining -= int64(len(m.labelLine))
	}

	// The random source belongs to this file and nothing else. A generator
	// reaching for a shared source would make its output depend on the order
	// files happen to be produced in.
	rng := core.NewRand(m.seed)

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

		// Cut to the exact number of bytes still owed. A word may end up
		// clipped, and that is the right trade - the size is the promise.
		if int64(len(buf)) > remaining {
			buf = buf[:remaining]
		}
		// A round that emits nothing would spin here for ever, and a run that
		// hangs is worse than one that fails - nobody can tell it apart from
		// a very large file. Found by mutation testing, not by reasoning.
		if len(buf) == 0 {
			return fmt.Errorf("txt: made no progress with %d B still owed", remaining)
		}

		if err := writeAll(w, buf); err != nil {
			return err
		}
		remaining -= int64(len(buf))
	}
	return nil
}

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

// words is the filler vocabulary. English by default, because a generated
// file is English unless the user asks for another locale.
var words = []string{
	"account", "after", "again", "amount", "answer", "before", "between",
	"branch", "buffer", "build", "cache", "change", "client", "column",
	"config", "content", "create", "default", "delete", "detail", "device",
	"differ", "domain", "during", "effect", "either", "engine", "entry",
	"error", "export", "field", "filter", "folder", "format", "handle",
	"header", "import", "index", "input", "insert", "invoice", "layer",
	"length", "level", "limit", "loader", "market", "matter", "member",
	"method", "module", "notice", "number", "object", "office", "option",
	"output", "packet", "parent", "parser", "period", "policy", "prefix",
	"process", "public", "query", "reason", "record", "region", "report",
	"result", "return", "sample", "schema", "script", "search", "sender",
	"server", "signal", "single", "source", "status", "stream", "string",
	"switch", "system", "target", "thread", "ticket", "toggle", "token",
	"update", "upload", "vendor", "verify", "version", "volume", "window",
	"worker", "wrapper", "writer",
}
