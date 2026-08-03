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

// hasVolumeName spots a Windows volume prefix such as "C:". filepath.VolumeName
// answers for the system this build runs on, which is exactly what must not
// decide it here.
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
	base, err := resolveAsFarAsItExists(dir)
	if err != nil {
		// Nothing could be resolved, so nothing can be judged. Saying "it
		// escapes" would refuse an ordinary run on a path we simply could not
		// examine, and the caller checks the text separately.
		return false
	}
	target, err := resolveAsFarAsItExists(full)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(base, target)
	if err != nil {
		// Different volumes have no relative path between them, which is as
		// far outside as it gets.
		return true
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return true
	}
	return crossesUnresolvedLink(base, rel)
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
