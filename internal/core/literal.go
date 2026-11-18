package core

import "strings"

// A value somebody types into a box exactly as it is written - a group name, a
// placeholder in a text, a colour, a word a setting takes - stands between two
// backticks in the description of a setting: "Leave it at `all`". Each surface
// shows the marks in its own way, and the text is written once:
//
//   - the window draws the value in the type of code, in a colour of its own
//     (parts.Explanation), and the backticks go.
//   - the site wraps it in <code>.
//   - the command line prints the backticks as they are, which is how the help
//     of a command marks something to type.
//
// The owner's decision of 2026-11-18, after reading the explanation of the
// groups of the picture sizes preset in the window: screens, avatars, social
// and all stood among the words describing them, and nothing said which of
// them went into the box (docs/SIZE-FIXES-2026-11-16.md section 3.9).
//
// A translation keeps every value between the marks exactly as the English has
// it, since what is typed is the same in every language -
// TestAValueToTypeIsTheSameInEveryLanguage holds every catalogue to that.

// Backtick is the mark on either side of a value to type.
const Backtick = "`"

// Piece is one run of a description: a value to type, or the words round it.
type Piece struct {
	Text    string
	Literal bool
}

// Pieces splits a description at its marks, in order, leaving out the empty
// runs. A mark without a partner is kept as it is written, in the words: an
// odd one is a backtick somebody meant, not the start of a value.
func Pieces(s string) []Piece {
	parts := strings.Split(s, Backtick)
	if len(parts)%2 == 0 {
		// An odd number of marks: the last one has no partner, so it and
		// everything after it are words.
		last := len(parts) - 2
		parts = append(parts[:last:last], parts[last]+Backtick+parts[last+1])
	}
	out := make([]Piece, 0, len(parts))
	for i, p := range parts {
		if p == "" {
			continue
		}
		out = append(out, Piece{Text: p, Literal: i%2 == 1})
	}
	return out
}

// Literals is every value to type in a description, in order.
func Literals(s string) []string {
	var out []string
	for _, p := range Pieces(s) {
		if p.Literal {
			out = append(out, p.Text)
		}
	}
	return out
}
