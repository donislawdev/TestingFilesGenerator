package recipe

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

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
func Canonical(src []byte) ([]byte, error) {
	f, err := parser.ParseBytes(src, parser.ParseComments)
	if err != nil {
		return nil, &SyntaxError{Name: "recipe", Detail: strings.TrimRight(err.Error(), "\n")}
	}
	// Measured on ten inputs, including an empty file, a comment on its own,
	// input with no closing newline and input with CRLF endings: the printer
	// always ends with a newline. A defensive branch here would be code no
	// test can reach, and mutation testing is what showed that up.
	return []byte(f.String()), nil
}

// documents counts the YAML documents in a file.
//
// A recipe is one document. A file holding two used to parse without a word
// and produce the first one only - half the fixtures somebody asked for, and
// exit code zero to say it went fine.
func documents(src []byte) (int, error) {
	f, err := parser.ParseBytes(src, parser.ParseComments)
	if err != nil {
		return 0, &SyntaxError{Name: "recipe", Detail: strings.TrimRight(err.Error(), "\n")}
	}
	return len(f.Docs), nil
}

// Hash identifies the content of a recipe, independent of how it was laid out.
//
// It goes into the manifest so that a run can be traced back to the recipe
// that shaped it.
func Hash(src []byte) (string, error) {
	canon, err := Canonical(src)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canon)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
