package guard

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/archive"
)

// A compressed archive is either written at the size it was ordered, or it is
// refused in words about the archive. There is no third answer.
//
// The third answer is what this was written for, and it existed twice.
// Measured 2026-09-02 on the shipped binary, both with exit code 8 and both
// passing --dry-run with exit code 0:
//
//	zip at exactly its own floor, any level but none: no file at all
//	zip holding two 1 MB docx entries, a band 50 B wide: no file at all
//
// The manifest said "generator for zip produced 2607 B where the plan said
// 8382 B", which reads as the tool being broken rather than as the size being
// impossible - and a person cannot act on it.
//
// TestACompressedArchiveStillHitsTheSizeToTheByte was written for this exact
// failure and says so: "a size where the space compression frees is larger than
// the padding can give back". It is green, honestly, and it cannot see either
// of these. It asks three round sizes - 64 KiB, 256 KiB and 1 MiB - with the
// contents left at their default. Round sizes never land on the floor, where
// the first fault lives, and the default contents are text, which deflate
// shrinks, so the second fault cannot happen at all: it needs contents deflate
// GROWS. Naming the right failure is not the same as sampling where it lives.
//
// So this one asks the registry where the floor is for each shape rather than
// carrying a number, and sweeps across it a byte at a time. Both halves have to
// appear in every sweep - some size accepted, some size refused - or the sweep
// straddles nothing and proves nothing.
func TestACompressedZipIsEitherWrittenAtItsSizeOrRefusedInWords(t *testing.T) {
	d, err := format.Get("zip")
	if err != nil {
		t.Fatalf("zip is not registered: %v", err)
	}

	shapes := []struct {
		name  string
		props map[string]string
		below int64 // how far under the floor to start
		above int64 // how far over it to stop
	}{
		// The floor itself, at every level that squeezes. The fault is one
		// byte wide here, so the sweep is narrow and starts under the floor to
		// pick up the refusals that prove it straddles something.
		{"floor, fast", map[string]string{archive.Compression: archive.CompressFast}, 3, 4},
		{"floor, default", map[string]string{archive.Compression: archive.CompressDefault}, 3, 4},
		{"floor, best", map[string]string{archive.Compression: archive.CompressBest}, 3, 4},
		// Contents deflate cannot shrink. A docx is already a compressed
		// container, so a big one comes out of deflate LARGER than it went in -
		// which is the case the arithmetic in this format was written without.
		// The band measured that day was 50 B, so the sweep reaches well past
		// it and into sizes that have to work.
		{"contents deflate grows", map[string]string{
			archive.Compression: archive.CompressBest,
			"entry_format":      "docx",
			"entry_size":        "1mb",
			"entries":           "2",
		}, 5, 250},
	}

	for _, s := range shapes {
		t.Run(s.name, func(t *testing.T) {
			floor := d.SmallestAccepted(format.Request{Label: true, Properties: s.props})
			if floor <= 0 {
				t.Fatalf("the registry reports a floor of %d B for this shape, so there is nothing to sweep", floor)
			}

			var written, refused int
			for size := floor - s.below; size <= floor+s.above; size++ {
				plan, planErr := d.Generator.Plan(format.Request{
					Bytes: size, Seed: 7741, Label: true, Properties: s.props,
				})
				if planErr != nil {
					if !isBelowMinimum(planErr) {
						t.Fatalf("%d B was refused by something other than a minimum: %v", size, planErr)
					}
					refused++
					continue
				}

				var buf bytes.Buffer
				writeErr := d.Generator.Write(context.Background(), &buf, plan)
				if writeErr != nil {
					if !isBelowMinimum(writeErr) {
						t.Errorf("%d B planned and then failed to write with a message that is not about the size: %v\n"+
							"A size the plan accepted and the writer cannot produce has to say what a person should ask for instead.",
							size, writeErr)
						continue
					}
					refused++
					continue
				}
				if int64(buf.Len()) != size {
					t.Errorf("%d B was planned and produced %d B, with no error at all - "+
						"the archive is the wrong length and nothing says so",
						size, buf.Len())
					continue
				}
				written++
			}

			// Both halves, or the sweep sat entirely on one side of the floor
			// and the run above asked nothing.
			if written == 0 {
				t.Errorf("no size in this sweep produced an archive, so it proves nothing about the ones that should work")
			}
			if refused == 0 {
				t.Errorf("no size in this sweep was refused, so the sweep never reached the floor it is aimed at")
			}
			t.Logf("floor %d B: %d written, %d refused", floor, written, refused)
		})
	}
}

// isBelowMinimum is the one refusal a size sweep is allowed to meet.
//
// Asked by type rather than by reading the sentence, because the sentence is
// written for a person and this is the part a script would act on.
func isBelowMinimum(err error) bool {
	var below *format.BelowMinimumError
	return errors.As(err, &below)
}
