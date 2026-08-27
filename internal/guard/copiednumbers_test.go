package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// A PARTIAL guard, and the word is load-bearing. It covers one defect class and
// says out loud what it leaves alone, because a control that is quiet about its
// limits is worse than no control - the same reason the regression surface
// writes CZESCIOWY instead of JEST.
//
// The class: a number that some test already prints, copied into prose. Prose
// has no guard, the documents live outside the repository so no diff shows the
// drift, and the copy reads exactly as confidently once it is false. Measured
// on 2026-08-05 by reading three documents: twelve sentences had drifted, and
// two of them contradicted another section of the same file.
//
// It is built on two rules, and the second one is the interesting one.
//
//   - Derivable here: the number must match what this package can compute.
//   - Printed by another test: the number must not appear in prose at all.
//
// Recomputing the second kind here would put a second implementation of the
// count beside the first, which is the very defect this guard exists to stop.
// So instead of checking the copy, it forbids the copy - the documents already
// say "the test prints it", and this makes that sentence enforceable.
//
// What it does NOT cover, named so nobody reads a green run as more than it is:
// numbers measured on the owner's machine (bytes, timings, throughput,
// allocation counts, sizes of other people's tools), and every statement about
// the world outside this repository. Those stay on the reader.
//
// And one more, dropped after being built and measured rather than guessed at:
// HOW MANY FORMATS. The first run raised 43 findings and most were false, all
// from that one rule - "25 formats" is the T1 scope, "five formats" is MVP-0,
// "ten formats" is the ones with an oracle, "150 formats" is a target for
// later. A count of formats in a sentence answers whichever question the
// sentence asked, and no pattern tells them apart. Keeping it would have meant
// either a wave of false alarms, which teaches everybody to skip the output, or
// an allowlist longer than the guard. Same decision as ST1005 in
// staticcheck.conf, and written down here for the same reason: so it is not
// tried again without a new idea.

// derivedTotals is the first rule. The key names the thing, the value is what
// this package computes right now.
func derivedTotals() map[string]int {
	return map[string]int{
		"parity reachable": len(reachableFromTheWindow),
		"parity total":     len(capabilities()),
	}
}

// Rule A. A pair on a line that is talking about parity.
var parityPair = regexp.MustCompile(`(\d+)\s*(?:z|of)\s*(\d+)`)

// retraction marks a sentence that quotes a number in order to withdraw it.
// This project keeps the retracted claim visible on purpose - CLAUDE.md calls
// striking it out quietly an invitation for the next session to draw the same
// wrong conclusion - so a guard that fired on those would push against a rule
// worth more than itself.
var retraction = []string{"~~", "Stało tu", "Stało „", "stało tu", "Poprzedni stan",
	"Oryginalny wpis", "Poprawione 20", "przy uzbrojeniu było"}

// forbidden is the second rule: numbers another test prints, which must not be
// written down here at all. The reason travels with the pattern so that a
// failure says which test to run instead of just saying no.
//
// Each Polish word also matches its form without diacritics. That is not
// sloppiness - these documents already carry whole passages typed without
// them, so a pattern demanding the accented spelling would have a hole exactly
// where somebody was typing in a hurry. Found by probing this guard rather
// than by reading it: the first probe planted six bad numbers and only three
// were caught, and the three that got through were the ones written plainly.
var forbidden = []struct {
	what    string
	pattern *regexp.Regexp
	instead string
}{
	{"the identifier count", regexp.MustCompile(`(?:^|[^-\d])\d+\s+identyfikator`),
		"go test ./internal/guard/ -run ^TestEveryIdentifierAReferencePointsAtExists$ -v"},
	{"the count of links between documents", regexp.MustCompile(`(?:^|[^-\d])\d+\s+odsy[łl]acz`),
		"go test ./internal/guard/ -run ^TestEveryLinkBetweenDocumentsLeadsSomewhere$ -v"},
	{"the mutation coverage breakdown", regexp.MustCompile(`(?:^|[^-\d])\d+\s+(?:mutacj[ąa]|sond[ąa]|niesprawdzonych)`),
		"go test ./internal/guard/ -run ^TestEveryGuardIsEither -v"},
	{"the number of guards", regexp.MustCompile(`(?:^|[^-\d])\d+\s+stra[żz]nik`),
		"go test ./internal/guard/ -run ^TestEveryGuardIsEither -v"},
	{"the coverage figure", regexp.MustCompile(`(?i)pokrycie\s+\d+[,.]\d+\s*%|\d+[,.]\d+\s*%\s*(?:pokrycia|przy progu)`),
		"a run with -coverpkg, from bash and not PowerShell - see CLAUDE.md"},
}

// allowed suppresses one line by a fragment of it, and demands a reason. A
// stale number is legitimate when the sentence records what was true on a day
// rather than what is true now - a milestone does not drift, it is dated. The
// reason is required for the same purpose as on provenByProbe: an entry with
// nothing written beside it is indistinguishable from a hole.
var allowed = map[string]string{
	"nie widziało 186 strażników":  "dated 2026-08-04: what the suite missed the day macOS was first run",
	"nie widziało 96 strażników":   "dated 2026-08-02: what it missed when the owner looked at ten files",
	"nie wykryło 96 strażników":    "the same measurement, quoted beside the batch it came from",
	"Na tamtym etapie było":        "the TXT milestone, and the sentence says outright that it is one",
	"różnica kosztowała 28":        "dated 2026-08-04: how many mutations had gone stale",
	"około 26 strażników twierdzi": "the same 2026-08-04 measurement, in QUALITY.md and REVIEW.md",
	"z `0 z 60` na `39 z 60`":      "records the milestone the window moved, and says so in the sentence",
	"przy uzbrojeniu było":         "OBSERVATIONS.md O37, what the number was when the guard was armed",
}

// snapshots names documents that record the state of ONE MOMENT rather than
// what is true today. A number copied into one of those is the point of the
// file rather than a defect in it, and holding it to the rule this guard
// enforces would mean deleting the only thing the file is for.
//
// The reason is required for the same purpose as on allowed: an entry with
// nothing written beside it is indistinguishable from a hole. The list is
// checked against the directory below, so a file renamed out from under an
// entry reddens this rather than quietly exempting nothing.
//
// Owner's decision, 2026-08-27, after this guard was found red on a file
// committed the evening before. It went red at the moment that file was
// written and stayed red across a session boundary, which is the second half
// of why the entry is here: a document whose whole job is to say what the
// counts were cannot also promise they still hold.
var snapshots = map[string]string{
	"PROMPT-NOWY-CZAT.md": "the state handed to the next session, true at one hour and saying so in its own first lines",
}

func documentBodies(t *testing.T) map[string]string {
	t.Helper()
	root := repoRoot(t)
	out := map[string]string{}

	docs := filepath.Join(root, "docs")
	entries, err := os.ReadDir(docs)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		seen[e.Name()] = true
		if _, snapshot := snapshots[e.Name()]; snapshot {
			continue
		}
		body, err := os.ReadFile(filepath.Join(docs, e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		out["docs/"+e.Name()] = string(body)
	}

	// An entry naming a file that is not there exempts nothing and reads as if
	// it does, which is the same shape as a mutation aimed at a renamed test.
	for name, why := range snapshots {
		if !seen[name] {
			t.Errorf("snapshots names docs/%s and no such document exists - it was renamed or removed", name)
		}
		if strings.TrimSpace(why) == "" {
			t.Errorf("docs/%s is exempt with no word about why, which is the same as no exemption at all", name)
		}
	}
	if body, err := os.ReadFile(filepath.Join(root, "CLAUDE.md")); err == nil {
		out["CLAUDE.md"] = string(body)
	}
	return out
}

// excused reports whether a line was deliberately let through, either because
// it withdraws the number it quotes or because it is on the list below.
func excused(line string) bool {
	for _, marker := range retraction {
		if strings.Contains(line, marker) {
			return true
		}
	}
	for fragment, reason := range allowed {
		if strings.TrimSpace(reason) != "" && strings.Contains(line, fragment) {
			return true
		}
	}
	return false
}

// checkDerived applies rule A to one line.
func checkDerived(where string, n int, line string, totals map[string]int) []string {
	var found []string
	lower := strings.ToLower(line)

	if strings.Contains(lower, "parytet") || strings.Contains(lower, "parity") {
		for _, m := range parityPair.FindAllStringSubmatch(line, -1) {
			got, total := atoi(m[1]), atoi(m[2])
			if total != totals["parity total"] || got == totals["parity reachable"] {
				continue
			}
			found = append(found, fmt.Sprintf(
				"%s:%d says D1 parity is %d of %d and the test says %d of %d",
				where, n, got, total, totals["parity reachable"], totals["parity total"]))
		}
	}
	return found
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// checkForbidden applies rule B to one line.
func checkForbidden(where string, n int, line string) []string {
	var found []string
	for _, f := range forbidden {
		if !f.pattern.MatchString(line) {
			continue
		}
		found = append(found, fmt.Sprintf(
			"%s:%d writes down %s, which a test already prints.\n    Remove the number and point at: %s",
			where, n, f.what, f.instead))
	}
	return found
}

func TestNoNumberATestPrintsIsCopiedIntoTheProse(t *testing.T) {
	bodies := documentBodies(t)
	if len(bodies) == 0 {
		t.Logf("SKIPPED: docs/ is not here, so nothing was compared. " +
			"The internal documents are excluded from the repository, so this check only runs on a machine that has them.")
		return
	}

	totals := derivedTotals()
	var found []string

	for where, body := range bodies {
		for i, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
			if excused(line) {
				continue
			}
			found = append(found, checkDerived(where, i+1, line, totals)...)
			found = append(found, checkForbidden(where, i+1, line)...)
		}
	}

	// Every one at once, for the reason RC7 gives about recipes: being sent
	// back once per answer is its own failure.
	if len(found) > 0 {
		sort.Strings(found)
		t.Errorf("%d number(s) copied out of a test and into prose:\n  %s\n\n"+
			"Each is a sentence that reads exactly as confidently once it is false, in a file no diff covers.",
			len(found), strings.Join(found, "\n  "))
	}

	t.Logf("PARTIAL: checked %d documents for the numbers this package can derive "+
		"(D1 parity: %d of %d) and for four counts other tests print. "+
		"NOT checked: anything measured on the owner's machine - bytes, timings, throughput, "+
		"allocations, sizes and limits of other people's tools - and how many formats there are, "+
		"for the reason written at the top of this file.",
		len(bodies), totals["parity reachable"], totals["parity total"])
}
