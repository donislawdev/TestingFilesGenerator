package guard

import (
	"errors"
	"strconv"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// A size parameter of any preset, set to the largest size there is, gives a
// set or a refusal about that size - never an answer about the bottom of the
// range, and never a refusal the command line reads as the tool failing.
//
// On 2026-10-27 the two presets that lay out a set around a limit answered
// that limit two different ways. size-boundaries gave the sentence --boundary
// gives, but as a bare error, so the command line ended with code 1, which the
// frozen table keeps for faults of the tool. upload-validation let the step
// above the limit wrap and said "over_1b would be -9223372036854775808 B ...
// Raise the limit above 1 B", the advice the size-boundaries fix of 2026-08-26
// had removed from that preset alone (docs/STABILITY-REVIEW-2026-10-26.md, R13).
// The presets are asked from the registry, so one added with a size parameter
// is asked too.
func TestEveryPresetAnswersTheLargestSizeAboutTheTop(t *testing.T) {
	asked := 0
	for _, p := range preset.All() {
		for _, param := range p.Parameters {
			if param.Kind != format.PropertySize {
				continue
			}
			asked++
			got, err := preset.Expand(p.ID, preset.Args{param.Name: "9223372036854775807b"})
			if err == nil {
				for _, target := range sourceTargets(t, p.ID, got.Source) {
					for _, size := range target.Sizes {
						if size < 0 {
							t.Errorf("%s with %s at the largest size built %s of %d B", p.ID, param.Name, target.ID, size)
						}
					}
				}
				continue
			}
			var impossible *preset.ImpossibleError
			if !errors.As(err, &impossible) {
				t.Errorf("%s with %s at the largest size gave %v, which is not a refusal of the set and ends as a fault of the tool", p.ID, param.Name, err)
				continue
			}
			if impossible.Setting != param.Name {
				t.Errorf("%s with %s at the largest size marks %q, not the setting that was given", p.ID, param.Name, impossible.Setting)
			}
			if id := impossible.Detail.ID(); id == "preset.StepBelowZero" || id == "preset.StepBelowFloor" {
				t.Errorf("%s with %s at the largest size answered about the bottom of the range: %v", p.ID, param.Name, err)
			}
		}
	}
	// Two presets lay a set out around a limit and one more takes a sample
	// size, so fewer than three asked means the registry was not read.
	if asked < 3 {
		t.Fatalf("only %d size parameter(s) were asked - the presets were not read", asked)
	}

	for _, id := range []string{"size-boundaries", "upload-validation"} {
		_, err := preset.Expand(id, preset.Args{"limit": "9223372036854775807b"})
		if !errors.Is(err, core.ErrBoundaryTooLarge) {
			t.Errorf("%s gave %v for a limit with no room above it, expected the refusal --boundary gives", id, err)
		}
	}
}

// The file far over the limit is the limit times the multiplier to the byte, or
// the set is refused - it is never a product that wrapped round.
//
// Measured on 2026-10-27: a limit of 1844674407370955264B at 10x gave
// far_over_10x a size of 1024 B, under the limit, while the manifest still
// expected it to be refused for size_limit (R13). The largest limit that fits
// builds, so this cannot be satisfied by refusing every large limit.
func TestTheFileFarOverTheLimitIsThatManyTimesTheLimitOrNothing(t *testing.T) {
	const most = int64(922337203685477580) // the largest int64 over ten
	args := func(limit int64) preset.Args {
		return preset.Args{"limit": strconv.FormatInt(limit, 10) + "b", "far-over": "10x", "bulk": "0", "allow": "txt", "deny": "exe"}
	}

	found := false
	for _, target := range expandedTargets(t, "upload-validation", args(most)) {
		if target.ID != "far_over_10x" {
			continue
		}
		found = true
		if len(target.Sizes) != 1 || target.Sizes[0] != 10*most {
			t.Errorf("far_over_10x at a limit of %d B is %v, expected %d B", most, target.Sizes, 10*most)
		}
	}
	if !found {
		t.Fatal("the set has no far_over_10x, so this guard read the wrong thing")
	}

	_, err := preset.Expand("upload-validation", args(most+1))
	var impossible *preset.ImpossibleError
	if !errors.As(err, &impossible) || impossible.Setting != "far-over" {
		t.Fatalf("a limit of %d B at 10x gave %v, expected the set refused about far-over", most+1, err)
	}
}

// sourceTargets reads the targets of a recipe a preset expanded to.
func sourceTargets(t *testing.T, id string, src []byte) []recipe.Target {
	t.Helper()
	rec, err := recipe.Parse(src, id+".yaml")
	if err != nil {
		t.Fatalf("the recipe %s expanded to does not read back: %v", id, err)
	}
	return rec.Targets
}
