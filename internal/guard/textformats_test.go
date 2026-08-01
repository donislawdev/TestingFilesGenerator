package guard

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"io"
	"regexp"
	"sort"
	"strings"
	"testing"

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
	for _, size := range []int64{117, 118, 512, 4097, 32769, densitySize} {
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
				if err == io.EOF {
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
