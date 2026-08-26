package guard

import (
	"context"
	"errors"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
)

// A boundary set exists so three files can be told apart, and until 2026-08-03
// they could not be. They arrived as files_0001, files_0002 and files_0003,
// and the only way to learn which one sat on the limit was to read the sizes
// back off the disk - which is exactly what somebody dragging one into an
// upload form is not going to do.
//
// Reported from manual testing. The sizes were always right.

func boundaryTarget(t *testing.T, id string, limit int64) engine.Target {
	t.Helper()
	sizes, err := core.BoundarySizes(limit)
	if err != nil {
		t.Fatalf("building the boundary set for %d: %v", limit, err)
	}
	return engine.Target{
		ID:            id,
		Format:        "txt",
		Sizes:         sizes,
		BoundaryLimit: limit,
		Label:         true,
	}
}

func TestABoundarySetSaysWhichFileIsWhich(t *testing.T) {
	const limit = 1 << 20
	dir := t.TempDir()

	opt := engine.Options{OutDir: dir, Seed: 7741, Command: "test",
		ManifestName: engine.DefaultManifestName}
	planned, err := engine.Plan([]engine.Target{boundaryTarget(t, "files", limit)}, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}
	if _, err := engine.Run(context.Background(), planned, opt); err != nil {
		t.Fatalf("the run failed: %v", err)
	}

	// The name has to match the size it carries. A set that labels the files
	// in the wrong order is worse than one that does not label them, because
	// it is believed.
	want := map[string]int64{
		"files_under_1b.txt": limit - 1,
		"files_at_limit.txt": limit,
		"files_over_1b.txt":  limit + 1,
	}
	for name, size := range want {
		info, err := os.Stat(dir + string(os.PathSeparator) + name)
		if err != nil {
			t.Errorf("no file called %s - a boundary set that does not name its "+
				"three files leaves the sizes as the only way to tell them apart", name)
			continue
		}
		if info.Size() != size {
			t.Errorf("%s is %d B and its name says it should be %d B", name, info.Size(), size)
		}
	}
}

// A name the user wrote still wins. Otherwise the fix for one complaint
// quietly takes away a feature somebody was already relying on.
func TestABoundarySetStillObeysANameTemplate(t *testing.T) {
	dir := t.TempDir()

	target := boundaryTarget(t, "files", 1<<20)
	target.NameTmpl = "invoice_{index:04}.txt"

	opt := engine.Options{OutDir: dir, Seed: 7741, Command: "test",
		ManifestName: engine.DefaultManifestName}
	planned, err := engine.Plan([]engine.Target{target}, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}
	if _, err := engine.Run(context.Background(), planned, opt); err != nil {
		t.Fatalf("the run failed: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading the directory: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), "_limit") {
			t.Errorf("%s was named by the boundary set even though a template was given", e.Name())
		}
	}
	for _, want := range []string{"invoice_0001.txt", "invoice_0002.txt", "invoice_0003.txt"} {
		if _, err := os.Stat(dir + string(os.PathSeparator) + want); err != nil {
			t.Errorf("no file called %s, so the template was ignored", want)
		}
	}
}

// An ordinary group is still numbered. Naming everything by role would be the
// other way to break this.
func TestAnOrdinaryGroupIsStillNumbered(t *testing.T) {
	dir := t.TempDir()

	opt := engine.Options{OutDir: dir, Seed: 7741, Command: "test",
		ManifestName: engine.DefaultManifestName}
	planned, err := engine.Plan([]engine.Target{txtTarget("files", 3, 4096)}, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}
	if _, err := engine.Run(context.Background(), planned, opt); err != nil {
		t.Fatalf("the run failed: %v", err)
	}

	if _, err := os.Stat(dir + string(os.PathSeparator) + "files_0001.txt"); err != nil {
		t.Error("an ordinary group stopped being numbered")
	}
}

// Both ends of a boundary limit are refused by the function that builds one.
//
// The upper end always was. The lower end was not, and BoundarySizes(0) handed
// back a file of minus one byte - which never reached anybody only because both
// callers checked it themselves, in two places, in two sentences that had
// already drifted apart by a comma. A third caller would have got the minus
// one, and the window is a third caller in everything but this: it draws a
// boundary box and reaches this through one of the two.
//
// Asked of core rather than through a command, because the point of the change
// is where the rule lives. Found by an outside review,
// docs/CODE-REVIEW-2026-08-23.md section 3.2.
func TestABoundaryLimitIsRefusedAtBothEnds(t *testing.T) {
	for _, c := range []struct {
		about string
		limit int64
		want  error
	}{
		{"nothing below zero", 0, core.ErrBoundaryTooSmall},
		{"nothing below a negative limit either", -1, core.ErrBoundaryTooSmall},
		{"no number above the largest", math.MaxInt64, core.ErrBoundaryTooLarge},
	} {
		t.Run(c.about, func(t *testing.T) {
			sizes, err := core.BoundarySizes(c.limit)
			if !errors.Is(err, c.want) {
				t.Fatalf("BoundarySizes(%d) gave %v, expected %v - and it handed back %v",
					c.limit, err, c.want, sizes)
			}
			if sizes != nil {
				t.Errorf("a refused limit still produced %v", sizes)
			}
		})
	}

	// The smallest limit that works, so the guard above cannot be satisfied by
	// refusing everything. One byte under it is a file of nothing, which is a
	// size this tool produces on purpose.
	sizes, err := core.BoundarySizes(1)
	if err != nil {
		t.Fatalf("a limit of 1 B was refused: %v", err)
	}
	if len(sizes) != 3 || sizes[0] != 0 || sizes[1] != 1 || sizes[2] != 2 {
		t.Errorf("a limit of 1 B gave %v, expected 0, 1 and 2", sizes)
	}
}

// The preset that builds the same set answers the same way.
//
// The rule above lives in core and --boundary asks it. The size-boundaries
// preset lays its set out itself and did not, so the same limit got a different
// answer through the other door. Measured on 2026-08-26:
//
//	--boundary 9223372036854775807      "there is no number above this one"
//	--preset ... --limit 9223372036854775807b
//	                                    "over_1b would be -9223372036854775808 B,
//	                                     and a file cannot be smaller than nothing.
//	                                     Raise the limit above 1051991 B"
//
// The second is an answer about the bottom of the range to a question about the
// top, and the advice points the wrong way - raising a limit that is already
// the largest number there is. Found by an outside review as N4.
func TestThePresetAnswersALimitTooLargeTheSameWayTheFlagDoes(t *testing.T) {
	_, err := preset.Expand("size-boundaries", preset.Args{
		"limit": "9223372036854775807b",
	})
	if !errors.Is(err, core.ErrBoundaryTooLarge) {
		t.Fatalf("the preset gave %v, expected the same refusal --boundary gives", err)
	}

	// And a limit it can build still builds, so this cannot be satisfied by
	// refusing every limit.
	if _, err := preset.Expand("size-boundaries", preset.Args{"limit": "10mb"}); err != nil {
		t.Fatalf("a limit of 10mb was refused: %v", err)
	}
}
