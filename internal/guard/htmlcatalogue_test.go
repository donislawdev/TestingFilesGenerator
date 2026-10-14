package guard

import (
	"bytes"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// What elements=every, characters=international and bom put into a page, each
// read from the bytes. The lists are written here from the standard and from
// the decision, not imported from the generator, so the two cannot agree on a
// list that lost a name.

// htmlStandardElements is the index "List of elements" of the HTML standard,
// read on 2026-10-14 (docs/HTML-2-2026-10-14.md section 11.1): 115 names.
var htmlStandardElements = strings.Fields(`a abbr address area article aside audio b
base bdi bdo blockquote body br button canvas caption cite code col colgroup data
datalist dd del details dfn dialog div dl dt em embed fieldset figcaption figure
footer form h1 h2 h3 h4 h5 h6 head header hgroup hr html i iframe img input ins
kbd label legend li link main map mark math menu meta meter nav noscript object
ol optgroup option output p picture pre progress q rp rt ruby s samp script
search section select selectedcontent slot small source span strong style sub
summary sup svg table tbody td template textarea tfoot th thead time title tr
track u ul var video wbr`)

// htmlInputTypes is the 22 keywords of the type attribute of input, read from
// the standard on the same day.
var htmlInputTypes = strings.Fields(`hidden text search tel url email password
date month week time datetime-local number range color checkbox radio file
submit image reset button`)

// htmlPageOnly is what only a whole page holds: its skeleton and its head.
var htmlPageOnly = map[string]bool{"base": true, "body": true, "head": true, "html": true,
	"link": true, "meta": true, "style": true, "title": true}

// TestEveryElementOfTheStandardIsInTheCatalogue is the claim of elements=every.
//
// Every name of the standard but script (decision D3), and in a fragment every
// name but the eight that only a page holds - with a note saying so, since a
// setting called every that quietly leaves eight out would be the manifest
// claiming a file that is not there. The page also reaches no network: the
// detail promises it, and a picture by address would make a browser fetch it.
func TestEveryElementOfTheStandardIsInTheCatalogue(t *testing.T) {
	d, err := format.Get("html")
	if err != nil {
		t.Fatal(err)
	}
	if len(htmlStandardElements) != 115 || len(htmlInputTypes) != 22 {
		t.Fatalf("the lists here hold %d names and %d kinds of input, and the standard read on 2026-10-14 has 115 and 22",
			len(htmlStandardElements), len(htmlInputTypes))
	}
	for _, shape := range []string{"document", "fragment"} {
		props := map[string]string{"elements": "every", "structure": shape}
		body := writeEncoded(t, "html", 32768, props)
		// Opening tags only: an element whose opening tag was swapped for
		// another one and whose closing tag stayed behind is not in the page.
		names := map[string]bool{}
		for _, m := range htmlNamedTag.FindAllSubmatch(body, -1) {
			if len(m[1]) == 0 {
				names[strings.ToLower(string(m[2]))] = true
			}
		}
		for _, name := range htmlStandardElements {
			want := name != "script" && !(shape == "fragment" && htmlPageOnly[name])
			if want != names[name] {
				t.Errorf("%s: the catalogue holds <%s> %v, and it should be %v", shape, name, names[name], want)
			}
		}
		for _, kind := range htmlInputTypes {
			if !bytes.Contains(body, []byte(`<input type="`+kind+`"`)) {
				t.Errorf("%s: there is no input of type %s", shape, kind)
			}
		}
		if bytes.Contains(body, []byte("://")) {
			t.Errorf("%s: the page names an address on a network, so opening it fetches something", shape)
		}

		p, err := d.Generator.Plan(format.Request{Bytes: 32768, Seed: 7741, Label: true, Properties: props})
		if err != nil {
			t.Fatal(err)
		}
		noted := false
		for _, n := range p.Notes {
			noted = noted || n.Code == "elements_left_out"
		}
		if noted != (shape == "fragment") {
			t.Errorf("%s: a note on the elements left out is %v, and only a fragment leaves any out", shape, noted)
		}
	}
}

// htmlScripts is what characters=international promises, by the words of its
// detail: Polish, Russian, Japanese, Arabic right to left, emoji, a letter in
// two code points and entities named and numbered.
var htmlScripts = []string{
	`<p lang="pl">Zażółć gęślą jaźń.</p>`,
	`<p lang="ru">Это текст на русском языке.</p>`,
	`<p lang="ja">これは日本語のテキストです。</p>`,
	`<p lang="ar" dir="rtl">هذا نص باللغة العربية.</p>`,
	string(rune(0x1F600)),
	string([]rune{0x1F469, 0x200D, 0x1F4BB}),
	"e" + string(rune(0x301)),
	"&nbsp; &copy; &euro; &#8364; &#x1F600;",
}

// TestInternationalCarriesEveryScriptAndASCIICarriesNone holds both halves:
// the paragraphs are there when they were ordered, and a page that was not
// asked for them is plain ASCII, the catalogue of every element included.
func TestInternationalCarriesEveryScriptAndASCIICarriesNone(t *testing.T) {
	for _, shape := range []string{"document", "fragment"} {
		body := writeEncoded(t, "html", 16384, map[string]string{"characters": "international", "structure": shape})
		for _, piece := range htmlScripts {
			if !bytes.Contains(body, []byte(piece)) {
				t.Errorf("%s: characters=international and the page does not carry %+q", shape, piece)
			}
		}
		for _, elements := range []string{"basic", "every"} {
			plain := writeEncoded(t, "html", 16384, map[string]string{"elements": elements, "structure": shape})
			if i := bytes.IndexFunc(plain, func(r rune) bool { return r > 0x7e }); i >= 0 {
				t.Errorf("%s, elements=%s: characters=ascii and byte %d of the page is outside ASCII", shape, elements, i)
			}
		}
	}
}

// TestTheHTMLByteOrderMarkIsThereOnlyWhenOrdered holds the mark: in front of
// the doctype of a page and of the label of a fragment, and nowhere when it was
// not ordered. The mark comes from the shared text encoding axis, so its
// declaration is held by familyaxes_test.go - this holds where it lands.
func TestTheHTMLByteOrderMarkIsThereOnlyWhenOrdered(t *testing.T) {
	mark := []byte{0xEF, 0xBB, 0xBF}
	for shape, next := range map[string]string{"document": "<!DOCTYPE html>", "fragment": "<h1>tfg"} {
		with := writeEncoded(t, "html", 4096, map[string]string{"bom": "true", "structure": shape})
		if !bytes.HasPrefix(with, append(append([]byte{}, mark...), next...)) {
			t.Errorf("%s: bom=true and the file does not open with the mark followed by %q", shape, next)
		}
		without := writeEncoded(t, "html", 4096, map[string]string{"structure": shape})
		if bytes.Contains(without, mark) {
			t.Errorf("%s: no mark was ordered and the file holds one", shape)
		}
	}
}
