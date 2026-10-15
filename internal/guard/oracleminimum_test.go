package guard

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/oracle"
)

// The smallest file a format will make is the one most likely to be wrong, and
// it was the one nothing independent ever looked at.
//
// TestEveryFormatSurvivesItsReferenceTool builds exactly one file per format,
// at MinBytes plus 300 KB, and that is the only place an outside
// implementation sees our output. The size guard walks about 120 sizes and
// only ever counts bytes. So a file that is the right size, repeatable and
// malformed only at the bottom of the range passes everything this project
// has.
//
// It did. Measured on 2026-08-03 with tools/probes/fidelity-sweep.py: SVG at
// exactly its declared minimum of 193 B rendered to a single flat colour -
// nothing painted at all - because at that size the padding channel is empty
// and the padding channel is the text of the label, so the document held one
// empty text element and no shapes. One byte more and it drew. The project has
// a renderer for exactly this failure and never pointed it at that size.
//
// Both ends are checked here rather than only the small one. A format that
// refuses everything except its one tested size would satisfy a guard that
// only knew about the minimum.
//
// One subtest per format, and a format whose tool is missing SKIPS rather than
// counting itself into a number in the log. Until 2026-10-15 a missing tool was
// that number and nothing else, so the guard was green whether it had put SVG
// past Inkscape or not - and the mutation run read green as caught: it scored
// the empty drawing at 193 B as a HOLE on the runner with no Inkscape and on a
// machine whose python has no Pillow. A skip says where nothing was asked.
func TestEveryFormatSurvivesItsReferenceToolAtItsSmallest(t *testing.T) {
	dir := t.TempDir()
	checked, skipped := 0, 0

	for _, d := range format.All() {
		if d.Oracle == format.OracleNone {
			continue
		}
		checker, known := oracle.For(d.Oracle)
		if !known {
			t.Errorf("%s declares the oracle %q and nothing implements it", d.ID, d.Oracle)
			continue
		}

		t.Run(d.ID, func(t *testing.T) {
			// The label costs bytes, so the smallest file with one is not the
			// smallest file. Both are produced, because a reader meets both.
			for _, label := range []bool{true, false} {
				for _, size := range []int64{d.MinBytes, d.MinBytes + 1, d.MinBytes + 2} {
					plan, err := d.Generator.Plan(format.Request{Bytes: size, Seed: 7741, Label: label})
					if err != nil {
						// Genuinely unreachable sizes are a legitimate answer and
						// the size guard already holds them to being typed
						// refusals. Nothing to check when no file exists.
						continue
					}

					path := filepath.Join(dir, "small"+d.Extension)
					f, createErr := os.Create(path)
					if createErr != nil {
						t.Fatalf("creating %s: %v", path, createErr)
					}
					writeErr := d.Generator.Write(context.Background(), f, plan)
					closeErr := f.Close()
					if writeErr != nil || closeErr != nil {
						t.Fatalf("%s at %d B: writing failed: %v %v", d.ID, size, writeErr, closeErr)
					}

					res := checker.Check(path)
					if !res.Available {
						skipped++
						t.Skipf("%s is not on this machine, so no %s was put past it", checker.Name, d.ID)
					}
					checked++
					if res.Err != nil {
						t.Errorf("%s at %d B with label=%v: %v\n  the file is the right size and repeatable, and %s still rejects it",
							d.ID, size, label, res.Err, checker.Name)
					}
					os.Remove(path)
				}
			}
		})
	}

	t.Logf("smallest files put past a reference tool: %d checked, %d format(s) skipped for a missing tool", checked, skipped)
	if checked == 0 && skipped > 0 {
		t.Log("NOTHING was verified against an external tool on this machine")
	}
}
