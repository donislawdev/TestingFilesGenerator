package guard

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// A file of the right size can still be the wrong file.
//
// The size guard walks ~120 sizes per format and says nothing about what is in
// them. The determinism guard compares two runs and says nothing either. The
// golden values notice a change but not what changed, and they would have to
// be re-measured for any deliberate edit - so they cannot be the guard that
// says "this is still a log".
//
// These are the structural guards for the text group, and mutation is what
// asked for them: truncating the closing entry of a log leaves the file at
// exactly the right size, so every guard above stayed green.

// combined is the Apache combined log format. Written here rather than shared
// with the generator, because a guard that reuses the code under test agrees
// with it whatever it does.
//
// The octet pattern forbids a leading zero on purpose. The first version of
// this guard demanded exactly three digits per octet, which is what the
// generator happened to produce - so it enforced our own defect instead of the
// format. Real logs write 93, not 093, and a leading zero is read as octal by
// some address parsers. A guard written to the output rather than to the
// specification agrees with whatever the code does, which is no guard at all.
const octet = `(?:0|[1-9]\d{0,2})`

var combined = regexp.MustCompile(
	`^` + octet + `\.` + octet + `\.` + octet + `\.` + octet +
		` - - \[[^\]]+\] "GET /\S* HTTP/1\.1" \d{3} \d+ "[^"]*" "[^"]*"$`)

// A log is read line by line, so a line that is not a whole entry is a broken
// file however right its length is. "The last line is truncated" is what a
// real log looks like caught mid rotation, which is why this cannot be left to
// somebody noticing.
func TestEveryLineOfALogIsAWholeEntry(t *testing.T) {
	// Sizes chosen to land the closing entry in different places: just above
	// the minimum, around the buffer boundary, and at awkward odd numbers.
	for _, size := range []int64{155, 156, 311, 512, 4096, 4097, 32769, 100000} {
		t.Run(sizeText(size), func(t *testing.T) {
			body := generateBytes(t, "log", size)
			if int64(len(body)) != size {
				t.Fatalf("produced %d B, expected %d B", len(body), size)
			}

			text := string(body)
			if !strings.HasSuffix(text, "\n") {
				t.Errorf("the file does not end with a newline, so the last entry is unterminated")
			}
			lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")

			entries := 0
			for i, line := range lines {
				if strings.HasPrefix(line, "# ") {
					continue // the label line, which says it is not an entry
				}
				if !combined.MatchString(line) {
					t.Errorf("line %d of %d is not a whole entry:\n  %q", i+1, len(lines), line)
					continue
				}
				entries++
			}
			if entries == 0 {
				t.Error("the file holds no entries at all, so this guard proved nothing")
			}
		})
	}
}

// Markdown is worth generating instead of text only because of the structure.
// A document that quietly stopped emitting blocks would be the right size,
// valid Markdown, and useless as a fixture for a renderer.
func TestAMarkdownDocumentCarriesRealStructure(t *testing.T) {
	// Large enough that every kind of block has come up. Below this the file
	// is legitimately mostly prose, which the size guard already covers.
	body := generateBytes(t, "md", 16384)
	text := string(body)
	lines := strings.Split(text, "\n")

	counts := map[string]int{}
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l, "## "):
			counts["heading"]++
		case strings.HasPrefix(l, "- "):
			counts["bullet"]++
		case strings.HasPrefix(l, "> "):
			counts["quote"]++
		case strings.HasPrefix(l, "| "):
			counts["table"]++
		case strings.HasPrefix(l, "```"):
			counts["fence"]++
		}
	}
	for _, kind := range []string{"heading", "bullet", "quote", "table", "fence"} {
		if counts[kind] == 0 {
			t.Errorf("a 16 KiB document carries no %s - the structure is what makes this format worth generating", kind)
		}
	}

	// An unclosed fence is the failure the block-or-prose split exists to
	// prevent. It renders, so nothing else would notice.
	if counts["fence"]%2 != 0 {
		t.Errorf("%d fence markers - the document ends inside a code block", counts["fence"])
	}

	// Tables have to be rectangular. A row cut short would still look like a
	// table to a reader skimming the file.
	pipes := map[int]int{}
	for _, l := range lines {
		if strings.HasPrefix(l, "| ") {
			pipes[strings.Count(l, "|")]++
		}
	}
	if len(pipes) != 1 {
		t.Errorf("table rows have differing column counts %v - one of them is cut short", pipes)
	}
}

// The record based formats share one failure nothing above would notice: the
// generator stops emitting records and lets the padding value swallow the file.
// The size stays exact, the run stays repeatable, and CSV, JSON and XML all
// still parse - a document holding one record with a megabyte long field is
// well formed and worthless.
//
// So the property is stated once, for all three: padding lands in the last
// record only, which puts a ceiling on any single value and a floor under the
// number of records.
const (
	// maxValueBytes is the most any single field, string or element text may
	// hold. The remainder to the exact byte is always smaller than one whole
	// record, so this is generous by a wide margin and still catches a collapse.
	maxValueBytes = 4096

	// densitySize is large enough that thousands of records have been written.
	densitySize = 128 << 10
)

// minRecords is derived from the ceiling above rather than picked separately.
// Two independent numbers would drift apart the first time one was tuned.
const minRecords = densitySize / maxValueBytes

func TestEveryRowOfACSVHasTheSameColumns(t *testing.T) {
	for _, size := range []int64{115, 116, 512, 4097, 32769, densitySize} {
		t.Run(sizeText(size), func(t *testing.T) {
			body := generateBytes(t, "csv", size)
			if int64(len(body)) != size {
				t.Fatalf("produced %d B, expected %d B", len(body), size)
			}
			if !strings.HasSuffix(string(body), "\n") {
				t.Error("the file does not end with a newline, so the last row is unterminated")
			}

			// encoding/csv fixes the column count from the first record and
			// refuses any row that differs, which is the ragged row this has to
			// catch. The Python module beside it is lenient about exactly that,
			// so the two readers answer different questions on purpose.
			r := csv.NewReader(bytes.NewReader(body))
			rows, err := r.ReadAll()
			if err != nil {
				t.Fatalf("the table does not parse: %v", err)
			}
			if len(rows) < 2 {
				t.Fatalf("the table holds %d row(s), so there is no data in it", len(rows))
			}

			for number, row := range rows {
				for _, field := range row {
					if len(field) > maxValueBytes {
						t.Errorf("row %d has a field of %d B - padding belongs in the last row, not in one enormous field",
							number+1, len(field))
					}
				}
			}

			if size == densitySize && len(rows)-1 < minRecords {
				t.Errorf("a %d B table holds %d data rows, expected at least %d - the padding has swallowed the file",
					size, len(rows)-1, minRecords)
			}
		})
	}
}

// A JSON fixture exists so a parser meets every value type. The types come from
// the format document, not from what the generator happens to emit - a
// generator that quietly stopped writing booleans would be the right size and
// would still parse.
func TestAJSONDocumentIsAnArrayOfRecordsWithEveryType(t *testing.T) {
	for _, size := range []int64{219, 220, 4097, densitySize} {
		t.Run(sizeText(size), func(t *testing.T) {
			body := generateBytes(t, "json", size)
			if int64(len(body)) != size {
				t.Fatalf("produced %d B, expected %d B", len(body), size)
			}

			var doc []map[string]any
			if err := json.Unmarshal(body, &doc); err != nil {
				t.Fatalf("the document does not parse as an array of objects: %v", err)
			}
			if len(doc) == 0 {
				t.Fatal("the array is empty")
			}

			kinds := map[string]bool{}
			var first []string
			for number, record := range doc {
				keys := make([]string, 0, len(record))
				for k := range record {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				if number == 0 {
					first = keys
				} else if strings.Join(keys, ",") != strings.Join(first, ",") {
					t.Fatalf("record %d has the keys %v and record 1 has %v", number+1, keys, first)
				}
				for _, v := range record {
					kinds[kindOf(v)] = true
					if s, ok := v.(string); ok && len(s) > maxValueBytes {
						t.Errorf("record %d holds a string of %d B - padding belongs in the last record",
							number+1, len(s))
					}
				}
			}

			for _, want := range []string{"null", "bool", "number", "string", "array", "object"} {
				if !kinds[want] {
					t.Errorf("no record carries a %s - a parser under test never meets that type", want)
				}
			}

			if size == densitySize && len(doc) < minRecords {
				t.Errorf("a %d B document holds %d records, expected at least %d - the padding has swallowed the file",
					size, len(doc), minRecords)
			}
		})
	}
}

// kindOf names the JSON type of a decoded value. Written against the six types
// the format document lists rather than against our record template.
func kindOf(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "bool"
	case float64:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	return "unknown"
}

// XML stops being well formed the moment a tag is left open or an ampersand
// reaches the file raw, and both leave the size exactly right. The decoder runs
// in strict mode, so an unknown entity is an error rather than something it
// quietly passes through.
func TestAnXMLDocumentIsWellFormedAndCarriesRecords(t *testing.T) {
	for _, size := range []int64{264, 265, 4097, densitySize} {
		t.Run(sizeText(size), func(t *testing.T) {
			body := generateBytes(t, "xml", size)
			if int64(len(body)) != size {
				t.Fatalf("produced %d B, expected %d B", len(body), size)
			}
			if !bytes.HasPrefix(body, []byte("<?xml ")) {
				t.Error("the document does not open with an XML declaration")
			}

			dec := xml.NewDecoder(bytes.NewReader(body))
			dec.Strict = true

			var (
				records    int
				attributes int
				escaped    bool
			)
			for {
				tok, err := dec.Token()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					t.Fatalf("the document is not well formed: %v", err)
				}
				switch el := tok.(type) {
				case xml.StartElement:
					if el.Name.Local == "record" {
						records++
					}
					attributes += len(el.Attr)
				case xml.CharData:
					if len(el) > maxValueBytes {
						t.Errorf("an element holds %d B of text - padding belongs in the last record", len(el))
					}
					// An escaped ampersand comes back as a bare one after
					// decoding. If it never appears, the fixture stopped
					// exercising entity handling.
					if bytes.Contains(el, []byte("&")) {
						escaped = true
					}
				}
			}

			if records == 0 {
				t.Fatal("the document holds no record elements")
			}
			if attributes == 0 {
				t.Error("no element carries an attribute - a fixture needs both attributes and elements")
			}

			// Only once the document is big enough to hold many records. A file
			// at the minimum carries a single record, and whether that one draws
			// a vendor with an ampersand is chance - demanding it there would be
			// a guard written to the output rather than to the format.
			if size == densitySize {
				if !escaped {
					t.Error("no text carries an escaped entity - the classic way a document stops being well formed goes unexercised")
				}
				if records < minRecords {
					t.Errorf("a %d B document holds %d records, expected at least %d - the padding has swallowed the file",
						size, records, minRecords)
				}
			}
		})
	}
}

// An SVG that parses can still draw nothing. Inkscape answers whether it
// renders, and it is an external tool that skips when it is missing - so the
// shape of the drawing is checked here too, where nothing can skip.
func TestAnSVGDrawingCarriesRealShapes(t *testing.T) {
	drawable := map[string]bool{
		"rect": true, "circle": true, "ellipse": true, "line": true,
		"polyline": true, "polygon": true, "path": true, "text": true,
	}

	for _, size := range []int64{194, 195, 4097, densitySize} {
		t.Run(sizeText(size), func(t *testing.T) {
			body := generateBytes(t, "svg", size)
			if int64(len(body)) != size {
				t.Fatalf("produced %d B, expected %d B", len(body), size)
			}
			text := string(body)
			if !strings.Contains(text, `xmlns="http://www.w3.org/2000/svg"`) {
				t.Error("the root carries no SVG namespace, so a renderer has no reason to draw it")
			}
			if !strings.Contains(text, "viewBox=") {
				t.Error("the root declares no viewBox")
			}

			// SVG is XML, so an unbalanced document is caught by the decoder in
			// strict mode rather than by anything written here.
			dec := xml.NewDecoder(bytes.NewReader(body))
			dec.Strict = true

			// An element that draws nothing does not count as one that does.
			//
			// "text" is on the list because a label is a real mark on the
			// canvas, and an empty text element is not. Counting the element
			// rather than what it holds is how this guard passed on a document
			// that was one empty text element and no shapes - valid SVG, the
			// exact size ordered, and a blank canvas. Found on 2026-08-03 by
			// pointing the renderer at the smallest size a format will make,
			// which nothing had ever done.
			shapes, pendingText := 0, false
			for {
				tok, err := dec.Token()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					t.Fatalf("the drawing is not well formed: %v", err)
				}
				switch el := tok.(type) {
				case xml.StartElement:
					if el.Name.Local == "text" {
						// Counted only once something is inside it.
						pendingText = true
						continue
					}
					if drawable[el.Name.Local] {
						shapes++
					}
				case xml.CharData:
					if pendingText && len(strings.TrimSpace(string(el))) > 0 {
						shapes++
						pendingText = false
					}
					if len(el) > maxValueBytes {
						t.Errorf("an element holds %d B of text - padding belongs in the closing label", len(el))
					}
				case xml.EndElement:
					if el.Name.Local == "text" {
						pendingText = false
					}
				}
			}

			if shapes == 0 {
				t.Fatal("the drawing holds nothing that draws anything")
			}
			if size == densitySize && shapes < minRecords {
				t.Errorf("a %d B drawing holds %d shapes, expected at least %d - the padding has swallowed the file",
					size, shapes, minRecords)
			}
		})
	}
}

// textFormats are the formats whose whole output is text, and binaryFormats is
// everything else. Between them they have to name every registered format.
//
// Two lists rather than one, and that is the repair. The check below fails on a
// name that is not a format, so neither list can point at nothing - but until
// 2026-08-05 nothing asked the other direction, and that direction is the one
// that matters. A format registered tomorrow simply would not have been checked
// for valid UTF-8, and the suite would have stayed green while covering less.
//
// This is the shape docs/OBSERVATIONS.md now calls out: an audit of
// completeness has to run FROM THE SOURCE towards the list. Walking the entries
// already written down cannot, by construction, find what is missing from them.
var textFormats = []string{"txt", "md", "log", "csv", "json", "xml", "html", "svg"}

var binaryFormats = []string{"avif", "bmp", "docx", "gif", "ico", "jpg", "jxl", "pdf", "png", "pptx", "targz", "tiff", "wav", "webp", "xlsx", "zip"}

// Every registered format is on exactly one of the two lists above.
//
// Cheap, and it is the only thing standing between "eight formats are checked
// for valid UTF-8" and "eight of the formats that existed when somebody last
// looked". Adding a format now forces the question rather than skipping it in
// silence.
func TestEveryFormatIsClassifiedAsTextOrBinary(t *testing.T) {
	said := map[string]string{}
	for _, id := range textFormats {
		said[id] = "text"
	}
	for _, id := range binaryFormats {
		if kind, twice := said[id]; twice {
			t.Errorf("%s is on both lists, so nobody can tell which it is (%s)", id, kind)
		}
		said[id] = "binary"
	}

	registered := map[string]bool{}
	for _, id := range format.IDs() {
		registered[id] = true
		if said[id] == "" {
			t.Errorf("the registry has %s and neither list names it, so nothing says whether its "+
				"output has to be valid UTF-8. Put it on textFormats or on binaryFormats.", id)
		}
	}
	for id := range said {
		if !registered[id] {
			t.Errorf("%s is classified and the registry no longer has it - remove the entry", id)
		}
	}
	if len(registered) == 0 {
		t.Fatal("no format was registered, so this guard would pass without checking anything")
	}
	t.Logf("%d format(s): %d text, %d binary", len(registered), len(textFormats), len(binaryFormats))
}

// Every record format pads its last value to an exact BYTE count and then cuts
// to length. Today every word in every vocabulary is ASCII, so a byte is a
// character and the cut always lands between two of them.
//
// The day that stops being true, the cut splits a character in half. Measured
// on 2026-08-02 with tools/probes/probe-utf8-filler.py, one format and a
// vocabulary of Polish words: 304 sizes, 304 files, **zero wrong sizes** and
// **86 files carrying invalid UTF-8**. The size is exact, so the size guard is
// green. The bytes repeat, so determinism is green. The file looks right.
//
// And most of the reference tools would not say anything either. Node reads a
// file with fs.readFileSync(path, "utf8"), which replaces a broken sequence
// with U+FFFD rather than refusing - so JSON.parse succeeds on a corrupt file.
// Python's open(encoding="utf-8") does refuse, so some formats are covered and
// others are not.
//
// This is not a defect today. It is a defect the moment somebody adds a locale
// pack, which the recipe schema already names - `locale` is a key this build
// refuses with "generated content is English only so far" - and which M5
// describes. The guard goes in before the code that needs it, which is how the
// first four guards in this project were built.
func TestEveryTextFormatIsValidUTF8(t *testing.T) {
	for _, id := range textFormats {
		d, err := format.Get(id)
		if err != nil {
			t.Errorf("textFormats names %q and the registry has no such format", id)
			continue
		}
		for _, size := range []int64{d.MinBytes, d.MinBytes + 1, 4097, densitySize} {
			t.Run(id+"/"+sizeText(size), func(t *testing.T) {
				body := generateBytes(t, id, size)
				if utf8.Valid(body) {
					return
				}
				// Point at the break rather than saying the file is bad, so
				// whoever reads this knows where to look.
				for i := 0; i < len(body); {
					r, n := utf8.DecodeRune(body[i:])
					if r == utf8.RuneError && n == 1 {
						from := max(0, i-30)
						t.Fatalf("byte %d of %d is not valid UTF-8, so a character was cut in half: %q",
							i, len(body), body[from:min(len(body), i+10)])
					}
					i += n
				}
				t.Fatal("the file is not valid UTF-8")
			})
		}
	}
}

// attrNum reads a numeric attribute off an element. Missing or unreadable
// counts as absent, and the callers treat absent as "this one constrains
// nothing" rather than guessing a value.
func attrNum(el xml.StartElement, name string) (float64, bool) {
	for _, a := range el.Attr {
		if a.Name.Local != name {
			continue
		}
		v, err := strconv.ParseFloat(a.Value, 64)
		return v, err == nil
	}
	return 0, false
}

// The label is the one thing inside a generated file that says what the file
// is, and SVG is the format here where it has to be painted rather than written
// into a comment. Painting has an order, and that order lost the label twice
// over - both found by looking at a render, neither by reading the code:
//
//   - the closing label sat on the same baseline and, written last, covered it
//   - shapes reached the bottom edge and covered both
//
// Nothing else notices. The size is exact either way, the document parses
// either way, and the render oracle counts colours - a page full of shapes is
// colourful whether or not anybody can read the corner.
func TestNothingIsPaintedOverTheSVGLabel(t *testing.T) {
	// A line of text inks roughly one em above its baseline and a third below.
	// Generous on purpose: a guard for "can this be read" should speak up
	// before the ink actually touches.
	const ascent, descent = 1.0, 0.34

	type span struct{ top, bottom float64 }
	hits := func(a, b span) bool { return a.top <= b.bottom && b.top <= a.bottom }

	// From 288 B up the drawing has room for the identity label beside a whole
	// shape. Below that it is left out on purpose, with a note saying so, and
	// that is a different behaviour with its own guard. 320 keeps this one near
	// the small end without sitting on the boundary.
	for _, size := range []int64{320, 4097, densitySize} {
		t.Run(sizeText(size), func(t *testing.T) {
			body := generateBytes(t, "svg", size)

			var texts, shapes []span

			dec := xml.NewDecoder(bytes.NewReader(body))
			dec.Strict = true
			for {
				tok, err := dec.Token()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					t.Fatalf("the drawing is not well formed: %v", err)
				}
				el, ok := tok.(xml.StartElement)
				if !ok {
					continue
				}
				switch el.Name.Local {
				case "text":
					y, okY := attrNum(el, "y")
					em, okEm := attrNum(el, "font-size")
					if !okY || !okEm {
						t.Fatal("a text element declares no baseline or no font size, so where it lands is left to the renderer")
					}
					texts = append(texts, span{y - em*ascent, y + em*descent})
				case "rect":
					y, okY := attrNum(el, "y")
					h, okH := attrNum(el, "height")
					if okY && okH {
						shapes = append(shapes, span{y, y + h})
					}
				case "circle":
					cy, okY := attrNum(el, "cy")
					r, okR := attrNum(el, "r")
					if okY && okR {
						shapes = append(shapes, span{cy - r, cy + r})
					}
				case "ellipse":
					cy, okY := attrNum(el, "cy")
					ry, okR := attrNum(el, "ry")
					if okY && okR {
						shapes = append(shapes, span{cy - ry, cy + ry})
					}
				case "line":
					y1, ok1 := attrNum(el, "y1")
					y2, ok2 := attrNum(el, "y2")
					if ok1 && ok2 {
						shapes = append(shapes, span{math.Min(y1, y2), math.Max(y1, y2)})
					}
				}
			}

			// Without both of them there is nothing to compare, and a guard
			// that quietly compares nothing is worse than no guard.
			if len(texts) < 2 {
				t.Fatalf("the drawing holds %d text elements, expected the identity label and the closing one", len(texts))
			}
			for i := range texts {
				for j := i + 1; j < len(texts); j++ {
					if hits(texts[i], texts[j]) {
						t.Errorf("two lines of text share the band %.0f..%.0f and %.0f..%.0f - whichever is written last covers the other, and the label is written first",
							texts[i].top, texts[i].bottom, texts[j].top, texts[j].bottom)
					}
				}
			}
			for _, s := range shapes {
				for _, tx := range texts {
					if hits(s, tx) {
						t.Fatalf("a shape spanning %.0f..%.0f crosses the text band %.0f..%.0f - shapes are drawn after the label, so the shape wins",
							s.top, s.bottom, tx.top, tx.bottom)
					}
				}
			}
		})
	}
}

// A record number that skips is not a broken file. It parses, it is exactly the
// size that was asked for, and it repeats byte for byte - so size, determinism
// and the golden values all stay green. It is a lie about the data instead, and
// for a tool whose whole point is telling a test suite what to expect, that is
// the worse kind: somebody asserts the ids run 1..N and gets a red run we
// caused.
//
// It came from the shared filling loop. That loop builds one record past the
// end to learn that it does not fit, throws the bytes away, and the builder had
// already counted it.
func TestRecordNumbersRunFromOneWithoutAGap(t *testing.T) {
	cases := []struct {
		format string
		number *regexp.Regexp
	}{
		{"csv", regexp.MustCompile(`(?m)^(\d+),`)},
		{"json", regexp.MustCompile(`\{"id":(\d+),`)},
		{"xml", regexp.MustCompile(`<record id="(\d+)"`)},
	}

	for _, c := range cases {
		for _, size := range []int64{1024, 4097, densitySize} {
			t.Run(c.format+"/"+sizeText(size), func(t *testing.T) {
				body := generateBytes(t, c.format, size)

				found := c.number.FindAllSubmatch(body, -1)
				// One record cannot show a gap, so a pass there would mean
				// nothing.
				if len(found) < 2 {
					t.Fatalf("found %d record numbers, too few for a gap to be visible", len(found))
				}

				for i, m := range found {
					n, err := strconv.Atoi(string(m[1]))
					if err != nil {
						t.Fatalf("record number %q does not read as a number: %v", m[1], err)
					}
					if want := i + 1; n != want {
						t.Fatalf("the %d record carries the number %d - the numbering skips, which is what happens when a record is built, counted, and then thrown away",
							want, n)
					}
				}
			})
		}
	}
}

// HTML is the weakest format here for checking, and that is the format's own
// doing: a parser is required to recover from almost anything, so the tolerant
// reader beside this accepts documents nobody would want. There is also only
// one such reader on this machine and no HTML parser in the standard library,
// so this scanner is written to the specification and carries the weight.
var (
	htmlTag    = regexp.MustCompile(`<(/?)([a-zA-Z][a-zA-Z0-9]*)([^>]*)>`)
	htmlEntity = regexp.MustCompile(`^&(?:[a-zA-Z][a-zA-Z0-9]{1,31}|#[0-9]+|#x[0-9a-fA-F]+);`)
)

func TestAnHTMLDocumentIsBalancedAndCarriesBlocks(t *testing.T) {
	// The HTML5 void elements. Anything else has to close.
	void := map[string]bool{
		"area": true, "base": true, "br": true, "col": true, "embed": true,
		"hr": true, "img": true, "input": true, "link": true, "meta": true,
		"param": true, "source": true, "track": true, "wbr": true,
	}
	block := map[string]bool{
		"p": true, "h1": true, "h2": true, "h3": true,
		"ul": true, "ol": true, "table": true, "blockquote": true,
	}

	for _, size := range []int64{118, 119, 4097, densitySize} {
		t.Run(sizeText(size), func(t *testing.T) {
			body := generateBytes(t, "html", size)
			if int64(len(body)) != size {
				t.Fatalf("produced %d B, expected %d B", len(body), size)
			}
			text := string(body)

			if !strings.HasPrefix(strings.ToLower(text), "<!doctype html>") {
				t.Error("the document does not open with an HTML5 doctype")
			}
			if !strings.HasSuffix(strings.TrimSpace(text), "</html>") {
				t.Error("the document does not end with a closing html tag")
			}

			var stack []string
			blocks, longest, last, escaped := 0, 0, 0, false
			for _, m := range htmlTag.FindAllStringSubmatchIndex(text, -1) {
				// Character data between the previous tag and this one. A bare
				// ampersand is the classic way a page stops being well formed,
				// and it leaves the size exactly right.
				chunk := text[last:m[0]]
				for pos := strings.IndexByte(chunk, '&'); pos >= 0; {
					if loc := htmlEntity.FindStringIndex(chunk[pos:]); loc == nil || loc[0] != 0 {
						t.Fatalf("a bare ampersand in %q - text has to escape it as &amp;", chunk)
					}
					escaped = true
					next := strings.IndexByte(chunk[pos+1:], '&')
					if next < 0 {
						break
					}
					pos += 1 + next
				}

				if run := m[0] - last; run > longest {
					longest = run
				}
				last = m[1]

				closing := m[3]-m[2] == 1
				name := strings.ToLower(text[m[4]:m[5]])
				rest := text[m[6]:m[7]]
				if void[name] || strings.HasSuffix(strings.TrimSpace(rest), "/") {
					continue
				}
				if closing {
					if len(stack) == 0 {
						t.Fatalf("</%s> closes an element that was never opened", name)
					}
					opened := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					if opened != name {
						t.Fatalf("</%s> closes while <%s> is open", name, opened)
					}
					continue
				}
				stack = append(stack, name)
				if block[name] {
					blocks++
				}
			}

			if len(stack) != 0 {
				t.Errorf("the document ends with %v still open", stack)
			}
			if blocks == 0 {
				t.Fatal("the body holds no block elements, so the page renders as nothing")
			}
			// Only where the document is big enough for every kind of block to
			// have come up. A page at the minimum carries one, and whether that
			// one carries an entity is chance.
			if size == densitySize && !escaped {
				t.Error("no text carries an escaped entity - the classic way a page stops being well formed goes unexercised")
			}
			if longest > maxValueBytes {
				t.Errorf("a run of %d B sits between two tags - padding belongs in the closing paragraph", longest)
			}
			if size == densitySize && blocks < minRecords {
				t.Errorf("a %d B page holds %d blocks, expected at least %d - the padding has swallowed the file",
					size, blocks, minRecords)
			}
		})
	}
}

// generateBytes produces one file of a format and returns its bytes, through
// the same plan and write path a run uses.
func generateBytes(t *testing.T, formatID string, size int64) []byte {
	t.Helper()
	desc, err := format.Get(formatID)
	if err != nil {
		t.Fatalf("no such format %q: %v", formatID, err)
	}
	plan, err := desc.Generator.Plan(format.Request{Bytes: size, Seed: 7741, Label: true})
	if err != nil {
		t.Fatalf("planning %d B of %s: %v", size, formatID, err)
	}
	var out strings.Builder
	if err := desc.Generator.Write(context.Background(), &out, plan); err != nil {
		t.Fatalf("writing %s: %v", formatID, err)
	}
	return []byte(out.String())
}
