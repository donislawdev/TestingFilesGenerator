package guard

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/oracle"
)

// TestTheStructuralCheckRefusesAPageWhoseTitleSaysNothing is the canary for the
// checker's half of O277.
//
// What it defends. Until 2026-10-15 every page no label filled - the smallest,
// and every one under --clean - carried <title></title>, which the Nu HTML
// Checker calls an error, and nothing here noticed, because the structural
// check never looked at the title. It looks now, and the program writes Page
// there. A checker that stopped looking would stay green on every page the
// program writes, so each page here is one the program wrote with its
// placeholder, broken one way, and each has to be refused with the verdict that
// names the title.
func TestTheStructuralCheckRefusesAPageWhoseTitleSaysNothing(t *testing.T) {
	const verdict = "the document has no title with text in it"
	d, err := format.Get("html")
	if err != nil {
		t.Fatal(err)
	}
	placeholder := []byte("<title>Page</title>")
	spoils := []struct {
		name string
		with []byte
	}{
		{"an empty title", []byte("<title></title>")},
		{"a title of white space", []byte("<title> </title>")},
		{"no title at all", nil},
	}

	dir := t.TempDir()
	for _, layout := range []string{"lines", "minified", "indented"} {
		props := map[string]string{"formatting": layout}
		smallest := d.SmallestAccepted(format.Request{Seed: 7741, Label: true, Properties: props})
		pages := map[string][]byte{
			"the smallest page":    writeEncoded(t, "html", smallest, props),
			"a page under --clean": unlabelledPage(t, d, 4096, props),
		}
		for which, page := range pages {
			if bytes.Count(page, placeholder) != 1 {
				t.Fatalf("%s, %s: the page does not carry %s once, so there is no placeholder to break", layout, which, placeholder)
			}
			for i, s := range spoils {
				bad := bytes.Replace(page, placeholder, s.with, 1)
				path := filepath.Join(dir, layout+strconv.Itoa(len(which))+strconv.Itoa(i)+".html")
				if err := os.WriteFile(path, bad, 0o600); err != nil {
					t.Fatal(err)
				}
				res := oracle.Strict("html", path, htmlSettingWords(props)...)
				if !res.Available {
					t.Skip("the structural check needs python")
				}
				if res.Err == nil || !strings.HasPrefix(res.Err.Error(), verdict) {
					t.Errorf("%s, %s with %s: answered %v, and %q was the verdict owed", layout, which, s.name, res.Err, verdict)
				}
			}
			// And the page as written is accepted, so the refusals above are
			// about the title and not about something else in the page.
			path := filepath.Join(dir, layout+strconv.Itoa(len(which))+"ok.html")
			if err := os.WriteFile(path, page, 0o600); err != nil {
				t.Fatal(err)
			}
			if res := oracle.Strict("html", path, htmlSettingWords(props)...); res.Err != nil {
				t.Errorf("%s, %s as the program wrote it: refused with %v", layout, which, res.Err)
			}
		}
	}
}

// unlabelledPage is a page written the way --clean writes it, with no label.
func unlabelledPage(t *testing.T, d format.Descriptor, size int64, props map[string]string) []byte {
	t.Helper()
	p, err := d.Generator.Plan(format.Request{Bytes: size, Seed: 7741, Label: false, Properties: props})
	if err != nil {
		t.Fatalf("planning %d B with no label and %v: %v", size, props, err)
	}
	var buf bytes.Buffer
	if err := d.Generator.Write(context.Background(), &buf, p); err != nil {
		t.Fatalf("writing %d B with no label and %v: %v", size, props, err)
	}
	return buf.Bytes()
}
