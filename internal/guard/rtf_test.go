package guard

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"golang.org/x/text/unicode/norm"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/rtf"
	"github.com/donislawdev/TestingFilesGenerator/internal/oracle"
)

// rtfCases is every value of every setting, one at a time, then where they
// meet: text_encoding while characters is ascii, which has to change nothing,
// and everything at once (docs/RTF-2026-10-15.md).
var rtfCases = []map[string]string{
	{},
	{rtf.Characters: rtf.International},
	{rtf.Characters: rtf.International, rtf.TextEncoding: rtf.Both},
	{rtf.Characters: rtf.International, rtf.TextEncoding: rtf.Codepage},
	{rtf.Characters: rtf.International, rtf.TextEncoding: rtf.Literal},
	{rtf.TextEncoding: rtf.Literal},
	{rtf.Elements: rtf.Every},
	{rtf.Nesting: "1"},
	{rtf.Nesting: "1000"},
	{rtf.Characters: rtf.International, rtf.TextEncoding: rtf.Both, rtf.Elements: rtf.Every, rtf.Nesting: "50"},
	{rtf.Characters: rtf.International, rtf.TextEncoding: rtf.Codepage, rtf.Elements: rtf.Every},
	// The pictures and the links (docs/RTF-2-2026-10-15.md), small enough that
	// a document of 20 000 B still has room for blocks.
	{"inline": "2", "inline_size": "2kb"},
	{"inline": "1", "inline_format": "jpg", "inline_size": "2kb"},
	{"inline": "2", "inline_size": "2kb", rtf.PictureData: rtf.Binary},
	{rtf.Links: "40"},
	{rtf.Links: "18", "inline": "1", "inline_size": "1kb", rtf.Characters: rtf.International, rtf.Elements: rtf.Every, rtf.Nesting: "5"},
}

// rtfPolish, rtfRussian, rtfJapanese and rtfArabic are the sentences
// characters=international adds, held here on their own rather than taken from
// the generator - a guard comparing the generator with itself agrees with any
// mistake it makes. The Arabic by code point, so it cannot turn the line round.
var (
	rtfPolish   = "Zażółć gęślą jaźń."
	rtfRussian  = "Это текст на русском языке."
	rtfJapanese = "これは日本語のテキストです。"
	rtfArabic   = string([]rune{0x647, 0x630, 0x627, ' ', 0x646, 0x635, ' ', 0x628, 0x627, 0x644, 0x644, 0x63A, 0x629, ' ',
		0x627, 0x644, 0x639, 0x631, 0x628, 0x64A, 0x629, '.'})
)

// The structural checker is told the settings and has to find the document made
// the way they say, and pandoc has to read it without a word of complaint.
// pandoc's own readings of good files are held as they were measured on four
// versions of it, 3.1.3, 3.7.0.2, 3.12.1 and 3.13: the four scripts read right
// in unicode and both, the Polish line wrong in codepage, the footnote kept
// and the hidden text left out. literal is the Polish line wrong before 3.13
// and the four scripts right from it, the version that stopped dropping the
// character after a letter written literal (jgm/pandoc#11981, measured
// 2026-11-11) - so which one is asked is the version pandoc says it is. Those
// readings are what the sentences of text_encoding and elements tell a user,
// so this goes red the day pandoc reads differently, and the sentence is what
// has to change. Emoji and the text of a link differ between the versions and
// are not asked.
//
// Every size from the smallest the settings allow, where the label does not fit
// and the document has no paragraph, through the next two, to a document with
// room for blocks - and the smallest again with no label asked. Named to match
// ReferenceTool, the one job in CI that installs pandoc. Anywhere without python
// it skips, and says so.
func TestEveryRTFSettingSurvivesItsReferenceTool(t *testing.T) {
	d, err := format.Get("rtf")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	major, minor, ok := oracle.PandocVersion()
	pv := pandocVersionOf(major, minor, ok)
	if ok {
		t.Logf("pandoc %d.%d answers", major, minor)
	}
	var strict, byPandoc atomic.Int64
	t.Run("cases", func(t *testing.T) {
		for i, props := range rtfCases {
			t.Run(strconv.Itoa(i), func(t *testing.T) {
				t.Parallel()
				smallest := d.SmallestAccepted(format.Request{Properties: props, Label: true})
				for _, c := range []struct {
					size  int64
					label bool
				}{{smallest, true}, {smallest + 1, true}, {smallest + 2, true}, {20000, true}, {smallest, false}} {
					b, p := writeRTF(t, d, c.size, uint64(41+i), c.label, props)
					path := filepath.Join(dir, strconv.Itoa(i)+"_"+strconv.FormatInt(c.size, 10)+"_"+strconv.FormatBool(c.label)+".rtf")
					if err := os.WriteFile(path, b, 0o600); err != nil {
						t.Fatal(err)
					}
					s, pd := rtfSurvivesReaders(t, path, b, props, p, c.size, pv)
					strict.Add(int64(s))
					byPandoc.Add(int64(pd))
				}
			})
		}
	})
	t.Logf("%d documents read by the structural checker, %d by pandoc", strict.Load(), byPandoc.Load())
	if strict.Load() == 0 {
		t.Skip("python is not installed, so no document was checked - a skip, not a pass")
	}
}

// pandocVersion is what the guard knows of the pandoc that answers: whether
// --version said, and whether it is 3.13 or later, which reads a letter
// written literal right where the versions before it dropped the character
// after it (jgm/pandoc#11981).
type pandocVersion struct {
	known, readsLiteral bool
}

// pandocVersionOf is what oracle.PandocVersion's answer means to this guard.
func pandocVersionOf(major, minor int, ok bool) pandocVersion {
	if !ok {
		return pandocVersion{}
	}
	return pandocVersion{known: true, readsLiteral: major > 3 || major == 3 && minor >= 13}
}

// The version pandoc says it is decides what a document written literal has
// to read as, and that is held here on words written out rather than on the
// pandoc installed: CI has 3.1.3 and this machine 3.13, so each of them only
// ever asks one side of the boundary (jgm/pandoc#11981). Each version is held
// to its reading and has to refuse the other side's.
func TestThePandocVersionDecidesWhatALiteralDocumentReadsAs(t *testing.T) {
	right := strings.Join([]string{rtfPolish, rtfRussian, rtfJapanese, rtfArabic}, "\n")
	lost := "Za g ja." // the Polish line with the letter after each one written literal dropped
	literal := map[string]string{rtf.Characters: rtf.International, rtf.TextEncoding: rtf.Literal}
	for _, c := range []struct {
		out          string
		known, reads bool
	}{
		{"pandoc 3.1.3\nFeatures: +server +lua\n", true, false},
		{"pandoc 3.12.1\n", true, false},
		{"pandoc.exe 3.13\n", true, true},
		{"pandoc 3.13.1\n", true, true},
		{"pandoc 4.0\n", true, true},
		{"pandoc: unknown flag --version\n", false, false},
		{"", false, false},
	} {
		pv := pandocVersionOf(oracle.ParsePandocVersion(c.out))
		if pv.known != c.known || pv.readsLiteral != c.reads {
			t.Errorf("%q: known %v and reads literal %v, where %v and %v were meant", c.out, pv.known, pv.readsLiteral, c.known, c.reads)
			continue
		}
		if !pv.known {
			continue
		}
		onRight, onLost := rtfScriptsProblem(right, literal, pv.readsLiteral), rtfScriptsProblem(lost, literal, pv.readsLiteral)
		if c.reads && (onRight != "" || onLost == "") || !c.reads && (onRight == "" || onLost != "") {
			t.Errorf("%q: the four scripts read right say %q and the Polish line lost says %q", c.out, onRight, onLost)
		}
	}
}

// rtfSurvivesReaders asks the checker and pandoc about one document and says
// which of them read it.
func rtfSurvivesReaders(t *testing.T, path string, body []byte, props map[string]string, p format.Plan, size int64, pv pandocVersion) (strict, byPandoc int) {
	t.Helper()
	labelled, _ := p.Properties[format.PropertyLabelEmbedded].(bool)
	s := oracle.Strict("rtf", path, rtfSettings(props, labelled)...)
	if !s.Available {
		return 0, 0
	}
	if s.Err != nil {
		t.Fatalf("%v at %d B: %v", props, size, s.Err)
	}
	if props["inline"] != "" && props["inline"] != "0" && props[rtf.PictureData] == rtf.Binary {
		if rtfPandocBreaksBinaryPictures(t, path, body, props, size) {
			return 1, 1
		}
		return 1, 0
	}
	text, warnings, res := oracle.PandocRTF(path)
	if !res.Available {
		return 1, 0
	}
	if res.Err != nil || warnings != "" {
		t.Fatalf("%v at %d B: pandoc %v %s", props, size, res.Err, warnings)
	}
	if got := strings.Contains(text, "tfg - rtf - "); got != labelled {
		t.Fatalf("%v at %d B: the manifest says the label is there %v, and pandoc read it %v", props, size, labelled, got)
	}
	if !pv.known {
		t.Fatalf("%v at %d B: pandoc read the document and its --version says nothing this guard reads, so what it should make of literal is unknown", props, size)
	}
	rtfPandocReadings(t, norm.NFC.String(text), props, size, pv.readsLiteral)
	rtfPandocKeepsHexPictures(t, path, body, props, size)
	return 1, 1
}

// rtfPandocKeepsHexPictures is pandoc's reading of pictures in hex, measured on
// its three versions: every picture written out as the very file the document
// carries, which is what the sentence of inline says of it.
func rtfPandocKeepsHexPictures(t *testing.T, path string, body []byte, props map[string]string, size int64) {
	t.Helper()
	want := rtfPicturesOf(t, body)
	got, warnings, res := oracle.PandocRTFPictures(path, t.TempDir())
	if !res.Available {
		return
	}
	if res.Err != nil || warnings != "" || len(got) != len(want) {
		t.Fatalf("%v at %d B: pandoc read %d of %d pictures, %v %s", props, size, len(got), len(want), res.Err, warnings)
	}
	for i := range want {
		if !bytes.Equal(got[i], want[i].data) {
			t.Fatalf("%v at %d B: pandoc wrote picture %d out as %d B other than the %d B the document carries", props, size, i+1, len(got[i]), len(want[i].data))
		}
	}
}

// rtfPandocBreaksBinaryPictures is the canary of picture_data: pandoc was
// measured on three versions to turn the line ends inside a binary picture
// into line feeds, so no png came out whole, and to refuse the document once
// the shift reached the closing brace. The day it reads one whole, the
// sentence that says so has to change. Says whether pandoc was there to ask.
func rtfPandocBreaksBinaryPictures(t *testing.T, path string, body []byte, props map[string]string, size int64) bool {
	t.Helper()
	want := rtfPicturesOf(t, body)
	got, _, res := oracle.PandocRTFPictures(path, t.TempDir())
	if !res.Available {
		return false
	}
	if res.Err != nil {
		return true
	}
	for i, g := range got {
		for _, w := range want {
			if bytes.Equal(g, w.data) {
				t.Fatalf("%v at %d B: pandoc wrote picture %d out whole from binary, and picture_data says no png comes out whole - the sentence has to change", props, size, i+1)
			}
		}
	}
	return true
}

// rtfPandocReadings is what pandoc was measured to make of the paragraphs in
// other scripts and of the catalogue of every part. readsLiteral is pandoc
// 3.13 or later, which reads literal as it reads unicode.
func rtfPandocReadings(t *testing.T, text string, props map[string]string, size int64, readsLiteral bool) {
	t.Helper()
	if problem := rtfScriptsProblem(text, props, readsLiteral); problem != "" {
		t.Fatalf("%v at %d B: %s", props, size, problem)
	}
	if props[rtf.Elements] == rtf.Every {
		if !strings.Contains(text, "This sentence is in a footnote.") || strings.Contains(text, "This sentence is hidden text.") {
			t.Fatalf("%v at %d B: elements says pandoc returns the footnote and leaves the hidden text out, and it read:\n%s", props, size, text)
		}
	}
}

// rtfScriptsProblem is what is wrong with pandoc's reading of the paragraphs
// in other scripts, or nothing: read right in unicode and both, and in literal
// from pandoc 3.13, and the Polish line read wrong otherwise.
func rtfScriptsProblem(text string, props map[string]string, readsLiteral bool) string {
	if props[rtf.Characters] != rtf.International {
		return ""
	}
	enc := props[rtf.TextEncoding]
	if enc == "" || enc == rtf.Unicode || enc == rtf.Both || enc == rtf.Literal && readsLiteral {
		for _, line := range []string{rtfPolish, rtfRussian, rtfJapanese, rtfArabic} {
			if !strings.Contains(text, line) {
				return fmt.Sprintf("pandoc did not read %q", line)
			}
		}
		return ""
	}
	if strings.Contains(text, rtfPolish) {
		return fmt.Sprintf("pandoc read the Polish line right, and text_encoding says it shows %s wrong - the sentence has to change", enc)
	}
	return ""
}

// rtfSettings is a case as the words the structural checker is told, with the
// label the manifest says the file carries.
func rtfSettings(props map[string]string, labelled bool) []string {
	out := []string{"label=" + strconv.FormatBool(labelled)}
	for k, v := range props {
		out = append(out, k+"="+v)
	}
	sort.Strings(out)
	return out
}

// writeRTF plans and writes one document, and holds it to the size it was
// asked for.
func writeRTF(t *testing.T, d format.Descriptor, size int64, seed uint64, label bool, props map[string]string) ([]byte, format.Plan) {
	t.Helper()
	p, err := d.Generator.Plan(format.Request{Bytes: size, Seed: seed, Label: label, Properties: props})
	if err != nil {
		t.Fatalf("planning %d B with %v: %v", size, props, err)
	}
	var buf bytes.Buffer
	if err := d.Generator.Write(context.Background(), &buf, p); err != nil {
		t.Fatalf("writing %d B with %v: %v", size, props, err)
	}
	if int64(buf.Len()) != size {
		t.Fatalf("%v: asked for %d B and got %d", props, size, buf.Len())
	}
	return buf.Bytes(), p
}

// A document too small for its label goes without one and says so - the note
// and label_embedded, both - rather than being refused or rounded. The sizes are
// taken from either side of where the label starts to fit, so the guard is in
// the state it is about: it asserts the bare document fits and the labelled one
// does not, and then that one byte more is enough.
func TestAnRTFTooSmallForItsLabelSaysItCarriesNone(t *testing.T) {
	d, err := format.Get("rtf")
	if err != nil {
		t.Fatal(err)
	}
	fits := int64(-1)
	for size := d.MinBytes; size < d.MinBytes+200; size++ {
		p, err := d.Generator.Plan(format.Request{Bytes: size, Seed: 7, Label: true})
		if err != nil {
			t.Fatalf("%d B is above the minimum %d and was refused: %v", size, d.MinBytes, err)
		}
		if p.Properties[format.PropertyLabelEmbedded] == true {
			fits = size
			break
		}
		if len(p.Notes) != 1 || p.Notes[0].Code != "label_omitted" {
			t.Fatalf("at %d B the label is left out and the notes are %+v", size, p.Notes)
		}
	}
	if fits <= d.MinBytes {
		t.Fatalf("the label fits at %d B, so no size between the minimum %d and it was ever asked about", fits, d.MinBytes)
	}
}

// The manifest records every setting whatever was asked, under the names the
// owner approved on 2026-10-15 - public names under rule 10, so a key that
// changed spelling is somebody's test failing.
func TestTheRTFManifestRecordsEverySetting(t *testing.T) {
	d, err := format.Get("rtf")
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Generator.Plan(format.Request{Bytes: 4096, Seed: 7, Label: true})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"characters": "ascii", "text_encoding": "unicode", "elements": "basic", "nesting": 0,
		"code_page": 1252, format.PropertyLabelEmbedded: true,
		// The second piece, approved the same day (docs/RTF-2-2026-10-15.md 9),
		// and the five keys of the pictures a page and a message carry too.
		"links": 0, "picture_data": "hex",
		"inline": 0, "inline_format": "png", "inline_size": int64(8192), "inline_bytes": int64(0), "inline_encoded_bytes": int64(0),
	}
	if len(p.Properties) != len(want) {
		t.Errorf("the manifest carries %d keys, %v, and the approved list is %d", len(p.Properties), p.Properties, len(want))
	}
	for k, v := range want {
		if p.Properties[k] != v {
			t.Errorf("%s is %v in the manifest, and %v by default", k, p.Properties[k], v)
		}
	}
}

// Cut at any size, a document ends on the braces that close its groups and
// itself, with nothing that could escape them in front, and no line end stands
// between two letters - RTF ignores a line end, so one there joins two words in
// every reader (docs/RTF-2026-10-15.md section 3). Three hundred sizes in a row
// from the smallest, in the three shapes the end can take, with no tool outside
// this process, so it runs wherever the suite does.
func TestAnRTFCutAtAnySizeEndsOnItsBraces(t *testing.T) {
	d, err := format.Get("rtf")
	if err != nil {
		t.Fatal(err)
	}
	for _, props := range []map[string]string{{}, {rtf.Nesting: "3"}, {rtf.Characters: rtf.International, rtf.Elements: rtf.Every}, {rtf.Links: "18"}} {
		smallest := d.SmallestAccepted(format.Request{Properties: props, Label: true})
		closing := 1
		if props[rtf.Nesting] != "" {
			n, _ := strconv.Atoi(props[rtf.Nesting])
			closing += n
		}
		for size := smallest; size < smallest+300; size++ {
			b, _ := writeRTF(t, d, size, 99, true, props)
			rtfEndsOnItsBraces(t, b, closing, props, size)
		}
	}
}

func rtfEndsOnItsBraces(t *testing.T, b []byte, closing int, props map[string]string, size int64) {
	t.Helper()
	body := bytes.TrimRight(b, "}")
	if got := len(b) - len(body); got != closing {
		t.Fatalf("%v at %d B ends with %d braces, and closes %d groups", props, size, got, closing)
	}
	if bytes.HasSuffix(body, []byte(`\`)) {
		t.Fatalf("%v at %d B: a backslash right before the closing braces escapes the first of them", props, size)
	}
	for i := bytes.Index(b, []byte("\r\n")); i >= 0; i = nextLineEnd(b, i) {
		start := i
		for start > 0 && isLetterOrDigit(b[start-1]) {
			start--
		}
		if start < i && (start == 0 || b[start-1] != '\\') && i+2 < len(b) && isLetterOrDigit(b[i+2]) {
			t.Fatalf("%v at %d B: a line end between %q and %q joins two words", props, size, b[start:i], b[i+2:min(i+10, len(b))])
		}
	}
}

func nextLineEnd(b []byte, i int) int {
	j := bytes.Index(b[i+2:], []byte("\r\n"))
	if j < 0 {
		return -1
	}
	return i + 2 + j
}

func isLetterOrDigit(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}
