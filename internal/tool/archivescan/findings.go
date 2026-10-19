package archivescan

import (
	"context"
	"math"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/tool"
)

// What a scan found, kept as notes from the first entry to the result.

// The keys of the notes. Also the keys of what --json reports as found, so
// each is a public name (untouchable rule 10).
const (
	noteKind        = "kind"
	noteEntries     = "entries"
	notePacked      = "size"
	noteUnpacked    = "unpacked"
	noteUnpackedAll = "unpacked_all"

	noteLeaves      = "leaves_folder"
	noteLinks       = "links"
	noteSpecial     = "special"
	noteCollides    = "collides"
	noteUnwritable  = "unwritable"
	noteTwoNames    = "two_names"
	noteOverlapping = "overlapping"
	noteBomb        = "bomb"
	noteLonger      = "longer_than_said"
	noteOverLimit   = "over_limit"
	noteDamaged     = "damaged"
	noteUnchecked   = "unchecked"
	noteStopped     = "stopped"

	noteNested = "nested"
)

// declared is every note, in the order a result says them: what the archive
// is, then everything that makes it unsafe, then what is worth knowing and is
// harmless on its own.
var declared = []struct {
	tool.Note
	// harmless is a note that does not make an archive unsafe.
	harmless bool
}{
	{tool.Note{ID: noteKind, Says: "Archive:"}, true},
	{tool.Note{ID: noteEntries, Says: "Entries:"}, true},
	{tool.Note{ID: notePacked, Says: "Size:"}, true},
	{tool.Note{ID: noteUnpacked, Says: "Unpacked:"}, true},
	{tool.Note{ID: noteUnpackedAll, Says: "Unpacked, with the archives inside it:"}, true},
	{tool.Note{ID: noteLeaves, Says: "Entries that would be written outside the folder:"}, false},
	{tool.Note{ID: noteLinks, Says: "Links, which a later entry can be written through:"}, false},
	{tool.Note{ID: noteSpecial, Says: "Entries that are not plain files or folders:"}, false},
	{tool.Note{ID: noteCollides, Says: "Entries that land on the same file as another on Windows or macOS:"}, false},
	{tool.Note{ID: noteUnwritable, Says: "Names that Windows, macOS or Linux cannot write as they are:"}, false},
	{tool.Note{ID: noteTwoNames, Says: "Entries named one way in the list of contents and another at the entry itself:"}, false},
	{tool.Note{ID: noteOverlapping, Says: "Entries that share their bytes with another, which only an archive built to explode does:"}, false},
	{tool.Note{ID: noteBomb, Says: "Unpacks to far more than its own size, the way an archive built to explode does:"}, false},
	{tool.Note{ID: noteLonger, Says: "Entries holding more than the archive says they do:"}, false},
	{tool.Note{ID: noteOverLimit, Says: "Unpacks to more than the limit given:"}, false},
	{tool.Note{ID: noteDamaged, Says: "Damaged, so programs that unpack it may disagree on what it holds:"}, false},
	{tool.Note{ID: noteUnchecked, Says: "Entries whose contents could not be checked:"}, false},
	{tool.Note{ID: noteStopped, Says: "Reading stopped before the end:"}, false},
	{tool.Note{ID: noteNested, Says: "Archives inside it, read as well:"}, true},
}

// declaredNotes is what the registry is told.
func declaredNotes() []tool.Note {
	out := make([]tool.Note, len(declared))
	for i, d := range declared {
		out[i] = d.Note
	}
	return out
}

// found is one thing a scan found: the entry it is about, as data, and the
// words for it when the entry alone does not say it.
type found struct {
	name  string
	words core.Said
}

// Report is what --json prints. Names are the names in the archive, byte for
// byte, so a script compares them - up to MostShownName bytes, where a longer
// one is cut. The window and the text show them escaped.
type Report struct {
	File     string `json:"file"`
	Kind     string `json:"kind"`
	Bytes    int64  `json:"bytes"`
	Entries  int    `json:"entries"`
	Unpacked uint64 `json:"unpacked_bytes"`
	// UnpackedAll is what it unpacks to with every archive inside it.
	UnpackedAll uint64 `json:"unpacked_all_bytes"`
	Safe        bool   `json:"safe"`
	// Found is every finding in this archive by its key, each with the
	// entries it is about, at most MostListed of them. A finding about the
	// archive as a whole has no entry and an empty list.
	Found map[string][]string `json:"found"`
	// More is how many findings of a key there are past what Found lists.
	More map[string]int `json:"more,omitempty"`
	// Inside are the reports of the archives in this one that are not safe,
	// each File the name it has here, and InsideMore how many more of them
	// there are than Inside carries.
	Inside     []*Report `json:"inside,omitempty"`
	InsideMore int       `json:"inside_more,omitempty"`
}

// scan is the reading of one archive: the one given, or one inside it.
type scan struct {
	path string
	size int64
	read int64

	kind     kind
	entries  int
	unpacked uint64
	names    *book
	stopped  bool
	// exploded is an archive found built to explode, noted once.
	exploded bool

	found findings
	nest  nest

	tree  *tree
	depth int
	// hold keeps what an entry holds while it is read, when that is an
	// archive to read in turn.
	hold holding
}

// findings are what a scan found: own in this archive, deep in the archives
// inside it, each said with the way to it, and in more and deepMore how many
// of each were found past what a report lists.
type findings struct {
	own      map[string][]found
	more     map[string]int
	deep     map[string][]found
	deepMore map[string]int
}

// nest is what the archives inside one came to: those kept for the report,
// how many more were not safe, whether any was not safe, and what they all
// unpack to.
type nest struct {
	kept     []*scan
	more     int
	unsafe   bool
	unpacked uint64
}

// newScan is the reading of the archive given.
func newScan(ctx context.Context, path string, size, limit int64, progress tool.Progress) *scan {
	t := &tree{
		ctx: ctx, limit: limit, progress: progress,
		placeRoom: MostPlaces, nameRoom: MostNameBytes, buf: make([]byte, 64<<10),
	}
	s := blank(path, size, t)
	t.root = s
	return s
}

// blank is the reading of an archive in the run t, before anything is read.
func blank(path string, size int64, t *tree) *scan {
	return &scan{
		path: path, size: size, tree: t, names: newBook(&t.placeRoom),
		found: findings{own: map[string][]found{}, more: map[string]int{}, deep: map[string][]found{}, deepMore: map[string]int{}},
	}
}

// note records a finding about an entry, shown as the entry's name.
func (s *scan) note(id, name string) {
	s.noteSaid(id, name, core.Said{})
}

// noteSaid records a finding with words of its own, or counts it once the
// report has no room left to list it.
func (s *scan) noteSaid(id, name string, words core.Said) {
	s.tree.add(s.found.own, s.found.more, id, found{name: name, words: words})
}

// counted tells the progress how much more of the archive has been read -
// of the archive given: the ones inside it are read from memory.
func (s *scan) counted(n int64) {
	s.read += n
	if s == s.tree.root {
		s.tree.progress(min(s.read, s.size), s.size)
	}
}

// stop ends reading, saying why. Everything after this point is unseen, so
// the archive cannot be called safe (MF5).
func (s *scan) stop(why core.Said) {
	if s.stopped {
		return
	}
	s.stopped = true
	s.noteSaid(noteStopped, "", why)
}

// going says whether reading goes on: not once this archive has stopped, nor
// once the run is found built to explode. The archive given says why then, and
// every archive between it and the one that found out stops with it.
func (s *scan) going() bool {
	if s.tree.exploded {
		s.stopped = true
	}
	return !s.stopped
}

// entry is what every kind of archive does for each entry it lists: count it,
// judge its name, and record where it lands. False when reading has to stop.
func (s *scan) entry(name string, folder bool) bool {
	if !s.going() {
		return false
	}
	t := s.tree
	if t.entries >= MostEntries {
		s.stop(core.Says("archivescan.StoppedEntries", "with the archives inside it, it lists more than %s entries, and no more are read",
			core.A("Most", core.Exactly(MostEntries))))
		return false
	}
	t.nameRoom -= len(name)
	if t.nameRoom < 0 || s.names.full() {
		s.stop(core.Says("archivescan.StoppedNames", "its names, with those of the archives inside it, take more room than this tool sets aside to compare them, and no more are read"))
		return false
	}
	t.entries++
	s.entries++
	if leaves(name) {
		s.note(noteLeaves, name)
	}
	if unwritable(name) {
		s.note(noteUnwritable, name)
	}
	if first, ok := s.names.add(name, folder); ok {
		s.noteSaid(noteCollides, name, core.Says("archivescan.Same", "%s = %s", core.A("Name", shownName(name)), core.A("First", shownName(first))))
	}
	return true
}

// grow adds to what an archive unpacks to, and to what the run does.
func (s *scan) grow(n uint64) {
	s.unpacked = addCapped(s.unpacked, n)
	s.tree.unpacked = addCapped(s.tree.unpacked, n)
}

// weigh is the size of what an archive unpacks to, against the archive and,
// with every archive read so far, against the archive given.
func (s *scan) weigh() {
	if !s.exploded && s.unpacked >= BombBytes && s.unpacked > uint64(s.size)*BombRatio {
		s.exploded = true
		s.noteSaid(noteBomb, "", core.Says("archivescan.Bomb", "%s from %s", core.A("Unpacked", bytesOf(s.unpacked)), core.A("Size", core.Bytes(s.size))))
	}
	s.tree.check(s)
}

// overLimit is what the archive given unpacks to, with every archive inside
// it, against the limit given.
func (s *scan) overLimit() {
	all := addCapped(s.unpacked, s.nest.unpacked)
	if limit := s.tree.limit; limit >= 0 && all > uint64(limit) {
		s.noteSaid(noteOverLimit, "", core.Says("archivescan.OverLimit", "%s against a limit of %s", core.A("Unpacked", bytesOf(all)), core.A("Limit", core.Bytes(limit))))
	}
}

// bytesOf is a count of bytes for a sentence. An archive can claim more than
// a signed number holds, and says the most one holds then.
func bytesOf(n uint64) core.Bytes {
	if n > math.MaxInt64 {
		return core.Bytes(math.MaxInt64)
	}
	return core.Bytes(n)
}

// special notes an entry that is not a plain file or folder, and why.
func (s *scan) special(name string, why core.Said) {
	s.noteSaid(noteSpecial, name, entryWhy(name, why))
}

// entryWhy is an entry and what is wrong with it, as one item of a note.
func entryWhy(name string, why core.Said) core.Said {
	return core.Says("archivescan.EntryWhy", "%s: %s", core.A("Name", shownName(name)), core.A("Why", why))
}
