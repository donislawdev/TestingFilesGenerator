package guard

import (
	"bytes"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/eml"
	"github.com/donislawdev/TestingFilesGenerator/internal/oracle"
)

// A newsletter, 2026-10-15 (docs/EML-3-2026-10-15.md). The structural checker
// reads the frame and the list header lines. What it cannot read is the
// manifest, so the one address to unsubscribe at - in the manifest, in
// List-Unsubscribe and in the footer of every text - is held here.

// emlUnsubscribe is the https address a newsletter unsubscribes at, its opaque
// part sixteen hexadecimal digits.
var emlUnsubscribe = regexp.MustCompile(`^https://example\.com/unsubscribe/([0-9a-f]{16})$`)

// TestANewsletterUnsubscribesAtTheOneAddressItsManifestGives is the claim of
// unsubscribe_url: the address the manifest gives is the one List-Unsubscribe
// offers and the one the footer of each text links to, its digits the
// Message-ID's - and a message laid out simple has no such key and no such
// line.
func TestANewsletterUnsubscribesAtTheOneAddressItsManifestGives(t *testing.T) {
	d, err := format.Get("eml")
	if err != nil {
		t.Fatal(err)
	}
	parts := map[string]int{eml.Plain: 1, eml.HTML: 1, eml.Alternative: 2}
	for body, texts := range parts {
		for _, layout := range []string{eml.Simple, eml.Newsletter} {
			props := map[string]string{eml.Layout: layout, eml.Body: body, eml.TextEncoding: eml.EightBit}
			b, p := writeEML(t, d, 6144, 7741, props)
			url, given := p.Properties[eml.UnsubscribeURL].(string)
			if layout == eml.Simple {
				if given || bytes.Contains(b, []byte("List-")) || bytes.Contains(b, []byte("unsubscribe")) {
					t.Errorf("%v: a message laid out simple names an address to unsubscribe at", props)
				}
				continue
			}
			m := emlUnsubscribe.FindStringSubmatch(url)
			if !given || m == nil {
				t.Fatalf("%v: the manifest gives %q as the address to unsubscribe at", props, url)
			}
			if !bytes.Contains(b, []byte("Message-ID: <"+m[1])) {
				t.Errorf("%v: the address to unsubscribe at ends in %s, which is not the Message-ID's digits", props, m[1])
			}
			// Second in List-Unsubscribe, after the mailto address, on a line
			// folded under it.
			if !bytes.Contains(b, []byte(">,\r\n <"+url+">\r\nList-Unsubscribe-Post: ")) {
				t.Errorf("%v: List-Unsubscribe does not offer %s", props, url)
			}
			if got, want := bytes.Count(b, []byte(url)), 1+texts; got != want {
				t.Errorf("%v: %s is in the message %d times, and the header and %d texts make %d", props, url, got, texts, want)
			}
		}
	}
}

// TestTheEMLCheckRefusesANewsletterBrokenOnPurpose is the canary for the
// checker's half: every message here is one the program wrote, broken one
// way, and each has to be refused with the verdict that names the break.
func TestTheEMLCheckRefusesANewsletterBrokenOnPurpose(t *testing.T) {
	d, err := format.Get("eml")
	if err != nil {
		t.Fatal(err)
	}
	newsletter := map[string]string{eml.Layout: eml.Newsletter, eml.Body: eml.Alternative, eml.TextEncoding: eml.EightBit}
	plainLetter := map[string]string{eml.Layout: eml.Newsletter, eml.TextEncoding: eml.EightBit}
	simple := map[string]string{eml.Body: eml.Alternative, eml.TextEncoding: eml.EightBit}
	listLines := regexp.MustCompile(`List-[^\r]*\r\n( [^\r]*\r\n)?`)
	// The last digit of an address one bit over: in the footer of the HTML,
	// or in List-Unsubscribe.
	another := func(b []byte) []byte {
		at := bytes.LastIndex(b, []byte(`">Unsubscribe</a>`))
		out := append([]byte{}, b...)
		out[at-1] ^= 1
		return out
	}
	anotherInTheHeader := func(b []byte) []byte {
		at := bytes.Index(b, []byte(">\r\nList-Unsubscribe-Post: "))
		out := append([]byte{}, b...)
		out[at-1] ^= 1
		return out
	}
	const fetches = "the HTML holds something a mail program fetches"
	cases := []struct {
		made, told map[string]string
		spoil      func([]byte) []byte
		verdict    string
	}{
		{newsletter, newsletter, func(b []byte) []byte {
			return bytes.Replace(b, []byte("List-Unsubscribe-Post: List-Unsubscribe=One-Click\r\n"), nil, 1)
		}, "the message opens with"},
		{newsletter, newsletter, another, "layout=newsletter and the HTML lacks: link to unsubscribe"},
		{newsletter, newsletter, anotherInTheHeader, "List-Unsubscribe is"},
		{newsletter, newsletter, func(b []byte) []byte {
			return bytes.Replace(b, []byte("<p>\r\n"), []byte("<img src=\"https://example.com/p.png\">\r\n<p>\r\n"), 1)
		}, "the HTML holds something a mail program fetches"},
		{newsletter, newsletter, func(b []byte) []byte {
			return bytes.Replace(b, []byte("</head>"), []byte("<link rel=\"stylesheet\" href=\"s.css\">\r\n</head>"), 1)
		}, "the HTML holds something a mail program fetches"},
		{plainLetter, plainLetter, func(b []byte) []byte {
			return bytes.Replace(b, []byte("Read more (https://example.com/newsletter)\r\n"), nil, 1)
		}, "layout=newsletter and the plain text opens"},
		{newsletter, simple, func(b []byte) []byte { return listLines.ReplaceAll(b, nil) },
			"layout=simple and the text/plain part is in the frame of a newsletter"},
		{simple, newsletter, func(b []byte) []byte { return b }, "the message opens with"},
		// Review of #186: an address in the middle of the plain text went with
		// every other word holding ://, and the HTML was asked only for src in
		// double quotes. Each of these passed before.
		{newsletter, newsletter, replaced("\r\n\r\nRead more (", "\r\nhttps://example.net/x\r\n\r\nRead more ("),
			"the two parts of alternative do not carry the same words"},
		{newsletter, newsletter, replaced("<p>\r\n", "<object data=\"https://example.net/o\"></object>\r\n<p>\r\n"), fetches},
		{newsletter, newsletter, replaced(`<tr><td style="padding: 0 24px;">`, `<tr><td background="https://example.net/b.png" style="padding: 0 24px;">`), fetches},
		{newsletter, newsletter, replaced("<p>\r\n", "<img src=\"cid:x\" srcset=\"https://example.net/s.png 2x\">\r\n<p>\r\n"), fetches},
		{newsletter, newsletter, replaced("<p>\r\n", "<video poster=\"https://example.net/p.png\"></video>\r\n<p>\r\n"), fetches},
		{newsletter, newsletter, replaced("</head>", "<meta http-equiv=\"refresh\" content=\"0; url=https://example.net/\">\r\n</head>"), fetches},
		{newsletter, newsletter, replaced("<p>\r\n", "<img src='https://example.net/q.png'>\r\n<p>\r\n"), fetches},
		// In the sheet rather than in an attribute, so only the reading of
		// the styles sees them. And addresses with nothing to say they are
		// remote, so only the rules for src and for link see them.
		{newsletter, newsletter, replaced("<style>\r\n", "<style>\r\n@import \"https://example.net/s.css\";\r\n"), fetches},
		{newsletter, newsletter, replaced("<style>\r\n", "<style>\r\np { background: url(b.png); }\r\n"), fetches},
		{newsletter, newsletter, replaced("<p>\r\n", "<img src=\"p.png\">\r\n<p>\r\n"), fetches},
		{newsletter, newsletter, replaced("<style>\r\nbody { margin: 0; background: #f2f2f2; }\r\n"+
			".frame { width: 600px; background: #ffffff; }\r\n"+
			".button { background: #1a73e8; color: #ffffff; text-decoration: none; }\r\n", "<style>\r\n"),
			"layout=newsletter and the HTML lacks: style sheet in its head"},
		// The page's language, which the frame writes since 2026-10-15 and
		// nothing read before (docs/HTML-6-2026-10-15.md).
		{newsletter, newsletter, replaced(`<html lang="en">`, "<html>"),
			"layout=newsletter and the HTML lacks: language of the page"},
	}
	dir := t.TempDir()
	for i, c := range cases {
		page, _ := writeEML(t, d, 6144, 7741, c.made)
		bad := c.spoil(page)
		if bytes.Equal(bad, page) && maps.Equal(c.made, c.told) {
			t.Fatalf("case %d: breaking the message changed nothing, so there is nothing to refuse", i)
		}
		path := filepath.Join(dir, strconv.Itoa(i)+".eml")
		if err := os.WriteFile(path, bad, 0o600); err != nil {
			t.Fatal(err)
		}
		res := oracle.Strict("eml", path, emlSettings(c.told)...)
		if !res.Available {
			t.Skip("the structural check needs python")
		}
		if res.Err == nil || !strings.HasPrefix(res.Err.Error(), c.verdict) {
			t.Errorf("case %d, made %v and told %v: a message broken on purpose was answered with %v, and %q was the verdict owed",
				i, c.made, c.told, res.Err, c.verdict)
		}
	}
}

// replaced is a way to break a message: the first old in it becomes new.
func replaced(old, new string) func([]byte) []byte {
	return func(b []byte) []byte { return bytes.Replace(b, []byte(old), []byte(new), 1) }
}
