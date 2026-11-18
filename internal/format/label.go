package format

import (
	"fmt"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// The self describing label ends the folder full of test1.txt, test2.txt and
// test_final_v2.txt. On a screenshot attached to a bug report you can see at
// once which file it is and how to reproduce it.
//
// It stays plain ASCII on purpose. Text formats reach into encodings such as
// Windows-1252 and Shift-JIS where a typographic separator has no
// representation, and a label that fails to encode is worse than a plain one.
// The damage it names is ASCII for the same reason (damage.Chain.Label).
//
// A method of the request rather than a function a generator hands numbers
// to, because the request is what knows whether the file will be damaged and
// what size it keeps on disk. A generator asking anything else could only
// state the size it was asked to write, which for a cut file is the size of
// the whole file before the cut - the label of a truncated text then named a
// size nobody asked for and no damage at all (O333).

// LabelText is the self describing label of the file r asks for. Every format
// that carries its label as text asks this, so the wording cannot drift between
// them. A picture asks PictureLabel.
func (r Request) LabelText(formatID string) string {
	return r.LabelSized(formatID, r.Bytes)
}

// PictureLabel is the label drawn into a picture: into its pixels, into the
// frames of a film, into the text of an SVG drawing. It names the format, the
// damage and the seed, and not the size.
//
// Because a picture is then the same picture whatever size of file it is
// asked for, and only the padding around it moves. Its smallest file is one
// number, so the minimum a refusal names is a size the next run takes. With the
// byte count drawn in, a compressed picture changed its length with the digits:
// JPG 640x480 refused 1 B naming 41549 B, then refused 41549 B naming 42484 B,
// measured on 2026-11-16 (docs/SIZE-FIXES-2026-11-16.md section 2). The size
// stays in the manifest and in the file's own length.
func (r Request) PictureLabel(formatID string) string {
	if r.Damage == "" {
		return fmt.Sprintf("tfg - %s - seed %s", formatID, core.SeedLabel(r.Seed))
	}
	return fmt.Sprintf("tfg - %s - damage %s - seed %s", formatID, r.Damage, core.SeedLabel(r.Seed))
}

// LabelSized is the label of a file of size bytes, for a generator that works
// its size out from its contents and so states a size the request does not
// hold yet - zip, eml and targz settle the label and the size against each
// other.
//
// A damaged file states its size on disk wherever the request knows it, which
// is wherever a damage changes the length, and the damage before the seed. A
// file nothing damages carries the label it always carried, to the byte (D11).
func (r Request) LabelSized(formatID string, size int64) string {
	if r.OnDisk > 0 {
		size = r.OnDisk
	}
	if r.Damage == "" {
		return fmt.Sprintf("tfg - %s - %d B - seed %s", formatID, size, core.SeedLabel(r.Seed))
	}
	return fmt.Sprintf("tfg - %s - %d B - damage %s - seed %s", formatID, size, r.Damage, core.SeedLabel(r.Seed))
}
