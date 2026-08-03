package core

import (
	"path"
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
