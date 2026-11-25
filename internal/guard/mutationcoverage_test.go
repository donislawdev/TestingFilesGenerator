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
// internal/guard/mutation/entries.py. Kept by hand means somebody eventually does not add
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
	// A pipe exists only off Windows, so this guard compiles only there, and
	// the mutation runner runs on the Windows machine the project is built on
	// - the substitution would be applied and the test would not exist to
	// catch it. It runs on every Linux and macOS runner of the CI. The two
	// breaks it is for - asking IsDir where IsRegular stands in
	// core.OpenRegular, and an open without O_NONBLOCK in core/openregular_unix.go.
	// The first was read, not run (2026-09-29). The second was run by hand in
	// a Linux container on 2026-09-30, through the tool rather than through
	// this test: without the flag a pipe was still waited on after five
	// seconds, and with it refused at once.
	"TestAToolNeverWaitsOnAPipe": true,
	// A directory entry costs one tar block and nothing more.
	//
	// There is no line of ours under this one to break. It asserts what
	// archive/tar does with a directory header, and that number is what the
	// tar size arithmetic is built on - so the guard is here to catch the
	// standard library moving, which is the one thing a substitution in this
	// repository cannot simulate. The five guards that came with it on
	// 2026-09-01 are proven by mutation instead.
	"TestADirectoryEntryCostsExactlyOneTarBlock": true,
	// The property belongs to the USTAR header rather than to anything here:
	// every field of it is fixed width, so the mode and the owner are written
	// into space already paid for whatever they say. There is no line of ours
	// to break that would make recording an owner cost bytes - a wrong mode
	// still costs nothing, and the guard beside it catches that. What this one
	// would catch is a FUTURE change: somebody swapping the declared tar format
	// for one that writes extended header blocks, which is exactly the kind of
	// change that looks harmless and moves every archive's size.
	"TestRecordingAnOwnerCostsNoBytes": true,
	// The property is the ABSENCE of an override, and no substitution can
	// remove code nobody wrote. It is on this list rather than the one below
	// because it is genuinely unproven: it drives the box directly, so it would
	// catch that box being replaced by something that is not a text box, and it
	// would NOT catch a wrapper put in front of one and stealing the presses.
	"TestThreeClicksInABoxSelectWhatIsInIt":       true,
	"TestAFreshRunIntoAnEmptyDirectoryStillWorks": true,
	// One menu, two widths, measured at two moments: the cause they were
	// written for was the toolkit putting its placeholder into a menu at its
	// first draw, and that is closed at the source since 2026-10-06 -
	// parts.NewChooser gives every menu the window's placeholder, held by
	// TestAMenuIsOneWidthBeforeAndAfterItIsDrawn. Their mutation, measuring the
	// placeholder in parts.menuWidth, scored HOLE on every Sunday run, and
	// rightly: measured on 2026-10-19 it widens every format menu from 154.73
	// to 180.23 px on every screen and in every row at once. With the
	// placeholder taken out of NewChooser as well, both stayed green that day,
	// so no pair of breaks here reaches them either.
	"TestOneSettingIsOneMenuWidthOnEveryScreen":   true,
	"TestEveryRowOfATableStandsUnderTheSameNames": true,
	// The property is a package NAME, and no substitution in the tree can
	// move a package without breaking the build. Every mutation that reaches
	// the constant it reads also reddens the size guards, which the runner
	// says out loud - so naming one here would be a proof of nothing.
	"TestTheArchiveEntryFormatSortsBeforeTheArchive":                  true,
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
	"TestGeneratingTwiceGivesTheSameBytes":                            true,
	"TestLayeringHoldsForEveryPackage":                                true,
	"TestNoNetworkImports":                                            true,
	"TestNoProcessExecutionInLowerLayers":                             true,
	"TestNothingIsEverWrittenOverInSilence":                           true,
	"TestOracleStaysOutOfProductionCode":                              true,
	"TestPlanningTenThousandFilesStaysCheap":                          true,
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

var (
	// A guard takes *testing.T. TestMain takes *testing.M and is how go test
	// starts the package (guardpart_test.go), not a guard of anything.
	testDeclaration = regexp.MustCompile(`(?m)^func (Test\w+)\(\w+ \*testing\.T\)`)
	// Either quote: Python writes a string in single quotes as readily as in
	// double ones, and an entry written by repr() comes out in single. With
	// only the double quote read, nine of the guards this reported as unproven
	// on 2026-10-14 had entries it could not see (O280). A pair of the same
	// quote, each in its own alternative: two classes side by side also read
	// 'TestFoo" out of the middle of a longer string (review of #185).
	mutationTarget = regexp.MustCompile(`"(Test\w+)"|'(Test\w+)'`)
)

// mutationTargets is every guard name the mutation list holds as a whole
// string, in either quote.
func mutationTargets(body string) map[string]bool {
	named := map[string]bool{}
	for _, m := range mutationTarget.FindAllStringSubmatch(body, -1) {
		named[m[1]+m[2]] = true
	}
	return named
}

// mutationEntries is the list of mutations, relative to the root.
const mutationEntries = "internal/guard/mutation/entries.py"

func TestEveryGuardIsEitherProvenByMutationOrListedAsNotProven(t *testing.T) {
	// The canary, before the list is looked for, so a fresh checkout runs it
	// too: both quotes read, a quote of one kind closed by the other not.
	sample := `("a", "f.go", "x", "y", "TestDouble"), ('b', 'f.go', 'x', 'y', 'TestSingle'), "'TestMixed"`
	if got := mutationTargets(sample); len(got) != 2 || !got["TestDouble"] || !got["TestSingle"] {
		t.Fatalf("the mutation list is read wrongly: %v from %s, and TestDouble and TestSingle alone were in it", got, sample)
	}

	root := repoRoot(t)

	// The list is tracked since 2026-10-15. Before that it lived in a
	// directory outside the repository, this guard returned early on every CI
	// run, and eight guards gathered there with nothing breaking them. A list
	// that is missing now is a fault, not a reason to look away.
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(mutationEntries)))
	if err != nil {
		t.Fatalf("the mutation list could not be read, so no guard was compared with it: %v", err)
	}

	named := mutationTargets(string(body))
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
				"  Write a mutation for it in "+mutationEntries+", or put it on provenByProbe with what you broke, or on notProvenByMutation to say out loud that it is unproven.\n"+
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
			t.Errorf("%s aims at %s and no such guard exists - that mutation proves nothing", mutationEntries, name)
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
