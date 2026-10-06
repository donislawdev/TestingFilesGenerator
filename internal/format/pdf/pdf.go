// Package pdf generates PDF documents.
//
// The highest fidelity bar in Tier 1 - the file has to open in Adobe, not
// merely satisfy a parser.
package pdf

import (
	"context"
	"fmt"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

const generatorVersion = "1"

func init() {
	format.Register(format.Descriptor{
		ID:          "pdf",
		Name:        "Portable Document Format",
		Extension:   ".pdf",
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,
		MinBytes:    minimumBytes(),

		Padding: format.PaddingChannel{
			Name:     "comment block after the trailer",
			Where:    format.PlacementEnd,
			Capacity: 0,
		},
		Label:            format.LabelVisible,
		Oracle:           "pdftotext",
		Properties:       properties,
		GeneratorVersion: generatorVersion,
		Generator:        generator{},
	})
}

type generator struct{}

type memo struct {
	opts  options
	seed  uint64
	label string
	// prefix is everything up to and including the trailer. Small - a few
	// kilobytes per page - so holding it costs nothing next to the padding.
	prefix []byte
	// suffix is startxref, the offset and the closing marker. Its length does
	// not depend on the padding, because the offset it carries points at the
	// cross reference table, which sits before the padding.
	suffix []byte
	padLen int64
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	opts, err := readOptions(r.Properties)
	if err != nil {
		return format.Plan{}, err
	}

	label := ""
	if r.Label {
		label = core.Label("pdf", r.Bytes, r.Seed)
	}

	m := memo{opts: opts, seed: r.Seed, label: label}
	m.prefix, m.suffix = document(m)

	// One number answers both questions: how much padding this file needs, and
	// what is too small to make. Every line of body text is the same width
	// whatever the seed drew, so the smallest document this format can produce
	// is the same for everybody - see lineWidth.
	bare := int64(len(m.prefix) + len(m.suffix))
	floor := bare

	p := format.Plan{
		Bytes:       r.Bytes,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties:  described(m, r.Label),
	}

	switch {
	case r.Bytes == bare:
		m.padLen = 0
	case r.Bytes < floor:
		return format.Plan{}, &format.BelowMinimumError{
			Format:    "PDF",
			Requested: r.Bytes,
			Minimum:   floor,
			Reason:    core.Says("format.AAlreadyNeedsThatMuchBefore", "a %s%s already needs that much before any padding", core.A("Opts", documentWords(opts)), core.A("Carrying", carrying(r.Label, opts))),
			Hint:      core.Says("format.AskForBOrMore2", "Ask for %d B or more%s.", core.A("Floor", floor), core.A("CleanHint", cleanHint(r.Label, opts))),
		}
	case r.Bytes < bare+minComment:
		// A comment is a per cent sign and a newline at the very least, so
		// exactly one byte above the bare document cannot be produced.
		return format.Plan{}, &format.BelowMinimumError{
			Format:    "PDF",
			Requested: r.Bytes,
			Minimum:   bare + minComment,
			Reason:    core.Says("format.ThisDocumentIsExactlyBAnd", "this document is exactly %d B and the shortest comment that could pad it is %d B, so one byte more than the document is the single size in between that cannot be reached", core.A("Bare", bare), core.A("MinComment", minComment)),
			Hint:      core.Says("format.AskForExactlyBOrFor", "Ask for exactly %d B or for %d B or more.", core.A("Bare", bare), core.A("Bare2", bare+minComment)),
		}
	default:
		m.padLen = r.Bytes - bare
	}

	p.Memo = m
	return p, nil
}

// described is what the manifest says about the file: what is in it, so a
// test can assert on it without opening the PDF.
//
// A value the document carries is always here, and a value it does not carry
// is not - an author nobody asked for is absent from both. The alternative,
// only what differs from the defaults, would make every consumer know the
// defaults of this build to read the manifest of it.
func described(m memo, labelled bool) map[string]any {
	o := m.opts
	props := map[string]any{
		"pages":                      o.pages,
		"page_size":                  o.sizeName,
		"orientation":                o.orientation,
		"rotate":                     o.rotate,
		"pdf_version":                o.version,
		"fonts_embedded":             false,
		format.PropertyLabelEmbedded: labelled,
		"compressed":                 false,
		"content_streams":            o.pages,
		"title":                      titleOf(m),
		"producer":                   o.info.producer,
	}
	if o.sizeName == mixed {
		cycle := make([]string, 0, len(o.sizes))
		for _, s := range o.sizes {
			cycle = append(cycle, s.name)
		}
		props["page_size_cycle"] = cycle
	}
	for key, value := range map[string]string{
		"author": o.info.author, "subject": o.info.subject,
		"keywords": o.info.keywords, "creator": o.info.creator,
	} {
		if value != "" {
			props[key] = value
		}
	}
	for key, d := range map[string]date{"created": o.info.created, "modified": o.info.modified} {
		if d.pdf != "" {
			props[key] = d.asked
		}
	}
	return props
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(memo)
	if !ok {
		return core.Defect(fmt.Errorf("pdf: the plan was not produced by this generator"))
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if _, err := w.Write(m.prefix); err != nil {
		return err
	}
	if err := writeComment(ctx, w, m.seed, m.padLen); err != nil {
		return err
	}
	_, err := w.Write(m.suffix)
	return err
}
