package guard

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// Every other guard here checks an input somebody thought of. The size guard
// walks about 120 sizes per format, the exit code guard walks 14 endings, and
// each of those lists was written by the same mind that wrote the code - so it
// contains the cases that mind had in view.
//
// Fuzzing is the only thing in this project that generates the input instead.
// It is native to Go, so it costs no dependency and cannot trip the gate that
// watches the module list.
//
// These run as ordinary tests over their seed corpus on every `go test`. To
// actually search, ask for it by name and give it a budget:
//
//	go test ./internal/guard/ -run FuzzParseSize -fuzz FuzzParseSize -fuzztime 60s
//
// What they assert is deliberately weak: no panic, and an answer that does not
// contradict itself. A stronger claim would need a second implementation of the
// same parser, and two readings of one grammar prove nothing the first did not.

// FuzzParseSize feeds arbitrary text to the size parser.
//
// It is the widest untrusted surface in the tool. Every size in a recipe and
// every --size on the command line goes through it, and it accepts several
// shapes at once: plain bytes, decimal points, a dozen unit spellings.
func FuzzParseSize(f *testing.F) {
	for _, seed := range []string{
		"1", "0", "1024", "10mb", "1.5gib", "700kB", "  8kb  ", "2MB",
		"-1", "", "mb", "1e9", "9223372036854775807", "1.7976931348623157e309",
		"1kb1", "٣", "1\x00mb", "1 mb", "+5", "0x10", "1,5mb",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, s string) {
		n, err := core.ParseSize(s)
		if err != nil {
			// A refusal is a fine answer. What is not fine is a refusal that
			// hands back a number somebody might use anyway.
			if n != 0 {
				t.Fatalf("ParseSize(%q) refused with %v and still returned %d", s, err, n)
			}
			return
		}
		// Accepted. A size is a byte count, so it cannot be negative - and a
		// negative one would reach the planner as a length to allocate.
		if n < 0 {
			t.Fatalf("ParseSize(%q) accepted and returned %d", s, n)
		}
	})
}

// FuzzNameTemplate throws arbitrary name templates at the planner.
//
// This is the one input in the tool where being wrong means writing outside the
// directory the user pointed at, and it has been wrong once already: `name:
// "../../x"` wrote two directories up and finished with exit code 0, found by
// reading rather than by any test. A guard went in with five hand written
// names. Nobody had thrown generated ones at it.
//
// A recipe travels between teams by design, so the name in it is a string
// somebody else wrote. The invariant is deliberately about the outcome rather
// than the rule: whatever the planner accepts has to land inside the output
// directory. Asserting the rule instead - "it contains no slash" - would only
// restate the implementation and would pass on any escape nobody thought of.
func FuzzNameTemplate(f *testing.F) {
	for _, seed := range []string{
		"", "a.txt", "invoice_{index:04}.txt", "{index:04}",
		"../x", "..", ".", "/etc/passwd", `C:\x`, "C:x", `a\b`, "a/b",
		"con", "nul", "a ", "a.", ".hidden", "a\x00b", "{index}", "{",
		strings.Repeat("n", 300) + ".txt", "\u202ecod.txt", "ą.txt",
	} {
		f.Add(seed)
	}

	const dir = "out"

	f.Fuzz(func(t *testing.T, tmpl string) {
		planned, err := engine.Plan([]engine.Target{{
			ID:       "files",
			Format:   "txt",
			Sizes:    engine.Uniform(2, 4096),
			NameTmpl: tmpl,
			Label:    true,
		}}, engine.Options{OutDir: dir, ManifestName: "manifest.json"})
		if err != nil {
			// Refusing is the safe answer and most templates get it.
			return
		}

		for _, p := range planned {
			// Where the file would actually land, worked out the way the
			// writing path works it out.
			landed := filepath.Join(dir, p.Name)
			rel, relErr := filepath.Rel(dir, landed)
			switch {
			case p.Name == "":
				t.Fatalf("template %q was accepted and produced a file with no name", tmpl)
			case relErr != nil:
				t.Fatalf("template %q produced %q, which does not sit under %s at all", tmpl, p.Name, dir)
			case rel != p.Name:
				// The join cleaned something away, so the name was carrying
				// path structure rather than being a name.
				t.Fatalf("template %q produced %q, which resolves to %q inside %s", tmpl, p.Name, rel, dir)
			case rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)):
				t.Fatalf("template %q escapes the output directory: %q", tmpl, rel)
			}
		}
	})
}

// FuzzCanonicalRecipe feeds arbitrary bytes to the formatter and the hash.
//
// This is a second door into the same dependency and it was not being knocked
// on. FuzzParseRecipe stops at recipe.Parse, which decodes. Canonical parses to
// a tree and prints it back, which is different code in the library, and it
// runs on the same untrusted bytes in two ordinary places: "tfg recipe fmt",
// and every "tfg generate <recipe>", because the recipe hash in the manifest is
// taken from the settled form.
//
// It matters more than a second entry point usually would, for two measured
// reasons. The dependency has panicked on ordinary input before - "targets: ! "
// took down the decoder, and the recover that fixed it is scoped narrowly to
// the decode call, deliberately, so it does not cover this path at all. And
// Canonical accepts strictly more than Parse does: measured on 2026-08-03,
// "tfg recipe fmt --check" ends with 0 on five different recipes that "tfg
// validate" refuses. So this reaches inputs the other target never sees.
//
// What it asserts is deliberately weak, the same as its neighbours: no panic,
// and an answer that does not contradict itself. Settling twice has to give the
// same bytes, because the whole point of a canonical form is that it is one -
// and the hash of it goes into somebody else's manifest.
func FuzzCanonicalRecipe(f *testing.F) {
	for _, seed := range []string{
		"version: 1\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\n",
		"",
		"---\n",
		"# just a comment\n",
		"targets: ! ",
		"a: &anchor\nb: *anchor\n",
		"a: !!binary\n",
		"? complex\n: mapping\n",
		"a: |\n  block\n",
		"\xef\xbb\xbfversion: 1\n",
		"a: [1, 2,\n",
		"{",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, src string) {
		once, err := recipe.Canonical([]byte(src), "fuzz.yaml")
		if err != nil {
			if once != nil {
				t.Fatalf("Canonical refused with %v and still returned %d bytes", err, len(once))
			}
			return
		}

		// A settled form that does not settle is not one. Two passes have to
		// agree, or "tfg recipe fmt -w" would rewrite a file every time it ran
		// and a pre commit hook would never go green.
		twice, err := recipe.Canonical(once, "fuzz.yaml")
		if err != nil {
			t.Fatalf("the settled form of %q was refused on the second pass: %v", src, err)
		}
		if string(twice) != string(once) {
			t.Fatalf("settling is not stable for %q:\nfirst  %q\nsecond %q", src, once, twice)
		}

		// The hash rides on the settled form, so it has to be as stable.
		h1, err1 := recipe.Hash([]byte(src))
		h2, err2 := recipe.Hash(once)
		if err1 != nil || err2 != nil {
			t.Fatalf("hashing what Canonical accepted failed: %v / %v", err1, err2)
		}
		if h1 != h2 {
			t.Fatalf("the hash moved when the file was settled, for %q: %s then %s", src, h1, h2)
		}
	})
}

// FuzzParseRecipe feeds arbitrary bytes to the recipe reader.
//
// A recipe is a file somebody else wrote, and this is where YAML from outside
// meets our validation. The parser is the one dependency in the project, so
// this is also the only place where a defect could arrive from code that is
// not ours.
func FuzzParseRecipe(f *testing.F) {
	for _, seed := range []string{
		"version: 1\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\n",
		"version: 1\n",
		"",
		"---\n",
		"version: 1\ntargets: []\n",
		"version: 1\ntargets:\n  - id: a\n    format: txt\n    boundary: 1mb\n",
		"version: 1\ntargets:\n  - id: a\n    format: zip\n    contains:\n      - format: pdf\n        count: 2\n        size: 1kb\n",
		"\xef\xbb\xbfversion: 1\n",
		"version: 99999999999999999999\n",
		"version: 1\ntargets:\n  - id: \"\\u0000\"\n    format: txt\n    size: 1\n",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, src string) {
		rec, err := recipe.Parse([]byte(src), "fuzz.yaml")
		if err != nil {
			// Refusing is the common answer and the right one. The contract is
			// that it says so rather than handing back half a recipe.
			if rec != nil {
				t.Fatalf("Parse refused with %v and still returned a recipe", err)
			}
			// RC7: every problem at once, and a message a person can act on.
			if strings.TrimSpace(err.Error()) == "" {
				t.Fatal("Parse refused with an empty message")
			}
			return
		}
		if rec == nil {
			t.Fatal("Parse returned no recipe and no error")
		}
		// Accepted. Every target has to carry the thing the planner will ask it
		// for, or the failure moves from here to the middle of a run.
		for i, tgt := range rec.Targets {
			if tgt.Format == "" {
				t.Fatalf("target %d was accepted with no format", i)
			}
			for _, size := range tgt.Sizes {
				if size < 0 {
					t.Fatalf("target %d was accepted with the size %d", i, size)
				}
			}
		}
	})
}
