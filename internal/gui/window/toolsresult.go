package window

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/tool"
)

// The Result section of the Tools tab: what a run found, drawn, and laid out
// with the rest of the screen. The words are worded in toolstext.go.

// showResult draws what a run found, or what will appear there before one.
//
// The canvas is told once, after the last change - a deferred word to it read
// as coming before the changes to the guard that holds this
// (TestABoxThatGainsOrLosesAPieceSaysSo), which cannot see when a defer runs.
func (t *Tools) showResult(d tool.Descriptor, r *tool.Result) {
	t.result.RemoveAll()
	for _, o := range t.resultObjects(d, r) {
		t.result.Add(o)
	}
	t.result.Refresh()
	t.relay()
}

// relay lays the screen out again after a section changed what it holds,
// because the toolkit does not - the same kind of fault as busy.relay. The
// sections kept the heights of the tool chosen before (the owner's window,
// 2026-10-05): Algorithm drawn under its section with its list hidden by the
// next one, and an empty band after a tool with fewer boxes.
//
// The view is told rather than the page under it. Measured in the real window
// the same day, three changes of tool with each build: as it was, all three
// drawn with the old heights; with the view told, or with the page told, all
// three right. The view, because it is also what grows the room to scroll
// when a page comes out longer than the window.
func (t *Tools) relay() {
	if t.view.scroll == nil {
		return
	}
	t.view.scroll.Refresh()
}

// resultObjects is what a run found, as the section shows it: the rows, the
// notes, the verdict in the colour of its answer and a way to copy all of it -
// or a sentence saying nothing has run yet.
func (t *Tools) resultObjects(d tool.Descriptor, r *tool.Result) []fyne.CanvasObject {
	if r == nil {
		return []fyne.CanvasObject{parts.Prose(text.ToolNothingYet())}
	}
	var out []fyne.CanvasObject
	// Only when there are rows: an empty grid still stood in the column and
	// cost the gap above the first note (the stored picture of a check,
	// 2026-10-05).
	if len(r.Rows) > 0 {
		rows := parts.Grid()
		for _, row := range r.Rows {
			rows.Add(t.resultRow(row))
		}
		out = append(out, rows)
	}
	for _, n := range r.Notes {
		for _, line := range noteLines(d, n, parts.NoteItemsShown) {
			out = append(out, parts.Prose(line))
		}
	}
	switch r.Verdict.Outcome {
	case tool.Match:
		out = append(out, parts.Verdict(parts.Agrees, verdictSaid(r.Verdict)))
	case tool.Mismatch:
		out = append(out, parts.Verdict(parts.Disagrees, verdictSaid(r.Verdict)))
	case tool.Unasked:
	}
	return append(out, t.copyAll(d, *r))
}

// copyAll is the button under a result that puts the whole of it on the
// clipboard - the owner's list of 2026-10-05: five checksums of one file were
// five presses of Copy, and the lines of a check could not be copied at all.
// Held to the left edge in a row of its own, the way the Preferences screen
// holds its buttons.
func (t *Tools) copyAll(d tool.Descriptor, r tool.Result) fyne.CanvasObject {
	press := parts.NewButton(parts.Secondary, text.ButtonCopyAll(), func() {
		t.host.Copy(resultText(d, r))
		showOn(t.status, text.ToolCopiedAll())
	})
	return container.NewHBox(parts.ButtonRow(press))
}

// resultRow is one row of a result: its first cell as the name in the column
// of names, the rest beside it, and a way to copy what is beside it. The name
// is data - an algorithm, a file - and is shown as it is (G8).
func (t *Tools) resultRow(row []string) fyne.CanvasObject {
	if len(row) == 0 {
		return parts.Prose("")
	}
	value := ""
	if len(row) > 1 {
		value = row[len(row)-1]
	}
	copyIt := parts.NewButton(parts.Secondary, text.ButtonCopy(), func() {
		t.host.Copy(value)
		showOn(t.status, text.ToolCopied(row[0]))
	})
	return parts.Wide(t.fields.Named(row[0], parts.NoDetail,
		container.NewBorder(nil, nil, nil, copyIt, parts.Prose(value))))
}
