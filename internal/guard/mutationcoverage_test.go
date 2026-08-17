package guard

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// A guard nobody ever broke on purpose is a guess about what it catches.
//
// Mutation is what turns "this test would be red if I broke that" from a
// sentence into a fact, and the list of mutations is kept by hand in
// tools/mutate/mutate.py. Kept by hand means somebody eventually does not add
// one - and this is not hypothetical. Two rows of the regression surface said a
// locality guard was proven by mutation and no entry named one. Found by this
// test on 2026-08-01, not by reading.
//
// So the rule is: every guard here is either named by a mutation or written
// down below as not proven. Both are fine. What is not fine is not knowing
// which, because a guard assumed proven gets trusted like one.
//
// The list is a statement of fact and carries no per test excuses. Inventing a
// reason for each would be prose nothing checks, and it would read as
// justification for a state that is meant to shrink.

// notProvenByMutation are the guards no mutation names today.
//
// Taking one off this list means writing a mutation that reddens it. Adding one
// means saying out loud that a guard is unproven. The list should only ever get
// shorter.
var notProvenByMutation = map[string]bool{
	"TestAFreshRunIntoAnEmptyDirectoryStillWorks":                     true,
	"TestCleanupNeverReachesOutsideTheDirectory":                      true,
	"TestVerifyNeverReachesOutsideTheDirectory":                       true,
	"TestAKeyThisBuildCannotHonourSaysSoRatherThanBeingIgnored":       true,
	"TestARunTooBigForTheDiskIsRefusedBeforeTheFirstByte":             true,
	"TestASizeThatIsNotAWholeByteIsRefused":                           true,
	"TestASkippedLabelIsVisibleInTheManifest":                         true,
	"TestATypoInAPropertyReachesTheUser":                              true,
	"TestAnArchiveDeclaringItsContentsNeedsNoSizeAndTheDryRunIsExact": true,
	"TestAnImageTooLargeToHoldIsRefused":                              true,
	"TestAnInterruptedRunLeavesNoPartialFileAndStillWritesAManifest":  true,
	"TestAnUnknownPropertyIsRefused":                                  true,
	"TestCommandLineIsAsciiOnly":                                      true,
	"TestDryRunWritesNothingAtAll":                                    true,
	"TestEveryEndingUsesACodeFromTheTable":                            true,
	"TestEveryFormatDeclaresTheFullSet":                               true,
	"TestGeneratingTwiceGivesTheSameBytes":                            true,
	"TestLayeringHoldsForEveryPackage":                                true,
	"TestNoNetworkImports":                                            true,
	"TestNoProcessExecutionInLowerLayers":                             true,
	"TestNonsenseSizesAreRefusedWithAUsefulMessage":                   true,
	"TestNothingIsEverWrittenOverInSilence":                           true,
	"TestOracleStaysOutOfProductionCode":                              true,
	"TestPlanningTenThousandFilesStaysCheap":                          true,
	"TestSizeBelowTheMinimumIsRefused":                                true,
	"TestSizesCountIn1024s":                                           true,
	"TestStandardLibraryOutputHasNotDrifted":                          true,
	"TestTextInTheRepositoryIsAsciiOnly":                              true,
	"TestTheFormatDocumentAgreesWithTheRegistry":                      true,
	"TestTheFreeSpaceGuardEndsWithItsOwnCode":                         true,
	"TestTheSameRecipeTwiceGivesTheSameFiles":                         true,
	"TestTheSameSeedGivesTheSameBytes":                                true,
	"TestTheTwoSpellingsAgree":                                        true,
	"TestTwoFilesCannotHeadForOneName":                                true,
	"TestVerifyAgainstAMissingDirectoryIsAReadFailureNotAMismatch":    true,
	"TestVerifyJSONCarriesTheDifferencesAndKeepsStdoutClean":          true,
	"TestVerifyMatchesADirectoryItJustGenerated":                      true,
	"TestVerifyOnAManifestClaimingNoFilesSaysThereWasNothingToCheck":  true,
}

// provenByProbe are the guards the mutation runner cannot express, broken on
// purpose by hand instead. Each entry says what was broken and what happened.
//
// Two reasons a guard lands here, and they are different:
//
//   - there is no product code underneath it. It reads documents and compares
//     lists, so a substitution in a .go file never reaches it.
//   - the substitution that would break it is not a literal. Entries in
//     mutate.py are read with ast.literal_eval, so a break that needs ninety
//     generated lines cannot be written as one.
//
// A separate list rather than a note in the one above, because "unproven" and
// "proven another way" are different states and lumping them together would
// send a later session to re-prove what is already proven.
var provenByProbe = map[string]string{
	"TestTabbingReachesTheControlsAndSaysInWhatOrder": "broken by hand on 2026-08-13: parts.Row was made to Hide the row it builds, and the guard went red naming four controls - " +
		"the size box, the seed box and two explanation buttons - then green again when it was put back. " +
		"That break is the real failure mode rather than an invented one. Visible() on a child answers for the CHILD, so a control inside a hidden container still reports itself visible " +
		"while the focus chain, which walks the visible tree, steps straight past it. A section quietly hidden is exactly how a screen stops being operable from the keyboard without looking any different in a screenshot. " +
		"Not a mutation entry because no substitution of one piece of text for another produces that state today: the failure lives in the disagreement between two ways of asking what is visible, " +
		"and nothing in the tree says Hide for a substitution to aim at.",

	"TestTheTaskbarIconIsTheSameDrawingAsTheWindowIcon": "checked 2026-08-13 with tools/probes/exe-icon.ps1, run on both binaries. " +
		"It reported 7 of 7 icon images inside tfg-gui.exe and 0 of 7 inside tfg.exe, which is the right answer for a command line with no window. " +
		"The defect it was written for was reported from a real taskbar the same day: app.SetIcon was in place and working, the window wore our chickpea, " +
		"and the taskbar wore the placeholder Windows draws for a binary with no icon resource - because that one is compiled into the exe and had never been. " +
		"Every guard was green because every guard was asking the toolkit. " +
		"A probe rather than a mutation because what this guard reads is three committed binary artefacts, and there is no product code beneath it " +
		"for a substitution in a .go file to break. The staleness it really watches for - a redraw that regenerates the PNG and forgets the ICO - " +
		"was confirmed by hand: emptying the ICO makes it red, and so does leaving an older .syso in place.",

	"TestReplacingAFileKeepsTheModeItHad": "checked 2026-08-12 with tools/probes/atomic-replace, run on Windows and on Linux through WSL2. " +
		"The version this replaced wrote the temporary copy with 0644 and renamed it over the original, and a rename moves the file it renames - " +
		"so a recipe somebody had made private at 0600 came back at 0644, readable by everyone on the machine. Measured on Linux, on the code that was shipping. " +
		"It cannot be a mutation entry because the guard SKIPS on Windows, which is where the mutation runner lives: Windows has no permission bits, " +
		"so the question cannot be put there at all and a substitution would be reported NOT CAUGHT for a guard that is working. " +
		"The probe is the honest instrument here, because it is the one that can be run where the answer exists.",

	"TestNoNumberATestPrintsIsCopiedIntoTheProse": "checked 2026-08-05 by planting a document in docs/ carrying one bad number per rule - " +
		"a D1 parity pair, an identifier count, a link count, a mutation breakdown in three spellings, a guard count and a coverage figure. " +
		"All eight were named, and deleting the file put the run back to green. " +
		"A probe rather than a mutation because what this guard reads is prose, not product code, so there is nothing beneath it for a substitution to break - " +
		"and breaking the guard itself would be mutating the judge. " +
		"The first probe also earned its keep: six planted numbers, only three caught, because the patterns wanted Polish diacritics " +
		"and these documents carry passages typed without them. The patterns now accept both.",

	"TestNothingIsQuietlyCreepingTowardsTheCeiling": "checked 2026-08-05 by hand. A function of seventy lines of code was dropped into internal/core, the count went from eleven to twelve against a cap of eleven, and the guard went red naming it. Removing the file put it back to green. It cannot be a mutation entry because the break is sixty extra lines rather than one substitution, and a mutation aimed at a function that happens to sit just under the threshold today would go stale at the first refactor that moves it.",

	"TestEveryTextFormatIsValidUTF8": "checked 2026-08-02 with tools/probes/probe-utf8-filler.py, which swaps in a vocabulary of Polish words and sweeps 304 sizes. " +
		"Before core.AppendFiller cut on a character boundary: 304 files, every one the right size, 86 of them carrying invalid UTF-8. After: 0. " +
		"It was a mutation until the fix landed, and the runner then reported it NOT CAUGHT - which is the fix being proven rather than a hole. " +
		"Breaking this guard now needs two changes at once, a non ASCII vocabulary and a byte cut, and that is not one substitution.",

	"TestNoFunctionOrFileHasGrownPastWhatAPersonCanFollow": "checked 2026-08-02 three ways, with tools/probes/probe-shape.py: a function grown past 80 lines of code went red, " +
		"a file grown past 550 went red, and the same function padded with 90 lines of COMMENT stayed green - so the ceiling is on code and not on explaining. " +
		"A probe rather than a mutation because the substitution is ninety lines long and mutate.py entries have to be literals.",

	"TestEveryIdentifierAReferencePointsAtExists": "checked 2026-08-01 three ways: a reference to a decision that does not exist, " +
		"a number removed from the middle of a range, and the summary table in CLAUDE.md announcing a range the document no longer defines. All three went red.",

	"TestEveryGuardIsEitherProvenByMutationOrListedAsNotProven": "checked 2026-08-01 three ways: a guard added with neither a mutation nor an entry, " +
		"an entry naming a guard that was renamed away, and a guard left on the list after it gained a mutation. All three went red.",

	"TestEveryLinkBetweenDocumentsLeadsSomewhere": "checked 2026-08-01 by adding a link to a document that does not exist. It went red.",

	"TestTheObservationsAreNumberedOnceEach": "checked 2026-08-02 two ways: one number used twice, and a number cut out of the middle leaving a hole. " +
		"Both went red. Written the same day the rule it guards was broken - two rows were appended reusing 25 and 26, and nothing noticed.",
}

var (
	testDeclaration = regexp.MustCompile(`(?m)^func (Test\w+)\(`)
	mutationTarget  = regexp.MustCompile(`"(Test\w+)"`)
)

func TestEveryGuardIsEitherProvenByMutationOrListedAsNotProven(t *testing.T) {
	root := repoRoot(t)

	// The mutation list lives in tools/, which is excluded from the repository,
	// so a fresh checkout has nothing to compare against. That skips out loud
	// rather than passing quietly.
	body, err := os.ReadFile(filepath.Join(root, "tools", "mutate", "mutate.py"))
	if err != nil {
		t.Logf("SKIPPED: tools/mutate/mutate.py is not here, so nothing was compared. "+
			"That directory is excluded from the repository, so this check only runs on a machine that has it. (%v)", err)
		return
	}

	named := map[string]bool{}
	for _, m := range mutationTarget.FindAllStringSubmatch(string(body), -1) {
		named[m[1]] = true
	}
	if len(named) == 0 {
		t.Fatal("the mutation list names no test at all - this guard would pass without checking anything")
	}

	guards := guardTests(t)
	if len(guards) == 0 {
		t.Fatal("no guard was found - this guard would pass without checking anything")
	}

	var proven, probed, unproven int
	for _, name := range guards {
		_, byProbe := provenByProbe[name]
		switch {
		case named[name] && notProvenByMutation[name]:
			t.Errorf("%s has a mutation naming it and is still on the not proven list - take it off, that is what shrinking the list looks like", name)
		case byProbe && (named[name] || notProvenByMutation[name]):
			t.Errorf("%s is on two lists at once - it is proven by mutation, by probe, or by neither", name)
		case named[name]:
			proven++
		case byProbe:
			probed++
		case notProvenByMutation[name]:
			unproven++
		default:
			t.Errorf("%s is a guard nothing breaks on purpose.\n"+
				"  Write a mutation for it in tools/mutate/mutate.py, or put it on provenByProbe with what you broke, or on notProvenByMutation to say out loud that it is unproven.\n"+
				"  Pick a break that leaves the size and the exit code right, or the mutation only re-proves what other guards already catch.", name)
		}
	}

	// A name on either list that no longer exists points at a test somebody
	// renamed. Left alone the mutation runner reports it as uncaught, which
	// reads as a hole in the product rather than as a stale string.
	exists := map[string]bool{}
	for _, name := range guards {
		exists[name] = true
	}
	for name := range notProvenByMutation {
		if !exists[name] {
			t.Errorf("notProvenByMutation names %s and no such guard exists - it was renamed or removed", name)
		}
	}
	for name, how := range provenByProbe {
		if !exists[name] {
			t.Errorf("provenByProbe names %s and no such guard exists - it was renamed or removed", name)
		}
		if strings.TrimSpace(how) == "" {
			t.Errorf("%s is on provenByProbe with no word about what was broken, which is the same as not being proven", name)
		}
	}
	for name := range named {
		if !exists[name] && strings.HasPrefix(name, "Test") {
			t.Errorf("tools/mutate/mutate.py aims at %s and no such guard exists - that mutation proves nothing", name)
		}
	}

	t.Logf("guards: %d proven by mutation, %d proven by probe, %d written down as not proven, %d in total",
		proven, probed, unproven, len(guards))
}

// guardTests lists every test declared in this package, sorted so a failure
// always reports the same one first.
func guardTests(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading this package: %v", err)
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		body, err := os.ReadFile(e.Name())
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		for _, m := range testDeclaration.FindAllStringSubmatch(string(body), -1) {
			names = append(names, m[1])
		}
	}
	sort.Strings(names)
	return names
}
