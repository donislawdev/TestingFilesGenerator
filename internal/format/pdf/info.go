package pdf

// The document information dictionary and the strings written into it.

import (
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

const (
	defaultTitle    = "Testing Files Generator"
	defaultProducer = "Testing Files Generator"

	// defaultCreated is fixed rather than read off the clock, because a
	// timestamp from the clock would make two runs of the same recipe differ.
	defaultCreated = "2020-01-01T00:00:00Z"

	// noDate is the value that leaves a date out of the document altogether.
	noDate = "none"
)

// docInfo is what the document says about itself.
type docInfo struct {
	// title is what was asked for. Empty means the label, or the tool's name
	// when there is no label - worked out in infoObject, because the label is
	// not known until the plan is.
	title    string
	author   string
	subject  string
	keywords string
	creator  string
	producer string
	created  date
	modified date

	// asked says whether any of it was set, which is what the refusal about
	// the minimum needs to know: text somebody typed makes the smallest
	// document bigger, and the refusal has to say so.
	asked bool
}

// date is one date as it was asked for and as the document writes it. An
// empty pdf means the document carries no such date.
type date struct {
	asked string
	pdf   string
}

// textKeys are the settings written as text, and dateKeys the ones written as
// dates. Together they are everything the document says about itself.
var (
	textKeys = []string{"title", "author", "subject", "keywords", "creator", "producer"}
	dateKeys = []string{"created", "modified"}
)

func readInfo(props map[string]string) (docInfo, error) {
	for _, k := range textKeys {
		if raw := props[k]; !utf8.ValidString(raw) {
			return docInfo{}, &format.PropertyValueError{
				Format: "pdf", Key: k, Value: strings.ToValidUTF8(raw, "?"),
				Reason: "it takes text, and this value is not valid UTF-8",
				Remedy: "Write the value in UTF-8.",
			}
		}
	}
	info := docInfo{
		title: props["title"], author: props["author"], subject: props["subject"],
		keywords: props["keywords"], creator: props["creator"], producer: props["producer"],
	}
	for _, k := range append(append([]string(nil), textKeys...), dateKeys...) {
		info.asked = info.asked || props[k] != ""
	}
	if info.producer == "" {
		info.producer = defaultProducer
	}
	var err error
	if info.created, err = readDate(props, "created", defaultCreated); err != nil {
		return docInfo{}, err
	}
	if info.modified, err = readDate(props, "modified", ""); err != nil {
		return docInfo{}, err
	}
	return info, nil
}

// dateShape is the three ways a date may be written: a day, a day and a time
// with no zone, and a day and a time in a zone.
//
// Strict on purpose. Go's own parser takes a fraction of a second after the
// seconds even when the layout has none, and a PDF date has no fractions - so
// accepting one would mean writing a date other than the one asked for.
var dateShape = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})(?:T(\d{2}):(\d{2}):(\d{2})(Z|[+-]\d{2}:\d{2})?)?$`)

// readDate reads one date, or says why it cannot be written.
//
// A time with no zone is written with no zone, never in the zone of the
// machine: two machines would otherwise write two files from one recipe.
func readDate(props map[string]string, key, fallback string) (date, error) {
	raw := props[key]
	if raw == "" {
		raw = fallback
	}
	if raw == "" || raw == noDate {
		return date{asked: raw}, nil
	}
	m := dateShape.FindStringSubmatch(raw)
	if m == nil {
		return date{}, &format.PropertyValueError{
			Format: "pdf", Key: key, Value: raw,
			Reason: "it takes a date written as 2024-02-29, 2024-02-29T13:45:00 or 2024-02-29T13:45:00+02:00, or none",
			Remedy: "Write the date in one of those three ways, with no fraction of a second.",
		}
	}
	if !exists(m) {
		return date{}, &format.PropertyValueError{
			Format: "pdf", Key: key, Value: raw,
			Reason: "no calendar has that day, time or zone",
			Remedy: "Write a date that exists, such as 2024-02-29T13:45:00+02:00.",
		}
	}
	pdf := "D:" + m[1] + m[2] + m[3] + m[4] + m[5] + m[6]
	switch zone := m[7]; {
	case zone == "Z":
		pdf += "Z"
	case zone != "":
		pdf += zone[:3] + "'" + zone[4:] + "'"
	}
	return date{asked: raw, pdf: pdf}, nil
}

// exists says whether the parts of a date name a real moment: the 29th of
// February only in a leap year, no hour 24, no second 60, no zone past 23:59.
func exists(m []string) bool {
	n := func(s string) int {
		v, _ := strconv.Atoi(s)
		return v
	}
	y, mo, d := n(m[1]), n(m[2]), n(m[3])
	h, mi, s := n(m[4]), n(m[5]), n(m[6])
	if h > 23 || mi > 59 || s > 59 {
		return false
	}
	t := time.Date(y, time.Month(mo), d, h, mi, s, 0, time.UTC)
	if t.Year() != y || int(t.Month()) != mo || t.Day() != d {
		return false
	}
	if zone := m[7]; len(zone) == 6 {
		return n(zone[1:3]) <= 23 && n(zone[4:6]) <= 59
	}
	return true
}

// infoObject is the document information dictionary.
//
// The keys that are always there come in the order they always came, and a
// key somebody asked for goes where the specification lists it. Nothing is
// written for a value nobody asked for, so a document with no settings is the
// document this format wrote before it had any.
func infoObject(m memo) string {
	i := m.opts.info
	var b strings.Builder
	b.WriteString("<<")
	entry(&b, "Title", titleOf(m))
	entry(&b, "Author", i.author)
	entry(&b, "Subject", i.subject)
	entry(&b, "Keywords", i.keywords)
	entry(&b, "Creator", i.creator)
	entry(&b, "Producer", i.producer)
	if i.created.pdf != "" {
		fmt.Fprintf(&b, "/CreationDate(%s)", i.created.pdf)
	}
	if i.modified.pdf != "" {
		fmt.Fprintf(&b, "/ModDate(%s)", i.modified.pdf)
	}
	b.WriteString(">>")
	return b.String()
}

// titleOf is the title the document carries: the one asked for, or the label,
// or the tool's name.
func titleOf(m memo) string {
	switch {
	case m.opts.info.title != "":
		return m.opts.info.title
	case m.label != "":
		return m.label
	}
	return defaultTitle
}

func entry(b *strings.Builder, key, value string) {
	if value == "" {
		return
	}
	b.WriteString("/" + key + pdfString(value))
}

// pdfString writes text the way every reader reads it back unchanged.
//
// Printable ASCII goes between brackets, as it always has. Anything else is
// written as UTF-16 with a byte order mark, in hex. A bracketed string is read
// in PDFDocEncoding, which has no Polish letters and turns a carriage return
// into a line feed - so "Zażółć" or a title with a line break in it would come
// back as something else.
func pdfString(s string) string {
	if !printableASCII(s) {
		return utf16Hex(s)
	}
	return "(" + escapeString(s) + ")"
}

func printableASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] > 0x7e {
			return false
		}
	}
	return true
}

// utf16Hex is s as UTF-16 with a byte order mark, written in hex.
func utf16Hex(s string) string {
	units := utf16.Encode([]rune(s))
	raw := make([]byte, 0, 2+2*len(units))
	raw = append(raw, 0xfe, 0xff)
	for _, u := range units {
		raw = append(raw, byte(u>>8), byte(u))
	}
	return "<" + strings.ToUpper(hex.EncodeToString(raw)) + ">"
}

// escapeString protects the three characters that end or nest a PDF string.
// Without this a label containing a bracket would produce a file no reader
// can parse.
func escapeString(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`)
	return r.Replace(s)
}
