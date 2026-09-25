package manifest

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// instructionsSuffix is what the name of the instructions ends in. The rest of
// the name is the manifest's, so the two stand side by side in any listing
// and two runs recorded under two manifests in one directory never meet.
const instructionsSuffix = ".instructions.md"

// InstructionsName is the name of the instructions written beside a manifest
// of the given name: manifest.json gives manifest.instructions.md, and a name
// that does not end in .json keeps all of itself.
func InstructionsName(manifestName string) string {
	stem := manifestName
	if ext := filepath.Ext(stem); strings.EqualFold(ext, ".json") {
		stem = strings.TrimSuffix(stem, ext)
	}
	return stem + instructionsSuffix
}

// isInstructionsName says whether a name read from a manifest can be the
// instructions beside it: a plain file name, no directory in it, ending the
// way InstructionsName ends one.
func isInstructionsName(name string) bool {
	return strings.HasSuffix(name, instructionsSuffix) &&
		!strings.ContainsAny(name, `/\:`) && core.ContainmentProblem(name) == ""
}

// Explains says whether any file of the run was given a purpose, which is
// when the run writes instructions at all. A run nobody explained - a single
// format from the command line - has nothing to say beyond the manifest.
func (m *Manifest) Explains() bool {
	for _, f := range m.Files {
		if f.Purpose != "" {
			return true
		}
	}
	return false
}

// Instructions is the manifest said in words for the person who opens the
// directory: what each file is, why it is in the set and what their system is
// expected to do with it. Nil when no file was given a purpose.
//
// Written from the manifest alone, so the command line and the window write
// the same bytes for the same run, and nothing in it depends on when or where
// the run happened - no time, no command line, no directory. The manifest is
// named rather than linked, because the reader may be looking at the file on
// its own.
//
// A list rather than a table. A file name here can hold a pipe, and a pipe
// ends a cell in every table syntax Markdown has.
func (m *Manifest) Instructions(manifestName string) []byte {
	if !m.Explains() {
		return nil
	}
	var b strings.Builder
	m.writeOpening(&b, manifestName)
	writeOutcomes(&b)
	groups := groupsOf(m.Files)
	for _, g := range groups {
		fmt.Fprintf(&b, "\n## %s (%s)\n\n", groupHeading(g.name, len(groups)), core.Count(len(g.files), "file", "files"))
		for _, run := range runsOf(g.files) {
			writeRun(&b, run)
		}
	}
	return []byte(b.String())
}

func (m *Manifest) writeOpening(b *strings.Builder, manifestName string) {
	b.WriteString("# What these files are for\n\n")
	var total int64
	for _, f := range m.Files {
		total += f.Bytes
	}
	fmt.Fprintf(b, "This directory holds %s, %s in all, written by %s %s",
		core.Count(len(m.Files), "file", "files"), core.ExactBytes(total), m.Tool.Name, m.Tool.Version)
	if p := m.Run.Preset; p != nil {
		fmt.Fprintf(b, " from the preset %s", codeSpan(p.ID))
		if p.Question != "" {
			fmt.Fprintf(b, ". It answers one question:\n\n> %s\n\n", oneLine(p.Question))
		} else {
			b.WriteString(".\n\n")
		}
	} else {
		b.WriteString(".\n\n")
	}
	fmt.Fprintf(b, "Each file below says what it is, why it is in the set and what your system is expected to do with it. "+
		"The same facts are in %s, in a form a program can read.\n", codeSpan(manifestName))
}

func writeOutcomes(b *strings.Builder) {
	b.WriteString("\n## How to read what is expected\n\n" +
		"- **accept** - your system should take the file.\n" +
		"- **reject** - your system should turn the file away.\n" +
		"- **sanitize** - your system should take the file and clean it, for example by renaming it.\n" +
		"- **unspecified** - it depends on the rules of your system. The file is here so that you decide, and then check that what happens is what you meant.\n\n" +
		"The word in brackets after it names the rule the file is about, such as `size_limit` or `filename_invalid`.\n")
}

// runsOf cuts a group's files into runs of one target each, so that fifty
// files of a mass upload read as one entry rather than fifty copies of one
// sentence. A file that failed, or was not written, stands on its own, because
// what happened to it is said about it alone.
func runsOf(files []File) [][]File {
	var out [][]File
	for _, f := range files {
		if n := len(out); n > 0 && sameRun(out[n-1][0], f) {
			out[n-1] = append(out[n-1], f)
			continue
		}
		out = append(out, []File{f})
	}
	return out
}

func sameRun(a, b File) bool {
	whole := func(f File) bool { return !f.Failed && f.Materialized }
	return a.TargetID != "" && a.TargetID == b.TargetID && whole(a) && whole(b)
}

// writeRun is one entry of the list: a file, or a run of files of one target.
func writeRun(b *strings.Builder, run []File) {
	if len(run) == 1 {
		writeFile(b, run[0])
		return
	}
	var total int64
	for _, f := range run {
		total += f.Bytes
	}
	first, last := run[0], run[len(run)-1]
	fmt.Fprintf(b, "- %s, %s to %s - %s, %s in all.", core.Count(len(run), "file", "files"),
		codeSpan(core.Shown(first.Name)), codeSpan(core.Shown(last.Name)), first.Format, core.ExactBytes(total))
	writeExpected(b, first.Expected)
	writePurpose(b, first)
}

func writeFile(b *strings.Builder, f File) {
	fmt.Fprintf(b, "- %s - %s, %s.", codeSpan(core.Shown(f.Name)), f.Format, core.ExactBytes(f.Bytes))
	writeExpected(b, f.Expected)
	switch {
	case f.Failed:
		fmt.Fprintf(b, "  Not written: %s\n", oneLine(f.Error))
	case !f.Materialized:
		b.WriteString("  Described in the manifest and not written to the disk.\n")
	}
	writePurpose(b, f)
}

// writeExpected ends the first line of an entry with what is expected of it.
func writeExpected(b *strings.Builder, e Expected) {
	if e.Outcome != "" {
		fmt.Fprintf(b, " Expected: **%s**", e.Outcome)
		if e.Reason != "" {
			fmt.Fprintf(b, " (%s)", codeSpan(e.Reason))
		}
		b.WriteString(".")
	}
	b.WriteString("\n")
}

// writePurpose is the second line of an entry: what the file is for.
func writePurpose(b *strings.Builder, f File) {
	if f.Purpose != "" {
		fmt.Fprintf(b, "  %s\n", oneLine(f.Purpose))
		return
	}
	b.WriteString("  No purpose was given for this file.\n")
}

// fileGroup is the files of one group, in the order the manifest lists them.
type fileGroup struct {
	name  string
	files []File
}

// groupsOf puts the files under their groups, the groups in the order their
// first file comes, so the instructions read in the order the run wrote.
func groupsOf(files []File) []fileGroup {
	var out []fileGroup
	at := map[string]int{}
	for _, f := range files {
		i, seen := at[f.Group]
		if !seen {
			i = len(out)
			at[f.Group] = i
			out = append(out, fileGroup{name: f.Group})
		}
		out[i].files = append(out[i].files, f)
	}
	return out
}

// groupHeading is what a group's section is called. Files nobody grouped are
// "the files" when they are all there is, and set apart from the rest when not.
func groupHeading(name string, groups int) string {
	switch {
	case name != "":
		return oneLine(name)
	case groups == 1:
		return "The files"
	default:
		return "Files in no group"
	}
}

// oneLine is text from a recipe made safe to stand on one line of a list: any
// run of white space, a line break included, becomes one space, and a
// character nobody can see is written as an escape.
func oneLine(s string) string {
	return core.Shown(strings.Join(strings.Fields(s), " "))
}

// codeSpan puts a name between backticks, so that nothing in it is read as
// Markdown - a name here can be <script>, $(id) or `id`.
//
// The fence is one backtick longer than the longest run inside the name, which
// is how CommonMark lets a code span hold backticks. A name that begins or
// ends with a backtick, or begins and ends with a space, gets a space inside
// each end, because a reader takes one space off each end of such a span.
func codeSpan(s string) string {
	longest, run := 0, 0
	for _, r := range s {
		if r != '`' {
			run = 0
			continue
		}
		run++
		longest = max(longest, run)
	}
	fence := strings.Repeat("`", longest+1)
	spaced := strings.HasPrefix(s, " ") && strings.HasSuffix(s, " ") && strings.TrimSpace(s) != ""
	if strings.HasPrefix(s, "`") || strings.HasSuffix(s, "`") || spaced {
		s = " " + s + " "
	}
	return fence + s + fence
}

// SaveInstructions writes the instructions under a name nobody holds.
//
// Claimed at the moment of writing rather than before the first file, like the
// manifest is. The manifest's claim already keeps two runs of one record apart,
// and this name is the manifest's own with a different ending - so the only run
// that could reach it is one refused before it wrote anything. The claim is
// here for the case that leaves: a file put there by hand while the run went.
func SaveInstructions(path string, text []byte) error {
	if err := claimName(path); err != nil {
		return err
	}
	err := writeClaimed(path, 0o644, func(w io.Writer) error {
		_, err := w.Write(text)
		return err
	})
	if err != nil {
		_ = Release(path)
		return err
	}
	return nil
}
