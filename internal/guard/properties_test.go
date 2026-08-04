package guard

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// A format used to declare the names of its settings and nothing else. The
// type and the range lived inside the generator that read them, one import
// away from everything that needed them: tfg formats could not print them, a
// window had nothing to build a field from, and each format phrased the same
// refusal in its own words.
//
// Two formats declare properties today and every format eventually will - a
// WAV its sample rate, a ZIP its compression method, a JPG its quality. These
// guards are about the shape holding up for those, not about the four.

// A value outside what the format declares is the caller's mistake, and the
// exit code has to say so.
//
// It used to say the opposite. A bad value surfaced as an ordinary error and
// fell through to RUNTIME, which in the frozen table means the tool itself
// broke - so CI could not tell "you typed that wrong" from "file a bug against
// this program". The exit code guard was green throughout, because it asks
// whether a code is in the table and 1 is in the table.
func TestABadPropertyValueIsTheCallersMistakeNotOurs(t *testing.T) {
	for _, tc := range []struct{ format, set, why string }{
		{"png", "width=abc", "not a number at all"},
		{"png", "width=-5", "below the declared minimum"},
		{"png", "width=99999", "above the declared maximum"},
		{"pdf", "page_size=a7", "not one of the declared choices"},
		// This one used to pass the first check and fail deeper, in different
		// words, because bit depth was declared as a range of 8 to 32 when it
		// is really a set of four values.
		{"wav", "bit_depth=20", "inside the old range but not a real depth"},
	} {
		t.Run(tc.format+" "+tc.set, func(t *testing.T) {
			dir := t.TempDir()
			code, stdout, errOut := run(t, "generate", "--format", tc.format,
				"--size", "200kb", "--set", tc.set, "--out", dir)

			if code == cli.ExitRuntime {
				t.Errorf("exit %d says this tool has a bug, and the value is %s: %s",
					code, tc.why, errOut)
			}
			if code != cli.ExitFormat {
				t.Errorf("exit %d, expected %d - a value the format cannot take is the "+
					"same class as a size below the minimum", code, cli.ExitFormat)
			}
			if stdout != "" {
				t.Errorf("a failed run wrote to stdout: %s", stdout)
			}
			// The value has to appear, or somebody with a recipe of thirty
			// targets cannot tell which one is wrong.
			if !strings.Contains(errOut, strings.SplitN(tc.set, "=", 2)[1]) {
				t.Errorf("the refusal does not quote the value: %s", errOut)
			}
		})
	}
}

// The declaration is what refuses, so every format refuses in the same words
// and a new format gets the wording by declaring rather than by writing it.
func TestTheRefusalComesFromTheDeclaration(t *testing.T) {
	d, err := format.Get("pdf")
	if err != nil {
		t.Fatalf("pdf is not registered: %v", err)
	}
	var pages format.Property
	for _, p := range d.Properties {
		if p.Name == "pages" {
			pages = p
		}
	}
	if pages.Kind != format.PropertyInt || pages.Max == 0 {
		t.Fatalf("pages is declared as %+v, expected a bounded whole number", pages)
	}
	// Both ends. Checking only the low one leaves half the declaration
	// unguarded, and a mutation disabling the upper bound stayed green until
	// this line existed.
	if bad := pages.Allows("0"); bad == "" {
		t.Error("zero pages was allowed by a property declared to start at 1")
	}
	if bad := pages.Allows(strconv.FormatInt(pages.Max+1, 10)); bad == "" {
		t.Errorf("%d pages was allowed by a property declared to stop at %d", pages.Max+1, pages.Max)
	}
	if bad := pages.Allows(pages.Default); bad != "" {
		t.Errorf("pdf advertises pages default %q and then refuses it: %s", pages.Default, bad)
	}
}

// tfg formats <id> is documented in docs/CLI.md and used to print the whole
// list and ignore the argument, ending with 0 - so there was no way to ask
// what a format accepts and the silence looked like an answer.
func TestFormatsAnswersAboutOneFormat(t *testing.T) {
	code, stdout, errOut := run(t, "formats", "pdf")
	if code != cli.ExitOK {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	for _, want := range []string{"pages", "page_size", "a4", "5000"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("the description of pdf does not mention %q:\n%s", want, stdout)
		}
	}
	// The whole list would also contain "pdf", so the assertion that the
	// argument was honoured is that the other formats are absent.
	if strings.Contains(stdout, "wav") {
		t.Errorf("asking about pdf described every format instead:\n%s", stdout)
	}

	code, stdout, _ = run(t, "formats", "nosuchformat")
	if code == cli.ExitOK {
		t.Error("a format nobody registered was described anyway")
	}
	if stdout != "" {
		t.Errorf("a failed run wrote to stdout: %s", stdout)
	}
}

// Every declared property reaches the machine readable form, because that is
// what a window and a script build from. Names alone were what the JSON
// carried before, which is the same gap one layer out.
func TestTheMachineReadableListCarriesTheWholeDeclaration(t *testing.T) {
	code, stdout, errOut := run(t, "formats", "png", "--json")
	if code != cli.ExitOK {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	for _, want := range []string{`"kind"`, `"min"`, `"max"`, `"unit"`, `"detail"`} {
		if !strings.Contains(stdout, want) {
			t.Errorf("the JSON for png has no %s, so a window cannot draw a field from it:\n%s",
				want, stdout)
		}
	}

	// The keys being present is the easy half, and until 2026-08-04 it was the
	// only half. Emptying every value left all five keys in place and this
	// guard green, so a window would have drawn fields with no unit, no
	// default and no help text and nothing would have said so.
	//
	// Compared against the registry rather than against words written here.
	// A second copy of the expected values would be a second place to keep up
	// to date, and this test exists to find the two disagreeing.
	d, err := format.Get("png")
	if err != nil {
		t.Fatalf("png is not registered: %v", err)
	}
	var list []struct {
		Properties []struct {
			Name, Kind, Unit, Default, Detail string
			Choices                           []string
		} `json:"properties"`
	}
	if err := json.Unmarshal([]byte(stdout), &list); err != nil {
		t.Fatalf("the machine readable list is not JSON: %v\n%s", err, stdout)
	}
	if len(list) != 1 {
		t.Fatalf("expected one format, got %d", len(list))
	}
	printed := map[string]string{}
	for _, p := range list[0].Properties {
		printed[p.Name] = p.Unit + "\x00" + p.Default + "\x00" + p.Detail
	}
	if len(d.Properties) == 0 {
		t.Fatal("png declares no properties, so this half of the guard checks nothing")
	}
	for _, want := range d.Properties {
		got, ok := printed[want.Name]
		if !ok {
			t.Errorf("the declared property %q is not in the machine readable list", want.Name)
			continue
		}
		if declared := want.Unit + "\x00" + want.Default + "\x00" + want.Detail; got != declared {
			t.Errorf("property %q is printed as %q and declared as %q", want.Name, got, declared)
		}
	}
}
