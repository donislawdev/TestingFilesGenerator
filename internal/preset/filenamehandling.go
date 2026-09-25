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
	{id: "polish_diacritics", group: "scripts", stem: "zażółć gęślą jaźń",
		purpose: "A Polish name with diacritics, each accented letter two bytes in UTF-8. A system that stores the name in one encoding and shows it in another turns it into mojibake."},
	{id: "cyrillic", group: "scripts", stem: "Отчёт за квартал",
		purpose: "A name in Cyrillic, so none of it is ASCII. It shows whether the name survives storage, display and download without turning into question marks."},
	{id: "cjk", group: "scripts", stem: "報告書",
		purpose: "A name in Japanese, three bytes a character in UTF-8. It checks that the name is kept whole and shown in its own script."},
	// Arabic letters with digits, written as escapes so that the line reads
	// the same in any editor: an Arabic word, 2024 and v2.
	{id: "rtl_with_digits", group: "scripts", stem: "\u062A\u0642\u0631\u064A\u0631 2024 v2",
		purpose: "An Arabic word followed by a year and a version, written right to left. A list or a log that ignores the direction of the text shows the parts in the wrong order."},
	{id: "emoji", group: "scripts", stem: "🎉",
		purpose: "A name that is one emoji - four bytes in UTF-8 and two units in UTF-16. Storage that allows three bytes a character, such as the old utf8 of MySQL, refuses it or cuts it."},
	{id: "emoji_zwj", group: "scripts", stem: "👨\u200D👩\u200D👧 family",
		purpose: "A family emoji made of three emoji joined by zero width joiners, then a word. Counting characters, cutting or reversing the name breaks the family apart."},
	{id: "nfd", group: "scripts", stem: "café", decomposed: true,
		purpose: "The word cafe with its accent written as a separate character after the e. It looks like the usual spelling, and a system comparing bytes treats the two as different files."},
	{id: "hangul_nfd", group: "scripts", stem: "한국어 보고서", decomposed: true,
		purpose: "A Korean name written as separate letters rather than syllables, the way macOS hands names over. It checks that the name is shown as syllables and matched to its usual spelling."},
	{id: "case_mapping", group: "scripts", stem: "İstanbul Straße",
		purpose: "A dotted capital I and a sharp s, whose upper and lower case forms change length or letter. A system that changes case before it compares or stores names gets them wrong."},
	{id: "fullwidth", group: "scripts", stem: "ｒｅｐｏｒｔ",
		purpose: "The word report in full width letters, which Unicode normalisation turns into ordinary ones. It shows whether a system normalises names and then finds two files under one name."},
	{id: "combining_stack", group: "scripts", stem: "z\u0335\u0321a\u0337l\u0338g\u0336o",
		purpose: "Letters with several combining marks stacked on each one. It checks that a list or a label keeps the name inside its line and never cuts it in the middle of a letter."},

	// What is shown is not what is stored.
	{id: "bidi_override", group: "lookalike", stem: "photo\u202Egpj",
		purpose: "A right to left override inside the name, so the file ends in .txt and is shown as if it ended in .jpg. It checks that the system shows the real extension and does not trust what is displayed."},
	{id: "zero_width", group: "lookalike", stem: "in\u200Bvoice",
		purpose: "The word invoice with a zero width space inside it. It looks exactly like invoice, so a system that shows names without marking such characters lets two different names look the same."},
	{id: "no_break_space", group: "lookalike", stem: "annual\u00A0report",
		purpose: "A no-break space between the two words instead of an ordinary one. It looks the same, and a search, a comparison or a trim that knows only the ordinary space misses it."},
	{id: "homoglyph", group: "lookalike", stem: "p\u0430ypal",
		purpose: "The word paypal with a Cyrillic a in place of the Latin one. It looks identical, and it checks whether the system warns about names that mix scripts."},
	{id: "leading_bom", group: "lookalike", stem: "\uFEFFreport",
		purpose: "A byte order mark at the start of the name, invisible on screen. A system that strips it from file contents and not from names ends up with a name nobody can type."},
	{id: "line_separator", group: "lookalike", stem: "report\u2028ERROR admin logged in",
		purpose: "A Unicode line separator in the name, followed by words that read like a log entry. A log that writes names as they are shows a line nobody logged."},
	{id: "unicode_tags", group: "lookalike", stem: "report" + tagged("hidden note"),
		purpose: "The word report followed by invisible Unicode tag characters that spell a short note. The name looks like report, and anything reading the raw name - a script, a language model - sees text a person cannot."},

	// Where a name is split, trimmed or hidden.
	{id: "leading_space", group: "spaces-and-dots", stem: " leading space",
		purpose: "A name that begins with a space. Many systems trim it on the way in or on the way out, and the file can then no longer be found under the name it was given."},
	{id: "leading_ideographic_space", group: "spaces-and-dots", stem: "\u3000report",
		purpose: "A name that begins with an ideographic space, the wide space of CJK scripts. It checks whether the system keeps it, trims it or shows it, and whether that is what you want."},
	{id: "double_space", group: "spaces-and-dots", stem: "two  spaces",
		purpose: "Two spaces between the words. A web page shows them as one, so the name on the page is not the name on the disk."},
	{id: "leading_dot", group: "spaces-and-dots", stem: ".hidden",
		purpose: "A name that begins with a dot, which Linux and macOS hide from ordinary listings. It checks that the file stays visible and manageable in your system."},
	{id: "leading_double_dot", group: "spaces-and-dots", stem: "..report", reason: "filename_traversal",
		purpose: "A name that begins with two dots. Code that looks for two dots to stop path traversal may refuse it, and code that strips dots may turn it into another name."},
	{id: "only_extension", group: "spaces-and-dots",
		purpose: "A name that is only the extension, with nothing before the dot. Linux and macOS hide it, and a system that splits the name at the dot is left with an empty name."},
	{id: "many_dots", group: "spaces-and-dots", stem: "v1.2.3.final", accepted: true,
		purpose: "A name with several dots, as in a version number. Only the part after the last dot is the extension, and a system that splits at the first dot gets the type wrong. Your system should take it."},
	{id: "no_extension", group: "spaces-and-dots", stem: "README", extension: noExtension,
		purpose: "A file called README with no extension at all. A system that requires an extension, or guesses the type from it, has nothing to go on."},
	{id: "upper_extension", group: "spaces-and-dots", stem: "REPORT", extension: upperExtension, accepted: true,
		purpose: "The extension in capitals. A system that compares extensions with case turns away a file it takes in lower case. Your system should take it."},

	// A name that means something to a shell, a query or an address.
	{id: "leading_dash", group: "metacharacters", stem: "-rf",
		purpose: "A name that begins with a dash, -rf. Passed to a command line without a -- before it, it is read as an option."},
	{id: "shell_substitution", group: "metacharacters", stem: "$(id) `id`",
		purpose: "A name holding $(id) and the same command in backticks, which a shell runs. It checks that no script or command line ever puts the name into a shell unquoted."},
	{id: "shell_separators", group: "metacharacters", stem: "a;b&c",
		purpose: "A name with a semicolon and an ampersand, which end one shell command and start another. It checks that the name never reaches a shell as text."},
	{id: "sql_quote", group: "metacharacters", stem: "'; DROP TABLE files; --",
		purpose: "A name that closes an SQL string and drops a table. It checks that names go into the database as parameters and are never pasted into a query."},
	{id: "script_quote", group: "metacharacters", stem: "'-alert(1)-'",
		purpose: "A name that closes a quoted string and calls alert. It checks that a page showing the name escapes it for JavaScript and for HTML."},
	{id: "url_encoded_traversal", group: "metacharacters", stem: "..%2F..%2Fetc%2Fpasswd", reason: "filename_traversal",
		purpose: "The path ../../etc/passwd with every slash written as %2F. A system that decodes the name after checking it for traversal lets it climb out of the upload directory."},
	{id: "fullwidth_traversal", group: "metacharacters", stem: "．．／．．／ｅｔｃ／ｐａｓｓｗｄ", reason: "filename_traversal",
		purpose: "The path ../../etc/passwd in full width dots and slashes, which Unicode normalisation turns into ordinary ones. A system that normalises after checking for traversal lets it climb out of the upload directory."},
	{id: "encoded_control", group: "metacharacters", stem: "report%00%0D%0A",
		purpose: "A name holding %00, %0D and %0A. Decoded, they are a null byte and a line break - a name cut short, or a header or a log line nobody wrote."},
	{id: "fullwidth_extension", group: "metacharacters", stem: "report", extension: fullwidthExtension,
		purpose: "The extension in full width letters, so it looks like the real one and is not. A system that normalises before it checks the type sees one extension, and one that does not sees another."},
	{id: "url_specials", group: "metacharacters", stem: "100% a+b #1",
		purpose: "A name with a percent sign, a plus and a hash. Each means something else in an address, so a download link built without encoding points at another file or cuts the name at the hash."},
	{id: "formula", group: "metacharacters", stem: "=1+1",
		purpose: "A name that begins with an equals sign, which a spreadsheet reads as a formula. It checks that a list of names exported to CSV or Excel does not run it."},

	// Names that mean something to a server or a desktop, whatever the file
	// holds - .htaccess with a PDF inside is still .htaccess.
	{id: "htaccess", group: "special-names", stem: ".htaccess", extension: noExtension,
		purpose: "A file called .htaccess. On an Apache server that serves the upload directory it changes how the server behaves, whatever the file holds."},
	{id: "web_config", group: "special-names", stem: "web.config", extension: noExtension,
		purpose: "A file called web.config. On an IIS server that serves the upload directory it changes how the server behaves, whatever the file holds."},
	{id: "dotenv", group: "special-names", stem: ".env", extension: noExtension,
		purpose: "A file called .env, the name applications read their settings and secrets from. It checks that an upload cannot land where it would be read as configuration."},
	{id: "ds_store", group: "special-names", stem: ".DS_Store", extension: noExtension,
		purpose: "A file called .DS_Store, which macOS writes into folders. Systems often hide or skip it, and it checks that the file is treated the way you mean."},
	{id: "desktop_ini", group: "special-names", stem: "desktop.ini", extension: noExtension,
		purpose: "A file called desktop.ini, which Windows Explorer reads to decide how a folder looks. It checks that an upload cannot change how a shared folder is shown."},
	{id: "office_lock", group: "special-names", stem: "~$report",
		purpose: "A name that begins with a tilde and a dollar sign, the way Microsoft Office names its lock files. Sync tools and backups often skip such files, so it may quietly not arrive."},

	// A name somebody reads as a value.
	{id: "null_word", group: "values", stem: "null", accepted: true,
		purpose: "A file called null. A system that turns the name into a value somewhere - JSON, a database, a template - may store no name at all. Your system should take it."},
	{id: "leading_zeros", group: "values", stem: "007", accepted: true,
		purpose: "A name made of digits with leading zeros. A spreadsheet, a JSON reader or a database column that reads it as a number drops the zeros. Your system should take it."},

	// Characters, bytes and UTF-16 units are three different numbers. Each is
	// made to a length in bytes together with the extension, so the set means
	// the same thing whatever --format says: 101 is one past what an ustar
	// archive keeps, 255 is the most a file system stores.
	{id: "ustar_101", group: "length", fill: "u", bytes: 101, reason: "filename_too_long",
		purpose: "A name of 101 bytes with its extension, one more than the name field of a ustar tar archive holds. Packing uploads into tar cuts it or moves it into an extra header."},
	{id: "max_ascii", group: "length", fill: "a", bytes: 255, reason: "filename_too_long",
		purpose: "A name of 255 bytes in ASCII, the longest most file systems store. A system that adds a prefix or a suffix on the way in pushes it over the limit."},
	{id: "cjk_bytes", group: "length", fill: "日", bytes: 255, reason: "filename_too_long",
		purpose: "A name of up to 255 bytes made of CJK characters, three bytes each. A limit counted in characters lets it through where the storage counts bytes."},
	{id: "emoji_bytes", group: "length", fill: "🎉", bytes: 255, reason: "filename_too_long",
		purpose: "A name of up to 255 bytes made of emoji, four bytes and two UTF-16 units each. Characters, bytes and units give three different lengths, and a limit checked with the wrong one lets it through or cuts it."},
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
	// purpose is what the instructions beside the manifest say about the
	// file - what the name is and what it catches.
	purpose string
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
	suffix := ext
	switch c.extension {
	case formatExtension:
	case noExtension:
		suffix = ""
	case upperExtension:
		suffix = strings.ToUpper(ext)
	case fullwidthExtension:
		suffix = fullwidth(ext)
	}
	return stem + suffix, nil
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
			expected: expected, reason: reason, purpose: c.purpose,
		})
	}
	// Every file asks the format the same question - one format, one size, the
	// label on - so asking it once answers for all fifty (PR7).
	if err := files[0].refused(); err != nil {
		return nil, err
	}
	return plan{preset: namesID, question: namesQuestion, targets: draftsOf(files)}.source()
}
