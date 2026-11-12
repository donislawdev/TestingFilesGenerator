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
// that carries a label asks this, so the wording cannot drift between them.
func (r Request) LabelText(formatID string) string {
	return r.LabelSized(formatID, r.Bytes)
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
