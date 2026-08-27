package core

import (
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// FoldName is the one spelling two file names are compared under when the
// question is whether a filesystem would treat them as the same file.
//
// It lives here rather than beside either caller because two layers ask it and
// they are not allowed to see each other. Planning asks it to refuse a recipe
// whose names would collide on somebody else's machine. Verify asks it to tell
// a file that is genuinely somebody else's apart from the same file written a
// different way - and until 2026-08-27 verify had no way to ask, which is how
// two commands came to give opposite answers about one file: verify called
// REPORT.TXT extra while cleanup deleted it, on the same directory and the same
// manifest. A rule with two copies drifts. A rule with one place cannot.
//
// Which spelling to fold under was measured rather than reasoned about, and the
// measurement lives in tools/probes/casefold: against nine pairs that real
// filesystems do and do not join, cases.Fold is right about 8 of 8 and
// strings.ToLower about 4. Lowercasing lets through three collisions APFS
// really makes - the sharp s, the long s and the ff ligature - and invents one
// no measured filesystem makes, because Go maps a dotted capital I onto a plain
// i in a single rune.
//
// NFC first, because a name can carry the same letters in two encodings and a
// filesystem that joins them would otherwise be invisible here.
//
// What this is NOT: an answer about what the host will do. A host has already
// given its answer by storing the name it stored, and reading that back is
// os.Stat's job. This answers the portable question - could these two ever be
// one file - which has to hold for every machine at once.
func FoldName(name string) string {
	return cases.Fold().String(norm.NFC.String(name))
}
