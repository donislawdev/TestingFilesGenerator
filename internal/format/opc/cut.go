package opc

import (
	"fmt"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// SettleForACut is Settle for a package a damage will cut after its first
// cutAt bytes (format.Request.CutAt): the padding is split into a part in front
// of every other part and what Settle would put behind them, so that the cut
// keeps the share of the parts it keeps of the whole package.
//
// With all of the padding behind the parts, as a package nothing cuts has it,
// a large package cut at any share kept every part and lost only padding.
// ECMA-376 part 2 names no order of parts and a reader finds them through the
// central directory: Python's zipfile, 7-Zip, bsdtar, python-docx, openpyxl,
// python-pptx and LibreOffice read a package whose padding part comes first as
// they read it today (docs/TRUNCATE-2026-11-11.md section 10.3). The map
// already counts every part as content from the first byte, so nothing of the
// content stands in front of the padding.
//
// The part in front costs its header before any padding, so the front is
// nothing or at least that, and grows where it cannot land exactly - the cut
// then takes a little more of the parts, the "at least" missing promises.
// Where the padding is too short for the parts the cut should take, all of it
// goes in front and the cut keeps more of them by the share of what stands
// behind (section 10.4). The manifest's cut reports what was kept either way.
func SettleForACut(parts []Part, shape Shape, want, cutAt int64) (Package, error) {
	p, err := Settle(parts, shape, want)
	if err != nil || cutAt <= 0 || cutAt >= want {
		return p, err
	}
	c, err := measureCut(parts, shape)
	if err != nil {
		return Package{}, err
	}
	pad := want - shape.Bare
	front := format.PaddingInFront(0, c.content, cutAt, want)
	if front <= 0 || pad < c.frontOverhead {
		return p, nil
	}
	front = max(front, c.frontHead)
	// What the part in front costs in all: its header and padding in front,
	// and its record in the central directory behind the parts.
	cost := c.frontOverhead + front - c.frontHead
	if cost > pad {
		cost = pad
		front = c.frontHead + pad - c.frontOverhead
	}
	q := Package{Parts: parts, Filler: NoFiller, Front: front - c.frontHead, Content: c.content}
	back := pad - cost
	switch {
	case back == 0:
	case back <= CommentLimit:
		q.Comment = int(back)
	case back >= c.endOverhead:
		q.Filler = back - c.endOverhead
	default:
		return Package{}, core.Defect(fmt.Errorf("opc: %d B behind the parts is more than a comment carries and less than a padding part costs (%d B)", back, c.endOverhead))
	}
	return q, nil
}

// cutShape is what SettleForACut needs to know of a set of parts beyond Shape,
// measured by writing them rather than added up from header widths, for the
// reason Shape gives.
type cutShape struct {
	// content is how long the parts come to in the archive.
	content int64
	// frontHead is how many bytes an empty part in front puts before the
	// parts, and frontOverhead what it costs in all, its record in the central
	// directory with it.
	frontHead, frontOverhead int64
	// endOverhead is what an empty padding part behind the parts costs when
	// one stands in front - it carries the longer name.
	endOverhead int64
}

func measureCut(parts []Part, shape Shape) (cutShape, error) {
	var c cutShape
	bare, err := size(Package{Parts: parts, Filler: NoFiller, Front: NoFiller})
	if err != nil {
		return c, err
	}
	front, err := size(Package{Parts: parts, Filler: NoFiller, Front: 0})
	if err != nil {
		return c, err
	}
	both, err := size(Package{Parts: parts, Filler: 0, Front: 0})
	if err != nil {
		return c, err
	}
	c.content = bare.content.Length
	c.frontHead = front.content.At
	c.frontOverhead = front.size - shape.Bare
	c.endOverhead = both.size - front.size
	if bare.size != shape.Bare || front.content.Length != c.content {
		return c, core.Defect(fmt.Errorf("opc: the parts measured %d B with %d B of them content, then %d B of content behind a padding part, where planning measured %d B",
			bare.size, c.content, front.content.Length, shape.Bare))
	}
	return c, nil
}

// mapped is a package's length and the region its parts take, read off the map
// it marks as it is written (size).
type mapped struct {
	size    int64
	content format.Region
}
