// Package pptx generates PowerPoint presentations.
//
// The most demanding of the three Office formats, and not because of its size.
// A one slide deck needs eleven parts where a Word document needs three: a
// presentation, a slide master, a slide layout, a theme, the slide itself and
// a relationship file for each of them. Leave any of them out and no reader
// opens the file.
//
// Measured 2026-08-19: that package is 4340 B. The format document called PPTX
// the highest minimum in Tier 1 at fifteen to twenty five kilobytes, which was
// an estimate written before anybody built one. It is the highest in PARTS,
// not in bytes - see docs/MVP-FORMATS.md section 2.9.
package pptx

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

	nsDraw = "http://schemas.openxmlformats.org/drawingml/2006/main"
	nsRel  = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	nsPres = "http://schemas.openxmlformats.org/presentationml/2006/main"

	ctBase         = "application/vnd.openxmlformats-officedocument.presentationml"
	ctPresentation = ctBase + ".presentation.main+xml"
	ctSlideMaster  = ctBase + ".slideMaster+xml"
	ctSlideLayout  = ctBase + ".slideLayout+xml"
	ctSlide        = ctBase + ".slide+xml"
	ctTheme        = "application/vnd.openxmlformats-officedocument.theme+xml"

	minSlides = 1
	// Each slide is a part and a relationship file, both held in memory while
	// the package is built. Five hundred is more slides than anybody presents
	// and about a megabyte of XML.
	maxSlides = 500

	// A slide is 10 by 7.5 inches in English Metric Units, which is what the
	// format counts in. Everything below is placed in the same units.
	slideWidth  = 9144000
	slideHeight = 6858000
)

func init() {
	format.Register(format.Descriptor{
		ID:          "pptx",
		Extension:   ".pptx",
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
				Name: "slides", Kind: format.PropertyInt,
				Min: minSlides, Max: maxSlides, Unit: "slides",
				Default: "1",
				Detail:  "How many slides the presentation holds. The rest of the size you asked for is carried by a part of the package that is not shown.",
			},
		},
		GeneratorVersion: generatorVersion,
		Generator:        generator{},
	})
}

type generator struct{}

type memo struct {
	slides int
	pkg    opc.Package
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	label := ""
	if r.Label {
		label = core.Label("pptx", r.Bytes, r.Seed)
	}

	slides, err := intProperty(r.Properties, "slides", 1)
	if err != nil {
		return format.Plan{}, err
	}

	built := parts(slides, r.Seed, label)
	shape, err := opc.Measure(built)
	if err != nil {
		return format.Plan{}, err
	}

	pkg, err := opc.Settle(built, shape, r.Bytes)
	if err != nil {
		return format.Plan{}, refusal(err, r.Bytes, shape, slides)
	}
	pkg.Seed = r.Seed

	return format.Plan{
		Bytes:       r.Bytes,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties: map[string]any{
			"slides":         slides,
			"parts":          len(built),
			"label_embedded": label != "",
			"padding_part":   opc.FillerName,
		},
		Memo: memo{slides: slides, pkg: pkg},
	}, nil
}

func refusal(err error, want int64, shape opc.Shape, slides int) error {
	if gap, ok := err.(*opc.Unreachable); ok {
		return &format.BelowMinimumError{
			Format:    "PPTX",
			Requested: want,
			Minimum:   gap.Above,
			Reason: fmt.Sprintf(
				"the archive comment carries at most %d B and the smallest extra part costs %d B, so nothing between those two is reachable",
				opc.CommentLimit, shape.FillerOverhead),
			Hint: fmt.Sprintf("Ask for %d B or less, or %d B or more.", gap.Below, gap.Above),
		}
	}
	return &format.BelowMinimumError{
		Format:    "PPTX",
		Requested: want,
		Minimum:   shape.Bare,
		Reason: fmt.Sprintf(
			"a deck of %s already packages to that much - a presentation needs a master, a layout and a theme whether it shows them or not",
			core.Count(slides, "slide", "slides")),
		Hint: fmt.Sprintf("Ask for %d B or more, or set fewer slides", shape.Bare),
	}
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(memo)
	if !ok {
		return fmt.Errorf("pptx: the plan was not produced by this generator")
	}
	return opc.Write(ctx, w, m.pkg)
}

func parts(slides int, seed uint64, label string) []opc.Part {
	rng := core.NewRand(seed)

	overrides := [][2]string{
		{"/ppt/presentation.xml", ctPresentation},
		{"/ppt/slideMasters/slideMaster1.xml", ctSlideMaster},
		{"/ppt/slideLayouts/slideLayout1.xml", ctSlideLayout},
		{"/ppt/theme/theme1.xml", ctTheme},
	}
	presRels := [][3]string{
		{"rId1", nsRel + "/slideMaster", "slideMasters/slideMaster1.xml"},
	}
	for i := 1; i <= slides; i++ {
		name := "/ppt/slides/slide" + strconv.Itoa(i) + ".xml"
		overrides = append(overrides, [2]string{name, ctSlide})
		presRels = append(presRels, [3]string{
			"rId" + strconv.Itoa(i+1), nsRel + "/slide",
			"slides/slide" + strconv.Itoa(i) + ".xml",
		})
	}

	out := []opc.Part{
		{Name: "[Content_Types].xml", Body: opc.ContentTypes(overrides)},
		{Name: "_rels/.rels", Body: opc.Rels([][3]string{
			{"rId1", nsRel + "/officeDocument", "ppt/presentation.xml"},
		})},
		{Name: "ppt/presentation.xml", Body: presentation(slides)},
		{Name: "ppt/_rels/presentation.xml.rels", Body: opc.Rels(presRels)},
		{Name: "ppt/slideMasters/slideMaster1.xml", Body: slideMaster()},
		{Name: "ppt/slideMasters/_rels/slideMaster1.xml.rels", Body: opc.Rels([][3]string{
			{"rId1", nsRel + "/slideLayout", "../slideLayouts/slideLayout1.xml"},
			{"rId2", nsRel + "/theme", "../theme/theme1.xml"},
		})},
		{Name: "ppt/slideLayouts/slideLayout1.xml", Body: slideLayout()},
		{Name: "ppt/slideLayouts/_rels/slideLayout1.xml.rels", Body: opc.Rels([][3]string{
			{"rId1", nsRel + "/slideMaster", "../slideMasters/slideMaster1.xml"},
		})},
		{Name: "ppt/theme/theme1.xml", Body: theme()},
	}

	for i := 1; i <= slides; i++ {
		text := opc.Line(rng, 6)
		if i == 1 && label != "" {
			text = opc.Text(opc.Fixed(label, opc.LabelWidth))
		}
		out = append(out,
			opc.Part{
				// Stored, so the length of a slide is exactly its bytes and the
				// floor of this format is one number for every seed - see
				// opc.LineWidth.
				Name: "ppt/slides/slide" + strconv.Itoa(i) + ".xml",
				Body: slide(text), Store: true,
			},
			opc.Part{
				Name: "ppt/slides/_rels/slide" + strconv.Itoa(i) + ".xml.rels",
				Body: opc.Rels([][3]string{
					{"rId1", nsRel + "/slideLayout", "../slideLayouts/slideLayout1.xml"},
				}),
			})
	}
	return out
}

const decl = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`

func namespaces() string {
	return `xmlns:a="` + nsDraw + `" xmlns:r="` + nsRel + `" xmlns:p="` + nsPres + `"`
}

// emptyTree is the shape group every slide, layout and master must have, even
// when it holds nothing.
const emptyTree = `<p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/>` +
	`<p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/></p:spTree>`

// clrMap ties the master's colour slots to the theme's. Every one of the
// twelve attributes is required.
const clrMap = `<p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" ` +
	`accent1="accent1" accent2="accent2" accent3="accent3" accent4="accent4" ` +
	`accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>`

func presentation(slides int) []byte {
	var b strings.Builder
	b.WriteString(decl)
	b.WriteString(`<p:presentation ` + namespaces() + `>`)
	b.WriteString(`<p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst>`)
	b.WriteString(`<p:sldIdLst>`)
	for i := 1; i <= slides; i++ {
		b.WriteString(`<p:sldId id="` + strconv.Itoa(255+i) + `" r:id="rId` + strconv.Itoa(i+1) + `"/>`)
	}
	b.WriteString(`</p:sldIdLst>`)
	b.WriteString(`<p:sldSz cx="` + strconv.Itoa(slideWidth) + `" cy="` + strconv.Itoa(slideHeight) + `"/>`)
	b.WriteString(`<p:notesSz cx="` + strconv.Itoa(slideHeight) + `" cy="` + strconv.Itoa(slideWidth) + `"/>`)
	b.WriteString(`</p:presentation>`)
	return []byte(b.String())
}

func slideMaster() []byte {
	return []byte(decl + `<p:sldMaster ` + namespaces() + `><p:cSld>` + emptyTree + `</p:cSld>` +
		clrMap +
		`<p:sldLayoutIdLst><p:sldLayoutId id="2147483649" r:id="rId1"/></p:sldLayoutIdLst>` +
		`</p:sldMaster>`)
}

func slideLayout() []byte {
	return []byte(decl + `<p:sldLayout ` + namespaces() + ` type="blank">` +
		`<p:cSld name="Blank">` + emptyTree + `</p:cSld></p:sldLayout>`)
}

func slide(text string) []byte {
	shape := `<p:sp><p:nvSpPr><p:cNvPr id="2" name="tfg"/><p:cNvSpPr txBox="1"/>` +
		`<p:nvPr/></p:nvSpPr><p:spPr><a:xfrm>` +
		`<a:off x="685800" y="2130425"/><a:ext cx="7772400" cy="1470025"/>` +
		`</a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>` +
		`<p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r>` +
		`<a:t>` + text + `</a:t></a:r></a:p></p:txBody></p:sp>`
	tree := `<p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/>` +
		`<p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/>` + shape + `</p:spTree>`
	return []byte(decl + `<p:sld ` + namespaces() + `><p:cSld>` + tree + `</p:cSld></p:sld>`)
}

// theme is the smallest one the specification allows: twelve colours, two font
// pairs, and three each of fill, line, effect and background styles. None of it
// is optional and a reader refuses the deck without it.
func theme() []byte {
	colours := [][2]string{
		{"dk1", "000000"}, {"lt1", "FFFFFF"}, {"dk2", "44546A"}, {"lt2", "E7E6E6"},
		{"accent1", "4472C4"}, {"accent2", "ED7D31"}, {"accent3", "A5A5A5"},
		{"accent4", "FFC000"}, {"accent5", "5B9BD5"}, {"accent6", "70AD47"},
		{"hlink", "0563C1"}, {"folHlink", "954F72"},
	}
	var b strings.Builder
	b.WriteString(decl)
	b.WriteString(`<a:theme xmlns:a="` + nsDraw + `" name="tfg"><a:themeElements>`)
	b.WriteString(`<a:clrScheme name="tfg">`)
	for _, c := range colours {
		b.WriteString(`<a:` + c[0] + `><a:srgbClr val="` + c[1] + `"/></a:` + c[0] + `>`)
	}
	b.WriteString(`</a:clrScheme><a:fontScheme name="tfg">`)
	for _, which := range []string{"major", "minor"} {
		// A typeface is named, not carried. No font travels with these files,
		// which is what keeps LICENSING.md section 6 out of this entirely - a
		// reader without the face substitutes its own.
		b.WriteString(`<a:` + which + `Font><a:latin typeface="Calibri"/>` +
			`<a:ea typeface=""/><a:cs typeface=""/></a:` + which + `Font>`)
	}
	b.WriteString(`</a:fontScheme><a:fmtScheme name="tfg">`)
	fill := `<a:solidFill><a:schemeClr val="phClr"/></a:solidFill>`
	line := `<a:ln><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln>`
	effect := `<a:effectStyle><a:effectLst/></a:effectStyle>`
	b.WriteString(`<a:fillStyleLst>` + strings.Repeat(fill, 3) + `</a:fillStyleLst>`)
	b.WriteString(`<a:lnStyleLst>` + strings.Repeat(line, 3) + `</a:lnStyleLst>`)
	b.WriteString(`<a:effectStyleLst>` + strings.Repeat(effect, 3) + `</a:effectStyleLst>`)
	b.WriteString(`<a:bgFillStyleLst>` + strings.Repeat(fill, 3) + `</a:bgFillStyleLst>`)
	b.WriteString(`</a:fmtScheme></a:themeElements></a:theme>`)
	return []byte(b.String())
}

func intProperty(props map[string]string, key string, fallback int) (int, error) {
	raw, ok := props[key]
	if !ok || raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("pptx: %s must be a whole number, got %q", key, raw)
	}
	if n < minSlides || n > maxSlides {
		return 0, fmt.Errorf("pptx: %s must be between %d and %d, got %d", key, minSlides, maxSlides, n)
	}
	return n, nil
}

// minimumBytes is the smallest package this generator can produce.
//
// Both label states are measured and the smaller wins, which is not the same
// thing for all three formats: here the label replaces the filler text of the first slide rather
// than adding to it, so the labelled deck is the SMALLER one. In a Word
// document the label is an extra paragraph and it goes the other way.
func minimumBytes() int64 {
	smallest := int64(1) << 62
	for _, label := range []string{"", core.Label("pptx", 0, 0)} {
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
