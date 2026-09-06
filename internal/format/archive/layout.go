package archive

import (
	"strconv"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// Where the files inside an archive sit, and whether the archive lists the
// directories themselves.
//
// Both containers read this one description rather than each growing its own,
// for the reason the package exists: zip and tar held six constants with
// identical values and two identical readers before it, and the only thing
// keeping them equal was a comment.
//
// Two settings rather than one, and the pairing is deliberate. A tester asking
// "does the tool under test cope with nested paths" wants depth. A tester
// asking "does it cope with an archive that does NOT list its directories"
// wants the other, because extractors differ: some create a directory when
// they meet a path that needs one, and some only create what the archive
// names. An archive is the one format where those are separate questions.
const (
	Depth            = "depth"
	DirectoryEntries = "directory_entries"
)

const (
	// defaultDepth is flat, and it has to stay flat. Every archive this tool
	// has written so far holds its files at the top, so any other default
	// would move the bytes of all of them - untouchable rule 3, and the reason
	// this change needs no version bump at all.
	defaultDepth = 0

	// maxDepth is measured rather than chosen, and the measurement is about
	// tar rather than about taste.
	//
	// targz pins tar.FormatUSTAR (size.go), which carries a path in two
	// fields: a 155 byte prefix and a 100 byte name, split ON A SLASH. So a
	// path is writable when some slash leaves at most 155 before it and at
	// most 100 after it, which is a rule about where the slashes fall and not
	// about length. With the segments below, slashes sit every 4 bytes, the
	// last usable one is at 155, and the path therefore has to come to 256
	// bytes or fewer: 4*depth + len(entry name) <= 256.
	//
	// Measured 2026-09-01 against Go's own archive/tar with USTAR pinned:
	// depth 61 with a 12 byte name is taken at 256 bytes and depth 62 is
	// REFUSED at 260. The size is flat the whole way - 1536 B at every depth
	// up to the refusal, no hidden step - so the tar arithmetic needs no
	// length term at all.
	//
	// 61 is therefore the ceiling for a 12 byte name and NOT the ceiling to
	// declare, because the entry name is not always 12 bytes. The longest one
	// this build can produce is targz_0001.tar.gz at 17, which lands the limit
	// at 59. Fifty leaves room for a name of 56 bytes, which is far past
	// anything the registry holds, and a guard proves it for every registered
	// format rather than trusting this paragraph.
	maxDepth = 50

	// A level is written as "d00/": the letter, the number padded to two
	// digits because maxDepth is two digits, and the separator. A fixed width
	// is what keeps the arithmetic above a multiplication rather than a walk.
	//
	// It used to be a "d%02d/" format string rendered through fmt, and that
	// cost more than everything else about naming an entry put together -
	// measured 2026-09-06 at depth 50 over 10 000 entries, 82.2 ms against
	// 30.6 ms once the verb went. Prefix writes the four bytes out by hand.
	//
	// dirSegmentBytes is what one level comes to. It was written out rather
	// than taken as the length of that format string, which is the bug the
	// depth guard caught the first time it ran: six bytes rather than four
	// said every path was 2 bytes per level longer than it is, which
	// understates the ceiling instead of overstating it, so nothing would have
	// failed until somebody widened the segment. A guard still compares this
	// against a really rendered path.
	dirSegmentBytes = 4
)

// Layout is what the two settings come to once read.
type Layout struct {
	// Depth is how many directories deep the files sit. Zero is flat.
	Depth int
	// DirEntries says whether the archive also names the directories.
	DirEntries bool
}

// Path is where an entry called name sits under this layout.
//
// The empty name gives the directory chain itself with its trailing slash,
// which is what both containers want a directory entry to be called.
func (l Layout) Path(name string) string {
	return l.Prefix() + name
}

// Prefix is the directory chain on its own, with nothing on the end.
//
// It depends on Depth and on nothing else, so a caller naming thousands of
// entries works it out once rather than once per entry. Measured 2026-09-06 at
// depth 50 over 10 000 entries: a median of 82.2 ms rebuilding it every time
// against 1.08 ms building it once, ranges disjoint.
//
// The default depth is zero and a flat archive spends nothing here either way,
// so this is a ceiling rather than a typical run - which is worth saying,
// because the number above reads like a saving every user gets.
//
// Rendered by hand rather than through fmt: the format verb is what made the
// old version expensive, and a two digit number with a floor of two is small
// enough to write out. "d%02d/" pads to two and lets a third digit through,
// which is what the branch below does.
func (l Layout) Prefix() string {
	if l.Depth <= 0 {
		return ""
	}
	var b strings.Builder
	b.Grow(l.Depth * dirSegmentBytes)
	for i := 0; i < l.Depth; i++ {
		b.WriteByte('d')
		if i < 10 {
			b.WriteByte('0')
		}
		b.WriteString(strconv.Itoa(i))
		b.WriteByte('/')
	}
	return b.String()
}

// Directories is every directory this layout creates, outermost first.
//
// Outermost first because that is the order an extractor wants to meet them
// in: a reader that creates directories as it goes cannot make d00/d01 before
// it has made d00. It is empty when nothing was asked for, so a caller can
// range over it without asking whether the setting is on.
func (l Layout) Directories() []string {
	if !l.DirEntries || l.Depth <= 0 {
		return nil
	}
	out := make([]string, 0, l.Depth)
	for i := 1; i <= l.Depth; i++ {
		out = append(out, Layout{Depth: i}.Path(""))
	}
	return out
}

// LongestPath is the longest path this layout can produce for an entry name of
// the given length. It exists for the guard that proves maxDepth is safe.
func LongestPath(depth, nameLen int) int {
	return depth*dirSegmentBytes + nameLen
}

// MaxDepth is the deepest nesting this build offers, for the guard that checks
// the declaration against what tar will actually take.
func MaxDepth() int { return maxDepth }

// ReadLayout works out where the files go, and refuses a pair that cannot mean
// anything.
//
// directory_entries with a flat archive is the pair, and it is a refusal
// rather than a setting quietly doing nothing. There are no directories in a
// flat archive, so the answer would be the same whichever way it was set - and
// rule 6 forbids exactly that silence. The message names BOTH halves, because
// a reader who set one of them cannot tell from "directory_entries is not
// allowed" which one to change.
//
// It is reachable from the window as well as from a recipe, which is why it
// has to be a good message rather than an internal check: the control is a
// checkbox, a checkbox always sends its value, and somebody can tick it while
// depth is still nought.
func ReadLayout(id string, r format.Request) (Layout, error) {
	depth, err := intProperty(id, r.Properties, Depth, defaultDepth, 0, maxDepth)
	if err != nil {
		return Layout{}, err
	}
	dirs, err := boolProperty(id, r.Properties, DirectoryEntries, false)
	if err != nil {
		return Layout{}, err
	}
	if dirs && depth == 0 {
		return Layout{}, &format.PropertyValueError{
			Format: id,
			Key:    DirectoryEntries,
			Value:  "true",
			Reason: "a flat archive has no directories to list, and " + Depth + " is 0",
			Remedy: "Ask for " + Depth + " of 1 or more, or leave " + DirectoryEntries + " off.",
		}
	}
	return Layout{Depth: depth, DirEntries: dirs}, nil
}

// boolProperty reads a true or false setting.
//
// The registry has already refused anything that is not true or false by the
// time a generator runs, since the declaration says the kind. This repeats the
// check for the same reason intProperty repeats its range: a caller reaching
// the generator directly is not going through the registry.
func boolProperty(id string, props map[string]string, key string, fallback bool) (bool, error) {
	raw, ok := props[key]
	if !ok || raw == "" {
		return fallback, nil
	}
	v, err := strconv.ParseBool(strings.ToLower(raw))
	if err != nil {
		return false, &format.PropertyValueError{
			Format: id,
			Key:    key,
			Value:  raw,
			Reason: "it takes true or false",
			Remedy: "Write " + key + ": true or " + key + ": false.",
		}
	}
	return v, nil
}
