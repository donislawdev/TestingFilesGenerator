package zip

import (
	stdzip "archive/zip"
	"context"
	"fmt"
	"io/fs"

	"github.com/donislawdev/TestingFilesGenerator/internal/format/archive"
)

// writeDirectories names the directories the archive was asked to list.
//
// Outermost first, because that is the order a reader can act on: one that
// creates a directory when it meets it cannot make d00/d01 before it has made
// d00. Layout.Directories gives them that way and gives nothing at all when
// the archive was not asked to name them, so there is no flag to read here.
//
// These are NOT entries in m.children, and that is the load bearing part.
// A child's seed is core.FileSeed(seed, index) over a running index, so a
// directory sitting in that list would shift the seed of every file after it
// and rewrite the contents of all of them - untouchable rule 2, where an edit
// in one place moves the bytes in another. Nothing here consumes an index.
//
// archiveSize still counts them, because it counts by running build against a
// counting writer rather than by arithmetic. So the size stays exact without
// anybody adding a term for this.
func writeDirectories(ctx context.Context, zw *stdzip.Writer, layout archive.Layout) error {
	for _, dir := range layout.Directories() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := writeDirectory(zw, dir); err != nil {
			return err
		}
	}
	return nil
}

// writeDirectory names one directory in the archive.
//
// It does not go through openEntry, and that is not a shortcut. openEntry is
// where locking happens, and locking is about content: a directory has none,
// so running it through the encrypting path would ask for a checksum and a
// header over nothing. Every reader takes a stored, empty, slash terminated
// entry as a directory.
func writeDirectory(zw *stdzip.Writer, name string) error {
	h := &stdzip.FileHeader{Name: name, Method: stdzip.Store}
	h.SetMode(fs.ModeDir | 0o755)
	if _, err := zw.CreateHeader(h); err != nil {
		return fmt.Errorf("zip: the directory %q could not be named: %w", name, err)
	}
	return nil
}
