package htmlfile

import (
	"regexp"
	"strings"
)

// An indented page: every line of a page of lines, indented two spaces for
// each element open where it starts - the width JSON indents with, and the
// children of html indented as Prettier lays them out. Nothing else about the
// lines changes, so an indented page is a page of lines with spaces in front
// of them (docs/HTML-5-2026-10-14.md section 2.2). Measured on 2026-10-14,
// Chromium places every element of the two the same.
//
// The pieces written once - the head, the paragraphs in other scripts and the
// catalogue - are indented here by the elements they open. The pieces a page
// repeats - the levels of nesting, the blocks, the links and the pictures -
// sit at a depth only the settings know, and take their indentation from it
// as they are written, so a hundred thousand levels are never built in memory.

// indentUnit is one level of indentation.
const indentUnit = "  "

// indentation is the indentation of a line depth levels deep.
func indentation(depth int) string { return strings.Repeat(indentUnit, depth) }

// indentationBytes is its length, worked out without building it.
func indentationBytes(depth int) int64 { return int64(depth * len(indentUnit)) }

// line is one line of a piece laid out by its elements, without its line
// break. A line inside pre is text rather than layout, and takes no
// indentation at all - not its own and not the depth it is written at.
type line struct {
	text string
	pre  bool
}

// voidElements have no end tag, so they open nothing.
var voidElements = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true, "hr": true, "img": true,
	"input": true, "link": true, "meta": true, "source": true, "track": true, "wbr": true,
}

// tag finds a start or an end tag: the slash, the name, and what follows the
// name up to the bracket. The doctype starts with "!" and is not one.
var tag = regexp.MustCompile(`<(/?)([a-zA-Z][a-zA-Z0-9-]*)([^>]*)>`)

// layOut indents every line of text by the depth of the elements open where it
// starts, beginning at depth: a line that starts by closing one stands at the
// depth of its start tag. text ends with a line break, the way every piece of
// a page of lines does.
func layOut(text string, depth int) []line {
	var out []line
	if text == "" {
		// Nothing is no lines, not one empty one - a fragment with no sheet
		// has no prologue, and would otherwise open with a blank line.
		return out
	}
	inPre := false
	for _, raw := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		out = append(out, lineAt(raw, depth, inPre))
		depth, inPre = after(raw, depth, inPre)
	}
	return out
}

// lineAt is one line written depth levels deep, or as it is inside pre.
func lineAt(raw string, depth int, inPre bool) line {
	if inPre {
		return line{text: raw, pre: true}
	}
	if strings.HasPrefix(raw, "</") {
		depth--
	}
	return line{text: indentation(max(depth, 0)) + raw}
}

// after is the depth once the tags of a line are read, and whether pre is still
// open.
func after(raw string, depth int, inPre bool) (int, bool) {
	for _, m := range tag.FindAllStringSubmatch(raw, -1) {
		closing, name := m[1] == "/", strings.ToLower(m[2])
		if voidElements[name] || strings.HasSuffix(m[3], "/") {
			continue
		}
		if name == "pre" {
			inPre = !closing
		}
		if closing {
			depth--
		} else {
			depth++
		}
	}
	return depth, inPre
}

// laidOutPieces are the pieces an indented page writes once inside its levels:
// the paragraphs in other scripts and the catalogue, laid out by their own
// elements at depth zero and given the depth of the content as they are
// written.
type laidOutPieces struct{ scripts, catalogue []line }

// laidOut are made when the program starts, one for each way of carrying
// styles, out of the pieces of a page of lines - an indented page is spelled as
// one.
var laidOut = func() map[string]laidOutPieces {
	out := map[string]laidOutPieces{}
	for _, styles := range []string{stylesNone, stylesEmbedded, stylesInline} {
		m := spellings[spellingKey{false, styles}]
		out[styles] = laidOutPieces{scripts: layOut(m[scripts], 0), catalogue: layOut(m[bodyCatalogue], 0)}
	}
	return out
}()

// laid is the laid out pieces of these settings.
func (s settings) laid() laidOutPieces { return laidOut[s.styleName()] }

// joined is lines laid out at depth zero, as one string.
func joined(lines []line) string {
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l.text + "\n")
	}
	return b.String()
}

// laidOutBytes is what lines take written at a depth whose indentation is lead
// bytes long.
func laidOutBytes(lines []line, lead int64) int64 {
	var n int64
	for _, l := range lines {
		n += int64(len(l.text)) + 1
		if !l.pre {
			n += lead
		}
	}
	return n
}

// appendLaidOut appends lines written at the depth whose indentation is lead.
func appendLaidOut(dst []byte, lines []line, lead string) []byte {
	for _, l := range lines {
		if !l.pre {
			dst = append(dst, lead...)
		}
		dst = append(dst, l.text...)
		dst = append(dst, '\n')
	}
	return dst
}
