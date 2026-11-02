package manifest

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// How a manifest becomes JSON, an entry at a time.
//
// Its own file since 2026-11-02, when writing a manifest stopped being one call
// into encoding/json and manifest.go was already past the size a person can
// follow.

// encodeBuffer is how much of the document waits in memory before it is
// handed to the file. Large enough that a write is never a few bytes, small
// against any document worth streaming.
const encodeBuffer = 64 << 10

// filesList is the list of files in a document with none, as encoding/json
// writes it one level in. A key one level in sits on its own line after two
// spaces, and a string cannot hold a line break, so in a document with no
// entries this is found once and only where the list is.
var filesList = []byte("\n  \"files\": []")

// Encode renders the manifest as JSON.
//
// The bytes are the ones json.NewEncoder writes with two spaces of indent, and
// a guard holds them to that byte for byte. What changed on 2026-11-02 is when
// they exist. encoding/json builds the whole document in memory before its
// first write, and indents it into a second copy, while every entry is still
// held. Measured with the runsize probe at 100 000 text files, saving the
// manifest peaked at 9.3 KB a file against 4.0 KB for writing every file -
// about ten times the manifest's own length - and a run of 200 000 text files
// in a container of 1 GB was killed for it after its last file, leaving the
// files and no manifest to clean them up with (docs/FILE-CEILING-2026-11-02.md
// section 8). Now the document is written the way size.go counts it: the
// document with no entries, cut where its list is, and the entries in between,
// one at a time.
//
// A manifest with no entries is written whole, as it always was. It is small,
// and a list that was never made encodes as null rather than [] - written
// whole, it says exactly what it said.
func (m *Manifest) Encode(w io.Writer) error {
	if len(m.Files) == 0 {
		return encodeWhole(w, m)
	}
	head, tail, err := m.shell()
	if err != nil {
		return err
	}
	out := bufio.NewWriterSize(w, encodeBuffer)
	if err := put(out, head, []byte("\n")); err != nil {
		return err
	}
	for i := range m.Files {
		// Through the slice rather than a copy, so the entry is encoded as
		// the whole document encodes it: addressable, the way a slice
		// element is.
		entry, err := json.MarshalIndent(&m.Files[i], entryIndent, "  ")
		if err != nil {
			return err
		}
		between := []byte(",\n")
		if i == 0 {
			between = nil
		}
		if err := put(out, between, []byte(entryIndent), entry); err != nil {
			return err
		}
	}
	if err := put(out, []byte("\n  "), tail); err != nil {
		return err
	}
	return out.Flush()
}

// put writes the parts in order and stops at the first that fails. A file
// that refused a write is not asked to take the rest of a document of a
// million entries.
func put(out *bufio.Writer, parts ...[]byte) error {
	for _, p := range parts {
		if _, err := out.Write(p); err != nil {
			return err
		}
	}
	return nil
}

// shell is this document with no entries, cut where its list of files is:
// head ends with the list's opening bracket and tail starts with its closing
// one.
func (m *Manifest) shell() (head, tail []byte, err error) {
	empty := *m
	empty.Files = []File{}
	var whole bytes.Buffer
	if err := encodeWhole(&whole, &empty); err != nil {
		return nil, nil, err
	}
	doc := whole.Bytes()
	if found := bytes.Count(doc, filesList); found != 1 {
		return nil, nil, core.Defect(fmt.Errorf("a manifest with no entries holds %d lists of files one level in, not one", found))
	}
	cut := bytes.Index(doc, filesList) + len(filesList) - len("]")
	return doc[:cut], doc[cut:], nil
}

// encodeWhole is encoding/json's rendering in one piece - the layout the
// streaming above reproduces, and the one a guard compares it with.
func encodeWhole(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
