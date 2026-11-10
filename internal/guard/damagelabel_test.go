package guard

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// labelNotGuaranteed is the note a damaged file carries in place of a claim
// to its label.
const labelNotGuaranteed = "label_not_guaranteed"

// TestADamagedFileNeverClaimsItsLabel holds the manifest to what it can know
// about a label after a damage.
//
// Measured 2026-11-09: zero-head on txt overwrote the label at the start of
// the text, and the manifest still said label_embedded: true. Where a label
// sits differs by format, so the rule is the same for all of them. A damaged
// file says false, in both places the manifest says it, and a note says why.
//
// Every format, each beside a healthy file of the same format and size. The
// healthy one is the control: a format that never claims its label would pass
// this guard without the rule being there, so the guard also counts the
// formats where the claim was really taken away.
func TestADamagedFileNeverClaimsItsLabel(t *testing.T) {
	dir := t.TempDir()
	chain := damage.Chain{{ID: damage.ZeroHead}}

	withdrawn := 0
	for _, f := range format.All() {
		out := filepath.Join(dir, f.ID)
		targets := []engine.Target{
			{ID: "good", Format: f.ID, Sizes: engine.Uniform(1, witnessSize), Label: true},
			{ID: "broken", Format: f.ID, Sizes: engine.Uniform(1, witnessSize), Damage: chain, Label: true},
		}
		opt := engine.Options{OutDir: out, Seed: 5, ManifestName: engine.DefaultManifestName}
		planned, err := engine.Plan(targets, opt)
		if err != nil {
			t.Fatalf("%s: planning: %v", f.ID, err)
		}
		res, err := engine.Run(t.Context(), planned, opt)
		if err != nil {
			t.Fatalf("%s: running: %v", f.ID, err)
		}
		entries := map[string]manifest.File{}
		for _, m := range res.Manifest.Files {
			entries[m.TargetID] = m
		}
		good, broken := entries["good"], entries["broken"]
		if good.Name == "" || broken.Name == "" {
			t.Fatalf("%s: the run did not write both files", f.ID)
		}

		if broken.LabelEmbedded {
			t.Errorf("%s: a damaged file claims label_embedded: true.\n"+
				"Reason: the damage came after the label was written and may have overwritten it,\n"+
				"so the manifest cannot know the label is still there.", f.ID)
		}
		if claim, ok := broken.Properties[format.PropertyLabelEmbedded].(bool); ok && claim {
			t.Errorf("%s: a damaged file claims label_embedded: true in its properties, "+
				"while the field beside them says false", f.ID)
		}
		noted := slices.ContainsFunc(broken.Notes, func(n manifest.Note) bool { return n.Code == labelNotGuaranteed })
		if noted != good.LabelEmbedded {
			t.Errorf("%s: the healthy file claims its label: %t, and the damaged one carries the note %s: %t.\n"+
				"Reason: the note says why a label that was written is no longer claimed, so it\n"+
				"belongs exactly where a claim was taken away.", f.ID, good.LabelEmbedded, labelNotGuaranteed, noted)
		}
		if good.LabelEmbedded {
			withdrawn++
		}
	}
	if withdrawn == 0 {
		t.Fatal("no healthy file claimed its label, so this guard took nothing away and proved nothing")
	}
	t.Logf("the claim was taken away in %d format(s)", withdrawn)
}
