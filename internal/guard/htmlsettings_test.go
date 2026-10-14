package guard

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/oracle"
)

// HTML gained four settings on 2026-10-14 - nesting, characters, elements and
// bom - measured on eight readers before a line was written
// (docs/HTML-2-2026-10-14.md section 11), and the pictures the same day
// (docs/HTML-3-2026-10-14.md). Every guard here reads the BYTES, for
// the reason htmlfragment_test.go gives: the tolerant reader beside this one
// accepts anything, and the two that build the deepest trees either flatten
// the levels (Chromium) or drop them with their content (libxml2), so a tree
// would report something other than the file.

// htmlCombos are the settings the guards below walk, one for each path the
// bytes take and the two shapes of every path that changes the end of a file.
var htmlCombos = []map[string]string{
	{},
	{"nesting": "1"},
	{"nesting": "40", "structure": "fragment"},
	{"characters": "international"},
	{"elements": "every"},
	{"elements": "every", "structure": "fragment"},
	{"bom": "true"},
	{"bom": "true", "structure": "fragment"},
	{"nesting": "600", "characters": "international", "elements": "every", "bom": "true"},
	{"inline": "2", "inline_size": "1kb"},
	{"inline": "1", "inline_format": "svg", "inline_size": "1kb", "structure": "fragment", "nesting": "2"},
	{"formatting": "minified"},
	{"formatting": "minified", "structure": "fragment", "nesting": "3"},
	{"styles": "embedded"},
	{"styles": "embedded", "structure": "fragment", "bom": "true"},
	{"styles": "inline", "characters": "international"},
	{"formatting": "minified", "styles": "inline", "elements": "every", "inline": "1", "inline_size": "1kb", "nesting": "2"},
	{"formatting": "minified", "styles": "embedded", "elements": "every", "structure": "fragment"},
}

// htmlLineEnd is what ends a line of a page of these settings - nothing, when
// the whole page is one line.
func htmlLineEnd(props map[string]string) string {
	if props["formatting"] == "minified" {
		return ""
	}
	return "\n"
}

// htmlClosing is what a file of these settings ends with after its closing
// paragraph, written here from the shape of HTML rather than taken from the
// generator.
func htmlClosing(props map[string]string) string {
	nl := htmlLineEnd(props)
	n, _ := strconv.Atoi(props["nesting"])
	end := strings.Repeat("</div>"+nl, n)
	if props["structure"] != "fragment" {
		end += "</body>" + nl + "</html>" + nl
	}
	return end
}

// htmlEmptyEnd matches the end of the smallest file of these settings: an
// empty closing paragraph, with the style it carries when styles are inline,
// and what closes after it.
func htmlEmptyEnd(props map[string]string) *regexp.Regexp {
	open := `<p>`
	if props["styles"] == "inline" {
		open = `<p style="[^"]+">`
	}
	return regexp.MustCompile(open + `</p>` + regexp.QuoteMeta(htmlLineEnd(props)+htmlClosing(props)) + `\z`)
}

// TestEveryHTMLSettingAnswersForItsOwnMinimum holds the arithmetic of every
// path.
//
// The floor is pinned from the FILE rather than from the generator's sums: at
// the smallest size a format accepts, the closing paragraph has to be empty -
// one byte of text in it means the floor could have been lower, and one byte
// less has to be refused naming that floor. A floor the format announces and
// then turns down, or one that wastes bytes nobody ordered, both go red here.
func TestEveryHTMLSettingAnswersForItsOwnMinimum(t *testing.T) {
	d, err := format.Get("html")
	if err != nil {
		t.Fatal(err)
	}
	for _, props := range htmlCombos {
		floor := d.SmallestAccepted(format.Request{Seed: 7741, Label: true, Properties: props})
		body := writeEncoded(t, "html", floor, props)
		if want := htmlEmptyEnd(props); !want.Match(body) {
			t.Errorf("%v: the smallest file, %d B, does not end in an empty closing paragraph and what closes after it (%s), so its floor is not its own",
				props, floor, want)
		}
		_, err := d.Generator.Plan(format.Request{Bytes: floor - 1, Seed: 7741, Label: true, Properties: props})
		var below *format.BelowMinimumError
		if !errors.As(err, &below) || below.Minimum != floor {
			t.Errorf("%v: one byte under the floor of %d B was answered with %v, not a refusal naming that floor",
				props, floor, err)
		}
	}
}

// htmlNamedTag is one tag, opening or closing, with a hyphen allowed in its name
// since the catalogue carries a custom element.
var htmlNamedTag = regexp.MustCompile(`<(/?)([a-zA-Z][a-zA-Z0-9-]*)[^>]*>`)

// htmlDivDepth is how deep div elements go in a page, read tag by tag.
func htmlDivDepth(body []byte) int {
	depth, deepest := 0, 0
	for _, m := range htmlNamedTag.FindAllSubmatch(body, -1) {
		if string(m[2]) != "div" {
			continue
		}
		if len(m[1]) == 0 {
			depth++
			deepest = max(deepest, depth)
		} else {
			depth--
		}
	}
	return deepest
}

// TestNestingPutsTheContentThatManyLevelsDeep is the claim of nesting.
//
// It ASKS how deep the file goes rather than assuming the setting reached the
// writer: the levels open after the label, close after the last paragraph, and
// nothing else in a basic page is a div, so the deepest div is the number
// ordered and not one more. The label is outside them - a reader that drops
// what it cannot hold keeps the line saying which file it read.
func TestNestingPutsTheContentThatManyLevelsDeep(t *testing.T) {
	for _, n := range []int{1, 3, 600} {
		props := map[string]string{"nesting": strconv.Itoa(n)}
		body := writeEncoded(t, "html", 65536, props)
		if got := htmlDivDepth(body); got != n {
			t.Errorf("nesting=%d: the page goes %d div elements deep", n, got)
		}
		label := bytes.Index(body, []byte("<body>\n<h1>tfg"))
		levels := bytes.Index(body, []byte(strings.Repeat("<div>\n", n)))
		if label < 0 || levels < label {
			t.Errorf("nesting=%d: the label is not in front of the levels, so a reader that drops deep elements drops it too", n)
		}
		if !bytes.HasSuffix(body, []byte("</p>\n"+htmlClosing(props))) {
			t.Errorf("nesting=%d: the page does not close its levels after its last paragraph", n)
		}
	}
}

// TestEveryHTMLSettingIsRecordedInTheManifest holds what a test reads back.
//
// Every setting is recorded whatever was asked, so a reader of the manifest
// never has to know a default - and a recorded value is the one the file was
// written with, not the one that was typed.
func TestEveryHTMLSettingIsRecordedInTheManifest(t *testing.T) {
	d, err := format.Get("html")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		props map[string]string
		want  map[string]any
	}{
		{nil, map[string]any{"structure": "document", "nesting": 0, "characters": "ascii", "elements": "basic", "bom": false,
			"formatting": "lines", "styles": "none", "line_ending": "lf",
			"inline": 0, "inline_format": "png", "inline_size": int64(8192), "inline_bytes": int64(0), "inline_encoded_bytes": int64(0)}},
		{map[string]string{"structure": "fragment", "nesting": "7", "characters": "international", "elements": "every", "bom": "true",
			"formatting": "minified", "styles": "inline", "inline": "2", "inline_format": "gif", "inline_size": "1kb"},
			map[string]any{"structure": "fragment", "nesting": 7, "characters": "international", "elements": "every", "bom": true,
				"formatting": "minified", "styles": "inline", "line_ending": "none",
				"inline": 2, "inline_format": "gif", "inline_size": int64(1024), "inline_bytes": int64(2048), "inline_encoded_bytes": int64(2736)}},
		{map[string]string{"styles": "embedded"}, map[string]any{"formatting": "lines", "styles": "embedded", "line_ending": "lf"}},
	} {
		p, err := d.Generator.Plan(format.Request{Bytes: 65536, Seed: 7741, Label: true, Properties: c.props})
		if err != nil {
			t.Fatal(err)
		}
		for k, v := range c.want {
			if p.Properties[k] != v {
				t.Errorf("%v: the manifest records %s as %v, not %v", c.props, k, p.Properties[k], v)
			}
		}
	}
}

// TestTheDefaultHTMLSettingsAreTheBytesItAlwaysWrote is the way back, for every
// setting at once: saying nothing and saying every default out loud are two
// routes through the parser and have to meet.
func TestTheDefaultHTMLSettingsAreTheBytesItAlwaysWrote(t *testing.T) {
	spoken := map[string]string{"nesting": "0", "characters": "ascii", "elements": "basic", "bom": "false",
		"formatting": "lines", "styles": "none", "inline": "0", "inline_format": "png", "inline_size": "8kb"}
	for _, size := range []int64{118, 119, 4096} {
		if !bytes.Equal(writeEncoded(t, "html", size, nil), writeEncoded(t, "html", size, spoken)) {
			t.Errorf("at %d B: saying nothing and saying every default produce different bytes", size)
		}
	}
}

// TestTheStructuralCheckIsToldEveryHTMLSetting is the canary for the checker.
//
// Every other guard hands it a file of the settings it asked for, so all of
// them would stay green if it stopped looking at any one of them. Each row
// below is a file of one value handed over as the other.
func TestTheStructuralCheckIsToldEveryHTMLSetting(t *testing.T) {
	dir := t.TempDir()
	for i, c := range []struct{ made, told map[string]string }{
		{map[string]string{"nesting": "3"}, map[string]string{"nesting": "2"}},
		{map[string]string{"nesting": "3"}, map[string]string{"nesting": "4"}},
		{map[string]string{"characters": "international"}, map[string]string{"characters": "ascii"}},
		{map[string]string{}, map[string]string{"characters": "international"}},
		{map[string]string{"elements": "every"}, map[string]string{"elements": "basic"}},
		{map[string]string{}, map[string]string{"elements": "every"}},
		{map[string]string{"bom": "true"}, map[string]string{"bom": "false"}},
		{map[string]string{}, map[string]string{"bom": "true"}},
		{map[string]string{"inline": "2", "inline_size": "1kb"}, map[string]string{"inline": "1", "inline_size": "1kb"}},
		{map[string]string{}, map[string]string{"inline": "1"}},
		{map[string]string{"inline": "1", "inline_format": "gif", "inline_size": "1kb"}, map[string]string{"inline": "1", "inline_size": "1kb"}},
		{map[string]string{"inline": "1", "inline_size": "1kb"}, map[string]string{"inline": "1", "inline_size": "2kb"}},
		{map[string]string{"formatting": "minified"}, map[string]string{}},
		{map[string]string{}, map[string]string{"formatting": "minified"}},
		{map[string]string{"styles": "embedded"}, map[string]string{}},
		{map[string]string{"styles": "inline"}, map[string]string{"styles": "embedded"}},
		{map[string]string{}, map[string]string{"styles": "inline"}},
		{map[string]string{"styles": "embedded", "structure": "fragment"}, map[string]string{"styles": "inline", "structure": "fragment"}},
		// On one line a rule follows the brace of the last one with nothing
		// between them - the honest half of this row is what keeps the checker
		// reading every rule there (review of #179).
		{map[string]string{"styles": "embedded", "formatting": "minified"}, map[string]string{"styles": "inline", "formatting": "minified"}},
	} {
		path := filepath.Join(dir, strconv.Itoa(i)+".html")
		if err := os.WriteFile(path, writeEncoded(t, "html", 32768, c.made), 0o600); err != nil {
			t.Fatal(err)
		}
		honest := oracle.Strict("html", path, htmlSettingWords(c.made)...)
		if !honest.Available {
			t.Skip("the structural check needs python")
		}
		if honest.Err != nil {
			t.Fatalf("%v handed over as itself was refused: %v", c.made, honest.Err)
		}
		if res := oracle.Strict("html", path, htmlSettingWords(c.told)...); res.Err == nil {
			t.Errorf("a page made with %v handed over as %v was called correct", c.made, c.told)
		}
	}
}

// htmlSettingWords is settings as the checker takes them.
func htmlSettingWords(props map[string]string) []string {
	out := make([]string, 0, len(props))
	for k, v := range props {
		out = append(out, k+"="+v)
	}
	return out
}
