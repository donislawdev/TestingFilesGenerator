package guard

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// CLAUDE.md and STATE.md have a size each, and CLAUDE.md holds no state.
//
// CLAUDE.md is read by the assistant at every message of every session, so
// every byte in it is paid for as many times as there are messages. Measured
// 2026-09-21: it had grown to 310 036 bytes, about 90 thousand tokens a
// message, and 98 KB of that was a journal - twenty blocks headed "STAN NA
// <date>", fourteen of them struck through, one added at the top by each
// session and none removed. The owner's decision that day: a ceiling of
// roughly 30 KB, the state in one document that is rewritten rather than
// appended to (docs/STATE.md), the history in HISTORY.md.
//
// Both ceilings are guarded rather than asked for, because the growth was not
// one session forgetting: it was the rhythm every session followed. And the
// ceiling of CLAUDE.md is a ratchet - it may only come down - because a ceiling
// far above the measurement is room to grow back into (ratchet_test.go).
//
// Skips loudly on a fresh clone: neither file is in the repository.

// summaryCeiling is today's size of CLAUDE.md, rounded up to the next
// kilobyte. Lower it when the file shrinks. It may not go up.
const summaryCeiling = 37 * 1024

// summaryRatchetBand is how far below its ceiling CLAUDE.md may sit before
// the ceiling has to follow it down.
const summaryRatchetBand = 4 * 1024

// stateCeiling is the most docs/STATE.md may hold. A state that does not fit
// is a state carrying history: move what is finished to HISTORY.md.
const stateCeiling = 12 * 1024

// journalBlock is the shape of the blocks that turned CLAUDE.md into a
// journal: a dated state heading. The sentence forbidding them, which names
// the phrase without a date, is not a match.
var journalBlock = regexp.MustCompile(`STAN NA \d{4}-\d{2}-\d{2}`)

// summaryHeadings are the sections a new session is promised. A CLAUDE.md
// missing one of them was cut in the wrong place.
var summaryHeadings = []string{
	"## Od czego zacząć w nowej sesji",
	"## Pułapki, które już kosztowały",
	"## Rytm weryfikacji",
	"## Tryb pracy: analiza przed działaniem",
	"## Jak mam myśleć i meldować",
	"## Reguły nietykalne",
	"## Czego nie robić",
	"## Powierzchnia regresji",
	"## Dokumenty",
	"## Zasady GUI",
}

func TestTheSummaryFitsItsCeilingAndHoldsNoState(t *testing.T) {
	root := repoRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		t.Logf("SKIPPED: CLAUDE.md is not here (%v) - it is excluded from the repository", err)
		return
	}
	size := len(body)
	if size > summaryCeiling {
		t.Errorf("CLAUDE.md is %d bytes and the ceiling is %d.\n"+
			"Reason: every byte here is paid for at every message of every session.\n"+
			"What to do: move what is a date, a measurement or a story to docs/ - state to STATE.md,\n"+
			"history to HISTORY.md, the machine to ENVIRONMENT.md. Rules and procedures stay. Do not raise the ceiling.",
			size, summaryCeiling)
	}
	if summaryCeiling-size > summaryRatchetBand {
		t.Errorf("CLAUDE.md is %d bytes and the ceiling is %d, which is more than %d bytes of room to grow back into.\n"+
			"What to do: lower summaryCeiling to today's size rounded up to the next kilobyte.",
			size, summaryCeiling, summaryRatchetBand)
	}
	text := string(body)
	if m := journalBlock.FindAllString(text, -1); len(m) > 0 {
		t.Errorf("CLAUDE.md holds %d dated state block(s) (%q ...).\n"+
			"Reason: this is how it grew to 310 KB - one block per session, none removed.\n"+
			"What to do: the state lives in docs/STATE.md and is rewritten there. Move the block.", len(m), m[0])
	}
	for _, h := range summaryHeadings {
		if !strings.Contains(text, "\n"+h) {
			t.Errorf("CLAUDE.md has no section %q - a new session is promised it", h)
		}
	}
	t.Logf("CLAUDE.md is %d bytes under a ceiling of %d", size, summaryCeiling)

	state, err := os.ReadFile(filepath.Join(root, "docs", "STATE.md"))
	if err != nil {
		t.Logf("SKIPPED the state half: docs/STATE.md is not here (%v)", err)
		return
	}
	if len(state) > stateCeiling {
		t.Errorf("docs/STATE.md is %d bytes and the ceiling is %d.\n"+
			"Reason: a state that does not fit is a state carrying history.\n"+
			"What to do: move what is finished to HISTORY.md and rewrite the rest. Do not raise the ceiling.",
			len(state), stateCeiling)
	}
	if strings.Count(string(state), "\n## Gdzie jestesmy") != 1 {
		t.Errorf("docs/STATE.md has %d sections headed \"Gdzie jestesmy\" and it has to have exactly one - "+
			"it is rewritten, not appended to", strings.Count(string(state), "\n## Gdzie jestesmy"))
	}
	t.Logf("docs/STATE.md is %d bytes under a ceiling of %d", len(state), stateCeiling)
}
