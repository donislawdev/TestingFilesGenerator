// Package eml generates mail messages (RFC 5322 with MIME): a text, in plain
// text, HTML or both, and real files of other formats attached to it in
// base64.
//
// What makes this format worth its place is what a system that takes mail in
// does with it - a helpdesk turning messages into tickets, a gateway scanning
// attachments, an archive keeping them. The size a limit is checked against
// is the message, a third larger than the files it carries. The names of the
// files can be written four ways, and readers do not agree on two of them.
// docs/EML-2026-10-07.md has the measurement this is built on.
package eml

import (
	"context"
	"fmt"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/archive"
)

const generatorVersion = "1"

func init() {
	format.Register(format.Descriptor{
		ID:          "eml",
		Name:        "Email message",
		Extension:   ".eml",
		MediaType:   "message/rfc822", // IANA, RFC 2046
		Fidelity:    format.FidelityFull,
		Determinism: format.DeterminismByte,

		// The header lines and an empty text, for the default settings. Every
		// other combination answers for itself through the same refusal, the
		// way json and geojson declare their default layout's minimum.
		MinBytes: minimumBytes(),

		Padding: format.PaddingChannel{
			Name: "the words of the text, in every text part",
			// Before the attached files when there are any, at the end when
			// there are none - so inside, where the format decides.
			Where:    format.PlacementInside,
			Capacity: 0,
		},
		Label:            format.LabelVisible,
		Oracle:           "python-email",
		Properties:       properties(),
		Container:        true,
		Members:          members,
		GeneratorVersion: generatorVersion,
		Generator:        generator{},
	})
}

type generator struct{}

// minimumBytes is the smallest message the default settings make, measured by
// the code that writes it.
func minimumBytes() int64 {
	s, err := parse(nil)
	if err != nil {
		panic(fmt.Sprintf("eml: the default settings do not parse: %v", err))
	}
	return newMessage(s, 0, nil).size()
}

func (generator) Plan(r format.Request) (format.Plan, error) {
	s, err := parse(r.Properties)
	if err != nil {
		return format.Plan{}, err
	}
	groups, err := archive.GroupsThrough("eml", r, door)
	if err != nil {
		return format.Plan{}, err
	}
	children, err := planChildren(r, groups, s)
	if err != nil {
		return format.Plan{}, err
	}
	m := newMessage(s, r.Seed, children)

	var notes []format.Note
	target := r.Bytes
	if r.SizeFromContents {
		target = settle(m, r)
	} else if notes, err = fit(m, r); err != nil {
		return format.Plan{}, err
	}
	share(m, target-m.size())
	if got := m.size(); got != target {
		return format.Plan{}, core.Defect(fmt.Errorf("eml: the message comes to %d B and %d B were planned", got, target))
	}
	p := describe(m, groups, target)
	p.Notes = notes
	return p, nil
}

// fit makes room for the label in a message of the size asked for, or says
// why the size is too small. A label that does not fit is left out with a
// note, as txt leaves it out - the minimum is the message without it.
func fit(m *message, r format.Request) ([]format.Note, error) {
	bare := m.size()
	if r.Bytes < bare {
		return nil, &format.BelowMinimumError{
			Format:    "EML",
			Requested: r.Bytes,
			Minimum:   bare,
			Reason: core.Says("eml.AMessageHoldsItsHeaderLines", "a message holds its header lines, its parts and every attached file in base64, "+
				"which is about a third larger than the file itself"),
			Hint: core.Says("format.AskForBOrMore", "Ask for %d B or more.", core.A("Min", bare)),
		}
	}
	if !r.Label {
		return nil, nil
	}
	m.label = core.Label("eml", r.Bytes, r.Seed)
	if m.size() <= r.Bytes {
		return nil, nil
	}
	cost := m.size() - bare
	m.label = ""
	// Silence is banned - a file the user believes carries a label has to say
	// that it does not.
	return []format.Note{{
		Code: "label_omitted",
		Detail: core.Says("format.TheLabelNeedsBAndThe", "The label needs %d B and the file is %d B, so this file carries no label. Its name and the manifest still identify it.",
			core.A("Length", cost), core.A("Bytes", r.Bytes)),
	}}, nil
}

// settle is the size of a message whose size comes from what it carries: no
// words beyond the label, and the label states the size. A longer number makes
// a longer label, so the two are settled against each other - the size only
// grows, so it settles in a few rounds.
func settle(m *message, r format.Request) int64 {
	size := m.size()
	for round := 0; r.Label && round < 8; round++ {
		m.label = core.Label("eml", size, r.Seed)
		next := m.size()
		if next == size {
			break
		}
		size = next
	}
	return size
}

// share hands the bytes still owed to the words of the text. alternative has
// the same words in both parts, so it takes them two at a time, and an odd one
// becomes the space parity puts in the HTML.
func share(m *message, rest int64) {
	if m.s.body != Alternative {
		m.text = rest
		return
	}
	m.parity = rest%2 == 1
	m.text = rest / 2
}

// describe is the plan and what reaches the manifest - public names under
// rule 10, approved on 2026-10-07 (docs/EML-2026-10-07.md section 10.5).
func describe(m *message, groups []format.Content, target int64) format.Plan {
	var raw, encoded int64
	for _, c := range m.children {
		raw += c.plan.Bytes
		encoded += c.encoded
	}
	p := format.Plan{
		Bytes:       target,
		Exact:       true,
		Determinism: format.DeterminismByte,
		Properties: map[string]any{
			Body:                         m.s.body,
			TextEncoding:                 m.s.encoding,
			Headers:                      m.s.headers,
			LineEndings:                  m.s.endings,
			FilenameStyle:                m.s.names,
			Attachments:                  len(m.children),
			"contains":                   archive.ContentSummary(groups),
			"attachment_bytes":           raw,
			"attachment_encoded_bytes":   encoded,
			"mime_entities":              m.entities(),
			"subject":                    m.id.subject,
			"message_id":                 m.id.messageID,
			format.PropertyLabelEmbedded: m.label != "",
		},
		Memo: m,
	}
	// The one format shape keeps the keys the archives give it, so a test
	// written against one container reads the other the same way.
	if len(groups) == 1 {
		p.Properties[AttachmentFormat] = groups[0].Format
		p.Properties[AttachmentSize] = groups[0].Bytes
	}
	return p
}

func (generator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	m, ok := p.Memo.(*message)
	if !ok {
		return core.Defect(fmt.Errorf("eml: the plan was not produced by this generator"))
	}
	o := out{w: w, ctx: ctx, eol: m.s.eol, scratch: make([]byte, 0, chunkSize+64)}
	m.emit(&o)
	return o.err
}
