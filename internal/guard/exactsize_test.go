package guard

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// The size is exact or it is an error. Never a file of a different size than
// the one ordered, and never quietly.
//
// This is stated once and it covers every format and the whole range of
// sizes, which is what a property test is for. Checking a handful of round
// numbers would miss exactly the sizes that break padding arithmetic - the
// ones a byte above or below a boundary.
//
// A note on the fixture trap this deliberately avoids: the assertion compares
// the size that was ORDERED against the size that was PRODUCED, read back
// from the bytes. A helper that conflated the two would pass every broken
// implementation and check nothing at all.

// sizesFor builds the sizes to try for one format - the awkward ones first,
// then a spread of random values.
func sizesFor(t *testing.T, d format.Descriptor) []int64 {
	t.Helper()

	min := d.MinBytes
	sizes := []int64{
		min,
		min + 1,
		min + 2,
		min + 63,
		min + 64,
		min + 65,
		min + 255,
		min + 256,
		min + 257,
		min + 1023,
		min + 1024,
		min + 1025,
		min + 4095,
		min + 4096,
		min + 4097,
		// Around the generator's internal chunk size, where an off by one in
		// the buffering would show up.
		min + 32*1024 - 1,
		min + 32*1024,
		min + 32*1024 + 1,
		min + 100000,
	}

	// Sizes below the declared minimum, so the refusal path is actually
	// walked. Without these the code that refuses could be deleted and every
	// test would still pass.
	for _, below := range []int64{0, 1, min - 1, min - 2, min / 2} {
		if below >= 0 && below < min {
			sizes = append(sizes, below)
		}
	}

	// A deterministic spread, so a failure is reproducible rather than a
	// flake that disappears on the next run.
	rng := rand.New(rand.NewPCG(0x5eed, 0xf11e))
	for i := 0; i < 40; i++ {
		sizes = append(sizes, min+int64(rng.IntN(200000)))
	}
	return sizes
}

func TestEveryFormatHitsTheOrderedSizeExactly(t *testing.T) {
	descriptors := format.All()
	if len(descriptors) == 0 {
		t.Fatal("no format is registered - this guard would pass without checking anything")
	}

	for _, d := range descriptors {
		produced, refused := 0, 0

		for _, withLabel := range []bool{true, false} {
			for _, ordered := range sizesFor(t, d) {
				name := fmt.Sprintf("%s/label=%v/%d", d.ID, withLabel, ordered)
				ok := t.Run(name, func(t *testing.T) {
					plan, err := d.Generator.Plan(format.Request{
						Bytes: ordered,
						Seed:  uint64(ordered)*31 + 7,
						Label: withLabel,
					})
					if err != nil {
						// Refusing is a legitimate answer. Some sizes are
						// genuinely unreachable - PNG cannot be 74 bytes,
						// because the smallest chunk that could make up the
						// difference costs twelve on its own. What is never
						// allowed is handing back a different size in
						// silence, so the refusal has to be the typed error
						// that carries the format, the minimum, the reason
						// and the way out.
						var below *format.BelowMinimumError
						if !errors.As(err, &below) {
							t.Fatalf("refused %d B with %T rather than a BelowMinimumError: %v", ordered, err, err)
						}
						refused++
						return
					}

					if plan.Bytes != ordered {
						t.Fatalf("the plan says %d B for an order of %d B", plan.Bytes, ordered)
					}

					var buf bytes.Buffer
					if err := d.Generator.Write(context.Background(), &buf, plan); err != nil {
						t.Fatalf("writing %d B failed: %v", ordered, err)
					}

					// Ordered against produced. Read back from the bytes, not
					// from anything the generator reported about itself.
					if got := int64(buf.Len()); got != ordered {
						t.Fatalf("ordered %d B, produced %d B - the size is exact or it is an error",
							ordered, got)
					}
					produced++
				})
				_ = ok
			}
		}

		// A generator that refuses everything would satisfy the rule above
		// while being useless, so the counts are checked too.
		if produced == 0 {
			t.Errorf("%s produced no file at all across every size tried", d.ID)
		}
		if refused > produced {
			t.Errorf("%s refused %d sizes and produced %d - refusing is for genuinely unreachable sizes, not the common case",
				d.ID, refused, produced)
		}
		t.Logf("%s: %d sizes produced exactly, %d refused as unreachable", d.ID, produced, refused)
	}
}

func TestTheSameSeedGivesTheSameBytes(t *testing.T) {
	checked := 0
	for _, d := range format.All() {
		for _, size := range []int64{d.MinBytes, d.MinBytes + 4096, d.MinBytes + 70000} {
			req := format.Request{Bytes: size, Seed: 7741, Label: true}

			first, ok := produce(t, d, req)
			if !ok {
				continue
			}
			second, _ := produce(t, d, req)
			if first != second {
				t.Errorf("%s at %d B produced different bytes for the same seed", d.ID, size)
			}

			// A different seed has to give different content, otherwise the
			// seed is decoration and this guard proves nothing.
			//
			// Checked with the label off as well. The label carries the seed
			// in its text, so with it on a generator could ignore the seed
			// everywhere else and still look correct - measured, that is
			// exactly what a mutation of the picture and the padding did.
			for _, label := range []bool{true, false} {
				base, okA := produce(t, d, format.Request{Bytes: size, Seed: 7741, Label: label})
				other, okB := produce(t, d, format.Request{Bytes: size, Seed: 7742, Label: label})
				if !okA || !okB {
					continue
				}
				if size > 256 && other == base {
					t.Errorf("%s at %d B with label=%v ignored the seed - two seeds gave identical bytes",
						d.ID, size, label)
				}
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no format was examined - this guard would pass without checking anything")
	}
}

// produce returns the hash of one generated file, or false when the format
// refuses that size. Refusing is legitimate - the minimum in the registry is
// the smallest file with no label, and a label costs extra.
func produce(t *testing.T, d format.Descriptor, r format.Request) ([32]byte, bool) {
	t.Helper()
	plan, err := d.Generator.Plan(r)
	if err != nil {
		var below *format.BelowMinimumError
		if errors.As(err, &below) {
			return [32]byte{}, false
		}
		t.Fatalf("%s: planning %d B failed: %v", d.ID, r.Bytes, err)
	}
	var buf bytes.Buffer
	if err := d.Generator.Write(context.Background(), &buf, plan); err != nil {
		t.Fatalf("%s: writing %d B failed: %v", d.ID, r.Bytes, err)
	}
	return sha256.Sum256(buf.Bytes()), true
}

func TestSizeBelowTheMinimumIsRefused(t *testing.T) {
	checked := 0
	for _, d := range format.All() {
		if d.MinBytes == 0 {
			// Nothing sits below zero, so there is nothing to refuse. TXT and
			// LOG are legitimately allowed to be empty.
			continue
		}
		_, err := d.Generator.Plan(format.Request{Bytes: d.MinBytes - 1, Seed: 1, Label: true})
		if err == nil {
			t.Errorf("%s accepted %d B while declaring a minimum of %d B",
				d.ID, d.MinBytes-1, d.MinBytes)
			continue
		}
		var below *format.BelowMinimumError
		if !errors.As(err, &below) {
			t.Errorf("%s refused a size below its minimum with %T - it has to be a BelowMinimumError so the message carries the format, the minimum, the reason and the hint", d.ID, err)
		}
		checked++
	}
	_ = checked
}

// And the file that reaches the disk is that size as well.
//
// The guard above hands the generator a bytes.Buffer, which is the right
// question for a generator and the wrong one for the engine: it never touches
// the write path, so nothing between "the generator produced the right bytes"
// and "verify agrees with the manifest" was asked about the file itself.
//
// That gap grew teeth on 2026-08-25, when the write path gained a buffer. The
// engine counts what went into the writer, not what reached the disk, so a
// missing flush leaves a short file and the count still agrees. Measured: with
// the flush taken out this guard fails and the size guard above does not.
//
// Why the buffer is there: measured the same day on the worst shape the format
// declarations allow - a BMP one pixel wide and twenty thousand tall, which is
// twenty thousand calls to Write - sixty of those files took 3.660 s unbuffered
// and 0.138 s buffered, ranges apart, drift anchor 0.2 %. An ordinary 1 MB BMP
// is 1.63x, plain text 1.16x, PNG 1.04x. The bytes are identical in every case,
// so D11 is untouched. docs/CODE-REVIEW-2026-08-23.md section 3.11.
func TestTheFileOnTheDiskIsTheSizeThatWasOrdered(t *testing.T) {
	for _, c := range []struct {
		about string
		args  []string
	}{
		// A shape whose rows are many and small, which is what a buffer is for
		// and what a missing flush loses most of.
		{"a picture of many small rows", []string{
			"--format", "bmp", "--set", "width=1", "--set", "height=2000", "--size", "8058"}},
		{"plain text", []string{"--format", "txt", "--size", "40kb"}},
	} {
		t.Run(c.about, func(t *testing.T) {
			dir := t.TempDir()
			args := append([]string{"generate"}, c.args...)
			args = append(args, "--seed", "7741", "--out", dir)
			if code, _, errOut := run(t, args...); code != cli.ExitOK {
				t.Fatalf("the run gave %d:\n%s", code, errOut)
			}

			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatalf("reading the directory: %v", err)
			}
			checked := 0
			for _, e := range entries {
				if e.Name() == "manifest.json" {
					continue
				}
				info, err := e.Info()
				if err != nil {
					t.Fatalf("looking at %s: %v", e.Name(), err)
				}
				want := wantedSize(t, c.args)
				if info.Size() != want {
					t.Errorf("%s is %d B on the disk and %d B was ordered - the count the engine "+
						"keeps is of what went into the writer, not of what came out of it",
						e.Name(), info.Size(), want)
				}
				checked++
			}
			if checked == 0 {
				t.Fatal("the run produced no file, so this guard would prove nothing")
			}
		})
	}
}

// wantedSize is the number the --size flag of a case asked for.
func wantedSize(t *testing.T, args []string) int64 {
	t.Helper()
	for i, a := range args {
		if a == "--size" && i+1 < len(args) {
			n, err := core.ParseSize(args[i+1])
			if err != nil {
				t.Fatalf("the case asks for %q, which is not a size: %v", args[i+1], err)
			}
			return n
		}
	}
	t.Fatal("the case names no size, so there is nothing to compare against")
	return 0
}
