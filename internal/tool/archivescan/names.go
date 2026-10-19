package archivescan

import (
	"strings"

	"golang.org/x/text/unicode/norm"

	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
)

// Where the entries of an archive land, judged the same way on every system:
// an archive travels between machines, and one judged safe on Linux is
// unpacked on Windows next. So both separators are separators everywhere, the
// way the generator refuses both in a name (engine/filename.go).

// isSeparator is either separator, on every system.
func isSeparator(r rune) bool { return r == '/' || r == '\\' }

// parts is a path inside an archive split at both separators, with the empty
// parts two separators in a row leave out.
func parts(name string) []string {
	return strings.FieldsFunc(name, isSeparator)
}

// leaves says whether unpacking an entry under this name writes outside the
// folder it is unpacked into: a path from the root, from a drive, or one that
// climbs out.
//
// Any part that climbs is reported, even "a/../b", which stays inside. The
// generator refuses ".." in a name for the same reason, and a name that only
// stays inside after a program works it out is one that some program will not.
// A part of dots and spaces alone counts as climbing too, because Windows
// drops dots and spaces at the end of a name, so ".. " is ".." there.
func leaves(name string) bool {
	if name == "" {
		return false
	}
	if name[0] == '/' || name[0] == '\\' {
		return true
	}
	if drive(name) {
		return true
	}
	for _, p := range parts(name) {
		if climbs(p) {
			return true
		}
	}
	return false
}

// drive says whether a name starts with a drive letter, as "C:" and "C:x" do -
// the second is a path on that drive's current folder on Windows.
func drive(name string) bool {
	return len(name) >= 2 && name[1] == ':' && isLetter(name[0])
}

// climbs says whether a part of a path names the folder above, on any system.
func climbs(part string) bool {
	return strings.Trim(part, ". ") == "" && strings.Count(part, ".") >= 2
}

func isLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// unwritable says whether some part of a name is one a system cannot hold as
// it is - judged by the rules the generator writes its own names by, part by
// part. An entry with no name at all is one too.
func unwritable(name string) bool {
	ps := parts(name)
	if len(ps) == 0 {
		return name == ""
	}
	// A drive is not a name, and an entry that starts with one is reported as
	// leaving the folder already.
	if drive(name) {
		ps = ps[1:]
	}
	for _, p := range ps {
		if p == "." || climbs(p) {
			continue
		}
		if engine.Unwritable(p) {
			return true
		}
	}
	return false
}

// landing is the name a part of a path lands under where names are told apart
// least: case ignored as Windows and macOS do, accents composed as macOS does,
// and dots and spaces at the end dropped as Windows does. Two names with one
// landing may be one file once unpacked - "may", because the case rules of the
// file systems are tables of their own and this is the part they share.
func landing(part string) string {
	if kept := strings.TrimRight(part, ". "); kept != "" {
		part = kept
	}
	return strings.ToLower(norm.NFC.String(part))
}

// place is one file or folder an archive unpacks to: the folder it is in, and
// its name as it lands.
type place struct {
	in   int32
	name string
}

// book is every place the entries seen so far unpack to, as a tree, to find
// the second entry that lands on a file already written, or a file where a
// folder has to be.
//
// A tree rather than whole paths, so the work for an entry is the length of
// its name: looking up every folder above an entry by its whole path costs the
// square of its depth, and an archive of deep, different paths would turn
// this tool into the thing it warns about.
type book struct {
	at    map[place]int32
	file  []bool
	named []string
	// room is the places the books of one run may still hold, all of them
	// together.
	room *int
}

// root is the folder an archive is unpacked into.
const root = -1

func newBook(room *int) *book {
	return &book{at: map[place]int32{}, room: room}
}

// full says the books of the run hold as many places as they are allowed to.
func (b *book) full() bool { return *b.room <= 0 }

// add records an entry, and says which entry seen before it lands on the same
// place - or put a file where this one needs a folder - if one did.
func (b *book) add(name string, folder bool) (string, bool) {
	// A copy, held: archive/tar hands back a name set in a PAX header as a
	// slice of the whole header, and holding the slice holds up to a
	// megabyte for every entry (docs/SKANER-W-GLAB-2026-10-19.md §1).
	name = strings.Clone(name)
	in := int32(root)
	ps := parts(name)
	for i, p := range ps {
		if p == "." {
			continue
		}
		file := i == len(ps)-1 && !folder
		here := place{in: in, name: landing(p)}
		id, seen := b.at[here]
		if !seen && b.full() {
			return "", false
		}
		if !seen {
			in = b.put(here, name, file)
			continue
		}
		if b.file[id] || file {
			return b.named[id], true
		}
		in = id
	}
	return "", false
}

// put records a place first reached by the entry with this name.
func (b *book) put(here place, name string, file bool) int32 {
	id := int32(len(b.named))
	*b.room--
	b.at[here] = id
	b.file = append(b.file, file)
	b.named = append(b.named, name)
	return id
}
