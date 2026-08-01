package guard

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/rand/v2"
	"testing"

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

	checked := 0
	for _, d := range descriptors {
		for _, withLabel := range []bool{true, false} {
			for _, ordered := range sizesFor(t, d) {
				name := fmt.Sprintf("%s/label=%v/%d", d.ID, withLabel, ordered)
				t.Run(name, func(t *testing.T) {
					plan, err := d.Generator.Plan(format.Request{
						Bytes: ordered,
						Seed:  uint64(ordered)*31 + 7,
						Label: withLabel,
					})
					if err != nil {
						t.Fatalf("planning %d B failed: %v", ordered, err)
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
					if produced := int64(buf.Len()); produced != ordered {
						t.Fatalf("ordered %d B, produced %d B - the size is exact or it is an error",
							ordered, produced)
					}
				})
				checked++
			}
		}
	}
	if checked == 0 {
		t.Fatal("no size was examined - this guard would pass without checking anything")
	}
}

func TestTheSameSeedGivesTheSameBytes(t *testing.T) {
	checked := 0
	for _, d := range format.All() {
		for _, size := range []int64{d.MinBytes, d.MinBytes + 1024, d.MinBytes + 70000} {
			req := format.Request{Bytes: size, Seed: 7741, Label: true}

			first := produce(t, d, req)
			second := produce(t, d, req)
			if first != second {
				t.Errorf("%s at %d B produced different bytes for the same seed", d.ID, size)
			}

			// A different seed has to give different content, otherwise the
			// seed is decoration and this guard proves nothing.
			other := produce(t, d, format.Request{Bytes: size, Seed: 7742, Label: true})
			if size > 128 && other == first {
				t.Errorf("%s at %d B ignored the seed - two seeds gave identical bytes", d.ID, size)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no format was examined - this guard would pass without checking anything")
	}
}

func produce(t *testing.T, d format.Descriptor, r format.Request) [32]byte {
	t.Helper()
	plan, err := d.Generator.Plan(r)
	if err != nil {
		t.Fatalf("%s: planning %d B failed: %v", d.ID, r.Bytes, err)
	}
	var buf bytes.Buffer
	if err := d.Generator.Write(context.Background(), &buf, plan); err != nil {
		t.Fatalf("%s: writing %d B failed: %v", d.ID, r.Bytes, err)
	}
	return sha256.Sum256(buf.Bytes())
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
