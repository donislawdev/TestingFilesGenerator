package guard

import (
	"bytes"
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// A page on one line and a page with styles, both 2026-10-14
// (docs/HTML-4-2026-10-14.md). Every guard here reads the BYTES, for the reason
// htmlfragment_test.go gives, and the names of the elements the rules are for
// are written here from the setting's meaning rather than taken from the
// generator.

// htmlStyledElements are the elements the rules of a page are for.
var htmlStyledElements = []string{"h1", "h2", "p", "ul", "li", "table", "th", "td", "blockquote"}

var (
	// htmlAnyTag is one tag of any kind, for taking every tag out of a text.
	htmlAnyTag = regexp.MustCompile(`<[^>]*>`)
	// htmlPreText is the text of a pre element as the catalogue writes it.
	htmlPreText = regexp.MustCompile(`(?s)<pre><code>(.*?)</code></pre>`)
	// htmlSheetRule is one rule of a sheet, readable or short, and
	// htmlStyledStart one start tag of an element a rule is for, its
	// attributes optional so a bare one is found as bare.
	htmlSheetRule   = regexp.MustCompile(`([a-z0-9]+) ?\{ ?([^}]*?) ?\}`)
	htmlStyledStart = regexp.MustCompile(`<(` + strings.Join(htmlStyledElements, "|") + `)(\s[^>]*)?>`)
)

// htmlTagNames is the tags of a page in order, a closing one with its slash,
// and none of their attributes.
func htmlTagNames(body []byte) []string {
	var out []string
	for _, m := range htmlNamedTag.FindAllSubmatch(body, -1) {
		out = append(out, string(m[1])+strings.ToLower(string(m[2])))
	}
	return out
}

// htmlCatalogueOf is where the catalogue of every element sits in a page, from
// its header to the end of its footer.
func htmlCatalogueOf(t *testing.T, body []byte) (start, end int) {
	t.Helper()
	start, end = bytes.Index(body, []byte("<header>")), bytes.LastIndex(body, []byte("</footer>"))
	if start < 0 || end < start {
		t.Fatal("the page carries no catalogue to read")
	}
	return start, end + len("</footer>")
}

// TestAMinifiedPageIsTheSamePageOnOneLine is the claim of formatting=minified.
//
// The page on one line has no line break anywhere, and it is the same page:
// the same tags in the same order as the page of lines with the same seed, as
// far as that one goes before its closing paragraph. A block on one line takes
// fewer bytes, so at the same size it holds the same blocks and more of them -
// core.FillRecords takes a block whenever it fits, and a shorter one fits
// wherever the longer one did.
func TestAMinifiedPageIsTheSamePageOnOneLine(t *testing.T) {
	walked := 0
	for _, props := range htmlCombos {
		if props["formatting"] != "" {
			continue
		}
		walked++
		one := maps.Clone(props)
		one["formatting"] = "minified"
		lines, page := writeEncoded(t, "html", 65536, props), writeEncoded(t, "html", 65536, one)
		if at := bytes.IndexAny(page, "\r\n"); at >= 0 {
			t.Errorf("%v: the page of one line has a line break at byte %d", props, at)
		}
		if bytes.Count(lines, []byte("\n")) < 2 {
			t.Errorf("%v: the page of lines is not lines, so nothing was taken out of it", props)
		}
		want, got := htmlTagNames(lines), htmlTagNames(page)
		cut := lastPara(want)
		if cut == 0 {
			t.Fatalf("%v: the page of lines has no closing paragraph to compare up to", props)
		}
		want = want[:cut]
		if len(got) < len(want) || !slices.Equal(got[:len(want)], want) {
			t.Errorf("%v: the page of one line is not the page of lines - %d tags against %d, and they part at tag %d",
				props, len(got), len(want), partAt(got, want))
		}
	}
	if walked < 8 {
		t.Fatalf("only %d of the settings walked are pages of lines, so this compared almost nothing", walked)
	}
}

// lastPara is where the last paragraph opens, the closing one.
func lastPara(tags []string) int {
	at := 0
	for i, tag := range tags {
		if tag == "p" {
			at = i
		}
	}
	return at
}

// partAt is the first place two lists of tags differ.
func partAt(a, b []string) int {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	return i
}

// TestTheCatalogueOnOneLineReadsAsItDoesOnMany holds the one place a line
// break is not just layout.
//
// Between two elements a line break shows as a space, and taken out it runs
// the text on each side together - 52 of the catalogue's 575 elements moved
// in Chromium when it was. So every tag is taken out with nothing in its place
// and the words have to be the same, and the line break inside pre, which is
// text, has to come back from its character reference.
func TestTheCatalogueOnOneLineReadsAsItDoesOnMany(t *testing.T) {
	for _, shape := range []string{"document", "fragment"} {
		props := map[string]string{"elements": "every", "structure": shape}
		one := maps.Clone(props)
		one["formatting"] = "minified"
		lines, page := writeEncoded(t, "html", 32768, props), writeEncoded(t, "html", 32768, one)
		a, b := htmlCatalogueOf(t, lines)
		lines = lines[a:b]
		a, b = htmlCatalogueOf(t, page)
		page = bytes.ReplaceAll(page[a:b], []byte("&#10;"), []byte("\n"))

		want := strings.Fields(string(htmlAnyTag.ReplaceAll(lines, nil)))
		got := strings.Fields(string(htmlAnyTag.ReplaceAll(page, nil)))
		if len(want) < 100 {
			t.Fatalf("%s: the catalogue holds %d words, so there was hardly anything to compare", shape, len(want))
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s: the catalogue on one line reads differently, from word %d on", shape, partAt(got, want))
		}
		pre, onePre := htmlPreText.FindSubmatch(lines), htmlPreText.FindSubmatch(page)
		if pre == nil || !bytes.Contains(pre[1], []byte("\n")) {
			t.Fatalf("%s: the catalogue has no pre with a line break in its text, so nothing here asks about one", shape)
		}
		if onePre == nil || !bytes.Equal(onePre[1], pre[1]) {
			t.Errorf("%s: the text of pre on one line is not the text it has on many", shape)
		}
	}
}

// TestInlineStylesAreTheRulesOfTheSheet is the claim that inline and embedded
// are one look in two places.
//
// The rules are read out of the embedded sheet, and on the inline page every
// element they are for - the label, the paragraphs in other scripts, the
// pictures' paragraphs and the blocks, everything but the catalogue - carries
// exactly its rule. Each element has to be met at least once, so the guard
// cannot pass by finding none.
func TestInlineStylesAreTheRulesOfTheSheet(t *testing.T) {
	for _, layout := range []string{"lines", "minified"} {
		props := map[string]string{"formatting": layout, "characters": "international", "elements": "every",
			"inline": "1", "inline_size": "1kb", "styles": "embedded"}
		withSheet := writeEncoded(t, "html", 65536, props)
		// The page's own sheet is the last one in the head - the catalogue's
		// comes first.
		at := bytes.LastIndex(withSheet, []byte("<style>"))
		sheet := map[string]string{}
		for _, m := range htmlSheetRule.FindAllSubmatch(withSheet[at:at+bytes.Index(withSheet[at:], []byte("</style>"))], -1) {
			sheet[string(m[1])] = string(m[2])
		}
		if len(sheet) != len(htmlStyledElements) {
			t.Fatalf("%s: the sheet holds %d rules, and there are %d elements they are for", layout, len(sheet), len(htmlStyledElements))
		}

		props["styles"] = "inline"
		page := writeEncoded(t, "html", 65536, props)
		a, b := htmlCatalogueOf(t, page)
		met := map[string]int{}
		for _, m := range htmlStyledStart.FindAllSubmatch(append(slices.Clip(page[:a]), page[b:]...), -1) {
			name, attrs := string(m[1]), string(m[2])
			if css, ok := strings.CutPrefix(attrs, ` style="`); !ok || !strings.HasPrefix(css, sheet[name]+`"`) {
				t.Errorf("%s: a %s carries %q and its rule is %q", layout, name, attrs, sheet[name])
			}
			met[name]++
		}
		for _, name := range htmlStyledElements {
			if met[name] == 0 {
				t.Errorf("%s: no %s outside the catalogue, so its rule went unchecked", layout, name)
			}
		}
	}
}

// TestAStyleSheetSitsWhereAParserPutsItInTheHead is where embedded puts the
// rules, and that no style names anything to fetch.
//
// In the head of a page, and at the very start of a fragment - behind the mark
// when there is one - where a parser puts it into the head it makes up. Anywhere
// further down is a sheet in the body, which the Nu HTML Checker calls an error
// rule by rule (docs/HTML-4-2026-10-14.md section 1).
func TestAStyleSheetSitsWhereAParserPutsItInTheHead(t *testing.T) {
	page := writeEncoded(t, "html", 16384, map[string]string{"styles": "embedded"})
	if s, h := bytes.Index(page, []byte("</style>")), bytes.Index(page, []byte("</head>")); s < 0 || h < s {
		t.Errorf("the sheet of a page is not in its head")
	}
	for _, bom := range []string{"false", "true"} {
		fragment := writeEncoded(t, "html", 16384, map[string]string{"styles": "embedded", "structure": "fragment", "bom": bom})
		if !bytes.HasPrefix(bytes.TrimPrefix(fragment, []byte{0xEF, 0xBB, 0xBF}), []byte("<style>")) {
			t.Errorf("bom=%s: the sheet of a fragment is not where it opens", bom)
		}
	}
	plain := writeEncoded(t, "html", 16384, nil)
	if bytes.Contains(plain, []byte("<style")) || bytes.Contains(plain, []byte(" style=")) {
		t.Error("a page ordered with no styles carries some")
	}
	for _, styles := range []string{"embedded", "inline"} {
		styled := writeEncoded(t, "html", 16384, map[string]string{"styles": styles})
		for _, fetch := range []string{"url(", "@import", "@font-face"} {
			if bytes.Contains(styled, []byte(fetch)) {
				t.Errorf("styles=%s: the page's styles carry %s, which a browser fetches when the page is opened", styles, fetch)
			}
		}
	}
}
