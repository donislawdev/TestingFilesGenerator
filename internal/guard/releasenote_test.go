package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// What this defends. What a release note has to say is decided in one place,
// and both halves that care about it read that place.
//
// Why it needed a guard. Two workflows had an opinion about the same sentences.
// release.yml wrote them into the note it generates, and verify-release.yml
// held the published page to a list written out inside itself. Nothing compared
// the two. Change the predicate URI in the generator alone and the check goes
// red on a correct page - somebody then debugs the release page. Change it in
// the check alone and a page telling people the wrong command sails through.
// This is the shape closed for archive settings earlier the same day, O171,
// found again a few hours later in a different corner.
//
// Why the wiring rather than the wording. A person edits the published note by
// hand, by the owner's decision of 2026-09-02, so what the note SAYS is theirs.
// What is not theirs is whether anything still looks: deleting the call from
// verify-release.yml removes the check with nothing going red, which is the
// silent kind this project writes guards for.
//
// What this does NOT check, and it is checked another way. That note_says.sh
// behaves. Running it from here would mean running bash from a Go test on a
// machine with three of them, where the one a lookup finds is not the one
// CreateProcess starts - measured 2026-09-02 and written down. It was run by
// hand instead, three ways: a real published note missing all three sentences
// fails and names all three, a note carrying them passes, and an empty list is
// refused rather than passing on any note at all.

const (
	releaseNoteList   = "release-note-must-say.txt"
	releaseNoteScript = "note_says.sh"
)

// requiredOfAReleaseNote reads the list the way the script does.
func requiredOfAReleaseNote(t *testing.T) []string {
	t.Helper()

	body, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", releaseNoteList))
	if err != nil {
		t.Skipf("no %s here: %v", releaseNoteList, err)
	}

	var out []string
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

// One list, and both workflows read it.
func TestWhatAReleaseNoteHasToSayIsDecidedInOnePlace(t *testing.T) {
	required := requiredOfAReleaseNote(t)

	// An empty list would let every check built on it pass on any page, which
	// is the gate that never reads its own answer.
	if len(required) == 0 {
		t.Fatalf("%s names nothing, so every check reading it passes on any release note",
			releaseNoteList)
	}

	script := filepath.Join(repoRoot(t), ".github", "scripts", releaseNoteScript)
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("%s is missing, so the workflows that call it fail at the worst moment "+
			"- one of them runs on a tag: %v", releaseNoteScript, err)
	}

	// Both halves. The one that WRITES the note and the one that reads the
	// PUBLISHED page. Either alone leaves the other free to drift.
	for _, wf := range []string{"release.yml", "verify-release.yml"} {
		text := workflowText(t, wf)
		if !strings.Contains(text, releaseNoteScript) {
			t.Errorf("%s never calls %s, so nothing holds it to %s.\n"+
				"release.yml has to check the note it generates and verify-release.yml has "+
				"to check the note that got published. A missing call here is a check that "+
				"stopped happening with nothing going red.",
				wf, releaseNoteScript, releaseNoteList)
		}
	}
}

// The generator says everything the list asks for.
//
// Caught here rather than at release time on purpose. release.yml checks its
// own note when it runs, which is a tag - so a line added to the list and never
// taught to the generator would break the build at the one moment nobody wants
// a surprise. This asks the same question on a pull request.
func TestTheGeneratedReleaseNoteSaysEverythingTheListAsksFor(t *testing.T) {
	required := requiredOfAReleaseNote(t)
	if len(required) == 0 {
		t.Fatalf("%s names nothing, so this proved nothing", releaseNoteList)
	}

	text := workflowText(t, "release.yml")

	// The block that builds the note, rather than the whole file. A substring
	// found somewhere else in the workflow - in the step that computes the
	// checksums, say - would answer yes to a question about the note.
	const closes = "} > notes.md"
	end := strings.Index(text, closes)
	if end < 0 {
		t.Fatalf("release.yml no longer ends a block with %q, so this guard cannot find "+
			"the note it generates. Teach it the new shape rather than deleting it.", closes)
	}
	const opens = "\n          {\n"
	start := strings.LastIndex(text[:end], opens)
	if start < 0 {
		t.Fatalf("release.yml has %q with no block opening before it, so this guard cannot "+
			"tell where the note starts", closes)
	}
	block := text[start:end]

	for _, promised := range required {
		if !strings.Contains(block, promised) {
			t.Errorf("%s asks a release note to say %q and the note release.yml generates "+
				"never says it.\n"+
				"The workflow would catch this itself - on a tag, while building a release. "+
				"Either teach the generator to say it, or take the line out of the list.",
				releaseNoteList, promised)
		}
	}
}
