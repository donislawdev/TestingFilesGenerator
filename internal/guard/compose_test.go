package guard

import (
	"errors"
	"strings"
	"testing"

	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// A recipe composed from what somebody typed survives anything they typed.
//
// This is the guard the size-boundaries preset needed and did not have. That
// preset writes its document with fmt.Fprintf, and the comment above it said
// every value was one the package built itself - so quoting was unnecessary.
// The comment was wrong: the id carried the caller's text, and "1\rB" reached
// the document raw and broke it, because a carriage return sits in the middle
// where a size parser trims only the ends. Found by fuzzing on 2026-08-05, not
// by reading.
//
// recipe.Compose takes EVERY value from somebody's typing, so that whole class
// is live rather than theoretical. The one that matters most is not exotic: the
// name template this tool tells people to use is {index:04}, which begins with a
// brace and holds a colon. Written into a document unquoted, YAML reads it as a
// flow mapping and the recipe either fails to parse or parses into something
// nobody asked for - and the second is worse, because it is silent.
//
// Two things are asserted and the first is the sharper one. A composed document
// must never be a SyntaxError: that would be this tool producing a broken file
// and then blaming the person who typed a quotation mark. And a value that is
// legal has to arrive as itself, byte for byte, because a value quietly altered
// on the way in is untouchable rule 6 - silence - with the tool doing the
// altering.
func TestARecipeComposedFromTypedTextSurvivesAnythingTypedIntoIt(t *testing.T) {
	hostile := []struct {
		name string
		text string
	}{
		{"the name template this tool recommends", "{index:04}"},
		{"a flow mapping", "{a: b}"},
		{"a flow sequence", "[one, two]"},
		{"a colon and a space", "group: files"},
		{"a quotation mark", `say "when"`},
		{"a single quote", "it's here"},
		{"a hash", "files #1"},
		{"a carriage return in the middle", "one\rtwo"},
		{"a newline in the middle", "one\ntwo"},
		{"a tab", "one\ttwo"},
		{"a leading space", " leading"},
		{"a trailing space", "trailing "},
		{"a backslash", `back\slash`},
		{"an anchor", "&anchor"},
		{"an alias", "*alias"},
		{"a document marker", "---"},
		{"a comment opener alone", "#"},
		{"an empty looking word", "null"},
		{"a word YAML reads as false", "no"},
		{"a number as text", "0755"},
		{"characters from another script", "pliki_zażółć_日本語"},
		{"an emoji", "files \U0001F4C4"},
	}

	for _, h := range hostile {
		t.Run(h.name, func(t *testing.T) {
			// The hostile text goes in all three free text places at once. Each
			// is a different position in the document - a key's value, a value
			// inside a nested entry - and quoting that works in one and not
			// another would otherwise pass.
			src, err := recipe.Compose(recipe.Document{
				OutDir: h.text,
				Targets: []recipe.TargetDraft{{
					ID:     h.text,
					Format: "txt",
					Count:  "2",
					Size:   "1kb",
					Name:   h.text,
					Group:  h.text,
				}},
			})
			// A control character has to be refused, because neither of the
			// other two answers is acceptable: the library breaks the document
			// on a carriage return and drops a tab without a word.
			if control := strings.ContainsFunc(h.text, isControl); control {
				if err == nil {
					t.Fatalf("this text holds a control character and composing accepted it.\n"+
						"Text: %q\nDocument:\n%s", h.text, src)
				}
				var bad *recipe.ValidationError
				if !errors.As(err, &bad) {
					t.Fatalf("the refusal is not the addressed kind, so a window could not mark\n"+
						"the box it came from: %v", err)
				}
				for _, p := range bad.Problems {
					if p.At == "" {
						t.Errorf("a refusal about an unwritable character carries no address: %q", p.What)
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("composing refused this text outright: %v\nText: %q", err, h.text)
			}

			rec, err := recipe.Parse(src, "composed")
			var syntax *recipe.SyntaxError
			if errors.As(err, &syntax) {
				t.Fatalf("the composed document is not readable as a recipe, which means this\n"+
					"tool wrote a broken file and would blame the person who typed the value.\n"+
					"Text: %q\nRefusal: %v\nDocument:\n%s", h.text, err, src)
			}
			if err != nil {
				// A validation refusal is a legitimate answer for some of these
				// - a name the file system will not take, for instance. What is
				// not legitimate is the document being unreadable, and that is
				// checked above.
				t.Logf("refused on its merits, which is allowed here: %v", err)
				return
			}

			if len(rec.Targets) != 1 {
				t.Fatalf("expected one target and got %d", len(rec.Targets))
			}
			got := rec.Targets[0]
			for _, pair := range []struct {
				field string
				value string
			}{
				{"id", got.ID},
				{"name", got.Name},
				{"group", got.Group},
			} {
				if pair.value != h.text {
					t.Errorf("the %s came back changed.\n  put in: %q\n  got out: %q\n"+
						"A value altered on the way through is this tool being silent about\n"+
						"editing somebody's input.", pair.field, h.text, pair.value)
				}
			}
			if rec.Output.Dir != h.text {
				t.Errorf("the output directory came back changed.\n  put in: %q\n  got out: %q",
					h.text, rec.Output.Dir)
			}
		})
	}
}

// The degenerate documents a screen really produces, which are not the same as
// bad ones.
//
// Every one of these is a state somebody can put a form into by doing nothing,
// and each has to end in a refusal that names something rather than in a crash
// or a syntax error. An empty screen is the one that would be met first by
// anybody opening the tool and pressing the button to see what happens.
func TestAnEmptyOrHalfFilledScreenComposesIntoARefusalThatNamesSomething(t *testing.T) {
	cases := []struct {
		name string
		doc  recipe.Document
		// wantAddressed is a setting the refusal has to name. Empty means the
		// document is expected to be accepted.
		wantAddressed string
	}{
		{
			name:          "nothing at all, which is what a screen holds before anybody adds a batch",
			doc:           recipe.Document{},
			wantAddressed: "targets",
		},
		{
			name:          "one batch with nothing typed into it",
			doc:           recipe.Document{Targets: []recipe.TargetDraft{{}}},
			wantAddressed: "targets[1].id",
		},
		{
			name: "a size box left empty",
			doc: recipe.Document{Targets: []recipe.TargetDraft{{
				ID: "a", Format: "txt",
			}}},
			wantAddressed: "targets[1].size",
		},
		{
			name: "two size boxes filled in, which three boxes invite",
			doc: recipe.Document{Targets: []recipe.TargetDraft{{
				ID: "a", Format: "txt", Size: "1kb", SizeRange: "1kb-2kb",
			}}},
			wantAddressed: "targets[1].size",
		},
		{
			name: "a reason with no outcome beside it",
			doc: recipe.Document{Targets: []recipe.TargetDraft{{
				ID: "a", Format: "txt", Size: "1kb", ExpectedReason: "size_limit",
			}}},
			wantAddressed: "targets[1].expected",
		},
		{
			name: "a container told to hold something with no size",
			doc: recipe.Document{Targets: []recipe.TargetDraft{{
				ID: "arch", Format: "zip",
				Contains: []recipe.ContentDraft{{Format: "pdf"}},
			}}},
			wantAddressed: "targets[1].contains[1].size",
		},
		{
			name: "two batches given the same name",
			doc: recipe.Document{Targets: []recipe.TargetDraft{
				{ID: "same", Format: "txt", Size: "1kb"},
				{ID: "same", Format: "txt", Size: "1kb"},
			}},
			wantAddressed: "targets[2].id",
		},
		{
			// The one that has to be accepted, so this test cannot pass by
			// everything being refused.
			name: "a batch filled in properly",
			doc: recipe.Document{
				Seed: "7", OutDir: "out", Manifest: "run.json",
				Targets: []recipe.TargetDraft{{
					ID: "a", Format: "txt", Count: "3", Size: "1kb",
					Name: "{index:04}", Group: "smoke", Expected: "accept",
				}},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src, err := recipe.Compose(c.doc)
			if err != nil {
				t.Fatalf("composing failed: %v", err)
			}

			var syntax *recipe.SyntaxError
			rec, err := recipe.Parse(src, "composed")
			if errors.As(err, &syntax) {
				t.Fatalf("a document a screen can produce is not readable at all:\n%v\n\n%s", err, src)
			}

			if c.wantAddressed == "" {
				if err != nil {
					t.Fatalf("this document was meant to be accepted and was refused:\n%v\n\n%s", err, src)
				}
				if len(rec.Targets) != len(c.doc.Targets) {
					t.Errorf("composed %d targets and read back %d", len(c.doc.Targets), len(rec.Targets))
				}
				return
			}

			var bad *recipe.ValidationError
			if !errors.As(err, &bad) {
				t.Fatalf("this document was meant to be refused and was accepted.\n\n%s", src)
			}
			var addresses []string
			for _, p := range bad.Problems {
				addresses = append(addresses, p.At)
			}
			for _, at := range addresses {
				if at == c.wantAddressed {
					return
				}
			}
			t.Errorf("nothing in the refusal is addressed to %q, so a screen would have no box\n"+
				"to mark and the message would go to the foot of the form.\n"+
				"  addressed: %s\n\n%s",
				c.wantAddressed, strings.Join(addresses, ", "), src)
		})
	}
}

// isControl is the class Compose refuses, restated here rather than imported.
//
// A guard that asked the code under test which characters it rejects would agree
// with it whatever it did. This is the second opinion.
func isControl(r rune) bool { return r < 0x20 || r == 0x7F }

// Every character, not the two dozen somebody thought of.
//
// The table above is a list of guesses, and a list of guesses is exactly how the
// carriage return survived in the size-boundaries preset until fuzzing found it.
// This sweeps every rune from nothing up through the C0 block, ASCII, Latin-1 and
// a few beyond, and demands one of two answers for each: refused, or carried
// through as itself. There is no third acceptable answer, and "quietly altered"
// is the one this is looking for.
//
// Cheap enough to run every time - it is a few hundred compose and parse pairs on
// a one target document.
func TestEveryCharacterIsEitherCarriedThroughAComposedRecipeOrRefused(t *testing.T) {
	var runes []rune
	for r := rune(0); r <= 0x100; r++ {
		runes = append(runes, r)
	}
	// A handful past Latin-1: a combining mark, a right to left mark, a zero
	// width space, an ideograph, something outside the basic plane, and the
	// replacement character itself.
	runes = append(runes, 0x0301, 0x200B, 0x200F, 0x3042, 0x1F4C4, 0xFFFD)

	for _, r := range runes {
		// Surrogates are not characters and cannot appear in a Go string as
		// themselves, so there is nothing here to ask about.
		if r >= 0xD800 && r <= 0xDFFF {
			continue
		}

		// Placed between two ordinary letters, because the ends of a value are
		// trimmed in several places and a character that only breaks in the
		// middle is the one that got through last time.
		value := "a" + string(r) + "b"

		src, err := recipe.Compose(recipe.Document{
			Targets: []recipe.TargetDraft{{
				ID: value, Format: "txt", Size: "1kb", Group: value,
			}},
		})

		if isControl(r) {
			if err == nil {
				t.Errorf("U+%04X is a control character and composing accepted it:\n%s", r, src)
			}
			continue
		}
		if err != nil {
			t.Errorf("U+%04X was refused by composing, and it is not a control character: %v", r, err)
			continue
		}

		rec, parseErr := recipe.Parse(src, "composed")
		var syntax *recipe.SyntaxError
		if errors.As(parseErr, &syntax) {
			t.Errorf("U+%04X composes into a document that cannot be read:\n%v\n%s", r, parseErr, src)
			continue
		}
		if parseErr != nil {
			// Refused on its merits is allowed. Unreadable is not, and that is
			// the case above.
			continue
		}
		if len(rec.Targets) != 1 {
			t.Errorf("U+%04X produced %d targets", r, len(rec.Targets))
			continue
		}
		if got := rec.Targets[0].ID; got != value {
			t.Errorf("U+%04X came back changed.\n  put in: %q\n  got out: %q", r, value, got)
		}
		if got := rec.Targets[0].Group; got != value {
			t.Errorf("U+%04X came back changed in the group.\n  put in: %q\n  got out: %q", r, value, got)
		}
	}
}
