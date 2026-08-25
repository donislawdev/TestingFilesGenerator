package guard

import (
	"testing"

	"fyne.io/fyne/v2"

	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// A box holding a size says what that size comes to, and follows what is typed.
//
// "10mb" is two numbers depending on the reader. This tool counts in 1024s -
// RECIPE.md section 9, settled and not reopened - and until 2026-08-24 the only
// place on any screen that said so was the sentence behind the little i, which
// gave the answer for the example 10mb rather than for whatever was in the box,
// and only to somebody who clicked.
//
// The count is asserted against core.ParseSize rather than against a number
// written here, so this cannot come to disagree with the files. The one number
// spelled out is 1024, and it is spelled out on purpose: a guard that computed
// the expected value the same way the code does would pass just as happily with
// both of them counting in thousands, which is the decision this whole thing
// exists to make visible.
func TestABoxHoldingASizeSaysWhatItComesTo(t *testing.T) {
	_, content := screen(t)

	count := byteCountBeside(t, content, text.FieldSize())
	if count.Text == "" {
		t.Fatal("the size box opens with 10mb in it and nothing says what that comes to")
	}

	// The size the box opens with, in the units this tool uses.
	if want := text.ExactBytes(10 * 1024 * 1024); count.Text != want {
		t.Errorf("the size box holds 10mb and the count beside it reads %q, not %q.\n"+
			"Reason: a megabyte is 1024 kilobytes here, and the count is the only place the screen says so",
			count.Text, want)
	}

	// It follows the box rather than being set once. A count that stopped at
	// the opening value would be worse than none: it would be a wrong number
	// beside a right one.
	fill(t, content, text.FieldSize(), "1kb")
	if want := text.ExactBytes(1024); count.Text != want {
		t.Errorf("1kb was typed and the count beside the box reads %q, not %q", count.Text, want)
	}

	// And says nothing at all about something that is not a size, which is
	// every box somebody is halfway through typing.
	fill(t, content, text.FieldSize(), "abc")
	if count.Text != "" {
		t.Errorf("the size box holds something that is not a size and the count reads %q, "+
			"so it is showing a number for a value the run will refuse", count.Text)
	}
}

// The count goes beside sizes and nowhere else.
//
// The obvious way to build this was to try parsing every box and show a count
// wherever it worked. "How many" holds 1, which parses as one byte, so every
// count box on every screen would have grown a caption reading "1 B" - a number
// that is true about nothing anybody asked.
//
// So it is declared, and this is what holds the declaration to something: the
// boxes that carry a count are exactly the ones whose value is one size.
func TestOnlyABoxHoldingASizeCarriesACount(t *testing.T) {
	_, content := screen(t)

	for _, label := range []string{text.FieldCount(), text.FieldSeed(), text.FieldTargetID(), text.FieldNameTemplate()} {
		if count := byteCountIn(fieldBox(content, label)); count != nil {
			t.Errorf("%q does not hold a size and carries a count reading %q", label, count.Text)
		}
	}
	// And the one that does still has it, so this cannot pass by there being no
	// counts anywhere.
	if byteCountIn(fieldBox(content, text.FieldSize())) == nil {
		t.Errorf("%q holds a size and carries no count, so the check above proved nothing", text.FieldSize())
	}
}

func byteCountBeside(t *testing.T, o fyne.CanvasObject, label string) *parts.ByteCount {
	t.Helper()
	count := byteCountIn(fieldBox(o, label))
	if count == nil {
		t.Fatalf("there is no count of bytes beside %q", label)
	}
	return count
}

func byteCountIn(o fyne.CanvasObject) *parts.ByteCount {
	if o == nil {
		return nil
	}
	var found *parts.ByteCount
	walk(o, func(obj fyne.CanvasObject) {
		if c, is := obj.(*parts.ByteCount); is && found == nil {
			found = c
		}
	})
	return found
}

// A declared setting that holds a size says what it comes to, on every screen
// that draws declared settings.
//
// Three screens draw them, not two. A preset declares its parameters as the
// same format.Property a format declares its settings as, and until 2026-08-24
// the preset screen drew them with a loop of its own - so it was the third
// place and the one nobody remembered. It had already fallen behind twice, and
// the count of bytes was the second time: it had to be written into that loop
// as well as into the shared one on the day it was added.
//
// Both kinds are asked about here because both exist. zip and targz declare
// entry_size, and the one preset this build registers declares limit, so a
// change that reached only formats or only presets fails on the other.
func TestADeclaredSizeSaysWhatItComesToOnEveryScreenThatDrawsOne(t *testing.T) {
	t.Run("a format setting", func(t *testing.T) {
		_, content := screen(t)
		picker, ok := controlUnder(content, text.FieldFormat()).(*parts.Chooser)
		if !ok {
			t.Fatal("this screen has no format list, so this guard read the wrong tree")
		}
		// A format that declares a size, so "does a size say what it comes to"
		// is a question this can ask at all.
		picker.SetSelected("zip")

		label := text.SettingLabel("entry_size")
		count := byteCountBeside(t, content, label)
		fill(t, content, label, "4kb")
		if want := text.ExactBytes(4 * 1024); count.Text != want {
			t.Errorf("%q holds 4kb and the count beside it reads %q, not %q", label, count.Text, want)
		}
	})

	t.Run("a preset parameter", func(t *testing.T) {
		_, content := presetScreen(t)

		label := text.SettingLabel("limit")
		count := byteCountBeside(t, content, label)
		// It arrives empty and has to: a preset parameter nobody stated is what
		// the manifest records as defaulted, untouchable rule 5. So there is
		// nothing to count until somebody types, and saying otherwise would be
		// a number for a value nobody gave.
		if count.Text != "" {
			t.Errorf("%q was left alone and the count beside it reads %q, so it is counting a value nobody stated",
				label, count.Text)
		}

		fill(t, content, label, "2mb")
		if want := text.ExactBytes(2 * 1024 * 1024); count.Text != want {
			t.Errorf("%q holds 2mb and the count beside it reads %q, not %q", label, count.Text, want)
		}
	})
}
