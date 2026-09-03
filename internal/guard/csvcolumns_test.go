package guard

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/oracle"
)

// csvColumnBounds is the range the registry offers, read from the declaration
// rather than written out here. A pair copied into a guard stops describing the
// thing it guards the moment somebody moves it.
func csvColumnBounds(t *testing.T) (min, max int64) {
	t.Helper()
	d, err := format.Get("csv")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range d.Properties {
		if p.Name != "columns" {
			continue
		}
		if p.Min <= 0 || p.Max <= p.Min {
			t.Fatalf("csv declares columns as %d to %d, so this guard would walk nothing", p.Min, p.Max)
		}
		return p.Min, p.Max
	}
	t.Fatal("csv declares no columns setting, so this guard would walk nothing")
	return 0, 0
}

// The table has the columns that were asked for, and the count moves the floor.
//
// What can be wrong here splits three ways, and only the first is what anybody
// would think to check.
//
// The count could be stored and ignored, which a size guard cannot see: a table
// of five columns where six were ordered is exactly as long, parses everywhere,
// and every row agrees with every other. So the FILE is counted, with a reader
// written in another language from the one that wrote it.
//
// The floor could miss that the count moves it. Every column adds its own width
// and a separator, so a floor worked out for six is wrong for every other
// number - and wrong in the SAFE direction going up, which nothing else would
// notice. That is asked by taking the floor the format announces and the byte
// below it, at each width.
//
// And the description could stop being last. It is the field the closing row
// stretches to reach an exact length, so a table that put it anywhere else
// would still be the right size while the padding landed in the middle of a
// row. Nothing about the length would change.
func TestTheCSVColumnCountIsInTheFileAndMovesTheFloor(t *testing.T) {
	min, max := csvColumnBounds(t)
	widths := []int64{min, min + 1, 5, 6, 7, 9, 40, max}
	seen := map[int64]bool{}

	for _, n := range widths {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			props := map[string]string{"columns": fmt.Sprint(n)}
			d, err := format.Get("csv")
			if err != nil {
				t.Fatal(err)
			}

			floor := csvFloor(t, props)
			seen[floor] = true
			if _, err := d.Generator.Plan(format.Request{Bytes: floor, Seed: 1, Label: true,
				Properties: props}); err != nil {
				t.Errorf("announces %d B as its floor for %d columns and then refuses it: %v", floor, n, err)
			}
			if _, err := d.Generator.Plan(format.Request{Bytes: floor - 1, Seed: 1, Label: true,
				Properties: props}); err == nil {
				t.Errorf("took %d B, one below the %d B it calls its floor for %d columns", floor-1, floor, n)
			}

			// Exact to the byte, at seeds rather than at sizes somebody picked.
			// The closing row is stretched to reach the length and every column
			// feeds into that arithmetic.
			for seed := uint64(1); seed <= 4; seed++ {
				for _, extra := range []int64{0, 1, 2, 733} {
					writeCSV(t, floor+extra, seed, props) // fails inside on a miss
				}
			}

			// Counted with encoding/csv, which is a different implementation
			// from the Python module the oracle uses and from the hand written
			// checker beside it.
			body := writeCSV(t, floor+733, 9, props)
			rows, err := csv.NewReader(bytes.NewReader(body)).ReadAll()
			if err != nil {
				t.Fatalf("%d columns: the table does not parse: %v", n, err)
			}
			if len(rows) < 2 {
				t.Fatalf("%d columns: the table holds %d row(s), so there is no data in it", n, len(rows))
			}
			for i, row := range rows {
				if int64(len(row)) != n {
					t.Fatalf("row %d has %d fields and %d columns were ordered", i+1, len(row), n)
				}
			}

			// The description is last, and it is the one the padding went into.
			if got := rows[0][len(rows[0])-1]; got != "description" {
				t.Errorf("the last column of the header is %q rather than description, and that is the "+
					"field the closing row stretches - moved anywhere else, the padding lands in the "+
					"middle of a row and the size stays perfect", got)
			}
			closing := rows[len(rows)-1]
			widest := 0
			for i, field := range closing {
				if len(field) > len(closing[widest]) {
					widest = i
				}
			}
			if widest != len(closing)-1 {
				t.Errorf("the longest field of the closing row is column %d of %d, so the padding did not "+
					"go into the description", widest+1, len(closing))
			}
		})
	}

	// The floor is not one number wearing every hat. Without this the whole
	// test above would pass on a build that worked the floor out for six
	// columns and handed it to the rest.
	if len(seen) != len(widths) {
		t.Errorf("%d widths produced %d distinct floors, and every one of them should differ - each "+
			"column adds its own width and a separator", len(widths), len(seen))
	}
}

// The six columns this format started with are still those six, in that order.
//
// Separate from everything above because it is the anchor rather than a
// property: the pinned bytes say the default file has not moved, and this says
// WHY in words, so a failure names the column that changed instead of handing
// over two hashes that differ.
func TestTheDefaultCSVStillHasItsSixOriginalColumns(t *testing.T) {
	body := writeCSV(t, 8192, 9, map[string]string{})
	header, _, ok := strings.Cut(string(body), "\n")
	if !ok {
		t.Fatal("the file has no first line")
	}
	const want = "id,name,email,amount,created,description"
	if header != want {
		t.Errorf("the default header is %q and it has always been %q. Changing it is a breaking change "+
			"under D11, so it needs the owner, a major version and a changelog entry", header, want)
	}
}

// Every width is well formed, judged by the checker rather than by us.
//
// The checker is TOLD the width, which is what makes this worth running beside
// the counting above. Counting can only ask whether the rows agree with each
// other, and a table that wrote the wrong number of columns agrees with itself
// perfectly. Being told, it can refuse - and the last case here feeds it a
// width that is not the file's, because a checker that accepted that would make
// every pass above worth nothing.
func TestEveryCSVWidthIsWellFormed(t *testing.T) {
	min, max := csvColumnBounds(t)
	dir := t.TempDir()

	check := func(t *testing.T, body []byte, name string, settings ...string) oracle.Result {
		t.Helper()
		path := filepath.Join(dir, name+".csv")
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatal(err)
		}
		return oracle.Strict("csv", path, settings...)
	}

	ran := 0
	widths := []int64{min, 3, 6, 12, 40, max}
	for _, n := range widths {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			props := map[string]string{"columns": fmt.Sprint(n)}
			body := writeCSV(t, csvFloor(t, props)+733, 9, props)
			res := check(t, body, fmt.Sprintf("w%d", n), fmt.Sprintf("columns=%d", n))
			if !res.Available {
				t.Skip("the structural check needs python")
			}
			ran++
			if res.Err != nil {
				t.Errorf("%d columns is not well formed: %v", n, res.Err)
			}
		})
	}
	if ran == 0 {
		t.Skip("the structural check never ran, so nothing here was judged")
	}
	if ran != len(widths) {
		t.Errorf("%d of %d widths reached the checker", ran, len(widths))
	}

	body := writeCSV(t, 8192, 4, map[string]string{"columns": "9"})
	for _, wrong := range []string{"columns=8", "columns=10"} {
		res := check(t, body, "wrong_"+wrong, wrong)
		if !res.Available {
			t.Skip("the structural check needs python")
		}
		if res.Err == nil {
			t.Errorf("a nine column table called %s passed the checker, so being told the width made it "+
				"a rubber stamp: %s", wrong, firstLineOf(res.Output))
		}
	}
}
