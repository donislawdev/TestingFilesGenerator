package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The build-on-demand workflow hands somebody an unsigned binary, which makes it
// the one workflow here whose failure mode is a person trusting a file they
// should not. Two things hold it, and they are different worries.
//
// It has to offer what the release offers. Somebody reports a bug on Linux
// arm64, the fix lands, and a workflow that quietly builds four platforms out of
// five has nothing to give them - and nothing would say so, because a missing
// platform is a build that simply did not happen. The platform list is the one
// fact the two workflows share, so it is the one thing read out of both.
//
// And it has to stay unable to publish. A release here is signed on two
// machines and published by a person on purpose. A second workflow that can
// write to a release page is a way around all of that, and it would not need to
// be used on purpose to do damage - contents: write plus one careless step is
// enough.
//
// Read as text rather than parsed as YAML, like the other workflow guards in
// this package. A parser would be better if anything here needed structure, and
// nothing does: both facts are lists somebody can see.

const devBuildWorkflow = "dev-build.yml"

// devBuildBody reads the workflow and FAILS when it is not there, rather than
// skipping the way workflowText does for the signing guards.
//
// The difference is deliberate. Those skip so that a partial checkout does not
// go red about a file it never had. Here the file is the subject: if it has been
// deleted, every question below is unanswered, and a guard that goes quiet about
// its own subject is the shape this project has written down twice.
func devBuildBody(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", devBuildWorkflow))
	if err != nil {
		t.Fatalf("reading %s: %v.\n"+
			"Reason: this guard is about that workflow. Without it there is nothing to check, and\n"+
			"passing quietly would read exactly like passing.", devBuildWorkflow, err)
	}
	return string(raw)
}

// crossCompiledTargets reads the GOOS/GOARCH list out of the shell loop that
// builds the command line binaries.
//
// Line by line, and the first version was not. It cut the text at the first
// "do" and got four characters, because "windows" carries one in the middle of
// it - so the guard read an empty list and said so rather than passing, which
// is the only reason it was noticed at once. The loop ends at a LINE that is
// `do`, which is the thing the shell means too.
func crossCompiledTargets(t *testing.T, body, name string) []string {
	t.Helper()
	_, after, found := strings.Cut(body, "for target in")
	if !found {
		t.Fatalf("%s has no `for target in` loop, so this guard is reading the wrong thing", name)
	}

	var out []string
	closed := false
	for _, line := range strings.Split(after, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "do" {
			closed = true
			break
		}
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		for _, field := range strings.Fields(strings.ReplaceAll(trimmed, "\\", " ")) {
			if strings.Contains(field, "/") {
				out = append(out, field)
			}
		}
	}
	if !closed {
		t.Fatalf("%s has a target loop that never opens, so this guard cannot tell where its list ends", name)
	}
	if len(out) == 0 {
		t.Fatalf("no targets were read out of %s - this guard would pass against any list", name)
	}
	return out
}

// matrixSystems reads the runner list out of the window build's matrix.
func matrixSystems(t *testing.T, body, name string) []string {
	t.Helper()
	_, after, found := strings.Cut(body, "matrix:")
	if !found {
		t.Fatalf("%s has no matrix, so this guard is reading the wrong thing", name)
	}
	var out []string
	for _, line := range strings.Split(after, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		// The list ends at the first line that is not a comment and not one of
		// its own entries.
		if !strings.HasPrefix(trimmed, "- ") {
			if len(out) > 0 && trimmed != "" && !strings.HasSuffix(trimmed, ":") {
				break
			}
			continue
		}
		if runner := strings.TrimPrefix(trimmed, "- "); strings.Contains(runner, "-") {
			out = append(out, runner)
		}
	}
	if len(out) == 0 {
		t.Fatalf("no runners were read out of %s - this guard would pass against any matrix", name)
	}
	return out
}

// The build-on-demand workflow builds every platform the release builds.
func TestTheBuildOnDemandOffersEveryPlatformTheReleaseDoes(t *testing.T) {
	release := workflowText(t, "release.yml")
	dev := devBuildBody(t)

	same := func(what string, want, got []string) {
		t.Helper()
		if strings.Join(want, " ") == strings.Join(got, " ") {
			return
		}
		t.Errorf("the release builds %s [%s] and %s builds [%s].\n"+
			"Reason: this workflow exists to hand somebody a fix before it is released, and a\n"+
			"platform it does not build is a fix that person cannot have. Nothing else would say\n"+
			"so, because a missing platform looks like a build that simply did not run.\n"+
			"What to do: bring the two lists back together, or say here why they differ.",
			what, strings.Join(want, ", "), devBuildWorkflow, strings.Join(got, ", "))
	}

	same("command line targets",
		crossCompiledTargets(t, release, "release.yml"),
		crossCompiledTargets(t, dev, devBuildWorkflow))
	same("window systems",
		matrixSystems(t, release, "release.yml"),
		matrixSystems(t, dev, devBuildWorkflow))
}

// The build-on-demand workflow cannot publish anything, and every archive it
// makes says out loud that it is not a release.
func TestTheBuildOnDemandCannotPublishAndSaysItIsUnofficial(t *testing.T) {
	body := devBuildBody(t)

	// Read only, and asked as "nothing is granted write" rather than as
	// "contents: read is present" - a second permission line beside it would
	// pass the second question and fail the first.
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || !strings.HasSuffix(trimmed, "write") {
			continue
		}
		t.Errorf("%s grants %q.\n"+
			"Reason: a release here is signed on two machines and published by a person. A workflow\n"+
			"that can write to a release page is a way around all of that, and it does not have to\n"+
			"be used deliberately to do harm.",
			devBuildWorkflow, trimmed)
	}

	// Nothing that puts a file anywhere a stranger would find it.
	for _, forbidden := range []string{"gh release", "softprops/action-gh-release", "GITHUB_TOKEN"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("%s mentions %q, which is how something gets published.\n"+
				"Reason: the binaries this workflow builds are unsigned. They belong on a run page\n"+
				"behind a login, not anywhere a stranger can reach them.",
				devBuildWorkflow, forbidden)
		}
	}

	// Every job that packages an archive writes the note. Two jobs, two calls -
	// counted rather than merely found, because one job losing its call would
	// leave the other one's proving nothing about it.
	if calls := strings.Count(body, "unofficial_note.sh"); calls != 2 {
		t.Errorf("%s calls unofficial_note.sh %d time(s) and there are two jobs that package archives.\n"+
			"Reason: the note is the only thing in the archive that can say which commit this is -\n"+
			"the version inside the binary is a constant and reports whatever the branch inherited.\n"+
			"An archive without it is an unsigned binary with nothing to identify it.",
			devBuildWorkflow, calls)
	}

	// And the note has to be there to be called.
	note := filepath.Join(repoRoot(t), ".github", "scripts", "unofficial_note.sh")
	if _, err := os.Stat(note); err != nil {
		t.Fatalf("the note script is missing: %v", err)
	}
}

// The note says the three things somebody unpacking this a month later needs to
// know, and it says them in words rather than by leaving them out.
func TestTheUnofficialNoteSaysWhatTheArchiveIsNot(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "scripts", "unofficial_note.sh"))
	if err != nil {
		t.Fatalf("reading the note script: %v", err)
	}
	body := string(raw)

	// Each of these is something a person could otherwise assume. Being unsigned
	// is why their system will refuse it, the missing attestation is what a
	// release has and this does not, and the version is the one that actively
	// misleads - it reports the version of whatever the branch started from.
	for _, must := range []string{"not signed", "attestation", "version"} {
		if !strings.Contains(strings.ToLower(body), must) {
			t.Errorf("the note in every unofficial archive never mentions %q.\n"+
				"Reason: this note is read by somebody who has the file and no memory of where it\n"+
				"came from. What it does not say, they will assume.", must)
		}
	}
}
