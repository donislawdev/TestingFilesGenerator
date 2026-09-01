package guard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/oracle"
)

// csvDialects is every combination the registry offers, built from the
// declaration rather than written out. A list copied into a guard stops
// describing the thing it guards the moment somebody adds a value.
func csvDialects(t *testing.T) (delims, endings []string) {
	t.Helper()
	d, err := format.Get("csv")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range d.Properties {
		switch p.Name {
		case "delimiter":
			delims = p.Choices
		case "line_ending":
			endings = p.Choices
		}
	}
	if len(delims) == 0 || len(endings) == 0 {
		t.Fatalf("csv declares %d separators and %d row endings, so this guard would walk nothing",
			len(delims), len(endings))
	}
	return delims, endings
}

// writeCSV produces one file and hands back its bytes, failing loudly rather
// than returning an error nobody reads.
func writeCSV(t *testing.T, size int64, seed uint64, props map[string]string) []byte {
	t.Helper()
	d, err := format.Get("csv")
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Generator.Plan(format.Request{Bytes: size, Seed: seed, Label: true, Properties: props})
	if err != nil {
		t.Fatalf("planning %d B with %v: %v", size, props, err)
	}
	var buf bytes.Buffer
	if err := d.Generator.Write(context.Background(), &buf, p); err != nil {
		t.Fatalf("writing %d B with %v: %v", size, props, err)
	}
	if int64(buf.Len()) != size {
		t.Fatalf("%v: asked for %d B and got %d", props, size, buf.Len())
	}
	return buf.Bytes()
}

// csvFloor is the smallest file this dialect will take, asked of the format
// rather than worked out here. Repeating the arithmetic in the guard would only
// prove the guard agrees with itself.
func csvFloor(t *testing.T, props map[string]string) int64 {
	t.Helper()
	d, err := format.Get("csv")
	if err != nil {
		t.Fatal(err)
	}
	_, err = d.Generator.Plan(format.Request{Bytes: 1, Seed: 1, Label: true, Properties: props})
	var below *format.BelowMinimumError
	if !errors.As(err, &below) {
		t.Fatalf("%v took a one byte table, or refused it without saying what the floor is: %v", props, err)
	}
	return below.Minimum
}

// The dialect asked for is the dialect in the file, and it moves the floor.
//
// Three settings, sixteen combinations, and each of them is a file somebody is
// really handed: a European export separates with a semicolon, anything written
// on Windows ends its rows with CRLF, and a feed dumped out of a database has
// no header. All three parse as CSV and all three break a reader that assumed
// the other thing.
//
// What could be wrong here divides in two, and the halves fail differently.
//
// The setting could be stored and then ignored - the file comes out the right
// size, every row still parses, and the manifest still says what was asked for.
// So the FILE is asked which separator is in it, not the manifest.
//
// Or the arithmetic could miss that the dialect moves the floor. A CRLF row
// costs a byte more than an LF one and a header is a whole line, so a floor
// worked out for one dialect is wrong for the others - and wrong in the safe
// direction for CRLF, which no size or determinism guard would ever see. That
// is asked by taking the floor the format announces and the byte below it.
func TestTheCSVDialectIsInTheFileAndMovesTheFloor(t *testing.T) {
	delims, endings := csvDialects(t)
	seen := map[int64]bool{}

	for _, delim := range delims {
		for _, eol := range endings {
			for _, header := range []string{"true", "false"} {
				name := delim + "/" + eol + "/header=" + header
				t.Run(name, func(t *testing.T) {
					props := map[string]string{
						"delimiter": delim, "line_ending": eol, "header": header,
					}

					n := csvFloor(t, props)
					seen[n] = true
					d, err := format.Get("csv")
					if err != nil {
						t.Fatal(err)
					}
					if _, err := d.Generator.Plan(format.Request{Bytes: n, Seed: 1, Label: true,
						Properties: props}); err != nil {
						t.Errorf("announces %d B as its floor and then refuses it: %v", n, err)
					}
					if _, err := d.Generator.Plan(format.Request{Bytes: n - 1, Seed: 1, Label: true,
						Properties: props}); err == nil {
						t.Errorf("took %d B, one below the %d B it calls its floor", n-1, n)
					}

					// Exact to the byte, at seeds rather than at sizes somebody
					// picked - the closing row is stretched to reach the length
					// and its arithmetic is what the separator and the ending
					// both feed into.
					for seed := uint64(1); seed <= 6; seed++ {
						for _, size := range []int64{n, n + 1, 1000, 8192} {
							if size < n {
								continue
							}
							writeCSV(t, size, seed, props) // fails inside on a miss
						}
					}

					body := writeCSV(t, 8192, 9, props)
					wantEOL := map[string]string{"lf": "\n", "crlf": "\r\n"}[eol]
					if !bytes.HasSuffix(body, []byte(wantEOL)) {
						t.Errorf("the file does not end with %s, so the last row is unterminated", eol)
					}
					if eol == "lf" && bytes.Contains(body, []byte("\r")) {
						t.Errorf("an lf file carries a carriage return, so some readers will see a different table")
					}

					sep := map[string]string{
						"comma": ",", "semicolon": ";", "tab": "\t", "pipe": "|",
					}[delim]
					if sep == "" {
						t.Fatalf("the registry offers %q and this guard does not know what it separates with. "+
							"Add it rather than deleting this check - a value nobody described is a value nobody verified.", delim)
					}
					lines := strings.Split(strings.TrimSuffix(string(body), wantEOL), wantEOL)
					if len(lines) < 3 {
						t.Fatalf("only %d rows, too few to say anything", len(lines))
					}
					for i, line := range lines[:3] {
						if !strings.Contains(line, sep) {
							t.Errorf("row %d carries no %s, so the setting was stored and the writer ignored it: %s",
								i+1, delim, line)
						}
					}

					hasHeader := strings.HasPrefix(lines[0], "id"+sep)
					if hasHeader != (header == "true") {
						t.Errorf("header=%s and the first row is %q", header, lines[0])
					}

					// The quoted field carries the separator, which is the only
					// reason the column is quoted at all.
					//
					// This is the half a reader of the code would not think to
					// ask. The description is padding, and padding that dropped
					// commas whatever the dialect would leave a semicolon file
					// looking perfect - right size, right separators between the
					// fields, every row the same width - while quietly never
					// exercising the quoted path that the separator setting
					// exists to test. Nothing else here would see it.
					//
					// Asked by counting: a row has five separators between its
					// six fields, so any row carrying more than five has them
					// inside the quotes.
					padded := lines[len(lines)-1]
					if n := strings.Count(padded, sep); n <= 5 {
						t.Errorf("the closing row carries %d %s and six fields need five of them, "+
							"so nothing sits inside the quotes and this dialect never exercises quoting: %.120s",
							n, delim, padded)
					}
				})
			}
		}
	}

	// The floor is not one number wearing sixteen hats. Without this the whole
	// test above would still pass on a build that ignored the dialect when
	// working the floor out.
	if len(seen) < 4 {
		t.Errorf("sixteen dialects produced %d distinct floors. A header is a whole line and a CRLF row "+
			"is a byte longer than an LF one, so a floor that did not move was worked out for one dialect "+
			"and handed to the rest.", len(seen))
	}
}

// Every dialect is well formed, judged by the checker rather than by us.
//
// This is the fidelity half, and it is separate on purpose. The guard above
// asks whether the bytes are what was ordered. This asks whether they are a
// CSV at all, and it asks something written in another language against the
// specification - because the settings above changed the very characters that
// decide where a field ends.
//
// The last case is the one that makes the rest mean anything. The checker is
// TOLD the dialect rather than sniffing it, so it could be a rubber stamp that
// agrees with whatever it is handed. Feeding it a file of one dialect while
// telling it another has to come back refused, or every pass above is worth
// nothing.
func TestEveryCSVDialectIsWellFormed(t *testing.T) {
	delims, endings := csvDialects(t)
	dir := t.TempDir()

	check := func(t *testing.T, body []byte, name string, settings ...string) oracle.Result {
		t.Helper()
		path := filepath.Join(dir, name+".csv")
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatal(err)
		}
		return oracle.Strict("csv", path, settings...)
	}

	ran := 0
	for _, delim := range delims {
		for _, eol := range endings {
			for _, header := range []string{"true", "false"} {
				name := delim + "_" + eol + "_" + header
				t.Run(name, func(t *testing.T) {
					props := map[string]string{
						"delimiter": delim, "line_ending": eol, "header": header,
					}
					body := writeCSV(t, 8192, 9, props)
					res := check(t, body, name,
						"delimiter="+delim, "line_ending="+eol, "header="+header)
					if !res.Available {
						t.Skip("the structural check needs python")
					}
					ran++
					if res.Err != nil {
						t.Errorf("%s is not well formed: %v", name, res.Err)
					}
				})
			}
		}
	}

	if ran == 0 {
		t.Skip("the structural check never ran, so nothing here was judged")
	}
	if ran != len(delims)*len(endings)*2 {
		t.Errorf("%d of %d dialects reached the checker", ran, len(delims)*len(endings)*2)
	}

	// The checker is told, so it has to disagree when it is told wrong.
	body := writeCSV(t, 8192, 4, map[string]string{"delimiter": "comma", "line_ending": "lf"})
	for _, wrong := range []struct{ what, setting string }{
		{"a comma file called semicolon", "delimiter=semicolon"},
		{"a comma file called tab", "delimiter=tab"},
		{"an lf file called crlf", "line_ending=crlf"},
	} {
		res := check(t, body, "wrong_"+strings.ReplaceAll(wrong.setting, "=", "_"), wrong.setting)
		if !res.Available {
			t.Skip("the structural check needs python")
		}
		if res.Err == nil {
			t.Errorf("%s passed the checker, so being told the dialect made it a rubber stamp: %s",
				wrong.what, firstLineOf(res.Output))
		}
	}
}

// The manifest says which dialect the file is in, because that is the half of
// this tool a test suite reads rather than a person.
//
// The separator goes in as the CHARACTER while the recipe names it as a word,
// and that difference is deliberate: the recipe states an intention and the
// manifest records a fact, the same split the contract already draws between
// size and bytes. A script reading this field would break if it changed.
func TestTheCSVManifestRecordsTheDialectAsFacts(t *testing.T) {
	d, err := format.Get("csv")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		props map[string]string
		sep   string
		eol   string
		head  bool
	}{
		{map[string]string{}, ",", "lf", true},
		{map[string]string{"delimiter": "semicolon"}, ";", "lf", true},
		{map[string]string{"delimiter": "tab", "line_ending": "crlf"}, "\t", "crlf", true},
		{map[string]string{"delimiter": "pipe", "header": "false"}, "|", "lf", false},
	}
	for _, c := range cases {
		t.Run(fmt.Sprint(c.props), func(t *testing.T) {
			p, err := d.Generator.Plan(format.Request{Bytes: 8192, Seed: 1, Label: true, Properties: c.props})
			if err != nil {
				t.Fatal(err)
			}
			if got := p.Properties["separator"]; got != c.sep {
				t.Errorf("separator is %q, wanted the character %q", got, c.sep)
			}
			if got := p.Properties["line_ending"]; got != c.eol {
				t.Errorf("line_ending is %v, wanted %q", got, c.eol)
			}
			if got := p.Properties["header"]; got != c.head {
				t.Errorf("header is %v, wanted %v", got, c.head)
			}
			// And it stays JSON a script can read, rather than something that
			// only looks right in a Go printout.
			if _, err := json.Marshal(p.Properties); err != nil {
				t.Errorf("the properties do not survive being written out: %v", err)
			}
		})
	}
}

func firstLineOf(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
