package archivescan

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/tool"
)

// What a scan found, as the registry hands it to both surfaces and as --json
// prints it. A report is bounded however much an archive holds: a note lists
// MostListed findings and counts the rest, a name is shown up to
// MostShownName bytes, and every name a report keeps, in the archive given and
// every one inside it, adds up to MostListedBytes at most.

// result is what the scan found, as the registry hands it to both surfaces.
func (s *scan) result() tool.Result {
	report := s.report()
	r := tool.Result{Data: report, Concluded: concludedSafe}
	if !report.Safe {
		r.Concluded = concludedUnsafe
	}
	summary := map[string]tool.Noted{
		noteKind:     {ID: noteKind, Items: []string{string(s.kind)}},
		noteEntries:  {ID: noteEntries, Items: []string{core.Exactly(int64(s.entries))}},
		notePacked:   sized(notePacked, uint64(s.size)),
		noteUnpacked: sized(noteUnpacked, s.unpacked),
	}
	if len(s.found.own[noteNested])+s.found.more[noteNested] > 0 {
		summary[noteUnpackedAll] = sized(noteUnpackedAll, report.UnpackedAll)
	}
	for _, d := range declared {
		if n, ok := summary[d.ID]; ok {
			r.Notes = append(r.Notes, n)
			continue
		}
		list := slices.Concat(s.found.own[d.ID], s.found.deep[d.ID])
		more := s.found.more[d.ID] + s.found.deepMore[d.ID]
		if len(list) > 0 || more > 0 {
			r.Notes = append(r.Notes, noted(d.ID, list, more))
		}
	}
	return r
}

// report is what --json says of this archive and of the archives inside it
// that are not safe.
func (s *scan) report() *Report {
	r := &Report{
		File: s.path, Kind: string(s.kind), Bytes: s.size, Entries: s.entries, Unpacked: s.unpacked,
		UnpackedAll: addCapped(s.unpacked, s.nest.unpacked), Safe: s.safe(),
		Found: map[string][]string{}, More: map[string]int{}, InsideMore: s.nest.more,
	}
	for _, d := range declared {
		if len(s.found.own[d.ID])+s.found.more[d.ID] > 0 {
			r.Found[d.ID] = namesOf(s.found.own[d.ID])
		}
		if s.found.more[d.ID] > 0 {
			r.More[d.ID] = s.found.more[d.ID]
		}
	}
	if len(r.More) == 0 {
		r.More = nil
	}
	for _, c := range s.nest.kept {
		r.Inside = append(r.Inside, c.report())
	}
	return r
}

// safe says whether nothing in this archive, nor in any archive inside it,
// makes unpacking it unsafe.
func (s *scan) safe() bool {
	if s.nest.unsafe {
		return false
	}
	for _, d := range declared {
		if !d.harmless && len(s.found.own[d.ID])+s.found.more[d.ID] > 0 {
			return false
		}
	}
	return true
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
// the words of all or of none - and how many more were found than it lists.
func noted(id string, list []found, more int) tool.Noted {
	n := tool.Noted{ID: id}
	worded := more > 0
	for _, f := range list {
		worded = worded || !f.words.IsZero()
	}
	for _, f := range list {
		words := f.words
		if words.IsZero() {
			words = core.Says("archivescan.Name", "%s", core.A("Name", shownName(f.name)))
		}
		if worded {
			n.Items = append(n.Items, words.String())
			n.Words = append(n.Words, words)
			continue
		}
		n.Items = append(n.Items, f.name)
	}
	if more > 0 {
		words := core.Says("archivescan.More", "and %s more", core.A("More", core.Exactly(int64(more))))
		n.Items = append(n.Items, words.String())
		n.Words = append(n.Words, words)
	}
	return n
}

// add keeps a finding under its key, or counts it under the same key in more
// when the report has no room left to list it.
func (t *tree) add(lists map[string][]found, more map[string]int, id string, f found) {
	list, kept := t.keep(lists[id], f)
	lists[id] = list
	if !kept {
		more[id]++
	}
}

// keep adds a finding to a list a report shows, while the list and the report
// have room for it, and says whether they had. What it keeps of a name is a
// copy of its first MostShownName bytes, never the name itself: a name can be
// a slice of something far larger - archive/tar hands back a name set in a
// PAX header as a slice of the whole header, up to a megabyte.
func (t *tree) keep(list []found, f found) ([]found, bool) {
	if len(list) >= MostListed || t.listed >= MostListedBytes {
		return list, false
	}
	if len(f.name) > MostShownName && f.words.IsZero() {
		f.words = core.Says("archivescan.Name", "%s", core.A("Name", shownName(f.name)))
	}
	f.name = cutName(f.name)
	t.listed += len(f.name) + len(f.words.String())
	return append(list, f), true
}

// shownName is a name as a sentence shows it: escaped, and past MostShownName
// bytes cut, with how long it is. Nothing in an archive is named "" - that is
// the one file a gzip file unpacks to.
func shownName(name string) any {
	switch {
	case name == "":
		return core.Says("archivescan.TheFileInside", "the file it unpacks to")
	case len(name) <= MostShownName:
		return core.Shown(strings.Clone(name))
	}
	return core.Says("archivescan.Cut", "%s... (%s in all)",
		core.A("Name", core.Shown(cutName(name))), core.A("Length", core.ExactBytes(int64(len(name)))))
}

// cutName is a copy of the first MostShownName bytes of a name, or of all of
// it when it is shorter, ending on a whole character: a cut through one
// leaves half of it, which --json writes as a character the name does not
// have. A name that is not UTF-8 has no characters to keep whole, so the cut
// goes back no further than one character is long.
func cutName(name string) string {
	n := min(len(name), MostShownName)
	for back := 1; back < utf8.UTFMax && n < len(name) && !utf8.RuneStart(name[n]); back++ {
		n--
	}
	return strings.Clone(name[:n])
}
