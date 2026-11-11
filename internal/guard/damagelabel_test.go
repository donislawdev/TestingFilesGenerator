package guard

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/imagelabel"
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

// TestADamagedFileStatesItsSizeOnDiskAndItsDamage holds the label of a damaged
// file to what is on disk.
//
// Measured 2026-11-11 before this held (O333): a text of 10 MiB cut with
// missing=50 said "20971520 B" in its first line - the size of the whole file
// before the cut - and named no damage, so a file cut by half and one cut by
// nine tenths looked the same from the inside. append-junk stated the size
// without the junk.
//
// Every format whose healthy file carries its label as text is asked, with
// junk appended: that damage keeps every byte the generator wrote, so the
// label is where the generator put it and can be read back. Which formats
// those are is read from the healthy file, not listed here - a picture draws
// its label in pixels and is left to the guards on pictures. The expected text
// is written out here rather than asked of format.Request, so a wording that
// drops the damage cannot agree with itself.
func TestADamagedFileStatesItsSizeOnDiskAndItsDamage(t *testing.T) {
	chain := damage.Chain{{ID: damage.AppendJunk}}
	const size = witnessSize
	before, err := chain.Before(size)
	if err != nil {
		t.Fatal(err)
	}
	brokenSeed := core.FileSeed(core.TargetSeed(lengthSeed, "broken"), 0)
	// The canary: the label this guard was written against, which states the
	// size the generator wrote and no damage, must not pass it.
	planted := fmt.Sprintf("tfg - txt - %d B - seed %s", before, core.SeedLabel(brokenSeed))
	if statesItsDamage([]byte(planted), "txt", size, chain, brokenSeed) == "" {
		t.Fatal("the check took a label naming the size before the damage, and no damage, for the label it asks for")
	}

	dir := t.TempDir()
	read := 0
	for _, f := range format.All() {
		good := engine.Target{ID: "good", Format: f.ID, Sizes: engine.Uniform(1, size), Label: true}
		broken := engine.Target{ID: "broken", Format: f.ID, Sizes: engine.Uniform(1, size), Damage: chain, Label: true}
		_, paths := lengthRun(t, filepath.Join(dir, f.ID), []engine.Target{good, broken})
		healthy, err := os.ReadFile(paths["good"])
		if err != nil {
			t.Fatal(err)
		}
		goodSeed := core.FileSeed(core.TargetSeed(lengthSeed, "good"), 0)
		if !bytes.Contains(healthy, fmt.Appendf(nil, "tfg - %s - %d B - seed %s", f.ID, size, core.SeedLabel(goodSeed))) {
			continue
		}
		got, err := os.ReadFile(paths["broken"])
		if err != nil {
			t.Fatal(err)
		}
		if why := statesItsDamage(got, f.ID, size, chain, brokenSeed); why != "" {
			t.Errorf("%s: %s", f.ID, why)
		}
		read++
	}
	if read < 10 {
		t.Fatalf("only %d format(s) carry their label as text the guard could find - it did not reach the formats it is about", read)
	}
	t.Logf("the label was read back from %d format(s)", read)
}

// TestATruncatedTextSaysHowMuchWasCut is the case the owner met: two texts of
// the same size cut by different shares used to begin with the same line.
// Their labels are the first thing a person opening them sees, so the share
// has to be there.
func TestATruncatedTextSaysHowMuchWasCut(t *testing.T) {
	const size = 64 << 10
	seed := core.FileSeed(core.TargetSeed(lengthSeed, "cut"), 0)
	for _, missing := range []string{"50", "90"} {
		chain := damage.Chain{{ID: damage.Truncate, Values: damage.Values{damage.SettingMissing: missing}}}
		target := engine.Target{ID: "cut", Format: "txt", Sizes: engine.Uniform(1, size), Damage: chain, Label: true}
		_, paths := lengthRun(t, filepath.Join(t.TempDir(), missing), []engine.Target{target})
		got, err := os.ReadFile(paths["cut"])
		if err != nil {
			t.Fatal(err)
		}
		first, _, _ := bytes.Cut(got, []byte("\n"))
		want := fmt.Sprintf("tfg - txt - %d B - damage truncate:missing=%s - seed %s", size, missing, core.SeedLabel(seed))
		if string(first) != want {
			t.Errorf("a text cut with missing=%s begins %q, expected %q", missing, first, want)
		}
	}
}

// statesItsDamage says what is wrong with the label in got, or nothing when
// it states size on disk and every entry of chain with its settings resolved.
func statesItsDamage(got []byte, id string, size int64, chain damage.Chain, seed uint64) string {
	want := fmt.Sprintf("tfg - %s - %d B - damage %s - seed %s", id, size, chain.Label(), core.SeedLabel(seed))
	if !strings.Contains(chain.Label(), "=") {
		return "the chain " + chain.Label() + " does not name its settings, so the label cannot say how much was broken"
	}
	if !bytes.Contains(got, []byte(want)) {
		return fmt.Sprintf("the file does not carry the label %q", want)
	}
	return ""
}

// TestEveryCharacterOfADamagedLabelCanBeDrawn holds the label a picture draws
// to the characters its font has.
//
// The font is 3 by 5 pixels and drawn by hand, and a character it lacks comes
// out as a box the shape of a 0. Before the label named a damage it never held
// an = or a comma, and the font had neither - a picture cut with missing=50
// would have said MISSING050. Every damage from the registry, at each end of
// every setting it takes, so a damage added with a new character in its name
// or settings turns this red.
func TestEveryCharacterOfADamagedLabelCanBeDrawn(t *testing.T) {
	if imagelabel.HasGlyph('~') {
		t.Fatal("the font draws ~, which it never had - the question below cannot tell a glyph from the stand-in")
	}
	asked := 0
	for _, d := range damage.All() {
		values := []damage.Values{{}}
		for _, p := range d.Parameters {
			values = append(values,
				damage.Values{p.Name: strconv.FormatInt(p.Min, 10)},
				damage.Values{p.Name: strconv.FormatInt(p.Max, 10)})
		}
		for _, v := range values {
			chain := damage.Chain{{ID: d.ID, Values: v}, {ID: d.ID, Values: v}}
			label := format.Request{Bytes: 1 << 40, Seed: 1, Damage: chain.Label()}.LabelText("png")
			for _, r := range label {
				if !imagelabel.HasGlyph(r) {
					t.Errorf("%s: the picture font has no %q, so the label %q is drawn with a box in its place", d.ID, r, label)
				}
			}
			asked++
		}
	}
	if asked == 0 {
		t.Fatal("no damage was asked - the registry was not read")
	}
}
