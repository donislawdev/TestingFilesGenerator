// Package rtf generates Rich Text Format documents.
//
// Analysed, measured with five readers and decided on 2026-10-15
// (docs/RTF-2026-10-15.md). The padding channel is the words of the closing
// paragraph: every reader shows them, a system that writes the document again
// keeps them, and a file of a hundred megabytes is then a document that long -
// which is what costs a reader, and what the format card says. A group every
// reader has to skip held the size as well, and a system that rewrites RTF
// throws it away, the argument that decided against the epilogue of a mail
// message.
//
// The readers are tolerant by design: of twelve broken files, the one
// LibreOffice, pandoc and RichEdit all refused was a document without its
// closing brace. So the structure is held by the structural checker, and this
// generator never ends its text in a way that could take the brace away.
package rtf

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
		ID:          "rtf",
		Name:        "Rich Text Format",
		Extension:   ".rtf",
		MediaType:   "application/rtf", // IANA, registered by Paul Lindner, magic number {\rtf
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,

		// The header and the font table, which is a whole document with no
		// paragraph in it - every one of the five readers opened it.
		MinBytes: minimumBytes(),

		Padding: format.PaddingChannel{
			Name:     "the words of the closing paragraph",
			Where:    format.PlacementEnd,
			Capacity: 0,
		},
		Label:            format.LabelVisible,
		Oracle:           "pandoc-rtf",
		Properties:       properties(),
		GeneratorVersion: generatorVersion,
		Generator:        generator{},
	})
}

type generator struct{}

type memo struct {
	settings settings
	seed     uint64
	// labelLine is the paragraph that carries the label, empty when absent.
	labelLine string
	pictures  []pictureLine
	// pictureBytes is what the paragraphs of the pictures take, together.
	pictureBytes int64
}

func (g generator) Plan(r format.Request) (format.Plan, error) {
	return g.PlanContext(context.Background(), r)
}

// PlanContext is Plan, stoppable between one picture and the next.
func (generator) PlanContext(ctx context.Context, r format.Request) (format.Plan, error) {
	s, err := settingsOf(r.Properties)
	if err != nil {
		return format.Plan{}, err
	}
	pictures, pictureBytes, err := planPictures(ctx, r, s)
	if err != nil {
		return format.Plan{}, err
	}
	min := s.layout().fixedBytes() + pictureBytes
	if r.Bytes < min {
		return format.Plan{}, belowMinimum(s, r.Bytes, min)
	}

	p := format.Plan{
		Bytes:       r.Bytes,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties:  s.manifest(),
	}
	describePictures(p.Properties, s.pictures, pictures)
	m := memo{settings: s, seed: r.Seed, pictures: pictures, pictureBytes: pictureBytes}
	if r.Label {
		line := labelLine(core.Label("rtf", r.Bytes, r.Seed))
		if min+int64(len(line)) <= r.Bytes {
			m.labelLine = line
		} else {
			p.Notes = append(p.Notes, format.Note{
				Code:   "label_omitted",
				Detail: core.Says("format.TheLabelNeedsBAndThe", "The label needs %d B and the file is %d B, so this file carries no label. Its name and the manifest still identify it.", core.A("Length", len(line)), core.A("Bytes", r.Bytes)),
			})
		}
	}
	p.Properties[format.PropertyLabelEmbedded] = m.labelLine != ""
	p.Memo = m
	return p, nil
}

// labelLine is the paragraph of the label, the first one of the body. Escaped
// the way all text is, so a label that ever held a brace or a backslash could
// not open or close a group.
func labelLine(label string) string {
	return string(appendEncoded(nil, label, nil, Unicode)) + `\par` + "\r\n"
}

// belowMinimum is the refusal of a size too small for these settings.
func belowMinimum(s settings, requested, min int64) error {
	reason := core.Says("rtf.MinimumDocument", "a document holds its header and its font table, and the two need that much")
	switch {
	case s.pictures.Count > 0 && s.binary:
		// The pictures are what a tester raises the size for, so they are what
		// the reason names, whatever else was asked - the way a page does.
		reason = core.Says("rtf.MinimumWithBinaryPictures", "a document holds its header, its font table and every picture byte for byte, and together they need that much")
	case s.pictures.Count > 0:
		reason = core.Says("rtf.MinimumWithPictures", "a document holds its header, its font table and every picture in hex, which is twice the size of the picture itself, and together they need that much")
	case s.adds():
		reason = core.Says("rtf.MinimumWithSettings", "a document holds its header and its font table and what %s, %s, %s and %s add to them, and together they need that much",
			core.A("Characters", core.LabelTerm(Characters)), core.A("Elements", core.LabelTerm(Elements)), core.A("Nesting", core.LabelTerm(Nesting)),
			core.A("Links", core.LabelTerm(Links)))
	}
	return &format.BelowMinimumError{
		Format:    "RTF",
		Requested: requested,
		Minimum:   min,
		Reason:    reason,
		Hint:      core.Says("format.AskForBOrMore", "Ask for %d B or more.", core.A("Min", min)),
	}
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(memo)
	if !ok {
		return core.Defect(fmt.Errorf("rtf: the plan was not produced by this generator"))
	}
	// The label sits outside the groups of nesting, as it does in a page, so
	// the file says what it is however deep the content is.
	l := m.settings.layout()
	if err := core.WriteAll(w, []byte(l.head+m.labelLine)); err != nil {
		return err
	}
	if err := writeBraces(ctx, w, '{', l.levels); err != nil {
		return err
	}
	if err := core.WriteAll(w, []byte(l.body)); err != nil {
		return err
	}
	if err := writeLinks(ctx, w, m.settings.links); err != nil {
		return err
	}
	if err := writePictures(ctx, w, m.pictures); err != nil {
		return err
	}
	filled := p.Bytes - l.fixedBytes() - m.pictureBytes - int64(len(m.labelLine))
	if err := core.FillRecords(ctx, w, core.NewRand(m.seed), filled, blocks{}); err != nil {
		return err
	}
	if err := writeBraces(ctx, w, '}', l.levels); err != nil {
		return err
	}
	return core.WriteAll(w, []byte{'}'})
}

// layout is what a document of these settings holds whatever its size: the
// header with the font table, and with every the properties, the page header
// and the footer, the groups of nesting, the paragraphs the settings add and
// the links. The pictures, planned apart because each is a file of its own,
// the blocks and the closing words go between those and the braces that close
// the groups, and the last brace closes the document.
type layout struct {
	head   string
	levels int
	body   string
	links  int64
}

func (s settings) layout() layout {
	fonts := plainFontTable
	if s.international {
		fonts = fontTable
	}
	l := layout{
		head:   `{\rtf1\ansi\ansicpg` + strconv.Itoa(declaredCodePage) + `\deff0\uc1` + fonts + "\r\n",
		levels: s.nesting,
		links:  linksBytes(s.links),
	}
	if s.every {
		l.head += catalogueHead
	}
	if s.international {
		l.body += international(s.encoding)
	}
	if s.every {
		l.body += catalogueBody
	}
	return l
}

// fixedBytes is everything but the label, the pictures, the blocks and the
// closing words: the smallest document of these settings without its label and
// its pictures.
func (l layout) fixedBytes() int64 {
	return int64(len(l.head)) + 2*int64(l.levels) + int64(len(l.body)) + l.links + 1
}

// minimumBytes is the smallest document, which is what the registry declares.
// Every combination of settings answers for its own.
func minimumBytes() int64 { return settings{encoding: Unicode}.layout().fixedBytes() }

// braceChunk is how many braces are written at once, and so how often a stop
// is noticed - ten million levels are ten megabytes on each side.
const braceChunk = 32 * 1024

// writeBraces writes n copies of one brace.
func writeBraces(ctx context.Context, w io.Writer, brace byte, n int) error {
	chunk := make([]byte, min(n, braceChunk))
	for i := range chunk {
		chunk[i] = brace
	}
	for n > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		k := min(n, len(chunk))
		if err := core.WriteAll(w, chunk[:k]); err != nil {
			return err
		}
		n -= k
	}
	return nil
}
