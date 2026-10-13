package guard

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/eml"
)

// The parts of the second set of mail settings (docs/EML-2-2026-10-13.md),
// each read back with Go's standard library, which runs everywhere: a message
// inside a message, messages forwarded inside each other, pictures the text
// shows, and many recipients.

// A message attached to a message goes in as it is - message/rfc822 with no
// Content-Transfer-Encoding, never base64 (RFC 2046 section 5.2.1) - and it is
// byte for byte the message this format writes for its size and its seed, with
// the line endings of the one around it. A message of two kinds of line ending
// would be a broken file rather than a test of either.
func TestAMessageCarriesAMessageAsItIs(t *testing.T) {
	d, err := format.Get("eml")
	if err != nil {
		t.Fatal(err)
	}
	props := withAttachments(map[string]string{eml.LineEndings: eml.LF}, "2", "eml", "1kb")
	b, plan := writeEML(t, d, 20000, 41, props)
	messages := partsOfKind(t, b, "message")
	if len(messages) != 2 {
		t.Fatalf("%d messages inside read, 2 attached", len(messages))
	}
	for n, m := range messages {
		if m.transfer != "" {
			t.Errorf("message %d inside says Content-Transfer-Encoding %q, and RFC 2046 keeps it as it is", n+1, m.transfer)
		}
		want := childBytesWith(t, "eml", 1<<10, core.FileSeed(41, n), map[string]string{eml.LineEndings: eml.LF})
		if !bytes.Equal(m.data, want) {
			t.Errorf("message %d inside is not the message its seed makes (%d B against %d B)", n+1, len(m.data), len(want))
		}
	}
	if plan.Properties[eml.Attachments] != 2 || plan.Properties[eml.MessageDepth] != 1 {
		t.Errorf("the manifest says attachments=%v and depth=%v", plan.Properties[eml.Attachments], plan.Properties[eml.MessageDepth])
	}
}

// depth forwards a message inside a message, the smallest each can be, down
// to the depth asked for: every message inside holds one more and nothing
// else, and its label states its own size - which is what makes it the
// smallest. The manifest gives the depth and counts no attachment for it.
func TestDepthForwardsTheSmallestMessagesInsideEachOther(t *testing.T) {
	d, err := format.Get("eml")
	if err != nil {
		t.Fatal(err)
	}
	b, plan := writeEML(t, d, 30000, 43, map[string]string{eml.MessageDepth: "4"})
	levels := 1
	for data := b; ; levels++ {
		inner := partsOfKind(t, data, "message")
		if len(inner) == 0 {
			break
		}
		if len(inner) != 1 {
			t.Fatalf("level %d holds %d messages, and depth forwards one", levels, len(inner))
		}
		data = inner[0].data
		if !bytes.Contains(data, []byte(fmt.Sprintf(" - %d B - ", len(data)))) {
			t.Errorf("the message %d deep is %d B and its label says otherwise", levels+1, len(data))
		}
	}
	if levels != 4 || plan.Properties[eml.MessageDepth] != 4 || plan.Properties[eml.Attachments] != 0 {
		t.Errorf("the messages go %d deep, and the manifest says depth=%v and attachments=%v",
			levels, plan.Properties[eml.MessageDepth], plan.Properties[eml.Attachments])
	}
}

// Every picture is the file its format writes for its size and the seed of its
// place after the attached files (RC4), so asking for pictures moves no
// attached file. Beside an HTML each one has a Content-ID and the HTML points
// at it through cid:. Beside a plain text nothing points at it, and it has
// none - the shape measured on 2026-10-13.
func TestPicturesAreTheFilesTheirSeedsMakeAndTheHTMLShowsEach(t *testing.T) {
	d, err := format.Get("eml")
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{eml.HTML, eml.Alternative, eml.Plain} {
		props := withAttachments(map[string]string{eml.Body: body, eml.Inline: "3", eml.InlineSize: "1kb", eml.InlineFormat: "gif"}, "1", "txt", "1kb")
		b, plan := writeEML(t, d, 40000, 47, props)
		if got := partsOfKind(t, b, "file"); len(got) != 1 || !bytes.Equal(got[0].data, childBytes(t, "txt", 1<<10, core.FileSeed(47, 0))) {
			t.Errorf("body=%s: the attached file is not the one its seed made before pictures existed", body)
		}
		html := ""
		for _, p := range partsOfKind(t, b, "text") {
			if p.mediaType == "text/html" {
				html = string(p.data)
			}
		}
		pics := partsOfKind(t, b, "picture")
		if len(pics) != 3 || plan.Properties[eml.Inline] != 3 {
			t.Fatalf("body=%s: %d pictures read and the manifest says %v, 3 ordered", body, len(pics), plan.Properties[eml.Inline])
		}
		for i, p := range pics {
			if !bytes.Equal(p.data, childBytes(t, "gif", 1<<10, core.FileSeed(47, 1+i))) {
				t.Errorf("body=%s: picture %d is not the gif its seed makes", body, i+1)
			}
			cid := strings.Trim(p.cid, "<>")
			if (body == eml.Plain) != (cid == "") || (cid != "" && !strings.Contains(html, `src="cid:`+cid+`"`)) {
				t.Errorf("body=%s: picture %d has the Content-ID %q and the HTML does not point at it the way body says", body, i+1, p.cid)
			}
		}
	}
}

// To carries every recipient asked for, each on a line of its own - the first
// by name, the rest numbered - and the standard library reads them all.
func TestToCarriesEveryRecipientOnALineOfItsOwn(t *testing.T) {
	d, err := format.Get("eml")
	if err != nil {
		t.Fatal(err)
	}
	b, plan := writeEML(t, d, 60000, 53, map[string]string{eml.Recipients: "1000"})
	_, msg, err := goReadMail(b)
	if err != nil {
		t.Fatal(err)
	}
	to, err := mail.ParseAddressList(msg.Header.Get("To"))
	if err != nil || len(to) != 1000 || to[0].Address != "bob@example.org" || to[999].Address != "recipient001000@example.org" {
		t.Fatalf("To reads as %d addresses (%v)", len(to), err)
	}
	if lines := bytes.Count(b, []byte("\r\n recipient")); lines != 999 || plan.Properties[eml.Recipients] != 1000 {
		t.Errorf("%d recipients on lines of their own, 999 after the first, and the manifest says %v", lines, plan.Properties[eml.Recipients])
	}
}

// A picture in the text has to be a picture, refused under the setting's own
// name with the formats that are - whether or not any picture was asked for,
// because a value the setting cannot take is a mistake either way.
func TestAPictureInTheTextHasToBeAPicture(t *testing.T) {
	d, err := format.Get("eml")
	if err != nil {
		t.Fatal(err)
	}
	for _, count := range []string{"1", "0"} {
		_, err := d.Generator.Plan(format.Request{Bytes: 1 << 20, Seed: 1,
			Properties: map[string]string{eml.Inline: count, eml.InlineFormat: "pdf"}})
		var bad *format.PropertyValueError
		if !errors.As(err, &bad) || bad.Key != eml.InlineFormat || !strings.Contains(bad.Remedy.String(), "png") {
			t.Errorf("inline=%s with inline_format=pdf was not refused under inline_format with the picture formats: %v", count, err)
		}
	}
}

// partsOfKind reads a message with the standard library and keeps the leaves
// of one kind.
func partsOfKind(t *testing.T, b []byte, kind string) []goPart {
	t.Helper()
	parts, _, err := goReadMail(b)
	if err != nil {
		t.Fatalf("the standard library refused the message: %v", err)
	}
	var out []goPart
	for _, p := range parts {
		if p.kind() == kind {
			out = append(out, p)
		}
	}
	return out
}

// childBytesWith is childBytes for a file asked for with settings.
func childBytesWith(t *testing.T, id string, size int64, seed uint64, props map[string]string) []byte {
	t.Helper()
	d, err := format.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Generator.Plan(format.Request{Bytes: size, Seed: seed, Label: true, Properties: props})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := d.Generator.Write(context.Background(), &buf, p); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
