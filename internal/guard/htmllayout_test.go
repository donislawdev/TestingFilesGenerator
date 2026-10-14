package guard

import (
	"bytes"
	"context"
	"errors"
	"maps"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// An indented page and a page of links, both 2026-10-14
// (docs/HTML-5-2026-10-14.md). Every guard here reads the BYTES, for the reason
// htmlfragment_test.go gives, and the indentation and the kinds of link are
// worked out here from what the settings mean rather than taken from the
// generator.

// htmlIndentCombos are the pages of lines the indentation guard indents: each
// shape, the levels, the catalogue with its pre, the paragraphs in other
// scripts behind a mark, both ways of carrying styles, pictures and links.
var htmlIndentCombos = []map[string]string{
	{},
	{"structure": "fragment"},
	{"nesting": "3"},
	{"nesting": "40", "structure": "fragment"},
	{"elements": "every"},
	{"elements": "every", "structure": "fragment", "nesting": "2"},
	{"characters": "international", "bom": "true"},
	{"styles": "embedded"},
	{"styles": "embedded", "structure": "fragment", "bom": "true"},
	{"styles": "inline", "characters": "international", "links": "20"},
	{"inline": "2", "inline_size": "1kb", "nesting": "2"},
	{"links": "40", "elements": "every", "nesting": "5", "styles": "inline"},
}

// htmlVoid are the elements of HTML with no end tag, from the standard.
var htmlVoid = map[string]bool{"area": true, "base": true, "br": true, "col": true, "embed": true, "hr": true,
	"img": true, "input": true, "link": true, "meta": true, "source": true, "track": true, "wbr": true}

// htmlIndentByElements is a page of lines indented the way the setting says:
// two spaces in front of a line for every element open where it starts, one
// fewer when the line starts by closing one, and nothing in front of the text
// of pre. Written here and not borrowed, so the generator and this cannot
// agree on a wrong depth.
func htmlIndentByElements(page []byte) []byte {
	var out bytes.Buffer
	depth, inPre := 0, false
	for _, line := range strings.SplitAfter(string(page), "\n") {
		if line == "" {
			continue
		}
		if !inPre {
			lead := depth
			if strings.HasPrefix(strings.TrimPrefix(line, string([]byte{0xEF, 0xBB, 0xBF})), "</") {
				lead--
			}
			out.WriteString(strings.Repeat("  ", max(lead, 0)))
		}
		out.WriteString(line)
		for _, m := range htmlNamedTag.FindAllStringSubmatch(line, -1) {
			name := strings.ToLower(m[2])
			if htmlVoid[name] || strings.HasSuffix(m[0], "/>") {
				continue
			}
			if name == "pre" {
				inPre = m[1] == ""
			}
			if m[1] == "" {
				depth++
			} else {
				depth--
			}
		}
	}
	return out.Bytes()
}

// writeUnlabelled is writeEncoded without the label, whose text carries the
// size and so differs between two files of different sizes.
func writeUnlabelled(t *testing.T, size int64, props map[string]string) []byte {
	t.Helper()
	d, err := format.Get("html")
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Generator.Plan(format.Request{Bytes: size, Seed: 7741, Properties: props})
	if err != nil {
		t.Fatalf("planning %d B with %v: %v", size, props, err)
	}
	var buf bytes.Buffer
	if err := d.Generator.Write(context.Background(), &buf, p); err != nil {
		t.Fatalf("writing %d B with %v: %v", size, props, err)
	}
	if int64(buf.Len()) != size {
		t.Fatalf("%v: ordered %d B and produced %d", props, size, buf.Len())
	}
	return buf.Bytes()
}

// TestAnIndentedPageIsThePageOfLinesIndentedByItsElements is the claim of
// formatting=indented, to the byte.
//
// The page of lines is indented here, and the indented page of exactly that
// size with the same seed has to be those bytes. They are the same blocks: an
// indented block is its block of lines and its indentation, so a block that
// fits one fits the other, and the closing paragraph is left the same number
// of bytes. It ASKS that the indentation did something, so a setting that
// stopped reaching the writer cannot pass as a page with nothing to indent.
func TestAnIndentedPageIsThePageOfLinesIndentedByItsElements(t *testing.T) {
	for _, props := range htmlIndentCombos {
		lines := writeUnlabelled(t, 16384, props)
		want := htmlIndentByElements(lines)
		if bytes.Equal(want, lines) {
			t.Fatalf("%v: indenting the page of lines changed nothing, so this compares nothing", props)
		}
		indented := maps.Clone(props)
		indented["formatting"] = "indented"
		got := writeUnlabelled(t, int64(len(want)), indented)
		if !bytes.Equal(got, want) {
			gl, wl := strings.Split(string(got), "\n"), strings.Split(string(want), "\n")
			at := 0
			for at < len(gl) && at < len(wl) && gl[at] == wl[at] {
				at++
			}
			t.Errorf("%v: the indented page is not the page of lines indented by its elements - they part at line %d", props, at+1)
		}
	}
}

// TestTheLabelOfAnIndentedPageSitsInTheBody holds the one line the comparison
// above leaves out: the label stands at the depth of the body, outside the
// levels, so a reader that drops the deep ones keeps it.
func TestTheLabelOfAnIndentedPageSitsInTheBody(t *testing.T) {
	for _, c := range []struct {
		props map[string]string
		lead  string
	}{
		{map[string]string{"formatting": "indented", "nesting": "4"}, "\n    <h1>tfg"},
		{map[string]string{"formatting": "indented", "nesting": "4", "structure": "fragment"}, "<h1>tfg"},
	} {
		body := writeEncoded(t, "html", 16384, c.props)
		at := bytes.Index(body, []byte(c.lead))
		if at < 0 || (c.props["structure"] == "fragment" && at != 0) {
			t.Errorf("%v: the label does not stand where the body puts it (%q)", c.props, c.lead)
		}
	}
}

var (
	// htmlLinkList is the list of links, its items and the address of each.
	htmlLinkList = regexp.MustCompile(`(?s)<ul id="tfg-links">\n(.*?)</ul>\n`)
	htmlLinkHref = regexp.MustCompile(`<li><a href="([^"]*)"[^>]*>([^<]+)</a></li>\n`)
	// htmlLinkHost is the host of an address that names one.
	htmlLinkHost = regexp.MustCompile(`^(?:[a-z]+:)?//([^/:?#]+)`)
)

// TestTheLinksAreTheKindsInTurnOnDomainsKeptForExamples is the claim of links.
//
// As many links as were ordered, each kind once before any repeats, in the
// same turn after it, the first pointing at the list itself - which is there
// to point at - and no address naming a host outside the domains RFC 2606 keeps
// for examples. The list sits above the pictures, which a reader can stop
// short of (docs/HTML-3-2026-10-14.md section 9).
func TestTheLinksAreTheKindsInTurnOnDomainsKeptForExamples(t *testing.T) {
	const kinds = 18
	for _, n := range []int{0, 1, kinds, kinds + 1, 2*kinds + 9} {
		for _, shape := range []string{"document", "fragment"} {
			props := map[string]string{"links": strconv.Itoa(n), "structure": shape, "inline": "1", "inline_size": "1kb"}
			body := writeEncoded(t, "html", 32768, props)
			list := htmlLinkList.FindSubmatchIndex(body)
			if n == 0 {
				if list != nil || bytes.Contains(body, []byte("tfg-links")) {
					t.Errorf("%v: no link was ordered and the page carries the list", props)
				}
				continue
			}
			if list == nil {
				t.Fatalf("%v: the page has no list of links", props)
			}
			if pic := bytes.Index(body, []byte(`<img alt="inline_`)); pic < list[1] {
				t.Errorf("%v: the list does not sit above the pictures", props)
			}
			items := htmlLinkHref.FindAllSubmatch(body[list[2]:list[3]], -1)
			if len(items) != n {
				t.Fatalf("%v: %d links were ordered and the list carries %d", props, n, len(items))
			}
			if string(items[0][1]) != "#tfg-links" || bytes.Count(body, []byte(`id="tfg-links"`)) != 1 {
				t.Errorf("%v: the first link does not point at the list, or the list is not there once to point at", props)
			}
			seen := map[string]bool{}
			for i, it := range items {
				href := string(it[1])
				if i < kinds && seen[href] {
					t.Errorf("%v: link %d repeats %s before every kind has come once", props, i+1, href)
				}
				seen[href] = true
				if i >= kinds && href != string(items[i-kinds][1]) {
					t.Errorf("%v: link %d is %s, and the kinds in turn make %s", props, i+1, href, items[i-kinds][1])
				}
				if h := htmlLinkHost.FindStringSubmatch(href); h != nil &&
					h[1] != "example.com" && h[1] != "example.net" && h[1] != "example.org" && !strings.HasSuffix(h[1], ".example") {
					t.Errorf("%v: link %d names %s, which is not a domain kept for examples", props, i+1, h[1])
				}
			}
		}
	}
}

// TestADeepIndentedPageAndAMillionLinksStopSoonAfterARunIsStopped holds the
// two things a page now writes a chunk at a time rather than at once: a
// hundred thousand indented levels, 10 GB in front of the first block, and a
// million links. Counted in bytes rather than timed, for the reason
// picturestop_test.go gives.
func TestADeepIndentedPageAndAMillionLinksStopSoonAfterARunIsStopped(t *testing.T) {
	const stopAt = int64(1 << 20)
	d, err := format.Get("html")
	if err != nil {
		t.Fatal(err)
	}
	for _, props := range []map[string]string{
		{"nesting": "100000", "formatting": "indented"},
		{"links": "1000000"},
	} {
		size := d.SmallestAccepted(format.Request{Seed: 7741, Label: true, Properties: props})
		if size < 64*stopAt {
			t.Fatalf("%v: the smallest page is %d B, too small to ask whether it stops early", props, size)
		}
		p, err := d.Generator.Plan(format.Request{Bytes: size, Seed: 7741, Label: true, Properties: props})
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		w := &stoppingWriter{cancel: cancel, after: stopAt}
		err = d.Generator.Write(ctx, w, p)
		cancel()
		if !errors.Is(err, context.Canceled) {
			t.Errorf("%v: stopped after 1 MiB of %d B and the write ended with %v, not the stop", props, size, err)
		}
		if w.n > 4*stopAt {
			t.Errorf("%v: stopped after 1 MiB and wrote %d B of %d B before it noticed", props, w.n, size)
		}
	}
}
