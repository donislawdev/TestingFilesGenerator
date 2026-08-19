// Package xlsx generates Excel workbooks.
//
// One sheet, inline strings rather than a shared string table. A shared table
// is what a real writer uses because it saves space when text repeats, and it
// costs a second part plus a level of indirection that buys this generator
// nothing: the text here is filler, and the size is settled by the padding
// part anyway.
//
// The packaging and the measured padding limit live in internal/format/opc,
// shared with docx and pptx.
package xlsx

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

	nsSheet = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
	nsRel   = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"

	ctWorkbook  = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"
	ctWorksheet = "application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"

	minRows = 1
	// The format itself stops at 1 048 576 rows. This ceiling is lower because
	// the sheet is built in memory before it is packaged, and a million rows of
	// inline text is about 80 MB of XML before compression.
	maxRows = 200_000

	minColumns = 1
	// Sixteen thousand three hundred and eighty four is the format's own limit.
	// This one is the width a person would actually look at.
	maxColumns = 64

	// The sheet is held in memory while the package is built, so the pair is
	// bounded as well as each side.
	maxCells = 2_000_000
)

func init() {
	format.Register(format.Descriptor{
		ID:          "xlsx",
		Extension:   ".xlsx",
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,

		MinBytes: minimumBytes(),

		Padding: format.PaddingChannel{
			Name:     "an extra part inside the package, with the archive comment for the last few bytes",
			Where:    format.PlacementEnd,
			Capacity: 0,
		},
		Label:  format.LabelVisible,
		Oracle: "libreoffice",
		Properties: []format.Property{
			{
				Name: "rows", Kind: format.PropertyInt,
				Min: minRows, Max: maxRows, Unit: "rows",
				Default: "1",
				Detail:  "How many rows of cells the sheet holds. The rest of the size you asked for is carried by a part of the package that is not shown.",
			},
			{
				Name: "columns", Kind: format.PropertyInt,
				Min: minColumns, Max: maxColumns, Unit: "columns",
				Default: "1",
				Detail:  "How many columns each row has.",
			},
		},
		JointLimits: []format.JointLimit{{
			Of: "rows", By: "columns", Max: maxCells,
			Unit: "million cells", Per: 1_000_000,
			Why: "the sheet is built in memory before it is packaged",
		}},
		GeneratorVersion: generatorVersion,
		Generator:        generator{},
	})
}

type generator struct{}

type memo struct {
	rows, columns int
	pkg           opc.Package
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	label := ""
	if r.Label {
		label = core.Label("xlsx", r.Bytes, r.Seed)
	}

	rows, err := intProperty(r.Properties, "rows", 1, minRows, maxRows)
	if err != nil {
		return format.Plan{}, err
	}
	columns, err := intProperty(r.Properties, "columns", 1, minColumns, maxColumns)
	if err != nil {
		return format.Plan{}, err
	}
	if err := checkJointLimits(rows, columns); err != nil {
		return format.Plan{}, err
	}

	built := parts(rows, columns, r.Seed, label)
	shape, err := opc.Measure(built)
	if err != nil {
		return format.Plan{}, err
	}

	pkg, err := opc.Settle(built, shape, r.Bytes)
	if err != nil {
		return format.Plan{}, refusal(err, r.Bytes, shape, rows, columns)
	}
	pkg.Seed = r.Seed

	return format.Plan{
		Bytes:       r.Bytes,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties: map[string]any{
			"rows":           rows,
			"columns":        columns,
			"sheets":         1,
			"parts":          len(built),
			"label_embedded": label != "",
			"padding_part":   opc.FillerName,
		},
		Memo: memo{rows: rows, columns: columns, pkg: pkg},
	}, nil
}

func checkJointLimits(rows, columns int) error {
	d, err := format.Get("xlsx")
	if err != nil {
		return err
	}
	for _, j := range d.JointLimits {
		if bad := j.Allows(int64(rows), int64(columns)); bad != "" {
			return &format.PropertyValueError{
				Format: "xlsx", Key: j.Of + " and " + j.By,
				Value:  fmt.Sprintf("%dx%d", rows, columns),
				Reason: bad + ". Ask for fewer rows or fewer columns",
			}
		}
	}
	return nil
}

func refusal(err error, want int64, shape opc.Shape, rows, columns int) error {
	var gap *opc.Unreachable
	if e, ok := err.(*opc.Unreachable); ok {
		gap = e
		return &format.BelowMinimumError{
			Format:    "XLSX",
			Requested: want,
			Minimum:   gap.Above,
			Reason: fmt.Sprintf(
				"the archive comment carries at most %d B and the smallest extra part costs %d B, so nothing between those two is reachable",
				opc.CommentLimit, shape.FillerOverhead),
			Hint: fmt.Sprintf("Ask for %d B or less, or %d B or more.", gap.Below, gap.Above),
		}
	}
	return &format.BelowMinimumError{
		Format:    "XLSX",
		Requested: want,
		Minimum:   shape.Bare,
		Reason: fmt.Sprintf(
			"a sheet of %s by %s already packages to that much, and a workbook cannot leave out its content types or its relationships",
			core.Count(rows, "row", "rows"), core.Count(columns, "column", "columns")),
		Hint: fmt.Sprintf("Ask for %d B or more, or set fewer rows", shape.Bare),
	}
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(memo)
	if !ok {
		return fmt.Errorf("xlsx: the plan was not produced by this generator")
	}
	return opc.Write(ctx, w, m.pkg)
}

func parts(rows, columns int, seed uint64, label string) []opc.Part {
	workbook := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<workbook xmlns="` + nsSheet + `" xmlns:r="` + nsRel + `">` +
		`<sheets><sheet name="tfg" sheetId="1" r:id="rId1"/></sheets></workbook>`

	return []opc.Part{
		{Name: "[Content_Types].xml", Body: opc.ContentTypes([][2]string{
			{"/xl/workbook.xml", ctWorkbook},
			{"/xl/worksheets/sheet1.xml", ctWorksheet},
		})},
		{Name: "_rels/.rels", Body: opc.Rels([][3]string{
			{"rId1", nsRel + "/officeDocument", "xl/workbook.xml"},
		})},
		{Name: "xl/workbook.xml", Body: []byte(workbook)},
		{Name: "xl/_rels/workbook.xml.rels", Body: opc.Rels([][3]string{
			{"rId1", nsRel + "/worksheet", "worksheets/sheet1.xml"},
		})},
		// Stored, so the length of the sheet is exactly its bytes and the floor
		// of this format is one number for every seed - see opc.LineWidth.
		{Name: "xl/worksheets/sheet1.xml", Body: sheet(rows, columns, seed, label), Store: true},
	}
}

func sheet(rows, columns int, seed uint64, label string) []byte {
	rng := core.NewRand(seed)
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<worksheet xmlns="` + nsSheet + `"><sheetData>`)
	for row := 1; row <= rows; row++ {
		b.WriteString(`<row r="` + strconv.Itoa(row) + `">`)
		for col := 1; col <= columns; col++ {
			text := opc.Line(rng, 2)
			// The label goes in the first cell, which is where a person looks
			// and where a spreadsheet reader shows it without scrolling.
			if row == 1 && col == 1 && label != "" {
				text = opc.Text(opc.Fixed(label, opc.LabelWidth))
			}
			b.WriteString(`<c r="` + column(col) + strconv.Itoa(row) + `" t="inlineStr"><is><t xml:space="preserve">`)
			b.WriteString(text)
			b.WriteString(`</t></is></c>`)
		}
		b.WriteString(`</row>`)
	}
	b.WriteString(`</sheetData></worksheet>`)
	return []byte(b.String())
}

// column turns 1 into A and 27 into AA, which is how a spreadsheet names them.
func column(n int) string {
	var out []byte
	for n > 0 {
		n--
		out = append([]byte{byte('A' + n%26)}, out...)
		n /= 26
	}
	return string(out)
}

func intProperty(props map[string]string, key string, fallback, min, max int) (int, error) {
	raw, ok := props[key]
	if !ok || raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("xlsx: %s must be a whole number, got %q", key, raw)
	}
	if n < min || n > max {
		return 0, fmt.Errorf("xlsx: %s must be between %d and %d, got %d", key, min, max, n)
	}
	return n, nil
}

// minimumBytes is the smallest package this generator can produce.
//
// Both label states are measured and the smaller wins, which is not the same
// thing for all three formats: here the label replaces the filler text of the first cell rather
// than adding to it, so the labelled workbook is the SMALLER one. In a Word
// document the label is an extra paragraph and it goes the other way.
func minimumBytes() int64 {
	smallest := int64(1) << 62
	for _, label := range []string{"", core.Label("xlsx", 0, 0)} {
		shape, err := opc.Measure(parts(1, 1, 0, label))
		if err != nil {
			continue
		}
		if shape.Bare < smallest {
			smallest = shape.Bare
		}
	}
	return smallest
}
