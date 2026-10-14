package htmlfile

import (
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/textenc"
)

// The settings of a page, and what each of them takes. Split out of html.go on
// 2026-10-14, when four settings joined the one there was and the file would
// have gone past the size the shape guard allows. The measurements behind every
// number below are in docs/HTML-2-2026-10-14.md section 11.

const (
	settingStructure  = "structure"
	structureDocument = "document"
	structureFragment = "fragment"

	settingNesting = "nesting"
	// maxNesting is past every threshold measured on 2026-10-14 - lxml losing
	// the content at about 250, Chromium at 511, jsdom, parse5 and DOMPurify
	// from about 2 000 in a fresh process - and far enough past them that
	// html5lib, which has no threshold, takes minutes. A ceiling below a
	// reader's threshold would leave that threshold impossible to order.
	maxNesting = 100000

	settingCharacters       = "characters"
	charactersASCII         = "ascii"
	charactersInternational = "international"

	settingElements = "elements"
	elementsBasic   = "basic"
	elementsEvery   = "every"
)

// settings is what one page was asked for.
type settings struct {
	shape string
	// nesting is how many div elements the content sits inside.
	nesting int
	// international adds the paragraphs in other scripts, and every the
	// catalogue of every element.
	international, every bool
	// bom is a byte order mark in front of everything else.
	bom bool
}

// properties is the declaration of every setting, in the order both surfaces
// show them.
func properties() []format.Property {
	return []format.Property{{
		Name: settingStructure, Kind: format.PropertyChoice,
		Choices: []string{structureDocument, structureFragment},
		Default: structureDocument,
		Detail:  "Whether the file is a whole page or only the blocks that would sit inside one. A fragment has no doctype, no html element and no body, which is what a content field or the body of an email really holds. It is far smaller, so the smallest fragment sits well below the smallest page.",
	}, {
		Name: settingNesting, Kind: format.PropertyInt,
		Min: 0, Max: maxNesting,
		Default: "0",
		Detail: "How many div elements the content sits inside, each one inside the last. 0 puts it straight in the body, and the label stays outside so the page still says what it is. " +
			"Chromium keeps 511 levels and moves the deeper ones out, and the Nu HTML Checker calls a page that deep an error. " +
			"lxml drops the content of a page nested about 250 deep and keeps only the label, and jsdom, parse5 and DOMPurify can run out of stack writing out a page from about 2 000 deep.",
	}, {
		Name: settingCharacters, Kind: format.PropertyChoice,
		Choices: []string{charactersASCII, charactersInternational},
		Default: charactersASCII,
		Detail: "Whether the page carries text outside ASCII. international adds paragraphs in Polish, Russian, Japanese and Arabic, the Arabic right to left, " +
			"with emoji, a letter written as two code points and named and numbered entities. A program without a Japanese or Arabic font shows boxes in their place.",
	}, {
		Name: settingElements, Kind: format.PropertyChoice,
		Choices: []string{elementsBasic, elementsEvery},
		Default: elementsBasic,
		Detail: "Which elements the page uses. basic is headings, paragraphs, lists, tables and quotes. " +
			"every adds a block that uses every element of the HTML standard at least once, all 22 kinds of input and small pictures and a sound inside the page, with no script and nothing fetched from a network. " +
			"A fragment has no head, so it leaves out the eight elements that only a whole page holds.",
	}, textenc.Axes(textenc.SettingBOM)[0]}
}

// settingsOf reads the settings.
//
// A value outside the declared set has already been refused by the registry,
// which checks every format against its declaration in one place. These
// branches stay for the same reason the CSV dialect and the text encoding keep
// theirs: this function is callable directly, a guard is such a caller, and a
// generator that trusts its input is one registry change away from writing a
// file nobody ordered.
func settingsOf(props map[string]string) (settings, error) {
	s := settings{shape: structureDocument}
	if v, ok := choice(props, settingStructure, structureDocument, structureFragment); ok {
		s.shape = v
	} else {
		return settings{}, refused(props, settingStructure, core.Says("html.HasToBeStructure", "it has to be %s or %s",
			core.A("Document", core.Choice{Of: settingStructure, Value: structureDocument}),
			core.A("Fragment", core.Choice{Of: settingStructure, Value: structureFragment})))
	}
	if v, ok := choice(props, settingCharacters, charactersASCII, charactersInternational); ok {
		s.international = v == charactersInternational
	} else {
		return settings{}, refused(props, settingCharacters, core.Says("html.HasToBeCharacters", "it has to be %s or %s",
			core.A("ASCII", core.Choice{Of: settingCharacters, Value: charactersASCII}),
			core.A("International", core.Choice{Of: settingCharacters, Value: charactersInternational})))
	}
	if v, ok := choice(props, settingElements, elementsBasic, elementsEvery); ok {
		s.every = v == elementsEvery
	} else {
		return settings{}, refused(props, settingElements, core.Says("html.HasToBeElements", "it has to be %s or %s",
			core.A("Basic", core.Choice{Of: settingElements, Value: elementsBasic}),
			core.A("Every", core.Choice{Of: settingElements, Value: elementsEvery})))
	}
	var err error
	if s.nesting, err = nestingOf(props); err != nil {
		return settings{}, err
	}
	codec, err := textenc.Parse("html", props)
	if err != nil {
		return settings{}, err
	}
	s.bom = codec.HasBOM()
	return s, nil
}

// choice reads a setting of two values, the first of them the default, and
// says whether the value is one of them.
func choice(props map[string]string, key, first, second string) (string, bool) {
	v, ok := props[key]
	if !ok || v == "" {
		return first, true
	}
	return v, v == first || v == second
}

// refused is a value outside the two a setting takes. The sentence comes from
// the caller, written out there, because a translator finds a sentence by its
// id and an id passed along as a value is one nobody can find.
func refused(props map[string]string, key string, reason core.Said) error {
	return &format.PropertyValueError{Format: "html", Key: key, Value: props[key], Reason: reason}
}

// nestingOf reads how deep the content sits.
func nestingOf(props map[string]string) (int, error) {
	v, ok := props[settingNesting]
	if !ok || v == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 || n > maxNesting {
		return 0, &format.PropertyValueError{
			Format: "html", Key: settingNesting, Value: v,
			Reason: core.Says("html.HasToBeNesting", "it has to be a whole number from 0 to %d", core.A("Max", maxNesting)),
		}
	}
	return n, nil
}

// manifest is what the manifest records about the settings, every one of them
// whatever was asked, so a reader never has to know a default.
func (s settings) manifest() map[string]any {
	chars, elements := charactersASCII, elementsBasic
	if s.international {
		chars = charactersInternational
	}
	if s.every {
		elements = elementsEvery
	}
	return map[string]any{
		settingStructure:   s.shape,
		settingNesting:     s.nesting,
		settingCharacters:  chars,
		settingElements:    elements,
		textenc.SettingBOM: s.bom,
	}
}
