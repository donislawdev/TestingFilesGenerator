// The layout: the one thing about a JSON document that every reader forgives
// and almost nothing downstream of the reader does.
//
// Measured 2026-09-07 on two implementations in two languages - Python's json
// and V8's JSON.parse - with the same records written four ways: minified, one
// record per line, indented by two and indented by four. All four parse
// everywhere. So this setting is not about whether a parser copes. It is about
// the file: the same three records are 130 B minified and 260 B indented, and a
// minified document of any size is one single line.
//
// That is what makes it worth having. A tester whose pipeline reads line by
// line, splits a file into chunks, diffs two exports or loads one into an
// editor meets a different file in each shape, and every one of them is valid
// JSON that no parser will complain about.
package jsonfile

import "github.com/donislawdev/TestingFilesGenerator/internal/format"

// Setting name. A public name, so it is spelled once.
const Formatting = "formatting"

// The three layouts, spelled as the recipe spells them.
const (
	Indented      = "indented"
	Minified      = "minified"
	RecordPerLine = "record-per-line"
)

// style is every literal that stands between one value of a record and the
// next, plus what opens the document and what closes it.
//
// Held as strings rather than as an indent width and a rule for applying it,
// because the arithmetic that hits an exact byte count has to measure these
// rather than predict them. A width plus a rule is two things that can
// disagree - the literals are one.
type style struct {
	name     string
	prologue string

	openID   string
	openName string
	openMail string
	openAmt  string
	openAct  string
	nullPart string
	openTags string
	tagSep   string
	openAddr string
	openZip  string
	openNote string

	// tailMore closes a record that has another after it, tailLast closes the
	// record that closes the document.
	tailMore string
	tailLast string
}

// fixed is every literal byte of a closing record: everything except the
// record number, the five words and the note.
//
// Computed from the style's own literals rather than written down beside them,
// so a layout added later cannot carry a number that disagrees with its own
// text. The same reason minimumBytes is computed rather than declared.
func (s style) fixed() int {
	return len(s.openID) + len(s.openName) + len(s.openMail) + len(emailDomain) +
		len(s.openAmt) + amountWidth + len(s.openAct) + widestBool + len(s.nullPart) +
		len(s.openTags) + len(s.tagSep) + len(s.openAddr) + len(s.openZip) + zipWidth +
		len(s.openNote) + len(s.tailLast)
}

// recordPerLine is the layout this format has always written. Leaving the
// setting alone produces the same bytes it always did.
var recordPerLine = style{
	name:     RecordPerLine,
	prologue: "[\n",
	openID:   `{"id":`,
	openName: `,"name":"`,
	openMail: `","email":"`,
	openAmt:  `","amount":`,
	openAct:  `,"active":`,
	nullPart: `,"retired":null`,
	openTags: `,"tags":["`,
	tagSep:   `","`,
	openAddr: `"],"address":{"city":"`,
	openZip:  `","zip":"`,
	openNote: `"},"note":"`,
	tailMore: "\"},\n",
	tailLast: "\"}\n]\n",
}

// minified is the same tokens with no whitespace anywhere, which puts the
// whole document on one line.
//
// It ends without a trailing newline, on purpose: a minified document that
// ends in a newline is not minified, and a file with no final newline is
// itself a thing worth handing to a tester. Said out loud in the setting's
// own description rather than left to be discovered.
var minified = style{
	name:     Minified,
	prologue: "[",
	openID:   `{"id":`,
	openName: `,"name":"`,
	openMail: `","email":"`,
	openAmt:  `","amount":`,
	openAct:  `,"active":`,
	nullPart: `,"retired":null`,
	openTags: `,"tags":["`,
	tagSep:   `","`,
	openAddr: `"],"address":{"city":"`,
	openZip:  `","zip":"`,
	openNote: `"},"note":"`,
	tailMore: `"},`,
	tailLast: `"}]`,
}

// indented is what json.dumps(indent=2), JSON.stringify(x, null, 2) and every
// formatter reached for by default produce: two spaces a level, every value on
// its own line, and the nested array and object opened out as well.
//
// Two rather than four, because two is what those three produce without being
// asked. A file indented by four is a different file, and it is a setting of
// its own the day somebody needs it rather than a second value here.
var indented = style{
	name:     Indented,
	prologue: "[\n",
	openID:   "  {\n    \"id\": ",
	openName: ",\n    \"name\": \"",
	openMail: "\",\n    \"email\": \"",
	openAmt:  "\",\n    \"amount\": ",
	openAct:  ",\n    \"active\": ",
	nullPart: ",\n    \"retired\": null",
	openTags: ",\n    \"tags\": [\n      \"",
	tagSep:   "\",\n      \"",
	openAddr: "\"\n    ],\n    \"address\": {\n      \"city\": \"",
	openZip:  "\",\n      \"zip\": \"",
	openNote: "\"\n    },\n    \"note\": \"",
	tailMore: "\"\n  },\n",
	tailLast: "\"\n  }\n]\n",
}

var styles = map[string]style{
	RecordPerLine: recordPerLine,
	Minified:      minified,
	Indented:      indented,
}

func defaultStyle() style { return recordPerLine }

// parseStyle reads the one setting this format takes.
//
// A value outside the declared set has already been refused by the registry,
// which checks every format against its declaration in one place. This branch
// stays for the reason the CSV dialect keeps its own: the function is callable
// directly, a guard is such a caller, and a generator that trusts its input is
// one registry change away from writing a file nobody ordered.
func parseStyle(props map[string]string) (style, error) {
	v, ok := props[Formatting]
	if !ok || v == "" {
		return defaultStyle(), nil
	}
	s, known := styles[v]
	if !known {
		return style{}, &format.PropertyValueError{
			Format: "json", Key: Formatting, Value: v,
			Reason: "it has to be " + Indented + ", " + Minified + " or " + RecordPerLine,
		}
	}
	return s, nil
}

func properties() []format.Property {
	return []format.Property{
		{
			Name: Formatting, Kind: format.PropertyChoice,
			Choices: []string{Indented, Minified, RecordPerLine},
			Default: RecordPerLine,
			Detail:  "How the document is laid out. Every reader accepts all three, so this changes what meets everything around the parser rather than the parser itself - minified puts the whole file on one line and ends without a newline, and indented makes the same records several times larger.",
		},
	}
}
