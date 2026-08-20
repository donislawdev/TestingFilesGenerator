package recipe

import (
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
)

// scalar is one value of a recipe, kept as the text its author wrote.
//
// Why this type exists at all, and it is not a style preference. YAML decides
// on its own what a bare number means, and its rules are not the rules a person
// reading the file applies. Measured on this parser on 2026-08-02, on our own
// binary, every one of these silently:
//
//	seed: 010          the run used 8
//	count: 010         eight files were produced, not ten
//	width: 0100        the PNG came out 64 by 64
//	seed: 0x10         the run used 16
//	seed: 1_000        read as 1000, by a rule nobody wrote down
//
// None of them said a word. A leading zero is what somebody types when they
// number runs 001, 002, 003, and a padded width is what somebody types to keep
// a column straight. The value changed, the manifest recorded the changed
// value, and there was nothing left to notice the mistake by. That is rule 6 of
// this project broken in the quietest way available.
//
// The fix is not a check bolted on afterwards. It is refusing to let YAML type
// our numbers in the first place: this type takes the raw source text of the
// node, and every number in a recipe is then parsed by us, in base ten, from
// what the author actually typed. What you see is what you get, and the trap
// has nowhere left to live.
//
// The cost, stated plainly: a spelling YAML would have accepted is now an
// error. 0x10 and 1_000 are refused rather than guessed at, because guessing is
// the behaviour this type was written to remove.
type scalar struct {
	// text is the source text of the node with any surrounding quotes taken
	// off. Quotes are a statement about YAML typing, not part of the value.
	text string

	// quoted records that the author wrote quotes. It is what tells "010" the
	// string apart from 010 the bare token, which matters for nothing today and
	// is kept because the difference is free to carry and expensive to recover.
	quoted bool
}

// UnmarshalYAML takes the node as it was written.
//
// goccy hands a BytesUnmarshaler the raw bytes of the node, which is the whole
// reason this works. Measured before the type was designed: a bare 010 arrives
// as "010\n" and a quoted one as "\"010\"", so both the digits and the author's
// intent survive.
func (s *scalar) UnmarshalYAML(b []byte) error {
	t := strings.TrimSpace(string(b))
	if len(t) >= 2 {
		first, last := t[0], t[len(t)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			s.quoted = true
			// A quoted scalar is the one place YAML's own rules apply to the
			// characters inside it, so it is the one place the raw text is the
			// wrong answer. Taking the quotes off by hand leaves a backslash
			// doubled and a \n as two characters, and the encoder quotes
			// exactly the values that need it - so a recipe written by this
			// tool and read back by it disagreed with itself about every
			// Windows path.
			//
			// Found on 2026-08-20 by the guard that puts awkward characters
			// through Compose and Parse and demands they come back unchanged.
			// It fired the moment text fields started arriving here, which is
			// what that guard is for: numbers never carry an escape, so this
			// was invisible until they did.
			//
			// Plain scalars are left exactly as written, which is the whole
			// point of this type. YAML does no escape processing on them, so
			// there is nothing to decode and 010 stays 010.
			var decoded string
			if err := yaml.Unmarshal(b, &decoded); err == nil {
				s.text = decoded
				return nil
			}
			t = t[1 : len(t)-1]
		}
	}
	s.text = t
	return nil
}

// single reports whether the node is one value rather than a list or a block.
//
// The reading path refused those before this type existed and has to keep
// refusing them, or "pages: [1, 2]" would arrive as the literal text of a list
// and be handed to a size parser.
func (s scalar) single() bool {
	if strings.ContainsAny(s.text, "\n\r") {
		return false
	}
	if s.quoted || s.text == "" {
		return true
	}
	switch s.text[0] {
	case '[', '{':
		return false
	case '-':
		// A negative number, unless a space follows - that is a block sequence.
		return len(s.text) > 1 && s.text[1] != ' '
	}
	return true
}

// digits reports whether the text is a plain base ten integer.
//
// Leading zeros are allowed and mean nothing, which is the entire point: 010 is
// ten because that is what it looks like. What is refused is everything that
// only means a number under a rule the reader has to know - a base prefix, a
// digit separator, a decimal point, a stray sign.
func (s scalar) digits() bool {
	t := strings.TrimPrefix(s.text, "-")
	if t == "" {
		return false
	}
	for _, r := range t {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// number reads the value as a whole number in base ten.
//
// The second result says whether it is one. Overflow counts as not one, so a
// seed past the range of the type is refused rather than wrapped into a
// different run than the one asked for.
func (s scalar) number() (int64, bool) {
	if !s.single() || !s.digits() {
		return 0, false
	}
	n, err := strconv.ParseInt(s.text, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// value renders the scalar as the text a --set flag would have carried, so the
// two surfaces cannot disagree about the same recipe.
func (s scalar) value() (string, bool) {
	if !s.single() {
		return "", false
	}
	return s.text, true
}
