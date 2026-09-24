package core

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"unicode/utf8"
)

// MaxNameBytes is the longest file name, in bytes of UTF-8, that every system
// this tool writes on will store.
//
// The three file systems count three different things, each up to 255: ext4
// counts bytes, NTFS counts UTF-16 units and APFS counts characters. A name
// never has fewer bytes than either of the other two, so a name that fits in
// 255 bytes fits on all three. Measured on 2026-09-24 with one recipe on each
// (docs/O239-LONG-NAMES-2026-09-24.md): a name of 200 CJK characters, 604
// bytes, was stored on Windows and on macOS and cannot be on Linux.
const MaxNameBytes = 255

// siblingTagDigits is how much of the digest of the whole name a shortened
// sibling carries, in hex digits. Sixty four bits, which puts the chance that
// two names of one run share a sibling at about three in a million million
// for the largest preset there is, 10 040 files.
const siblingTagDigits = 16

// SiblingName is the name of a file that stands beside another one while it is
// being written: the name with suffix after it, whenever that fits.
//
// It exists because the plain join did not always fit. Every file of a run is
// written as "<name>.tfg-partial-<pid>" and renamed afterwards, and that suffix
// is eighteen bytes, so a name from 238 bytes up was one no file system would
// take in its temporary form - on every system, while the name itself was
// perfectly legal (O239, measured on 2026-09-24 on Windows, Linux and macOS).
//
// When the join is longer than MaxNameBytes, the name is cut to whole
// characters and followed by "~" and the start of the SHA-256 of the whole
// name, and then the suffix. The digest is what keeps two long names that
// begin alike apart - a name template numbering files at the end of a long
// name gives exactly that, inside one run. The suffix stays last in both
// forms, so what an interrupted run leaves behind is recognised by
// IsPartialName and IsWritingName either way.
//
// A name short enough comes back as the plain join, byte for byte, so nothing
// changed for any name this tool could write before.
func SiblingName(name, suffix string) string {
	if len(name)+len(suffix) <= MaxNameBytes {
		return name + suffix
	}
	sum := sha256.Sum256([]byte(name))
	tag := "~" + hex.EncodeToString(sum[:])[:siblingTagDigits]
	return cutToWholeCharacters(name, MaxNameBytes-len(suffix)-len(tag)) + tag + suffix
}

// SiblingPath is SiblingName for a path: the sibling in the same directory,
// with the directory spelt exactly as it was given.
func SiblingPath(path, suffix string) string {
	dir, name := filepath.Split(path)
	return dir + SiblingName(name, suffix)
}

// cutToWholeCharacters is the longest start of s that is no longer than n bytes
// and does not end part way through a character.
func cutToWholeCharacters(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}
