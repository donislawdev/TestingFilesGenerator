// Package docx generates Word documents.
//
// The first of the three Office formats, and the first container whose parts
// are fixed by a specification rather than chosen by the caller. A ZIP holds
// user files. A DOCX holds a document, a table of content types and a set of
// relationships, and none of those is optional.
//
// The packaging, the measured padding limit and the size arithmetic live in
// internal/format/opc, shared with xlsx and pptx.
package docx

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/opc"
)

const (
	generatorVersion = "1"

	nsWord = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	nsRel  = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"

	ctDocument = "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"

	minParagraphs = 1
	// A paragraph costs about a hundred bytes of XML, so the ceiling is about
	// what a document of a few megabytes of prose would hold. Past that the
	// padding part carries the size, which is cheaper for everyone.
	maxParagraphs = 50_000

	// wordsPerParagraph keeps a paragraph the length of a sentence rather than
	// a word, so a document opened by a person reads as a document.
	wordsPerParagraph = 12
)

func init() {
	format.Register(format.Descriptor{
		ID:          "docx",
		Extension:   ".docx",
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,

		MinBytes: minimumBytes(),

		Padding: format.PaddingChannel{
			// Measured 2026-08-19 against LibreOffice, the only reader of
			// these packages on the machine this was written on. The archive
			// comment - which the format document named as the channel, at
			// 64 KB - stops working at 513 B. The padding part has no ceiling
			// that anything found.
			Name:     "an extra part inside the package, with the archive comment for the last few bytes",
			Where:    format.PlacementEnd,
			Capacity: 0,
		},
		Label:  format.LabelVisible,
		Oracle: "libreoffice",
		Properties: []format.Property{
			{
				Name: "paragraphs", Kind: format.PropertyInt,
				Min: minParagraphs, Max: maxParagraphs, Unit: "paragraphs",
				Default: "1",
				Detail:  "How many paragraphs of text the document holds. The rest of the size you asked for is carried by a part of the package that is not shown.",
			},
		},
		GeneratorVersion: generatorVersion,
		Generator:        generator{},
	})
}

type generator struct{}

type memo struct {
	paragraphs int
	seed       uint64
	pkg        opc.Package
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	label := ""
	if r.Label {
		label = core.Label("docx", r.Bytes, r.Seed)
	}

	paragraphs, err := intProperty(r.Properties, "paragraphs", 1)
	if err != nil {
		return format.Plan{}, err
	}

	parts := parts(paragraphs, r.Seed, label)
	shape, err := opc.Measure(parts)
	if err != nil {
		return format.Plan{}, err
	}

	pkg, err := opc.Settle(parts, shape, r.Bytes)
	if err != nil {
		return format.Plan{}, refusal(err, r.Bytes, shape, paragraphs)
	}
	pkg.Seed = r.Seed

	return format.Plan{
		Bytes:       r.Bytes,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties: map[string]any{
			"paragraphs":                 paragraphs,
			"parts":                      len(parts),
			format.PropertyLabelEmbedded: label != "",
			"padding_part":               opc.FillerName,
		},
		Memo: memo{paragraphs: paragraphs, seed: r.Seed, pkg: pkg},
	}, nil
}

// refusal turns the packager's arithmetic into the four part message D6 asks
// for, in this format's own words.
func refusal(err error, want int64, shape opc.Shape, paragraphs int) error {
	// The ceiling before the floor, because a package past it is refused by
	// the packager for a reason that has nothing to do with being too small.
	if big, ok := err.(*opc.TooLarge); ok {
		return &format.AboveMaximumError{
			Format:    "DOCX",
			Requested: big.Want,
			Maximum:   big.Ceiling,
			Reason: "an Office file is a ZIP archive, and this build works out its size before it writes it " +
				"in a way that cannot account for the zip64 records a larger one needs",
			Hint: "Ask for less than 4 GiB, or split the content across several files.",
		}
	}
	var gap *opc.Unreachable
	if ok := asUnreachable(err, &gap); ok {
		return &format.BelowMinimumError{
			Format:    "DOCX",
			Requested: want,
			Minimum:   gap.Above,
			Reason: fmt.Sprintf(
				"the archive comment carries at most %d B and the smallest extra part costs %d B, so nothing between those two is reachable",
				opc.CommentLimit, shape.FillerOverhead),
			Hint: fmt.Sprintf("Ask for %d B or less, or %d B or more.", gap.Below, gap.Above),
		}
	}
	return &format.BelowMinimumError{
		Format:    "DOCX",
		Requested: want,
		Minimum:   shape.Bare,
		Reason: fmt.Sprintf(
			"a document of %s already packages to that much, and a Word file cannot leave out its content types or its relationships",
			core.Count(paragraphs, "paragraph", "paragraphs")),
		Hint: fmt.Sprintf("Ask for %d B or more, or set fewer paragraphs", shape.Bare),
	}
}

func asUnreachable(err error, target **opc.Unreachable) bool {
	if e, ok := err.(*opc.Unreachable); ok {
		*target = e
		return true
	}
	return false
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(memo)
	if !ok {
		return fmt.Errorf("docx: the plan was not produced by this generator")
	}
	return opc.Write(ctx, w, m.pkg)
}

// parts is the smallest set a Word document can have: the table of content
// types, the relationship pointing at the document, and the document.
func parts(paragraphs int, seed uint64, label string) []opc.Part {
	return []opc.Part{
		{Name: "[Content_Types].xml", Body: opc.ContentTypes([][2]string{
			{"/word/document.xml", ctDocument},
		})},
		{Name: "_rels/.rels", Body: opc.Rels([][3]string{
			{"rId1", nsRel + "/officeDocument", "word/document.xml"},
		})},
		// Stored rather than compressed, and it is the one part that is. Its
		// length is then exactly its bytes, so the floor of this format is one
		// number for every seed and every requested size - see opc.LineWidth.
		{Name: "word/document.xml", Body: document(paragraphs, seed, label), Store: true},
	}
}

func document(paragraphs int, seed uint64, label string) []byte {
	rng := core.NewRand(seed)
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<w:document xmlns:w="` + nsWord + `"><w:body>`)
	if label != "" {
		b.WriteString(paragraph(opc.Text(opc.Fixed(label, opc.LabelWidth))))
	}
	for i := 0; i < paragraphs; i++ {
		b.WriteString(paragraph(opc.Line(rng, wordsPerParagraph)))
	}
	b.WriteString(`<w:sectPr/></w:body></w:document>`)
	return []byte(b.String())
}

func paragraph(text string) string {
	return `<w:p><w:r><w:t xml:space="preserve">` + text + `</w:t></w:r></w:p>`
}

func intProperty(props map[string]string, key string, fallback int) (int, error) {
	raw, ok := props[key]
	if !ok || raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("docx: %s must be a whole number, got %q", key, raw)
	}
	if n < minParagraphs || n > maxParagraphs {
		return 0, fmt.Errorf("docx: %s must be between %d and %d, got %d", key, minParagraphs, maxParagraphs, n)
	}
	return n, nil
}

// minimumBytes is the smallest package this generator can produce.
//
// Both label states are measured and the smaller wins, which is not the same
// thing for all three formats: here the label is an extra paragraph, so a labelled document is
// the larger one. In a workbook and in a deck the label REPLACES the filler
// text of the first cell or slide, so there the labelled file is smaller.
func minimumBytes() int64 {
	smallest := int64(1) << 62
	for _, label := range []string{"", core.Label("docx", 0, 0)} {
		shape, err := opc.Measure(parts(1, 0, label))
		if err != nil {
			continue
		}
		if shape.Bare < smallest {
			smallest = shape.Bare
		}
	}
	return smallest
}
