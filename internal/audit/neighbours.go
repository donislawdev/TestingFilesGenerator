// Part of package audit. See audit.go.
package audit

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// What the OTHER runs recorded in a directory say they wrote.
//
// A directory is allowed to hold more than one run. output.manifest exists for
// exactly that, so that a second run records itself beside the first instead of
// being refused, and people use it - a set of fixtures per test suite, one
// directory. What verify did with it was call every one of the neighbour's
// files "extra", which is the word for a file nobody asked for, and the report
// then read as a directory somebody had polluted.
//
// Measured on 2026-09-07, two runs one after another into one directory with
// name templates that do not collide, both ending 0:
//
//	verify manifest-alpha.json   3 differences, exit 7
//	verify manifest-beta.json    4 differences, exit 7
//
// Every one of those differences was the other run's work, and which run had
// written it was recorded in the same directory the whole time.
//
// The rule this follows is ATTRIBUTION, NOT SUPPRESSION, and the difference
// matters more than the repair. Every file stays in the report. What changes is
// the word it is given and whether it makes the directory a mismatch. So
// untouchable rule 6 is kept literally rather than on trust, and a manifest
// somebody drops into a directory cannot hide a file - at most it can claim
// one, out loud, with its own name printed beside it.

// neighbourClaims is what the other runs in this directory account for.
type neighbourClaims struct {
	// records is the path of every file that is itself another run's manifest.
	records map[string]bool
	// claimedBy maps a file to the base name of the record listing it, which
	// is what a reader needs in order to know which run to ask.
	claimedBy map[string]string
}

// findNeighbours reads the manifests of other runs sitting in this directory.
//
// candidates are the files this manifest does not claim, which is the only set
// worth looking at: anything our own manifest lists has already been compared
// against it, and a neighbour listing it too cannot change that answer.
//
// A candidate that cannot be read, or that is not a manifest, or that is a
// manifest this build refuses, is simply not a neighbour. Nothing is reported
// about it here and it keeps whatever verify would have called it. Guessing on
// a file we could not read is how a tool starts accounting for files nobody
// wrote.
func findNeighbours(ctx context.Context, dir string, candidates []string) neighbourClaims {
	found := neighbourClaims{
		records:   map[string]bool{},
		claimedBy: map[string]string{},
	}
	for _, rel := range candidates {
		if ctx.Err() != nil {
			// A cancelled pass stops looking. The caller reports what it
			// compared and calls nothing sound.
			return found
		}
		if !couldBeNamedLikeARecord(rel) {
			continue
		}
		full := filepath.Join(dir, filepath.FromSlash(rel))
		m, err := manifest.Load(full)
		if err != nil {
			continue
		}
		found.records[rel] = true
		base := filepath.Base(rel)
		found.take(m, base)
	}
	return found
}

// take records what one neighbour says it wrote.
//
// Its own function rather than a loop inside a loop, which is the ceiling on
// how deeply this project nests talking. It is also the better shape to read:
// the caller decides WHICH files are records, and this decides what a record
// accounts for.
func (n neighbourClaims) take(m *manifest.Manifest, base string) {
	for _, f := range Claimed(m) {
		// First record wins, and the walk that produced the candidates is in a
		// fixed order, so two neighbours claiming one file name the same one of
		// themselves every time rather than whichever was read first on the day.
		//
		// The path is put through the same spelling both sides of the
		// comparison use. Without it a neighbour writing "./a.txt" would claim
		// nothing, which is the fault comparablePath was written for.
		key := comparablePath(f.Path)
		if _, taken := n.claimedBy[key]; !taken {
			n.claimedBy[key] = base
		}
	}
}

// couldBeNamedLikeARecord is the sieve that costs nothing, and it had to exist.
//
// Reading the first bytes of every file the manifest does not claim was the
// first design and it was measured out of existence on 2026-09-07: on a
// directory holding ten thousand unclaimed files it made verify several times
// slower, because opening a file on Windows is not free - a scanner sees every
// one of them. The exact factor is refused, because the canary runs of the
// unchanged binary disagreed with each other by a factor of seven while it was
// taken. What is not in doubt is that it was large enough to change the design.
//
// So a file is only opened when its name could be a record at all. This is a
// NARROWING rather than a guess about the world, and the difference is what
// makes it safe: a neighbour's manifest under some other extension is reported
// exactly as it was reported before any of this existed, as extra. The cost of
// the sieve being wrong is yesterday's answer rather than a wrong one.
//
// The extension is the one this tool writes and the one it documents.
// DefaultManifestName is manifest.json, the help for verify says
// "tfg verify <manifest.json>", and every preset and every example writes one.
// Written down in docs/SHARED-DIRECTORY-2026-09-07.md section 2.3 as a limit
// rather than left to be discovered.
//
// It is the only sieve, and a second one was written and taken out again the
// same day. That one read the first half kilobyte of every candidate and looked
// for the first key of a manifest, to keep a large JSON document that is not one
// from being read in full. Nothing could make it fail: a document that got past
// it was refused by manifest.Load's schema check anyway, so removing it changed
// no answer and no test could tell. A defence nothing can redden is not a
// defence, and this project takes those out rather than keeping them.
//
// What is left is the name, and then manifest.Load, which has a ceiling of its
// own - so the worst an unclaimed JSON document can cost is one read of at most
// that ceiling.
func couldBeNamedLikeARecord(rel string) bool {
	return strings.EqualFold(filepath.Ext(rel), ".json")
}
