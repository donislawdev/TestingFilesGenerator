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
	// Both take the share at 64 KiB and above. At 4 KiB what stands behind the
	// picture is more than a small share, so a cut of 1% keeps all of it.
	"webp": {smallBehind, 4 << 10, 1},
	"tiff": {smallBehind, 4 << 10, 1},
}

// listedBeforeTheRule is every format notYetCutAtTheShare may name: the ones
// that missed the share on 2026-11-12, when the owner decided that every format
// cuts at the share before the next release and that a format added later does
// from its first commit (docs/TRUNCATE-2026-11-11.md section 3a). It never
// grows, so a new format cannot wait on the ratchet - tools/doctor.py runs this
// guard for it, and it stays red until the format splits its padding for
// format.Request.CutAt. A format that leaves the ratchet leaves this list too,
// so it cannot go back.
var listedBeforeTheRule = []string{"webp", "tiff"}

// listedAfterTheRule is a problem for every format notYetAt names that
// listedBeforeTheRule does not.
func listedAfterTheRule(notYetAt map[string]notYet) []string {
	var out []string
	for id := range notYetAt {
		if !slices.Contains(listedBeforeTheRule, id) {
			out = append(out, id+": listed in notYetCutAtTheShare after 2026-11-12 - a format added since cuts at the share from its first commit, so split its padding for format.Request.CutAt instead (docs/TRUNCATE-2026-11-11.md section 3a)")
		}
	}
	slices.Sort(out)
	return out
}

const (
	paddingBehind = "its padding stands only behind its content, so a large file cut at any share keeps all of it"
	smallBehind   = "in a small file the padding or directory behind its content is more than a small share"
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
// format listed as not yet cutting at the share that does, and a file planned
// for the sweep that the manifest leaves out.
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

	plannedOf := map[string]engine.PlannedFile{}
	for _, f := range planned {
		plannedOf[f.Target.ID] = f
	}
	problems := map[string]string{}
	inManifest := map[string]bool{}
	for _, m := range res.Manifest.Files {
		inManifest[m.TargetID] = true
		missing, ok := missingOf[m.TargetID]
		if !ok || m.Failed {
			t.Errorf("%s: no file was written (%s)", m.TargetID, m.Error)
			continue
		}
		problem := shareProblem(missing, m)
		f := plannedOf[m.TargetID]
		if _, closes := endClosesTheFile[f.Desc.ID]; closes && problem == cutInTheEnd {
			// The end the format has to close with is longer than what the
			// cut takes, and nothing else stands behind the content that the
			// format could have put in front of it. Both read off the whole
			// file's map, not the manifest the cut file's map wrote.
			whole := wholeMap(t, f)
			_, at, cuts := f.Target.Damage.Cut(f.Plan.Bytes)
			if cuts && cutFallsInTheEnd(whole, at) && onlyTheEndBehind(whole) {
				problem = ""
			}
		}
		problems[m.TargetID] = problem
	}
	for _, problem := range shareVerdicts(problems, notYetCutAtTheShare) {
		t.Error(problem)
	}
	for _, problem := range absentFromTheManifest(missingOf, inManifest) {
		t.Error(problem)
	}
	if len(absentFromTheManifest(map[string]int64{"webp-4096-1": 1}, map[string]bool{})) != 1 {
		t.Errorf("canary: a planned file the manifest left out passed")
	}
	for _, problem := range listedAfterTheRule(notYetCutAtTheShare) {
		t.Error(problem)
	}
	if len(listedAfterTheRule(map[string]notYet{"newformat": {paddingBehind, 64 << 10, 50}})) != 1 {
		t.Errorf("canary: a format listed that was not listed on 2026-11-12 passed")
	}

	// Canaries.
	whole := int64(1000)
	kept := manifest.File{Damage: []manifest.Damage{{Cut: &manifest.Cut{In: "content", ContentBytes: &whole, ContentKeptBytes: whole}}}}
	if shareProblem(50, kept) == "" {
		t.Errorf("canary: a cut that kept all its content passed for one that took half")
	}
	region := func(role format.Role, at, length int64) format.Region {
		return format.Region{Role: role, At: at, Length: length}
	}
	if onlyTheEndBehind([]format.Region{region(format.RoleContent, 0, 10), region(format.RolePadding, 10, 5), region(format.RoleEnd, 15, 5)}) {
		t.Errorf("canary: padding behind the content passed for a file with nothing but its end behind it")
	}
	if !onlyTheEndBehind([]format.Region{region(format.RolePadding, 0, 5), region(format.RoleContent, 5, 10), region(format.RoleEnd, 15, 5)}) {
		t.Errorf("canary: a file with all of its padding in front was refused for padding behind")
	}
	ends := []format.Region{region(format.RoleContent, 0, 10), region(format.RoleEnd, 10, 0), region(format.RoleEnd, 10, 5)}
	if cutFallsInTheEnd(ends, 5) {
		t.Errorf("canary: a cut in the content, reported as one in the end, passed for a cut in the end")
	}
	if !cutFallsInTheEnd(ends, 12) {
		t.Errorf("canary: a cut in the end was refused for one outside it")
	}
	if cutFallsInTheEnd(ends[:2], 10) {
		t.Errorf("canary: a cut at an end of no bytes passed for a cut in the end")
	}
	listed := maps.Clone(notYetCutAtTheShare)
	listed["txt"] = notYet{paddingBehind, 64 << 10, 50}
	if !slices.ContainsFunc(shareVerdicts(problems, listed), func(s string) bool { return strings.HasPrefix(s, "txt:") }) {
		t.Errorf("canary: txt listed as not cutting at the share, which it does, was not reported")
	}
}

// absentFromTheManifest is every file planned for the sweep that the manifest
// has no entry for. shareVerdicts asks only the cases a file was written for,
// so without this a format that loses one of its files to the run still
// passes on the others.
func absentFromTheManifest(missingOf map[string]int64, inManifest map[string]bool) []string {
	var out []string
	for id := range missingOf {
		if !inManifest[id] {
			out = append(out, id+": planned for the sweep and not in the manifest, so its cut was never asked")
		}
	}
	slices.Sort(out)
	return out
}

// cutInTheEnd is what shareProblem says of a cut that fell in the end a file
// closes with: a central directory, a trailer.
const cutInTheEnd = "the cut fell in the end"

// endClosesTheFile is every format whose specification puts its end after all
// of its content, so a cut shorter than the end can only fall in it
// (onlyTheEndBehind). A format whose end may stand anywhere - a TIFF's image
// directory, which the specification lets stand before the pictures - is not
// here: it could put its end in front of the content instead.
var endClosesTheFile = map[string]string{
	"zip":   "APPNOTE 4.3.6 lays a ZIP out with the central directory and its end record after every entry",
	"docx":  "a ZIP package, laid out as APPNOTE 4.3.6 lays out a ZIP: the central directory after every part",
	"xlsx":  "a ZIP package, laid out as APPNOTE 4.3.6 lays out a ZIP: the central directory after every part",
	"pptx":  "a ZIP package, laid out as APPNOTE 4.3.6 lays out a ZIP: the central directory after every part",
	"targz": "POSIX tar puts the end of archive marker after every entry, and RFC 1952 closes a gzip member with its trailer",
	"pdf":   "ISO 32000-1 section 7.5 puts the cross reference table and the trailer after the body, and the end of file marker last",
}

// onlyTheEndBehind is whether nothing but the end stands behind the content of
// a file's whole map - no padding there that the format could have put in
// front of the content instead.
//
// A cut that falls in the end of such a file falls there because the end is
// longer than the part the cut takes: a ZIP's central directory with a record
// for every part, cut by 1% of a few kilobytes (docs/TRUNCATE-2026-11-11.md
// section 10). No layout reaches the content then, so the share cannot be
// asked of that file. With padding behind the content the format could have
// moved it, and the cut is a miss like any other.
func onlyTheEndBehind(regions []format.Region) bool {
	last := -1
	for i, r := range regions {
		if r.Role == format.RoleContent {
			last = i
		}
	}
	if last < 0 {
		return false
	}
	for _, r := range regions[last+1:] {
		if r.Role != format.RoleEnd && r.Length != 0 {
			return false
		}
	}
	return true
}

// cutFallsInTheEnd is whether the byte at, the first a cut drops, lies in an
// end of a file's whole map - asked of the map, because the manifest says
// where the cut fell from the map the cut file wrote, which is what the
// exception for a cut in the end must not take on trust.
func cutFallsInTheEnd(regions []format.Region, at int64) bool {
	return slices.ContainsFunc(regions, func(r format.Region) bool {
		return r.Role == format.RoleEnd && r.At <= at && at < r.At+r.Length
	})
}

// wholeMap is the map of the whole file f is cut from: its generator written
// out through its own plan to nowhere, the damage left to the engine's
// streams, which are not here.
func wholeMap(t *testing.T, f engine.PlannedFile) []format.Region {
	t.Helper()
	c := &countingSink{}
	rec := &format.MapRecorder{W: c}
	if err := f.Desc.Generator.Write(t.Context(), rec, f.Plan); err != nil {
		t.Errorf("%s: writing the whole file again: %v", f.Target.ID, err)
		return nil
	}
	return rec.Map(c.n, true)
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
		if cut.In == string(format.RoleEnd) {
			return cutInTheEnd
		}
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
