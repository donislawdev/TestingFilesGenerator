package guard

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// lengthSeed is the run seed these guards write with. Any number does, as long
// as the run being compared against uses the same one.
const lengthSeed = 11

// lengthRun plans and writes targets in one run and answers each target's
// manifest entry and path by its id.
func lengthRun(t *testing.T, out string, targets []engine.Target) (map[string]manifest.File, map[string]string) {
	t.Helper()
	opt := engine.Options{OutDir: out, Seed: lengthSeed, ManifestName: engine.DefaultManifestName}
	planned, err := engine.Plan(targets, opt)
	if err != nil {
		t.Fatalf("planning in %s: %v", out, err)
	}
	var sum int64
	for _, tg := range targets {
		for _, s := range tg.Sizes {
			sum += s
		}
	}
	if got := engine.TotalBytes(planned); got != sum {
		t.Errorf("the plan in %s totals %d B and the sizes asked for add up to %d B - the free space check, the progress bar and --dry-run would all say the wrong number",
			out, got, sum)
	}
	res, err := engine.Run(t.Context(), planned, opt)
	if err != nil {
		t.Fatalf("running in %s: %v", out, err)
	}
	entries, paths := map[string]manifest.File{}, map[string]string{}
	for _, m := range res.Manifest.Files {
		if m.Failed {
			t.Fatalf("target %s failed: %s", m.TargetID, m.Error)
		}
		entries[m.TargetID] = m
		paths[m.TargetID] = filepath.Join(out, filepath.FromSlash(m.Path))
	}
	return entries, paths
}

// lengthChangers is every damage that changes the length of what it is given,
// each at its defaults - which a guard has to ASSERT is not empty, or it
// would pass with nothing to judge.
func lengthChangers(t *testing.T) []damage.Chain {
	t.Helper()
	var out []damage.Chain
	for _, d := range damage.All() {
		if d.Length != nil {
			out = append(out, damage.Chain{{ID: d.ID}})
		}
	}
	if len(out) < 2 {
		t.Fatalf("%d damage(s) change the length, and truncate and append-junk both do - this guard would judge less than it claims", len(out))
	}
	return out
}

// TestADamagedFileIsExactlyTheSizeAskedFor holds untouchable rule 1 for every
// damage that changes the length of a file.
//
// Such a damage makes the generator write a different length from the one on
// disk, and until 2026-11-10 six places read the plan's count as the size of
// the file - the manifest, the length check, progress, the run's total, the
// free space check and --dry-run. One of them reading the wrong number gives a
// manifest that says 40 kB about a 20 kB file, or a file refused for being the
// size it was asked to be. So every format meets every such damage, at the
// smallest size the pair takes and at 20 kB, and the file on disk, the
// manifest and the plan's total all have to say the size that was asked for.
//
// A drawn size is asked too, in the formats with sizes they cannot reach - a
// band above a PNG's picture, odd counts in UTF-16 - because the draw has to
// ask the format about the size the damage starts from, not the one on disk.
func TestADamagedFileIsExactlyTheSizeAskedFor(t *testing.T) {
	dir := t.TempDir()
	judged := 0
	for _, f := range format.All() {
		var targets []engine.Target
		for n, chain := range lengthChangers(t) {
			smallest := engine.SmallestDamaged(f, chain, format.Request{Label: true})
			for _, size := range witnessSizes(smallest) {
				targets = append(targets, engine.Target{ID: fmt.Sprintf("d%d-%d", n, size), Format: f.ID,
					Sizes: engine.Uniform(1, size), Damage: chain, Label: true})
			}
		}
		entries, paths := lengthRun(t, filepath.Join(dir, f.ID), targets)
		for _, tg := range targets {
			judged += checkLength(t, tg, entries[tg.ID], paths[tg.ID], tg.Sizes[0])
		}
	}

	for _, c := range []struct {
		format     string
		properties map[string]string
		min, max   int64
	}{
		{"png", nil, 150, 400},
		{"docx", nil, 1300, 4000},
		{"xml", map[string]string{"encoding": "utf-16le", "bom": "true"}, 600, 900},
	} {
		for n, chain := range lengthChangers(t) {
			tg := engine.Target{ID: fmt.Sprintf("range%d", n), Format: c.format, Properties: c.properties,
				Range: engine.SizeRange{Used: true, Min: c.min, Max: c.max}, Sizes: engine.Uniform(40, 0),
				Damage: chain, Label: true}
			out := filepath.Join(dir, fmt.Sprintf("%s-range-%d", c.format, n))
			opt := engine.Options{OutDir: out, Seed: lengthSeed, ManifestName: engine.DefaultManifestName}
			planned, err := engine.Plan([]engine.Target{tg}, opt)
			if err != nil {
				t.Errorf("%s %s: a range of %d to %d B was refused: %v", c.format, chain, c.min, c.max, err)
				continue
			}
			res, err := engine.Run(t.Context(), planned, opt)
			if err != nil {
				t.Fatalf("%s %s: %v", c.format, chain, err)
			}
			for i, m := range res.Manifest.Files {
				judged += checkLength(t, tg, m, filepath.Join(out, filepath.FromSlash(m.Path)), planned[i].Plan.Bytes)
				if m.Bytes < c.min || m.Bytes > c.max {
					t.Errorf("%s %s: a file of %d B was drawn from the range %d to %d B", c.format, chain, m.Bytes, c.min, c.max)
				}
			}
		}
	}
	if judged == 0 {
		t.Fatal("no damaged file was measured, so this guard proved nothing")
	}
}

// checkLength is one damaged file against the size it was asked to be, and
// answers 1 for a file it measured.
func checkLength(t *testing.T, tg engine.Target, m manifest.File, path string, want int64) int {
	t.Helper()
	where := fmt.Sprintf("%s with %s", tg.Format, tg.Damage)
	info, err := os.Stat(path)
	if err != nil {
		t.Errorf("%s: %v", where, err)
		return 0
	}
	if info.Size() != want || m.Bytes != want {
		t.Errorf("%s: asked for %d B, the file on disk is %d B and the manifest says %d B", where, want, info.Size(), m.Bytes)
	}
	return 1
}

// TestATruncatedFileIsTheStartOfTheWholeOne holds D11 for truncate, and the
// early stop with it.
//
// A truncated file is the first bytes of the whole file the same recipe would
// write at the size the cut starts from - byte for byte, in every format. That
// is what makes the damage reproducible from its recipe, and it is also what
// proves stopping the generator early changes nothing: a generator writes from
// the front, so the bytes before the stop are the bytes it would have written
// anyway. A generator that wrote differently once stopped would turn this red.
//
// The whole file comes from a second run with the same seed and target id, so
// both files are drawn from the same seed. A size the cut would have to move
// up to - a band the format cannot write - is skipped, and the guard asserts
// that most formats were compared.
func TestATruncatedFileIsTheStartOfTheWholeOne(t *testing.T) {
	dir := t.TempDir()
	chain := damage.Chain{{ID: damage.Truncate}}
	const size = witnessSize
	whole, err := chain.Before(size)
	if err != nil {
		t.Fatal(err)
	}
	compared := 0
	for _, f := range format.All() {
		cut := engine.Target{ID: "same", Format: f.ID, Sizes: engine.Uniform(1, size), Damage: chain, Label: true}
		full := engine.Target{ID: "same", Format: f.ID, Sizes: engine.Uniform(1, whole), Label: true}
		if _, err := engine.Plan([]engine.Target{full}, engine.Options{OutDir: t.TempDir(), Seed: lengthSeed}); err != nil {
			// A size this format cannot write exactly: the cut started from
			// the next one it can, which is a different whole file.
			continue
		}
		_, cutPath := lengthRun(t, filepath.Join(dir, f.ID, "cut"), []engine.Target{cut})
		_, fullPath := lengthRun(t, filepath.Join(dir, f.ID, "whole"), []engine.Target{full})
		got, err := os.ReadFile(cutPath["same"])
		if err != nil {
			t.Fatal(err)
		}
		all, err := os.ReadFile(fullPath["same"])
		if err != nil {
			t.Fatal(err)
		}
		if int64(len(all)) != whole || !bytes.Equal(got, all[:size]) {
			t.Errorf("%s: the truncated file is not the first %d B of the %d B file the same recipe writes", f.ID, size, whole)
		}
		compared++
	}
	if compared < len(format.All())*3/4 {
		t.Errorf("only %d of %d formats were compared, so this guard says little", compared, len(format.All()))
	}
}

// TestAppendedJunkIsTheWholeFileAndThenTheSameBytesEveryTime holds D11 and
// RC4 for append-junk.
//
// A file with junk appended is the whole file the same recipe writes at the
// smaller size, followed by the junk - nothing before the end moves. And the
// junk is frozen: drawn from the damage's own seed, derived from the file's,
// in a way the golden hash below pins. Changing how the bytes are drawn would
// change every file this damage ever wrote, which is a major version and a
// Breaking entry - this is the guard that says so.
func TestAppendedJunkIsTheWholeFileAndThenTheSameBytesEveryTime(t *testing.T) {
	dir := t.TempDir()
	chain := damage.Chain{{ID: damage.AppendJunk}}
	const size = witnessSize
	before, err := chain.Before(size)
	if err != nil {
		t.Fatal(err)
	}
	junk := size - before

	broken := engine.Target{ID: "same", Format: "txt", Sizes: engine.Uniform(1, size), Damage: chain, Label: true}
	whole := engine.Target{ID: "same", Format: "txt", Sizes: engine.Uniform(1, before), Label: true}
	_, brokenPath := lengthRun(t, filepath.Join(dir, "broken"), []engine.Target{broken})
	_, wholePath := lengthRun(t, filepath.Join(dir, "whole"), []engine.Target{whole})
	got, err := os.ReadFile(brokenPath["same"])
	if err != nil {
		t.Fatal(err)
	}
	all, err := os.ReadFile(wholePath["same"])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got[:before], all) {
		t.Errorf("the first %d B of the damaged file are not the %d B file the same recipe writes", before, before)
	}

	// The junk is the damage's own draw for this file, and nothing else.
	fileSeed := core.FileSeed(core.TargetSeed(lengthSeed, "same"), 0)
	var tail bytes.Buffer
	w, streams, err := chain.Open(&tail, junk, fileSeed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(nil); err != nil {
		t.Fatal(err)
	}
	if err := chain.Finish(streams); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got[before:], tail.Bytes()) {
		t.Error("the bytes after the whole file are not the damage's own draw for this file's seed")
	}

	const golden = "62f8bf77f16ef1828d49e40899c817b3f7d811a242b03b6671a9b23827935f40"
	sum := sha256.Sum256(got[before:])
	if hex.EncodeToString(sum[:]) != golden {
		t.Errorf("the junk appended to the same recipe hashes to %s, and it was %s.\n"+
			"Reason: the bytes are frozen from the first release (D11). A change here changes every\n"+
			"file append-junk ever wrote - a major version and a Breaking entry, not a quiet edit.",
			hex.EncodeToString(sum[:]), golden)
	}
}

// TestTheSmallestDamagedFileIsTakenAndOneByteLessIsNot asks every format and
// every damage that changes the length about the floor from both sides.
//
// The format's own minimum applies to the file the damage STARTS from, so a
// truncated PNG can be smaller than any healthy one and a PNG with junk
// appended has to be larger. The size a person sees is the one on disk, and
// the refusal has to name the smallest one that works in that size - not the
// size the generator was asked for, which nobody wrote. Planning only, so
// every pair is asked.
func TestTheSmallestDamagedFileIsTakenAndOneByteLessIsNot(t *testing.T) {
	asked := 0
	for _, f := range format.All() {
		for _, chain := range lengthChangers(t) {
			smallest := engine.SmallestDamaged(f, chain, format.Request{Label: true})
			plan := func(size int64) error {
				_, err := engine.Plan([]engine.Target{{ID: "t", Format: f.ID, Sizes: engine.Uniform(1, size),
					Damage: chain, Label: true}}, engine.Options{OutDir: t.TempDir(), Seed: lengthSeed})
				return err
			}
			if err := plan(smallest); err != nil {
				t.Errorf("%s with %s: %d B is announced as the smallest and was refused: %v", f.ID, chain, smallest, err)
				continue
			}
			asked++
			if smallest == 0 {
				continue
			}
			// One byte under, and far under: a refusal has to name the size
			// that works from anywhere below it, not a floor of the damage
			// that the format then refuses in turn.
			for _, under := range []int64{smallest - 1, smallest / 3} {
				if under == smallest-1 || under > 0 {
					checkNamesTheSmallest(t, f.ID, chain, under, smallest, plan(under))
				}
			}
		}
	}
	if asked == 0 {
		t.Fatal("no pair was asked, so this guard proved nothing")
	}
}

// checkNamesTheSmallest is one size under the smallest, refused in words that
// name the smallest.
func checkNamesTheSmallest(t *testing.T, formatID string, chain damage.Chain, under, smallest int64, err error) {
	t.Helper()
	var below *format.BelowMinimumError
	var small *damage.TooSmallError
	switch {
	case err == nil:
		t.Errorf("%s with %s: %d B was taken, under the %d B announced as the smallest", formatID, chain, under, smallest)
	case errors.As(err, &below):
		if below.Minimum != smallest || below.Requested != under {
			t.Errorf("%s with %s: the refusal of %d B names %d B as the smallest and %d B as asked for: %v",
				formatID, chain, under, below.Minimum, below.Requested, err)
		}
	case errors.As(err, &small):
		if small.Floor != smallest {
			t.Errorf("%s with %s: the refusal of %d B names %d B as the smallest: %v", formatID, chain, under, small.Floor, err)
		}
	default:
		t.Errorf("%s with %s: %d B was refused with %T, which names no size: %v", formatID, chain, under, err, err)
	}
}
