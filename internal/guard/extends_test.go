package guard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// A recipe that builds on a preset: extends and with, docs/RECIPE.md section
// 6, built on 2026-09-22 after docs/EXTENDS-WITH-2026-09-22.md.
//
// The whole of the promise is that such a file is the same run as "tfg preset
// eject" followed by editing - PR5 in a second form. So the guards here run
// both roads and compare the bytes, hold every preset to contributing targets
// and nothing else, hold every door a file comes through to the one that
// expands a preset, and read every refusal about the preset side by the line
// it names.

// A preset's expansion is a version and a list of targets, and nothing else.
//
// This is the rule that makes the merge simple: the file owns the seed, the
// defaults and the output section, and the preset never contests them. It is
// asked of every registered preset with its declared defaults, so the day a
// preset expands into a seed of its own, this goes red before any recipe has
// built on it. ParseExtending asks the same question of the one expansion it
// is handed, which is what makes the rule a fact for a preset this list has
// not met.
func TestAPresetExpandsToTargetsAndNothingElse(t *testing.T) {
	presets := preset.All()
	if len(presets) == 0 {
		t.Fatal("no preset is registered - this guard would pass without checking anything")
	}
	for _, p := range presets {
		t.Run(p.ID, func(t *testing.T) {
			settled, err := p.Settle(nil)
			if err != nil {
				t.Fatal(err)
			}
			src, err := p.Expand(settled)
			if err != nil {
				t.Fatal(err)
			}
			if err := recipe.CheckBase(src, p.ID); err != nil {
				t.Errorf("%v\nA recipe that builds on this preset takes its targets and owns "+
					"everything else, so the preset may not carry anything else.", err)
			}
		})
	}

	// The predicate itself, on a base the tree does not contain. Every
	// registered preset passes today, so a rule that stopped looking would
	// find nothing to refuse and stay green.
	for _, c := range []struct {
		src string
		bad bool
		why string
	}{
		{"version: 1\ntargets:\n  - id: a\n    format: txt\n    count: 1\n    size: 1\n", false, "a version and targets"},
		{"version: 1\nseed: 3\ntargets:\n  - id: a\n    format: txt\n    count: 1\n    size: 1\n", true, "a seed"},
		{"version: 1\ndefaults:\n  label: false\ntargets:\n  - id: a\n    format: txt\n    count: 1\n    size: 1\n", true, "a defaults section"},
		{"version: 1\noutput:\n  dir: x\ntargets:\n  - id: a\n    format: txt\n    count: 1\n    size: 1\n", true, "an output section"},
	} {
		err := recipe.CheckBase([]byte(c.src), "base")
		if (err != nil) != c.bad {
			t.Errorf("CheckBase on a base carrying %s: got %v, and the rule says refused=%v", c.why, err, c.bad)
		}
	}
}

// A recipe built on a preset and the ejected preset with the same targets
// appended produce the same bytes, and only the first records the preset.
//
// Both roads go through the command line, as a person would take them: one
// file says extends and with, the other is what "tfg preset eject" printed
// with the same batch typed under it and the same seed and defaults above.
// The files are compared byte for byte and the two manifests' records read:
// run.preset is present on the first road, with exactly the parameters the
// file left out listed as defaulted, and absent on the second, because that
// recipe stands alone. The recipe hashes differ, and that is asserted rather
// than avoided - they are two files, and the owner's decision is that the
// hash names the file that was run.
func TestARecipeBuildingOnAPresetGivesTheBytesOfTheEjectedOneWithItsTargetsAppended(t *testing.T) {
	root := t.TempDir()
	fromExtends := filepath.Join(root, "extends")
	fromEject := filepath.Join(root, "eject")

	own := "  - id: mine\n    format: txt\n    count: 2\n    size: 100\n"
	extending := filepath.Join(root, "extending.yaml")
	if err := os.WriteFile(extending, []byte(
		"version: 1\nseed: 7\ndefaults:\n  label: false\n"+
			"extends: preset:size-boundaries\nwith:\n  limit: 4mb\n  format: txt\n"+
			"targets:\n"+own), 0o644); err != nil {
		t.Fatal(err)
	}

	var source, notes bytes.Buffer
	if code := cli.Run(context.Background(), []string{
		"preset", "eject", "size-boundaries", "--limit", "4mb", "--format", "txt",
	}, &source, &notes); code != cli.ExitOK {
		t.Fatalf("eject ended with %d: %s", code, notes.String())
	}
	ejected := filepath.Join(root, "ejected.yaml")
	if err := os.WriteFile(ejected, append(source.Bytes(), []byte(own+"seed: 7\ndefaults:\n  label: false\n")...), 0o644); err != nil {
		t.Fatal(err)
	}

	a := runAndReadManifest(t, fromExtends, []string{"generate", extending, "--out", fromExtends})
	b := runAndReadManifest(t, fromEject, []string{"generate", ejected, "--out", fromEject})

	if a.Run.Preset == nil {
		t.Fatal("the run built on a preset recorded no run.preset, so the manifest does not say where the set came from")
	}
	if got, want := a.Run.Preset.ID, "size-boundaries"; got != want {
		t.Errorf("run.preset.id is %q, want %q", got, want)
	}
	// Two parameters were given and one was not, so exactly one stood in.
	if got := strings.Join(a.Run.Preset.Defaulted, ","); got != "spread" {
		t.Errorf("run.preset.defaulted is %q, and the file gave every parameter but spread", got)
	}
	if b.Run.Preset != nil {
		t.Errorf("the run from the ejected recipe recorded run.preset %+v, and that recipe stands alone", b.Run.Preset)
	}
	if a.Run.RecipeHash == b.Run.RecipeHash {
		t.Errorf("both runs recorded the hash %s, and they were two different files", a.Run.RecipeHash)
	}

	// The preset's targets first and the file's own after them, in the
	// manifest as in the recipe - the order is part of what a consumer
	// reads, and swapping it would leave every byte in place.
	if first, last := a.Files[0].TargetID, a.Files[len(a.Files)-1].TargetID; first != "under_1mb" || last != "mine" {
		t.Errorf("the manifest lists %s first and %s last, and the preset's targets come before the file's own", first, last)
	}

	// The bytes themselves. Nine files, seven from the preset and two typed.
	compared := 0
	for _, f := range a.Files {
		x, err := os.ReadFile(filepath.Join(fromExtends, f.Path))
		if err != nil {
			t.Fatal(err)
		}
		y, err := os.ReadFile(filepath.Join(fromEject, f.Path))
		if err != nil {
			t.Errorf("%s came from the recipe building on the preset and not from the ejected one", f.Path)
			continue
		}
		if !bytes.Equal(x, y) {
			t.Errorf("%s differs between the two roads, so building on a preset is not the same run as ejecting it", f.Path)
		}
		compared++
	}
	if compared != 9 {
		t.Fatalf("%d files were compared and the two recipes describe nine", compared)
	}
}

// The batch screen, with the switch on, produces the bytes the file produces.
//
// The window is the third road to the same run, and it is pressed rather
// than looked at: the switch is turned on, the limit typed, the format
// chosen, one batch filled in, Generate pressed, and the manifest read back
// and compared with the one the command line wrote from the equivalent file.
// A screen that drew the section and dropped the choice on the way to the
// engine would produce seven files fewer and a different record.
func TestARecipeBuiltOnAPresetFromTheWindowGivesTheBytesTheFileGives(t *testing.T) {
	root := t.TempDir()
	fromWindow := filepath.Join(root, "window")
	fromFile := filepath.Join(root, "file")

	host := newFakeHost(t)
	screen := window.NewRecipe(host)
	content := screen.Object()
	t.Cleanup(func() { join(host) })

	// Turned on through the control itself, which fires the same change a
	// press does and rebuilds the form with the preset's fields on it.
	switchOn := checkNamed(content, text.FieldBuildOnPreset())
	if switchOn == nil {
		t.Fatal("there is no switch to build on a preset on the batch screen")
	}
	switchOn.SetChecked(true)
	fields := screen.Fields()
	setBox(t, fields, recipe.KeyWith+".limit", "4mb")
	chooserIn(t, fields, recipe.KeyWith+".format").SetSelected("txt")
	setBox(t, fields, recipe.TargetAddress(1, recipe.KeyID), "mine")
	chooserIn(t, fields, recipe.TargetAddress(1, recipe.KeyFormat)).SetSelected("txt")
	setBox(t, fields, recipe.TargetAddress(1, recipe.KeySize), "100")
	setBox(t, fields, recipe.KeySeed, "7")
	entryUnder(t, content, text.FieldOutputDir()).SetText(fromWindow)
	press(t, content, text.ButtonGenerate())
	join(host)

	// The label switch on this screen starts off, so the file says so too -
	// a recipe with no defaults section has the label on, and the two would
	// differ in every byte for a reason that is not this section's.
	file := filepath.Join(root, "same.yaml")
	if err := os.WriteFile(file, []byte(
		"version: 1\nseed: 7\ndefaults:\n  label: false\nextends: preset:size-boundaries\nwith:\n  limit: 4mb\n  format: txt\n"+
			"targets:\n  - id: mine\n    format: txt\n    count: 1\n    size: 100\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b := runAndReadManifest(t, fromFile, []string{"generate", file, "--out", fromFile})
	a := wholeManifest(t, filepath.Join(fromWindow, "manifest.json"))

	if a.Run.Preset == nil || a.Run.Preset.ID != "size-boundaries" {
		t.Fatalf("the window recorded run.preset %+v, so the switch never reached the recipe", a.Run.Preset)
	}
	if got := strings.Join(a.Run.Preset.Defaulted, ","); got != "spread" {
		t.Errorf("the window recorded defaulted %q, and only spread was left alone", got)
	}
	if len(a.Files) != len(b.Files) || len(a.Files) != 8 {
		t.Fatalf("the window wrote %d files and the file %d, and both describe eight", len(a.Files), len(b.Files))
	}
	for i := range a.Files {
		if a.Files[i].Path != b.Files[i].Path || a.Files[i].Hashes.SHA256 != b.Files[i].Hashes.SHA256 {
			t.Errorf("file %d: the window wrote %s %s and the file %s %s", i,
				a.Files[i].Path, a.Files[i].Hashes.SHA256, b.Files[i].Path, b.Files[i].Hashes.SHA256)
		}
	}
}

// Every refusal about the preset side of a recipe names the line it is about.
//
// A screen marks a box by the address a refusal carries and the command line
// lists it, so a refusal about with.limit that arrived addressed to nothing
// would fall to the foot of the form. Each case is one thing a person can
// write wrong, with the address and a phrase the refusal has to carry.
func TestEveryRefusalAboutThePresetSideOfARecipeNamesItsLine(t *testing.T) {
	for _, c := range []struct {
		name string
		src  string
		at   string
		says string
	}{
		{"an unknown preset", "version: 1\nextends: preset:nope\n", "extends", "this build does not have"},
		{"a scheme this build lacks", "version: 1\nextends: other.yaml\n", "extends", "does not name a preset"},
		{"with and no extends", "version: 1\nwith:\n  limit: 1kb\ntargets:\n  - id: a\n    format: txt\n    count: 1\n    size: 1\n", "with", "no preset is named"},
		{"an unknown parameter", "version: 1\nextends: preset:size-boundaries\nwith:\n  spreed: 1kb\n", "with.spreed", "does not take"},
		{"a value the parameter refuses", "version: 1\nextends: preset:size-boundaries\nwith:\n  limit: banana\n", "with.limit", "cannot be \"banana\""},
		{"a set the preset cannot build", "version: 1\nextends: preset:size-boundaries\nwith:\n  limit: 1kb\n", "with.limit", "cannot build this set"},
		{"a parameter written as a list", "version: 1\nextends: preset:size-boundaries\nwith:\n  spread: [1kb, 2kb]\n", "with.spread", "written as a list"},
		{"a format this build lacks", "version: 1\nextends: preset:size-boundaries\nwith:\n  format: nope\n", "with.format", "does not have"},
		{"an id the preset already uses", "version: 1\nextends: preset:size-boundaries\ntargets:\n  - id: at_limit\n    format: txt\n    count: 1\n    size: 1\n", "targets[1].id", "already builds"},
		{"a preset's targets handed to a plain reader", "version: 1\nextends: preset:size-boundaries\n", "extends", "were not supplied"},
	} {
		t.Run(c.name, func(t *testing.T) {
			var err error
			if strings.HasPrefix(c.name, "a preset's targets handed") {
				_, err = recipe.Parse([]byte(c.src), c.name)
			} else {
				_, err = preset.ReadRecipe([]byte(c.src), c.name)
			}
			var invalid *recipe.ValidationError
			if !errors.As(err, &invalid) {
				t.Fatalf("got %v, and this recipe has to be refused as invalid", err)
			}
			found := false
			for _, p := range invalid.Problems {
				if p.At == c.at && strings.Contains(p.String(), c.says) {
					found = true
				}
			}
			if !found {
				t.Errorf("no problem is addressed to %q saying %q. The problems are:\n%v", c.at, c.says, err)
			}
			for _, p := range invalid.Problems {
				if p.Why == "" || p.Fix == "" {
					t.Errorf("the problem %q arrives without all four parts (D6): why=%q fix=%q", p.What, p.Why, p.Fix)
				}
			}
		})
	}
}

// A recipe file is read through the door that knows presets, on both surfaces.
//
// recipe.Parse cannot expand a preset and refuses a file that names one, so
// a surface reading a file through it would turn away every recipe built on
// a preset with a sentence about a reader. The three places that read
// expansions rather than files - the budget of a preset, a --preset run, and
// the preset screen - are named by file, and everything else in cli and gui
// has to go through preset.ReadRecipe. Read from the source, the way the
// layer guard reads imports.
func TestEveryRecipeFileIsReadThroughTheDoorThatKnowsPresets(t *testing.T) {
	root := repoRoot(t)
	// The files that parse an EXPANSION, which has no extends by the rule
	// above, and so may use Parse.
	allowed := map[string]bool{
		filepath.Join("internal", "cli", "preset.go"):           true,
		filepath.Join("internal", "gui", "window", "preset.go"): true,
	}
	seen := 0
	for _, dir := range []string{"internal/cli", "internal/gui"} {
		err := filepath.Walk(filepath.Join(root, dir), func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			for i, line := range strings.Split(string(body), "\n") {
				if !strings.Contains(line, "recipe.Parse(") || strings.HasPrefix(strings.TrimSpace(line), "//") {
					continue
				}
				seen++
				if allowed[rel] {
					continue
				}
				t.Errorf("%s:%d reads a recipe through recipe.Parse, which cannot expand the preset a "+
					"file may build on. Read it through preset.ReadRecipe, which can.", rel, i+1)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	// The three allowed call sites are read today. A walk that found none
	// would have stopped seeing the call rather than proved the rule.
	if seen < 3 {
		t.Errorf("%d calls of recipe.Parse were found under cli and gui, and there are three that parse expansions - "+
			"either they moved or the way this reads them stopped working", seen)
	}
}

// runAndReadManifest runs the command line and reads back what it wrote.
func runAndReadManifest(t *testing.T, dir string, args []string) manifest.Manifest {
	t.Helper()
	var out, errOut bytes.Buffer
	if code := cli.Run(context.Background(), args, &out, &errOut); code != cli.ExitOK {
		t.Fatalf("%v ended with %d: %s", args, code, errOut.String())
	}
	return wholeManifest(t, filepath.Join(dir, "manifest.json"))
}

// wholeManifest reads a manifest as the manifest package spells it, the whole
// of it rather than the narrow view the recipe guards share - run.preset is
// what these guards ask about and that view does not carry it.
func wholeManifest(t *testing.T, path string) manifest.Manifest {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m manifest.Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}
