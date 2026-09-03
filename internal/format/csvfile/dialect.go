// The dialect: the four ways a CSV file differs before its contents do.
//
// A tester does not usually get to choose the CSV they are handed. A European
// export separates with a semicolon, anything written on Windows ends its rows
// with CRLF, a feed dumped straight out of a database has no header row at
// all, and a file written by a spreadsheet may quote every field or none. All
// of them parse as CSV and all of them break readers that assumed the other
// thing, which is why they are settings rather than a fixed shape.
//
// Every value here leaves the file exactly the size that was asked for. What
// moves is the minimum: a CRLF row is a byte longer than an LF one, a file
// with no header has one fewer line to pay for, and a file that quotes every
// field pays two bytes for each of them.
package csvfile

import (
	"bytes"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// Setting names. Public names, so they are spelled once.
const (
	Delimiter  = "delimiter"
	LineEnding = "line_ending"
	Header     = "header"
	QuoteStyle = "quote_style"
)

// delimiters are the separators offered, by name rather than by character.
//
// By name because the alternative does not survive the trip. A tab cannot be
// typed as a flag value without an escape, a pipe is a shell metacharacter, and
// this repository has recorded more than a dozen occasions where a backslash
// went missing between a shell and a file. A word has none of those problems.
//
// Every one of them is a single byte, which is what lets the row arithmetic
// stay as it was. A separator of two bytes would move every width here.
var delimiters = map[string]byte{
	"comma":     ',',
	"semicolon": ';',
	"tab":       '\t',
	"pipe":      '|',
}

// delimiterIDs is the closed set the registry offers, in one order so that
// every surface lists them the same way.
var delimiterIDs = []string{"comma", "semicolon", "tab", "pipe"}

var lineEndings = map[string]string{
	"lf":   "\n",
	"crlf": "\r\n",
}

// lineEndingIDs is the closed set, and it is deliberately the same vocabulary
// the log format uses. One setting under one name means one thing, whichever
// format offers it.
var lineEndingIDs = []string{"lf", "crlf"}

// quoting is the settled form of quote_style: what it does rather than what it
// is called.
//
// Three values and two questions, and the two are not independent. Whether the
// plain fields carry quotes is one. Whether the description may carry the
// separator is the other, and "none" has to answer no - an unquoted field
// cannot hold a separator without ending early. So this setting changes the
// CONTENT of the file and not only the punctuation around it, which is the one
// thing about it that is not obvious from its name.
type quoting struct {
	id string

	// everyField says whether the plain fields and the header names carry
	// quotes, which only "all" does. None of them ever needs quoting on its
	// own account: they are digits, a word, a word and a domain, an amount and
	// a fixed date.
	everyField bool

	// separatorsInDescription says whether the description may carry the
	// separator, which is the whole reason that column was ever quoted.
	separatorsInDescription bool
}

var quoteStyles = map[string]quoting{
	"minimal": {id: "minimal", separatorsInDescription: true},
	"all":     {id: "all", everyField: true, separatorsInDescription: true},
	"none":    {id: "none"},
}

// quoteStyleIDs is the closed set, in the vocabulary RFC 4180 readers already
// use. A fourth value outside it was considered and rejected on 2026-09-02:
// see the CSV card in docs/MVP-FORMATS.md.
var quoteStyleIDs = []string{"minimal", "all", "none"}

// wraps says whether a description of these bytes carries quotes.
//
// It reads the bytes rather than working the answer out beside them. With
// "minimal" the question is whether this particular description carries the
// separator, and the alternative is arithmetic over the word count and the
// schedule that puts separators in - two things that would have to keep
// agreeing with each other for as long as this format exists.
//
// This is the only place that question is answered. The arithmetic that sizes
// the closing row asks it too, so the length and the bytes cannot disagree.
func (q quoting) wraps(description []byte, sep byte) bool {
	switch {
	case q.everyField:
		return true
	case !q.separatorsInDescription:
		return false
	default:
		return bytes.IndexByte(description, sep) >= 0
	}
}

// quoteBytes is what the quotes cost in the SHORTEST row, which is the row the
// floor is made of.
//
// That row has an empty description, and an empty field carries no separator,
// so minimal leaves it bare and pays nothing. Only "all" pays, and it pays for
// every column.
func (q quoting) quoteBytes() int {
	if !q.everyField {
		return 0
	}
	return 2 * len(columnNames)
}

// mark writes the quote that wraps a plain field, which only "all" has.
func (q quoting) mark(dst []byte) []byte {
	if !q.everyField {
		return dst
	}
	return append(dst, quoteMark)
}

// dialect is the settled form of the four settings: the names for the
// manifest, and the bytes for the writer.
type dialect struct {
	delimiterID string
	sep         byte

	lineEndingID string
	eol          string

	header bool

	quotes quoting
}

func defaultDialect() dialect {
	return dialect{
		delimiterID:  "comma",
		sep:          ',',
		lineEndingID: "lf",
		eol:          "\n",
		header:       true,
		quotes:       quoteStyles["minimal"],
	}
}

// parseDialect reads the three settings.
//
// A value that is not in the declared set has already been refused by the
// registry, which checks it against the declaration for every format at once.
//
// Until 2026-09-02 that check compared without regard for case, so a REALISTIC
// style spelling walked past it and was refused here instead, in different
// words - O168. The registry compares exactly now, so nothing arriving through
// a recipe or a flag can reach these branches. They stay because this function
// is callable directly and a guard calling it is such a caller, and because a
// generator that trusts its input is one registry change away from writing a
// file nobody ordered.
func parseDialect(props map[string]string) (dialect, error) {
	d := defaultDialect()

	if v, ok := props[Delimiter]; ok && v != "" {
		sep, known := delimiters[v]
		if !known {
			return dialect{}, badValue(Delimiter, v,
				"it has to be "+strings.Join(delimiterIDs, ", "))
		}
		d.delimiterID, d.sep = v, sep
	}

	if v, ok := props[LineEnding]; ok && v != "" {
		eol, known := lineEndings[v]
		if !known {
			return dialect{}, badValue(LineEnding, v, "it has to be lf or crlf")
		}
		d.lineEndingID, d.eol = v, eol
	}

	if v, ok := props[Header]; ok && v != "" {
		switch v {
		case "true":
			d.header = true
		case "false":
			d.header = false
		default:
			return dialect{}, badValue(Header, v, "it has to be true or false")
		}
	}

	if v, ok := props[QuoteStyle]; ok && v != "" {
		q, known := quoteStyles[v]
		if !known {
			return dialect{}, badValue(QuoteStyle, v,
				"it has to be "+strings.Join(quoteStyleIDs, ", "))
		}
		d.quotes = q
	}

	return d, nil
}

func badValue(key, val, why string) error {
	return &format.PropertyValueError{Format: "csv", Key: key, Value: val, Reason: why}
}

// columnNames are the columns this table has always had. The last one is the
// description, which is the field the closing row stretches to reach an exact
// length, so it stays last whatever else changes.
var columnNames = []string{"id", "name", "email", "amount", "created", "description"}

// headerLine is the first row, built from the dialect rather than written out,
// so the separator in it cannot disagree with the separator in the rows below.
// That disagreement is the whole defect this format would have: a file whose
// header says one thing and whose rows do another still has the right size and
// still ends every line properly.
// It follows quote_style too. "all" means every field in the file, and the
// header names are fields - a writer set to quote everything quotes them as
// well, so a header left bare would be the one row disagreeing with the
// setting that produced it.
func (d dialect) headerLine() string {
	names := columnNames
	if d.quotes.everyField {
		names = make([]string, 0, len(columnNames))
		for _, column := range columnNames {
			names = append(names, string(quoteMark)+column+string(quoteMark))
		}
	}
	return strings.Join(names, string(d.sep)) + d.eol
}

// headerBytes is what the header costs, which is nothing when there is none.
func (d dialect) headerBytes() int64 {
	if !d.header {
		return 0
	}
	return int64(len(d.headerLine()))
}

// properties is what the registry declares. Kept beside the settings rather
// than in the descriptor, so a value added to a set above cannot be forgotten
// in the declaration below.
func properties() []format.Property {
	return []format.Property{
		{
			Name: Delimiter, Kind: format.PropertyChoice,
			Choices: delimiterIDs, Default: "comma",
			Detail: "What separates the fields. Choose semicolon for the shape a European spreadsheet exports.",
		},
		{
			Name: LineEnding, Kind: format.PropertyChoice,
			Choices: lineEndingIDs, Default: "lf",
			Detail: "How each row ends. Choose crlf for the shape RFC 4180 asks for and Excel writes.",
		},
		{
			Name: Header, Kind: format.PropertyBool,
			Default: "true",
			Detail:  "Whether the first row names the columns. Turn it off for a table dumped straight out of a database.",
		},
		{
			Name: QuoteStyle, Kind: format.PropertyChoice,
			Choices: quoteStyleIDs, Default: "minimal",
			Detail: "Which fields carry quotes. With none the description stops carrying separators as well, because an unquoted field cannot hold one.",
		},
	}
}
