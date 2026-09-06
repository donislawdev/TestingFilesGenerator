package guard

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// A recipe somebody else wrote cannot exhaust this machine.
//
// The comment on recipe.MaxBytes used to say the size limit took care of that:
// "A megabyte caps the worst case at seconds rather than minutes". Measured on
// 2026-09-02 it does not. A document of 40 kB nesting flow collections twenty
// thousand deep took 918 MB of heap, and 40 kB is a twenty fifth of what the
// size limit allows - so the limit that was supposed to bound the work buys as
// much memory as the machine will give.
//
// The legal halves are not decoration, and one of them is the reason the
// defence is shaped the way it is. Counting brackets in the raw bytes - the
// first thing anybody reaches for - refuses a value that merely CONTAINS
// brackets, and that is a legal document. The lexer has already told those two
// apart, so it is asked instead of the bytes.
//
// What is deliberately NOT guarded here: a second shape that looked like it
// never finished, and did not exist. Chaining two thousand bracket pairs reads
// in 0.19 s. What hung was the harness measuring it, which wrote a 4 kB refusal
// into a pipe nothing was reading - and the pipe holds 4096 bytes, which is
// exactly where the apparent cliff sat. A budget on the reading was written for
// it and taken back out, because nothing could have made it fire.
func TestAHostileRecipeCannotHangTheReader(t *testing.T) {
	const tail = "\ntargets:\n  - id: a\n    format: txt\n    size: 10\n"

	t.Run("nesting is refused by depth", func(t *testing.T) {
		src := []byte("version: 1\npolicy:\n  x: " +
			strings.Repeat("[", 20000) + strings.Repeat("]", 20000) + tail)
		_, err := recipe.Parse(src, "bomb.yaml")
		var deep *recipe.TooDeepError
		if !errors.As(err, &deep) {
			t.Fatalf("a recipe nesting 20000 deep was answered with %v, expected a refusal about its depth", err)
		}
	})

	t.Run("a legal recipe at nearly the size limit is read", func(t *testing.T) {
		// Twenty thousand targets comes to about 949 kB, just under MaxBytes,
		// and is the slowest legal read there can be - 1.02 s when it was
		// measured. A defence that ever came near this would be refusing
		// honest work.
		var b strings.Builder
		b.WriteString("version: 1\nseed: 7\noutput:\n  dir: out\ntargets:\n")
		for i := 0; i < 20000; i++ {
			b.WriteString("  - id: t")
			b.WriteString(strconv.Itoa(i))
			b.WriteString("\n    format: txt\n    size: 100\n")
		}
		if _, err := recipe.Parse([]byte(b.String()), "big.yaml"); err != nil {
			t.Fatalf("a legal recipe of %d B was refused: %v", b.Len(), err)
		}
	})

	t.Run("a value in flow style is not a bomb", func(t *testing.T) {
		// A thousand targets written with braces reaches depth one, the same
		// as spread: [1B, 1kb]. Anything counting markers rather than nesting
		// would see two thousand of them and refuse this.
		var b strings.Builder
		b.WriteString("version: 1\nseed: 7\noutput:\n  dir: out\ntargets:\n")
		for i := 0; i < 1000; i++ {
			b.WriteString("  - {id: t")
			b.WriteString(strconv.Itoa(i))
			b.WriteString(", format: txt, size: 100}\n")
		}
		if _, err := recipe.Parse([]byte(b.String()), "flow.yaml"); err != nil {
			t.Fatalf("a legal recipe written in flow style was refused: %v", err)
		}
	})

	t.Run("brackets inside a value are not collections", func(t *testing.T) {
		src := []byte("version: 1\nseed: 7\noutput:\n  dir: out\ntargets:\n" +
			"  - id: a\n    format: txt\n    size: 100\n    name: \"" +
			strings.Repeat("[", 20000) + strings.Repeat("]", 20000) + ".txt\"\n")
		_, err := recipe.Parse(src, "brackets.yaml")
		var deep *recipe.TooDeepError
		if errors.As(err, &deep) {
			t.Fatalf("a value holding brackets was refused as if they nested: %v", err)
		}
	})

	t.Run("the same nesting written in block style is refused too", func(t *testing.T) {
		// "- - - - x" is four nested sequences in eight bytes and holds not one
		// bracket, so the limit that counted brackets walked straight past it.
		// Measured on 2026-09-06 against the shipped binary, before this was
		// counted: 500 kB of it took 88 seconds and ended in "fatal error: out
		// of memory" with 41 kB of Go stack on standard error, and 1 MB - the
		// most the size limit allows - left with exit 1 and no output at all.
		// After: 756 ms and this refusal.
		src := []byte(strings.Repeat("- ", 20000) + "x")
		_, err := recipe.Parse(src, "blockbomb.yaml")
		var deep *recipe.TooDeepError
		if !errors.As(err, &deep) {
			t.Fatalf("a block style bomb was answered with %v, expected a refusal about its depth", err)
		}
	})

	t.Run("a long list at one level is not nesting", func(t *testing.T) {
		// Every dash in the same column is the next entry of one sequence
		// rather than a sequence inside the last one. Anything counting dashes
		// instead of nesting refuses this at the thirty third target, which is
		// an ordinary recipe.
		var b strings.Builder
		b.WriteString("version: 1\nseed: 7\noutput:\n  dir: out\ntargets:\n")
		for i := 0; i < 200; i++ {
			b.WriteString("  - id: t")
			b.WriteString(strconv.Itoa(i))
			b.WriteString("\n    format: txt\n    size: 100\n")
		}
		if _, err := recipe.Parse([]byte(b.String()), "long.yaml"); err != nil {
			t.Fatalf("a recipe with 200 targets in one list was refused: %v", err)
		}
	})

	t.Run("a list inside a list is two, not two hundred", func(t *testing.T) {
		// The deepest shape this tool actually produces: targets, and an
		// archive declaring what it contains. Two levels, whatever the length
		// of either list.
		var b strings.Builder
		b.WriteString("version: 1\nseed: 7\noutput:\n  dir: out\ntargets:\n")
		for i := 0; i < 50; i++ {
			b.WriteString("  - id: a")
			b.WriteString(strconv.Itoa(i))
			b.WriteString("\n    format: zip\n    contains:\n")
			for j := 0; j < 20; j++ {
				b.WriteString("      - format: txt\n        count: 1\n        size: 8kb\n")
			}
		}
		if _, err := recipe.Parse([]byte(b.String()), "nested.yaml"); err != nil {
			t.Fatalf("an archive declaring its contents was refused: %v", err)
		}
	})
}
