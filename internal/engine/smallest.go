package engine

import (
	"context"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// The fifth way of stating a size (ARCHITECTURE.md section 9, AR10): a target
// says smallest and each file is the smallest its format makes at its
// settings. The rule AR10 exists for still holds - the size is known while
// planning, so a dry run reports it to the byte and nothing is written before
// a refusal - because the plan of a picture with its width and height set
// encodes the picture and knows what it comes to.
//
// For pictures only, and only with their width and height known: the owner's
// decision of 2026-11-14 (docs/PLACEHOLDER-2026-11-14.md section 10). Without
// them a picture is chosen to fit the bytes asked for, so there is nothing to
// be the smallest of, and the smallest file of the other formats is an answer
// nobody has asked a question for yet.

// checkSmallest refuses smallest for a target it cannot answer.
func checkSmallest(t *Target, desc format.Descriptor) error {
	if !t.Smallest {
		return nil
	}
	if !strings.HasPrefix(desc.MediaType, "image/") {
		return &RecipeError{Setting: format.SettingSize,
			Detail:  core.Says("engine.SmallestNotAPicture", "target %q asks for the smallest %s file, and smallest is for pictures", core.A("Target", t.ID), core.A("Format", desc.ID)),
			Because: core.Says("engine.SmallestNotAPictureWhy", "a picture with its width and height set has one smallest file, the picture with nothing added to reach a size"),
			Remedy:  core.Says("engine.SmallestNotAPictureFix", "Give the target a size such as 2mb, or choose a picture format")}
	}
	for _, side := range []string{"width", "height"} {
		if t.Properties[side] == "" && declaredDefault(desc, side) == "" {
			return &RecipeError{Setting: format.SettingSize,
				Detail:  core.Says("engine.SmallestNeedsASize", "target %q asks for the smallest %s and does not say how large the picture is", core.A("Target", t.ID), core.A("Format", desc.ID)),
				Because: core.Says("engine.SmallestNeedsASizeWhy", "without a width and a height the picture is chosen to fit the bytes asked for, so there is no picture to be the smallest of"),
				Remedy:  core.Says("engine.SmallestNeedsASizeFix", "Set width and height, such as width 1920 and height 1080, or give the target a size")}
		}
	}
	return nil
}

// declaredDefault is what a format declares a setting to be when nothing says,
// or nothing when it works the value out itself.
func declaredDefault(desc format.Descriptor, name string) string {
	for _, p := range desc.Properties {
		if p.Name == name {
			return p.Default
		}
	}
	return ""
}

// planFile is the plan of one file: at the size asked for, or at the smallest
// its format makes when the target says smallest - and then held to the floor
// of its damage, which checkDamageFloor could not ask before the size existed.
func planFile(ctx context.Context, t *Target, desc format.Descriptor, r format.Request) (format.Plan, error) {
	if !t.Smallest {
		return planWithoutCrashing(ctx, desc, r)
	}
	p, err := planGuarded(desc, func() (format.Plan, error) { return desc.PlanSmallest(ctx, r) })
	if err != nil {
		return p, err
	}
	return p, checkFloorOf(t, p.Bytes)
}
