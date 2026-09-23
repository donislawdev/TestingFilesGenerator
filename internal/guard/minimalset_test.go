package guard

import (
	"bytes"
	"runtime"
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
	// The targets themselves, because the two counts above are counts of MAPS
	// keyed by format - a second minimal png would overwrite the first and
	// leave both of them reading exactly as they do now. Named by CodeRabbit on
	// 2026-09-22, and it is the shape this project calls a guard that stopped
	// reaching the state it guards.
	if want := len(format.IDs()) + len(canBeEmpty); len(doc.Targets) != want {
		t.Errorf("the set holds %d targets and %d were expected - one per format, plus one more "+
			"for every format that can be empty", len(doc.Targets), want)
	}
}

// The same formats in two orders build the same set.
//
// The order somebody types is not part of what they asked for. It used to be:
// "--formats png,zip" and "--formats zip,png" produced different recipe text,
// so the same selection carried two different recipe_hash values into two
// manifests and listed the files the other way round. Measured on 2026-09-22 -
// f76a3883e against 073157029 - after a comment in this package had claimed
// registry order for weeks without anything walking the registry.
//
// The bytes of the files never moved, because a seed comes from the id of a
// target rather than from its place in the list. That is what made this quiet:
// every file was right and only the record of them disagreed.
// The smallest size of each format is worked out once, not at every expansion.
//
// Finding it means planning the format at growing sizes, and for a picture
// that means encoding one. The window expands this set on every change while
// the batch screen builds on it, so a set worked out afresh each time made a
// keystroke there cost 380 ms and ~379 MB of garbage in the real window on
// 2026-09-23 (docs/GUI-MEMORY-2026-09-23.md section 2.3). Measured here the
// same day, least of five: 50.4 MB per expansion afresh, 0.51 MB remembered.
// The line sits a factor of ten from each.
//
// The least of several readings, because the counter is the whole process's
// and a reading can only be too high - the lesson of tools/probes/alloccount.
// That the remembered sizes are the RIGHT ones is
// TestTheMinimalSetSitsOnEveryFormatsFloor's question, not this one's.
func TestTheMinimalSetIsWorkedOutOnceAndNotAtEveryExpansion(t *testing.T) {
	const ceiling = 5 << 20
	if _, err := preset.Expand("empty-and-minimal", preset.Args{}); err != nil {
		t.Fatalf("the set did not expand, so nothing was asked: %v", err)
	}
	least := ^uint64(0)
	for i := 0; i < 5; i++ {
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		if _, err := preset.Expand("empty-and-minimal", preset.Args{}); err != nil {
			t.Fatal(err)
		}
		runtime.ReadMemStats(&after)
		if spent := after.TotalAlloc - before.TotalAlloc; spent < least {
			least = spent
		}
	}
	if least > ceiling {
		t.Errorf("expanding the minimal set a second time allocated %d bytes, over %d - "+
			"the smallest size of every format is being worked out again", least, ceiling)
	}
}

func TestTheMinimalSetIsTheSameWhateverOrderTheFormatsAreNamedIn(t *testing.T) {
	// Two formats far apart in the registry, so a walk that kept the typing
	// cannot pass by accident.
	first, err := preset.Expand("empty-and-minimal", preset.Args{"formats": "png,bmp,zip"})
	if err != nil {
		t.Fatalf("the preset refused three formats: %v", err)
	}
	second, err := preset.Expand("empty-and-minimal", preset.Args{"formats": "zip,png,bmp"})
	if err != nil {
		t.Fatalf("the preset refused the same three in another order: %v", err)
	}
	if !bytes.Equal(first.Source, second.Source) {
		t.Errorf("the same formats in two orders built two recipes.\n--- png,bmp,zip ---\n%s\n--- zip,png,bmp ---\n%s",
			first.Source, second.Source)
	}

	// And the order they come out in is the registry's, rather than merely
	// being the same both times - two runs agreeing on a wrong order would
	// satisfy the check above and still put bmp after zip.
	doc, err := recipe.Parse(first.Source, "empty-and-minimal.yaml")
	if err != nil {
		t.Fatalf("the preset wrote a recipe this build cannot read: %v", err)
	}
	seen := make([]string, 0, len(doc.Targets))
	for _, target := range doc.Targets {
		seen = append(seen, target.Format)
	}
	if want := []string{"bmp", "png", "zip"}; strings.Join(seen, ",") != strings.Join(want, ",") {
		t.Errorf("the set is laid out as %v and the registry names them %v", seen, want)
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
