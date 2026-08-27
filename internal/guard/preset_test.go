package guard

import (
	"errors"
	"strings"
	"testing"

	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// A preset is a function from parameters to a recipe, and the recipe it
// returns is source rather than a structure - so that what "preset eject"
// prints and what a run consumes are the same bytes. PR5 says there are no
// closed presets, and one set of bytes is the strongest way to keep that true.
//
// Which makes the first thing to guard obvious: the source a preset produces
// has to be a recipe this build accepts. A preset that expands into something
// the parser refuses is a preset nobody can run and nobody can eject.

func TestEveryPresetExpandsIntoARecipeThisBuildAccepts(t *testing.T) {
	presets := preset.All()
	if len(presets) == 0 {
		t.Fatal("no preset is registered - this guard would pass without checking anything")
	}

	for _, p := range presets {
		t.Run(p.ID, func(t *testing.T) {
			args, err := p.Settle(nil)
			if err != nil {
				t.Fatalf("settling the declared defaults failed: %v", err)
			}
			src, err := p.Expand(args)
			if err != nil {
				t.Fatalf("expanding with nothing but its own defaults failed: %v", err)
			}
			rec, err := recipe.Parse(src, p.ID)
			if err != nil {
				t.Fatalf("the recipe it produced does not parse:\n%v\n--- source ---\n%s", err, src)
			}
			if len(rec.Targets) == 0 {
				t.Error("it expands into a recipe with no targets")
			}
			for _, target := range rec.Targets {
				if target.Group == "" {
					t.Errorf("target %q carries no group, so nothing can assert about the class it belongs to", target.ID)
				}
			}
		})
	}
}

// The set itself: seven files, three below the limit, one on it, three above,
// and the expectation flipping exactly at the limit. That flip is the whole
// point of the preset - it is what turns a folder of files into a statement
// about somebody's system.
func TestSizeBoundariesPutsTheFlipExactlyAtTheLimit(t *testing.T) {
	p, err := preset.Get("size-boundaries")
	if err != nil {
		t.Fatalf("size-boundaries is not registered: %v", err)
	}
	args, err := p.Settle(preset.Args{"limit": "10mb", "format": "txt"})
	if err != nil {
		t.Fatal(err)
	}
	src, err := p.Expand(args)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := recipe.Parse(src, "size-boundaries")
	if err != nil {
		t.Fatalf("%v\n--- source ---\n%s", err, src)
	}

	const limit = 10 * 1024 * 1024
	if len(rec.Targets) != 7 {
		t.Fatalf("expected 7 targets and got %d", len(rec.Targets))
	}

	var total int64
	for _, target := range rec.Targets {
		if len(target.Sizes) != 1 {
			t.Errorf("%s asks for %d files and every step of the set is one", target.ID, len(target.Sizes))
			continue
		}
		size := target.Sizes[0]
		total += size

		wantAccept := size <= limit
		got := target.Expected == "accept"
		if got != wantAccept {
			t.Errorf("%s is %d B against a limit of %d and it expects %q",
				target.ID, size, limit, target.Expected)
		}
		if !wantAccept && target.ExpectedReason != "size_limit" {
			t.Errorf("%s expects a rejection for the reason %q and the reason is the limit",
				target.ID, target.ExpectedReason)
		}
	}

	// The distances cancel in pairs, so the whole set comes to exactly seven
	// times the limit. Worth pinning: the format document said about twice,
	// which is the number somebody would plan disk space around.
	if want := int64(7 * limit); total != want {
		t.Errorf("the set comes to %d B and seven times the limit is %d B", total, want)
	}
}

// PR7 and the untouchable rule about silence. A limit so small that the files
// below it cannot exist has to be refused as a set, not delivered as the part
// that happened to fit - and the part that fits is never the part the run was
// about, because the interesting files are the ones nearest the limit.
func TestSizeBoundariesRefusesASetItCannotCompleteRatherThanPartOfIt(t *testing.T) {
	p, err := preset.Get("size-boundaries")
	if err != nil {
		t.Fatal(err)
	}
	// PDF cannot be small, and a limit of 1 kB puts every step below its floor.
	args, err := p.Settle(preset.Args{"limit": "1kb", "format": "pdf"})
	if err != nil {
		t.Fatal(err)
	}

	src, err := p.Expand(args)
	if err == nil {
		t.Fatalf("it produced a set for a limit no file of it can reach:\n%s", src)
	}

	var impossible *preset.ImpossibleError
	if !asImpossible(err, &impossible) {
		t.Fatalf("the refusal is %T and it should say the set cannot be built: %v", err, err)
	}
	for _, want := range []string{"size-boundaries", "limit"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not mention %q:\n%s", want, err)
		}
	}
	// The engine writes this sentence once and both surfaces show it word for
	// word, so it may not be spelled the way only one of them takes its input.
	// The window labels these fields "limit" and "spread" and has no "--limit"
	// on it anywhere, so a reader there was being sent to translate. Seen on a
	// screenshot of the refused state on 2026-08-11, O79.
	//
	// This asks about flag syntax rather than about the two names, because the
	// defect is the syntax: any "--something" here is a spelling one surface
	// does not have.
	if strings.Contains(err.Error(), "--") {
		t.Errorf("the refusal uses command line flag syntax, which the window has no such thing as:\n%s", err)
	}
}

func asImpossible(err error, target **preset.ImpossibleError) bool {
	var e *preset.ImpossibleError
	if errors.As(err, &e) {
		*target = e
		return true
	}
	return false
}
