package guard

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// TAR.GZ works its size out by arithmetic, and every other format here works it
// out by measuring what it built.
//
// ZIP counts its own structure with the contents left out, because every part
// of a ZIP lands in the file one for one. TAR.GZ cannot: its bytes pass through
// deflate, so measuring without the contents would answer a different question
// and measuring with them would make --dry-run cost what a run costs. So the
// size comes from a formula over the stored block framing of gzip and the 512
// byte blocks of tar.
//
// A formula can be right for the shape somebody tested and wrong for the next
// one. The property test beside this walks about 120 sizes for every format,
// but it asks for each format with its default settings - one entry of 4 kB for
// this one - so it never varies the two things this arithmetic depends on: how
// many entries there are, and where their sizes fall against 512.
//
// This does. It is the guard for the decision rather than for the format.
func TestTarGzSizeArithmeticSurvivesEveryShape(t *testing.T) {
	d, err := format.Get("targz")
	if err != nil {
		t.Fatalf("targz is not registered: %v", err)
	}

	// Sizes that sit either side of a tar block, and one that crosses a stored
	// block of gzip, because those are the two places the arithmetic steps.
	entrySizes := []string{"0", "1", "511", "512", "513", "8kb", "65535", "65536"}
	// Offsets above the smallest size this shape accepts. The first three are
	// where the comment has to do the work on its own, the last few are where
	// the padding entry appears and the comment closes the remainder.
	offsets := []int64{0, 1, 2, 511, 512, 513, 4095, 4096, 4097, 65534, 65535, 65536, 100000}

	checked, refused := 0, 0
	for _, entries := range []int{0, 1, 3} {
		for _, entrySize := range entrySizes {
			props := map[string]string{
				"entries":    fmt.Sprint(entries),
				"entry_size": entrySize,
			}
			base := format.Request{Seed: 4242, Label: true, Properties: props}
			floor := d.SmallestAccepted(base)

			for _, off := range offsets {
				want := floor + off
				req := base
				req.Bytes = want

				plan, err := d.Generator.Plan(req)
				if err != nil {
					// A refusal is a legitimate answer for a size in a gap, and
					// the guard below makes sure they are not all refusals.
					refused++
					continue
				}
				if plan.Bytes != want {
					t.Errorf("entries=%d entry_size=%s at %d B: the plan says %d B",
						entries, entrySize, want, plan.Bytes)
					continue
				}

				var counter countingWriter
				if err := d.Generator.Write(context.Background(), &counter, plan); err != nil {
					t.Errorf("entries=%d entry_size=%s at %d B: writing failed: %v",
						entries, entrySize, want, err)
					continue
				}
				if counter.n != want {
					t.Errorf("entries=%d entry_size=%s: the plan promised %d B and %d B were written - "+
						"the size arithmetic and the writer disagree",
						entries, entrySize, want, counter.n)
				}
				checked++
			}
		}
	}

	// Without this the whole test passes when every shape is refused, which is
	// the way a guard like this dies quietly.
	if want := len(entrySizes) * len(offsets) * 3; checked < want*9/10 {
		t.Fatalf("only %d of %d shapes produced a file, and %d were refused - "+
			"this guard is not exercising what it claims to", checked, want, refused)
	}
	t.Logf("%d shapes written at an exact size, %d refused", checked, refused)
}

// The one byte above the floor that TAR.GZ cannot reach without a label.
//
// Measured on 2026-08-04 by sweeping every size upward from the floor rather
// than by reasoning from the rule - the same section of the format document
// carries two numbers that were derived from a rule and turned out wrong.
//
// It is a property of the writer rather than of the format: a zero length
// comment with the flag set is legal, costs the one terminating byte, and all
// five readers accept it, but Go leaves the flag off for an empty string. So
// the cheapest comment it will emit costs two.
func TestTarGzHasExactlyOneGapAboveItsFloor(t *testing.T) {
	d, err := format.Get("targz")
	if err != nil {
		t.Fatalf("targz is not registered: %v", err)
	}
	props := map[string]string{"entries": "0"}

	clean := format.Request{Seed: 1, Label: false, Properties: props}
	floor := d.SmallestAccepted(clean)
	if floor != d.MinBytes {
		t.Errorf("with no label the floor should be the structural minimum %d B and it is %d B",
			d.MinBytes, floor)
	}

	var gaps []int64
	for n := floor; n <= floor+64; n++ {
		req := clean
		req.Bytes = n
		if _, err := d.Generator.Plan(req); err != nil {
			gaps = append(gaps, n)
		}
	}
	if len(gaps) != 1 || gaps[0] != floor+1 {
		t.Errorf("without a label the only unreachable size just above %d B should be %d B, and these were refused: %v",
			floor, floor+1, gaps)
	}

	// With the label there is no gap at all, because the comment already costs
	// more than two and grows a byte at a time from there.
	labelled := format.Request{Seed: 1, Label: true, Properties: props}
	from := d.SmallestAccepted(labelled)
	for n := from; n <= from+64; n++ {
		req := labelled
		req.Bytes = n
		if _, err := d.Generator.Plan(req); err != nil {
			t.Errorf("with a label %d B was refused, and every size from %d B upward should work: %v", n, from, err)
			break
		}
	}
}

type countingWriter struct{ n int64 }

func (c *countingWriter) Write(p []byte) (int, error) { c.n += int64(len(p)); return len(p), nil }

var _ io.Writer = (*countingWriter)(nil)
