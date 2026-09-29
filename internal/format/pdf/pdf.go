// Package pdf generates PDF documents.
//
// The highest fidelity bar in Tier 1 - the file has to open in Adobe, not
// merely satisfy a parser.
package pdf

import (
	"context"
	"fmt"
	"io"
	"strconv"

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
		Label:  format.LabelVisible,
		Oracle: "pdftotext",
		Properties: []format.Property{
			{
				Name: "pages", Kind: format.PropertyInt,
				Min: 1, Max: maxPages,
				Default: strconv.Itoa(defaultPages),
				Detail:  "How many pages the document has.",
			},
			{
				Name: "page_size", Kind: format.PropertyChoice,
				// Written out rather than read from the map, so one build
				// cannot offer a different set from the next. The ORDER is no
				// longer decided here: registration sorts every closed set, so
				// the menu in the window, "tfg formats pdf" and the wording of
				// a refusal all list them the same way round.
				Choices: []string{"a4", "a3", "a5", "letter", "legal"},
				Default: "a4",
				Detail:  "The paper size every page uses.",
			},
		},
		GeneratorVersion: generatorVersion,
		Generator:        generator{},
	})
}

type generator struct{}

type memo struct {
	pages    int
	pageSize pageSize
	seed     uint64
	label    string
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
	pages, err := pageCount(r.Properties)
	if err != nil {
		return format.Plan{}, err
	}
	size, err := paperSize(r.Properties)
	if err != nil {
		return format.Plan{}, err
	}

	label := ""
	if r.Label {
		label = core.Label("pdf", r.Bytes, r.Seed)
	}

	m := memo{pages: pages, pageSize: size, seed: r.Seed, label: label}
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
		Properties: map[string]any{
			"pages":                      pages,
			"page_size":                  size.name,
			"pdf_version":                "1.7",
			"fonts_embedded":             false,
			format.PropertyLabelEmbedded: r.Label,
			"compressed":                 false,
			"content_streams":            pages,
		},
	}

	switch {
	case r.Bytes == bare:
		m.padLen = 0
	case r.Bytes < floor:
		return format.Plan{}, &format.BelowMinimumError{
			Format:    "PDF",
			Requested: r.Bytes,
			Minimum:   floor,
			Reason: fmt.Sprintf("a %d page %s document%s already needs that much before any padding",
				pages, size.name, labelCost(r.Label)),
			Hint: fmt.Sprintf("Ask for %d B or more%s.", floor, cleanHint(r.Label)),
		}
	case r.Bytes < bare+minComment:
		// A comment is a per cent sign and a newline at the very least, so
		// exactly one byte above the bare document cannot be produced.
		return format.Plan{}, &format.BelowMinimumError{
			Format:    "PDF",
			Requested: r.Bytes,
			Minimum:   bare + minComment,
			Reason: fmt.Sprintf(
				"this document is exactly %d B and the shortest comment that could pad it is %d B, so one byte more than the document is the single size in between that cannot be reached",
				bare, minComment),
			Hint: fmt.Sprintf("Ask for exactly %d B or for %d B or more.", bare, bare+minComment),
		}
	default:
		m.padLen = r.Bytes - bare
	}

	p.Memo = m
	return p, nil
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(memo)
	if !ok {
		return fmt.Errorf("pdf: the plan was not produced by this generator")
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
