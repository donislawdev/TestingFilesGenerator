package engine

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
	"github.com/donislawdev/TestingFilesGenerator/internal/version"
)

// Target is one group of files to produce.
type Target struct {
	ID     string
	Format string
	// Sizes is the exact size of every file in the group, one entry per file,
	// so the number of files is the length of this list.
	//
	// A single size repeated is the common case and Uniform builds it. The
	// list is the general form because files in one group do not have to be
	// the same size - a boundary set is three consecutive sizes under one id,
	// and a range will be a different size per file. Carrying a count and one
	// size would have needed a second way of saying it the moment the first of
	// those arrived.
	Sizes []int64
	// Contains is what a container holds. Only a format that declares itself a
	// container may have it, and asking it of any other is refused before
	// planning starts rather than ignored.
	Contains []format.Content
	// SizeFromContents says no size was named and the container works it out
	// from Contains. The entries in Sizes then only carry the count, and their
	// value is not read.
	SizeFromContents bool
	// SizeIsRange says the sizes are drawn from SizeMin to SizeMax rather than
	// stated, and Sizes arrives carrying only the count.
	//
	// The draw happens here rather than in the recipe package because this is
	// the first place that knows the seed the run will actually use - the
	// --seed flag overrides the recipe, and a size drawn before that would
	// belong to a different run than the manifest describes. It still happens
	// during planning, so AR10 holds and a dry run reports exact numbers.
	SizeIsRange bool
	SizeMin     int64
	SizeMax     int64
	// BoundaryLimit is the limit a boundary set was built around, zero when
	// this target is not one. The three files name themselves from it.
	//
	// Reported by hand: three files called files_0001, files_0002 and
	// files_0003 do not say which is the limit and which is a byte either
	// side, and somebody dropping one into an upload form has no way to tell.
	BoundaryLimit int64
	NameTmpl      string
	Label         bool
	// Expected is what the system under test should do with these files, and
	// ExpectedReason is why, from the closed list in docs/MANIFEST.md.
	Expected       string
	ExpectedReason string
	// Group names the class of case these files belong to and reaches the
	// manifest, so a test can assert about a whole class at once.
	Group      string
	Properties map[string]string
}

// drawSizes settles the size of every file of a range target.
//
// Judged at the low end before a single size is drawn, and that order is the
// point rather than an optimisation. A range whose low end the format cannot
// deliver - below the minimum of the format, or too small to hold what the
// container was told to hold - would otherwise fail on some runs and not
// others, depending on what came out of the seed. A tool whose whole promise
// is that the same seed gives the same run cannot have an error that appears
// and disappears. So the low end is planned first and the range either works
// for every file or for none.
//
// The judge is the generator itself rather than a second copy of its rules
// here. A copy would be a place for the two to disagree, and the disagreement
// would surface as a file that planning accepted and writing refused.
func drawSizes(t *Target, desc format.Descriptor, targetSeed uint64) error {
	if _, err := planWithoutCrashing(desc, format.Request{
		Bytes:      t.SizeMin,
		Contains:   t.Contains,
		Seed:       core.FileSeed(targetSeed, 0),
		Label:      t.Label,
		Properties: t.Properties,
	}); err != nil {
		return err
	}

	span := uint64(t.SizeMax - t.SizeMin)
	for i := range t.Sizes {
		if span == 0 {
			// Both ends the same is legal and means identical files. Drawing
			// from a range of one is not wrong, it just reads worse.
			t.Sizes[i] = t.SizeMin
			continue
		}
		// Per index, never from a running stream. Raising a count then leaves
		// the sizes of the earlier files alone, which is rule 2 and the reason
		// core.SizeSeed takes an index at all.
		r := core.NewRand(core.SizeSeed(targetSeed, i))
		t.Sizes[i] = t.SizeMin + int64(r.Uint64N(span+1))
	}
	return nil
}

// Uniform is n files of the same size, which is what most targets ask for.
func Uniform(n int, bytes int64) []int64 {
	if n <= 0 {
		return nil
	}
	out := make([]int64, n)
	for i := range out {
		out[i] = bytes
	}
	return out
}

// Options are the settings of one run.
type Options struct {
	OutDir  string
	Seed    int64
	DryRun  bool
	Command string

	// RecipeHash and Overrides describe where the settings of this run came
	// from. Both are empty for a run driven by flags alone.
	RecipeHash string
	Overrides  map[string]manifest.Override
	// Preset says which named question this run came from, when it came from
	// one. Nil for a recipe file and for flags on their own.
	Preset *manifest.Preset
	// ManifestName is the file the manifest lands in, relative to OutDir.
	ManifestName string

	// AvailableBytes reports the free space at a path. It is injected so a
	// test can describe a small disk without owning one - and so that a test
	// of the guard writes kilobytes rather than trying for a petabyte when
	// the guard is broken. Nil means ask the operating system.
	AvailableBytes func(path string) (int64, error)

	// MaxPlanBytes is the ceiling on what the plan may cost in memory. Zero
	// means core.MaxPlanBytes, which is what every real caller passes.
	//
	// Injected for the same reason AvailableBytes above it is: a test of this
	// guard would otherwise have to allocate two gigabytes to reach it, which
	// is slower than the whole suite and would sit inside the mutation
	// runner's own memory cap. With a small ceiling the mechanism is exercised
	// on a handful of files, and the real number is asked about separately.
	MaxPlanBytes int64

	// OnProgress is called as the run advances. Nil means silence, which is
	// what every caller that has nobody to show it to should pass.
	//
	// Called from the same goroutine doing the work, so there is no
	// concurrency here to get wrong. Called often - once per write inside a
	// file, not only once per finished file - so rate limiting what actually
	// reaches a screen belongs to the caller. Without the writes inside a
	// file, one 5 GB file would report once, at the end.
	OnProgress func(Progress)
}

func (o Options) availableBytes(path string) (int64, error) {
	if o.AvailableBytes != nil {
		return o.AvailableBytes(path)
	}
	return core.AvailableBytes(path)
}

// Result is what a run produced.
type Result struct {
	Manifest *manifest.Manifest
	// Failures counts files that could not be produced. A run with failures
	// keeps its good files and reports the partial outcome.
	Failures int

	// Started says whether the run got past its preflight checks.
	//
	// A manifest is written even when a run is cut short, because otherwise
	// cleanup has nothing to work from. That rule was applied one step too
	// wide: a run refused before it wrote anything also produced a manifest,
	// which replaced the record of an earlier run and left that run's files
	// with nothing able to remove them. A refused run has nothing to record.
	Started bool
}

// PlannedFile is one file worked out before anything is written.
//
// Properties inside Plan is the map the target was given, not a copy of it, and
// it reaches the manifest by the same reference. Nothing copies it because no
// generator keeps that map or touches it while writing - they read it in Plan
// and answer with sizes. A copy would defend against a generator that does not
// exist, and this tree has removed several defences of that shape. A generator
// that ever does keep it has to copy at that end, where the guard for it can
// be written.
type PlannedFile struct {
	ID     string
	Target *Target
	Index  int
	Name   string
	Seed   uint64
	Desc   format.Descriptor
	Plan   format.Plan
}

// settleTarget checks everything that has to be true about one target before
// any of its files are planned, and settles the sizes of a range.
//
// Split out of Plan when that function crossed the shape ceiling. The line is
// what a part does rather than how long it is: this answers "is this target
// askable at all", and the loop it was taken out of answers "what files does it
// come to". seen carries across targets because a duplicate id is a fact about
// the run rather than about one entry.
func settleTarget(t *Target, opt Options, seen map[string]bool) (format.Descriptor, error) {
	if t.ID == "" {
		return format.Descriptor{}, &RecipeError{Setting: SettingID,
			Detail:  "a target has no id",
			Because: "every target needs a stable id, it anchors the seed and links to the manifest"}
	}
	if seen[t.ID] {
		return format.Descriptor{}, &RecipeError{Setting: SettingID,
			Detail:  fmt.Sprintf("target id %q is used twice", t.ID),
			Because: "ids identify targets, so a duplicate is an error rather than a silent overwrite"}
	}
	seen[t.ID] = true

	if len(t.Sizes) == 0 {
		return format.Descriptor{}, &RecipeError{Setting: SettingCount,
			Detail: fmt.Sprintf("target %q asks for 0 files", t.ID),
			Remedy: "Ask for at least one"}
	}

	desc, err := format.Get(t.Format)
	if err != nil {
		return format.Descriptor{}, err
	}

	if err := desc.CheckProperties(t.Properties); err != nil {
		return format.Descriptor{}, err
	}

	// A format that holds nothing cannot be asked what it holds. Silently
	// ignoring contains would give an archive with none of the files somebody
	// listed, reported as a success - the file looks right and the test suite
	// believes it.
	if len(t.Contains) > 0 && !desc.Container {
		return format.Descriptor{}, &format.NotAContainerError{Format: t.Format, Containers: format.Containers()}
	}

	// A size below the minimum is refused by the generator itself, on the first
	// size that cannot be delivered, and planning writes nothing - so every size
	// of a boundary set is judged before any file exists.
	//
	// There used to be a loop here checking the same thing against the declared
	// minimum first. Mutation showed no test could tell whether it ran, because
	// every generator already refuses and a guard walks sizes below the minimum
	// for every registered format. It was removed rather than kept as defence
	// nobody can verify.
	if t.SizeIsRange {
		if err := drawSizes(t, desc, core.TargetSeed(opt.Seed, t.ID)); err != nil {
			return format.Descriptor{}, err
		}
	}
	return desc, nil
}

// Plan works out every file of every target without touching the disk.
//
// Everything is planned before anything is written. A size a format cannot
// deliver is refused here, which is what makes the promise of "zero files on
// disk" true rather than nearly true.
//
// It changes the targets it was given, which the signature does not show. A
// range target arrives carrying only a count and leaves carrying a size per
// file, because the draw needs the seed the run will actually use and this is
// the first place that knows it. Every planned file also keeps a pointer into
// the caller's slice rather than a copy of the target.
//
// Both of those are depended on rather than merely tolerated, so a later
// tidying that copies the slice has to change the caller in the same breath:
// echoBoundaries finds the files of a boundary set by comparing that pointer
// against the address of its own target, and a copy would leave it matching
// nothing and printing a heading with no files under it. The guard on that
// output asks only about the heading, deliberately, so the suite would stay
// green while the three lines that name the limit disappeared.
// Plan works out every file of a run without writing anything.
//
// The version without a context, for callers that have none. It is the one the
// guards use and the one a caller reaches for first, and it cannot disagree
// with PlanContext because it is PlanContext.
//
// Two names for one thing rather than one name with a context argument,
// because the argument would have to be threaded through thirty four call
// sites to say "no context" thirty of those times. The standard library takes
// the same shape wherever it grew a context late - exec.Command beside
// exec.CommandContext, sql.Query beside QueryContext.
func Plan(targets []Target, opt Options) ([]PlannedFile, error) {
	return PlanContext(context.Background(), targets, opt)
}

// PlanContext is Plan, stoppable.
//
// Planning was assumed to be fast, and the comment in the window that said so
// carried a measurement: "15.7 ms for ten thousand files". That number was
// taken on txt. Measured on 2026-08-26 across formats, two thousand files by
// --dry-run: txt 380-416 ms, pdf 474-581 ms, docx 835-1052 ms, gif about 5.3 s,
// jpg 9.9-14.6 s and png 16.5-22.8 s - because png, jpg and gif ENCODE the
// picture while planning, and walk a ladder of sizes doing it when no size is
// given. Ten thousand PNGs is a minute and a half before a byte is written.
//
// So planning is work somebody may want to stop, which is the whole reason
// this exists.
func PlanContext(ctx context.Context, targets []Target, opt Options) ([]PlannedFile, error) {
	seen := map[string]bool{}
	pl := &planning{names: map[string]nameOwner{}}

	// The running total of the whole run, kept here rather than worked out
	// afterwards. Both of these used to be settled too late to help: the file
	// count was only bounded by whatever a caller had already allocated, and
	// the byte total was summed after planning by a function that could not
	// report a wrap - so a total that had left the range was handed to the free
	// space check as a negative requirement and satisfied it.
	var totalFiles int

	// Watches what the plan costs while it is being built. Constructed here
	// rather than at the first file because constructing it is what takes the
	// reference point, and that has to happen before any of the plan exists.
	pl.budget = newPlanMemory(opt.MaxPlanBytes)

	// The manifest lands beside the files, so its name is a name too. A path
	// here would leave a manifest outside the directory the run was pointed
	// at, describing files that are not next to it.
	if opt.ManifestName != "" {
		if err := checkFileName(SettingOutputManifest, "the manifest", opt.ManifestName); err != nil {
			return nil, err
		}
	}
	// The manifest holds its name before any target can take it, because a file
	// landing on that name is the worst ending this tool has. Measured on
	// 2026-08-25: the run wrote every file, renamed the last one over the
	// manifest's claim, and then refused to save the manifest because the name
	// was no longer empty - exit 5, files on the disk, and nothing that could
	// remove them. validate and --dry-run both called it fine, which is the
	// same shape preflight was written to end.
	//
	// Seeded from manifestNameOf rather than opt.ManifestName, because a run
	// that never names one still writes manifest.json and a target can be
	// pointed straight at it.
	pl.names[collisionKey(manifestNameOf(opt))] = nameOwner{
		name: manifestNameOf(opt), manifest: true}

	if opt.OutDir == "" {
		return nil, &RecipeError{Setting: SettingOutDir,
			Detail: "the output directory is empty",
			// "or leave it out to use the current one" used to close this
			// sentence and it was removed on 2026-08-26. It is true of a
			// recipe file and of the command line, and it is a thing a person
			// CANNOT do in the window: there, leaving it out is exactly what
			// they just did, and all three screens refuse it. One refusal
			// cannot carry two surfaces' advice, and the half that survives is
			// the half that is true on both. O125.
			Remedy: "Name a directory, for example ./fixtures"}
	}

	for i := range targets {
		t := &targets[i]

		// Everything refused from here down is refused about one target, so it
		// carries which one. See atTarget for what that is worth and what it
		// deliberately leaves alone.
		desc, err := settleTarget(t, opt, seen)
		if err != nil {
			return nil, atTarget(i+1, err)
		}
		targetSeed := core.TargetSeed(opt.Seed, t.ID)

		// Counted across every target, not per target. A ceiling on one target
		// alone is one somebody reaches by writing the number out in pieces.
		totalFiles += len(t.Sizes)
		if totalFiles > core.MaxFilesPerRun {
			// Addressed to the target that took the total past the ceiling,
			// which is the one somebody can shrink. The ceiling is a fact
			// about the run rather than about one entry, so the sentence says
			// so - but a refusal a window cannot place is a refusal at the
			// foot of a form with twenty batches above it and nothing marked.
			//
			// Reachable only from here, and measured on 2026-08-25: the recipe
			// reader bounds each target on its own, so three batches of four
			// hundred thousand pass it and the total is refused here. Before
			// this line "validate --json" carried no "at" for it.
			return nil, &RecipeError{
				Setting: core.TargetAddress(i+1, SettingCount),
				Detail: fmt.Sprintf("this run asks for %s across %s",
					core.Count(totalFiles, "file", "files"),
					core.Count(len(targets), "target", "targets")),
				Because: core.TooManyFilesWhy,
				Remedy:  core.TooManyFilesFix}
		}

		if err := pl.files(ctx, t, desc, targetSeed, i+1); err != nil {
			return nil, err
		}
	}
	return pl.out, nil
}

// TotalBytes is what a plan will occupy on disk. Known before the first byte
// is written, which is what the free space guard and --dry-run stand on.
func TotalBytes(files []PlannedFile) int64 {
	var n int64
	for _, f := range files {
		n += f.Plan.Bytes
	}
	return n
}

// Progress is how far a run has got. Both counts are known from the plan, so
// the fractions are exact rather than estimated.
type Progress struct {
	FilesDone  int
	FilesTotal int
	BytesDone  int64
	BytesTotal int64
}

// DefaultManifestName is where the manifest lands when nothing says otherwise.
//
// It lives here rather than in the caller because the engine has to know the
// name to protect it from being written over, and two copies of a file name
// are two things to keep in step.
const DefaultManifestName = "manifest.json"

// preflight answers whether this run may start, without writing anything.
//
// Both checks used to sit after the dry run had already returned, so
// --dry-run reported success for runs that would refuse to start.
func preflight(ctx context.Context, files []PlannedFile, opt Options) error {
	// Free space first. Finding out at file five thousand of ten thousand
	// leaves a half written set and a full disk on a machine somebody works on.
	needed := TotalBytes(files)
	if available, err := opt.availableBytes(opt.OutDir); err == nil {
		if available < needed {
			return &SpaceError{Needed: needed, Available: available, Path: opt.OutDir}
		}
	}
	// A failure to read the free space is not a reason to refuse. A disk we
	// cannot measure is not the same as a disk that is full.

	// Pointing --out at a file rather than a directory is a mistake somebody
	// makes, and it used to arrive as two messages about one fault, the first
	// of them saying "there is nothing at that path" about a path that has
	// something at it. The system reports ENOTDIR and our mapping only knew
	// "missing", "no permission" and "already there".
	if info, err := os.Stat(opt.OutDir); err == nil && !info.IsDir() {
		return &RecipeError{Setting: SettingOutDir,
			Detail: fmt.Sprintf("the output directory %s is a file, not a directory", opt.OutDir),
			Remedy: "Point the output directory at a directory, or at one that does not exist yet and it will be created"}
	}

	// The manifest is checked with the files it would describe, and leaving it
	// out cost exactly what it protects. A second run into the same directory
	// wrote a fresh manifest over the old one, so every file the old one listed
	// stopped being anybody's - cleanup reported nothing to remove and those
	// files could never be cleaned up by this tool again. It happened on a
	// successful run as readily as on a refused one, and on the successful one
	// it happened in silence.
	if path := ManifestPath(opt); exists(path) {
		return &CollisionError{Path: path, Manifest: true}
	}

	// Nothing else is written over either. This tool runs in directories that
	// belong to the user, so destroying their work is the one failure that
	// cannot be undone by running again.
	//
	// The temporary name is checked as well as the final one. It used to be
	// created with os.Create, which truncates, so a file already sitting under
	// that name lost its contents without a word while the collision check
	// looked only at the name the file ends up with. Asking the filesystem
	// instead, with O_EXCL at the write, was tried on 2026-08-25 and taken
	// back out - see writeOne for what it costs on Windows. So this check is
	// the whole of the protection rather than the friendlier half of it.
	//
	// Which run leaves such a file behind is worth being accurate about,
	// because the comment here used to get it wrong. The name carries the
	// process id, so a run killed outright leaves one that no later run can
	// meet: the next run builds a different name and walks past it. Nor is it
	// lost - verify names it as one of ours rather than as something nobody
	// asked for, and cleanup leaves it alone because untouchable rule 7 makes
	// the manifest the whole authority over what may be deleted, and a file
	// that never finished never reached one.
	for _, f := range files {
		// Two questions of the filesystem per planned file, so a large run on a
		// slow share spends real time in here. Until 2026-08-26 that time could
		// not be interrupted: preflight took no context, so a preview had
		// nothing to cancel, the window offered no button for it, and closing
		// the window waited for the whole loop on the interface thread. The
		// same reasoning that put a context into the directory walk in
		// internal/audit on 2026-08-25.
		if err := ctx.Err(); err != nil {
			return err
		}
		if path := filepath.Join(opt.OutDir, f.Name); exists(path) {
			return &CollisionError{Path: path}
		}
		if path := tempPathFor(opt.OutDir, f.Name); exists(path) {
			return &CollisionError{Path: path}
		}
	}
	return nil
}

// tempPathFor is the name a file is written under before it is renamed into
// place. One definition, used by the check above and by the write below, so
// the two cannot disagree about what is being protected.
func tempPathFor(outDir, name string) string {
	return fmt.Sprintf("%s%s%d", filepath.Join(outDir, name), core.PartialMarker, os.Getpid())
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// manifestNameOf is where this run's record lands. One definition, because the
// preflight check, the claim and the save all have to mean the same file.
func manifestNameOf(opt Options) string {
	if opt.ManifestName == "" {
		return DefaultManifestName
	}
	return opt.ManifestName
}

// ManifestPath is the file this run's record lands in, for the callers that
// save it. The engine claims that name before the first file and refuses the
// run if it is taken, so a saver joining the path its own way is answering a
// question this package has already answered.
//
// Exported on 2026-08-27 because both savers did join it their own way. The
// window's did not handle an empty name at all, so a caller that left it blank
// would have asked the manifest to be written to the output directory itself -
// a rename of a file onto a directory. Nothing reached that, because all three
// screens fill the field in, and "nothing reaches it today" is the description
// of a fault waiting for a fourth screen rather than of a safe piece of code.
func ManifestPath(opt Options) string {
	return filepath.Join(opt.OutDir, manifestNameOf(opt))
}

// Run writes a planned set of files.
//
// Each file is written under a temporary name and only then renamed, so the
// output directory never holds an incomplete file. That invariant covers the
// process ending - Ctrl+C, kill, a CI timeout. It does not cover power loss,
// because that would need a flush per file and ten thousand of those is a
// real cost.
//
// A manifest is returned even when the run is cut short, otherwise cleanup
// has nothing to work with.
func Run(ctx context.Context, files []PlannedFile, opt Options) (*Result, error) {
	m := manifest.New(
		"testing-files-generator", version.Version,
		runID(opt.Seed), opt.Command, opt.Seed,
		runtime.GOOS, runtime.GOARCH,
	)
	m.Run.RecipeHash = opt.RecipeHash
	m.Run.Overrides = opt.Overrides
	m.Run.Preset = opt.Preset
	res := &Result{Manifest: m}

	// Whether this run may start at all is settled before anything is written
	// and before a dry run returns. A dry run exists to count and show before
	// the disk is touched, so reporting success for a run that refuses to
	// start on the very next line answers the wrong question.
	if err := preflight(ctx, files, opt); err != nil {
		return res, err
	}

	if opt.DryRun {
		// Nothing is written. Every entry is what would have been produced,
		// which is the same planning path the real run uses rather than a
		// separate one that can drift away from it.
		res.Started = true
		for _, f := range files {
			m.Add(entryFor(f, "", false, nil))
		}
		m.Run.Complete = true
		return res, nil
	}

	if err := os.MkdirAll(opt.OutDir, 0o755); err != nil {
		return res, fmt.Errorf("cannot create the output directory %s: %w", opt.OutDir, err)
	}

	// The manifest name is taken before the first file, not after the last one.
	//
	// Claiming it at save time already stopped two runs from both writing a
	// manifest, but it happened at the end - so a second run wrote its whole set
	// of files and only then found out it had nowhere to record them. Measured
	// on 2026-08-03: two runs started together under different ids ended 0 and
	// 5, with sixteen files on the disk and eight of them in nobody's manifest.
	// Taking the name here turns that into a refusal before anything is written.
	manifestPath := ManifestPath(opt)
	if err := manifest.Claim(manifestPath); err != nil {
		// Only a name that is genuinely taken is a collision. Reporting every
		// failure that way said "manifest.json already exists ... it is the
		// only record of what an earlier run wrote" about an empty directory
		// the user simply had no permission to write in - a sentence that is
		// untrue and sends somebody looking for a run that never happened.
		// Measured on 2026-08-04 with write denied on the output directory.
		if errors.Is(err, fs.ErrExist) {
			return res, &CollisionError{Path: manifestPath, Manifest: true}
		}
		return res, fmt.Errorf("cannot start a run in %s: %w", opt.OutDir, err)
	}

	// Past this point the run owns the name and may write. Started says so, and
	// it is what tells the caller a manifest is worth saving - set here rather
	// than before the claim, because a run that could not take the name has
	// written nothing and has nothing to record. It used to be set earlier, so
	// a refused claim was followed by a second message about failing to write
	// the manifest it had just been told it could not have.
	res.Started = true

	// Given back if this run ends without writing one, or the name would stay
	// taken by a run that never happened.
	defer func() {
		if !res.Manifest.Run.Complete && res.Failures == 0 && len(res.Manifest.Files) == 0 {
			_ = manifest.Release(manifestPath)
		}
	}()

	totalBytes := TotalBytes(files)
	var bytesDone int64

	for i, f := range files {
		select {
		case <-ctx.Done():
			// Stop starting new files. What is already finished stays, and
			// the manifest describes exactly that.
			m.Run.Complete = false
			return res, ctx.Err()
		default:
		}

		// Built per file rather than once, because it closes over how far the
		// run had got before this file started. Left nil when nobody is
		// listening, so a run without progress allocates nothing for it.
		var report func(int64)
		if opt.OnProgress != nil {
			report = func(inFile int64) {
				opt.OnProgress(Progress{
					FilesDone: i, FilesTotal: len(files),
					BytesDone: bytesDone + inFile, BytesTotal: totalBytes,
				})
			}
		}

		sum, err := writeOne(ctx, f, opt.OutDir, report)
		if err == nil {
			// Only what reached the disk. Counting a file that failed would
			// have the bar claim bytes nobody can find, and on a run where
			// several fail the total would arrive before the files do.
			bytesDone += f.Plan.Bytes
		}
		if opt.OnProgress != nil {
			opt.OnProgress(Progress{
				FilesDone: i + 1, FilesTotal: len(files),
				BytesDone: bytesDone, BytesTotal: totalBytes,
			})
		}
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				m.Run.Complete = false
				return res, err
			}
			// One file failing does not end the run. Nine thousand good
			// files are worth keeping, and the entry says what went wrong.
			res.Failures++
			m.Add(entryFor(f, "", false, err))
			continue
		}
		m.Add(entryFor(f, sum, true, nil))
	}

	m.Run.Complete = true
	return res, nil
}

func writeOne(ctx context.Context, f PlannedFile, outDir string, report func(int64)) (string, error) {
	final := filepath.Join(outDir, f.Name)
	// The process id is in the name because two runs writing into one directory
	// used to meet on it. Measured on 2026-08-03: two runs of the same target
	// collided on the temporary file, one of them reported two files it could
	// not produce, and the bytes of the other had already gone through the same
	// handle. The name never survives the run, so nothing about it has to be
	// repeatable - and the file it becomes is settled by the plan, not by this.
	tmp := tempPathFor(outDir, f.Name)

	// os.Create, and O_EXCL was tried here and taken back out on 2026-08-25.
	//
	// The idea was sound: the check in preflight answers "this name is free"
	// a few hundred lines before the write, and O_EXCL would have the
	// filesystem answer it at the moment of writing instead. What it costs on
	// Windows is not sound. Measured with a probe, a file created in a
	// directory reached through a symbolic link:
	//
	//   os.Create                 works
	//   O_CREATE|O_EXCL|O_WRONLY  fails with "The file exists"
	//
	// about a file that does not exist. Go asks for the reparse point rather
	// than what it points at when O_EXCL is set, so every file of a run whose
	// output directory is a link fails - and this tool supports exactly that
	// on purpose, because people keep fixtures on a mounted workspace or a
	// scratch disk. Two guards said so within a minute of the change.
	//
	// The window O_EXCL would have closed is a real one and it is small:
	// preflight refuses every name that is taken before the run starts, so
	// what is left is somebody else creating our temporary name, with our
	// process id in it, during the run. Trading a supported way of pointing
	// the tool at a directory for that is the wrong way round.
	fh, err := os.Create(tmp)
	if err != nil {
		return "", err
	}

	h := sha256.New()
	buffered := bufio.NewWriterSize(fh, 64<<10)
	counter := &countingWriter{w: io.MultiWriter(buffered, h), report: report}

	writeErr := writeWithoutCrashing(ctx, f, counter)
	if writeErr == nil {
		writeErr = buffered.Flush()
	}
	closeErr := fh.Close()

	if writeErr != nil {
		_ = os.Remove(tmp)
		return "", writeErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return "", closeErr
	}

	// The size is the promise. A generator that missed it by a byte is a bug
	// worth catching here rather than in someone's test suite, so the file
	// never reaches its final name.
	if counter.n != f.Plan.Bytes {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("generator for %s produced %d B where the plan said %d B",
			f.Desc.ID, counter.n, f.Plan.Bytes)
	}

	if err := os.Rename(tmp, final); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func entryFor(f PlannedFile, sha string, materialized bool, failure error) manifest.File {
	var notes []manifest.Note
	for _, n := range f.Plan.Notes {
		notes = append(notes, manifest.Note{Code: n.Code, Detail: n.Detail})
	}

	label, _ := f.Plan.Properties[format.PropertyLabelEmbedded].(bool)

	e := manifest.File{
		ID:            f.ID,
		Path:          filepath.ToSlash(f.Name),
		Name:          f.Name,
		Materialized:  materialized,
		Bytes:         f.Plan.Bytes,
		Format:        f.Desc.ID,
		Fidelity:      string(f.Desc.Fidelity),
		Hashes:        manifest.Hashes{SHA256: sha},
		Seed:          core.SeedLabel(f.Seed),
		Generator:     manifest.GeneratorRef{Name: f.Desc.ID, Version: f.Desc.GeneratorVersion},
		Determinism:   string(f.Plan.Determinism),
		Properties:    f.Plan.Properties,
		LabelEmbedded: label,
		Notes:         notes,
		Expected:      expectationFor(f),
		Group:         f.Target.Group,
		TargetID:      f.Target.ID,
	}

	if failure != nil {
		e.Failed = true
		e.Error = failure.Error()
		e.Materialized = false
		e.Notes = append(e.Notes, manifest.Note{
			Code:   "generation_failed",
			Detail: "This file was not produced. The run carried on and ended with the partial exit code.",
		})
	}
	return e
}

func expectationFor(f PlannedFile) manifest.Expected {
	switch f.Target.Expected {
	case "":
		return manifest.Expected{
			Outcome:    manifest.OutcomeUnspecified,
			Detail:     "No expectation was declared for this file.",
			Confidence: "policy_dependent",
		}
	case manifest.OutcomeAccept, manifest.OutcomeReject,
		manifest.OutcomeSanitize, manifest.OutcomeUnspecified:
		return manifest.Expected{
			Outcome:    f.Target.Expected,
			Reason:     f.Target.ExpectedReason,
			Confidence: "certain",
		}
	default:
		return manifest.Expected{
			Outcome:    manifest.OutcomeUnspecified,
			Detail:     fmt.Sprintf("Unrecognised expectation %q was declared.", f.Target.Expected),
			Confidence: "policy_dependent",
		}
	}
}

// indexToken is the one placeholder a name template understands.
const indexToken = "{index:04}"

// boundaryRoles names the three files of a boundary set in the order the sizes
// are built, which is one byte under the limit, the limit, one byte over.
//
// The words rather than the arithmetic, because the arithmetic form has a plus
// sign in it and these files exist to be dropped into upload forms, where a
// plus sign in a name is a well known way to lose an afternoon.
// Changed on 2026-08-18, decision of the owner. They used to be "under_limit"
// and "over_limit", which left out the number that matters: the file is one
// byte under, not merely under. The preset that builds a wider set has always
// named the distance - 10mb_under_1kb - so this is also two naming schemes
// becoming one, across a flag and a preset that answer the same question.
var boundaryRoles = [3]string{"under_1b", "at_limit", "over_1b"}

func renderName(t *Target, d format.Descriptor, index int) (string, error) {
	tmpl := t.NameTmpl
	switch {
	case tmpl != "":
		// A template the user wrote wins. They asked for it by name.
	case t.BoundaryLimit > 0 && index < len(boundaryRoles):
		// A boundary set says which file is which. The role goes in as literal
		// text rather than as a token, so it still passes the same name checks
		// below - the id comes from the user and can hold anything.
		tmpl = t.ID + "_" + boundaryRoles[index] + d.Extension
	default:
		tmpl = t.ID + "_" + indexToken + d.Extension
	}
	name := strings.ReplaceAll(tmpl, indexToken, fmt.Sprintf("%04d", index+1))

	// Every token was just replaced, so a brace left over is a placeholder
	// that does not exist. Left alone it becomes part of the file name, and
	// somebody ends up with a file called invoice_{index}.pdf rather than the
	// numbering they asked for.
	if strings.Contains(name, "{") {
		return "", &RecipeError{Setting: SettingName,
			Detail: fmt.Sprintf("target %q has a name template this build does not understand: %q", t.ID, tmpl),
			Remedy: fmt.Sprintf("The only placeholder is %s, so a name looks like invoice_%s.pdf", indexToken, indexToken)}
	}

	if err := checkFileName(SettingName, fmt.Sprintf("target %q", t.ID), name); err != nil {
		return "", err
	}
	return name, nil
}

// checkFileName keeps a name a name.
//
// A name carrying a path escapes the directory the run was pointed at. A
// recipe travels between teams by design, so "../../something" in a file
// somebody sent over would write outside the directory its reader chose - and
// the free space check, the collision check and cleanup all work on the
// directory, so none of them would be looking in the right place.
//
// Both separators are refused on every system, not just the local one. A name
// holding a backslash is legal on Linux and cannot exist on Windows, and a
// recipe that only works on the machine it was written on is not portable.
func checkFileName(setting, where, name string) error {
	switch {
	case name == "":
		return &RecipeError{Setting: setting, Detail: fmt.Sprintf("%s produces a file with no name", where)}

	case strings.ContainsAny(name, `/\`):
		return &RecipeError{Setting: setting,
			Detail:  fmt.Sprintf("%s produces the name %q, which is a path rather than a file name", where, name),
			Because: "names stay inside the output directory, and a separator is refused on every system so that a recipe works everywhere",
			Remedy:  "Choose the directory with the output setting instead"}

	// A colon, on every system, for the same reason as a separator.
	//
	// Windows reads it as the start of an alternate data stream, so
	// "AB:c.txt" names a stream called c.txt inside a file called AB.
	// Measured on 2026-08-04: the run reported the file as not produced and
	// ended with the partial code, and an empty file called AB was left in the
	// directory anyway - not in the manifest, reported by verify as something
	// nobody asked for, and beyond the reach of cleanup for good. A single
	// letter in front of it is read as a drive instead, which is how the same
	// recipe came to be accepted on Linux and refused on Windows.
	//
	// Legal in a name on Linux and macOS, and refused there too. A recipe
	// travels between machines by design, and one that quietly leaves debris on
	// somebody else's is worse than one refused on all of them.
	case strings.Contains(name, ":"):
		return &RecipeError{Setting: setting,
			Detail:  fmt.Sprintf("%s produces the name %q, which holds a colon", where, name),
			Because: "Windows reads that as a drive or as an alternate data stream rather than as part of the name, so the file arrives called something else or not at all. It is refused on every system so that a recipe means one thing everywhere",
			Remedy:  "Take the colon out, or ask for the file inside an archive where the name survives"}

	// Characters Windows will not put in a file name, refused on every system
	// for the same reason as the separator and the colon above.
	//
	// Measured on 2026-08-25, on each of <>"|?* and on a name holding a tab.
	// All seven planned cleanly, --dry-run answered "1 file in 1 target" and
	// exit 0, and the run then failed that one file with the system's own
	// words: "open a<b.txt.tfg-partial-53628: The filename, directory name, or
	// volume label syntax is incorrect". Three things wrong in one line. The
	// dry run answered for a run that could not happen, which is the fault
	// preflight exists to stop. The sentence is not ours and carries none of
	// the four parts a refusal owes a reader, while carrying the temporary
	// name, which is ours and nobody else's business. And the same recipe
	// writes the file on Linux, where all of these are legal - the reason the
	// separator has been refused everywhere since 2026-08-03.
	//
	// Producing such a name on purpose is a real test case and it belongs to
	// the name laboratory, which writes it into an archive rather than onto
	// the host filesystem. See D10.
	case firstForbidden(name) != 0:
		bad := firstForbidden(name)
		return &RecipeError{Setting: setting,
			Detail:  fmt.Sprintf("%s produces the name %q, which holds %s", where, name, describeForbidden(bad)),
			Because: "Windows refuses that character in a file name, so the file is not written there at all. It is refused on every system so that a recipe means one thing everywhere",
			Remedy:  "Take the character out, or ask for the file inside an archive where the name survives"}

	// One reserved device name, and one only.
	//
	// The folklore list has twenty two - CON, PRN, AUX, COM1 to COM9, LPT1 to
	// LPT9 - and every one of them was measured on 2026-08-25 rather than
	// remembered, on two editions: Windows 11 Pro 26200 and Windows Server
	// 2025 build 26100. con, con.txt, prn, aux, com1, com1.bin, lpt1 and
	// conin$ each came back an ordinary file that verify then passed, on both.
	// Refusing that list would refuse con.pdf, a name both systems store
	// perfectly well, which is a rule written from memory doing damage.
	//
	// NUL is the exception on both, and it is the one worth catching, because
	// it does not fail. The write succeeds and the bytes go nowhere, which is
	// the silence rule broken as completely as it can be - a manifest
	// describing a file that was never on the disk. The run is refused a step
	// later today, by the collision check finding something at the path, and
	// that is safe but tells the reader to remove a file nobody can remove.
	//
	// The bare name only. An extension saves it - nul.txt is an ordinary file
	// on both editions - so this is not the "any extension" rule the folklore
	// describes either.
	case strings.EqualFold(name, "nul"):
		return &RecipeError{Setting: setting,
			Detail:  fmt.Sprintf("%s produces the name %q, which names the null device on Windows rather than a file", where, name),
			Because: "writing there succeeds and the bytes go nowhere, so the run would record a file that is not on the disk. It is refused on every system so that a recipe means one thing everywhere",
			Remedy:  "Give it an extension, nul.txt is an ordinary name, or choose another one"}

	case name == "." || name == "..":
		return &RecipeError{Setting: setting, Detail: fmt.Sprintf(
			"%s produces the name %q, which names a directory rather than a file", where, name)}

	// A name Windows stores under a different name than the one it was given.
	// Refused on every system for the same reason a separator is: a recipe that
	// only works on the machine it was written on is not portable.
	//
	// Measured on 2026-08-03, and it is the silence rule broken rather than a
	// portability nicety. "--name trailing." finished with exit code 0, the
	// file landed as "trailing", and the manifest recorded "trailing." - so the
	// run described a file that was not there under that name, and "tfg verify"
	// on the tool's own output failed with exit code 7 a second later.
	//
	// Producing such a name deliberately is a real test case and it belongs to
	// the name laboratory, which writes it into an archive rather than onto the
	// host filesystem for exactly this reason. See D10.
	case strings.HasSuffix(name, ".") || strings.HasSuffix(name, " "):
		return &RecipeError{Setting: setting,
			Detail:  fmt.Sprintf("%s produces the name %q, which ends in a dot or a space", where, name),
			Because: "Windows stores such a name without it, so the file on disk would not be the file the manifest describes and verify would report both",
			Remedy:  "Take the last character off, or ask for the file inside an archive where the name survives"}

	// Judged the same way on every system, like the separator above. Using
	// filepath here asks the machine this build runs on, and the answer
	// differs: measured on 2026-08-04, "a:b.txt" was accepted on Linux and
	// refused on Windows from one recipe, so a fixture set written on one
	// machine failed on the next. That is the failure this rule exists to
	// prevent, arriving through the rule itself.
	case filepath.IsAbs(name) || core.HasVolumeName(name):
		return &RecipeError{Setting: setting,
			Detail:  fmt.Sprintf("%s produces the absolute path %q", where, name),
			Because: "a recipe carries no absolute paths, because then it only works on the machine it was written on",
			Remedy:  "Choose the directory with the output setting instead"}
	}
	return nil
}

// forbiddenChars are the printable characters Windows refuses in a file name.
//
// The separator and the colon are not here. They are refused above with their
// own sentences, because what goes wrong with them is not "the file is not
// written" but something worse and worth its own explanation - a name that
// leaves the output directory, and a name Windows reads as a drive or as a
// stream inside another file.
const forbiddenChars = `<>"|?*`

// firstForbidden is the first character of a name that Windows will not store,
// or zero when there is none.
//
// The first rather than all of them, because a refusal naming one character a
// reader can find beats a list they have to compare against their own name.
func firstForbidden(name string) rune {
	for _, r := range name {
		// Below the space, which is every control character. Windows refuses
		// the whole range, and a name holding one is unreadable on any system
		// - a tab in a file name is a name nobody can type back.
		if r < 0x20 || strings.ContainsRune(forbiddenChars, r) {
			return r
		}
	}
	return 0
}

// describeForbidden names a character in a way somebody can act on. A control
// character has nothing to show, so it is given as its number instead of being
// printed into the middle of a sentence where it would do what it says.
func describeForbidden(r rune) string {
	if r < 0x20 {
		return fmt.Sprintf("a control character, U+%04X", r)
	}
	return fmt.Sprintf("the character %q", string(r))
}

func runID(seed int64) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("run:%d", seed)))
	return "run_" + hex.EncodeToString(h[:5])
}

type countingWriter struct {
	w io.Writer
	n int64
	// report, when set, is called with the running total for this file. It is
	// what gives progress inside a single large file rather than only between
	// files - the case where silence is worst, because one 5 GB file is one
	// callback if you only count finished files.
	report func(int64)
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	if c.report != nil {
		c.report(c.n)
	}
	return n, err
}
