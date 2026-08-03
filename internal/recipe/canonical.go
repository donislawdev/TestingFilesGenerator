package recipe

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

// Canonical is the recipe in one settled shape, comments and blank lines kept.
//
// Two things ride on this. The window saves recipes, and a save that reordered
// keys would produce a whole file diff every time somebody pressed the button -
// after which people stop committing recipes at all, and reproducibility from
// a file in a repository exists only in the documentation. And the hash below
// is taken from this form rather than from the raw bytes, so reformatting a
// file does not pretend to be a change of content.
//
// Measured on this parser: comments in every position survive, blank lines
// between sections survive, and a second pass changes nothing. The probe that
// settled it is in tools/probes/yaml-roundtrip.
//
// name is the file the bytes came from. It appears only in the error, and it
// is there because a hook running the formatter over a directory has to be
// told which file it stopped on.
//
// A byte order mark is dropped rather than kept, so the settled shape of a
// file that had one does not have one. A recipe saved by an editor that adds
// a mark is then reported as unsettled, and -w takes the mark off.
func Canonical(src []byte, name string) ([]byte, error) {
	// The formatter has to turn away the same files the rest of the tool turns
	// away. It used to lay out a file holding two recipes without a word, and
	// with -w it settled the file so that --check then passed - after which
	// "tfg generate" refused the very file a pre commit hook had just called
	// clean. Two commands disagreeing about what a recipe is sends the reader
	// looking for the wrong thing.
	f, err := oneDocument(withoutBOM(src), name)
	if err != nil {
		return nil, err
	}
	// Measured on ten inputs, including an empty file, a comment on its own,
	// input with no closing newline and input with CRLF endings: the printer
	// always ends with a newline. A defensive branch here would be code no
	// test can reach, and mutation testing is what showed that up.
	out := []byte(f.String())

	// Proved rather than assumed: settle the result again and require it not to
	// move. A settled shape that settles differently is not one.
	//
	// The whole pass is repeated rather than only the parser, and that
	// distinction was measured rather than reasoned about. Checking with the
	// parser alone passed on a space followed by a byte order mark, while a
	// second real pass gave a different file: the parser drops the space, which
	// promotes the mark to the front, where the next pass strips it. Anything
	// that only re-reads misses the steps around the reading.
	again, err := oneDocument(withoutBOM(out), name)
	if err != nil {
		return nil, &ValidationError{Name: name, Problems: []Problem{{
			What: "the settled shape of this file cannot be read back",
			Why:  "the layout this file needs is one the formatter cannot write, so settling it would replace it with something no longer readable",
			Fix:  "leave the file as it is, and check it for an anchor or an alias used where a key belongs",
		}}}
	}
	if again.String() != string(out) {
		return nil, &ValidationError{Name: name, Problems: []Problem{{
			What: "the settled shape of this file does not stay settled",
			Why:  "settling it twice gives two different files, so a check would never pass and the recipe hash in a manifest would not describe it",
			Fix:  "leave the file as it is, and simplify whatever it uses that has no single written form - a byte order mark that is not at the very start is the usual cause",
		}}}
	}
	return out, nil
}

// Why the check above exists at all, and it is not tidiness.
//
// The printer is not ours and it is not faithful for everything the parser
// accepts. Found by fuzzing on 2026-08-03, in under a second: the input
//
//	*0000000 : 000
//
// parses, prints as "*0000000: 000", and that does not parse. An alias used as
// a key survives the reading and not the writing.
//
// Without the check the damage is not a bad message, it is the user's file.
// Measured the same day, following the tool's own instructions:
//
//	tfg recipe fmt --check r.yaml  ->  3, "not in its settled shape, run -w"
//	tfg recipe fmt -w r.yaml       ->  0, "rewritten"
//	tfg recipe fmt --check r.yaml  ->  3, cannot be read as a recipe
//
// The original content is gone at that point, replaced by something this tool
// cannot read, and the command that did it reported success. "recipe fmt" is
// the one command here that writes over a file somebody wrote by hand.

// oneDocument parses a recipe and refuses a file holding more than one.
//
// A recipe is one document. A file holding two used to parse without a word
// and produce the first one only - half the fixtures somebody asked for, and
// exit code zero to say it went fine.
//
// This is the single place that decides how many recipes a file holds, so the
// reading path and the formatting path cannot drift into disagreeing.
func oneDocument(src []byte, name string) (*ast.File, error) {
	f, err := parser.ParseBytes(src, parser.ParseComments)
	if err != nil {
		return nil, &SyntaxError{Name: name, Detail: strings.TrimRight(err.Error(), "\n")}
	}
	if n := recipesIn(f); n > 1 {
		return nil, &ValidationError{Name: name, Problems: []Problem{{
			What: fmt.Sprintf("the file holds %d YAML documents", n),
			Why:  "a recipe is one document, and everything after the first separator would be ignored without a word",
			Fix:  "remove the --- separators, or split the file into one recipe per file",
		}}}
	}
	return f, nil
}

// bom is the byte order mark. Notepad on Windows writes one by default, and
// this tool is aimed at testers on Windows.
//
// Written as an escape because Go source may not carry the character itself,
// and because it keeps this file inside the ASCII the guard scans for.
const bom = "\ufeff"

// withoutBOM drops a leading byte order mark.
//
// Left in place it reaches the decoder as part of the first key, which then
// refuses `version` as an unknown field - and the mark does not render, so the
// message reads "unknown field version" and points at a typo that is not
// there. Measured, the message was:
//
//	[1:1] unknown field "<mark>version"
//
// That is the exact failure the strict decoder exists to prevent, arriving by
// the one route strictness cannot help with.
//
// Only leading marks. Further along the file it is content, and a recipe
// carrying one in a file name or a label is entitled to keep it.
//
// Every leading mark rather than the first, and that was not the original rule.
// Found by fuzzing on 2026-08-03: a file starting with two of them settled to
// one mark on the first pass and to none on the second, so "recipe fmt -w"
// changed the file every time it ran and "--check" never went green. One tool
// writing a mark over a file that already had one is how the second gets there,
// and this project is aimed at testers on Windows where editors add them.
//
// The reasoning behind the old rule is untouched: a mark that is not at the
// start is content. Two marks at the start are both at the start.
func withoutBOM(src []byte) []byte {
	out := string(src)
	for strings.HasPrefix(out, bom) {
		out = strings.TrimPrefix(out, bom)
	}
	return []byte(out)
}

// recipesIn counts the documents in a parsed file that carry a recipe.
//
// It is deliberately not len(f.Docs), and the difference is not academic.
// Measured on this parser, tools/probes/yaml-roundtrip probe4: a comment
// sitting BEFORE a leading "---" becomes a document of its own, and so does a
// trailing "---" with nothing after it. Both files hold exactly one recipe,
// and counting raw documents turned both away with a message about separators
// the reader had not got wrong. A leading "---" is ordinary YAML house style,
// so the file it refused was one somebody had every reason to write.
//
// The other half was measured too, probe5: with the count loosened, the strict
// decoder reads the recipe rather than the empty first document, on all seven
// layouts tried. So this is a fix and not a different failure.
//
// Re-measure both probes on a parser upgrade. This counts on how the library
// attaches comments, which is not something it promises.
func recipesIn(f *ast.File) int {
	n := 0
	for _, d := range f.Docs {
		// Nothing at all, or nothing but a comment. A comment between two
		// separators belongs to the recipe beside it, not to a recipe of its
		// own - there is no such thing as a recipe made of one comment.
		if d.Body == nil || d.Body.Type() == ast.CommentType {
			continue
		}
		n++
	}
	return n
}

// Hash identifies the content of a recipe, independent of how it was laid out.
//
// It goes into the manifest so that a run can be traced back to the recipe
// that shaped it.
// It takes no file name because every caller reaches it through Parse, which
// has already refused anything Canonical could object to.
func Hash(src []byte) (string, error) {
	canon, err := Canonical(src, "recipe")
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canon)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
