// Package htmlfile generates HTML documents.
//
// The package is not called "html" so that it cannot be confused with the
// standard library package of that name at a glance, the same reason logfile is
// not called log. The format id is "html".
package htmlfile

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/textenc"
)

// Measured on 2026-08-01, a comment holds arbitrary bytes to 1 MiB both after
// the closing tag and inside the body. That says where the format tolerates
// arbitrary bytes, and the filling still does not go there - a five megabyte
// page that is one comment renders as an empty page.
//
// Whole blocks instead, and the remainder to the exact byte in the text of a
// closing paragraph. Fifth format built this way.
//
// HTML is the weakest case in the group for checking. There is one tolerant
// reader on this machine and it is lenient by design - the format itself says
// a parser must recover from almost anything, so "it parsed" says very little.
// The guards written against this format carry more of the weight than they do
// for SVG, which has a renderer behind it.

const generatorVersion = "1"

func init() {
	format.Register(format.Descriptor{
		ID:          "html",
		Name:        "HyperText Markup Language",
		Extension:   ".html",
		MediaType:   "text/html", // IANA, W3C
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,

		// A document with an empty body is legal HTML and renders as a blank
		// page. That is a shape request rather than a byte count. The minimum
		// here is the skeleton and one whole block.
		MinBytes: minimumBytes(),

		Padding: format.PaddingChannel{
			Name:     "the text of the closing paragraph",
			Where:    format.PlacementEnd,
			Capacity: 0,
		},

		// A page shows what it is, so the label is a visible heading rather
		// than a comment.
		Label:  format.LabelVisible,
		Oracle: "python-html",
		// Pictures inside the page, style sheets and a page on one line come
		// later (docs/HTML-2-2026-10-14.md section 8). Declaring only what is
		// here is what makes a recipe asking for them fail loudly instead of
		// quietly producing something else.
		Properties:       properties(),
		GeneratorVersion: generatorVersion,
		Generator:        generator{},
	})
}

type generator struct{}

type memo struct {
	head      string // the prologue, title included
	labelLine string // the visible heading, empty when absent
	seed      uint64
	settings  settings
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	s, err := settingsOf(r.Properties)
	if err != nil {
		return format.Plan{}, err
	}

	min := minimumFor(s)
	if r.Bytes < min {
		return format.Plan{}, belowMinimum(s, r.Bytes, min)
	}

	// A fragment has no doctype, and saying it has one would be the manifest
	// describing a file that is not there.
	doctype := "html"
	if s.shape == structureFragment {
		doctype = "none"
	}

	p := format.Plan{
		Bytes:       r.Bytes,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties: map[string]any{
			"encoding":    "utf-8",
			"line_ending": "lf",
			"doctype":     doctype,
			"language":    "en",
		},
	}
	for k, v := range s.manifest() {
		p.Properties[k] = v
	}
	if s.every && s.shape == structureFragment {
		// The setting says every element, and a fragment cannot hold eight of
		// them. Saying so here is what keeps the manifest from claiming a
		// catalogue the file does not carry.
		p.Notes = append(p.Notes, format.Note{
			Code: "elements_left_out",
			Detail: core.Says("html.FragmentLeavesOut", "A fragment has no head and no skeleton, so %s are not in it.",
				core.A("Elements", core.Joined{Items: pageOnly, And: true})),
		})
	}

	m := memo{seed: r.Seed, head: s.prologue(), settings: s}
	if r.Label {
		var note *format.Note
		m.head, m.labelLine, note = labelledHead(s, r, min)
		if note != nil {
			p.Notes = append(p.Notes, *note)
		}
	}

	p.Properties[format.PropertyLabelEmbedded] = m.labelLine != ""
	p.Memo = m
	return p, nil
}

// belowMinimum is the refusal of a size too small for these settings.
func belowMinimum(s settings, requested, min int64) error {
	reason := core.Says("html.MinimumPage", "a page holds a head, a body and whole blocks, and one of each needs that much")
	switch {
	case s.adds():
		reason = core.Says("html.MinimumWithSettings", "the file holds whole blocks and what %s, %s, %s and %s add to them, and together they need that much",
			core.A("Nesting", core.LabelTerm(settingNesting)), core.A("Characters", core.LabelTerm(settingCharacters)),
			core.A("Elements", core.LabelTerm(settingElements)), core.A("BOM", core.LabelTerm(textenc.SettingBOM)))
	case s.shape == structureFragment:
		reason = core.Says("html.MinimumFragment", "a fragment holds whole blocks, and one of them needs that much")
	}
	return &format.BelowMinimumError{
		Format:    "HTML",
		Requested: requested,
		Minimum:   min,
		Reason:    reason,
		Hint:      core.Says("format.AskForBOrMore", "Ask for %d B or more.", core.A("Min", min)),
	}
}

// labelledHead works out what a labelled file carries: the head, the visible
// heading, and a note instead of both when there is no room beside a whole
// block.
//
// Split out of Plan when that function reached the crowding threshold. The line
// is what a part does rather than how long it is: this answers "does the label
// fit and what does it cost", and Plan answers "is this request askable at all".
func labelledHead(s settings, r format.Request, min int64) (head, heading string, note *format.Note) {
	label := core.Label("html", r.Bytes, r.Seed)
	heading = "<h1>" + label + "</h1>\n"

	plain := s.prologue()
	head, extra := plain, int64(0)
	if s.shape == structureDocument {
		// The title and the heading say the same thing, which is what a page
		// does - one for the tab and one for the reader. A fragment has no head
		// to put a title in, so it carries only the heading, and the label is
		// still visible because a heading is.
		head = strings.Replace(plain, emptyTitle, "<title>"+label+"</title>", 1)
		extra = int64(len(head) - len(plain))
	}
	if extra+int64(len(heading))+min <= r.Bytes {
		return head, heading, nil
	}
	return plain, "", &format.Note{
		Code:   "label_omitted",
		Detail: core.Says("format.TheLabelNeedsBAndThis", "The label needs %d B and this file has no room for it beside a whole block. Its name and the manifest still identify it.", core.A("Length", len(heading))),
	}
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(memo)
	if !ok {
		return core.Defect(fmt.Errorf("html: the plan was not produced by this generator"))
	}

	// The label sits outside the levels of nesting, so a reader that drops
	// elements past its depth - libxml2 does, measured on 2026-10-14 - still
	// shows which file it read.
	head := m.head + m.labelLine + m.settings.opening()
	if err := core.WriteAll(w, []byte(head)); err != nil {
		return err
	}

	rng := core.NewRand(m.seed)
	return core.FillRecords(ctx, w, rng, p.Bytes-int64(len(head)), m.settings.blocks())
}

// minimumBytes is the smallest whole page, which is what the registry declares.
// Every combination of settings answers for its own, the way the JSON layouts
// do.
func minimumBytes() int64 { return minimumFor(settings{shape: structureDocument}) }

// minimumFor is the prologue, what the settings open the body with and one
// whole block, computed from the pieces Write uses rather than written down so
// it cannot drift away from the template.
func minimumFor(s settings) int64 {
	return int64(len(s.prologue())) + s.openingBytes() + int64(len(paraOpen)) + s.tailBytes()
}
