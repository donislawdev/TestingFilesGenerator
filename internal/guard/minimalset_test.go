package guard

import (
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// The empty-and-minimal set asks every format for the smallest size it takes,
// and carries a separate empty file for every format that has one.
//
// The set is the whole product here, so this asserts what is IN it rather than
// that it expanded. Three claims, and each one is a way the set could go quietly
// wrong:
//
// Every file sits exactly on its format's floor. A size one byte above would
// still run, still verify and still look like a set about minimums - and would
// stop being the thing the preset promises.
//
// A format whose floor is nought bytes gets TWO files: one of a byte for the
// positive control, one of nothing for the half nobody can promise an answer
// for. One entry each would mean either dropping those formats out of the
// control or giving an empty file an expectation nobody can back, which is
// untouchable rule 5.
//
// And the count of empty files equals the count of formats that can be empty,
// asserted from the registry rather than from the number two. Two is today's
// answer, md and txt, and a third format reaching nought would otherwise leave
// this guard green while the set silently changed shape.
func TestTheMinimalSetSitsOnEveryFormatsFloor(t *testing.T) {
	expanded, err := preset.Expand("empty-and-minimal", preset.Args{})
	if err != nil {
		t.Fatalf("the preset refused its own defaults: %v", err)
	}
	doc, err := recipe.Parse(expanded.Source, "empty-and-minimal.yaml")
	if err != nil {
		t.Fatalf("the preset wrote a recipe this build cannot read: %v\n%s", err, expanded.Source)
	}

	canBeEmpty := map[string]bool{}
	for _, id := range format.IDs() {
		desc, err := format.Get(id)
		if err != nil {
			t.Fatalf("the registry lists %s and then does not have it: %v", id, err)
		}
		if desc.SmallestAccepted(format.Request{Label: true}) == 0 {
			canBeEmpty[id] = true
		}
	}
	if len(canBeEmpty) == 0 {
		t.Fatal("no format in this build reaches nought bytes, so the empty half of this set " +
			"cannot exist and this guard is asserting nothing")
	}

	sizes := map[string]int64{}
	empties := map[string]bool{}
	for _, target := range doc.Targets {
		desc, err := format.Get(target.Format)
		if err != nil {
			t.Fatalf("the set holds a target in format %q, which this build does not have", target.Format)
		}
		floor := desc.SmallestAccepted(format.Request{Label: true})

		// The settled sizes rather than the text, because the text is what the
		// recipe wrote and these are what the run will produce. One file per
		// target here, so a target holding several sizes is itself a failure.
		if len(target.Sizes) != 1 {
			t.Errorf("the target %s holds %d sizes and every file of this set is one file",
				target.ID, len(target.Sizes))
			continue
		}
		size := target.Sizes[0]

		if size == 0 {
			empties[target.Format] = true
			if !canBeEmpty[target.Format] {
				t.Errorf("the set holds an empty %s and %s cannot be empty - its floor is %d B",
					target.Format, target.Format, floor)
			}
			if target.Expected != "unspecified" || target.ExpectedReason != "size_zero" {
				t.Errorf("the empty %s expects %q for %q - an empty file is legal and what to do "+
					"with it is the application's policy, so it has to be unspecified with size_zero",
					target.Format, target.Expected, target.ExpectedReason)
			}
			continue
		}

		if target.Expected != "accept" {
			t.Errorf("the smallest valid %s expects %q - it is a valid file, so it is the positive "+
				"control and has to be accept", target.Format, target.Expected)
		}
		want := floor
		if want == 0 {
			// A format whose floor is nought still needs a file with something
			// in it, or it drops out of the positive control.
			want = 1
		}
		if size != want {
			t.Errorf("the minimal %s is %d B and the smallest this build takes is %d B",
				target.Format, size, want)
		}
		sizes[target.Format] = size
	}

	if len(sizes) != len(format.IDs()) {
		t.Errorf("the set covers %d formats and this build has %d - every format is one path "+
			"through somebody's reader", len(sizes), len(format.IDs()))
	}
	if len(empties) != len(canBeEmpty) {
		t.Errorf("the set holds %d empty files and %d formats in this build can be empty",
			len(empties), len(canBeEmpty))
	}
}

// A set that came out with only one of its halves says so.
//
// Untouchable rule 6. "--formats zip" is a sensible thing to ask for and no
// archive has a legal empty form, so the set is all minimal and no empty - and a
// preset named after both halves that ships one quietly is a promise it did not
// keep.
//
// The second half of this guard is the one that stops it from passing for the
// wrong reason: a preset that said this on EVERY run would also pass the first
// half, and would be noise on the run that has both halves.
func TestASetWithNoEmptyHalfSaysSoAndOneWithBothStaysQuiet(t *testing.T) {
	quiet, err := preset.Expand("empty-and-minimal", preset.Args{"formats": "txt,zip"})
	if err != nil {
		t.Fatalf("the preset refused a set with both halves: %v", err)
	}
	for _, note := range quiet.Notes() {
		if strings.Contains(note, "no empty files") {
			t.Errorf("a set that HAS empty files still says %q", note)
		}
	}

	spoken, err := preset.Expand("empty-and-minimal", preset.Args{"formats": "zip"})
	if err != nil {
		t.Fatalf("the preset refused a set of one format: %v", err)
	}
	said := strings.Join(spoken.Notes(), "\n")
	if !strings.Contains(said, "no empty files") {
		t.Errorf("a set with no empty half says nothing about it. It said: %q", said)
	}
	// The sentence names the formats that CAN be empty, and it has to name them
	// from the registry - a list written into the sentence would go stale green
	// the day a third format reached nought bytes.
	for _, id := range format.IDs() {
		desc, err := format.Get(id)
		if err != nil || desc.SmallestAccepted(format.Request{Label: true}) != 0 {
			continue
		}
		if !strings.Contains(said, id) {
			t.Errorf("%s reaches nought bytes and the note does not name it: %q", id, said)
		}
	}
}

// A list parameter refuses two spellings of one item.
//
// This is the 2026-08-05 collision one layer up, and it came back on 2026-09-22
// in the shared list parser: "PNG,png" passed a duplicate check made on what was
// typed, became two targets of one id, and surfaced as "target id minimal_png is
// used twice" - a refusal about an id nobody had written, pointing at the recipe
// rather than at the value.
//
// Asserted through the message rather than only through the failure, because
// both spellings failing is not the point. WHICH refusal arrives is the point.
func TestAListParameterRefusesTwoSpellingsOfOneItem(t *testing.T) {
	_, err := preset.Expand("empty-and-minimal", preset.Args{"formats": "PNG,png"})
	if err == nil {
		t.Fatal("the preset built a set holding one format twice")
	}
	if !strings.Contains(err.Error(), "the same format") {
		t.Errorf("the refusal is %q.\nIt has to be about the repeated value, not about the "+
			"recipe the value produced", err.Error())
	}
}

// No format is called by the word that means all of them.
//
// The formats parameter takes "all" as a keyword, so a format registered under
// that id would be unreachable through the parameter that exists to reach it -
// silently, because the keyword would simply win. Nothing else in the tree would
// notice.
//
// Cheap, and it is the kind of collision that turns up years later with no clue
// attached. See internal/preset/emptyandminimal.go, which names this guard.
func TestNoFormatIsCalledByTheWordThatMeansAllOfThem(t *testing.T) {
	ids := format.IDs()
	if len(ids) == 0 {
		t.Fatal("no format is registered, so this guard checked nothing")
	}
	for _, id := range ids {
		if strings.EqualFold(id, "all") {
			t.Errorf("a format is registered as %q, which is the word the empty-and-minimal "+
				"formats parameter uses for every format - one of the two has to be renamed", id)
		}
	}
}
