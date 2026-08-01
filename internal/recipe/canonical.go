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
// name is the file the bytes came from. It appears only in the error, and it
// is there because a hook running the formatter over a directory has to be
// told which file it stopped on.
func Canonical(src []byte, name string) ([]byte, error) {
	// The formatter has to turn away the same files the rest of the tool turns
	// away. It used to lay out a file holding two recipes without a word, and
	// with -w it settled the file so that --check then passed - after which
	// "tfg generate" refused the very file a pre commit hook had just called
	// clean. Two commands disagreeing about what a recipe is sends the reader
	// looking for the wrong thing.
	f, err := oneDocument(src, name)
	if err != nil {
		return nil, err
	}
	// Measured on ten inputs, including an empty file, a comment on its own,
	// input with no closing newline and input with CRLF endings: the printer
	// always ends with a newline. A defensive branch here would be code no
	// test can reach, and mutation testing is what showed that up.
	return []byte(f.String()), nil
}

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
	if n := len(f.Docs); n > 1 {
		return nil, &ValidationError{Name: name, Problems: []Problem{{
			What: fmt.Sprintf("the file holds %d YAML documents", n),
			Why:  "a recipe is one document, and everything after the first separator would be ignored without a word",
			Fix:  "remove the --- separators, or split the file into one recipe per file",
		}}}
	}
	return f, nil
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
