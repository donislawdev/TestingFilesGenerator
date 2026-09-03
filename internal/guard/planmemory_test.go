package guard

import (
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// A plan too big to hold is refused while it is being built, not after.
//
// core.MaxFilesPerRun counts files and its comment justifies the number with a
// measurement - "about 2.4 kB a file, straight all the way", so a million files
// is roughly 2.4 GB. That measurement was taken on txt. Re-measured on
// 2026-08-26 with tools/probes/plansize at two thousand files a format: txt 850
// B, docx 2228 B, zip 6153 B, pdf 7527 B, and a pdf of a thousand pages
// 5244230 B.
//
// So "--format pdf --set pages=1000 --count 10000" asks for about 52 GB of plan
// while every number in it is legal - ten thousand is far under the file
// ceiling and a thousand pages is under the page ceiling. The ceiling meant to
// keep the plan inside memory was counting the wrong quantity.
//
// The ceiling is injected here rather than reached for real, the same way the
// free space guard is tested against a small disk instead of a petabyte. A
// guard that had to allocate two gigabytes would be slower than the rest of the
// suite put together and would sit inside the mutation runner's own memory cap.
func TestAPlanTooBigToHoldIsRefusedWhileItIsBuilt(t *testing.T) {
	// Fifty thousand text files is about 42 MB of plan against a ceiling of
	// four, so the plan passes it by roughly ten times.
	//
	// The margin is the point, and it was measured the hard way. The first
	// version asked four thousand files to pass a 64 kB ceiling - three and a
	// half megabytes against sixty four kilobytes, which reads like plenty. It
	// passed on its own and FAILED inside the suite, because this guard is the
	// one thing here that measures the real heap, and a heap reading in a
	// process that has just run four hundred other tests carries megabytes of
	// noise in both directions. A ceiling under the noise floor is a check
	// whose answer depends on what ran before it.
	targets := []engine.Target{{
		ID:     "files",
		Format: "txt",
		Sizes:  engine.Uniform(50000, 4096),
	}}

	_, err := engine.Plan(targets, engine.Options{
		OutDir:       "out",
		ManifestName: "manifest.json",
		MaxPlanBytes: 4 << 20,
	})
	if err == nil {
		t.Fatal("planning fifty thousand files under a 4 MB plan ceiling was accepted")
	}

	// The four parts of D6. A refusal that names no ceiling leaves the person
	// with nothing to change.
	said := err.Error()
	for _, want := range []string{"plan", "ceiling"} {
		if !strings.Contains(said, want) {
			t.Errorf("the refusal does not mention %q: %s", want, said)
		}
	}
	// And it has to say which target, because a recipe with twenty batches
	// otherwise gets a refusal at the foot of the form with nothing marked.
	var addressed interface{ AboutSetting() string }
	if !asAddressed(err, &addressed) {
		t.Errorf("the refusal carries no address, so no window can place it: %s", said)
	} else if addressed.AboutSetting() == "" {
		t.Errorf("the refusal carries an empty address: %s", said)
	}
}

// The same run under the real ceiling still plans.
//
// Without this the test above passes on a build that refuses everything, which
// is the shape this project has thrown away six pieces of defensive code for -
// a check nothing can turn red is not a check.
func TestAnOrdinaryRunIsNotRefusedByThePlanCeiling(t *testing.T) {
	targets := []engine.Target{{
		ID:     "files",
		Format: "txt",
		Sizes:  engine.Uniform(4000, 4096),
	}}

	files, err := engine.Plan(targets, engine.Options{
		OutDir:       "out",
		ManifestName: "manifest.json",
		// Zero means the real ceiling, which is what every caller passes.
	})
	if err != nil {
		t.Fatalf("four thousand text files were refused under the real ceiling: %v", err)
	}
	if len(files) != 4000 {
		t.Fatalf("planned %d files, expected 4000", len(files))
	}
}

// A run too short to have been weighed is weighed.
//
// planMemory used to take its reference point only once a run had announced
// sixty four files, and account returned at once until it had one - so a run of
// sixty three files had no ceiling at all. The count it waited on was the whole
// run's, added up across targets, which made it worse than that sounds: sixty
// four targets holding one file each took the reference point after SIXTY THREE
// files had already been planned, and those then sat inside the baseline for
// the rest of the run, however long it ran.
//
// What makes that reachable rather than theoretical was measured on 2026-09-03
// with tools/probes/plansize: a zip of ten thousand pdf entries costs 74 740
// 758 B of plan a file, five points from one file to sixteen, linear to within
// 0.05%. Twenty nine of those come to 2.17 GB against a ceiling of two, and
// twenty nine is under sixty four.
//
// Asked here with ONE file, which is the sharpest way to put the rule: the
// reference point exists before the first file rather than after the sixty
// fourth. The ceiling is injected for the same reason as the guard above, and
// the shape is a five thousand page pdf, measured at 25 194 908 B of plan - so
// one file passes a 4 MB ceiling six times over, which is clear of the
// megabytes of noise a heap reading carries in a process that has already run
// four hundred other tests.
func TestARunTooShortToHaveBeenWeighedIsWeighed(t *testing.T) {
	targets := []engine.Target{{
		ID:         "one",
		Format:     "pdf",
		Sizes:      engine.Uniform(1, 20<<20),
		Properties: map[string]string{"pages": "5000"},
	}}

	_, err := engine.Plan(targets, engine.Options{
		OutDir:       "out",
		ManifestName: "manifest.json",
		MaxPlanBytes: 4 << 20,
	})
	if err == nil {
		t.Fatal("one file whose plan is six times the ceiling was accepted, so nothing weighed it")
	}

	said := err.Error()
	for _, want := range []string{"plan", "ceiling"} {
		if !strings.Contains(said, want) {
			t.Errorf("the refusal does not mention %q: %s", want, said)
		}
	}
	// Where the run had got to, which for a single file is the half of the
	// sentence that says the check ran at all.
	if !strings.Contains(said, "after 1 file") {
		t.Errorf("the refusal does not say the plan was weighed at the first file: %s", said)
	}
}

// The same shape one setting down still plans.
//
// Without this the guard above passes on a build that refuses every short run,
// and a check nothing can turn green is worth as little as one nothing can turn
// red.
func TestAShortOrdinaryRunIsNotRefusedByThePlanCeiling(t *testing.T) {
	targets := []engine.Target{{
		ID:         "one",
		Format:     "pdf",
		Sizes:      engine.Uniform(1, 3<<20),
		Properties: map[string]string{"pages": "1000"},
	}}

	files, err := engine.Plan(targets, engine.Options{
		OutDir:       "out",
		ManifestName: "manifest.json",
		// Zero means the real ceiling, which is what every caller passes.
	})
	if err != nil {
		t.Fatalf("a single thousand page pdf was refused under the real ceiling: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("planned %d files, expected 1", len(files))
	}
}

// The ceiling this build works to is the one core states.
//
// Asked separately from the mechanism above, because that one runs against an
// injected number and would stay green if the real constant were changed to
// something a run could never reach.
func TestThePlanCeilingIsTheOneCoreStates(t *testing.T) {
	if core.MaxPlanBytes <= 0 {
		t.Fatalf("the plan ceiling is %d, which no run can be under", core.MaxPlanBytes)
	}
	// Generous enough for every run this tool was designed around. The largest
	// preset asks for 10 040 files, which measured at 850 B a file for txt is
	// about 9 MB, and at 7527 B for pdf about 76 MB.
	if core.MaxPlanBytes < 256<<20 {
		t.Errorf("the plan ceiling is %d B, which is under the 256 MB the largest shapes this tool ships need",
			core.MaxPlanBytes)
	}
	// And the sentence has to name it, or the refusal sends somebody looking
	// for a number that is written somewhere else.
	if !strings.Contains(core.PlanTooLargeWhy, core.HumanBytes(core.MaxPlanBytes)) {
		t.Errorf("the refusal reason does not name the ceiling %s: %s",
			core.HumanBytes(core.MaxPlanBytes), core.PlanTooLargeWhy)
	}
}

func asAddressed(err error, target *interface{ AboutSetting() string }) bool {
	if v, ok := err.(interface{ AboutSetting() string }); ok {
		*target = v
		return true
	}
	return false
}
