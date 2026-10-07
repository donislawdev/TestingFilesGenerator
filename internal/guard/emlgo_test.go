package guard

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/eml"
)

// Go's standard library is the fourth reader of a mail message, and the one
// every service written in Go meets without a mail library: net/mail for the
// message, mime/multipart for the parts, Part.FileName for a name. Measured
// 2026-10-07 (docs/EML-2026-10-07.md section 5.1), it is the strictest of the
// four on structure - the only one that refused a message cut short, one with
// no closing boundary and one with a stray character in base64 - and the only
// one that disagrees about names: it reads none from Content-Type, and it hands
// back an RFC 2047 name inside quotes as the encoded text itself. The owner's
// decision of the same day put it in the guards, where it costs nothing to run.
//
// It runs in this process, so it needs no tool installed and runs everywhere.
// net/mail is linked into the guards only - the command line links no network
// package at all (D16), and a guard checks that.

// goPart is one leaf of a message as the standard library reads it.
type goPart struct {
	mediaType string
	name      string
	attached  bool
	data      []byte
}

// goReadMail reads a message the way a Go service does and returns its leaves.
func goReadMail(b []byte) ([]goPart, *mail.Message, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(b))
	if err != nil {
		return nil, nil, fmt.Errorf("ReadMessage: %w", err)
	}
	var out []goPart
	err = goWalk(&out, msg.Header.Get("Content-Type"), msg.Header.Get("Content-Transfer-Encoding"), "", "", msg.Body)
	return out, msg, err
}

// goWalk reads one entity, going down into a multipart one - alternative
// inside mixed is the shape eml writes.
func goWalk(out *[]goPart, contentType, transfer, disposition, name string, body io.Reader) error {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return fmt.Errorf("ParseMediaType %q: %w", contentType, err)
	}
	if strings.HasPrefix(mediaType, "multipart/") {
		mr := multipart.NewReader(body, params["boundary"])
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return fmt.Errorf("NextPart: %w", err)
			}
			if err := goWalk(out, p.Header.Get("Content-Type"), p.Header.Get("Content-Transfer-Encoding"),
				p.Header.Get("Content-Disposition"), p.FileName(), p); err != nil {
				return err
			}
		}
	}
	// A part reading quoted-printable decodes it itself and drops the header,
	// so only base64 is left to undo here.
	if strings.EqualFold(transfer, "base64") {
		body = base64.NewDecoder(base64.StdEncoding, body)
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("reading a %s part: %w", mediaType, err)
	}
	*out = append(*out, goPart{mediaType: mediaType, name: name, attached: strings.HasPrefix(disposition, "attachment"), data: data})
	return nil
}

// goNameFor is the name the standard library is measured to read for an
// attached file - the one each setting writes, but for the two places it was
// measured to read something else.
func goNameFor(props map[string]string, written string) string {
	switch {
	case props[eml.FilenameStyle] == eml.ContentType:
		return ""
	case props[eml.FilenameStyle] == eml.RFC2047 && props[eml.Headers] == eml.Encoded:
		return "=?UTF-8?B?"
	}
	return written
}

// TestGosStandardLibraryReadsEveryEMLTheWayItWasMeasured reads a message of
// every setting with the standard library and asks for three things: every
// attached file comes back byte for byte the file its own format writes with
// the seed it was given (RC4), every name is the one the settings write - or
// exactly the other thing this reader was measured to read - and the text
// parts are the ones body orders.
//
// The name of each file is asked of a setting that makes it plain to read, so
// the guard asserts that the disagreement is still there rather than assuming
// it: a reader that started decoding rfc2047 would turn this red, and the
// sentence beside filename_style would then be wrong.
func TestGosStandardLibraryReadsEveryEMLTheWayItWasMeasured(t *testing.T) {
	d, err := format.Get("eml")
	if err != nil {
		t.Fatal(err)
	}
	disagreed := 0
	for i, props := range emlCases {
		props = withAttachments(props, "2", "pdf", "4kb")
		b, plan := writeEML(t, d, 40000+int64(i)*101, uint64(70+i), props)
		parts, _, err := goReadMail(b)
		if err != nil {
			t.Fatalf("%v: the standard library refused the message: %v", props, err)
		}
		var files []goPart
		var texts []string
		for _, p := range parts {
			if p.attached {
				files = append(files, p)
			} else {
				texts = append(texts, p.mediaType)
			}
		}
		if want := textsFor(props[eml.Body]); strings.Join(texts, " ") != want {
			t.Errorf("%v: the text parts are %v, and body orders %s", props, texts, want)
		}
		if len(files) != 2 {
			t.Fatalf("%v: %d attached files read, 2 ordered", props, len(files))
		}
		for n, f := range files {
			if want := childBytes(t, "pdf", 4<<10, core.FileSeed(uint64(70+i), n)); !bytes.Equal(f.data, want) {
				t.Errorf("%v: attached file %d is not the pdf its seed makes (%d B against %d B)", props, n+1, len(f.data), len(want))
			}
			written := emlName(props, "pdf", n+1, ".pdf")
			want := goNameFor(props, written)
			if (want == "=?UTF-8?B?" && strings.HasPrefix(f.name, want)) || f.name == want {
				disagreed += boolInt(want != written)
				continue
			}
			t.Errorf("%v: the standard library read the name %q, and %q was measured", props, f.name, want)
		}
		if got := plan.Properties["mime_entities"]; got != len(parts)+entitiesAbove(props) {
			t.Errorf("%v: the manifest counts %v MIME entities and the reader found %d leaves", props, got, len(parts))
		}
	}
	if disagreed == 0 {
		t.Error("no case reached a name the standard library reads otherwise, so the measured disagreement was not asked about")
	}
}

// textsFor is the text parts body orders, as their media types in order.
func textsFor(body string) string {
	switch body {
	case eml.HTML:
		return "text/html"
	case eml.Alternative:
		return "text/plain text/html"
	}
	return "text/plain"
}

// entitiesAbove is how many entities hold the leaves: the message itself as
// multipart/mixed, and multipart/alternative when body orders it.
func entitiesAbove(props map[string]string) int {
	if props[eml.Body] == eml.Alternative {
		return 2
	}
	return 1
}

// emlName is the name the generator gives an attached file, spelled here from
// the rule rather than read from the generator, so the two can disagree.
func emlName(props map[string]string, id string, n int, ext string) string {
	name := fmt.Sprintf("%s_%04d%s", id, n, ext)
	if props[eml.Headers] == eml.Encoded || props[eml.Headers] == eml.UTF8 {
		name = "Załącznik_日本語_" + name
	}
	return name
}

// childBytes is the file a format writes for a size and a seed, as the message
// is meant to carry it.
func childBytes(t *testing.T, id string, size int64, seed uint64) []byte {
	t.Helper()
	d, err := format.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Generator.Plan(format.Request{Bytes: size, Seed: seed, Label: true})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := d.Generator.Write(context.Background(), &buf, p); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
