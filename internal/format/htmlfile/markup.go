package htmlfile

import (
	"regexp"
	"strings"
)

// How a page is spelled: every piece of markup it is written with, for one
// layout and one way of carrying its styles (docs/HTML-4-2026-10-14.md).
//
// Held as finished strings, the way the JSON layouts hold theirs, because the
// arithmetic that lands a page on its byte count has to measure what it writes
// rather than predict it. The plain spelling is the bytes every page has been
// written with, and every other spelling is made from it by two rules - the
// rules of the sheet put on each element, and the line breaks taken out. The
// pieces sit in one array, so a rule reaches every piece there is, including
// one added later, and there is no list of them to forget one on.

// markup is one spelling of a page, a piece to each part.
type markup [parts]string

// part names one piece of a page's markup.
type part int

// The pieces a page writes itself, then those it does not. Everything before
// ownParts is spelled by the rules below, and the catalogue of every element,
// written by hand once, keeps its look and has a rule of its own.
const (
	// The skeleton in two halves, the style sheet between them or in front of
	// a fragment, and what closes the body and the document.
	headOpen part = iota
	headClose
	sheet
	bodyClose
	// The label, one level of nesting, and a paragraph.
	labelOpen
	labelClose
	divOpen
	divClose
	paraOpen
	paraClose
	// The blocks that fill a page.
	h2Open
	h2Close
	ulOpen
	liOpen
	liClose
	ulClose
	// The table opens on a line of its own and its head row on the next, so
	// an indented page can put the row a level further in.
	tableOpen
	tableHead
	rowOpen
	cellSep
	rowClose
	tableClose
	quoteOpen
	quoteClose
	// The list of links around its items, which are spelled as the items of
	// any list.
	linksOpen
	linksClose
	// The paragraphs in other scripts, and a picture's line around its name
	// and its base64.
	scripts
	pictureLead
	pictureTail

	ownParts
)

// The catalogue's two pieces, after the page's own.
const (
	headCatalogue = ownParts + iota
	bodyCatalogue
	parts
)

// plain is how a page has been spelled since 2026-08-01, the golden hashes
// holding it there. The skeleton is in two halves so the catalogue and the
// style sheet can go into the head between them, and the title says Page until
// a label fills it - what the smallest document and every --clean one carry
// (an empty title until 2026-10-15, O277).
var plain = markup{
	headOpen: `<!DOCTYPE html>` + "\n" +
		`<html lang="en">` + "\n" +
		`<head>` + "\n" +
		`<meta charset="utf-8">` + "\n" +
		placeholderTitle + "\n",
	headClose: `</head>` + "\n" +
		`<body>` + "\n",
	bodyClose:  "</body>\n</html>\n",
	labelOpen:  "<h1>",
	labelClose: "</h1>\n",
	// One level of nesting, each on a line of its own.
	divOpen:   "<div>\n",
	divClose:  "</div>\n",
	paraOpen:  "<p>",
	paraClose: "</p>\n",
	h2Open:    "<h2>",
	h2Close:   "</h2>\n",
	ulOpen:    "<ul>\n",
	liOpen:    "<li>",
	liClose:   "</li>\n",
	ulClose:   "</ul>\n",
	tableOpen: "<table>\n",
	tableHead: "<tr><th>id</th><th>field</th><th>value</th></tr>\n",
	rowOpen:   "<tr><td>",
	cellSep:   "</td><td>",
	rowClose:  "</td></tr>\n",
	// The table closes and the quote holds a paragraph.
	tableClose: "</table>\n",
	quoteOpen:  "<blockquote><p>",
	quoteClose: "</p></blockquote>\n",
	// The id is what the first link points at, and one no document property
	// has: DOMPurify 3.4.17 takes out an id that would shadow one - links and
	// forms both, measured on 2026-10-14 - and the link would point nowhere.
	linksOpen:     `<ul id="tfg-links">` + "\n",
	linksClose:    "</ul>\n",
	scripts:       international,
	pictureLead:   `<p><img alt="`,
	pictureTail:   `"></p>` + "\n",
	headCatalogue: catalogueHead,
	bodyCatalogue: catalogueBody,
}

// rule is the look of one element: its declarations, property and value.
type rule struct {
	element      string
	declarations [][2]string
}

// rules are the styles a page carries, one list for both places they go. A
// font every system has, colours and spacing, and nothing that names an
// address: a page that fetches a font or a picture for its style reaches a
// network when it is opened (D16). No rule for div either, or the levels of
// nesting would each push the content further in.
var rules = []rule{
	{"h1", [][2]string{{"font-family", "sans-serif"}, {"font-size", "28px"}, {"color", "#0b3d91"}, {"border-bottom", "2px solid #0b3d91"}, {"margin", "0 0 16px"}}},
	{"h2", [][2]string{{"font-family", "sans-serif"}, {"font-size", "20px"}, {"color", "#0b3d91"}, {"margin", "16px 0 8px"}}},
	{"p", [][2]string{{"font-family", "sans-serif"}, {"color", "#222222"}, {"line-height", "1.5"}, {"margin", "0 0 12px"}}},
	{"ul", [][2]string{{"margin", "0 0 12px"}, {"padding-left", "24px"}}},
	{"li", [][2]string{{"font-family", "sans-serif"}, {"color", "#222222"}, {"margin", "4px 0"}}},
	{"table", [][2]string{{"border-collapse", "collapse"}, {"margin", "0 0 12px"}}},
	{"th", [][2]string{{"font-family", "sans-serif"}, {"background", "#0b3d91"}, {"color", "#ffffff"}, {"text-align", "left"}, {"padding", "4px 8px"}}},
	{"td", [][2]string{{"font-family", "sans-serif"}, {"border", "1px solid #cccccc"}, {"padding", "4px 8px"}}},
	{"blockquote", [][2]string{{"border-left", "4px solid #cccccc"}, {"margin", "12px 0"}, {"padding-left", "12px"}, {"color", "#555555"}}},
}

// css is a rule's declarations, readable on a page of lines and as short as
// they go on a page of one.
func (r rule) css(minified bool) string {
	colon, sep := ": ", "; "
	if minified {
		colon, sep = ":", ";"
	}
	parts := make([]string, len(r.declarations))
	for i, d := range r.declarations {
		parts[i] = d[0] + colon + d[1]
	}
	return strings.Join(parts, sep)
}

// sheetOf is the rules as a style element: one rule to a line, or all of them
// on one.
func sheetOf(minified bool) string {
	var b strings.Builder
	if minified {
		b.WriteString("<style>")
		for _, r := range rules {
			b.WriteString(r.element + "{" + r.css(true) + "}")
		}
		b.WriteString("</style>")
		return b.String()
	}
	b.WriteString("<style>\n")
	for _, r := range rules {
		b.WriteString(r.element + " { " + r.css(false) + " }\n")
	}
	b.WriteString("</style>\n")
	return b.String()
}

// styledTag finds the start tag of every element a rule is for, in one pass, so
// a tag given an attribute is never found again. The space or the bracket after
// the name keeps p from matching pre and th from matching thead.
var styledTag = func() *regexp.Regexp {
	names := make([]string, len(rules))
	for i, r := range rules {
		names[i] = r.element
	}
	return regexp.MustCompile(`<(` + strings.Join(names, "|") + `)([ >])`)
}()

// withStyles puts each element's rule on it as a style attribute, first among
// its attributes.
func withStyles(s string, minified bool) string {
	css := make(map[string]string, len(rules))
	for _, r := range rules {
		css[r.element] = r.css(minified)
	}
	return styledTag.ReplaceAllStringFunc(s, func(tag string) string {
		m := styledTag.FindStringSubmatch(tag)
		return "<" + m[1] + ` style="` + css[m[1]] + `"` + m[2]
	})
}

// oneLineCatalogue is the catalogue on one line. Where a line ended it keeps a
// space, because between two inline elements a line break shows as one - taken
// out, 52 of its 575 elements move on the screen (Chromium 152, 2026-10-14).
// The one break that is text, in the pre element, becomes a reference to the
// character, which is what keeps its text the same.
func oneLineCatalogue(body string) string {
	body = strings.Replace(body, preformatted, strings.ReplaceAll(preformatted, "\n", "&#10;"), 1)
	return strings.ReplaceAll(body, "\n", " ")
}

// spell makes the spelling of one layout and one way of carrying styles out of
// the plain one.
func spell(minified bool, styles string) *markup {
	m := plain
	if styles == stylesInline {
		for p := range ownParts {
			m[p] = withStyles(m[p], minified)
		}
	}
	if styles == stylesEmbedded {
		m[sheet] = sheetOf(minified)
	}
	if minified {
		for p := range ownParts {
			m[p] = strings.ReplaceAll(m[p], "\n", "")
		}
		m[headCatalogue] = strings.ReplaceAll(m[headCatalogue], "\n", "")
		m[bodyCatalogue] = oneLineCatalogue(m[bodyCatalogue])
	}
	return &m
}

// spellings are made once, when the program starts: the engine holds the plan
// of every file at once, and a plan holds a pointer to one of these rather
// than a copy of the attributes on every element.
var spellings = func() map[spellingKey]*markup {
	out := map[spellingKey]*markup{}
	for _, minified := range []bool{false, true} {
		for _, styles := range []string{stylesNone, stylesEmbedded, stylesInline} {
			out[spellingKey{minified, styles}] = spell(minified, styles)
		}
	}
	return out
}()

type spellingKey struct {
	minified bool
	styles   string
}

// styleName is the value of styles, none for settings made without one.
func (s settings) styleName() string {
	if s.styles == "" {
		return stylesNone
	}
	return s.styles
}

// spelling is how these settings spell a page. An indented page is spelled as
// a page of lines and indented as it is written.
func (s settings) spelling() *markup {
	return spellings[spellingKey{s.minified(), s.styleName()}]
}

// lineEnding is what the manifest says the lines of the page end with: none for
// a page on one line, which has no line to end, where lf would describe a file
// that is not there.
func (s settings) lineEnding() string {
	if s.minified() {
		return "none"
	}
	return "lf"
}
