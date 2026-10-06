package pdf

// The smallest document this format produces, and how a refusal explains it.

import (
	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// documentWords names the document a refusal is about, the way a person would:
// "1 page A4 document", "3 page mixed size upright and landscape document".
// The words for the settings nobody changed are the words this refusal always
// used.
func documentWords(o options) core.Said {
	size := core.Says("pdf.Size", "%s", core.A("Size", core.Choice{Of: "page_size", Value: o.sizeName}))
	if o.sizeName == mixed {
		size = core.Says("pdf.MixedSize", "mixed size")
	}
	switch o.orientation {
	case orientLandscape:
		return core.Says("pdf.DocumentLandscape", "%d page %s landscape document", core.A("Pages", o.pages), core.A("Size", size))
	case mixed:
		return core.Says("pdf.DocumentMixed", "%d page %s upright and landscape document", core.A("Pages", o.pages), core.A("Size", size))
	}
	return core.Says("pdf.Document", "%d page %s document", core.A("Pages", o.pages), core.A("Size", size))
}

// carrying and cleanHint keep the message about the minimum honest. The
// figure in the registry is the smallest document with no label and nothing
// said about it, so a user who left the label on, or typed a title, and hits
// the limit needs to be told why the number they were shown is not the number
// they got.
func carrying(label bool, o options) core.Said {
	var what core.Conjoined
	if label {
		what = append(what, core.Says("pdf.TheLabel", "the self describing label"))
	}
	if o.info.asked {
		what = append(what, core.Says("pdf.TheProperties", "the document properties that were set"))
	}
	if len(what) == 0 {
		return core.Said{}
	}
	return core.Says("pdf.Carrying", " carrying %s", core.A("What", what))
}

func cleanHint(label bool, o options) core.Said {
	switch {
	case label && o.info.asked:
		return core.Says("pdf.HintAll", ", ask for fewer pages by setting pages to 1, shorten the document properties, or drop the label")
	case label:
		return core.Says("pdf.HintLabel", ", ask for fewer pages by setting pages to 1, or drop the label")
	case o.info.asked:
		return core.Says("pdf.HintProperties", ", ask for fewer pages by setting pages to 1, or shorten the document properties")
	}
	return core.Says("pdf.HintPages", " or ask for fewer pages by setting pages to 1")
}

// minimumBytes is the smallest document this generator can produce: one A4
// page with no label and every setting at its default. Measured at start up
// rather than guessed.
func minimumBytes() int64 {
	opts, err := readOptions(nil)
	if err != nil {
		panic("pdf: the default settings are refused: " + err.Error())
	}
	prefix, suffix := document(memo{opts: opts})
	return int64(len(prefix) + len(suffix))
}
