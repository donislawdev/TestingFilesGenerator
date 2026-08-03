// Package audit reads a finished run back off the disk.
//
// It is the other half of the engine. The engine writes files and records what
// it wrote. This compares that record against what is actually there, and
// removes what the record covers.
//
// It knows nothing about the command line. The rule about which files a
// manifest claims lives here rather than in the two commands, so "verify" and
// "cleanup" cannot disagree about what the manifest is claiming.
package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// Kind names a way the directory and the manifest disagree.
type Kind string

const (
	// Missing is a file the manifest claims and the directory does not have.
	Missing Kind = "missing"
	// Extra is a file in the directory that the manifest does not claim.
	Extra Kind = "extra"
	// WrongSize is a file of the wrong length. It is reported separately from
	// a wrong hash because the two point at different causes - a truncated
	// transfer against an edited file - and the fix is different.
	WrongSize Kind = "wrong-size"
	// WrongHash is a file of the right length and the wrong content.
	WrongHash Kind = "wrong-hash"
	// Unreadable is a file that is there and could not be read. It is not
	// agreement and it is not a mismatch, and calling it either would be a
	// guess.
	Unreadable Kind = "unreadable"
)

// Difference is one disagreement, in the words a person needs to act on it.
type Difference struct {
	Kind Kind
	Path string
	Want string
	Got  string
}

func (d Difference) String() string {
	switch d.Kind {
	case Missing:
		return fmt.Sprintf("missing   %s", d.Path)
	case Extra:
		return fmt.Sprintf("extra     %s", d.Path)
	case Unreadable:
		return fmt.Sprintf("unreadable %s - %s", d.Path, d.Got)
	default:
		return fmt.Sprintf("%-9s %s\n            expected %s\n            found    %s", d.Kind, d.Path, d.Want, d.Got)
	}
}

// EscapeError is a manifest entry that leaves the directory once the links on
// the way have been followed.
//
// Separate from the check manifest.Load already makes, and it has to be. That
// one reads the text of the path and this one asks the filesystem, so a path
// with no climb in it - "jn/VICTIM.txt" - passes the first and is caught here
// when "jn" turns out to point somewhere else.
type EscapeError struct {
	Dir  string
	Path string
}

func (e *EscapeError) Error() string {
	return fmt.Sprintf(
		"the manifest lists %q, which lands outside %s once the links on the way are followed. "+
			"This tool never reads or removes anything outside the directory it was pointed at, so it will not act on this manifest. "+
			"Check that the directory is the one the run wrote to, and that nothing inside it points elsewhere.",
		e.Path, e.Dir)
}

// resolved turns a manifest entry into the path on disk, refusing one that
// leaves the directory.
//
// Both commands go through this, so neither can be the one that forgets. It is
// the second half of the rule docs/SECURITY.md section 2.4 states: a name from
// somebody else's file is a name, and where it lands is settled after the links
// have been followed rather than by reading the string.
func resolved(dir string, f manifest.File) (string, error) {
	full := filepath.Join(dir, filepath.FromSlash(f.Path))
	if core.EscapesAfterResolving(dir, full) {
		return "", &EscapeError{Dir: dir, Path: f.Path}
	}
	return full, nil
}

// Claimed lists the files a manifest says are on the disk.
//
// Two kinds of entry are deliberately not claimed. An entry marked failed
// describes a file that was never written - reporting it missing would be a
// false alarm about the one thing the manifest was honest about. And an entry
// that was not materialised was never meant to reach the disk at all.
//
// This is the single reading of the manifest. Both commands use it, so neither
// can drift into claiming something the other does not.
func Claimed(m *manifest.Manifest) []manifest.File {
	var out []manifest.File
	for _, f := range m.Files {
		if f.Failed || !f.Materialized {
			continue
		}
		out = append(out, f)
	}
	return out
}

// Verify compares a directory against a manifest.
//
// skip is the name of the manifest file itself. It is never reported as extra:
// the manifest usually sits in the directory it describes, and a tool that
// fails on its own output on the most obvious invocation is not usable.
// Matched on the base name rather than the path, because a restored copy
// carries its own copy of the manifest beside the files.
//
// A run that is cancelled reports what it managed to compare and says so
// through the context error. Reporting "sound" on the strength of half a
// directory is the one answer this must never give.
func Verify(ctx context.Context, dir string, m *manifest.Manifest, skip string) ([]Difference, error) {
	claimed := Claimed(m)

	present, err := walk(dir)
	if err != nil {
		return nil, err
	}

	var diffs []Difference
	seen := make(map[string]bool, len(claimed))

	for _, f := range claimed {
		if err := ctx.Err(); err != nil {
			return diffs, err
		}
		seen[f.Path] = true

		full, err := resolved(dir, f)
		if err != nil {
			return nil, err
		}
		info, statErr := os.Stat(full)
		if statErr != nil {
			diffs = append(diffs, Difference{Kind: Missing, Path: f.Path})
			continue
		}

		// Size first. It is free, it catches the common failure, and it names
		// the cause more precisely than a hash mismatch would.
		if info.Size() != f.Bytes {
			diffs = append(diffs, Difference{
				Kind: WrongSize, Path: f.Path,
				Want: fmt.Sprintf("%d B", f.Bytes),
				Got:  fmt.Sprintf("%d B", info.Size()),
			})
			continue
		}

		sum, hashErr := hashFile(full)
		if hashErr != nil {
			diffs = append(diffs, Difference{Kind: Unreadable, Path: f.Path, Got: hashErr.Error()})
			continue
		}
		if sum != f.Hashes.SHA256 {
			diffs = append(diffs, Difference{
				Kind: WrongHash, Path: f.Path,
				Want: f.Hashes.SHA256,
				Got:  sum,
			})
		}
	}

	for _, p := range present {
		if seen[p] || filepath.Base(p) == skip {
			continue
		}
		diffs = append(diffs, Difference{Kind: Extra, Path: p})
	}

	sort.Slice(diffs, func(i, j int) bool {
		if diffs[i].Path != diffs[j].Path {
			return diffs[i].Path < diffs[j].Path
		}
		return diffs[i].Kind < diffs[j].Kind
	})
	return diffs, ctx.Err()
}

// walk lists every file under dir as a slash separated path relative to it.
//
// Recursive because the manifest carries a path rather than a bare name, and
// a run that groups its output into folders has to verify the same way.
func walk(dir string) ([]string, error) {
	// The root is resolved first, because WalkDir does not follow links and a
	// directory that is itself one would be handed to the callback as a single
	// entry that is not a directory. Found on 2026-08-03 by the guard for
	// generating into a linked directory: verify reported "extra ." and called
	// the whole run a mismatch. People keep fixtures on redirected paths, so
	// this is an ordinary setup rather than a corner.
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}

	var out []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(dir, p)
		if relErr != nil {
			return relErr
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// hashFile streams the file rather than reading it in. A run of this tool
// produces files measured in gigabytes, and verify has to survive its own
// output.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
