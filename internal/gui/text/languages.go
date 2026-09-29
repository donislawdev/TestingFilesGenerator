package text

import (
	"encoding/json"
	"io/fs"
	"path"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/language"
)

// Language is one language the window can speak: the tag its catalogue file is
// named by, and its name written in itself - "Polski" rather than "Polish" -
// because that is the name a person looks for in a list they may not be able to
// read yet.
type Language struct {
	Tag  string
	Name string
}

// English is the language every build carries. The English is written in the
// code beside each entry, so it answers even with no catalogue at all.
const English = "en"

// languageNameID is the catalogue entry that names the language a file is
// written in - see LanguageName in screens.go.
const languageNameID = "LanguageName"

// Languages is every language compiled into this build, English first and the
// rest by their tags.
//
// Read from the files rather than listed here, so a language arrives by adding
// its file and nothing else. A list typed beside the files is a list that one
// day names a language that is not there, or leaves out one that is.
func Languages() []Language {
	return languagesIn(builtIn, "locale")
}

// languagesIn is Languages over any catalogue, so a guard can hand it one.
//
// A file whose name entry is missing or unreadable is listed by its tag rather
// than left out: a language somebody sees under an odd name is a defect
// somebody reports, and one that is silently missing is not. A guard holds
// every file to carrying the entry, so this is the fallback and not the plan.
func languagesIn(fsys fs.FS, dir string) []Language {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return []Language{{Tag: English, Name: LanguageName()}}
	}
	var out []Language
	for _, entry := range entries {
		if entry.IsDir() || path.Ext(entry.Name()) != ".json" {
			continue
		}
		tag := strings.TrimSuffix(entry.Name(), ".json")
		out = append(out, Language{Tag: tag, Name: nameIn(fsys, path.Join(dir, entry.Name()), tag)})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if (out[i].Tag == English) != (out[j].Tag == English) {
			return out[i].Tag == English
		}
		return out[i].Tag < out[j].Tag
	})
	return out
}

// nameIn is what one catalogue file calls its own language, or the tag.
func nameIn(fsys fs.FS, file, tag string) string {
	raw, err := fs.ReadFile(fsys, file)
	if err != nil {
		return tag
	}
	var entries map[string]json.RawMessage
	if json.Unmarshal(raw, &entries) != nil {
		return tag
	}
	var entry struct {
		Other string `json:"other"`
	}
	if json.Unmarshal(entries[languageNameID], &entry) == nil && entry.Other != "" {
		return entry.Other
	}
	return tag
}

// Choice is the language the window speaks and how it came to it.
type Choice struct {
	// Tag is one of the carried languages, English when nothing else fits.
	Tag string
	// Missing is a saved choice this build does not carry, and empty when
	// there was none or it was found. The window says so rather than quietly
	// speaking something else - untouchable rule 6.
	Missing string
}

// Resolve decides which language the window speaks.
//
// A language somebody chose comes first. Nothing chosen - the saved value is
// empty - means the language of the system, which the owner decided on
// 2026-09-29 (docs/PRODUCT.md D9). English when the system's language is not
// carried, or cannot be read at all.
//
// A choice this build does not carry is not an error: a later version may
// have dropped a language, or the file may have been edited by hand. It falls
// through to the system as if nothing were chosen, and says what was missing.
//
// Matched rather than compared, because the system says pl-PL where the
// catalogue says pl - see closest.
func Resolve(saved, system string, carried []Language) Choice {
	if saved != "" {
		if tag, ok := closest(saved, carried); ok {
			return Choice{Tag: tag}
		}
		choice := fromSystem(system, carried)
		choice.Missing = saved
		return choice
	}
	return fromSystem(system, carried)
}

func fromSystem(system string, carried []Language) Choice {
	if tag, ok := closest(system, carried); ok {
		return Choice{Tag: tag}
	}
	return Choice{Tag: English}
}

// closest is the carried language nearest to what was asked for, when it is
// near enough.
//
// High confidence and above, and the line is drawn there on purpose: pl-PL
// finds pl and en-GB finds en, while zh-TW does not find a catalogue written
// in simplified Chinese - a person who reads one script is not served by the
// other, and English is the better answer for them.
func closest(asked string, carried []Language) (string, bool) {
	want, err := language.Parse(asked)
	if err != nil || len(carried) == 0 {
		return "", false
	}
	tags := make([]language.Tag, len(carried))
	for i, l := range carried {
		tags[i] = language.Make(l.Tag)
	}
	_, index, confidence := language.NewMatcher(tags).Match(want)
	if confidence < language.High {
		return "", false
	}
	return carried[index].Tag, true
}

// Speaking is the language this window was opened in.
//
// Set by Load, once, before any screen is built - the same constraint as the
// localiser, and for the same reason. The Preferences screen compares it with
// the saved choice to know whether a restart would change anything.
func Speaking() string {
	if speaking == "" {
		return English
	}
	return speaking
}

var speaking string

// Pseudo is a language nobody speaks, for whoever builds the window.
//
// Every sentence comes out with its letters accented and about two fifths
// longer, between brackets - so every screen can be seen the way a translation
// will make it before any translation exists: a label that runs out of its
// column, a heading that pushes a button off the row, a sentence cut short
// (the closing bracket is missing), a word that was never asked for from the
// catalogue (it has no accents). Asked for with --pseudo-language and never
// offered in the list, and never saved.
const Pseudo = "pseudo"

// pseudo is whether this window speaks Pseudo. Written once, like speaking.
var pseudo bool

// pseudoOf is one sentence in Pseudo.
//
// The values a sentence carries - {{.Name}} - are left as they are, because
// they are filled in afterwards and a field renamed on the way is a field that
// comes out as "<no value>". An empty sentence stays empty: several entries
// are empty on purpose, and a pair of brackets where nothing should stand would
// be a defect this language made up.
func pseudoOf(sentence string) string {
	if sentence == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("[")
	letters := 0
	for i := 0; i < len(sentence); {
		if strings.HasPrefix(sentence[i:], "{{") {
			end := strings.Index(sentence[i:], "}}")
			if end < 0 {
				b.WriteString(sentence[i:])
				break
			}
			b.WriteString(sentence[i : i+end+2])
			i += end + 2
			continue
		}
		r, size := utf8.DecodeRuneInString(sentence[i:])
		if accented, ok := pseudoLetters[r]; ok {
			r = accented
		}
		if unicode.IsLetter(r) {
			letters++
		}
		b.WriteRune(r)
		i += size
	}
	b.WriteString(" ")
	b.WriteString(strings.Repeat(string(rune(0x00B7)), (letters*2+4)/5))
	b.WriteString("]")
	return b.String()
}

// pseudoLetters are the letters Pseudo accents, each one a letter of a real
// language the window may be translated into - so a glyph missing from the
// window's typeface shows up here first. Written as numbers because the tools
// that write this tree have turned escaped letters into other characters
// before (CLAUDE.md, trap 14).
var pseudoLetters = map[rune]rune{
	'a': 0x00E5, 'c': 0x00E7, 'd': 0x010F, 'e': 0x00E9, 'g': 0x011F,
	'i': 0x00EE, 'k': 0x0137, 'l': 0x0142, 'n': 0x00F1, 'o': 0x00F6,
	'r': 0x0159, 's': 0x0161, 't': 0x0165, 'u': 0x00FC, 'y': 0x00FD,
	'z': 0x017E,
	'A': 0x00C5, 'C': 0x00C7, 'D': 0x010E, 'E': 0x00C9, 'G': 0x011E,
	'I': 0x00CE, 'K': 0x0136, 'L': 0x0141, 'N': 0x00D1, 'O': 0x00D6,
	'R': 0x0158, 'S': 0x0160, 'T': 0x0164, 'U': 0x00DC, 'Y': 0x00DD,
	'Z': 0x017D,
}
