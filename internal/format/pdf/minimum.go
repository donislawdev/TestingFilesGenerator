package pdf

// The smallest document this format produces, and how a refusal explains it.

import (
	"fmt"
	"strings"
)

// documentWords names the document a refusal is about, the way a person would:
// "1 page A4 document", "3 page mixed size upright and landscape document".
// The words for the settings nobody changed are the words this refusal always
// used.
func documentWords(o options) string {
	size := o.sizeName
	if size == mixed {
		size = "mixed size"
	}
	way := ""
	switch o.orientation {
	case orientLandscape:
		way = " landscape"
	case mixed:
		way = " upright and landscape"
	}
	return fmt.Sprintf("%d page %s%s document", o.pages, size, way)
}

// carrying and cleanHint keep the message about the minimum honest. The
// figure in the registry is the smallest document with no label and nothing
// said about it, so a user who left the label on, or typed a title, and hits
// the limit needs to be told why the number they were shown is not the number
// they got.
func carrying(label bool, o options) string {
	var what []string
	if label {
		what = append(what, "the self describing label")
	}
	if o.info.asked {
		what = append(what, "the document properties that were set")
	}
	if len(what) == 0 {
		return ""
	}
	return " carrying " + strings.Join(what, " and ")
}

func cleanHint(label bool, o options) string {
	switch {
	case label && o.info.asked:
		return ", ask for fewer pages by setting pages to 1, shorten the document properties, or drop the label"
	case label:
		return ", ask for fewer pages by setting pages to 1, or drop the label"
	case o.info.asked:
		return ", ask for fewer pages by setting pages to 1, or shorten the document properties"
	}
	return " or ask for fewer pages by setting pages to 1"
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
