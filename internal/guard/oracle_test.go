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

// Our own tests are written by whoever wrote the generator, so they cannot be
// the only judge of whether a file is correct. An independent implementation
// can.
//
// This exists because mutation found the gap: removing the alignment padding
// from a RIFF chunk changed the bytes, kept the size exact, kept the run
// repeatable - and every guard stayed green. Size and determinism say nothing
// about whether the file is well formed.
//
// A missing tool skips loudly. A quietly skipped oracle is a green run that
// checked nothing.

func TestEveryFormatSurvivesItsReferenceTool(t *testing.T) {
	dir := t.TempDir()

	var (
		checked int
		skipped []string
		noTool  []string
	)

	for _, d := range format.All() {
		if d.Oracle == format.OracleNone {
			noTool = append(noTool, d.ID)
			continue
		}

		checker, known := oracle.For(d.Oracle)
		if !known {
			t.Errorf("%s declares the oracle %q and nothing implements it - a declaration nobody honours is worse than none",
				d.ID, d.Oracle)
			continue
		}

		// A size big enough to be a realistic file rather than a corner case.
		size := d.MinBytes + 300*1024
		plan, err := d.Generator.Plan(format.Request{Bytes: size, Seed: 7741, Label: true})
		if err != nil {
			t.Fatalf("%s: planning %d B failed: %v", d.ID, size, err)
		}

		path := filepath.Join(dir, "sample"+d.Extension)
		f, err := os.Create(path)
		if err != nil {
			t.Fatalf("creating %s: %v", path, err)
		}
		err = d.Generator.Write(context.Background(), f, plan)
		closeErr := f.Close()
		if err != nil {
			t.Fatalf("%s: writing failed: %v", d.ID, err)
		}
		if closeErr != nil {
			t.Fatalf("%s: closing failed: %v", d.ID, closeErr)
		}

		res := checker.Check(path)
		switch {
		case !res.Available:
			skipped = append(skipped, d.ID+" ("+checker.Name+" is not installed)")
		case res.Err != nil:
			checked++
			t.Errorf("%s: %v\n  the file is the right size and repeatable, and %s still rejects it",
				d.ID, res.Err, checker.Name)
		default:
			checked++
			t.Logf("%s: %s accepted it - %s", d.ID, checker.Name, firstLine(res.Output))
		}

		// The tolerant readers answer "would a viewer accept this". The
		// structural check answers "is it well formed", and measurement showed
		// those are different questions - a reader ignores a wrong size in the
		// RIFF header, an unverified chunk checksum and a cross reference
		// offset that is off by one.
		if !oracle.StrictKnows(d.ID) {
			continue
		}
		strict := oracle.Strict(d.ID, path)
		if !strict.Available {
			skipped = append(skipped, d.ID+" (the structural check needs python)")
			continue
		}
		if strict.Err != nil {
			t.Errorf("%s is not well formed: %v", d.ID, strict.Err)
			continue
		}
		t.Logf("%s: structurally sound - %s", d.ID, firstLine(strict.Output))
	}

	// The report at the end, so a run that checked almost nothing cannot look
	// like a run that checked everything.
	t.Logf("reference tools: %d format(s) checked, %d skipped, %d declare no tool",
		checked, len(skipped), len(noTool))
	for _, s := range skipped {
		t.Logf("  SKIPPED: %s", s)
	}
	for _, n := range noTool {
		t.Logf("  no tool declared: %s", n)
	}

	if checked == 0 && len(skipped) > 0 {
		t.Logf("NOTHING was verified against an external tool on this machine")
	}
}

func firstLine(s string) string {
	for i, r := range s {
		if r == '\n' || r == '\r' {
			return s[:i]
		}
	}
	if len(s) > 120 {
		return s[:120]
	}
	return s
}
