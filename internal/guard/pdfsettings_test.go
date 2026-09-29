package guard

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/oracle"
)

// Every PDF setting is read back by a reader that is not ours.
//
// The reference tool guard opens one PDF at its default settings, and the
// reader it asks - pdftotext - sees the words on the page and nothing else. A
// title that never reached the file, a page that was meant to lie wide and
// stands upright, a date written in the wrong zone: every one of those leaves
// the text, the size and the determinism exactly as they were. Twelve settings
// arrived on 2026-09-29 and none of them was visible to any guard until this.
//
// pdfinfo answers for all of them: the eight fields of the information
// dictionary, the size and the turn of every page, the version in the header.
// It is poppler, where pdftotext here is Xpdf, so it is a second engine as well
// as a second question.
//
// The reader has to be able to say no, or its yes means nothing. So the file
// with no settings is asked about the fields nobody set, and a field it
// reports there is a failure - the same reader that reads an author back has
// to report none when there is none.
//
// Named for the reference tool so the CI job that installs poppler runs it.
// Without pdfinfo it skips, and says so.
func TestEveryPdfSettingSurvivesItsReferenceTool(t *testing.T) {
	if _, err := exec.LookPath("pdfinfo"); err != nil {
		t.Skip("pdfinfo is not installed, so no PDF setting was read back - install poppler to run this")
	}
	d, err := format.Get("pdf")
	if err != nil {
		t.Fatalf("pdf is not registered: %v", err)
	}

	cases := pdfWitnessCases()

	// Every setting the format declares is set by at least one case. A setting
	// added tomorrow with no case here would otherwise be one this guard never
	// asks about, and it would stay green.
	covered := map[string]bool{}
	for _, c := range cases {
		for k := range c.set {
			covered[k] = true
		}
	}
	for _, p := range d.Properties {
		if !covered[p.Name] {
			t.Errorf("pdf declares %s and no case here sets it, so nothing reads it back", p.Name)
		}
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := writePdf(t, d, c.set)
			info := readPdfinfo(t, path)
			for key, want := range c.want {
				got, ok := info[key]
				switch {
				case want == "" && ok:
					t.Errorf("pdfinfo reports %s %q in a file where nobody set it", key, got)
				case want != "" && !ok:
					t.Errorf("pdfinfo reports no %s, and the file was asked for %q", key, want)
				case want != "" && !strings.HasPrefix(got, want):
					t.Errorf("pdfinfo reads %s as %q, and the file was asked for %q", key, got, want)
				}
			}
			if res := oracle.Strict("pdf", path); res.Available && res.Err != nil {
				t.Errorf("the structural check refuses it: %v", res.Err)
			}
		})
	}
}

type pdfWitnessCase struct {
	name string
	set  map[string]string
	// want is what pdfinfo has to print for a key, compared as a prefix
	// because poppler shortens a zone of +02:00 to +02. An empty value is a
	// key it must NOT print.
	want map[string]string
}

func pdfWitnessCases() []pdfWitnessCase {
	return []pdfWitnessCase{
		{
			name: "nothing set",
			set:  map[string]string{},
			want: map[string]string{
				"Producer": "Testing Files Generator", "CreationDate": "2020-01-01T00:00:00",
				"Author": "", "Subject": "", "Keywords": "", "Creator": "", "ModDate": "",
				"PDF version": "1.7", "Pages": "1",
				"Page    1 size": "595 x 842", "Page    1 rot": "0",
			},
		},
		{
			name: "every field of the document",
			set: map[string]string{
				"title": "Zażółć (gęślą) jaźń", "author": "Jan Kowalski", "subject": "Invoices",
				"keywords": "a, b", "creator": "Microsoft Word", "producer": "Producer 1.0",
				"created": "2024-02-29T13:45:00+02:00", "modified": "1999-12-31T23:59:59Z",
			},
			want: map[string]string{
				"Title": "Zażółć (gęślą) jaźń", "Author": "Jan Kowalski", "Subject": "Invoices",
				"Keywords": "a, b", "Creator": "Microsoft Word", "Producer": "Producer 1.0",
				"CreationDate": "2024-02-29T13:45:00+02", "ModDate": "1999-12-31T23:59:59",
			},
		},
		{
			name: "no creation date",
			set:  map[string]string{"created": "none", "modified": "2024-02-29"},
			want: map[string]string{"CreationDate": "", "ModDate": "2024-02-29T00:00:00"},
		},
		{
			// Six pages walk the cycle once and start it again, with every
			// second page lying wide.
			name: "mixed sizes both ways up, turned, under the older header",
			set: map[string]string{
				"pages": "6", "page_size": "mixed", "orientation": "mixed",
				"rotate": "90", "pdf_version": "1.4",
			},
			want: pagesWant("1.4", "90",
				"595 x 842", "792 x 612", "612 x 1008", "1191 x 842", "420 x 595", "842 x 595"),
		},
		{
			name: "one size lying wide",
			set:  map[string]string{"pages": "2", "page_size": "a5", "orientation": "landscape", "rotate": "270"},
			want: pagesWant("1.7", "270", "595 x 420", "595 x 420"),
		},
	}
}

// pagesWant is the size and turn of each page, and the header version.
func pagesWant(version, rot string, sizes ...string) map[string]string {
	want := map[string]string{"PDF version": version, "Pages": fmt.Sprint(len(sizes))}
	for i, s := range sizes {
		want[fmt.Sprintf("Page %4d size", i+1)] = s
		want[fmt.Sprintf("Page %4d rot", i+1)] = rot
	}
	return want
}

func writePdf(t *testing.T, d format.Descriptor, props map[string]string) string {
	t.Helper()
	plan, err := d.Generator.Plan(format.Request{Bytes: 64 * 1024, Seed: 7741, Label: true, Properties: props})
	if err != nil {
		t.Fatalf("planning: %v", err)
	}
	path := filepath.Join(t.TempDir(), "sample.pdf")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating %s: %v", path, err)
	}
	err = d.Generator.Write(context.Background(), f, plan)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatalf("writing: %v", err)
	}
	return path
}

// pdfinfoLine is one "Key: value" line. The key may hold spaces - "Page    1
// size" - so it is everything up to the first colon followed by spaces.
var pdfinfoLine = regexp.MustCompile(`^([^:]+):\s+(.*)$`)

func readPdfinfo(t *testing.T, path string) map[string]string {
	t.Helper()
	cmd := exec.Command("pdfinfo", "-enc", "UTF-8", "-isodates", "-f", "1", "-l", "9999", path)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("pdfinfo refused the file: %v %s", err, stderr.String())
	}
	if strings.TrimSpace(stderr.String()) != "" {
		t.Errorf("pdfinfo complained: %s", stderr.String())
	}
	info := map[string]string{}
	for _, line := range strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n") {
		if m := pdfinfoLine.FindStringSubmatch(line); m != nil {
			info[m[1]] = strings.TrimSpace(m[2])
		}
	}
	// The reader answered in the shape this guard reads, or every comparison
	// above is against an empty map and a missing key reads as "not set".
	if info["Pages"] == "" || info["PDF version"] == "" {
		t.Fatalf("pdfinfo printed nothing this guard can read - the comparisons would all be against nothing:\n%s", out)
	}
	return info
}
