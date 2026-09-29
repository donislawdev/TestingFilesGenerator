package pdf

// Reading the settings a request carries: how many pages, which paper, which
// way up, and what the document says about itself.

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

const (
	defaultPages = 1
	maxPages     = 5000

	defaultVersion = "1.7"

	orientPortrait  = "portrait"
	orientLandscape = "landscape"
	// mixed is the value page_size and orientation share for "not all alike".
	mixed = "mixed"
)

type pageSize struct {
	name          string
	width, height int
}

var pageSizes = map[string]pageSize{
	"a4":     {"A4", 595, 842},
	"a3":     {"A3", 842, 1191},
	"a5":     {"A5", 420, 595},
	"letter": {"Letter", 612, 792},
	"legal":  {"Legal", 612, 1008},
}

// mixedSizes is the order page_size=mixed walks through, one size a page.
//
// Taken from the page number rather than drawn from the seed, and that is the
// lesson of lineWidth: a draw would make the smallest document one seed can
// produce differ from the next. A4 and Letter come first because a document
// mixing the two is the one people actually meet - pages scanned on each side
// of the Atlantic and put together.
var mixedSizes = []string{"a4", "letter", "legal", "a3", "a5"}

// options is everything a request asks of the document, read once.
type options struct {
	pages int
	// sizes is one size, or the cycle page_size=mixed walks through.
	sizes       []pageSize
	sizeName    string
	orientation string
	rotate      int
	version     string
	info        docInfo
}

// geometry is the paper page i is drawn on, the right way up.
//
// Landscape swaps the two sides of the page itself, so every reader shows it
// wide. That is a different thing from rotate, which leaves the page upright
// and asks the reader to turn it - see the declaration.
func (o options) geometry(i int) pageSize {
	s := o.sizes[i%len(o.sizes)]
	if o.orientation == orientLandscape || (o.orientation == mixed && i%2 == 1) {
		s.width, s.height = s.height, s.width
	}
	return s
}

// readOptions reads the settings, or says which one cannot be used.
//
// The registry has already refused a key it does not know and a value outside
// a declared set, so what is refused here is what only this format can judge:
// a date that does not exist, text that is not text, and "mixed" in a document
// with one page.
func readOptions(props map[string]string) (options, error) {
	pages, err := pageCount(props)
	if err != nil {
		return options{}, err
	}
	o := options{pages: pages, version: defaultVersion, orientation: orientPortrait}
	if o.sizes, o.sizeName, err = paperSizes(props, pages); err != nil {
		return options{}, err
	}
	if raw := props["orientation"]; raw != "" {
		if raw == mixed && pages < 2 {
			return options{}, needsTwoPages("orientation", "choose portrait or landscape")
		}
		o.orientation = raw
	}
	if o.rotate, err = rotation(props); err != nil {
		return options{}, err
	}
	if raw := props["pdf_version"]; raw != "" {
		o.version = raw
	}
	if o.info, err = readInfo(props); err != nil {
		return options{}, err
	}
	return o, nil
}

func pageCount(props map[string]string) (int, error) {
	raw, ok := props["pages"]
	if !ok || raw == "" {
		return defaultPages, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("pdf: pages must be a whole number, got %q", raw)
	}
	if n < 1 || n > maxPages {
		return 0, fmt.Errorf("pdf: pages must be between 1 and %d, got %d", maxPages, n)
	}
	return n, nil
}

// paperSizes is the paper every page uses, or the cycle a mixed document
// walks through, and the name the manifest gives it.
func paperSizes(props map[string]string, pages int) ([]pageSize, string, error) {
	raw, ok := props["page_size"]
	if !ok || raw == "" {
		return []pageSize{pageSizes["a4"]}, pageSizes["a4"].name, nil
	}
	if raw == mixed {
		if pages < 2 {
			return nil, "", needsTwoPages("page_size", "choose one size such as a4")
		}
		cycle := make([]pageSize, 0, len(mixedSizes))
		for _, k := range mixedSizes {
			cycle = append(cycle, pageSizes[k])
		}
		return cycle, mixed, nil
	}
	s, ok := pageSizes[strings.ToLower(raw)]
	if !ok {
		names := make([]string, 0, len(pageSizes))
		for k := range pageSizes {
			names = append(names, k)
		}
		return nil, "", fmt.Errorf("pdf: page_size %q is not one of: %s", raw, strings.Join(sorted(names), ", "))
	}
	return []pageSize{s}, s.name, nil
}

// needsTwoPages refuses "mixed" in a document of one page.
//
// Refused rather than written, because the file would be a single page of one
// size and one way up, with a manifest saying it is mixed. Somebody testing how
// a reader copes with pages that differ would get a pass from a document where
// nothing differs.
func needsTwoPages(key, other string) *format.PropertyValueError {
	return &format.PropertyValueError{
		Format: "pdf",
		Key:    key,
		Value:  mixed,
		Reason: "mixed pages need at least two pages and this document has one",
		Remedy: fmt.Sprintf("Set pages to 2 or more, or %s.", other),
	}
}

// rotation is the turn every page asks the reader for, in degrees.
func rotation(props map[string]string) (int, error) {
	raw := props["rotate"]
	if raw == "" {
		return 0, nil
	}
	switch raw {
	case "0", "90", "180", "270":
		return strconv.Atoi(raw)
	}
	return 0, fmt.Errorf("pdf: rotate %q is not one of: 0, 90, 180, 270", raw)
}

func sorted(in []string) []string {
	out := append([]string(nil), in...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}
