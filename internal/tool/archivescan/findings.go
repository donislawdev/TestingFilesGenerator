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
	noteKind     = "kind"
	noteEntries  = "entries"
	notePacked   = "size"
	noteUnpacked = "unpacked"

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
	{tool.Note{ID: noteNested, Says: "Archives inside it, which were not opened:"}, true},
}

// declaredNotes is what the registry is told.
func declaredNotes() []tool.Note {
	out := make([]tool.Note, len(declared))
	for i, d := range declared {
		out[i] = d.Note
	}
	return out
}

// harmless says whether a note leaves an archive safe to unpack.
func harmless(id string) bool {
	for _, d := range declared {
		if d.ID == id {
			return d.harmless
		}
	}
	return false
}

// found is one thing a scan found: the entry it is about, as data, and the
// words for it when the entry alone does not say it.
type found struct {
	name  string
	words core.Said
}

// Report is what --json prints. Names are the names in the archive, byte for
// byte, so a script compares them. The window and the text show them escaped.
type Report struct {
	File     string `json:"file"`
	Kind     string `json:"kind"`
	Bytes    int64  `json:"bytes"`
	Entries  int    `json:"entries"`
	Unpacked uint64 `json:"unpacked_bytes"`
	Safe     bool   `json:"safe"`
	// Found is every finding by its key, each with the entries it is about.
	// A finding about the archive as a whole has no entry and an empty list.
	Found map[string][]string `json:"found"`
}

// scan is one run of the tool over one archive.
type scan struct {
	ctx      context.Context
	path     string
	size     int64
	limit    int64
	progress tool.Progress
	read     int64

	kind     kind
	entries  int
	unpacked uint64
	names    *book
	nameRoom int
	stopped  bool
	// exploded is an archive found built to explode, noted once.
	exploded bool

	found map[string][]found
}

func newScan(ctx context.Context, path string, size, limit int64, progress tool.Progress) *scan {
	return &scan{
		ctx: ctx, path: path, size: size, limit: limit, progress: progress,
		names: newBook(), nameRoom: MostNameBytes, found: map[string][]found{},
	}
}

// note records a finding about an entry, shown as the entry's name.
func (s *scan) note(id, name string) {
	s.found[id] = append(s.found[id], found{name: name})
}

// noteSaid records a finding with words of its own.
func (s *scan) noteSaid(id, name string, words core.Said) {
	s.found[id] = append(s.found[id], found{name: name, words: words})
}

// counted tells the progress how much more of the archive has been read.
func (s *scan) counted(n int64) {
	s.read += n
	s.progress(min(s.read, s.size), s.size)
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

// entry is what every kind of archive does for each entry it lists: count it,
// judge its name, and record where it lands. False when reading has to stop.
func (s *scan) entry(name string, folder bool) bool {
	if s.entries >= MostEntries {
		s.stop(core.Says("archivescan.StoppedEntries", "it lists more than %s entries, and no more are read", core.A("Most", core.Exactly(MostEntries))))
		return false
	}
	s.nameRoom -= len(name)
	if s.nameRoom < 0 || s.names.full() {
		s.stop(core.Says("archivescan.StoppedNames", "its names take more room than this tool sets aside to compare them, and no more are read"))
		return false
	}
	s.entries++
	if leaves(name) {
		s.note(noteLeaves, name)
	}
	if unwritable(name) {
		s.note(noteUnwritable, name)
	}
	if first, ok := s.names.add(name, folder); ok {
		s.noteSaid(noteCollides, name, core.Says("archivescan.Same", "%s = %s", core.A("Name", core.Shown(name)), core.A("First", core.Shown(first))))
	}
	return true
}

// weigh is the size of what an archive unpacks to, against the archive and
// against the limit given.
func (s *scan) weigh() {
	if !s.exploded && s.unpacked >= BombBytes && s.unpacked > uint64(s.size)*BombRatio {
		s.exploded = true
		s.noteSaid(noteBomb, "", core.Says("archivescan.Bomb", "%s from %s", core.A("Unpacked", bytesOf(s.unpacked)), core.A("Size", core.Bytes(s.size))))
	}
	if s.limit >= 0 && s.unpacked > uint64(s.limit) {
		s.noteSaid(noteOverLimit, "", core.Says("archivescan.OverLimit", "%s against a limit of %s", core.A("Unpacked", bytesOf(s.unpacked)), core.A("Limit", core.Bytes(s.limit))))
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

// result is what the scan found, as the registry hands it to both surfaces.
func (s *scan) result() tool.Result {
	report := Report{
		File: s.path, Kind: string(s.kind), Bytes: s.size, Entries: s.entries,
		Unpacked: s.unpacked, Safe: true, Found: map[string][]string{},
	}
	summary := map[string]tool.Noted{
		noteKind:     {ID: noteKind, Items: []string{string(s.kind)}},
		noteEntries:  {ID: noteEntries, Items: []string{core.Exactly(int64(s.entries))}},
		notePacked:   sized(notePacked, uint64(s.size)),
		noteUnpacked: sized(noteUnpacked, s.unpacked),
	}
	r := tool.Result{Data: &report, Concluded: concludedSafe}
	for _, d := range declared {
		if n, ok := summary[d.ID]; ok {
			r.Notes = append(r.Notes, n)
			continue
		}
		list := s.found[d.ID]
		if len(list) == 0 {
			continue
		}
		r.Notes = append(r.Notes, noted(d.ID, list))
		report.Found[d.ID] = namesOf(list)
		if !d.harmless {
			report.Safe = false
			r.Concluded = concludedUnsafe
		}
	}
	return r
}

// namesOf is the entries a list of findings is about, for --json: a finding
// about the archive as a whole names none.
func namesOf(list []found) []string {
	names := make([]string, 0, len(list))
	for _, f := range list {
		if f.name != "" {
			names = append(names, f.name)
		}
	}
	return names
}

// special notes an entry that is not a plain file or folder, and why.
func (s *scan) special(name string, why core.Said) {
	s.noteSaid(noteSpecial, name, entryWhy(name, why))
}

// entryWhy is an entry and what is wrong with it, as one item of a note.
func entryWhy(name string, why core.Said) core.Said {
	return core.Says("archivescan.EntryWhy", "%s: %s", core.A("Name", core.Shown(name)), core.A("Why", why))
}

// sized is a note of one size, said the way a person reads it with the exact
// count beside it. Below a kilobyte the two are one number, said once.
func sized(id string, n uint64) tool.Noted {
	words := core.Says("archivescan.Size", "%s (%s)", core.A("Size", bytesOf(n)), core.A("Exactly", core.ExactBytes(int64(bytesOf(n)))))
	if n < 1024 {
		words = core.Says("archivescan.Bytes", "%s", core.A("Size", bytesOf(n)))
	}
	return tool.Noted{ID: id, Items: []string{words.String()}, Words: []core.Said{words}}
}

// noted is a note as the registry carries it: each finding as data, and in
// words where any of them has words - then every one, since a window shows
// the words of all or of none.
func noted(id string, list []found) tool.Noted {
	n := tool.Noted{ID: id}
	worded := false
	for _, f := range list {
		worded = worded || !f.words.IsZero()
	}
	for _, f := range list {
		words := f.words
		if words.IsZero() {
			words = core.Says("archivescan.Name", "%s", core.A("Name", core.Shown(f.name)))
		}
		if worded {
			n.Items = append(n.Items, words.String())
			n.Words = append(n.Words, words)
			continue
		}
		n.Items = append(n.Items, f.name)
	}
	return n
}
