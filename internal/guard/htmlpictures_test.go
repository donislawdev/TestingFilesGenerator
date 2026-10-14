package guard

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	pic "github.com/donislawdev/TestingFilesGenerator/internal/format/picture"
	"github.com/donislawdev/TestingFilesGenerator/internal/oracle"
)

// The pictures a page carries inside itself, from 2026-10-14
// (docs/HTML-3-2026-10-14.md). Read from the bytes, for the reason
// htmlsettings_test.go gives.

// htmlPictureLine is one picture's line as a page carries it, the base64 on the
// one line the Nu HTML Checker allows (a line break inside src is an error
// there, measured on 2026-10-14).
var htmlPictureLine = regexp.MustCompile(`<p><img alt="inline_([0-9]{4,5})(\.[a-z]+)" src="data:([a-z0-9.+/-]+);base64,([A-Za-z0-9+/=]*)"></p>\n`)

// TestAPictureInAPageIsTheFileItsFormatWritesAlone is the claim of inline.
//
// For every picture format of this build, from the registry rather than from a
// list here: the page carries as many pictures as were ordered, each named and
// typed as its format, each on one line, and each the very bytes the format
// writes on its own with the seed of its place - so a picture is a real file a
// tester can cut out of the page, not something that only looks like one.
func TestAPictureInAPageIsTheFileItsFormatWritesAlone(t *testing.T) {
	formats := pic.Formats()
	if len(formats) == 0 {
		t.Fatal("this build has no picture format, so there is nothing a page could carry")
	}
	for _, id := range formats {
		desc, err := format.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		props := map[string]string{"inline": "2", "inline_format": id, "inline_size": "2kb"}
		body := writeEncoded(t, "html", 65536, props)
		found := htmlPictureLine.FindAllSubmatch(body, -1)
		if len(found) != 2 {
			t.Errorf("%s: inline=2 was ordered and the page carries %d pictures on lines of their own", id, len(found))
			continue
		}
		if n := bytes.Count(body, []byte(`<img alt="inline_`)); n != len(found) {
			t.Errorf("%s: %d pictures in the page and %d of them on one line in the shape a page carries them", id, n, len(found))
		}
		for i, m := range found {
			if want := fmt.Sprintf("%04d", i+1); string(m[1]) != want || string(m[2]) != desc.Extension || string(m[3]) != desc.MediaType {
				t.Errorf("%s: picture %d is inline_%s%s in %s", id, i+1, m[1], m[2], m[3])
			}
			got, err := base64.StdEncoding.DecodeString(string(m[4]))
			if err != nil {
				t.Errorf("%s: picture %d is not base64: %v", id, i+1, err)
				continue
			}
			if alone := htmlPictureAlone(t, desc, core.FileSeed(7741, i)); !bytes.Equal(got, alone) {
				t.Errorf("%s: picture %d is %d B and not the file the format writes on its own with the seed of its place", id, i+1, len(got))
			}
		}
	}
}

// htmlPictureAlone is the file a picture format writes on its own, the way a
// page asks for it.
func htmlPictureAlone(t *testing.T, desc format.Descriptor, seed uint64) []byte {
	t.Helper()
	p, err := desc.Generator.Plan(format.Request{Bytes: 2048, Seed: seed, Label: true})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := desc.Generator.Write(context.Background(), &buf, p); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestAPageRefusesPicturesThatAreNotPictures holds the refusal, with no
// pictures ordered as well: a value the setting cannot take is a mistake either
// way, and accepting it in silence would hand back a page the recipe did not
// describe. The remedy names the formats it does take.
func TestAPageRefusesPicturesThatAreNotPictures(t *testing.T) {
	d, err := format.Get("html")
	if err != nil {
		t.Fatal(err)
	}
	for _, count := range []string{"0", "1"} {
		_, err := d.Generator.Plan(format.Request{Bytes: 65536, Seed: 7741,
			Properties: map[string]string{"inline": count, "inline_format": "pdf"}})
		var bad *format.PropertyValueError
		if !errors.As(err, &bad) || bad.Key != "inline_format" || !strings.Contains(bad.Remedy.String(), "png") {
			t.Errorf("inline=%s with inline_format=pdf was answered with %v, not a refusal naming the picture formats", count, err)
		}
	}
}

// TestAPageOfPicturesTooLargeToCountIsRefused holds the ceiling the sums are
// counted under. Ten thousand pictures of a format with no ceiling of its own
// add up past it, and a page planned past it would be counted in an int64 that
// wrapped round - a size that is a lie rather than a refusal.
func TestAPageOfPicturesTooLargeToCountIsRefused(t *testing.T) {
	d, err := format.Get("html")
	if err != nil {
		t.Fatal(err)
	}
	_, err = d.Generator.Plan(format.Request{Bytes: 65536, Seed: 7741,
		Properties: map[string]string{"inline": "10000", "inline_format": "svg", "inline_size": "100000gb"}})
	var above *format.AboveMaximumError
	if !errors.As(err, &above) || above.Maximum != int64(1)<<60 {
		t.Errorf("ten thousand pictures of 100000 GB each were answered with %v, not a refusal at the ceiling", err)
	}
}

// TestEveryFormatWithPicturesDeclaresThemAlike holds the owner's decision of
// 2026-10-14: the picture settings are one set of names with one meaning
// wherever a format carries pictures, so a test reads them the same way from a
// message and from a page. The group and the sentences may differ, since they
// say what the pictures are in that format.
func TestEveryFormatWithPicturesDeclaresThemAlike(t *testing.T) {
	want := map[string]format.Property{}
	for _, p := range pic.Properties("", pic.Details{}) {
		want[p.Name] = p
	}
	carriers := 0
	for _, d := range format.All() {
		found := 0
		for _, p := range d.Properties {
			w, ok := want[p.Name]
			if !ok {
				continue
			}
			found++
			if p.Kind != w.Kind || p.Min != w.Min || p.Max != w.Max || p.Default != w.Default || p.Shape != w.Shape {
				t.Errorf("%s declares %s other than the picture settings do", d.ID, p.Name)
			}
		}
		if found != 0 && found != len(want) {
			t.Errorf("%s declares %d of the %d picture settings", d.ID, found, len(want))
		}
		if found != 0 {
			carriers++
		}
	}
	if carriers < 2 {
		t.Errorf("%d formats carry pictures, and eml and html both do", carriers)
	}
}

// TestTheStructuralCheckGivesAVerdictOnABrokenPicture holds the review of #178:
// a picture the size it should be whose bytes stop a format's own check part
// way - a PNG whose last chunk runs to the very end, so its checksum is not
// there to unpack - is a verdict about the picture, not a traceback that
// reads as the checker saying nothing useful.
func TestTheStructuralCheckGivesAVerdictOnABrokenPicture(t *testing.T) {
	props := map[string]string{"inline": "1", "inline_size": "4kb"}
	body := writeEncoded(t, "html", 65536, props)
	m := htmlPictureLine.FindSubmatchIndex(body)
	if m == nil {
		t.Fatal("the page carries no picture line, so there is nothing to break")
	}
	broken := append([]byte("\x89PNG\r\n\x1a\n"), 0, 0, 0x0f, 0xf0)
	broken = append(broken, "IHDR"...)
	broken = append(broken, make([]byte, 4096-len(broken))...)
	page := append(append(append([]byte{}, body[:m[8]]...), base64.StdEncoding.EncodeToString(broken)...), body[m[9]:]...)
	path := filepath.Join(t.TempDir(), "broken.html")
	if err := os.WriteFile(path, page, 0o600); err != nil {
		t.Fatal(err)
	}
	res := oracle.Strict("html", path, htmlSettingWords(props)...)
	if !res.Available {
		t.Skip("the structural check needs python")
	}
	if res.Err == nil || !strings.HasPrefix(res.Err.Error(), "picture 1 is not a good png file") {
		t.Errorf("a page with a broken picture was answered with %v, not a verdict on the picture", res.Err)
	}
}
