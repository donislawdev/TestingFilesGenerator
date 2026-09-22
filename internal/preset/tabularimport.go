package preset

import (
	"strconv"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

const (
	tabularID = "tabular-import"

	rowsParam      = "rows"
	columnsParam   = "columns"
	defaultRows    = "1000"
	defaultColumns = "10"

	// The ranges the spreadsheet declares, written here because a preset is
	// registered at init and the format registry has not necessarily finished
	// filling by then - the same reason Global() reads the formats when it is
	// called rather than when it is declared.
	//
	// A number copied by hand goes stale green, so this pair is not left to
	// care: TestTheTabularPresetTakesTheRangesTheSheetDeclares asks the
	// registry and reddens when they drift.
	sheetRowsMax    = 200000
	sheetColumnsMax = 32768

	dialectGroup = "csv-dialects"
	wideGroup    = "csv-wide"
	sheetGroup   = "xlsx-rows"
	layoutGroup  = "json-layouts"

	// tableSample is how big the files that are about SHAPE rather than size
	// are - the dialects and the layouts. Big enough to hold hundreds of rows,
	// so a reader that copes with the first row and not the hundredth is
	// caught, and small enough that eleven of them are noise beside the
	// spreadsheet.
	tableSample = 64 << 10

	// The settings this set varies, spelled here because no package owns these
	// names the way textenc owns the encoding. What keeps them honest is the
	// registry: the axes are read from what CSV declares, and these are only
	// used to say which of them is a matter of policy.
	csvDelimiter  = "delimiter"
	csvHeader     = "header"
	csvQuoteStyle = "quote_style"
	jsonFormat    = "formatting"

	// tabularQuestion is announced by the preset AND written into the header of
	// an ejected recipe. Named once rather than typed twice.
	tabularQuestion = "Does my table import survive what real tools export?"
)

func init() {
	Register(Preset{
		ID:       tabularID,
		Title:    "Tabular import",
		Question: tabularQuestion,

		Parameters: []format.Property{
			{
				Name: rowsParam, Kind: format.PropertyInt,
				Min: 1, Max: sheetRowsMax, Unit: "rows",
				Default: defaultRows,
				Detail: "How many rows the spreadsheet holds. It is written at exactly the size " +
					"that many rows package to, so the budget above moves with this.",
			},
			{
				Name: columnsParam, Kind: format.PropertyInt,
				Min: 1, Max: sheetColumnsMax, Unit: "columns",
				Default: defaultColumns,
				Detail: "How many columns each row of the spreadsheet has. Rows times columns " +
					"has a ceiling, and asking past it is refused before anything is written.",
			},
		},

		Requires: []string{"MVP"},
		Catches: []string{
			"a semicolon file read as one column, because the delimiter was assumed rather than looked for",
			"a CRLF file split into rows with an empty row after each one",
			"a headerless table whose first row of data is eaten as column names",
			"an import that keeps the columns it can show and drops the rest without a word",
			"a reader that takes JSON records one line at a time and stops at the first indented document",
		},

		Expand: expandTabularImport,
	})
}

// dialectPolicy says whether changing one CSV setting produces a file any
// reader that takes CSV has to cope with, or a file whose acceptance is that
// reader's own policy.
//
// The line is drawn where a standard draws it. RFC 4180 asks for CRLF and for
// quoting, so a reader that breaks on either has a defect rather than a policy.
// A semicolon file is what a European spreadsheet exports and it is a different
// dialect - a reader that takes commas only and says so is a correct reader,
// and MF5 forbids inventing the answer for it. A table with no header row is
// the same kind of question.
//
// Every axis CSV declares reaches the set whether or not it is named here -
// TestEveryDialectTheTableDeclaresIsInTheSet asks the registry for that. What
// an axis missing from this map loses is the stronger half of its verdict: it
// lands on the policy side and says "your call" where it might have said "this
// has to work". A weaker claim rather than a wrong one, which is the right way
// round for a default nobody chose.
var dialectPolicy = map[string]bool{
	lineEndingSetting: false,
	csvQuoteStyle:     false,
	csvDelimiter:      true,
	csvHeader:         true,
}

// dialectFiles is the base table and one file for every other value of every
// setting CSV varies by.
//
// One axis at a time rather than the product, and that is the whole design of
// this group. Four delimiters times two line endings times two headers times
// three quote styles is forty eight files that say LESS than eight, because a
// failure in one of them names no cause. Eight files name one setting each.
//
// The base is a file of its own rather than four files restating it. Every
// setting at its declared default appears once, under the name default, and
// each of the other seven differs from it in exactly one place.
func dialectFiles(csv format.Descriptor) []setFile {
	base := map[string]string{}
	for _, axis := range dialectAxes(csv) {
		base[axis.Name] = axis.Default
	}
	out := []setFile{{
		id: "dialect_default", name: "default.csv", group: dialectGroup,
		desc: csv, props: base, size: tableSample, expected: "accept",
	}}

	for _, axis := range dialectAxes(csv) {
		out = append(out, dialectVariants(csv, base, axis)...)
	}
	return out
}

// dialectVariants is one file for every value of one setting except the one the
// base already stands for.
func dialectVariants(csv format.Descriptor, base map[string]string, axis format.Property) []setFile {
	var out []setFile
	for _, value := range valuesOf(axis) {
		if value == axis.Default {
			continue
		}
		file := setFile{
			id:    "dialect_" + axis.Name + "_" + value,
			name:  axis.Name + "_" + value + ".csv",
			group: dialectGroup, desc: csv, props: besides(base, axis.Name, value),
			size: tableSample, expected: "accept",
		}
		if dialectPolicy[axis.Name] {
			file.expected, file.reason = "unspecified", "none"
		}
		out = append(out, file)
	}
	return out
}

// besides is the base settings with one of them changed, leaving the base as
// it was. Every file of the group differs from it in exactly one place, so the
// copy is what keeps that true.
func besides(base map[string]string, name, value string) map[string]string {
	out := make(map[string]string, len(base))
	for k, v := range base {
		out[k] = v
	}
	out[name] = value
	return out
}

// dialectAxes is every setting of a format that names a shape rather than a
// size, in the order the format declares them.
//
// Read from the registry, so a fifth dialect setting joins the set without a
// line changing here. The whole numbers are left out because they are counts
// rather than dialects, and the one that matters has a group of its own.
func dialectAxes(desc format.Descriptor) []format.Property {
	var out []format.Property
	for _, p := range desc.Properties {
		if p.Kind == format.PropertyChoice || p.Kind == format.PropertyBool {
			out = append(out, p)
		}
	}
	return out
}

// valuesOf is what one setting can be, for a choice and for a switch.
func valuesOf(p format.Property) []string {
	if p.Kind == format.PropertyBool {
		return []string{"false", "true"}
	}
	return p.Choices
}

// wideFile is the table with more columns than a spreadsheet will show.
//
// The count is the most this build writes rather than a number chosen here,
// because the registry is the only place that knows it and the file is about
// the ceiling. Its size is whatever that many columns need, so this one file
// sets the floor of the whole set and cannot be made smaller.
func wideFile(csv format.Descriptor) setFile {
	columns, _ := declared(csv, columnsParam)
	return setFile{
		id: "wide_table", name: "wide.csv", group: wideGroup, desc: csv,
		props:   map[string]string{columnsParam: strconv.FormatInt(columns.Max, 10)},
		atFloor: true,
		// A spreadsheet shows what it can show and drops the rest without
		// saying so. Whether an import should refuse such a table, truncate it
		// or take it whole is its owner's decision, so this is a position
		// rather than a promise - MF5.
		expected: "unspecified", reason: "count_limit",
	}
}

// sheetFile is the spreadsheet at the row and column counts asked for.
func sheetFile(xlsx format.Descriptor, rows, columns string) setFile {
	return setFile{
		id: "sheet", name: "sheet.xlsx", group: sheetGroup, desc: xlsx,
		props:    map[string]string{rowsParam: rows, columnsParam: columns},
		atFloor:  true,
		expected: "accept",
	}
}

// layoutFiles is the same records written the three ways a document can be
// laid out.
//
// All three are accepted by every JSON reader - the format says so in its own
// declaration - so this group is stated rather than left open. What it finds is
// a consumer that is not a JSON reader at all but a loop over lines.
func layoutFiles(js format.Descriptor) []setFile {
	layout, ok := declared(js, jsonFormat)
	if !ok {
		return nil
	}
	out := make([]setFile, 0, len(layout.Choices))
	for _, value := range layout.Choices {
		out = append(out, setFile{
			id:   "layout_" + strings.ReplaceAll(value, "-", "_"),
			name: value + ".json", group: layoutGroup, desc: js,
			props: map[string]string{jsonFormat: value},
			size:  tableSample, expected: "accept",
		})
	}
	return out
}

// refusedTable turns a refusal from the registry into one about the setting
// somebody typed.
//
// The sheet refuses rows times columns above its ceiling, and the sentence it
// writes is the right one - it names both counts, the limit and why there is
// one. What it cannot know is that those two numbers came from parameters of a
// preset rather than from a recipe, so this is where the refusal learns which
// box to stand beside.
func refusedTable(f setFile) error {
	err := f.refused()
	if err == nil {
		return nil
	}
	return &ImpossibleError{
		Preset:  tabularID,
		Setting: settingBehind(f),
		// No hint of our own. The sheet's refusal ends with what to do about it
		// and says it better than a general sentence could, because it knows
		// which of the two counts it was and by how much.
		Detail: strings.TrimPrefix(err.Error(), f.desc.ID+": "),
	}
}

// settingBehind is the parameter a file of this set was built from, so a
// refusal can stand beside the box that caused it. Empty where the file is
// built from no parameter at all.
func settingBehind(f setFile) string {
	if f.group == sheetGroup {
		return rowsParam
	}
	return ""
}

func expandTabularImport(args Args) ([]byte, error) {
	csv, err := format.Get("csv")
	if err != nil {
		return nil, err
	}
	xlsx, err := format.Get("xlsx")
	if err != nil {
		return nil, err
	}
	js, err := format.Get("json")
	if err != nil {
		return nil, err
	}

	files := dialectFiles(csv)
	files = append(files, wideFile(csv), sheetFile(xlsx, args[rowsParam], args[columnsParam]))
	files = append(files, layoutFiles(js)...)

	for _, f := range files {
		if err := refusedTable(f); err != nil {
			return nil, err
		}
	}
	return plan{preset: tabularID, question: tabularQuestion, targets: draftsOf(files)}.source()
}
