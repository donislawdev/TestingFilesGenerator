package core

import "fmt"

// The self describing label ends the folder full of test1.txt, test2.txt and
// test_final_v2.txt. On a screenshot attached to a bug report you can see at
// once which file it is and how to reproduce it.
//
// It stays plain ASCII on purpose. Text formats reach into encodings such as
// Windows-1252 and Shift-JIS where a typographic separator has no
// representation, and a label that fails to encode is worse than a plain one.

// Label composes the self describing label for one file. Every format that
// carries a label uses this one, so the wording cannot drift between them.
func Label(formatID string, bytes int64, seed uint64) string {
	return fmt.Sprintf("tfg - %s - %d B - seed %s", formatID, bytes, SeedLabel(seed))
}
