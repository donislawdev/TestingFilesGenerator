package guard

// How a JSON document is laid out, and why that needed a guard of its own.
//
// Measured 2026-09-07 on two implementations in two languages: the same records
// minified, one per line, indented by two and indented by four all parse in
// CPython's json and in V8. So no reader can tell this guard whether the layout
// that was ordered is the layout in the file - which is exactly the question,
// and the reason the structural checker is TOLD the layout rather than left to
// work it out.
//
// The canary at the bottom is the half that makes the rest mean anything: a
// checker told the wrong layout has to refuse. Without it, a checker that
// looked at the setting and shrugged would pass every case above.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/jsonfile"
	"github.com/donislawdev/TestingFilesGenerator/internal/oracle"
)

var jsonLayouts = []string{jsonfile.Indented, jsonfile.Minified, jsonfile.RecordPerLine}

func jsonProps(layout string) map[string]string {
	return map[string]string{jsonfile.Formatting: layout}
}

// writeJSONDocument produces one document and insists on the ordered size before
// anything else looks at it.
func writeJSONDocument(t *testing.T, size int64, props map[string]string) []byte {
	t.Helper()
	d, err := format.Get("json")
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Generator.Plan(format.Request{Bytes: size, Seed: 7741, Label: true, Properties: props})
	if err != nil {
		t.Fatalf("planning %d B with %v: %v", size, props, err)
	}
	var buf bytes.Buffer
	if err := d.Generator.Write(context.Background(), &buf, p); err != nil {
		t.Fatalf("writing %d B with %v: %v", size, props, err)
	}
	if int64(buf.Len()) != size {
		t.Fatalf("%v: ordered %d B and produced %d - the size is exact or it is an error",
			props, size, buf.Len())
	}
	return buf.Bytes()
}

// TestAJSONDocumentIsLaidOutTheWayItWasOrdered is the claim: exact size, a
// layout somebody can see, and a reader that agrees the file is what it says.
//
// The line arithmetic is asserted here as well as in the checker, and that is
// not a duplicate. This one says what each layout IS - minified holds no
// newline at all, a record per line means one line each - in a place where the
// failure names the layout. The checker says the same thing to anybody running
// the tool without Go.
func TestAJSONDocumentIsLaidOutTheWayItWasOrdered(t *testing.T) {
	dir := t.TempDir()
	d, err := format.Get("json")
	if err != nil {
		t.Fatal(err)
	}
	checked, skipped := 0, 0

	for _, layout := range jsonLayouts {
		props := jsonProps(layout)
		smallest := d.SmallestAccepted(format.Request{Seed: 7741, Label: true, Properties: props})

		for _, size := range []int64{smallest, smallest + 1, 4096, 65536} {
			t.Run(fmt.Sprintf("%s/%d", layout, size), func(t *testing.T) {
				body := writeJSONDocument(t, size, props)
				newlines := int64(bytes.Count(body, []byte("\n")))

				switch layout {
				case jsonfile.Minified:
					if newlines != 0 {
						t.Errorf("minified holds %d newline(s), and minified means none", newlines)
					}
					if bytes.HasSuffix(body, []byte("\n")) {
						t.Error("minified ends with a newline, so it is not minified")
					}
				default:
					if newlines < 3 {
						t.Errorf("%s holds %d newline(s), which is not laid out at all", layout, newlines)
					}
					if !bytes.HasSuffix(body, []byte("]\n")) {
						t.Errorf("%s does not close with the array and a newline", layout)
					}
				}

				path := filepath.Join(dir, fmt.Sprintf("%s-%d.json", layout, size))
				if err := os.WriteFile(path, body, 0o600); err != nil {
					t.Fatal(err)
				}
				res := oracle.Strict("json", path, jsonfile.Formatting+"="+layout)
				if !res.Available {
					skipped++
					t.Skip("the structural check needs python")
				}
				if res.Err != nil {
					t.Fatalf("%s is not a well formed %s document: %v", path, layout, res.Err)
				}
				checked++
			})
		}
	}

	if checked == 0 {
		t.Errorf("nothing was read by anything outside this package - %d case(s) skipped", skipped)
	}
	t.Logf("%d document(s) read by the structural checker, %d skipped", checked, skipped)
}

// TestEachJSONLayoutAnswersForItsOwnMinimum holds the floor to the layout.
//
// A layout that opens every value onto its own line needs more room for one
// record than a layout with no whitespace at all, so one declared number
// cannot serve all three. The registry declares the DEFAULT layout's floor and
// the generator answers for the rest, which is how CSV does it - and what makes
// the two agree is that SmallestAccepted asks the generator rather than reading
// the declaration.
func TestEachJSONLayoutAnswersForItsOwnMinimum(t *testing.T) {
	d, err := format.Get("json")
	if err != nil {
		t.Fatal(err)
	}

	floors := map[string]int64{}
	for _, layout := range jsonLayouts {
		props := jsonProps(layout)
		floor := d.SmallestAccepted(format.Request{Seed: 7741, Label: true, Properties: props})
		floors[layout] = floor

		if _, err := d.Generator.Plan(format.Request{
			Bytes: floor, Seed: 7741, Label: true, Properties: props}); err != nil {
			t.Errorf("%s: the floor it reports is %d B and that is refused: %v", layout, floor, err)
		}

		_, err := d.Generator.Plan(format.Request{
			Bytes: floor - 1, Seed: 7741, Label: true, Properties: props})
		var below *format.BelowMinimumError
		if !errors.As(err, &below) {
			t.Errorf("%s: one byte under the floor was answered with %v, not a BelowMinimumError", layout, err)
			continue
		}
		// The refusal says which layout it is about, because the same size is
		// legal in another one and a person reading it needs to know that.
		if !strings.Contains(below.Reason, layout) {
			t.Errorf("%s: the refusal does not name the layout it is about: %q", layout, below.Reason)
		}
	}

	// An opened out record cannot need the same room as one with no whitespace.
	// Equal floors would mean the layout never reached the arithmetic.
	if floors[jsonfile.Indented] <= floors[jsonfile.RecordPerLine] ||
		floors[jsonfile.RecordPerLine] <= floors[jsonfile.Minified] {
		t.Errorf("the floors are %v, and indented has to need more room than a record per line, which needs more than minified",
			floors)
	}
	t.Logf("floors: %v", floors)
}

// TestTheDefaultLayoutIsTheBytesJSONAlwaysWrote is the way back.
//
// The same pin the text formats got when encoding arrived: saying nothing and
// saying the default out loud have to be one file, because a default that moves
// is a breaking change wearing the clothes of a feature.
func TestTheDefaultLayoutIsTheBytesJSONAlwaysWrote(t *testing.T) {
	for _, size := range []int64{219, 1024, 65536} {
		silent := writeJSONDocument(t, size, nil)
		spoken := writeJSONDocument(t, size, jsonProps(jsonfile.RecordPerLine))
		if !bytes.Equal(silent, spoken) {
			t.Errorf("at %d B: saying nothing and saying record-per-line produce different bytes", size)
		}
	}
}

// TestTheJSONCheckerRefusesALayoutTheFileIsNot is the canary.
//
// Every case above hands the checker a file and the name of the layout it was
// written in, and a checker that ignored the name would pass all of them. This
// hands it every WRONG name instead. Six pairs, six refusals, or the layer
// underneath is a rubber stamp.
func TestTheJSONCheckerRefusesALayoutTheFileIsNot(t *testing.T) {
	dir := t.TempDir()
	refused := 0

	for _, written := range jsonLayouts {
		body := writeJSONDocument(t, 4096, jsonProps(written))
		path := filepath.Join(dir, written+".json")
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatal(err)
		}
		for _, told := range jsonLayouts {
			if told == written {
				continue
			}
			res := oracle.Strict("json", path, jsonfile.Formatting+"="+told)
			if !res.Available {
				t.Skip("the structural check needs python")
			}
			if res.Err == nil {
				t.Errorf("a %s document was called a well formed %s one", written, told)
				continue
			}
			refused++
		}
	}

	if refused != 6 {
		t.Errorf("%d of the 6 wrong pairings were refused", refused)
	}
}
