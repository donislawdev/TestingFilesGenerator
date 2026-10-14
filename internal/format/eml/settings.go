package eml

import (
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/archive"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/picture"
)

// Setting names. Public names under rule 10, so each is spelled once.
const (
	Body          = "body"
	TextEncoding  = "text_encoding"
	Charset       = "charset"
	Headers       = "headers"
	Recipients    = "recipients"
	LineEndings   = "line_endings"
	FilenameStyle = "filename_style"

	Attachments      = "attachments"
	AttachmentFormat = "attachment_format"
	AttachmentSize   = "attachment_size"
	MessageDepth     = "message_depth"
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
	Base64          = "base64"
	QuotedPrintable = "quoted-printable"
)

// The values of charset, spelled the way the charset parameter spells them.
const (
	CharsetUTF8        = "utf-8"
	CharsetLatin2      = "iso-8859-2"
	CharsetWindows1250 = "windows-1250"
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
// breaks, content-type after the only header it writes, and none after what it
// writes.
const (
	RFC2231     = "rfc2231"
	RFC2047     = "rfc2047"
	Both        = "both"
	ContentType = "content-type"
	NoName      = "none"
)

// attachmentsGroup is the block the attachment settings sit in, on both
// surfaces.
const attachmentsGroup = "Attached files"

// defaultAttachments is none: a message is text first, and a file attached by
// default would put base64 in front of somebody who asked for a mail.
const defaultAttachments = 0

const (
	// maxRecipients is the most addresses To carries. A hundred thousand is a
	// header of 3.1 MB - past the 1 MiB mailparser refuses, which about 33 800
	// addresses reach, and long enough for the minutes Python's email package
	// takes over it (measured 2026-10-13, docs/EML-2-2026-10-13.md section 7).
	maxRecipients = 100000
	// maxDepth is the most messages one message holds inside each other. Past
	// the 488 at which Python's email package runs out of recursion and the 514
	// at which MimeKit stops taking them apart, measured on this generator's
	// own messages the same day, and at about a megabyte still a message a
	// test can send.
	maxDepth = 2000
)

// settings is every choice a message is made with, read once in Plan.
type settings struct {
	body     string
	encoding string
	// charset is the one asked for. What the text parts declare is
	// declared(), which is us-ascii whenever the text is plain ASCII.
	charset string
	headers string
	endings string
	names   string
	// eol is what ends every line of the file, and its length is what the
	// arithmetic counts - never a two written as a constant.
	eol string
	// recipients is how many addresses To carries and depth how many
	// messages deep the message goes, itself counted.
	recipients int
	depth      int
	pictures   picture.Settings
}

// international says whether the text carries the line in Polish (and in
// Japanese, where the character set has it) - which is what quoted-printable,
// 8bit and base64 exist for, and what 7bit cannot carry at all.
func (s settings) international() bool { return s.encoding != SevenBit }

// textEOL is what ends the lines of the text itself. Inside base64 the text
// is in its canonical form, which RFC 2045 section 6.8 says ends lines in
// CRLF whatever the file around it does.
func (s settings) textEOL() string {
	if s.encoding == Base64 {
		return "\r\n"
	}
	return s.eol
}

// door is how this format names its contents, for archive.GroupsThrough.
var door = archive.Door{
	Members:      members,
	DefaultCount: defaultAttachments,
	Fewer:        core.Says("eml.AskForAttachmentsOrFewer", "Ask for %d attachments or fewer.", core.A("Max", archive.MaxMembers)),
	Nests:        true,
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
	for _, p := range ownProperties() {
		v, err := valueOf(props, p)
		if err != nil {
			return settings{}, err
		}
		values[p.Name] = v
	}
	s := settings{
		body:     values[Body],
		encoding: values[TextEncoding],
		charset:  values[Charset],
		headers:  values[Headers],
		endings:  values[LineEndings],
		names:    values[FilenameStyle],
		eol:      "\r\n",
	}
	if s.endings == LF {
		s.eol = "\n"
	}
	var err error
	if s.recipients, err = strconv.Atoi(values[Recipients]); err != nil {
		return settings{}, core.Defect(err)
	}
	if s.depth, err = strconv.Atoi(values[MessageDepth]); err != nil {
		return settings{}, core.Defect(err)
	}
	s.pictures, err = picture.Parse(values)
	return s, err
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
// message says and how it is written, then what it carries, then the pictures
// its text shows.
func properties() []format.Property {
	out := append(textProperties(), attachmentProperties()...)
	return append(out, pictureProperties()...)
}

// ownProperties are the settings this package reads itself - all but the
// three attachment settings archive.GroupsThrough reads.
func ownProperties() []format.Property {
	out := append(textProperties(), filenameStyle(), depthProperty())
	return append(out, pictureProperties()...)
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
			Choices: []string{SevenBit, EightBit, Base64, QuotedPrintable},
			Default: SevenBit,
			Detail: "How the text is written. 7bit keeps it to plain ASCII. " +
				"quoted-printable, 8bit and base64 add a line in Polish, and in Japanese when charset is utf-8, because that is what they are for. " +
				"quoted-printable spells those letters as =C5=BC, 8bit writes them as they are, and base64 writes the whole text in base64. " +
				"To reach every size base64 ends the text with empty lines, and with crlf an odd size gives the Message-ID one more digit.",
		},
		charsetProperty(),
		{
			Name: Headers, Kind: format.PropertyChoice,
			Choices: []string{ASCII, Encoded, UTF8},
			Default: ASCII,
			Detail: "Whether the subject, the names of the sender and the recipient and the names of attached files " +
				"carry letters outside ASCII, and how. encoded writes them the way RFC 2047 and RFC 2231 say. " +
				"utf8 writes them as they are (RFC 6532), and Python's email package reads that and reports a defect in the From and To lines.",
		},
		recipientsProperty(),
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

// charsetProperty is the character set the text is written in.
func charsetProperty() format.Property {
	return format.Property{
		Name: Charset, Kind: format.PropertyChoice,
		Choices: []string{CharsetLatin2, CharsetUTF8, CharsetWindows1250},
		Default: CharsetUTF8,
		Detail: "The character set of the text when text_encoding is not 7bit. iso-8859-2 and windows-1250 hold Polish and no Japanese, " +
			"and they write ą, ś and ź as different bytes, so a program that takes one for the other shows the wrong letters. " +
			"MimeKit with no code page provider registered reads both as Latin-1, and Go's standard library converts no character set at all. " +
			"With 7bit the text is plain ASCII and says us-ascii, whatever this is set to.",
	}
}

// recipientsProperty is how many addresses To carries.
func recipientsProperty() format.Property {
	return format.Property{
		Name: Recipients, Kind: format.PropertyInt,
		Min: 1, Max: maxRecipients,
		Default: "1",
		Detail: "How many addresses the To line carries, each on a line of its own. " +
			"mailparser refuses a header of more than 1 MiB, which about 33 800 addresses reach with crlf and 34 900 with lf, " +
			"and Python 3.14's email package takes minutes over tens of thousands of them.",
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
			Detail: "The format of the attached files. Run tfg formats to see what this build supports. " +
				"eml attaches a message as it is, the way mail is forwarded, with no base64 around it.",
			Group: attachmentsGroup,
		},
		{
			Name: AttachmentSize, Kind: format.PropertySize,
			Default: archive.DefaultSizeText,
			Detail: "How big each attached file is. The message grows by about a third more, " +
				"because attached files travel in base64.",
			Group: attachmentsGroup,
		},
		filenameStyle(),
		depthProperty(),
	}
}

// filenameStyle is how the names of attached files and pictures are written.
func filenameStyle() format.Property {
	return format.Property{
		Name: FilenameStyle, Kind: format.PropertyChoice,
		Choices: []string{Both, ContentType, NoName, RFC2047, RFC2231},
		Default: RFC2231,
		Detail: "How the names of attached files and pictures are written. rfc2231 is the standard way. " +
			"rfc2047 is the encoded form RFC 2047 forbids inside a name, and many readers decode it anyway. " +
			"both writes the two, and content-type puts the name only where old mail programs looked for it. " +
			"none writes no name at all, and a program that needs one has to make one up. " +
			"Go's standard library reads no name from content-type and the encoded text itself from rfc2047. " +
			"Unless headers is encoded a name is written as it is and only where it goes changes, " +
			"so rfc2047 and both write the same bytes.",
		Group: attachmentsGroup,
	}
}

// depthProperty is how many messages deep the message goes.
func depthProperty() format.Property {
	return format.Property{
		Name: MessageDepth, Kind: format.PropertyInt,
		Min: 1, Max: maxDepth,
		Default: "1",
		Detail: "How many messages deep the message goes. 1 is a message alone. 2 carries a forwarded message as an attachment, " +
			"and each step adds one more inside the last, the smallest message each can be. " +
			"Python 3.14's email package runs out of recursion at around 490, and MimeKit stops taking them apart past 513 without a word.",
		Group: attachmentsGroup,
	}
}
