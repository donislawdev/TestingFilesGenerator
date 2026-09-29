package pdf

// The settings this format declares - what the window draws, what tfg formats
// prints and what the registry checks a value against before the generator
// ever sees it.

import (
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// documentProperties is the block of settings describing the document rather
// than its pages. The window draws the name above them and tfg formats prints
// it, see format.Property.Group.
const documentProperties = "Document properties"

// anyText is what a setting written as free text accepts, said once for the
// six of them.
const anyText = "any text"

// dateText is what a date accepts. The three ways a date may be written are
// the ones dateShape reads, and none leaves the date out.
const dateText = "a date such as 2024-02-29 or 2024-02-29T13:45:00+02:00, or none"

var properties = []format.Property{
	{
		Name: "pages", Kind: format.PropertyInt,
		Min: 1, Max: maxPages,
		Default: strconv.Itoa(defaultPages),
		Detail:  "How many pages the document has.",
	},
	{
		Name: "page_size", Kind: format.PropertyChoice,
		// Written out rather than read from the map, so one build cannot
		// offer a different set from the next. The ORDER is not decided here:
		// registration sorts every closed set, so the menu in the window,
		// "tfg formats pdf" and the wording of a refusal all list them the
		// same way round.
		Choices: []string{"a4", "a3", "a5", "letter", "legal", mixed},
		Default: "a4",
		Detail: "The paper size every page uses. Mixed goes through a4, letter, legal, a3 and a5 " +
			"in turn, one a page, and needs at least two pages.",
	},
	{
		Name: "orientation", Kind: format.PropertyChoice,
		Choices: []string{orientPortrait, orientLandscape, mixed},
		Default: orientPortrait,
		Detail: "Whether pages stand upright or lie wide. Mixed alternates the two, starting upright, " +
			"and needs at least two pages.",
	},
	{
		Name: "rotate", Kind: format.PropertyChoice,
		Choices: []string{"0", "90", "180", "270"},
		Default: "0",
		Detail: "Asks the reader to turn every page clockwise by this many degrees when it shows it. " +
			"The page itself stays upright, so a reader that ignores the request shows it unturned.",
	},
	{
		Name: "pdf_version", Kind: format.PropertyChoice,
		Choices: []string{"1.4", defaultVersion},
		Default: defaultVersion,
		Detail: "The version written at the start of the file. Nothing else changes, " +
			"because the document uses nothing newer than 1.4.",
	},
	{
		Name: "title", Kind: format.PropertyText, Shape: anyText, Group: documentProperties,
		Detail: "The title a reader shows in its title bar and in the document properties. " +
			"Left empty, the title is the self describing label, or the name of this tool without one.",
	},
	{
		Name: "author", Kind: format.PropertyText, Shape: anyText, Group: documentProperties,
		Detail: "The author in the document properties. Left empty, the document names none.",
	},
	{
		Name: "subject", Kind: format.PropertyText, Shape: anyText, Group: documentProperties,
		Detail: "The subject in the document properties. Left empty, the document has none.",
	},
	{
		Name: "keywords", Kind: format.PropertyText, Shape: anyText, Group: documentProperties,
		Detail: "The keywords in the document properties, as one line of text. Left empty, the document has none.",
	},
	{
		Name: "creator", Kind: format.PropertyText, Shape: anyText, Group: documentProperties,
		Detail: "The program the document says it was written in, such as Microsoft Word. " +
			"Left empty, the document names none.",
	},
	{
		Name: "producer", Kind: format.PropertyText, Shape: anyText, Group: documentProperties,
		Default: defaultProducer,
		Detail:  "The program the document says turned it into a PDF.",
	},
	{
		Name: "created", Kind: format.PropertyText, Shape: dateText, Group: documentProperties,
		Default: defaultCreated,
		Detail: "When the document says it was created. A time written without a zone is kept without one, " +
			"and none leaves the date out.",
	},
	{
		Name: "modified", Kind: format.PropertyText, Shape: dateText, Group: documentProperties,
		Detail: "When the document says it was last changed, written the same way as created. " +
			"Left empty, the document has no such date.",
	},
}
