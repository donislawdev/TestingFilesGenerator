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

// csvChoices is the closed set the registry offers for one setting, built from
// the declaration rather than written out. A list copied into a guard stops
// describing the thing it guards the moment somebody adds a value.
func csvChoices(t *testing.T, name string) []string {
	t.Helper()
	d, err := format.Get("csv")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range d.Properties {
		if p.Name != name {
			continue
		}
		if len(p.Choices) == 0 {
			t.Fatalf("csv declares %s with no values at all, so this guard would walk nothing", name)
		}
		return p.Choices
	}
	t.Fatalf("csv declares no setting called %s, so this guard would walk nothing", name)
	return nil
}

// csvDialects is every axis this file walks. Header is not among them because
// it has two values and no declaration to read them from - it is a bool.
func csvDialects(t *testing.T) (delims, endings, styles []string) {
	t.Helper()
	return csvChoices(t, "delimiter"), csvChoices(t, "line_ending"), csvChoices(t, "quote_style")
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
// Four settings, forty eight combinations, and each of them is a file somebody
// is really handed: a European export separates with a semicolon, anything
// written on Windows ends its rows with CRLF, a feed dumped out of a database
// has no header, and a spreadsheet may quote every field or none. All of them
// parse as CSV and all of them break a reader that assumed the other thing.
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
	delims, endings, styles := csvDialects(t)
	seen := map[int64]bool{}

	for _, delim := range delims {
		for _, eol := range endings {
			for _, header := range []string{"true", "false"} {
				for _, style := range styles {
					name := delim + "/" + eol + "/header=" + header + "/" + style
					t.Run(name, func(t *testing.T) {
						csvDialectCase(t, seen, delim, eol, header, style)
					})
				}
			}
		}
	}

	// The floor is not one number wearing forty eight hats, and this is the
	// only thing here that can say so.
	//
	// A floor worked out wrong is invisible everywhere else, and that is worth
	// spelling out because it is not obvious. Shortest is the worst DRAW - a
	// nineteen digit row number and the longest word twice - so a real closing
	// row is some forty bytes under it. A floor that is one or twelve bytes
	// too low still produces files of exactly the right size at every seed and
	// passes every other check on this page.
	//
	// Eight is today's measurement, not a looser number: twelve combinations of
	// the three settings that move the floor, and minimal and none share their
	// four, because the row a floor is made of has an empty description and an
	// empty field carries no separator to need a quote for. Measured with the
	// binary on 2026-09-03 - 115, 117, 74, 75, 139, 141, 86, 87. It was written
	// as six first, which is the count that would survive a build where CRLF
	// stopped costing its second byte, and two mutations walked straight
	// through it.
	if len(seen) < 8 {
		t.Errorf("forty eight dialects produced %d distinct floors and the axes make 8. A header is a whole "+
			"line, a CRLF row is a byte longer than an LF one and quoting every field costs two bytes a "+
			"column, so a floor that did not move was worked out for one dialect and handed to the rest.",
			len(seen))
	}
}

// csvDialectCase is one combination. It lives outside the loops above so the
// body is readable at the depth the code shape guard asks for.
func csvDialectCase(t *testing.T, seen map[int64]bool, delim, eol, header, style string) {
	t.Helper()
	props := map[string]string{
		"delimiter": delim, "line_ending": eol, "header": header, "quote_style": style,
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

	// The header names the first column, and under quote_style
	// all it does so in quotes - because the header row is made
	// of fields like any other, and a writer told to quote
	// everything quotes those too.
	firstColumn := "id" + sep
	if style == "all" {
		firstColumn = `"id"` + sep
	}
	hasHeader := strings.HasPrefix(lines[0], firstColumn)
	if hasHeader != (header == "true") {
		t.Errorf("header=%s under quote_style %s and the first row is %q", header, style, lines[0])
	}

	// No dialect leaks another dialect's separator into the file.
	//
	// This is the half a reader of the code would not think to ask. The
	// description is padding, and padding that dropped commas whatever the
	// dialect was asked for would leave a semicolon file looking perfect -
	// right size, right separators between the fields, every row the same
	// width - while quietly never exercising the quoted path that the
	// separator setting exists to test. Nothing else here would see it, and
	// asking it this way needs no threshold and no lucky row.
	for other, ch := range map[string]string{"comma": ",", "semicolon": ";", "tab": "\t", "pipe": "|"} {
		if other == delim || !strings.Contains(string(body), ch) {
			continue
		}
		t.Errorf("a %s file carries a %s, so something in it is separating with a character "+
			"this dialect never asked for", delim, other)
	}

	csvPaddingCarriesTheSeparator(t, props, sep, delim, style, wantEOL)

	if style == "none" && bytes.Contains(body, []byte(`"`)) {
		t.Error("quote_style none produced a file with a quote in it")
	}
	if style == "all" && !bytes.HasPrefix(body, []byte(`"`)) {
		t.Errorf("quote_style all left the first field bare: %.60s", lines[0])
	}
}

// csvPaddingCarriesTheSeparator asks whether the padded row puts the separator
// inside the quotes, which is the only reason that column is ever quoted.
//
// Asked by counting: a row has five separators between its six fields, so any
// row carrying more than five has them inside the quotes.
//
// Over a BAND of sizes rather than one, and that is the whole reason this is a
// function of its own. The closing row is whatever length was left over, and a
// short one carries no separator at all - which is legal, and which made the
// first version of this check fail on quote_style all while the build was
// right. Measured 2026-09-03: the filler first carries a separator at 30 B of
// description, and a closing description runs anywhere from empty to about
// four times that. One size proves nothing either way. So the band is walked
// and the count is asserted, rather than one row being assumed to be long.
func csvPaddingCarriesTheSeparator(t *testing.T, props map[string]string, sep, delim, style, eol string) {
	t.Helper()
	const band = 24
	carried, widest := 0, 0
	for size := int64(8192); size < 8192+band; size++ {
		body := string(writeCSV(t, size, 9, props))
		rows := strings.Split(strings.TrimSuffix(body, eol), eol)
		last := rows[len(rows)-1]
		if inside := strings.Count(last, sep) - 5; inside > 0 {
			carried++
		}
		if len(last) > widest {
			widest = len(last)
		}
	}

	if style == "none" {
		if carried != 0 {
			t.Errorf("%d of %d closing rows carry a %s beyond the five that separate the fields, and "+
				"quote_style none has no quotes to hide one in", carried, band, delim)
		}
		return
	}
	if carried == 0 {
		t.Errorf("not one of %d closing rows carries a %s inside its description - the widest of them was "+
			"%d B, so either the padding stopped emitting the separator or this guard never reached a row "+
			"long enough to hold one", band, delim, widest)
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
	delims, endings, styles := csvDialects(t)
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
				for _, style := range styles {
					name := delim + "_" + eol + "_" + header + "_" + style
					t.Run(name, func(t *testing.T) {
						props := map[string]string{
							"delimiter": delim, "line_ending": eol,
							"header": header, "quote_style": style,
						}
						body := writeCSV(t, 8192, 9, props)
						res := check(t, body, name, "delimiter="+delim, "line_ending="+eol,
							"header="+header, "quote_style="+style)
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
	}

	if ran == 0 {
		t.Skip("the structural check never ran, so nothing here was judged")
	}
	if want := len(delims) * len(endings) * len(styles) * 2; ran != want {
		t.Errorf("%d of %d dialects reached the checker", ran, want)
	}

	// The checker is told, so it has to disagree when it is told wrong.
	//
	// Quoting needs both directions here and the dialect above does not,
	// because quoting is the one axis where every value produces a well formed
	// file. A file that quotes nothing and a file that quotes everything both
	// parse, so nothing about the table gives the style away - if the checker
	// were a rubber stamp on this axis it would be a rubber stamp silently.
	plain := writeCSV(t, 8192, 4, map[string]string{"delimiter": "comma", "line_ending": "lf"})
	everything := writeCSV(t, 8192, 4, map[string]string{"quote_style": "all"})
	nothing := writeCSV(t, 8192, 4, map[string]string{"quote_style": "none"})
	for _, wrong := range []struct {
		what, setting string
		body          []byte
	}{
		{"a comma file called semicolon", "delimiter=semicolon", plain},
		{"a comma file called tab", "delimiter=tab", plain},
		{"an lf file called crlf", "line_ending=crlf", plain},
		{"a minimal file called all", "quote_style=all", plain},
		{"an all file called minimal", "quote_style=minimal", everything},
		{"an all file called none", "quote_style=none", everything},
		{"a none file called all", "quote_style=all", nothing},
	} {
		res := check(t, wrong.body, "wrong_"+strings.ReplaceAll(wrong.setting, "=", "_"), wrong.setting)
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
		style string
	}{
		{map[string]string{}, ",", "lf", true, "minimal"},
		{map[string]string{"delimiter": "semicolon"}, ";", "lf", true, "minimal"},
		{map[string]string{"delimiter": "tab", "line_ending": "crlf"}, "\t", "crlf", true, "minimal"},
		{map[string]string{"delimiter": "pipe", "header": "false"}, "|", "lf", false, "minimal"},
		{map[string]string{"quote_style": "all"}, ",", "lf", true, "all"},
		{map[string]string{"quote_style": "none", "delimiter": "tab"}, "\t", "lf", true, "none"},
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
			// The word rather than a count of quotes, because the recipe and
			// the manifest name this one the same way. There is no second
			// spelling of it the way there is for the separator.
			if got := p.Properties["quote_style"]; got != c.style {
				t.Errorf("quote_style is %v, wanted %q", got, c.style)
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
