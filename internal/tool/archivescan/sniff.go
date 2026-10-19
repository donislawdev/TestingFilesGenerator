package archivescan

import "bytes"

// Telling what a file is from the bytes it begins with, never from its name -
// a name is the first thing that lies about a file somebody was sent.

// kind is an archive this tool reads.
type kind string

const (
	kindZip   kind = "zip"
	kindTar   kind = "tar"
	kindTarGz kind = "tar.gz"
	kindGzip  kind = "gzip"
)

// headLength is how much of a file is read to tell what it is: a TAR header
// is one block of 512 bytes with its magic at 257.
const headLength = 512

var (
	zipLocal  = []byte("PK\x03\x04")
	zipEmpty  = []byte("PK\x05\x06")
	gzipMagic = []byte{0x1f, 0x8b}
	tarMagic  = []byte("ustar")
)

// others are archives recognised and not read, by the bytes they begin with -
// named in the refusal, so a person learns what they gave rather than that it
// is nothing.
var others = []struct {
	name  string
	magic []byte
}{
	{"7z", []byte{'7', 'z', 0xBC, 0xAF, 0x27, 0x1C}},
	{"RAR", []byte("Rar!\x1a\x07")},
	{"xz", []byte{0xFD, '7', 'z', 'X', 'Z', 0x00}},
	{"bzip2", []byte("BZh")},
	{"zstd", []byte{0x28, 0xB5, 0x2F, 0xFD}},
	{"CAB", []byte("MSCF")},
}

// sniff is the kind of an archive from its first bytes, or the name of one
// this tool does not read, or neither.
func sniff(head []byte) (kind, string) {
	switch {
	case bytes.HasPrefix(head, zipLocal), bytes.HasPrefix(head, zipEmpty):
		return kindZip, ""
	case bytes.HasPrefix(head, gzipMagic):
		return kindGzip, ""
	case isTar(head):
		return kindTar, ""
	}
	for _, o := range others {
		if bytes.HasPrefix(head, o.magic) {
			return "", o.name
		}
	}
	return "", ""
}

// isTar says whether a block is a TAR header, or the block of zeros an empty
// TAR is. POSIX and GNU both write "ustar" at 257. The old V7 layout has no
// magic at all and is not told apart from any other file.
func isTar(head []byte) bool {
	if len(head) < headLength {
		return false
	}
	return bytes.Equal(head[257:257+len(tarMagic)], tarMagic) || allZero(head[:headLength])
}

// archiveIn names the archive a piece of contents begins as, or nothing - for
// an entry holding another archive, which is reported and never opened.
func archiveIn(head []byte) string {
	if k, other := sniff(head); other != "" {
		return other
	} else if k != "" && !(k == kindTar && allZero(head)) {
		return string(k)
	}
	return ""
}

// allZero says whether every byte is zero.
func allZero(b []byte) bool {
	for _, c := range b {
		if c != 0 {
			return false
		}
	}
	return true
}
