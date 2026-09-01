package guard

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/archive"
)

// The deepest path this build can produce still has to fit a USTAR header.
//
// This guard was written BEFORE depth existed, because the behaviour it holds
// is the one the change could break silently. targz pins tar.FormatUSTAR, and
// USTAR carries a path in a 155 byte prefix and a 100 byte name split ON A
// SLASH - so whether a path fits is a question about where its slashes fall,
// not about its length. Go answers it by refusing to write the header, and a
// refusal at write time is the worst place to find out: the size has already
// been planned and promised.
//
// Measured 2026-09-01: with four byte segments the last usable slash sits at
// 155, so a path has to come to 256 bytes or fewer. Depth 61 with a 12 byte
// name is taken at 256 and depth 62 is refused at 260. That makes the ceiling
// depend on the ENTRY NAME, which is not a constant - the longest this build
// can make is targz_0001.tar.gz at 17 bytes, which puts the real limit at 59
// rather than 61.
//
// So the declaration cannot be checked against a number somebody wrote down.
// It is checked against every registered format, at the deepest nesting the
// build offers, by asking tar itself. A format added tomorrow with a longer id
// or extension reddens this without a line changing here.
func TestTheDeepestPathEveryFormatCanMakeStillFitsAUstarHeader(t *testing.T) {
	depth := archive.MaxDepth()
	layout := archive.Layout{Depth: depth}

	checked := 0
	for _, d := range format.All() {
		// 9999 rather than 0001: the counter is four digits wide, so the
		// longest name a format can produce is the one with the largest
		// number in it. They are the same width today and would stop being so
		// the day the counter grows.
		name := fmt.Sprintf("%s_%04d%s", d.ID, 9999, d.Extension)
		path := layout.Path(name)

		if got, want := len(path), archive.LongestPath(depth, len(name)); got != want {
			t.Errorf("%s: the path is %d bytes and LongestPath says %d - "+
				"the guard and the thing it guards disagree about the arithmetic",
				d.ID, got, want)
		}

		if err := ustarAccepts(path); err != nil {
			t.Errorf("%s: the deepest path this build can make cannot be written:\n"+
				"  path  %q (%d bytes)\n"+
				"  tar   %v\n"+
				"  Lower maxDepth in internal/format/archive/layout.go, or shorten the segment.",
				d.ID, path, len(path), err)
		}
		checked++
	}

	if checked == 0 {
		t.Fatal("no formats were checked, so this proved nothing about any of them")
	}
}

// And the one below it has to be refused, or the ceiling is decoration.
//
// A ceiling nobody can reach on purpose is a ceiling nobody has watched work,
// and this project has thrown away several pieces of defensive code for that
// reason. If tar took every depth, maxDepth would be holding back nothing and
// the guard above would pass whatever it said.
func TestAPathOneStepPastTheCeilingIsRefusedByTar(t *testing.T) {
	// The longest name the registry holds, so this asks about the tightest
	// format rather than a comfortable one.
	longest := ""
	for _, d := range format.All() {
		if name := fmt.Sprintf("%s_%04d%s", d.ID, 9999, d.Extension); len(name) > len(longest) {
			longest = name
		}
	}

	// Walk out until tar says no. It has to say no somewhere, and it has to
	// say no ABOVE the depth we offer rather than at it.
	refusedAt := 0
	for depth := archive.MaxDepth(); depth <= archive.MaxDepth()+200; depth++ {
		if err := ustarAccepts((archive.Layout{Depth: depth}).Path(longest)); err != nil {
			refusedAt = depth
			break
		}
	}

	if refusedAt == 0 {
		t.Fatalf("tar took every depth from %d to %d for %q, so nothing here is a ceiling",
			archive.MaxDepth(), archive.MaxDepth()+200, longest)
	}
	if refusedAt <= archive.MaxDepth() {
		t.Fatalf("tar refuses depth %d for %q and this build offers %d - "+
			"the declared ceiling is already past what tar takes",
			refusedAt, longest, archive.MaxDepth())
	}
}

// Asking for directory entries in a flat archive is a refusal naming both
// halves, not a setting that quietly does nothing.
//
// Rule 6 forbids the silence: with depth 0 there are no directories, so the
// archive would come out identical whichever way the box was ticked, and the
// person who ticked it would never learn that. The message has to name both
// keys because from "directory_entries is not allowed" a reader cannot tell
// which of the two to change - and the window makes this pair easy to reach,
// since a checkbox always sends its value.
func TestAskingForDirectoryEntriesInAFlatArchiveNamesBothSettings(t *testing.T) {
	_, err := archive.ReadLayout("zip", format.Request{
		Properties: map[string]string{archive.DirectoryEntries: "true"},
	})
	if err == nil {
		t.Fatal("directory_entries with depth 0 was accepted, so the setting did nothing and said nothing")
	}

	var refusal *format.PropertyValueError
	if !errors.As(err, &refusal) {
		t.Fatalf("the refusal is %T, so it does not carry the four things a refusal owes a reader", err)
	}
	said := refusal.Reason + " " + refusal.Remedy
	for _, half := range []string{archive.Depth, archive.DirectoryEntries} {
		if !strings.Contains(said, half) {
			t.Errorf("the refusal never names %q, so a reader cannot tell which half to change:\n  %s",
				half, said)
		}
	}
}

// Flat stays flat, and that is what keeps every existing hash where it is.
//
// The default has to be the layout every archive this tool has written so far
// already had. Anything else moves the bytes of all of them, which is
// untouchable rule 3 - and it would do it without a single recipe changing.
func TestAnArchiveNobodyAskedToNestIsStillFlat(t *testing.T) {
	l, err := archive.ReadLayout("zip", format.Request{})
	if err != nil {
		t.Fatalf("an archive with nothing asked for was refused: %v", err)
	}
	if l.Depth != 0 {
		t.Errorf("depth defaults to %d, so every archive already written moves", l.Depth)
	}
	if got := l.Path("txt_0001.txt"); got != "txt_0001.txt" {
		t.Errorf("a flat archive puts its entry at %q rather than at the top", got)
	}
	if dirs := l.Directories(); len(dirs) != 0 {
		t.Errorf("a flat archive names %d directories: %v", len(dirs), dirs)
	}
}

// The directories come outermost first, because that is the only order a
// reader can act on.
//
// A reader that creates directories as it meets them cannot make d00/d01
// before it has made d00. Sorting them the other way produces an archive that
// is correct on paper and fails in a real extractor, which is the kind of
// defect no size check would ever notice.
func TestTheDirectoriesAnArchiveNamesComeOutermostFirst(t *testing.T) {
	l, err := archive.ReadLayout("zip", format.Request{Properties: map[string]string{
		archive.Depth: "3", archive.DirectoryEntries: "true",
	}})
	if err != nil {
		t.Fatalf("a nested archive with directory entries was refused: %v", err)
	}

	dirs := l.Directories()
	if len(dirs) != 3 {
		t.Fatalf("depth 3 names %d directories, want 3: %v", len(dirs), dirs)
	}
	for i, dir := range dirs {
		if !strings.HasSuffix(dir, "/") {
			t.Errorf("%q does not end in a slash, so neither container reads it as a directory", dir)
		}
		if i > 0 && !strings.HasPrefix(dir, dirs[i-1]) {
			t.Errorf("%q does not sit inside %q, so the chain is not a chain", dir, dirs[i-1])
		}
	}
	if want := l.Path(""); dirs[len(dirs)-1] != want {
		t.Errorf("the innermost directory is %q and the files live in %q", dirs[len(dirs)-1], want)
	}
}

// ustarAccepts asks tar itself rather than reimplementing the split rule.
//
// The rule is "some slash with at most 155 bytes before it and at most 100
// after", and writing that out here would be a second implementation to keep
// in step with the standard library - the exact shape O163 punished, where a
// channel measured against five readers said nothing about what Go does.
func ustarAccepts(path string) error {
	w := tar.NewWriter(io.Discard)
	err := w.WriteHeader(&tar.Header{
		Name: path, Size: 0, Mode: 0o644, Format: tar.FormatUSTAR,
	})
	if err != nil {
		return err
	}
	return w.Close()
}

// A directory entry costs one block and nothing else, which is the whole of
// what the tar arithmetic has to learn.
//
// size.go computes the tar length as 1024 plus, per entry, 512 plus the
// content rounded up to a block. A directory has no content, so it should add
// exactly one block - measured here rather than assumed, because the whole
// exact-size promise rests on that number being right.
func TestADirectoryEntryCostsExactlyOneTarBlock(t *testing.T) {
	measure := func(h *tar.Header) int {
		var buf bytes.Buffer
		w := tar.NewWriter(&buf)
		if err := w.WriteHeader(h); err != nil {
			t.Fatalf("writing %q: %v", h.Name, err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("closing after %q: %v", h.Name, err)
		}
		return buf.Len()
	}

	empty := func() int {
		var buf bytes.Buffer
		w := tar.NewWriter(&buf)
		if err := w.Close(); err != nil {
			t.Fatalf("closing an empty tar: %v", err)
		}
		return buf.Len()
	}()

	withDir := measure(&tar.Header{
		Name: "d00/", Mode: 0o755, Typeflag: tar.TypeDir, Format: tar.FormatUSTAR,
	})

	if got, want := withDir-empty, 512; got != want {
		t.Errorf("a directory entry adds %d bytes, and the size arithmetic assumes %d", got, want)
	}
}
