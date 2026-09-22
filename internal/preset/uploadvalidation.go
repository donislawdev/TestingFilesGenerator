package preset

import (
	"fmt"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

const (
	uploadID = "upload-validation"

	// limit is spelled the same as the boundary set's, and that is the point
	// rather than a collision. Two presets asking for the number a system
	// declares as its limit have to ask for it in the same word, or somebody
	// learns one and types the other. The shared namespace is what makes
	// "--preset upload-validation --limit 5mb" read as one sentence, and
	// Declaring names every owner so the refusal beside a bare --limit can too.
	uploadLimitParam = "limit"
	allowParam       = "allow"
	denyParam        = "deny"
	farOverParam     = "far-over"
	bulkParam        = "bulk"

	defaultUploadLimit = "10mb"
	defaultAllow       = "jpg,png,pdf"
	defaultDeny        = "svg,html,exe,sh"
	// The owner's decision of 2026-09-22. Ten times the limit is the case worth
	// having - a form that reads the whole body into memory before it looks at
	// the size dies on it - but at the placeholder limit that one file is 100 MB
	// of the 205 MB an untouched run would write. Twice the limit asks the same
	// question of a form that checks after reading, and somebody who wants the
	// heavier case types --far-over 10x.
	defaultFarOver = "2x"
	farOverOff     = "off"
	defaultBulk    = "50"
	// A mass upload of ten thousand files is already a thousand times the limit
	// in bytes and far past what any form takes at once. The ceiling is here so
	// that a slip of the keyboard is refused by the declaration rather than
	// discovered as a full disk.
	mostBulk = 10000

	// The fractions of the limit the set is built from. Half the limit is
	// comfortably inside it for the positive control, and a tenth of it is a
	// file somebody would really upload fifty of at once.
	halfShare = 2
	bulkShare = 10

	// uploadSample is how big a file that is about its NAME or what is INSIDE
	// it should be. Those files say nothing about size, so they are as small as
	// they can be without being about smallness too.
	uploadSample = 4 << 10

	// longestExtension is what this preset will take as one. Nothing on any
	// filesystem here is near it - it is a bound rather than a rule, so that a
	// pasted sentence is refused as a sentence instead of becoming a file name.
	longestExtension = 16

	sizeGroup       = "size-limit"
	degenerateGroup = "degenerate"
	allowedGroup    = "allowed-types"
	deniedGroup     = "denied-types"
	mismatchGroup   = "extension-content-mismatch"
	anomalyGroup    = "extension-anomalies"
	nameGroup       = "hostile-names"
	bulkGroup       = "bulk-upload"

	// fillerFormat is what a file holds when the set needs bytes of no
	// particular kind: an empty file, or a name whose extension the registry
	// has never heard of.
	fillerFormat = "txt"

	// uploadQuestion is announced by the preset AND written into the header of
	// an ejected recipe. Named once rather than typed twice.
	uploadQuestion = "Does my upload form take what it should and turn the rest away?"
)

func init() {
	Register(Preset{
		ID:       uploadID,
		Title:    "Upload validation",
		Question: uploadQuestion,

		Parameters: []format.Property{
			{
				Name: uploadLimitParam, Kind: format.PropertySize,
				Default: defaultUploadLimit,
				Detail: "The size limit your upload form declares. This set takes one step either " +
					"side of it - for a file at every distance, run the size-boundaries preset.",
			},
			{
				Name: allowParam, Kind: format.PropertyText,
				Shape:   "format ids separated by commas",
				Default: defaultAllow,
				Detail: "Which types your form is supposed to accept. Each one becomes a real file " +
					"of that type, and they are the positive control of the whole set.",
			},
			{
				Name: denyParam, Kind: format.PropertyText,
				Shape:   "extensions separated by commas",
				Default: defaultDeny,
				Detail: "Which extensions your form is supposed to turn away. An extension this build " +
					"has no format for still gets a file under that name, holding plain text.",
			},
			{
				Name: farOverParam, Kind: format.PropertyChoice,
				Choices: []string{"10x", "2x", farOverOff},
				Default: defaultFarOver,
				Detail: "How far past the limit the one big file goes. Turn it off where writing " +
					"several times the limit is not worth the disk.",
			},
			{
				Name: bulkParam, Kind: format.PropertyInt,
				Min: 0, Max: mostBulk, Unit: "files",
				Default: defaultBulk,
				Detail: "How many files the mass upload holds. Nought leaves that group out " +
					"of the set altogether.",
			},
		},

		SaidWhenDefaulted: map[string]string{
			uploadLimitParam: "no limit was given, so this set is built around " + defaultUploadLimit +
				" - that is our placeholder and not your form's limit. Pass the limit your form declares, or the files say nothing about it.",
		},

		Requires: []string{"MVP"},
		Catches: []string{
			"a limit enforced in the browser and not on the server",
			"an SVG or an HTML file taken for a picture or for plain text, which is a way to get a script past a form",
			"a file checked by its extension and never opened, so a PDF named .jpg goes through",
			"a form that reads the whole body into memory before it looks at how big it is",
			"an upload named PHOTO.JPG turned away where photo.jpg is taken, or the other way round",
			"a name with spaces, brackets or characters outside ASCII written to disk unchanged",
		},

		Says:   saidAboutTheUploadSet,
		Expand: expandUploadValidation,
	})
}

// saidAboutTheUploadSet names what this set is not, given what it was asked for.
//
// Three silences, and untouchable rule 6 is about all of them. A group that a
// parameter emptied looks exactly like a group that was forgotten. And a file
// whose inside is a stand-in reads as a real one: somebody testing a content
// sniffer against denied.exe would be testing it against plain text and would
// never find out from the file.
func saidAboutTheUploadSet(args Args) []string {
	s, err := settleUpload(args)
	if err != nil {
		// Expand is about to refuse these same values with a message that names
		// the one that is wrong. A sentence here would be a second opinion.
		return nil
	}
	var out []string
	if stood := standIns(s.denied); len(stood) > 0 {
		out = append(out, fmt.Sprintf(
			"this build has no format called %s, so %s %s plain text under %s. That tests a form reading the end of a name, not one reading what is inside.",
			joinWithOr(stood), joinWithAnd(namesOf(stood)),
			core.Noun(len(stood), "holds", "hold"),
			core.Noun(len(stood), "that name", "those names")))
	}
	if s.farOver == 0 {
		out = append(out, "far-over is off, so nothing in this set is well past the limit. The largest file is one byte over it.")
	}
	if s.bulk == 0 {
		out = append(out, "bulk is nought, so this set holds no mass upload.")
	}
	if len(s.allowed) < 2 {
		out = append(out, "only one type is allowed, so the set holds no file named as one allowed type and filled with another - that needs two.")
	}
	return out
}

// standIns is the denied extensions this build has no format for.
func standIns(denied []deniedEntry) []string {
	var out []string
	for _, entry := range denied {
		if !entry.known {
			out = append(out, entry.ext)
		}
	}
	return out
}

// joinWithAnd and joinWithOr write a list the way a sentence takes one.
//
// A comma between every pair is how a machine writes a list and it reads as an
// enumeration rather than as a sentence: "denied.exe, denied.sh holds plain
// text" has no number to agree with. The text rules in CLAUDE.md ask for a
// sentence, so the last pair gets its conjunction.
func joinWithAnd(items []string) string { return joinWith(items, "and") }

func joinWithOr(items []string) string { return joinWith(items, "or") }

func joinWith(items []string, conjunction string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " " + conjunction + " " + items[len(items)-1]
}

func namesOf(extensions []string) []string {
	out := make([]string, 0, len(extensions))
	for _, ext := range extensions {
		out = append(out, "denied."+ext)
	}
	return out
}
