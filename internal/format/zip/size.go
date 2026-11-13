package zip

import (
	"context"
	"fmt"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/archive"
)

// How long an archive comes to, measured by writing its structure: what
// planning, the minimum and the layout for a cut all ask.

// archiveSize measures the archive without generating anything it holds.
//
// The structure is built for real - every header, every name, the comment, the
// central directory - and only the file contents are left out and added on as
// numbers. Everything inside is stored rather than compressed, so that is exact
// rather than close.
//
// Measured on the standard library on 2026-08-03, because a document about a
// format has been wrong here five times out of five and this one is arithmetic
// standing in for a measurement:
//
//	an archive of 3 entries with no contents          424 B
//	the same 3 entries holding 4096 B each         12 712 B
//	424 + 3*4096                                   12 712 B   exact
//
// Checked at 0, 1, 1000, 4096 and 100 000 bytes an entry, and the comment moves
// the total by exactly its own length at 0, 1, 2, 50, 4096 and 65 535.
//
// Why it matters: planning used to call this two or three times and each call
// generated every file inside. That made --dry-run cost about what the real run
// costs - measured at 960 ms against a 56 ms baseline for a 256 MB archive,
// while writing it for real took 1585 ms. A dry run is the step this project
// tells people to take before anything large, so it has to be the cheap one.
//
// If the arithmetic is ever wrong the size guard says so immediately: the
// engine refuses any file whose written length differs from its plan, and the
// property test walks about 120 sizes for this format.
//
// It reads the region the files take off the map the pass marks, with the
// contents added on the same way, for laying the padding out around them
// (cut.go).
func archiveSize(m memo) (mapped, error) {
	// What this pass writes of the files is their headers alone, so the
	// length the writing pass marks them with is not theirs here.
	m.split.sized = false
	c := &counter{}
	rec := &format.MapRecorder{W: c}
	if err := build(context.Background(), rec, m, false); err != nil {
		return mapped{}, err
	}
	content, err := contentOf(rec.Map(c.n, true))
	if err != nil {
		return mapped{}, err
	}
	var files int64
	for _, ch := range m.children {
		files += ch.plan.Bytes + m.lock.EntryOverhead()
	}
	total := c.n + files
	content.Length += files
	if m.split.front {
		front := m.split.frontBytes + m.lock.EntryOverhead()
		total += front
		content.At += front
	}
	if m.withFiller {
		total += m.fillerSize + m.lock.EntryOverhead()
	}
	return mapped{size: total, content: content}, nil
}

type counter struct{ n int64 }

func (c *counter) Write(p []byte) (int, error) { c.n += int64(len(p)); return len(p), nil }

// minimumBytes is the smallest archive this generator can produce: one empty
// entry, no label, no padding.
// It panics rather than returning a number nobody measured.
//
// This used to answer 1<<62 on any failure, on the reasoning that refusing
// every size is safer than declaring a wrong minimum. It is not safer, it is
// quieter: ZIP would refuse every request ever made with a message about a
// minimum of 4611686018427387904 B, and nothing would say why. That is rule 6
// broken in the way this project keeps finding.
//
// The condition is a programming mistake rather than a runtime one. It needs
// the default entry format to be unregistered when this runs, and the language
// says when that can happen: packages are initialised in the order of their
// import paths, so the txt package is registered before this one by rule rather
// than by luck. This paragraph said "true by accident" until 2026-08-25, when
// an outside review built an argument on the opposite claim - that the
// specification guarantees nothing here. Both were wrong, and the comment being
// wrong about its own mechanism is the worse of the two.
//
// What the rule does not cover is a rename that moves the entry format after
// this package in that order, or a second entry point linking this package
// without that one. The first is what a guard asks, since a format has been
// renamed in this tree before. format.Register already panics on the same class
// of mistake, so this matches it: a build that cannot state its own minimum
// fails at start rather than at every use.
func minimumBytes() int64 {
	desc, err := format.Get(archive.DefaultFormat)
	if err != nil {
		panic(fmt.Sprintf("zip: the default entry format %q is not registered yet, so the minimum size of an archive cannot be worked out. Check the import order in internal/format/all", archive.DefaultFormat))
	}
	cp, err := desc.Generator.Plan(format.Request{Bytes: 0, Seed: 0, Label: false})
	if err != nil {
		panic(fmt.Sprintf("zip: the default entry format %q cannot produce an empty file, so the minimum size of an archive cannot be worked out: %v", archive.DefaultFormat, err))
	}
	n, err := archiveSize(memo{children: []child{{
		name: "txt_0001.txt", desc: desc, plan: cp,
	}}})
	if err != nil {
		panic(fmt.Sprintf("zip: the smallest archive cannot be measured: %v", err))
	}
	return n.size
}
