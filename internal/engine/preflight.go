package engine

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// What has to be true before a run may start, and where the things it writes
// land.
//
// Split out of engine.go on 2026-09-05, when reading the output directory once
// instead of asking about every planned file twice took that file past the
// length ceiling. The guard's own message asks for a split by what the parts
// do rather than for a bigger number, and this is that part: no bytes are
// written from here, every function answers a question about a name.

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
	// One listing of the directory rather than two questions of the filesystem
	// for every planned file.
	//
	// The comment that stood here named the cost - "a large run on a slow share
	// spends real time in here" - and until 2026-08-26 that time could not even
	// be interrupted, which is why the context arrived. Measured on 2026-09-05,
	// ten thousand planned names with half of them present: 1.937 s of stat
	// calls against 8.9 ms for one listing, ranges disjoint over eight
	// repetitions with a steady canary. That is a local disk. The share the old
	// comment was written for pays a round trip for each of those twenty
	// thousand questions.
	//
	// Names cannot hold a separator - checkFileName refuses both on every
	// system - so one listing of the output directory covers every planned file
	// and every temporary name beside it.
	taken := namesIn(opt.OutDir)

	for _, f := range files {
		// Asked per file rather than once, so a preview of a hundred thousand
		// files can still be cancelled. That reasoning is unchanged by the
		// listing above, it is only much cheaper to be interrupted now.
		if err := ctx.Err(); err != nil {
			return err
		}
		if path := filepath.Join(opt.OutDir, f.Name); isTaken(taken, path, f.Name) {
			return &CollisionError{Path: path}
		}
		if path := tempPathFor(opt.OutDir, f.Name); isTaken(taken, path, tempNameFor(f.Name)) {
			return &CollisionError{Path: path}
		}
	}
	return nil
}

// namesIn is every name the output directory holds, or nil when it could not be
// listed at all.
//
// nil is not the same answer as empty, and the difference is the one failure
// this check exists to prevent. A directory that cannot be READ can still be
// one a file may be created in - both systems allow that combination - and
// treating it as empty would let a run write over somebody's work. So nil means
// "ask file by file" rather than "there is nothing there".
//
// A directory that does not exist yet is a different answer and a correct one:
// it holds no names, nothing can collide, and that is the ordinary case for a
// fresh run.
func namesIn(dir string) map[string]struct{} {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]struct{}{}
	}
	if err != nil {
		return nil
	}
	taken := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		taken[e.Name()] = struct{}{}
	}
	return taken
}

// isTaken answers whether a name is already in use - from the listing when
// there is one, and from the filesystem when there is not.
//
// The two are not quite the same question and the listing asks the better one.
// os.Stat follows a link, so a link pointing at nothing answered "no name
// here" and the run went on to replace it. A directory ENTRY is what a name
// being taken means, whatever it points at, and the rule this serves is that
// nothing already there is written over. So the listing refuses a little more
// than the stat did, in the direction that cannot lose somebody's work.
func isTaken(taken map[string]struct{}, path, name string) bool {
	if taken == nil {
		return exists(path)
	}
	_, ok := taken[name]
	return ok
}

// tempPathFor is the name a file is written under before it is renamed into
// place. One definition, used by the check above and by the write below, so
// the two cannot disagree about what is being protected.
func tempPathFor(outDir, name string) string {
	return filepath.Join(outDir, tempNameFor(name))
}

// tempNameFor is that name without the directory in front of it, which is what
// a directory listing gives back. Split out so the listing and the path cannot
// disagree about how the temporary name is spelt.
func tempNameFor(name string) string {
	return fmt.Sprintf("%s%s%d", name, core.PartialMarker, os.Getpid())
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
