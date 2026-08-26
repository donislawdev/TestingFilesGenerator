package core

import (
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Whether a string resolved against a directory still lands inside it.
//
// The writing side answers a stricter version of this question in the engine,
// where a file name may not carry a separator at all. That rule subsumes this
// one, so the two cannot disagree: a string with no separator, no volume and no
// dots is contained by construction.
//
// The reading side needs the looser version. A manifest carries a path rather
// than a bare name, because a run that groups its output into folders has to be
// verified the same way - so "invoices/a.txt" is legitimate and "../a.txt" is
// not, and only the second is refused.
//
// It lives here rather than beside either caller because both are asking one
// question about one concept, and two implementations of it are two things to
// keep in step. That is the same argument that put ParseSize here.

// ContainmentProblem reports why a relative path would not stay inside the
// directory it is resolved against. An empty string means it stays.
//
// The answer does not depend on the system this runs on. A manifest travels
// between machines by design - it ships with a fixture set and it arrives in
// pull requests - so a path that is refused on Windows has to be refused on
// Linux too. A backslash is a separator here on every system, and a volume
// letter is a volume letter even where the local rules would read it as an
// ordinary name.
func ContainmentProblem(p string) string {
	if p == "" {
		return "it is empty"
	}

	unified := strings.ReplaceAll(p, `\`, "/")

	// A leading separator starts at the root of the disk, and two of them start
	// at a machine name. Neither is a path inside anything.
	if strings.HasPrefix(unified, "/") {
		return "it starts at the root of the disk rather than inside the directory"
	}
	if hasVolumeName(unified) {
		return "it names a disk volume rather than a place inside the directory"
	}

	// Cleaned with slash rules rather than the local ones, so the verdict is
	// the same everywhere. Anything that still begins with a climb has left.
	cleaned := path.Clean(unified)
	switch {
	case cleaned == "..", strings.HasPrefix(cleaned, "../"):
		return "it climbs above the directory"
	case cleaned == ".":
		return "it names the directory itself rather than a file inside it"
	}
	return ""
}

// HasVolumeName spots a Windows volume prefix such as "C:".
//
// filepath.VolumeName answers for the system this build runs on, which is
// exactly what must not decide it. Measured on 2026-08-04: the name "a:b.txt"
// was accepted on Linux and refused on Windows as an absolute path, from one
// recipe - so a fixture set written on one machine failed on the next, which
// is the thing the separator rule beside it exists to prevent.
func HasVolumeName(p string) bool { return hasVolumeName(p) }

func hasVolumeName(p string) bool {
	if len(p) < 2 || p[1] != ':' {
		return false
	}
	c := p[0]
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// EscapesAfterResolving reports whether full, once every link on the way has
// been followed, still lands inside dir.
//
// ContainmentProblem above reads the string. This asks the filesystem, and the
// difference is the whole point: "jn/VICTIM.txt" holds no climb and passes
// every reading of the text, while a directory called "jn" that points
// somewhere else takes it out of the directory anyway.
//
// Measured on 2026-08-03, with the textual check already in place: a junction
// inside the output directory and an entry naming a file through it, and
// cleanup --yes --force removed a file above the directory with exit code 0.
// docs/SECURITY.md section 2.4 already carried the rule - check after resolving
// the path, not before - marked as brought in from elsewhere and NOT VERIFIED
// here. It was right.
//
// A link is not refused for being a link. People keep fixtures on redirected
// paths on purpose, and a workspace that is itself a link has to keep working.
// The question is only where the path ends up.
func EscapesAfterResolving(dir, full string) bool {
	return NewBoundary(dir).Escapes(full)
}

// Boundary is the directory a run was pointed at, with the links on the way
// followed once instead of once per entry.
//
// It exists for speed, and the speed is not a detail. Judging one entry used to
// resolve the whole directory from the root down, and a run judges thousands of
// them. Measured on 2026-08-20 with 3000 entries, verify on Windows:
// resolving the directory again for every entry was 5665 ms of a 13.4 s run,
// and resolving each file from the root down was another 8703 ms - against
// 626 ms to actually open and hash all 3000 files. On Windows the cost of
// filepath.EvalSymlinks grows with the depth of the path, about 0.24 ms per
// component here, so both numbers are really the same mistake: walking the
// same ancestors over and over. The full measurement is observation O117.
//
// What it does NOT cache is the part that does the work. Every entry still gets
// its own walk below the boundary, asking the filesystem in the state it is in
// at that moment - see crossesUnresolvedLink. Only the boundary itself is
// settled once, and that is the directory the person named, whose own
// redirections are their business rather than ours.
type Boundary struct {
	named    string
	abs      string
	resolved string
	known    bool
}

// NewBoundary follows the links on the way to dir, once.
//
// The absolute spelling is kept beside the name because filepath.Rel refuses to
// compare a relative path against an absolute one, and the directory arrives
// however the person typed it - "tfg verify ./tfg-out/manifest.json" is the
// ordinary way to run this. Without it the cheap comparison in Escapes would
// fail for exactly the people who type the shorter thing, fall back to the
// expensive reading, and be slow in the field while every guard here stayed
// green: they all build their directories with t.TempDir, which is absolute.
func NewBoundary(dir string) Boundary {
	b := Boundary{named: dir, abs: dir}
	if abs, err := filepath.Abs(dir); err == nil {
		b.abs = abs
	}
	r, err := resolveAsFarAsItExists(dir)
	if err != nil {
		// Nothing could be resolved, so nothing can be judged - and Escapes
		// says so by refusing rather than by allowing. See the paragraph on
		// b.known there for why that way round.
		//
		// This is narrower than it looks. resolveAsFarAsItExists returns an
		// error only when filepath.Abs does, and that needs the working
		// directory to be gone, so a run reaching here is already broken in a
		// way that has nothing to do with containment.
		return b
	}
	b.resolved, b.known = r, true
	return b
}

// Dir is the directory as the caller named it, before any link was followed.
func (b Boundary) Dir() string { return b.named }

// Escapes reports whether full lands outside this boundary once the links on
// the way have been followed.
//
// It gives the same answer as resolving both ends, in every case. The saving is
// not a different rule, it is the same rule reached without walking the
// ancestors again, and it only applies to the case where nothing redirects at
// all - which is every ordinary run.
//
// The reasoning, because getting it wrong here is a containment bug. When a
// path is written inside the directory we were given and no step below that
// directory redirects anywhere, the path as written IS the real path: the
// ancestors resolve to the boundary we already resolved, and the steps below it
// resolve to themselves. So it is inside, and resolving it could not have said
// otherwise.
//
// The moment a step below the boundary does redirect, that shortcut stops
// holding and the thorough reading decides. It has to, and this was nearly got
// wrong: a link inside the directory that points at another file inside the
// same directory is NOT an escape, and the old reading allows it - it follows
// the link, lands inside, and says so. Refusing it here because a link was
// seen would turn a contained directory into a hard refusal, which is a change
// to what this tool accepts rather than a change to how fast it answers.
// Both ways of not knowing answer "it escapes", and that direction is the
// whole of this paragraph. A boundary that could not be resolved and a path
// that could not be made absolute are the same situation: this cannot tell
// where the path lands. Answering "it stays inside" there is an answer to a
// question nobody asked and it is the unsafe one - it lets a link out of the
// directory past the only check that looks at links, since the other layer
// reads the text and a name that leaves through a link holds no climb to read.
//
// It used to answer "it stays inside" on both, reasoning that refusing would
// stop an ordinary run over a path we simply could not examine. That reasoning
// had no case behind it: a zero value Boundary is the only way to reach the
// first branch without the working directory being gone, and a run whose
// working directory is gone is not an ordinary run. Turned round on 2026-08-27
// with the review's N5, and the comment on NewBoundary was turned round with
// it rather than left saying the opposite of the code beneath it.
func (b Boundary) Escapes(full string) bool {
	if !b.known {
		return true
	}
	target, err := filepath.Abs(full)
	if err != nil {
		return true
	}
	// Written inside the directory we were given, and nothing below that
	// directory redirects anywhere. Then the path as written is the real path,
	// and resolving it from the root down could not have said otherwise.
	if rel, err := filepath.Rel(b.abs, target); err == nil && !climbsOut(rel) && !crossesUnresolvedLink(b.resolved, rel) {
		return false
	}
	// Anything else is a question rather than an answer, and the thorough
	// reading is what answers it. A path written outside the name can still be
	// inside once the links are followed, and a redirection below the boundary
	// can point either way.
	return b.escapesTheThoroughWay(full)
}

// escapesTheThoroughWay is the original reading: resolve both ends and compare
// them. Kept for a full path that was not written inside the directory, where
// the cheap comparison above has no shared prefix to work from.
func (b Boundary) escapesTheThoroughWay(full string) bool {
	target, err := resolveAsFarAsItExists(full)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(b.resolved, target)
	if err != nil {
		// Different volumes have no relative path between them, which is as
		// far outside as it gets.
		return true
	}
	if climbsOut(rel) {
		return true
	}
	return crossesUnresolvedLink(b.resolved, rel)
}

// climbsOut reports whether a relative path steps above what it is relative to.
func climbsOut(rel string) bool {
	return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// crossesUnresolvedLink reports whether any step from base down to rel is a
// redirection the resolver above did not follow.
//
// It exists because filepath.EvalSymlinks does not resolve a Windows junction.
// Measured on 2026-08-03, on a directory made three ways:
//
//	symbolic link  Lstat says ModeSymlink, EvalSymlinks resolves it
//	junction       Lstat says neither, EvalSymlinks returns the path unchanged
//	               and reports no error at all
//
// So the comparison above says "inside" while the filesystem goes somewhere
// else. That is worse than not checking, because it reads as a check that
// passed. The escape was reproduced with a junction after the resolving check
// was already in place: cleanup --yes --force removed a file above the output
// directory with exit code 0, while the guard - which builds its link with
// os.Symlink - stayed green.
//
// A step that cannot be resolved and cannot be ruled out is refused rather than
// followed. This walks only the part of the path that came from the manifest,
// never the directory the user named: that directory is the boundary, so
// whatever redirections lie above it are their choice and not our business.
func crossesUnresolvedLink(base, rel string) bool {
	if rel == "." {
		return false
	}
	cur := base
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		cur = filepath.Join(cur, part)
		fi, err := os.Lstat(cur)
		if err != nil {
			// Nothing there to travel through. A file that does not exist
			// cannot lead anywhere, and cleanup asks about entries that are
			// already gone.
			return false
		}
		// ModeIrregular is what a junction arrives as. ModeSymlink is here too
		// because a link created between the two passes would otherwise slip
		// through on the strength of the earlier resolution.
		if fi.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return true
		}
	}
	return false
}

// resolveAsFarAsItExists follows the links it can and keeps the rest of the
// name as written.
//
// A file that is not there yet still has to be judged - cleanup asks about
// entries that are already gone - so this walks up to the deepest part that
// does exist, resolves that, and puts the remainder back on.
func resolveAsFarAsItExists(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	rest := ""
	for cur := abs; ; {
		if resolved, err := filepath.EvalSymlinks(cur); err == nil {
			if rest == "" {
				return resolved, nil
			}
			return filepath.Join(resolved, rest), nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			// Reached the root without finding anything that exists.
			return filepath.Join(cur, rest), nil
		}
		rest = filepath.Join(filepath.Base(cur), rest)
		cur = parent
	}
}
