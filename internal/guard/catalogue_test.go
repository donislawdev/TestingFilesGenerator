package guard

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const localeDir = "../gui/text/locale"

// entryInCatalogue is one message as a translation file carries it.
type entryInCatalogue struct {
	Description string `json:"description"`
	Other       string `json:"other"`
	One         string `json:"one"`
	Few         string `json:"few"`
	Many        string `json:"many"`
	Zero        string `json:"zero"`
	Two         string `json:"two"`
}

// englishInTheCode reads every message the window states, by asking the source
// rather than by running it.
//
// The source rather than the package, because there is no way to enumerate
// functions at runtime and a list written out by hand is a list that goes
// stale. This is the same choice two of the boundary guards made on 2026-08-20
// and for the same reason: where there is nothing to assert against, reading
// what was written is better than asserting nothing.
func englishInTheCode(t *testing.T) (map[string]string, map[string]string) {
	t.Helper()
	out := map[string]string{}
	// The singular of every plural message, kept apart because only some
	// messages have one.
	singular := map[string]string{}
	for _, name := range []string{"screens.go", "text.go"} {
		path := filepath.Join("../gui/text", name)
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			fn, ok := call.Fun.(*ast.Ident)
			if !ok {
				return true
			}
			// Three ways of stating a message, and all three have to be read
			// here. Reading only say(, which is what this did until 2026-08-26,
			// makes every sentence carrying a value invisible to the guard -
			// and the guard would then report the catalogue's own entries as
			// orphans, which is how this was noticed.
			var id, english string
			var okID, okText bool
			switch {
			case fn.Name == "say" && len(call.Args) == 2:
				id, okID = literal(call.Args[0])
				english, okText = literal(call.Args[1])
			case fn.Name == "sayf" && len(call.Args) == 3:
				id, okID = literal(call.Args[0])
				english, okText = literal(call.Args[1])
			case fn.Name == "sayN" && len(call.Args) == 5:
				// The plural form is what the catalogue calls "other", and it
				// is the one this compares. The singular is checked beside it
				// below, because a catalogue holding one of the two is worse
				// than one holding neither: it looks complete.
				id, okID = literal(call.Args[0])
				english, okText = literal(call.Args[2])
				if one, ok := literal(call.Args[1]); ok && okID {
					singular[id] = one
				}
			default:
				return true
			}
			if !okID || !okText {
				t.Errorf("%s: a message is stated with something other than plain text", path)
				return true
			}
			if was, seen := out[id]; seen && was != english {
				t.Errorf("%s is stated twice and differently: %q and %q", id, was, english)
			}
			out[id] = english
			return true
		})
	}
	if len(out) == 0 {
		t.Fatal("no message was found in the text package, so this guard is asserting about nothing")
	}
	if len(singular) == 0 {
		t.Fatal("no message with a plural was found, so the half of this guard that reads them proves nothing")
	}
	return out, singular
}

func literal(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	v, err := strconv.Unquote(lit.Value)
	return v, err == nil
}

// TestTheEnglishCatalogueSaysWhatTheCodeSays keeps the copy from becoming a
// second opinion.
//
// English is written once, beside the entry and beside the reason it is worded
// that way, and it answers when no catalogue is loaded. en.json is that same
// English written out again so a translator has a complete file to copy instead
// of a Go package to read - which means there are now two places one sentence
// exists, and nothing but this stops them disagreeing.
//
// The failure it is really for is quieter than a mismatched word: a message
// ADDED to the window and not to the catalogue. A translator copying en.json
// would then be handed a file missing that sentence, translate everything in
// it, and ship a window with one English line nobody can find the source of.
func TestTheEnglishCatalogueSaysWhatTheCodeSays(t *testing.T) {
	code, singular := englishInTheCode(t)

	raw, err := os.ReadFile(filepath.Join(localeDir, "en.json"))
	if err != nil {
		t.Fatalf("the English catalogue is not there: %v", err)
	}
	var have map[string]entryInCatalogue
	if err := json.Unmarshal(raw, &have); err != nil {
		t.Fatalf("the English catalogue is not readable: %v", err)
	}

	for id, english := range code {
		entry, listed := have[id]
		if !listed {
			t.Errorf("the window says %q and the English catalogue has no entry for it.\n"+
				"Run: python tools/gen-locale.py", id)
			continue
		}
		if entry.Other != english {
			t.Errorf("%s says %q in the code and %q in the English catalogue.\n"+
				"Run: python tools/gen-locale.py", id, english, entry.Other)
		}
		// docs/GUI.md section 6: a translator reading a bare sentence cannot
		// tell a field label from a column heading.
		if strings.TrimSpace(entry.Description) == "" {
			t.Errorf("%s has no sentence saying where it appears", id)
		}
		// A plural message carries two English forms and the catalogue has to
		// hold both. One of the two looks complete and is not.
		if one, plural := singular[id]; plural && entry.One != one {
			t.Errorf("%s says %q in the singular in the code and %q in the English catalogue.\n"+
				"Run: python tools/gen-locale.py", id, one, entry.One)
		}
	}
	for id := range have {
		if _, still := code[id]; !still {
			t.Errorf("the English catalogue carries %q and the window no longer says it.\n"+
				"Run: python tools/gen-locale.py", id)
		}
	}
}

// TestEveryTranslationObeysThePunctuationRule closes the hole docs/GUI.md
// section 6 named before there was anything to close.
//
// D17 is a flat hyphen and no semicolons, and PRODUCT.md section 3.7 says it
// holds in translation files in every language. The punctuation guard covers
// the README, both changelogs, package comments and the literals in
// internal/cli and cmd/tfg - and deliberately not internal/gui, because a
// window may carry characters outside ASCII and that guard refuses them.
//
// So on the day the first translation file appeared, D17 lost its guard exactly
// where PRODUCT.md declares its scope. This is that guard, and it keeps only the
// half that means anything here: the dashes and the semicolons. Refusing
// characters outside ASCII in a translation would be refusing the point of one.
func TestEveryTranslationObeysThePunctuationRule(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(localeDir, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no translation file was found, so this guard is asserting about nothing")
	}

	banned := []struct {
		what string
		mark string
	}{
		{"an en dash", "–"},
		{"an em dash", "—"},
		{"a semicolon", ";"},
	}

	sort.Strings(files)
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		var have map[string]entryInCatalogue
		if err := json.Unmarshal(raw, &have); err != nil {
			t.Fatalf("%s is not readable: %v", path, err)
		}
		ids := make([]string, 0, len(have))
		for id := range have {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			e := have[id]
			// Every form a language can carry, rather than the one English
			// happens to use. Polish picks one of three from the number, and a
			// rule that only read "other" would let two of them through.
			for _, said := range []string{e.Other, e.One, e.Few, e.Many, e.Zero, e.Two} {
				for _, bad := range banned {
					if strings.Contains(said, bad.mark) {
						t.Errorf("%s: %s carries %s - D17 asks for a flat hyphen and no semicolons:\n  %s",
							filepath.Base(path), id, bad.what, said)
					}
				}
			}
		}
	}
}
