package eml

import (
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/archive"
)

// Setting names. Public names under rule 10, so each is spelled once.
const (
	Body          = "body"
	TextEncoding  = "text_encoding"
	Headers       = "headers"
	LineEndings   = "line_endings"
	FilenameStyle = "filename_style"

	Attachments      = "attachments"
	AttachmentFormat = "attachment_format"
	AttachmentSize   = "attachment_size"
)

// The values of body.
const (
	Plain       = "plain"
	HTML        = "html"
	Alternative = "alternative"
)

// The values of text_encoding, spelled the way Content-Transfer-Encoding
// spells them, because that is the line a tester reads them back from.
const (
	SevenBit        = "7bit"
	EightBit        = "8bit"
	QuotedPrintable = "quoted-printable"
)

// The values of headers.
const (
	ASCII   = "ascii"
	Encoded = "encoded"
	UTF8    = "utf8"
)

// The values of line_endings.
const (
	CRLF = "crlf"
	LF   = "lf"
)

// The values of filename_style. Named after the standard each one follows or
// breaks, and content-type after the only header it writes.
const (
	RFC2231     = "rfc2231"
	RFC2047     = "rfc2047"
	Both        = "both"
	ContentType = "content-type"
)

// attachmentsGroup is the block the attachment settings sit in, on both
// surfaces.
const attachmentsGroup = "Attached files"

// defaultAttachments is none: a message is text first, and a file attached by
// default would put base64 in front of somebody who asked for a mail.
const defaultAttachments = 0

// settings is every choice a message is made with, read once in Plan.
type settings struct {
	body     string
	encoding string
	headers  string
	endings  string
	names    string
	// eol is what ends every line of the file, and its length is what the
	// arithmetic counts - never a two written as a constant.
	eol string
}

// international says whether the text carries the line in Polish and
// Japanese - which is what quoted-printable and 8bit exist for, and what 7bit
// cannot carry at all.
func (s settings) international() bool { return s.encoding != SevenBit }

// door is how this format names its contents, for archive.GroupsThrough.
var door = archive.Door{
	Members:      members,
	DefaultCount: defaultAttachments,
	Fewer:        core.Says("eml.AskForAttachmentsOrFewer", "Ask for %d attachments or fewer.", core.A("Max", archive.MaxMembers)),
}

var members = format.Members{Count: Attachments, Format: AttachmentFormat, Size: AttachmentSize}

// parse reads the settings a request carries.
//
// A value outside the declaration has already been refused by the registry,
// which checks every format in one place. The check stays here for the reason
// geojson gives: a guard calls Plan directly, and a generator that trusts its
// input is one registry change away from writing a message nobody ordered.
// The three attachment settings are read by archive.GroupsThrough, beside
// contains, because the two doors share their rules.
func parse(props map[string]string) (settings, error) {
	values := map[string]string{}
	for _, p := range textProperties() {
		v, err := valueOf(props, p)
		if err != nil {
			return settings{}, err
		}
		values[p.Name] = v
	}
	names, err := valueOf(props, filenameStyle())
	if err != nil {
		return settings{}, err
	}
	s := settings{
		body:     values[Body],
		encoding: values[TextEncoding],
		headers:  values[Headers],
		endings:  values[LineEndings],
		names:    names,
		eol:      "\r\n",
	}
	if s.endings == LF {
		s.eol = "\n"
	}
	return s, nil
}

// valueOf is one setting's value, or its default, refused in the
// declaration's own words when the declaration does not allow it.
func valueOf(props map[string]string, p format.Property) (string, error) {
	v, ok := props[p.Name]
	if !ok || v == "" {
		return p.Default, nil
	}
	if bad := p.Allows(v); !bad.IsZero() {
		return "", &format.PropertyValueError{Format: "eml", Key: p.Name, Value: v, Reason: bad, Remedy: p.Instead()}
	}
	return v, nil
}

// properties is the declaration, in the order a person meets it: what the
// message says and how it is written, then what it carries.
func properties() []format.Property {
	return append(textProperties(), attachmentProperties()...)
}

// textProperties are the settings of the text and of the header lines.
func textProperties() []format.Property {
	return []format.Property{
		{
			Name: Body, Kind: format.PropertyChoice,
			Choices: []string{Alternative, HTML, Plain},
			Default: Plain,
			Detail: "Which text the message carries. plain is one text part and html one HTML part. " +
				"alternative carries both with the same words, and a mail program shows one of them.",
		},
		{
			Name: TextEncoding, Kind: format.PropertyChoice,
			Choices: []string{SevenBit, EightBit, QuotedPrintable},
			Default: SevenBit,
			Detail: "How the text is written. 7bit keeps it to plain ASCII. " +
				"quoted-printable and 8bit add a line in Polish and Japanese, because that is what they are for - " +
				"quoted-printable spells those letters as =C5=BC, and 8bit writes them as they are.",
		},
		{
			Name: Headers, Kind: format.PropertyChoice,
			Choices: []string{ASCII, Encoded, UTF8},
			Default: ASCII,
			Detail: "Whether the subject, the names of the sender and the recipient and the names of attached files " +
				"carry letters outside ASCII, and how. encoded writes them the way RFC 2047 and RFC 2231 say. " +
				"utf8 writes them as they are (RFC 6532), and Python's email package reads that and reports a defect in the From and To lines.",
		},
		{
			Name: LineEndings, Kind: format.PropertyChoice,
			Choices: []string{CRLF, LF},
			Default: CRLF,
			Detail: "How every line of the file ends. The standard is crlf. " +
				"Python, MimeKit, mailparser and Go's standard library all read lf too, " +
				"so the difference shows in programs that split the lines themselves.",
		},
	}
}

// attachmentProperties are the settings of what the message carries.
func attachmentProperties() []format.Property {
	return []format.Property{
		{
			Name: Attachments, Kind: format.PropertyInt,
			Min: 0, Max: archive.MaxMembers,
			Default: strconv.Itoa(defaultAttachments),
			Detail: "How many files the message carries. Use contains instead when the files are not all alike. " +
				"mailparser refuses a message of more than 1000 MIME entities, and the manifest gives this one's count as mime_entities.",
			Group: attachmentsGroup,
		},
		{
			Name: AttachmentFormat, Kind: format.PropertyText,
			Shape: "the id of a format, as tfg formats lists them",
			// Not a choice, for the reason the archives give: the allowed
			// values are whatever this build registered.
			Default: archive.DefaultFormat,
			Detail:  "The format of the attached files. Run tfg formats to see what this build supports.",
			Group:   attachmentsGroup,
		},
		{
			Name: AttachmentSize, Kind: format.PropertySize,
			Default: archive.DefaultSizeText,
			Detail: "How big each attached file is. The message grows by about a third more, " +
				"because attached files travel in base64.",
			Group: attachmentsGroup,
		},
		filenameStyle(),
	}
}

// filenameStyle is the one attachment setting this package reads itself.
func filenameStyle() format.Property {
	return format.Property{
		Name: FilenameStyle, Kind: format.PropertyChoice,
		Choices: []string{Both, ContentType, RFC2047, RFC2231},
		Default: RFC2231,
		Detail: "How the names of attached files are written. rfc2231 is the standard way. " +
			"rfc2047 is the encoded form RFC 2047 forbids inside a name, and many readers decode it anyway. " +
			"both writes the two, and content-type puts the name only where old mail programs looked for it. " +
			"Go's standard library reads no name from content-type and the encoded text itself from rfc2047. " +
			"Unless headers is encoded a name is written as it is and only where it goes changes, " +
			"so rfc2047 and both write the same bytes.",
		Group: attachmentsGroup,
	}
}
