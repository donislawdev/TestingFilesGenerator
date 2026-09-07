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

		// The first layer, for the formats that name a reader.
		//
		// A format naming none used to skip the REST of this loop along with
		// it, so the structural check below never ran for it. That sat unseen
		// while the only two formats without a reader were also the only two
		// without a checker - and it would have made the checkers TXT and MD
		// gained on 2026-09-07 dead on arrival, silently, with this guard
		// green and reporting them as covered. Found by running it and reading
		// the log, not by reading the code.
		switch checker, known := oracle.For(d.Oracle); {
		case d.Oracle == format.OracleNone:
			noTool = append(noTool, d.ID)
		case !known:
			t.Errorf("%s declares the oracle %q and nothing implements it - a declaration nobody honours is worse than none",
				d.ID, d.Oracle)
		default:
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

// structurallyChecked is the formats the second layer covers, written down.
//
// TXT and MD were absent on purpose until 2026-09-07, and the reason they gave
// is worth keeping because it stopped being true rather than being wrong: for
// those two there was no specification to check against beyond "these are the
// bytes we meant", so they carried one layer and said so.
//
// Declaring an encoding is what changed it. A file that says it is UTF-16LE
// with a mark in front of it makes a claim somebody else's decoder can settle,
// and three of them settled it - Python, V8 and .NET all reject a UTF-16 file
// cut to an odd length, and all three repair it in silence when asked
// leniently. So the list still exists rather than being derived, and it is now
// every format answering true.
//
// Without this, dropping a format from oracle.StrictKnows removes its
// structural check and every test stays green - the loop above simply skips it.
// A guard that can be switched off in silence is the failure this project keeps
// finding, so the list is stated and compared rather than trusted.
// Every registered format is named here. The map keeps its shape rather than
// becoming a list, because a format that answers false is a state this project
// has been in and can be in again - a new format arrives before its checker
// does, and saying so out loud is the whole job of this list.
//
// It held only the trues until 2026-08-25, and an outside review found what
// that let through: a format added to neither this list nor oracle.StrictKnows
// answers false on both sides, agrees with itself, and is never checked
// structurally - in silence. The drift between the two lists was guarded, the
// absence from both was not.
//
// Three Tier 1 formats are still to come and all three are binary, which is where
// the structural check earns most: at JPG it caught bytes after EOI that Pillow
// read without complaint, and at TIFF it is the only layer that sees a
// directory lying about how many bytes of pixels there are - measured
// 2026-08-29 on five readers, four of which accepted that file.
var structurallyChecked = map[string]bool{
	"png": true, "wav": true, "pdf": true, "zip": true, "targz": true,
	"log": true, "csv": true, "json": true, "xml": true, "svg": true, "html": true,
	"bmp": true, "gif": true, "ico": true, "jpg": true, "tiff": true, "webp": true,
	"avif": true, "jxl": true,
	"docx": true, "xlsx": true, "pptx": true,
	// Since 2026-09-07, when a text file gained something to be checked
	// against: the encoding it declares.
	"txt": true, "md": true,
}

func TestTheStructuralCheckerCoversEveryFormatItShould(t *testing.T) {
	descriptors := format.All()
	if len(descriptors) == 0 {
		t.Fatal("no format is registered - this guard would pass without checking anything")
	}
	for _, d := range descriptors {
		want, stated := structurallyChecked[d.ID]
		if !stated {
			t.Errorf("%s is registered and this list says nothing about it - decide whether it has a "+
				"structural check and write the answer here. Left out it is never checked and nothing says so", d.ID)
			continue
		}
		got := oracle.StrictKnows(d.ID)
		switch {
		case want && !got:
			t.Errorf("%s should have a structural check and oracle.StrictKnows does not know it - "+
				"the reference tool test skips that layer in silence", d.ID)
		case !want && got:
			t.Errorf("%s has a structural check that this list does not mention - add it here, "+
				"so the two cannot drift", d.ID)
		}
	}
	for id := range structurallyChecked {
		if _, err := format.Get(id); err != nil {
			t.Errorf("this list names %q and no such format is registered", id)
		}
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
