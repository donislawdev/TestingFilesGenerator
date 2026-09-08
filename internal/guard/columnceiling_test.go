package guard

import (
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// spreadsheetWidth is where a spreadsheet stops, measured rather than
// remembered: 2026-09-03 for CSV and again 2026-09-08 for XLSX, both with
// LibreOffice Calc 26.2.5.2 headless. A table of this many columns comes back
// whole. One column more comes back with this many, the last column dropped,
// exit 0 and not one word on either stream.
const spreadsheetWidth = 16384

// narrowerOnPurpose names a format whose columns cannot reach past a
// spreadsheet because the format itself stops first, with the reason.
//
// It is empty, and that is the point rather than an oversight. A format ends up
// here only when its OWN structure caps it - the way an icon stores each side
// in a single byte - and never because a smaller number felt tidier. Writing
// the reason down is the price of the exception, which is what stops this
// becoming the sort of list somebody adds to instead of thinking.
var narrowerOnPurpose = map[string]string{}

// A format with columns has to offer more of them than a spreadsheet accepts.
//
// The ceiling of a setting belongs to the reader under test, not to what this
// tool finds comfortable to write. Standing on both sides of a limit is what a
// boundary set is for, so a ceiling that stops at the limit offers the last
// table that survives and never the first that does not.
//
// This is not hypothetical tidiness, it is a defect this project shipped.
// XLSX declared 64 from the day it was written until 2026-09-08, with the
// reason "the width a person would actually look at" - which describes a
// document somebody reads rather than a fixture somebody tests with. CSV asked
// the same question of the same reader and answered 32768. Nothing compared
// them, and at 64 there was no way to build a sheet that asks Excel about its
// own limit at all.
//
// What it asks about is the OFFER, because that is what a ceiling is. That the
// file itself works was measured on 2026-09-08 rather than asserted here: a
// workbook of 16385 columns built by this tool converts through Calc at exit 0
// and comes back with 16384, which is the same silent loss a workbook from
// another writer produces. Building one costs twelve megabytes, which is not a
// price worth paying on every run of this suite.
//
// Asked of every registered format rather than of the two that have the
// setting today, so a third tabular format is covered on the day it arrives.
func TestAFormatWithColumnsReachesPastWhatASpreadsheetAccepts(t *testing.T) {
	examined := 0

	for _, d := range format.All() {
		for _, p := range d.Properties {
			if p.Name != "columns" || p.Kind != format.PropertyInt {
				continue
			}
			if why, narrow := narrowerOnPurpose[d.ID]; narrow {
				if p.Max > spreadsheetWidth {
					t.Errorf("%s is excused as narrower on purpose (%s) and reaches %d anyway - take the excuse off",
						d.ID, why, p.Max)
				}
				continue
			}
			examined++
			if p.Max <= spreadsheetWidth {
				t.Errorf("%s offers at most %d columns and a spreadsheet accepts %d, "+
					"so no fixture built from it can ask the reader about its own limit - "+
					"raise the ceiling, or name the structural reason in narrowerOnPurpose",
					d.ID, p.Max, spreadsheetWidth)
			}
		}
	}

	if examined == 0 {
		t.Fatal("no format offers a column count, so this proved nothing")
	}
}
