package guard

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// What the stored pictures of a refusal need in order to be pictures of one.
// Kept apart from screenpixels_test.go, which runs the scenes, because this is
// the one question asked of a scene's STATE rather than of its pixels - and
// the day it was not asked, two pictures named "refused" held a successful
// preview for a whole pull request (#125, found on #126).

// emptyTheFirstBatch clears the two boxes the first batch cannot do without,
// by position rather than by label - two batches mean two boxes of each name,
// and the first of them is the one this empties.
func emptyTheFirstBatch(t *testing.T, o fyne.CanvasObject) {
	t.Helper()
	for _, label := range []string{text.FieldTargetID(), text.FieldSize()} {
		boxes := entriesUnder(o, label)
		if len(boxes) == 0 {
			t.Fatalf("there is no %q box on this screen to empty", label)
		}
		boxes[0].SetText("")
	}
}

// showsARefusal says whether a stored tree holds words in the error colour -
// a refusal under a field or at the foot of the form. The star of a required
// field is in the same colour and is on every screen at rest, so a text that
// is only the star does not count.
func showsARefusal(markup string) bool {
	for _, line := range strings.Split(markup, "\n") {
		if !strings.Contains(line, `color="error"`) || !strings.Contains(line, "<text") {
			continue
		}
		start := strings.Index(line, ">")
		end := strings.LastIndex(line, "</text>")
		if start < 0 || end <= start {
			continue
		}
		if words := strings.TrimSpace(line[start+1 : end]); words != "" && words != "*" {
			return true
		}
	}
	return false
}
