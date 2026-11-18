package format

import (
	"fmt"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// What a setting takes, said to a person looking at an empty field: the start
// of the explanation under it, which tfg formats prints and the window builds
// in its own words from the same parts. Beside format.go, which holds the
// refusal a value meets (Allows), rather than in it: the two say the same
// words, a guard reads them side by side, and format.go reached the ceiling
// on the length of a file when the values in this one were marked (O358).

// sizePhrase is what a size setting takes under an empty field, its example
// marked as a value to type (core.Literal).
//
// Allows says the same words, and until 2026-08-26 the two said it
// differently - "a size written the way any size is, such as 2mb or a plain
// byte count" against "a size such as 2mb, or a plain byte count". The other
// four kinds agree word for word, so size was the one place where the sentence
// a person reads depended on whether they had already typed something. O64,
// held by TestASettingIsDescribedTheSameWayEmptyOrRefused, which reads the two
// without the marks: a refusal is a sentence of the engine and carries none.
//
// The shorter half won: "written the way any size is" is a sentence about
// sizes rather than about this setting, and somebody who has just been refused
// needs the shape, not the provenance.
const sizePhrase = "a size such as `2mb`, or a plain byte count"

func (p Property) unitSuffix() string {
	if p.Unit == "" {
		return ""
	}
	return " of " + p.Unit
}

// Allowed says what this property accepts, as one phrase for a person.
//
// The values in it a person types as they are - each choice, the default, the
// examples - stand between backticks (core.Literal), which the window draws in
// the type of code, the site as code and the command line as written. The
// owner's decision of 2026-11-18 (O358, docs/HINT-VALUES-2026-11-18.md): the
// sentences under it marked them already, and the start of the same
// explanation did not.
//
// It lives here rather than beside a consumer because there are two of them and
// they cannot import each other. "tfg formats png" prints it in a list, and the
// window shows it under the field it drew from this same declaration - so a
// second copy would be two surfaces coming to describe one format differently,
// which is D1 in the place nobody thinks to compare.
//
// Near neighbour of Allows above, and deliberately not folded into it. Allows
// answers somebody who has already typed a wrong value and names the flag that
// would have taken a right one. This answers somebody looking at an empty field.
// The two phrasings agree today for every kind but size, and unifying them would
// change a message the command line already prints - a decision for the owner
// rather than a tidy up. Recorded as O64.
func (p Property) Allowed() string {
	var what string
	switch p.Kind {
	case PropertyInt:
		if p.Min != 0 || p.Max != 0 {
			what = fmt.Sprintf("whole number%s from %d to %d", p.unitSuffix(), p.Min, p.Max)
		} else {
			what = "whole number" + p.unitSuffix()
		}
	case PropertyChoice:
		what = "one of: " + strings.Join(core.LiteralEach(p.Choices), ", ")
	case PropertyBool:
		what = core.Literal("true") + " or " + core.Literal("false")
	case PropertySize:
		what = sizePhrase
	case PropertyDuration:
		what = p.durationPhrase()
	default:
		// A text setting describes itself with Shape or not at all. Saying
		// "text" under a field is a word where a description should be.
		what = p.Shape
	}
	if p.Default != "" && what == "" {
		return "default " + core.Literal(p.Default)
	}
	if p.Default != "" {
		what += ", default " + core.Literal(p.Default)
	}
	return what
}

// durationPhrase is what a length of time takes under an empty field: the
// refusal's own words without its first two (takesDuration), so an empty field
// and a refused one describe the setting identically, and the examples marked
// as values to type.
func (p Property) durationPhrase() string {
	examples := core.Literal("10s") + ", " + core.Literal("1m30s") + " or " + core.Literal("500ms")
	if p.Min == 0 && p.Max == 0 {
		return "a length of time such as " + examples
	}
	return "a length of time from " + core.FormatDuration(p.Min) + " to " + core.FormatDuration(p.Max) + ", such as " + examples
}
