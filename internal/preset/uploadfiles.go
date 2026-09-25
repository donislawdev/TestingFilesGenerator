package preset

// The groups of files the upload set is made of, one function each, and what
// the instructions say about every one of them. Out of uploadset.go on
// 2026-09-25, when the sentences for the instructions took that file past
// the ceiling - the parameters and how they are settled stay there.

import (
	"fmt"
	"strconv"
	"strings"
)

// farOverFiles is the one file well past the limit, or none when it was turned
// off.
func (s uploadSet) farOverFiles() []setFile {
	if s.farOver == 0 {
		return nil
	}
	times := strconv.FormatInt(s.farOver, 10) + "x"
	desc := s.allowed[0]
	return []setFile{{
		id: "far_over_" + times, group: sizeGroup, desc: desc,
		name:     s.limitText + "_far_over_" + times + desc.Extension,
		size:     s.limit * s.farOver,
		expected: "reject", reason: "size_limit",
		purpose: fmt.Sprintf("A %s file %d times the limit of %s. Your form should turn it away before it has read the whole of it - "+
			"one that reads the body into memory first shows it here.", desc.Name, s.farOver, s.limitText),
	}}
}

// degenerateFiles is the upload of nothing at all.
//
// Written by the filler rather than by an allowed format, because a file of
// nought bytes is bytes of no kind and most formats have no such thing. The
// name still carries an allowed extension, which is what makes it the case a
// form really meets: somebody pressed upload on an empty file.
func (s uploadSet) degenerateFiles() []setFile {
	return []setFile{{
		id: "empty", group: degenerateGroup, desc: s.filler,
		name: "empty" + s.allowed[0].Extension, size: 0,
		// Legal, and what a form should do with it is its own decision -
		// storage keeps it, an upload form usually turns it away, and both are
		// defensible. MF5.
		expected: "unspecified", reason: "size_zero",
		purpose: "An empty file, nought bytes, under a name your form accepts - somebody pressed upload on nothing. " +
			"Whether your form takes it or turns it away is its own policy - decide which, and check that it does that.",
	}}
}

// allowedFiles is one real file of every allowed type, comfortably inside the
// limit. If these fail, every refusal the rest of the set reports means nothing.
func (s uploadSet) allowedFiles() []setFile {
	out := make([]setFile, 0, len(s.allowed))
	for _, desc := range s.allowed {
		out = append(out, setFile{
			id: "allowed_" + desc.ID, group: allowedGroup, desc: desc,
			name: "allowed_" + desc.ID + desc.Extension,
			size: s.limit / halfShare, expected: "accept",
			purpose: fmt.Sprintf("A real %s file well inside the limit, of a type your form accepts. "+
				"Your form should take it - if it does not, every refusal in the rest of the set means nothing.", desc.Name),
		})
	}
	return out
}

// deniedFiles is one file per denied extension.
func (s uploadSet) deniedFiles() []setFile {
	out := make([]setFile, 0, len(s.denied))
	for _, entry := range s.denied {
		out = append(out, setFile{
			id: "denied_" + entry.ext, group: deniedGroup, desc: entry.desc,
			name: "denied" + entry.extension(), size: sampleFor(entry.desc),
			expected: "reject", reason: "extension_rule",
			purpose: entry.purpose(),
		})
	}
	return out
}

// purpose is what the instructions say about the file of one denied
// extension, and whether it really is what its name claims.
func (e deniedEntry) purpose() string {
	said := fmt.Sprintf("A file ending in %s, an extension your form turns away. It should be refused by its name, whatever it holds.", e.extension())
	if !e.known {
		said += " This build has no format of that name, so the file holds plain text."
	}
	return said
}

// mismatchFiles is every allowed type under the name of the next one.
//
// Built from the allow list rather than from three names written here, so the
// group is about the types the reader said it takes. Each file is something the
// form accepts, wearing the extension of something else it accepts - which is
// the shape that gets past a check made on the name.
//
// It needs two allowed types and there is nothing to say when there is one.
func (s uploadSet) mismatchFiles() []setFile {
	if len(s.allowed) < 2 {
		return nil
	}
	out := make([]setFile, 0, len(s.allowed))
	for i, named := range s.allowed {
		inside := s.allowed[(i+1)%len(s.allowed)]
		out = append(out, setFile{
			id: "mismatch_" + named.ID, group: mismatchGroup, desc: inside,
			name: inside.ID + "_as_" + named.ID + named.Extension,
			size: sampleFor(inside), expected: "reject", reason: "mime_mismatch",
			purpose: fmt.Sprintf("A real %s file under a name ending in %s. Your form takes both types and should still refuse this one - "+
				"a form that checks only the extension, and never looks inside, takes it.", inside.Name, named.Extension),
		})
	}
	return out
}

// anomalyFiles is three names that are not quite an extension.
func (s uploadSet) anomalyFiles() []setFile {
	desc := s.allowed[0]
	size := sampleFor(desc)
	out := []setFile{
		{
			id: "no_extension", group: anomalyGroup, desc: desc,
			name: "invoice", size: size,
			// Whether a form insists on an extension is its own rule, and both
			// answers are defensible.
			expected: "unspecified", reason: "extension_rule",
			purpose: "A file of a type your form accepts, named invoice with no extension at all. " +
				"Whether your form insists on an extension is its own rule - check that it gives the answer you meant.",
		},
		{
			id: "uppercase_extension", group: anomalyGroup, desc: desc,
			name: "PHOTO" + strings.ToUpper(desc.Extension), size: size,
			expected: "unspecified", reason: "extension_rule",
			purpose: "A file of a type your form accepts, with the name and the extension in capitals. " +
				"A form that compares extensions with case turns it away where it takes the same file in lower case - decide which you want.",
		},
	}
	// The double extension needs something denied to end with. The deny list
	// cannot be empty - the parser refuses one that names nothing - so this is
	// a guard against a list that arrived some other way rather than a case.
	if len(s.denied) > 0 {
		out = append(out, setFile{
			id: "double_extension", group: anomalyGroup, desc: desc,
			name: "invoice" + desc.Extension + s.denied[0].extension(), size: size,
			expected: "reject", reason: "extension_rule",
			purpose: "An allowed extension followed by a denied one. Your form should turn it away - " +
				"one that looks for an allowed extension anywhere in the name takes it, and a server may then run it.",
		})
	}
	return out
}

// nameFiles is three names a form has to write to disk, or refuse for a reason
// it can say out loud.
//
// The names are what they are on purpose: a long one for a column that is 255
// characters wide, one outside ASCII for a path handled as bytes, and one with
// spaces and brackets that is perfectly legal and still breaks a script that
// did not quote it.
func (s uploadSet) nameFiles() []setFile {
	desc := s.allowed[0]
	size := sampleFor(desc)
	return []setFile{
		{
			id: "long_name", group: nameGroup, desc: desc,
			name: strings.Repeat("a", 204) + desc.Extension, size: size,
			expected: "unspecified", reason: "filename_too_long",
			purpose: "A name of 204 letters and the extension. A database column or a path limit shorter than that cuts it or refuses it - " +
				"check which, and that the file is never stored under a shortened name without a word.",
		},
		{
			id: "outside_ascii", group: nameGroup, desc: desc,
			name: "wiadomość_日本語_🎉" + desc.Extension, size: size,
			expected: "unspecified", reason: "filename_invalid",
			purpose: "A name with Polish letters, Japanese and an emoji. A form that handles names as bytes, or in a single encoding, stores or shows it garbled.",
		},
		{
			id: "spaces_and_brackets", group: nameGroup, desc: desc,
			name: "my report (final) v2" + desc.Extension, size: size,
			// A legal name on every filesystem this runs on, so this one is a
			// promise rather than a question.
			expected: "accept",
			purpose:  "A name with spaces and brackets, legal on every system. Your form should take it - a script that does not quote names breaks on it.",
		},
	}
}

// bulkFiles is the mass upload, or none when it was turned off.
func (s uploadSet) bulkFiles() []setFile {
	if s.bulk == 0 {
		return nil
	}
	desc := s.allowed[0]
	return []setFile{{
		id: "bulk", group: bulkGroup, desc: desc, count: s.bulk,
		name: "bulk_{index:04}" + desc.Extension,
		size: s.limit / bulkShare, expected: "accept",
		purpose: fmt.Sprintf("%d files of a type your form accepts, uploaded together. Your form should take every one of them - "+
			"the set checks a limit on how many files arrive at once, and that none of them is lost.", s.bulk),
	}}
}
