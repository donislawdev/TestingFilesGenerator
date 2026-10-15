package eml

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// The people a message is between. Addresses only in the domains RFC 2606
// keeps for examples, so a message that leaves a test by mistake reaches
// nobody.
const (
	fromAddress = "alice@example.com"
	toAddress   = "bob@example.org"

	fromASCII = "Alice Example"
	toASCII   = "Bob Example"

	// The same people with names outside ASCII, for headers set to encoded or
	// utf8: Polish for one, Japanese for the other, so a reader that handles
	// two byte letters and not three byte ones is caught as well.
	fromWide = "Łukasz Żółć"
	toWide   = "山田 太郎"

	subjectASCII = "Test message"
	subjectWide  = "Zgłoszenie testowe 日本語"

	// wordBytes is how much text one encoded word carries. Thirty bytes of
	// UTF-8 are forty characters of base64 and fifty two with the markers, so
	// the first word fits beside "Subject: " under 78 characters and no word
	// comes near the 75 RFC 2047 allows.
	wordBytes = 30
)

// dateWindow is the year a message is dated in, from the start of 2026. The
// date comes from the seed rather than the clock, because the same recipe has
// to give the same bytes (D11).
const dateWindow = 365 * 24 * 60 * 60

var dateEpoch = time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC).Unix()

// identity is what makes one message this message, worked out from its seed.
//
// Every piece has a fixed length whatever the seed, so the size of a message
// does not depend on which seed it was made with - the minimum is one number
// for one set of settings.
type identity struct {
	// messageID is the Message-ID line's value, angle brackets and all,
	// because that is how a test finds the ticket the message became.
	messageID string
	date      string
	// subject is the subject as a person reads it, before any encoding.
	subject string
	// longMessageID is the same with one more digit, which a message in
	// base64 with crlf line endings carries when its size is odd - the one
	// byte nothing else in it can give (docs/EML-2-2026-10-13.md section 8).
	longMessageID string
	// boundary separates the parts of multipart/mixed, inner those of
	// multipart/alternative and related those of multipart/related. All carry
	// "=_", which neither base64 nor quoted-printable can produce, and they
	// differ in their last letter so that none is the start of another - RFC
	// 2046 lets a reader match a boundary at the start of a longer line.
	boundary, inner, related string
	// from, to and subjectLine are the header values as they are written,
	// encoded words and folding included. Worked out once here, because the
	// message is counted several times while it is planned and a header built
	// at every count is an allocation the resource guard counts too.
	from, to, subjectLine string
	// unsubscribe is the https address a newsletter unsubscribes at, the
	// message's digits its opaque part (docs/EML-3-2026-10-15.md).
	unsubscribe string
}

func identify(seed uint64, s settings) identity {
	hex := fmt.Sprintf("%016x", seed)
	subject, from, to := subjectASCII, fromASCII, toASCII
	if s.headers != ASCII {
		subject, from, to = subjectWide, fromWide, toWide
	}
	subject += " " + core.SeedLabel(seed)
	return identity{
		messageID:     "<" + hex + "@example.com>",
		longMessageID: "<" + hex + "0@example.com>",
		date:          time.Unix(dateEpoch+int64(seed%dateWindow), 0).UTC().Format("Mon, 02 Jan 2006 15:04:05 -0700"),
		subject:       subject,
		boundary:      "=_tfg_" + hex + "_m",
		inner:         "=_tfg_" + hex + "_a",
		related:       "=_tfg_" + hex + "_r",
		from:          phrase(s, from) + " <" + fromAddress + ">",
		to:            phrase(s, to) + " <" + toAddress + ">",
		subjectLine:   phrase(s, subject),
		unsubscribe:   unsubscribeWeb + hex,
	}
}

// head writes the lines every message opens with.
func (m *message) head(o *out) {
	eol := m.s.eol
	o.put("From: ", m.id.from, eol)
	o.put("To: ", m.id.to)
	m.moreRecipients(o)
	o.put(eol)
	o.put("Date: ", m.id.date, eol)
	o.put("Message-ID: ", m.messageID(), eol)
	o.put("Subject: ", m.id.subjectLine, eol)
	o.put(m.frame.headers)
	o.put("MIME-Version: 1.0", eol)
}

// messageID is the Message-ID line's value as this message writes it.
func (m *message) messageID() string {
	if m.longID {
		return m.id.longMessageID
	}
	return m.id.messageID
}

// recipientWidth is how long every address after the first is - six digits,
// enough for maxRecipients, so the size of a message does not depend on which
// addresses it holds.
const recipientWidth = len("recipient000002@example.org")

// moreRecipients writes the addresses after the first, each folded onto a line
// of its own. Counted rather than walked while the message is only being
// sized, and built in the scratch buffer while it is written, because a
// hundred thousand of them kept in the plan would be 3 MB per file in a run
// that holds every plan at once (engine/planmemory.go).
func (m *message) moreRecipients(o *out) {
	extra := int64(m.s.recipients - 1)
	if extra <= 0 {
		return
	}
	if o.w == nil || o.err != nil {
		o.n += extra * int64(2+len(m.s.eol)+recipientWidth)
		return
	}
	buf := o.scratch[:0]
	for i := 2; i <= m.s.recipients && !o.cancelled(); i++ {
		buf = append(append(buf, ','), m.s.eol...)
		buf = appendRecipient(append(buf, ' '), i)
		if len(buf) >= chunkSize {
			o.bytes(buf)
			buf = buf[:0]
		}
	}
	o.bytes(buf)
	o.scratch = buf
}

// appendRecipient appends the address of recipient n, its number in six
// digits.
func appendRecipient(buf []byte, n int) []byte {
	buf = append(buf, "recipient"...)
	for d := 100000; d >= 1; d /= 10 {
		buf = append(buf, byte('0'+n/d%10))
	}
	return append(buf, "@example.org"...)
}

// phrase is text for a header line, written the way headers says: as it is
// when it is ASCII or when utf8 asks for it raw, and as encoded words folded
// onto lines of their own when encoded asks for them.
func phrase(s settings, text string) string {
	if s.headers != Encoded || isASCII(text) {
		return text
	}
	return strings.Join(encodedWords(text), s.eol+" ")
}

// encodedWords is text as RFC 2047 B words, cut between characters and never
// inside one, so each word decodes on its own as the RFC requires.
//
// Written here rather than taken from mime.WordEncoder. Its way of cutting is
// the standard library's to change, and a cut moved by a newer Go would move
// the bytes of every message with a name outside ASCII (D11, O169).
func encodedWords(text string) []string {
	var out []string
	for start := 0; start < len(text); {
		end := wordEnd(text, start)
		out = append(out, "=?UTF-8?B?"+base64.StdEncoding.EncodeToString([]byte(text[start:end]))+"?=")
		start = end
	}
	return out
}

// wordEnd is where the encoded word starting at start ends: as many whole
// characters as fit in wordBytes.
func wordEnd(text string, start int) int {
	end := start
	for end < len(text) {
		_, size := utf8.DecodeRuneInString(text[end:])
		if end+size-start > wordBytes {
			break
		}
		end += size
	}
	return end
}

// isASCII says whether text needs nothing of headers to be written.
func isASCII(text string) bool {
	for i := 0; i < len(text); i++ {
		if text[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}
