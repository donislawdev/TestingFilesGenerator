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
//
// THE FIRST VERSION OF THIS LIMIT COUNTED THE WRONG THING, and that cost more
// than the bomb it was written for. It counted flow collections - the ones
// written with brackets and braces - because that is how the bomb of
// 2026-09-02 was written. The same nesting written in block style walks past
// it: "- - - - x" is four nested sequences in eight bytes, and the lexer calls
// none of them a bracket. Measured on 2026-09-06 against the shipped binary,
// with tfg validate:
//
//	"- " x20 000     40 kB    0.40 s   refused, by a limit inside the parser
//	"- " x100 000   200 kB    7.41 s   refused
//	"- " x250 000   500 kB   88.2 s    fatal error: out of memory, exit 2
//	"- " x500 000     1 MB   70.7 s    exit 1, and not one word of output
//
// The third of those printed 41 kB of Go runtime stack to standard error and
// left with the exit code the frozen table gives a MISTYPED FLAG. The machine
// it did that on has 31.9 GB.
//
// THE COST IS THE NESTING, NOT THE SIZE, and that is measured rather than
// assumed: 500 kB of flat mapping - a hundred thousand lines of "a: 1" - is
// refused cleanly in 0.62 s. So a limit on bytes could never have answered
// this, and neither could one on how many tokens a document holds.
const (
	// MaxNestingDepth is how far collections may nest, in any style.
	//
	// This is the deterministic half, and it is deterministic because the lexer
	// has already decided what opens a collection and what is a character
	// inside a quoted value. Measured across shapes on 2026-09-02 and again on
	// 2026-09-06 with tools/probes/yamldepth:
	//
	//	block style, a thousand targets           0
	//	forty thousand brackets inside a string   0
	//	spread: [1B, 1kb, 1mb]                    1
	//	a thousand targets written in flow style  1
	//	an ordinary recipe                        1
	//	the flow bomb                         20000
	//	the block bomb                       500000
	//
	// So one is what real recipes reach and thirty two is far above anything a
	// person writes. Counting rather than guessing also rules out the two
	// obvious mistakes: a bracket inside a quoted value is not a collection,
	// and neither is one inside a comment.
	MaxNestingDepth = 32
)

// TooDeepError is returned for a recipe whose collections nest past
// MaxNestingDepth.
type TooDeepError struct {
	Name  string
	Depth int
}

func (e *TooDeepError) Error() string {
	return fmt.Sprintf(
		"%s nests lists and mappings %d deep and the limit is %d. Reading a deeply nested document costs memory that grows far faster than the document does, so a small file can exhaust this machine before anything is written. Write the targets out as an ordinary list instead",
		e.Name, e.Depth, MaxNestingDepth)
}

// nestingDepth is the deepest the collections in src nest, counting both
// styles.
//
// The lexer is asked rather than the bytes, and that is the whole point of
// doing it this way: it has already decided which brackets open a collection
// and which are characters inside a quoted value or a comment. Measured on
// 2026-09-02, forty thousand brackets inside one quoted value come back as
// depth nought, which a scan over the raw bytes could only manage by
// reimplementing the quoting rules.
//
// Cheap enough to run on every recipe, and this is the measurement the whole
// defence rests on, because it has to happen BEFORE the parser sees anything.
// Measured on 2026-09-06 with tools/probes/yamldepth, on the block bomb:
//
//	 20 000 levels    40 kB      7 ms     5.5 MB
//	100 000 levels   200 kB     42 ms    28.9 MB
//	500 000 levels     1 MB    422 ms   145.2 MB
//
// The last line is the worst input the size limit allows, and it is what the
// parser answered with 70 seconds and no output at all. The memory is the
// tokeniser's own and it is bounded by MaxBytes, which is the same bargain this
// function has always made - it is not new work, only work that now counts one
// more thing.
func nestingDepth(src []byte) int {
	flow, deepest := 0, 0

	// The columns of the block sequences that are open. A dash further right
	// than the innermost open one starts a sequence inside it, a dash at the
	// same column is the next entry of that one, and a dash further left closes
	// however many it has come back out of. That is the whole of block nesting
	// as far as this needs to know.
	var dashes []int

	for _, t := range lexer.Tokenize(string(src)) {
		if t.Type == token.SequenceEntryType && flow == 0 && t.Position != nil {
			dashes = openSequences(dashes, t.Position.Column)
		} else {
			flow += depthChange(t.Type)
		}
		if d := flow + len(dashes); d > deepest {
			deepest = d
		}
	}
	return deepest
}

// openSequences is the block sequences still open after a dash at this column.
//
// A function of its own rather than the body of the loop above, for the reason
// written beside depthChange: together they nest three deep - the loop, the
// branch, the pop - and the shape guard counts how many functions sit that deep
// as well as how deep the deepest one is.
//
// WHAT THIS OVER-COUNTS, said out loud because a limit that hides its edges is
// worse than none. Two sequences that are siblings under different keys, the
// second indented further than the first, are read as nested:
//
//	a:
//	  - x
//	b:
//	    - y      counted as depth two, and it is two sequences at depth one
//
// Reaching the limit that way needs thirty two keys each indented further than
// the one before, in one document, which is not a shape anybody writes and is
// not what our own canonical form produces. The alternative is to track where
// mappings open as well, which is more machinery for a case that costs a
// refusal naming the file rather than a wrong answer.
func openSequences(open []int, column int) []int {
	for len(open) > 0 && open[len(open)-1] > column {
		open = open[:len(open)-1]
	}
	if len(open) == 0 || open[len(open)-1] < column {
		open = append(open, column)
	}
	return open
}

// depthChange is what one token does to the flow nesting: a collection opening
// adds a level, one closing takes it away, and anything else leaves it where it
// was.
//
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
