package eml

import "strings"

// A newsletter is the text in the frame a mailing list sends it in, and the
// header lines by which a mail system tells bulk mail apart (owner's decision
// of 2026-10-14, docs/HTML-5-2026-10-14.md section 8a, and the second pass in
// docs/EML-3-2026-10-15.md).
//
// The frame is fixed text, so the size of a message still does not depend on
// its seed and the words still bring it to the byte. Every address is on a
// domain RFC 2606 keeps for examples, and nothing in the frame is fetched when
// the message is opened - no remote picture, no style sheet by address - so
// opening one reaches nobody (D16).

// UnsubscribeURL is the key the manifest gives a newsletter's https address to
// unsubscribe at - a public name under rule 10, and only a newsletter has one.
const UnsubscribeURL = "unsubscribe_url"

const (
	// newsletterName is the name of the list, in its title, in its footer and
	// in List-Id.
	newsletterName = "Example Newsletter"
	// listID is the List-Id line's value (RFC 2919): the name, then the list
	// in a domain kept for examples.
	listID = newsletterName + " <newsletter.example.com>"
	// unsubscribeMail is the address List-Unsubscribe offers first. RFC 2369
	// asks for a mailto address, RFC 8058 for an https one beside it, and its
	// own example puts them in this order.
	unsubscribeMail = "mailto:unsubscribe@example.com?subject=unsubscribe"
	// unsubscribeWeb is the https address before the message's own sixteen
	// hexadecimal digits - the opaque part RFC 8058 asks for, the same length
	// whatever the seed.
	unsubscribeWeb = "https://example.com/unsubscribe/"
	readMoreURL    = "https://example.com/newsletter"
)

// frame is what stands around the words of the text, each piece in the
// encoding the text is written in, worked out once when the message is
// planned rather than at every count. A message laid out simple has a frame
// too - the pieces it always wrote - so the parts are written one way.
type frame struct {
	// headers are the header lines after Subject, in the file's line endings.
	headers string
	// plainOpen and plainClose stand before and after the plain text.
	plainOpen, plainClose string
	// htmlBefore stands before <html>, htmlOpen after it and htmlClose after
	// the paragraph of words.
	htmlBefore, htmlOpen, htmlClose string
}

// simpleFrame is the frame of a message laid out simple: the body element and
// nothing else, the bytes this format has always written.
func simpleFrame(s settings) frame {
	return frame{htmlOpen: "<body>" + s.textEOL(), htmlClose: "</body></html>"}
}

// newsletterFrame is the frame of a newsletter, unsubscribe being the https
// address List-Unsubscribe offers.
func newsletterFrame(s settings, unsubscribe string) frame {
	return frame{
		headers: "List-Id: " + listID + s.eol +
			"List-Unsubscribe: <" + unsubscribeMail + ">," + s.eol + " <" + unsubscribe + ">" + s.eol +
			"List-Unsubscribe-Post: List-Unsubscribe=One-Click" + s.eol,
		plainOpen: frameLines(s, newsletterName, ""),
		plainClose: s.textEOL() + frameLines(s, "", "Read more ("+readMoreURL+")", "",
			"You receive this because you subscribed to "+newsletterName+".") +
			frameLine(s, "Unsubscribe ("+unsubscribe+")"),
		htmlBefore: frameLines(s, "<!DOCTYPE html>"),
		htmlOpen:   frameLines(s, newsletterHead...),
		htmlClose: frameLines(s, newsletterFoot(unsubscribe)...) +
			frameLine(s, "</html>"),
	}
}

// newsletterHead is the frame's lines from the head of the page to the cell
// the text sits in. Its styles are in the sheet and in style attributes as
// well, because a mail program that drops the sheet keeps what an attribute
// says.
var newsletterHead = []string{
	"<head>",
	"<title>" + newsletterName + "</title>",
	"<style>",
	"body { margin: 0; background: #f2f2f2; }",
	".frame { width: 600px; background: #ffffff; }",
	".button { background: #1a73e8; color: #ffffff; text-decoration: none; }",
	"</style>",
	"</head>",
	`<body style="margin: 0; background: #f2f2f2;">`,
	`<table role="presentation" width="100%" cellpadding="0" cellspacing="0"`,
	` style="background: #f2f2f2;">`,
	`<tr><td align="center">`,
	`<table role="presentation" class="frame" width="600" cellpadding="0"`,
	` cellspacing="0" style="width: 600px; background: #ffffff;">`,
	`<tr><td style="padding: 24px; font-size: 24px;">` + newsletterName + `</td></tr>`,
	`<tr><td style="padding: 0 24px;">`,
}

// newsletterFoot is the frame's lines after the cell the text sits in: the
// button, the footer with the unsubscribe link, and the page closed but for
// the last tag.
func newsletterFoot(unsubscribe string) []string {
	return []string{
		"</td></tr>",
		`<tr><td style="padding: 24px;">`,
		`<a class="button" href="` + readMoreURL + `"`,
		` style="background: #1a73e8; color: #ffffff; padding: 12px 24px;">`,
		"Read more</a>",
		"</td></tr>",
		`<tr><td style="padding: 24px; font-size: 12px; color: #666666;">`,
		"You receive this because you subscribed to " + newsletterName + ".",
		`<a href="` + unsubscribe + `">Unsubscribe</a>`,
		"</td></tr>",
		"</table>",
		"</td></tr>",
		"</table>",
		"</body>",
	}
}

// frameLines is lines of the frame, each ending the way the text's lines do.
func frameLines(s settings, lines ...string) string {
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(frameLine(s, l))
		b.WriteString(s.textEOL())
	}
	return b.String()
}

// frameLine is one line of the frame in the encoding of the text, with no
// line ending: in quoted-printable a line on its own, so an = in an attribute
// becomes =3D and a long line breaks softly, the way a real newsletter in
// quoted-printable reads.
func frameLine(s settings, line string) string {
	if s.encoding == QuotedPrintable {
		return quotedPrintable(line, s.eol)
	}
	return line
}

// newFrame is the frame the layout asks for.
func newFrame(s settings, id identity) frame {
	if s.layout == Newsletter {
		return newsletterFrame(s, id.unsubscribe)
	}
	return simpleFrame(s)
}
