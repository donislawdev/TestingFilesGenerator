package parts

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// KindOfFile is the picture drawn in front of a format in a menu.
//
// Twenty formats in a list, three letters each, in one alphabetical run:
// bmp, csv, docx, gif, html, ico, jpg, json and so on down. Somebody who knows
// they want "some kind of picture" has to read the list and recognise the
// abbreviations, and eight of the twenty fit on the screen at once. Reported in
// the design audit of 2026-08-20.
//
// A picture rather than a grouping, and that is a constraint rather than a
// preference. The order of a closed set is a written rule with a guard behind
// it - one order in every surface, so the menu, "tfg formats" and the wording
// of a refusal cannot describe one format three ways. Grouping the menu would
// break that. An icon changes nothing about the order and nothing about the
// height of a row.
//
// The toolkit's own file icons were tried first and are useless for this: at
// the size a row draws them, FileImageIcon, FileTextIcon and DocumentIcon are
// the same sheet of paper with different creases. Looked at on a render on
// 2026-08-20 rather than chosen from the names - six rows of one icon, which is
// worse than no icon because it looks like information and is not. What is used
// instead is the set that draws six different SHAPES.
//
// The classification lives here rather than in the registry because it is a
// fact about drawing rather than about the format: nothing in the engine, the
// recipe or the manifest needs to know that a BMP is a picture, and putting it
// on the Descriptor would put it into "tfg formats --json", which is a contract
// key (untouchable rule 10).
//
// A list kept by hand is a list that goes stale in silence - O80 names two
// of those already. TestEveryRegisteredFormatHasAKind is what stops this
// becoming the third: a format registered without a kind fails the suite rather
// than quietly drawing nothing.
func KindOfFile(id string) fyne.Resource {
	switch fileKinds[id] {
	case kindPicture:
		return theme.MediaPhotoIcon()
	case kindDocument:
		return theme.DocumentIcon()
	case kindArchive:
		return theme.StorageIcon()
	case kindSound:
		return theme.MediaMusicIcon()
	case kindMoving:
		return theme.MediaVideoIcon()
	case kindWords:
		return theme.ListIcon()
	case kindUnknown:
		// Named rather than left out. A format whose kind nobody declared
		// draws no icon, which is the sentence at the top of this function,
		// and a kind added later reddens the linter rather than landing here.
	}
	return nil
}

type fileKind int

const (
	kindUnknown fileKind = iota
	// kindPicture is anything a person would open expecting to see an image.
	kindPicture
	// kindDocument is a page somebody would open in an office application.
	kindDocument
	// kindArchive holds other files.
	kindArchive
	// kindSound and kindMoving are the two media kinds. Nothing is registered
	// as moving yet - MP4 is one of the five formats still to come - and the
	// entry is here rather than added later because the guard below reads this
	// table and the day MP4 arrives is the day somebody is thinking about
	// something else.
	kindSound
	kindMoving
	// kindWords is text and structured data: what a parser reads.
	kindWords
)

// fileKinds is every registered format, sorted into kinds.
//
// Kept in one table rather than declared per format, so the whole
// classification can be read at once and the guard can compare it against the
// registry in one place.
var fileKinds = map[string]fileKind{
	"bmp":   kindPicture,
	"gif":   kindPicture,
	"ico":   kindPicture,
	"jpg":   kindPicture,
	"png":   kindPicture,
	"svg":   kindPicture,
	"tiff":  kindPicture,
	"webp":  kindPicture,
	"avif":  kindPicture,
	"docx":  kindDocument,
	"pdf":   kindDocument,
	"pptx":  kindDocument,
	"xlsx":  kindDocument,
	"targz": kindArchive,
	"zip":   kindArchive,
	"wav":   kindSound,
	"csv":   kindWords,
	"html":  kindWords,
	"json":  kindWords,
	"log":   kindWords,
	"md":    kindWords,
	"txt":   kindWords,
	"xml":   kindWords,
}

// FileKinds is the table, for the guard that keeps it in step with the
// registry. Exported as names rather than as the type, because what the guard
// asks is "does every format have one", not "which one".
func FileKinds() map[string]bool {
	out := make(map[string]bool, len(fileKinds))
	for id, kind := range fileKinds {
		out[id] = kind != kindUnknown
	}
	return out
}

// SameKind says whether two formats are the same kind of thing, for the guard
// that checks two DIFFERENT kinds are not drawn with one picture. Formats of
// one kind share a picture on purpose.
func SameKind(a, b string) bool { return fileKinds[a] == fileKinds[b] }

// IsEveryFormat says whether a menu is offering the whole list of formats,
// which is what earns it the pictures. See NewChooser for why the question is
// asked there rather than at each list.
//
// Asked of this table rather than of the registry, and that is what keeps the
// answer honest without a second import: the table is already held against the
// registry by TestEveryRegisteredFormatHasAKind, so a format registered without
// a kind fails that guard rather than quietly making this one say no.
//
// The whole set rather than "every value happens to be a format", because a
// subset is exactly the case that must NOT get pictures - two values of one
// kind would draw one picture twice.
func IsEveryFormat(values []string) bool {
	if len(values) != len(fileKinds) {
		return false
	}
	for _, v := range values {
		if fileKinds[v] == kindUnknown {
			return false
		}
	}
	return true
}
