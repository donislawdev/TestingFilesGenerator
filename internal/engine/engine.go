// Package engine plans a run, writes the files and backs verify and cleanup.
//
// It knows nothing about the command line or the window. That rule erodes one
// exception at a time, so a test enforces it instead of good intentions.
package engine

import (
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

	"golang.org/x/text/unicode/norm"

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
	if _, err := desc.Generator.Plan(format.Request{
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
		return format.Descriptor{}, &RecipeError{Setting: SettingID, Detail: "a target has no id: every target needs a stable id, it anchors the seed and links to the manifest"}
	}
	if seen[t.ID] {
		return format.Descriptor{}, &RecipeError{Setting: SettingID, Detail: fmt.Sprintf("target id %q is used twice: ids identify targets, so a duplicate is an error rather than a silent overwrite", t.ID)}
	}
	seen[t.ID] = true

	if len(t.Sizes) == 0 {
		return format.Descriptor{}, &RecipeError{Setting: SettingCount, Detail: fmt.Sprintf("target %q asks for 0 files: ask for at least one", t.ID)}
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
func Plan(targets []Target, opt Options) ([]PlannedFile, error) {
	var out []PlannedFile
	seen := map[string]bool{}
	names := map[string]nameOwner{}

	// The running total of the whole run, kept here rather than worked out
	// afterwards. Both of these used to be settled too late to help: the file
	// count was only bounded by whatever a caller had already allocated, and
	// the byte total was summed after planning by a function that could not
	// report a wrap - so a total that had left the range was handed to the free
	// space check as a negative requirement and satisfied it.
	var totalBytes int64
	var totalFiles int

	// The manifest lands beside the files, so its name is a name too. A path
	// here would leave a manifest outside the directory the run was pointed
	// at, describing files that are not next to it.
	if opt.ManifestName != "" {
		if err := checkFileName("the manifest", opt.ManifestName); err != nil {
			return nil, err
		}
	}
	if opt.OutDir == "" {
		return nil, &RecipeError{Setting: SettingOutDir, Detail: "the output directory is empty: name a directory, for example ./fixtures, or leave it out to use the current one"}
	}

	for i := range targets {
		t := &targets[i]

		desc, err := settleTarget(t, opt, seen)
		if err != nil {
			return nil, err
		}
		targetSeed := core.TargetSeed(opt.Seed, t.ID)

		// Counted across every target, not per target. A ceiling on one target
		// alone is one somebody reaches by writing the number out in pieces.
		totalFiles += len(t.Sizes)
		if totalFiles > core.MaxFilesPerRun {
			return nil, &RecipeError{Detail: fmt.Sprintf(
				"this run asks for %s across %s - %s",
				core.Count(totalFiles, "file", "files"),
				core.Count(len(targets), "target", "targets"), core.ErrTooManyFiles)}
		}

		for idx, size := range t.Sizes {
			fileSeed := core.FileSeed(targetSeed, idx)

			p, err := desc.Generator.Plan(format.Request{
				Bytes:            size,
				SizeFromContents: t.SizeFromContents,
				Contains:         t.Contains,
				Seed:             fileSeed,
				Label:            t.Label,
				Properties:       t.Properties,
			})
			if err != nil {
				return nil, err
			}

			name, err := renderName(t, desc, idx)
			if err != nil {
				return nil, err
			}
			// Two files heading for one name means one of them would be
			// destroyed by the other, and the manifest would still describe
			// both. A manifest that quietly lost a file looks complete and
			// reaches the test suite as a false truth.
			//
			// "One name" is not "the same string", and reading it that way cost
			// exactly what this check exists to prevent. Most filesystems people
			// run this on keep the case somebody typed and match without it -
			// NTFS, APFS and exFAT do, ext4 does not - so "report.txt" and
			// "REPORT.TXT" are two files on one machine and one file on the
			// next. Measured on Windows, 2026-08-03: exit 0, two entries in the
			// manifest, one file on the disk, and "tfg verify" failing on the
			// tool's own output a second later.
			//
			// Case is not the only way to spell one name twice. An accented
			// letter is one code point, U+00E9, and it is also the plain letter
			// followed by a combining accent, U+0301. Both are valid UTF-8 and
			// they print identically. APFS normalises what it is given, so on
			// macOS the two spellings are one file, while NTFS and ext4 keep
			// them apart. Measured there 2026-08-04: exit 0, two entries in the
			// manifest, one file on the disk, and "tfg verify" failing on the
			// tool's own output a second later - the same defect the case rule
			// exists to stop, reached by spelling a letter the other way.
			//
			// So the key folds case and normalises, and neither alone is
			// enough: case folding does not bring the two spellings together
			// and normalising does not bring REPORT.TXT to report.txt.
			//
			// Refused everywhere rather than only where it bites, the same as a
			// path separator and for the same reason: a recipe travels between
			// machines by design, and one that quietly loses a file on somebody
			// else's is worse than one refused on both. Producing such a pair on
			// purpose belongs to the name laboratory and its archive mode, D10.
			key := collisionKey(name)
			if owner, clash := names[key]; clash {
				return nil, &RecipeError{Detail: collisionDetail(owner, t.ID, name) +
					" - give one of them a name template containing " + indexToken}
			}
			names[key] = nameOwner{id: t.ID, name: name}

			if totalBytes, err = core.AddSizes(totalBytes, p.Bytes); err != nil {
				return nil, &RecipeError{Detail: fmt.Sprintf(
					"target %q brings the run to a size that is too large to measure: %s", t.ID, err)}
			}

			out = append(out, PlannedFile{
				ID:     fmt.Sprintf("f_%04d", len(out)+1),
				Target: t,
				Index:  idx,
				Name:   name,
				Seed:   fileSeed,
				Desc:   desc,
				Plan:   p,
			})
		}
	}
	return out, nil
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
func preflight(files []PlannedFile, opt Options) error {
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
		return &RecipeError{Setting: SettingOutDir, Detail: fmt.Sprintf(
			"the output directory %s is a file, not a directory. Point the output directory at a directory, or at one that does not exist yet and it will be created",
			opt.OutDir)}
	}

	// The manifest is checked with the files it would describe, and leaving it
	// out cost exactly what it protects. A second run into the same directory
	// wrote a fresh manifest over the old one, so every file the old one listed
	// stopped being anybody's - cleanup reported nothing to remove and those
	// files could never be cleaned up by this tool again. It happened on a
	// successful run as readily as on a refused one, and on the successful one
	// it happened in silence.
	if path := filepath.Join(opt.OutDir, manifestNameOf(opt)); exists(path) {
		return &CollisionError{Path: path, Manifest: true}
	}

	// Nothing else is written over either. This tool runs in directories that
	// belong to the user, so destroying their work is the one failure that
	// cannot be undone by running again.
	//
	// The temporary name is checked as well as the final one. It is created
	// with os.Create, which truncates, so a file already sitting under that
	// name lost its contents without a word - and the collision check only
	// ever looked at the name the file ends up with. It is an unlikely name to
	// meet by accident and an easy one to leave behind: a run killed outright
	// leaves exactly this, and the next run into the same directory would eat
	// it silently.
	for _, f := range files {
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

// nameOwner remembers who took a name and how it was spelled, so a refusal can
// show both names as they were written rather than as they were compared.
type nameOwner struct {
	id   string
	name string
}

// collisionKey is the spelling two names are compared under. Two names sharing
// a key are one file on some filesystem somebody runs this on.
//
// Folding case covers NTFS, APFS and exFAT, which keep the case that was typed
// and match without it. Normalising covers APFS again, which folds the two
// spellings of an accented letter into one name. Neither step covers the other:
// case folding leaves the two spellings apart, and normalising leaves
// REPORT.TXT apart from report.txt.
func collisionKey(name string) string {
	return strings.ToLower(norm.NFC.String(name))
}

// collisionDetail says what the two names have in common. Two names that
// collide can print identically on screen, so a refusal that only shows them is
// one the reader cannot act on. A refusal has to say what is wrong, what is
// allowed and what to do instead.
func collisionDetail(owner nameOwner, id, name string) string {
	switch {
	case owner.name == name:
		return fmt.Sprintf("targets %q and %q both produce a file named %s", owner.id, id, name)
	case strings.EqualFold(owner.name, name):
		return fmt.Sprintf(
			"targets %q and %q produce the names %s and %s, which differ only in case. Most filesystems treat those as one file, so one would be written over the other and the manifest would describe both",
			owner.id, id, owner.name, name)
	default:
		return fmt.Sprintf(
			"targets %q and %q produce the names %s and %s. Those print the same because they are one name spelled two ways, an accented letter against the plain letter with its accent as a separate character. macOS stores both under one name, so one file would be written over the other and the manifest would describe both",
			owner.id, id, owner.name, name)
	}
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
	if err := preflight(files, opt); err != nil {
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
	manifestPath := filepath.Join(opt.OutDir, manifestNameOf(opt))
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

	fh, err := os.Create(tmp)
	if err != nil {
		return "", err
	}

	h := sha256.New()
	counter := &countingWriter{w: io.MultiWriter(fh, h), report: report}

	writeErr := f.Desc.Generator.Write(ctx, counter, f.Plan)
	closeErr := fh.Close()

	if writeErr != nil {
		os.Remove(tmp)
		return "", writeErr
	}
	if closeErr != nil {
		os.Remove(tmp)
		return "", closeErr
	}

	// The size is the promise. A generator that missed it by a byte is a bug
	// worth catching here rather than in someone's test suite, so the file
	// never reaches its final name.
	if counter.n != f.Plan.Bytes {
		os.Remove(tmp)
		return "", fmt.Errorf("generator for %s produced %d B where the plan said %d B",
			f.Desc.ID, counter.n, f.Plan.Bytes)
	}

	if err := os.Rename(tmp, final); err != nil {
		os.Remove(tmp)
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func entryFor(f PlannedFile, sha string, materialized bool, failure error) manifest.File {
	var notes []manifest.Note
	for _, n := range f.Plan.Notes {
		notes = append(notes, manifest.Note{Code: n.Code, Detail: n.Detail})
	}

	label, _ := f.Plan.Properties["label_embedded"].(bool)

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
		return "", &RecipeError{Detail: fmt.Sprintf(
			"target %q has a name template this build does not understand: %q. The only placeholder is %s, so a name looks like invoice_%s.pdf",
			t.ID, tmpl, indexToken, indexToken)}
	}

	if err := checkFileName(fmt.Sprintf("target %q", t.ID), name); err != nil {
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
func checkFileName(where, name string) error {
	switch {
	case name == "":
		return &RecipeError{Setting: SettingName, Detail: fmt.Sprintf("%s produces a file with no name", where)}

	case strings.ContainsAny(name, `/\`):
		return &RecipeError{Setting: SettingName, Detail: fmt.Sprintf(
			"%s produces the name %q, which is a path rather than a file name. Names stay inside the output directory, and a separator is refused on every system so that a recipe works everywhere. Choose the directory with the output setting instead",
			where, name)}

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
		return &RecipeError{Setting: SettingName, Detail: fmt.Sprintf(
			"%s produces the name %q, which holds a colon. Windows reads that as a drive or as an alternate data stream rather than as part of the name, so the file arrives called something else or not at all. It is refused on every system so that a recipe means one thing everywhere - take the colon out, or ask for the file inside an archive where the name survives",
			where, name)}

	case name == "." || name == "..":
		return &RecipeError{Setting: SettingName, Detail: fmt.Sprintf(
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
		return &RecipeError{Setting: SettingName, Detail: fmt.Sprintf(
			"%s produces the name %q, which ends in a dot or a space. Windows stores such a name without it, so the file on disk would not be the file the manifest describes and verify would report both. Take the last character off, or ask for the file inside an archive where the name survives",
			where, name)}

	// Judged the same way on every system, like the separator above. Using
	// filepath here asks the machine this build runs on, and the answer
	// differs: measured on 2026-08-04, "a:b.txt" was accepted on Linux and
	// refused on Windows from one recipe, so a fixture set written on one
	// machine failed on the next. That is the failure this rule exists to
	// prevent, arriving through the rule itself.
	case filepath.IsAbs(name) || core.HasVolumeName(name):
		return &RecipeError{Setting: SettingName, Detail: fmt.Sprintf(
			"%s produces the absolute path %q. A recipe carries no absolute paths, because then it only works on the machine it was written on. Choose the directory with the output setting instead",
			where, name)}
	}
	return nil
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
