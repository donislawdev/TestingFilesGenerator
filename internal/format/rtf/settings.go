package rtf

import (
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// The settings of a document and what each of them takes. The names and the
// manifest keys were approved by the owner on 2026-10-15, after the five
// readers behind every sentence below were measured (docs/RTF-2026-10-15.md
// sections 0 and 6.1). They are public names under rule 10.
const (
	Characters    = "characters"
	ASCII         = "ascii"
	International = "international"

	TextEncoding = "text_encoding"
	Unicode      = "unicode"
	Both         = "both"
	Codepage     = "codepage"
	Literal      = "literal"

	Elements = "elements"
	Basic    = "basic"
	Every    = "every"

	Nesting = "nesting"
	// MaxNesting is where the readers' cost is, rather than where one of them
	// loses the content - none did. At 10 000 000 levels LibreOffice took
	// 18.5 GB and pandoc 14.5 GB, against about 2 GB each at 1 000 000, and a
	// ceiling below that would leave the threshold impossible to order
	// (docs/RTF-2026-10-15.md section 6.1 e, the owner's decision the same
	// day). The same reasoning as the ceiling of nesting in HTML.
	MaxNesting = 10_000_000

	// CodePage is the manifest key of the code page the header declares.
	CodePage = "code_page"
)

// declaredCodePage is what \ansicpg says. With codepage the fonts carry code
// pages of their own, and this is the one a reader falls back on for a font
// that names none.
const declaredCodePage = 1252

// settings is what one document was asked for.
type settings struct {
	international bool
	// encoding is how the letters outside ASCII are written - one of the four
	// values of text_encoding.
	encoding string
	every    bool
	nesting  int
}

// properties is the declaration of every setting, in the order both surfaces
// show them: what the text says, how it is written, what the document holds,
// and how deep.
func properties() []format.Property {
	return []format.Property{{
		Name: Characters, Kind: format.PropertyChoice,
		Choices: []string{ASCII, International},
		Default: ASCII,
		Detail: "Whether the document carries text outside ASCII. international adds paragraphs in Polish, Russian, Japanese and Arabic, the Arabic right to left, " +
			"and one with emoji, a letter written as two code points, the three characters RTF escapes and the ones it names, such as the dashes and the quotes. " +
			"A program without a Japanese or Arabic font shows boxes in their place.",
	}, {
		Name: TextEncoding, Kind: format.PropertyChoice,
		Choices: []string{Unicode, Both, Codepage, Literal},
		Default: Unicode,
		Detail: "How the letters outside ASCII are written, which changes nothing while characters is ascii. " +
			"unicode writes each one as a number with a question mark after it for a reader that does not know numbers. " +
			"both puts the letter in the code page of its font after the number, two bytes for Japanese, the way Word writes it. " +
			"codepage writes only the code page bytes, and the emoji still as numbers, since no code page holds them. " +
			"literal writes the question mark as a plain character. " +
			"LibreOffice, Tika and the RichEdit control of Windows read all four. " +
			"pandoc shows codepage text in the wrong letters and drops the character after every letter written literal, and pandoc 3.1 and 3.7 drop the emoji in all four. " +
			"striprtf returns an emoji as two halves, which Python refuses to write as UTF-8.",
	}, {
		Name: Elements, Kind: format.PropertyChoice,
		Choices: []string{Basic, Every},
		Default: Basic,
		Detail: "Which parts the document uses. basic is headings, paragraphs with bold and italic words, lists, tables and indented quotes. " +
			"every adds document properties, a page header and footer, a footnote, a comment, hidden text, a link, a bookmark, " +
			"a group every reader has to skip, a word no reader knows and line, tab, page and section breaks, and each part says where it is. " +
			"Tika returns the most of them, the header, the footer and the comment among them. " +
			"LibreOffice, striprtf and RichEdit return the hidden text as text, RichEdit with the address of the link, " +
			"and pandoc returns the footnote and leaves the hidden text out.",
	}, {
		Name: Nesting, Kind: format.PropertyInt,
		Min: 0, Max: MaxNesting,
		Default: "0",
		Detail: "How many groups the content sits inside, each one inside the last. 0 puts it straight in the document, and the label stays outside so the file still says what it is. " +
			"No reader measured lost the content at 10 000 000 levels, but LibreOffice took 18.5 GB of memory over them and pandoc 14.5 GB, against about 2 GB each at 1 000 000.",
	}}
}

// settingsOf reads the settings.
//
// A value outside the declaration has already been refused by the registry,
// which checks every format in one place. The check stays here for the reason
// eml and geojson give: a guard calls Plan directly, and a generator that
// trusts its input is one registry change away from writing a document nobody
// ordered.
func settingsOf(props map[string]string) (settings, error) {
	values := map[string]string{}
	for _, p := range properties() {
		v, err := valueOf(props, p)
		if err != nil {
			return settings{}, err
		}
		values[p.Name] = v
	}
	n, err := strconv.Atoi(values[Nesting])
	if err != nil {
		// Allows has already said the value is a whole number in range, so a
		// value that still does not parse is this package's mistake.
		return settings{}, core.Defect(err)
	}
	return settings{
		international: values[Characters] == International,
		encoding:      values[TextEncoding],
		every:         values[Elements] == Every,
		nesting:       n,
	}, nil
}

// valueOf is one setting's value, or its default, refused in the declaration's
// own words when the declaration does not allow it.
func valueOf(props map[string]string, p format.Property) (string, error) {
	v, ok := props[p.Name]
	if !ok || v == "" {
		return p.Default, nil
	}
	if bad := p.Allows(v); !bad.IsZero() {
		return "", &format.PropertyValueError{Format: "rtf", Key: p.Name, Value: v, Reason: bad, Remedy: p.Instead()}
	}
	return v, nil
}

// manifest is what the manifest records about the settings, every one of them
// whatever was asked, so a reader never has to know a default.
func (s settings) manifest() map[string]any {
	chars, elements := ASCII, Basic
	if s.international {
		chars = International
	}
	if s.every {
		elements = Every
	}
	return map[string]any{
		Characters:   chars,
		TextEncoding: s.encoding,
		Elements:     elements,
		Nesting:      s.nesting,
		CodePage:     declaredCodePage,
	}
}

// adds says the settings put something into the document beyond the header,
// which is what the refusal of a size too small has to name.
func (s settings) adds() bool {
	return s.international || s.every || s.nesting > 0
}
