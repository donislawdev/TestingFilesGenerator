package guard

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// notYet is a format whose cut does not take the share of its content yet:
// why, and a file it misses the share in - its size and the share cut.
type notYet struct {
	why     string
	size    int64
	missing int64
}

// notYetCutAtTheShare is every format whose cut does not yet take the share
// of the content that missing asks for (docs/TRUNCATE-2026-11-11.md section
// 7.1, measured 2026-11-12).
//
// A ratchet rather than a list of exceptions: each format here is cut once, in
// the file named, and a format that takes the share there is red until it
// leaves the list, so the list cannot go stale in either direction. A format
// that leaves is held to every size and share, and so is a new format unless
// it is added here with its reason.
var notYetCutAtTheShare = map[string]notYet{
	"gif":   {paddingBehind, 64 << 10, 50},
	"avif":  {paddingBehind, 64 << 10, 50},
	"jxl":   {paddingBehind, 64 << 10, 50},
	"mp4":   {paddingBehind, 64 << 10, 50},
	"webm":  {paddingBehind, 64 << 10, 50},
	"pdf":   {paddingBehind, 64 << 10, 50},
	"zip":   {paddingBehind, 64 << 10, 50},
	"docx":  {paddingBehind, 64 << 10, 50},
	"xlsx":  {paddingBehind, 64 << 10, 50},
	"pptx":  {paddingBehind, 64 << 10, 50},
	"targz": {paddingBehind, 64 << 10, 50},
	"jpg":   {paddingInFront, 64 << 10, 50},
	"ico":   {paddingInFront, 64 << 10, 50},
	// Both take the share at 64 KiB and above. At 4 KiB what stands behind the
	// picture is more than a small share, so a cut of 1% keeps all of it.
	"webp": {smallBehind, 4 << 10, 1},
	"tiff": {smallBehind, 4 << 10, 1},
}

const (
	paddingBehind  = "its padding stands only behind its content, so a large file cut at any share keeps all of it"
	paddingInFront = "its padding stands only in front of its content, so a large file cut at any share keeps none of it"
	smallBehind    = "in a small file the padding or directory behind its content is more than a small share"
)

// shareSlack is how far, in points of a percent, the share of the content a
// cut takes may be from the share missing asks for. Rounding to whole chunks,
// frames and records moves it by a few bytes, and a header or a short tail
// beside the content moves the share of the content from the share of the
// file by a little more in a small file.
const shareSlack = 1.0

// shareSizes and shareMissing are the files the sweep cuts of a format held to
// the share: two sizes, so a picture planned with and without measuring it is
// cut, and three shares.
//
// 64 KiB is already a file of mostly padding in every format that pads behind
// its content - png keeps a picture of about 6 KB in it. Measured 2026-11-12:
// each format listed for where its padding stands misses the share at 64 KiB
// as it does at 10 MiB, so a megabyte would cost the race job of CI about a
// minute and say nothing new.
var (
	shareSizes   = []int64{4 << 10, 64 << 10}
	shareMissing = []int64{1, 50, 90}
)

// A cut takes the share of the file's content that missing asks for, in every
// format that does not say otherwise.
//
// The meaning of missing a person reads off the result: half of the picture,
// half of the sound, half of the words. A cut that falls in padding takes
// nothing a reader sees whatever the share, and in a large file most of the
// bytes can be padding (docs/TRUNCATE-2026-11-11.md section 1). The manifest's
// cut is the measurement, and the map behind it is held to the file's
// structure by TestEveryFormatsMapIsTheOneItsStructureReadsBack.
//
// Canaries: a cut that kept all its content must be reported, and so must a
// format listed as not yet cutting at the share that does.
func TestACutTakesTheShareOfTheContentItIsAskedFor(t *testing.T) {
	missingOf := map[string]int64{}
	var targets []engine.Target
	for _, f := range format.All() {
		for _, c := range shareCases(f.ID, notYetCutAtTheShare) {
			chain := damage.Chain{{ID: damage.Truncate, Values: damage.Values{damage.SettingMissing: strconv.FormatInt(c[1], 10)}}}
			if c[0] < engine.SmallestDamaged(f, chain, format.Request{Label: true}) {
				continue
			}
			id := shareID(f.ID, c)
			missingOf[id] = c[1]
			targets = append(targets, engine.Target{ID: id, Format: f.ID, Sizes: engine.Uniform(1, c[0]), Label: true, Damage: chain})
		}
	}
	opt := engine.Options{OutDir: t.TempDir(), Seed: 7, ManifestName: engine.DefaultManifestName}
	planned, err := engine.Plan(targets, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}
	res, err := engine.Run(t.Context(), planned, opt)
	if err != nil {
		t.Fatalf("running: %v", err)
	}

	problems := map[string]string{}
	for _, m := range res.Manifest.Files {
		missing, ok := missingOf[m.TargetID]
		if !ok || m.Failed {
			t.Errorf("%s: no file was written (%s)", m.TargetID, m.Error)
			continue
		}
		problems[m.TargetID] = shareProblem(missing, m)
	}
	for _, problem := range shareVerdicts(problems, notYetCutAtTheShare) {
		t.Error(problem)
	}

	// Canaries.
	whole := int64(1000)
	kept := manifest.File{Damage: []manifest.Damage{{Cut: &manifest.Cut{In: "content", ContentBytes: &whole, ContentKeptBytes: whole}}}}
	if shareProblem(50, kept) == "" {
		t.Errorf("canary: a cut that kept all its content passed for one that took half")
	}
	listed := maps.Clone(notYetCutAtTheShare)
	listed["txt"] = notYet{paddingBehind, 64 << 10, 50}
	if !slices.ContainsFunc(shareVerdicts(problems, listed), func(s string) bool { return strings.HasPrefix(s, "txt:") }) {
		t.Errorf("canary: txt listed as not cutting at the share, which it does, was not reported")
	}
}

// shareCases is the files the sweep cuts of a format, each as its size and the
// share cut: every size and share, or the one file named for a format listed
// as not taking the share yet.
func shareCases(id string, notYetAt map[string]notYet) [][2]int64 {
	if n, listed := notYetAt[id]; listed {
		return [][2]int64{{n.size, n.missing}}
	}
	var out [][2]int64
	for _, size := range shareSizes {
		for _, missing := range shareMissing {
			out = append(out, [2]int64{size, missing})
		}
	}
	return out
}

func shareID(format string, c [2]int64) string {
	return fmt.Sprintf("%s-%d-%d", format, c[0], c[1])
}

// shareVerdicts is every problem with the sweep, from what each file cut got
// wrong by its id, empty when it took the share: a format that cut nothing, a
// format held to the share that missed it, and a format listed as not yet
// cutting at the share that took it.
func shareVerdicts(problems map[string]string, notYetAt map[string]notYet) []string {
	var out []string
	for _, f := range format.All() {
		cut := 0
		var missed []string
		for _, c := range shareCases(f.ID, notYetAt) {
			id := shareID(f.ID, c)
			problem, ok := problems[id]
			if !ok {
				continue
			}
			cut++
			if problem != "" {
				missed = append(missed, id+": "+problem)
			}
		}
		_, listed := notYetAt[f.ID]
		switch {
		case cut == 0:
			out = append(out, f.ID+": no file of it was cut, so the share was never asked")
		case listed && len(missed) == 0:
			out = append(out, f.ID+": took the share in the file notYetCutAtTheShare says it misses it in - take it off the list")
		case !listed:
			out = append(out, missed...)
		}
	}
	for id := range notYetAt {
		if _, err := format.Get(id); err != nil {
			out = append(out, id+": listed in notYetCutAtTheShare and not a format")
		}
	}
	return out
}

// shareProblem says how the cut of f misses the share missing asks for, or
// nothing when it takes it.
func shareProblem(missing int64, f manifest.File) string {
	if len(f.Damage) != 1 || f.Damage[0].Cut == nil {
		return fmt.Sprintf("no cut is reported: %+v", f.Damage)
	}
	cut := f.Damage[0].Cut
	if cut.In != "content" {
		return fmt.Sprintf("the cut fell in the %s", cut.In)
	}
	if cut.ContentBytes == nil || *cut.ContentBytes <= 0 {
		return "the length of the content is not stated"
	}
	whole := *cut.ContentBytes
	gone := 100 * float64(whole-cut.ContentKeptBytes) / float64(whole)
	if gone < float64(missing)-shareSlack || gone > float64(missing)+shareSlack {
		return fmt.Sprintf("%.2f%% of the content is gone, where %d%% was asked for", gone, missing)
	}
	return ""
}
