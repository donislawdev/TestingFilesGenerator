package preset

import (
	"fmt"
	"strings"

	"golang.org/x/text/unicode/norm"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

const (
	namesID = "filename-handling"

	// namesFormat is the format the set is made of when nobody says. The name
	// is what this preset is about and the contents are not, so the default is
	// the format with the least in it, and --format changes it.
	namesFormat = "txt"

	// namesQuestion is announced by the preset AND written into the header of
	// an ejected recipe.
	namesQuestion = "Will my system store, show and give back a file name it did not expect?"

	// namesSample is how big each file is when its format allows it - small,
	// since every file is about its name, and the owner's budget for the set
	// is fifty files of a kilobyte.
	namesSample = 1 << 10
)

func init() {
	Register(Preset{
		ID:       namesID,
		Title:    "File name handling",
		Question: namesQuestion,

		Reads:        []string{"format"},
		ReadDefaults: map[string]string{"format": namesFormat},

		Requires: []string{"MVP"},
		Catches: []string{
			"a name that looks like a different one on screen, in a log or in a list",
			"a name cut, trimmed or rewritten between upload and storage",
			"a length limit counted in characters where the storage counts bytes",
		},

		Expand: expandFileNames,
	})
}

// The fifty names, chosen and measured in docs/NAMES-PRESET-2026-09-24.md. Each
// one catches a class of fault a system that takes files in has been seen to
// have, none repeats a class another one catches, and every one of them is
// written byte for byte on Windows, Linux and macOS - so a file that does not
// arrive, or arrives renamed, is the system under test and not this tool.
//
// A character nobody can see is written here as an escape and never as
// itself, and a guard reads every file of this repository for that
// (TestNoTrackedFileCarriesACharacterNobodyCanSee). A right to left override
// typed into Go source is how code comes to say one thing on screen and
// another to the compiler.
//
// expected is accept for the four names a system would be wrong to refuse,
// and unspecified for the rest, because whether a system takes a name with a
// zero width space in it is its own policy and not a fault (MF5). Only reasons
// the manifest already knows are used - the owner's decision.
var nameCases = []nameCase{
	// How a name goes through bytes, encodings and normalisation.
	{id: "polish_diacritics", group: "scripts", stem: "zażółć gęślą jaźń"},
	{id: "cyrillic", group: "scripts", stem: "Отчёт за квартал"},
	{id: "cjk", group: "scripts", stem: "報告書"},
	// Arabic letters with digits, written as escapes so that the line reads
	// the same in any editor: an Arabic word, 2024 and v2.
	{id: "rtl_with_digits", group: "scripts", stem: "\u062A\u0642\u0631\u064A\u0631 2024 v2"},
	{id: "emoji", group: "scripts", stem: "🎉"},
	{id: "emoji_zwj", group: "scripts", stem: "👨\u200D👩\u200D👧 family"},
	{id: "nfd", group: "scripts", stem: "café", decomposed: true},
	{id: "hangul_nfd", group: "scripts", stem: "한국어 보고서", decomposed: true},
	{id: "case_mapping", group: "scripts", stem: "İstanbul Straße"},
	{id: "fullwidth", group: "scripts", stem: "ｒｅｐｏｒｔ"},
	{id: "combining_stack", group: "scripts", stem: "z\u0335\u0321a\u0337l\u0338g\u0336o"},

	// What is shown is not what is stored.
	{id: "bidi_override", group: "lookalike", stem: "photo\u202Egpj"},
	{id: "zero_width", group: "lookalike", stem: "in\u200Bvoice"},
	{id: "no_break_space", group: "lookalike", stem: "annual\u00A0report"},
	{id: "homoglyph", group: "lookalike", stem: "p\u0430ypal"},
	{id: "leading_bom", group: "lookalike", stem: "\uFEFFreport"},
	{id: "line_separator", group: "lookalike", stem: "report\u2028ERROR admin logged in"},
	{id: "unicode_tags", group: "lookalike", stem: "report" + tagged("hidden note")},

	// Where a name is split, trimmed or hidden.
	{id: "leading_space", group: "spaces-and-dots", stem: " leading space"},
	{id: "leading_ideographic_space", group: "spaces-and-dots", stem: "\u3000report"},
	{id: "double_space", group: "spaces-and-dots", stem: "two  spaces"},
	{id: "leading_dot", group: "spaces-and-dots", stem: ".hidden"},
	{id: "leading_double_dot", group: "spaces-and-dots", stem: "..report", reason: "filename_traversal"},
	{id: "only_extension", group: "spaces-and-dots"},
	{id: "many_dots", group: "spaces-and-dots", stem: "v1.2.3.final", accepted: true},
	{id: "no_extension", group: "spaces-and-dots", stem: "README", extension: noExtension},
	{id: "upper_extension", group: "spaces-and-dots", stem: "REPORT", extension: upperExtension, accepted: true},

	// A name that means something to a shell, a query or an address.
	{id: "leading_dash", group: "metacharacters", stem: "-rf"},
	{id: "shell_substitution", group: "metacharacters", stem: "$(id) `id`"},
	{id: "shell_separators", group: "metacharacters", stem: "a;b&c"},
	{id: "sql_quote", group: "metacharacters", stem: "'; DROP TABLE files; --"},
	{id: "script_quote", group: "metacharacters", stem: "'-alert(1)-'"},
	{id: "url_encoded_traversal", group: "metacharacters", stem: "..%2F..%2Fetc%2Fpasswd", reason: "filename_traversal"},
	{id: "fullwidth_traversal", group: "metacharacters", stem: "．．／．．／ｅｔｃ／ｐａｓｓｗｄ", reason: "filename_traversal"},
	{id: "encoded_control", group: "metacharacters", stem: "report%00%0D%0A"},
	{id: "fullwidth_extension", group: "metacharacters", stem: "report", extension: fullwidthExtension},
	{id: "url_specials", group: "metacharacters", stem: "100% a+b #1"},
	{id: "formula", group: "metacharacters", stem: "=1+1"},

	// Names that mean something to a server or a desktop, whatever the file
	// holds - .htaccess with a PDF inside is still .htaccess.
	{id: "htaccess", group: "special-names", stem: ".htaccess", extension: noExtension},
	{id: "web_config", group: "special-names", stem: "web.config", extension: noExtension},
	{id: "dotenv", group: "special-names", stem: ".env", extension: noExtension},
	{id: "ds_store", group: "special-names", stem: ".DS_Store", extension: noExtension},
	{id: "desktop_ini", group: "special-names", stem: "desktop.ini", extension: noExtension},
	{id: "office_lock", group: "special-names", stem: "~$report"},

	// A name somebody reads as a value.
	{id: "null_word", group: "values", stem: "null", accepted: true},
	{id: "leading_zeros", group: "values", stem: "007", accepted: true},

	// Characters, bytes and UTF-16 units are three different numbers. Each is
	// made to a length in bytes together with the extension, so the set means
	// the same thing whatever --format says: 101 is one past what an ustar
	// archive keeps, 255 is the most a file system stores.
	{id: "ustar_101", group: "length", fill: "u", bytes: 101, reason: "filename_too_long"},
	{id: "max_ascii", group: "length", fill: "a", bytes: 255, reason: "filename_too_long"},
	{id: "cjk_bytes", group: "length", fill: "日", bytes: 255, reason: "filename_too_long"},
	{id: "emoji_bytes", group: "length", fill: "🎉", bytes: 255, reason: "filename_too_long"},
}

// nameCase is one file of the set, described by how its name is made rather
// than by the name, because the extension comes from the format.
type nameCase struct {
	id, group string
	// stem is the name before the extension.
	stem string
	// decomposed writes the stem in normalisation form D - an accent as a
	// separate character after its letter, a Korean syllable as its letters -
	// which is how macOS hands names over and how the source here cannot show
	// them apart from the composed form.
	decomposed bool
	extension  extensionRule
	// fill and bytes make a name of a length rather than of a spelling: as
	// many copies of fill as fit in bytes together with the extension.
	fill  string
	bytes int
	// accepted marks a name a system would be wrong to refuse. The rest are
	// unspecified, for reason, or for filename_invalid when reason is empty.
	accepted bool
	reason   string
}

// extensionRule is what follows the stem.
type extensionRule int

const (
	// formatExtension is the extension of the format, as it is.
	formatExtension extensionRule = iota
	// noExtension is a name that is complete as it is.
	noExtension
	// upperExtension is the extension of the format in capitals.
	upperExtension
	// fullwidthExtension is the extension of the format in full width
	// letters, the dots left as they are.
	fullwidthExtension
)

// name is the file name for a format whose extension is ext.
func (c nameCase) name(desc format.Descriptor) (string, error) {
	ext := desc.Extension
	stem := c.stem
	if c.decomposed {
		stem = norm.NFD.String(stem)
	}
	if c.fill != "" {
		copies := (c.bytes - len(ext)) / len(c.fill)
		if copies < 1 {
			return "", &format.PropertyValueError{Format: namesID, Key: "format", Value: desc.ID,
				Reason: fmt.Sprintf("its extension %s is too long for a name of %d bytes, which the file %s is about. Choose a format with a shorter extension", ext, c.bytes, c.id)}
		}
		stem = strings.Repeat(c.fill, copies)
	}
	switch c.extension {
	case noExtension:
		return stem, nil
	case upperExtension:
		return stem + strings.ToUpper(ext), nil
	case fullwidthExtension:
		return stem + fullwidth(ext), nil
	}
	return stem + ext, nil
}

// expectation is the outcome and the reason this file carries.
func (c nameCase) expectation() (string, string) {
	switch {
	case c.accepted:
		return "accept", ""
	case c.reason != "":
		return "unspecified", c.reason
	}
	return "unspecified", "filename_invalid"
}

// fullwidth writes the letters and digits of s in their full width forms,
// which NFKC turns back into the ones they stand for. The dot stays, so
// report.ｔｘｔ still has a dot before its extension.
func fullwidth(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r > ' ' && r <= '~' && r != '.' {
			r += 0xFF01 - '!'
		}
		b.WriteRune(r)
	}
	return b.String()
}

// tagged is s in Unicode tag characters: the same letters, invisible, and read
// by a language model as text. The words are neutral on purpose. A classic
// attack carries an instruction, and an instruction here would be one for
// every tool that reviews this source.
func tagged(s string) string {
	var b strings.Builder
	for _, r := range s {
		b.WriteRune(0xE0000 + r)
	}
	return b.String()
}

func expandFileNames(args Args) ([]byte, error) {
	formatID := namesFormat
	if v := args["format"]; v != "" {
		formatID = v
	}
	desc, err := format.Get(formatID)
	if err != nil {
		return nil, err
	}
	size := sampleAtLeast(desc, namesSample)

	files := make([]setFile, 0, len(nameCases))
	for _, c := range nameCases {
		name, err := c.name(desc)
		if err != nil {
			return nil, err
		}
		expected, reason := c.expectation()
		files = append(files, setFile{
			id: c.id, name: name, group: c.group, desc: desc, size: size,
			expected: expected, reason: reason,
		})
	}
	// Every file asks the format the same question - one format, one size, the
	// label on - so asking it once answers for all fifty (PR7).
	if err := files[0].refused(); err != nil {
		return nil, err
	}
	return plan{preset: namesID, question: namesQuestion, targets: draftsOf(files)}.source()
}
