package guard

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Every guard file the verdict table of REGRESSION.md cites as evidence has a
// paragraph further down the same document saying what it defends.
//
// The two tables were one table until 2026-08-12, when the justifications were
// moved out because they were 12.8k tokens of a file that is read at every step
// of every session. The split copied the table byte for byte, and from that day
// only one of the two grew - the whole surface of the window arrived in one and
// nothing pulled the other after it.
//
// This is not one session forgetting. It is drift built into splitting a
// document, and nothing was watching for it: CLAUDE.md has a guard for its
// references and another for its numbering, which makes it easy to assume this
// is watched as well. Measured 2026-08-26: the table in CLAUDE.md cites 73
// guard files and REGRESSION.md justifies 60. O132.
//
// What is actually at stake is the verdict column. "JEST" with nothing behind
// it reads as an assurance, which is precisely what the header of REGRESSION.md
// warns against.
//
// Until 2026-09-21 the verdict table lived in CLAUDE.md and the paragraphs
// here, so this compared two files. That day CLAUDE.md was cut from 310 KB to
// a guarded ceiling and the table moved in above the paragraphs, so the two
// halves are now two sections of one document, split at the heading below.
// The drift this watches for is unchanged: a row can be added to the table
// without a paragraph, whichever file the table is in.

// notYetJustified is the sixteen this check started with, and it may only
// shrink.
//
// The same shape as the mutation coverage guard, which passes a guard that is
// either proven or named as not proven. A name leaves this list when somebody
// reads the commits and the guard itself and writes what it defends - not from
// the file name, because a justification reconstructed from a name is exactly
// what rule 2 forbids, and it would read the same as one that was earned.
//
// A name that is here and NO LONGER cited is a failure too. Otherwise the list
// would quietly become the place drift hides.
var notYetJustified = []string{
	"actionbar_test.go",
	"compose_test.go",
	"darkmenus_test.go",
	"doccomments_test.go",
	"dropdown_test.go",
	"everyfield_test.go",
	"exeproperties_test.go",
	"foldedsections_test.go",

	"guitext_test.go",
	"keyboard_test.go",
	"livecheck_test.go",
	"openlist_test.go",
	"recipescreen_test.go",
	"refusaladdress_test.go",
}

var guardFileName = regexp.MustCompile(`[A-Za-z0-9_]+_test\.go`)

// justificationsHeading is where the verdict table ends and the paragraphs
// begin in REGRESSION.md.
const justificationsHeading = "\n## Uzasadnienia"

// citedInTables is every guard file named inside a table row.
//
// Table rows rather than the whole file, because both documents also mention
// guard files in prose, and prose is where somebody explains a thing rather
// than claims it is covered. The row is the claim.
func citedInTables(body string) map[string]bool {
	found := map[string]bool{}
	for _, line := range strings.Split(body, "\n") {
		row := strings.TrimSpace(line)
		if !strings.HasPrefix(row, "|") || !strings.HasSuffix(row, "|") {
			continue
		}
		for _, name := range guardFileName.FindAllString(row, -1) {
			found[name] = true
		}
	}
	return found
}

func TestEveryGuardCitedInTheSummaryIsJustifiedInRegression(t *testing.T) {
	root := repoRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "docs", "REGRESSION.md"))
	if err != nil {
		t.Logf("SKIPPED: REGRESSION.md is not here, so nothing was compared. "+
			"The internal documents are excluded from the repository, so this check only "+
			"runs on a machine that has them. (%v)", err)
		return
	}

	// The document is two halves: the verdict table, then the paragraphs. The
	// split is asserted rather than assumed - a document without the heading
	// is one this guard cannot read, not one it may pass - and so is the table
	// having rows to compare, because two empty sets agree about everything.
	summaryBody, detailBody, ok := strings.Cut(string(body), justificationsHeading)
	if !ok {
		t.Fatalf("REGRESSION.md has no heading %q, so the verdict table and the paragraphs cannot be told apart", strings.TrimSpace(justificationsHeading))
	}
	cited := citedInTables(summaryBody)
	justified := citedInTables(detailBody)
	if len(cited) == 0 {
		t.Fatal("the verdict table of REGRESSION.md cites no guard file at all, so this guard has nothing to compare")
	}

	allowed := map[string]bool{}
	for _, name := range notYetJustified {
		allowed[name] = true
	}

	var unjustified, stale []string
	for name := range cited {
		if !justified[name] && !allowed[name] {
			unjustified = append(unjustified, name)
		}
	}
	// A name excused here that is no longer cited has either been justified or
	// has left the table, and either way the excuse outlived its reason.
	for _, name := range notYetJustified {
		if !cited[name] || justified[name] {
			stale = append(stale, name)
		}
	}
	sort.Strings(unjustified)
	sort.Strings(stale)

	for _, name := range unjustified {
		t.Errorf("the verdict table of REGRESSION.md cites %s as evidence and the paragraphs below say nothing about it.\n"+
			"Reason: a verdict of JEST with no paragraph behind it reads as an assurance, which is\n"+
			"what the header of REGRESSION.md warns against.\n"+
			"What to do: write what that guard defends, from its commits and its code rather than\n"+
			"from its name - or add it to notYetJustified and say so out loud.", name)
	}
	for _, name := range stale {
		t.Errorf("%s is excused in notYetJustified and no longer needs to be.\n"+
			"Reason: it is either justified now or no longer cited, so the excuse outlived its\n"+
			"reason - and a list nobody prunes is where this drift would hide next.\n"+
			"What to do: take it off the list.", name)
	}

	// Both directions and the files themselves, because a table naming a guard
	// that is gone is the same defect one step further on.
	for _, set := range []map[string]bool{cited, justified} {
		for name := range set {
			if _, err := os.Stat(filepath.Join(root, "internal", "guard", name)); err != nil {
				t.Errorf("a table names %s and there is no such guard: %v", name, err)
			}
		}
	}

	// Said rather than asserted. A paragraph whose file the verdict table does
	// not cite is not a defect - plenty of rows carry their evidence in the
	// paragraph and leave the table's last column empty - but the number is
	// worth seeing, because it is the same drift facing the other way.
	var onlyJustified []string
	for name := range justified {
		if !cited[name] {
			onlyJustified = append(onlyJustified, name)
		}
	}
	sort.Strings(onlyJustified)
	t.Logf("%d guard file(s) cited in the verdict table, %d justified in the paragraphs, %d still excused",
		len(cited), len(justified), len(notYetJustified))
	if len(onlyJustified) > 0 {
		t.Logf("justified but not cited in the summary, which is the same drift the other way: %v",
			onlyJustified)
	}
}
