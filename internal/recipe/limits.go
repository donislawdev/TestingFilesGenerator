// Part of package recipe. See recipe.go.
package recipe

import (
	"fmt"

	"github.com/goccy/go-yaml/lexer"
	"github.com/goccy/go-yaml/token"
)

// What a recipe is allowed to cost to read, beyond its size.
//
// MaxBytes bounds the input and its comment used to claim that bounded the
// work: "A megabyte caps the worst case at seconds rather than minutes". That
// was measured on 2026-08-02 and it is not true. Measured again on 2026-09-02,
// on this build: a document of 40 kB nesting flow collections twenty thousand
// deep took 918 MB of heap, and it would have taken more had the machine had
// more. Forty kilobytes is a twenty fifth of what the size limit allows.
//
// Reading time is not the problem and one measurement here was WRONG before it
// was checked. A chain of two thousand bracket pairs looked like it never
// finished, and a budget on the reading was written to answer it. It finishes
// in 0.19 s. What hung was the harness measuring it: the tool wrote a 4 kB
// refusal into a pipe nothing was reading, and the pipe holds 4096 bytes. The
// apparent cliff sat exactly there - 1900 pairs produce a 4010 B message and
// pass, 1950 produce 4110 B and blocked. The budget was taken back out, since
// nothing could have made it fire.

const (
	// MaxFlowDepth is how far flow collections - the ones written with
	// brackets and braces - may nest.
	//
	// This is the deterministic half, and it is deterministic because the
	// lexer has already decided what is a bracket and what is a character
	// inside a quoted value. Measured across shapes on 2026-09-02:
	//
	//	block style, a thousand targets           0
	//	forty thousand brackets inside a string   0
	//	spread: [1B, 1kb, 1mb]                    1
	//	a thousand targets written in flow style  1
	//	the nesting bomb                      20000
	//
	// So one is what real recipes reach and thirty two is far above anything
	// a person writes. Counting rather than guessing also rules out the two
	// obvious mistakes: a bracket inside a quoted value is not a collection,
	// and neither is one inside a comment.
	MaxFlowDepth = 32
)

// TooDeepError is returned for a recipe whose flow collections nest past
// MaxFlowDepth.
type TooDeepError struct {
	Name  string
	Depth int
}

func (e *TooDeepError) Error() string {
	return fmt.Sprintf(
		"%s nests brackets and braces %d deep and the limit is %d. Reading a deeply nested document costs memory that grows far faster than the document does, so a small file can exhaust this machine before anything is written. Write the targets out as an ordinary list instead",
		e.Name, e.Depth, MaxFlowDepth)
}

// flowDepth is the deepest the flow collections in src nest.
//
// The lexer is asked rather than the bytes, and that is the whole point of
// doing it this way: it has already decided which brackets open a collection
// and which are characters inside a quoted value or a comment. Measured on
// 2026-09-02, forty thousand brackets inside one quoted value come back as
// depth nought, which a scan over the raw bytes could only manage by
// reimplementing the quoting rules.
//
// Cheap enough to run on every recipe: 0.002 s for a forty kilobyte document,
// 0.008 s for the bomb, and the cost grows with the input rather than with the
// shape - which is exactly what the parser underneath does not do.
func flowDepth(src []byte) int {
	current, deepest := 0, 0
	for _, t := range lexer.Tokenize(string(src)) {
		current += depthChange(t.Type)
		if current > deepest {
			deepest = current
		}
	}
	return deepest
}

// depthChange is what one token does to the nesting: a collection opening adds
// a level, one closing takes it away, and anything else leaves it where it was.
//
// A function of its own rather than a switch inside the loop above, because
// together they nested three deep - the loop, the switch, the comparison - and
// the shape guard counts how many functions sit that deep as well as how deep
// the deepest one is. Splitting is what that guard asks for and it costs
// nothing here.
// The default is not decoration. token.Type has thirty four members and a
// switch on it without one is reported as incomplete, which is correct of the
// linter and wrong about this function: everything that is not a bracket or a
// brace leaves the nesting where it was, and listing thirty of them would say
// less than one line saying so.
func depthChange(t token.Type) int {
	switch t {
	case token.SequenceStartType, token.MappingStartType:
		return 1
	case token.SequenceEndType, token.MappingEndType:
		return -1
	default:
		return 0
	}
}
