package window

import (
	"path/filepath"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/tool"
)

// What the Tools tab says about a run as words rather than as controls: the
// lines of a note, the verdict, what Copy all puts on the clipboard, and the
// folder Open folder leads to. Apart from the screen's type and from the
// toolkit (GUI rule 15), so a line is worded once whether a control draws it
// or the clipboard takes it.

// noteLines is one note of a result as lines: its sentence with the item on
// the same line when there is one, or followed by a line an item. When most
// is above nought, no more items than that, then how many were left out and
// where to see them. Each item goes through core.Shown, the same as on the
// command line, since an item is usually a file name and a name may hold a
// line break.
func noteLines(d tool.Descriptor, n tool.Noted, most int) []string {
	says := text.ToolNote(d.ID, n.ID, d.NoteSays(n.ID))
	if len(n.Items) == 1 {
		return []string{says + " " + itemOf(n, 0)}
	}
	out := []string{says}
	for i := range n.Items {
		if most > 0 && i == most {
			out = append(out, text.ToolMoreItems(len(n.Items)-i, d.ID))
			break
		}
		out = append(out, itemOf(n, i))
	}
	return out
}

// itemOf is one item of a note: in the window's words where the tool gave
// some, and as the data it is otherwise.
func itemOf(n tool.Noted, i int) string {
	if len(n.Words) == len(n.Items) {
		return core.ShownText(text.Sentence(n.Words[i]))
	}
	return core.Shown(n.Items[i])
}

// verdictSaid is tool.Verdict.Said in the window's language.
func verdictSaid(v tool.Verdict) string {
	switch {
	case v.Outcome == tool.Match && v.Listed:
		return text.ToolListMatches(v.About)
	case v.Outcome == tool.Mismatch && v.Listed:
		return text.ToolListDoesNotMatch(v.About)
	case v.Outcome == tool.Match:
		return text.ToolMatches(v.About, v.Got)
	}
	return text.ToolDoesNotMatch(v.About, v.Got, v.Wanted)
}

// rowLine is one row of a result as a line: its name and the value the
// section shows beside it.
func rowLine(row []string) string {
	if len(row) < 2 {
		return strings.Join(row, "")
	}
	return text.ToolResultLine(row[0], row[len(row)-1])
}

// resultText is what Copy all puts on the clipboard: the result as the section
// shows it, in its order and in the window's language (the owner's choice of
// 2026-10-05) - every row, every note, the verdict.
//
// Every item of a note, where the section stops at NoteItemsShown. The
// section cuts a long list short so it stays a screen, and a list somebody
// copies is one they want whole.
func resultText(d tool.Descriptor, r tool.Result) string {
	lines := make([]string, 0, len(r.Rows)+len(r.Notes)+1)
	for _, row := range r.Rows {
		lines = append(lines, rowLine(row))
	}
	for _, n := range r.Notes {
		lines = append(lines, noteLines(d, n, 0)...)
	}
	if r.Verdict.Outcome != tool.Unasked {
		lines = append(lines, verdictSaid(r.Verdict))
	}
	return strings.Join(lines, "\n")
}

// ranIn is the folder a run of a tool worked in, for Open folder: the folder
// it was given, or the folder of the file it was given - the first thing the
// tool works on, which is the one its question is about. Empty when it was
// given nothing to work on.
func ranIn(d tool.Descriptor, in tool.Request) string {
	if len(d.Inputs) == 0 {
		return ""
	}
	first := d.Inputs[0]
	named := in.Inputs[first.Name]
	if named == "" {
		return ""
	}
	folder := named
	if first.Kind != tool.Folder {
		folder = filepath.Dir(named)
	}
	if abs, err := filepath.Abs(folder); err == nil {
		folder = abs
	}
	return folder
}
