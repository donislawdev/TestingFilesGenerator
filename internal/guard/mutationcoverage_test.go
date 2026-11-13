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

// provenByProbe are the guards the mutation runner cannot express, broken on
// purpose by hand instead. Each entry says what was broken and what happened.
//
// Two reasons a guard lands here, and they are different:
//
//   - there is no product code underneath it. It reads documents and compares
//     lists, so a substitution in a .go file never reaches it.
//   - the substitution that would break it is not a literal. Entries in
//     entries.py are read with ast.literal_eval, so a break that needs ninety
//     generated lines cannot be written as one.
//
// A separate list rather than a note in the one above, because "unproven" and
// "proven another way" are different states and lumping them together would
// send a later session to re-prove what is already proven.
var provenByProbe = map[string]string{
	// Broken once each on 2026-11-12 while they were written, one change at a
	// time and restored (docs/DAMAGE-SETTINGS-LABEL-2026-11-12.md). The first
	// also carries a canary of its own check.
	"TestEveryDamageSettingItsDeclarationForbidsIsRefusedWithoutARecipe": "its own canary - a plan that went through and a refusal about the first entry of the chain, both rejected by its check - and broken on 2026-11-12 " +
		"by taking damage.Chain.Check out of engine.prepareDamage: red, every value went through.",
	"TestADamageSettingOutsideItsRangeIsRefusedUnderItsBox": "broken on 2026-11-12 by taking damage.Chain.Check out of engine.prepareDamage: red, the window wrote the file and the box said nothing.",
	"TestADamagedFileStatesItsSizeOnDiskAndItsDamage": "its own canary - the label it was written against, naming the size before the damage and no damage, rejected by its check - and broken on 2026-11-12 " +
		"twice: Request.LabelSized ignoring OnDisk, and leaving the damage out. Red both times.",
	"TestATruncatedTextSaysHowMuchWasCut": "broken on 2026-11-12 twice: Request.LabelSized ignoring OnDisk, and leaving the damage out. Red both times.",
	"TestALongDamageLabelLeavesEveryFormatWhole": "broken on 2026-11-12 by letting damage.Chain.Label return the whole chain past LabelLimit: red, " +
		"its own reach check says forty damages no longer fit the limit (docs/REVIEW-234-2026-11-12.md).",
	"TestEveryAvifRungStillFitsInsideItsDeclaredCeilings": "its own reach check - no damaged label drawn is a failure - and broken on 2026-11-12 twice, with " +
		"the heaviest seeds: the first ceiling of 640x480 set back to 16389 (red, 17314 B) and the second to 18722 (red, 20024 B). " +
		"Without those seeds both stayed green (docs/REVIEW-234-2026-11-12.md).",
	"TestEveryFormatsMapIsTheOneItsStructureReadsBack": "its own canaries - the first boundary moved by a byte must not pass, a mark for bytes already written must be " +
		"reported, a sweep with no padding or with no padding on both sides of a content is a failure - and broken on 2026-11-12 by marking the png " +
		"padding a byte late: red, and by marking the picture of a png laid out for a cut a byte late: red at 8 KiB and at 64 KiB.",
	"TestACutFileSaysWhereTheCutFellAndWhatIsLeft": "its own canary - a cut named in the wrong role, and content claimed gone that is there, must not pass - " +
		"and broken on 2026-11-12 by reporting every cut in the content: red for the picture and the icon.",
	"TestATarGzLaidOutForACutComesToEverySizeAndReads": "its own canary - a stream built by hand, one and then two empty blocks of " +
		"fixed codes, must inflate to its bytes and read as those runs - its reach check, every frame of the marker met - and red on " +
		"2026-11-13 for a frame a byte short (the plan refuses at 65541 B) and for an empty block's BTYPE bit one place off " +
		"(compress/flate refuses at 65537 B), where the first passed the share and the map guards.",
	"TestATarGzCutNearItsSmallestComesToEverySize": "its reach check - the frame met, and without a label the size after its floor " +
		"that keeps the layout of an archive nothing cuts, which with a label is due never - and red on 2026-11-13 without the " +
		"case of one byte over the floor (an internal defect at 9757 B) and with that case taken for a label too (label true).",
	"TestACutAtTheSmallestSizeSaysHowLongTheContentWas": "its own canaries - a cut that does not state the content's length, and one stating a " +
		"byte less than the map runs, must not pass - its reach check - the ZIP packages at their smallest have no padding in front and are cut " +
		"in the content - and red on 2026-11-13 before the Office packages marked their parts with a length without a padding part in front: " +
		"docx at 610 B, xlsx at 863 B, pptx at 2405 B.",
	"TestACutTakesTheShareOfTheContentItIsAskedFor": "its own canaries - a cut that kept all its content must not pass for one that took half, and txt listed as " +
		"not cutting at the share must be reported - and broken on 2026-11-12 by never telling png where its cut falls: red in all six of its files.",
	"TestAGuardThatDrawsAfterALongPauseStillDraws": "its own canary - the box drawn after test.NewApp alone has to come out blank, or the pause " +
		"does not reach the state - and broken on 2026-11-12 by making readFyneClock do nothing: red, the box drawn after newTestApp was blank too.",
	"TestEveryGuardTakesItsApplicationWithTheClockRead": "its own canary - a source with test.NewApp and test.NewTempApp has to give both calls - and " +
		"a reach check on the number of guard files it read.",
	"TestTheRecordOfACutRunIsNeverLongerThanItsForecast": "its own reach check - a run with no cut in the padding is a failure - and broken on 2026-11-12 " +
		"by leaving the cut out of the forecast: red, a record of 9638 B told 8130 B.",
	"TestADamageLabelIsBoundedAndSaysHowManyItLeftOut": "its own canary - no chain up to 200 damages shortened is a failure - and broken on 2026-11-12 by letting damage.Chain.Label return the whole chain: red.",
	"TestADamagedMailCarriesItsLabelWholeInEveryTextEncoding": "broken on 2026-11-12 twice: eml writing the label raw in quoted-printable (red, the one damage label read back as bytesd), " +
		"and eml leaving a long label on one line (red, a line of 92 characters).",
	"TestEveryCharacterOfADamagedLabelCanBeDrawn": "its own canary - a character the font never had must not count as drawn - and broken on 2026-11-12 by taking = out of the picture font: red.",
	// What breaks it lives in a module, not in a file of this repository, so
	// no mutation of this tree reaches it - the dependency it was written
	// against is the break.
	"TestAnAVIFLargerThanOneAV1TileSurvivesItsReferenceTool": "broken on 2026-11-11 by gav1d v0.2.5, go.mod as it stood before v0.2.6: both pictures went red saying tile_start_and_end_present_flag is set in an OBU_FRAME, " +
		"the tile_info of several tiles read over the data of one. With v0.2.6 both read back as 2 tiles, gav1d's decoder read them whole and Pillow accepted them (docs/GAV1D-026-2026-11-11.md).",
	// Their canaries run inside them on every machine with python, which is
	// the proof the owner asked for first on 2026-11-11: a check that cannot
	// fail turns them red where they stand, with no entry to rewrite when the
	// code moves.
	"TestTheOTLPCheckRefusesMetricsThatDoNotAddUp": "its own canaries: six files of metrics broken one way each - a temporality, the empty negative buckets, a bucket, a date, a counter that falls, exemplars nobody asked for - " +
		"each refused with the reason that names it, beside two healthy files that pass. On 2026-11-11 one canary was refused for another reason and the guard went red, which is how it found that a count one more never reaches the check of the buckets.",
	"TestTheExponentialBucketsEndWhereTheirBoundsSay": "its own canary: the table with one entry a microsecond high and then a microsecond low, each refused at that entry alone, before the table itself is asked.",
	"TestEveryOTLPPitfallMeetsItsReferenceTool": "its own canary: a file of ids in capitals handed to the oracle as ids in base64, which the Collector reads whole where it was told to expect nothing back, and the oracle has to refuse it. " +
		"On 2026-11-11 the guard passed with the Collector 0.165.0 on Windows over 34 files, every pitfall in every kind of telemetry, in 124 s.",
	// Runs only in the CI job that has the Collector, so a mutation job without
	// one would meet a skip. What the guard adds to the structural check is the
	// oracle itself, and the oracle was handed three files it has to refuse.
	"TestEveryOTLPSettingSurvivesItsReferenceTool": "broken on 2026-11-11 by its input, through the same script in Docker with the Collector 0.165.0 (tools/probes/otlp/oracle-in-docker.py): a span with no status member, an enum written as its name and a line of 2 MiB. " +
		"The oracle said the first two came back different at the byte where the Collector writes its own spelling, and that 0 of 1 batches came back for the third. Three healthy files passed beside them.",
	// Reached only where the filesystem folds case and still keeps STRASSE.txt
	// apart from a sharp s spelling - NTFS does, the Linux the mutation job runs
	// on keeps every case apart and APFS joins the pair, so no CI mutation of
	// the branch can redden it.
	"TestARefusalNamesTheFileThatIsReallyInTheWay": "broken by hand on Windows on 2026-10-27: os.SameFile(here, there) in listing.holds (preflight.go) turned into os.SameFile(there, there), so the first spelling of a fold is named. " +
		"The guard went red saying the refusal named STRASSE.txt, and the file was put back and checked with md5sum -c.",
	// Three guards each held by two layers, so no single substitution can redden
	// them and each mutation of one layer scored a HOLE in the CI run of
	// 2026-10-15. Broken that day in a worker tree, one layer at a time and then
	// both: green, green, red, for all three.
	"TestAShortcutCannotStartASecondRunDuringTheFirst": "broken on 2026-10-15 in two places at once: the check on the button in pressIfLive (runactions.go) and the refusal of onGenerate while the state is occupied (run.go, since 2026-09-21). " +
		"With either one taken away alone the guard stayed green, because the other refused the second run. With both gone it went red saying the screen reports a run that wrote nothing, which is the second one.",
	"TestAFieldWiredBeforeTheScreenListensReportsOnce": "broken on 2026-10-15 in two places at once: WhenTypedIn walking the fields it already has, and chainOnce chaining again rather than pointing the one handler it put in (both in parts/fields.go). " +
		"With either alone the guard stayed green - a second wiring is pointed rather than chained, and without the walk nothing wires twice. With both it went red: typing into one box told the screen 2 times.",
	"TestEveryPresetRendersARecipeThatParsesBack": "broken on 2026-10-15 in two places at once: the whitelist in checkDistance (preset/sizeboundaries.go) and the refusal in recipe.Compose of a character a recipe cannot hold (refuseUnwritable). " +
		"With either alone the guard stayed green, 154 values tried. With both it went red: size-boundaries accepted spread=\"1\\rB\" and produced source the parser refuses. " +
		"The whitelist's other job, a space reaching a file name, is the mutation on TestEveryPresetEndingHasTheCodeItShould.",
	"TestARefusalShowsANameNobodyCanReadAsAnEscape": "broken on 2026-10-15 in three places at once: the escape of the two names in the refusal of a case collision (engine/names.go), the escape of the path in the refusal of a taken name (engine/errors.go), and the escape of the whole message on its way out of the command line (describeError in cli/errors.go). " +
		"With any one of them gone the guard stayed green, because the line is escaped either way. With all three it went red: the refusal of two names that collide printed U+200B raw. " +
		"The two escapes in the sentences are the mutations on TestTheWindowsWordsForARefusalShowANameNobodyCanReadAsAnEscape, where the window has no funnel to cover them.",
	"TestAFolderThatCannotBeListedIsRefusedWholeAndVerifyStillStopsAtIt": "broken on 2026-09-30 in a Linux container (golang:1.27-alpine, the repository read only, an ordinary user), through the command line rather than through this test, which cannot build there without the window's libraries. " +
		"With internal/audit/walk.go as it is, verify of a run with a folder nobody may list ended with 5 naming it. With the three lines that hand verify the first unreadable folder removed - the overlay put the mutated file in place - the same verify ended with 0 and said the directory matches. " +
		"A probe rather than a mutation entry because this guard skips on Windows, where the runner runs, and would score NOT CAUGHT about a healthy guard.",
	"TestAWalkStoppedHalfWayHandsBackNothingItFound": "broken on 2026-10-05 in the same kind of container (golang:1.27-alpine, the repository read only, user 1000), through a program that does what this guard does - the folder a locked, b/y after it, a whole walk to count its looks, then a walk cancelled at the last look. " +
		"With internal/audit/walk.go as it is, the cancelled walk handed back nothing it had found. With the three lines that turn a stopped walk into an empty Found replaced by \"return found, err\" - the overlay put the mutated file in place - it handed back the folder it could not list, which is what this guard reads as red. " +
		"A probe rather than a mutation entry for the reason the guard above is one.",
	"TestThePackageSourcesAreTrackedByGit": "broken by hand on 2026-09-25 and put back: git rm --cached on packaging/chocolatey/tools/chocolateyuninstall.window.ps1.in made it red, naming that file, and git add made it green again. " +
		"A probe rather than a mutation entry because what it reads is git's index, not the text of a file - no substitution in any file untracks one.",
	"TestAManifestCarryingACredentialIsWrittenForItsOwner": "broken by hand on 2026-09-06 and put back, because the mutation is expressible and the OBSERVATION is not - Windows has no permission bits, Go maps only the owner write bit onto its read only attribute, and this machine is the one the mutation runner runs on. Changed internal/manifest mode() from 0o600 to 0o666, cross compiled the guard binary for linux/amd64 and ran it in a debian container against the real repository: red, naming the case - \"a manifest with a password came out 0644 and should be 0600\" - while the two cases that must stay 0644 stayed green. A probe rather than a mutation entry because a runner on Windows would score this NOT CAUGHT about a healthy guard, which is the worst answer of the three. The half of this pair that runs everywhere is TestTheRecordAndTheRegistryAgreeOnWhatIsACredential, and that one has a mutation.",
	"TestTheEncoderSurvivesTheSizeThatCrashedItsAssembly":  "proved by tools/probes/avifasm on 2026-08-29, which is the sweep that found the fault in the first place. Run with the assembly, a 640x256 picture killed the process inside cflAcMain8AVX2 at av1/cfl_amd64.s:281 with an access violation - in two runs out of three, so it turns on what the heap looks like rather than on the input alone. One size out of 240 crashed. Run with the tag this project ships, 240 out of 240 encoded and the bytes were identical either way. A probe rather than a mutation entry because what would have to be broken is a BUILD FLAG, not a line of code: the substitution that removes the tag lives in .github/build-tags, and a run without it does not fail this guard, it takes the whole test binary down with it. That is loud, and it is the honest shape for a guard against memory read outside its buffer, but it is not something the runner can score.",
	"TestTheIconMacOSReadsCarriesEverySizeItIsAskedFor": "broken by hand on 2026-08-28, three ways, and put back byte for byte - the file is untracked in git, so the restore was checked by hash rather than by a clean diff. " +
		"Cutting the icp4 chunk out made it red naming that entry and the 16 px it holds, which is the failure that matters most: the file still opens, still shows an icon, and is missing the size a screen without Retina asks for. " +
		"Resizing the 1024 px picture to 900 made it red as well, which is what an upscale from the wrong master would look like. " +
		"Lying in the length field of the header made it red a third time, and that is the one a viewer would forgive and a reader that trusts the header would not. " +
		"A probe rather than a mutation entry because what it reads is a committed BINARY asset, not code: no substitution in a .go or .py file changes those bytes, " +
		"and the script that writes them - tools/appicon.py - is not run in CI at all, so mutating it would leave this guard green while proving nothing. " +
		"Apple's own iconutil was asked the same question on a real Mac on the day this went in and handed back all ten entries at these pixel sizes, " +
		"which is the half of the answer that cannot be asked again on a machine without macOS.",

	"TestEveryScriptAWorkflowRunsDirectlyIsExecutable": "broken by hand on 2026-08-28, in both directions, and put back byte for byte. " +
		"git update-index --chmod=-x on .github/scripts/make_app_bundle.sh made it red, naming the file, the mode it found and the command that fixes it. " +
		"Putting the bit back made it green again. " +
		"A probe rather than a mutation entry because what it reads is not text: the thing that can be wrong is a FILE MODE recorded in the git index, " +
		"and entries in entries.py substitute one string for another inside a file, which never touches a mode. " +
		"Worth having at all because this failure is invisible on the machine it is written on - Windows has no such bit and git bash ignores it, " +
		"so the script looks fine, passes every guard that reads it as text, and the FIRST release build stops on it with exit 126. " +
		"That is not hypothetical: make_app_bundle.sh went in as 100644 and this guard is what found it.",

	"TestEveryFakeHostComesFromTheConstructorThatJoinsIt": "broken by hand on 2026-08-27, in both directions it can fail, and put back byte for byte. " +
		"Turning one newFakeHost(t) in tabstrip_test.go back into the literal made it red, naming the file and the line. " +
		"The other direction matters more and was checked first: written as a plain constant, the search string in this guard is itself a hand built host, " +
		"and the guard reported ITSELF on its first run - so it is assembled from two pieces, which keeps its own file under the same rule as the rest. " +
		"It also refuses to pass when the literal appears nowhere at all, because a search that finds nothing reads exactly like a search that finds nothing wrong. " +
		"A probe rather than a mutation entry because there is no product code underneath: it reads the test files of this package, " +
		"so a substitution in a shipped .go file never reaches it.",

	"TestTheSummaryFitsItsCeilingAndHoldsNoState": "broken by hand on 2026-09-21 in all five directions it can fail - a dated STAN NA block in CLAUDE.md, a promised heading gone, " +
		"2 KB past the ceiling, STATE.md with a second \"Gdzie jestesmy\", STATE.md past its ceiling - red each time, green with the files put back byte for byte (cmp). " +
		"A probe rather than a mutation entry because there is no product code underneath: it reads CLAUDE.md and docs/STATE.md, neither of which is in the repository.",

	"TestEveryGuardCitedInTheSummaryIsJustifiedInRegression": "broken by hand on 2026-08-26, in all three directions it can fail, and put back byte for byte through the checked restore in tools/mutate/runner.py. " +
		"Taking keyboard_test.go off notYetJustified made it red, because CLAUDE.md cites that guard and REGRESSION.md still says nothing about it. " +
		"Adding verify_test.go to the list made it red the other way, which is the direction that matters most: a list nobody prunes is where this drift would hide next. " +
		"Naming a guard that does not exist made it red as well. Green again after each, and the run before them all was green, so none of the three was red for a reason that was already there. " +
		"A probe rather than a mutation entry because there is no product code underneath: it reads docs/REGRESSION.md (both halves of it since 2026-09-21), which is not in the repository, " +
		"so a substitution in a .go file never reaches it.",

	"TestARunIsStoppableFromTheMomentItStarts": "checked 2026-08-23 and again 2026-11-04 with tools/probes/run-stop-order.py, which reproduces the original defect faithfully: it moves the assignment of the handle " +
		"(r.stop then, r.work since the work got a type of its own) to after the goroutine that writes the files, builds, and runs the guard. Reported CAUGHT both times, and the file was restored. " +
		"A probe rather than a mutation entry because the defect is a MOVE of a statement past the goroutine, not a substitution of one piece of text for another, " +
		"and the entries in entries.py have to be literals. Deleting the handle would compile since 2026-11-04 and would only prove that it is assigned, not where.",

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
		"A probe rather than a mutation because the substitution is ninety lines long and the entries in entries.py have to be literals.",

	"TestEveryIdentifierAReferencePointsAtExists": "checked 2026-08-01 three ways: a reference to a decision that does not exist, " +
		"a number removed from the middle of a range, and the summary table in CLAUDE.md announcing a range the document no longer defines. All three went red.",

	"TestEveryGuardIsEitherProvenByMutationOrListedAsNotProven": "checked 2026-08-01 three ways: a guard added with neither a mutation nor an entry, " +
		"an entry naming a guard that was renamed away, and a guard left on the list after it gained a mutation. All three went red. " +
		"And 2026-10-14 (review of #185): the name read between two quote classes again, with an empty group kept for the second " +
		"capture. Red with the canary's own message. Not a mutation entry, because on the machine with tools/ this guard was " +
		"red that day before anything was broken, ten old findings, so a runner would have called any entry caught.",

	"TestEveryLinkBetweenDocumentsLeadsSomewhere": "checked 2026-08-01 by adding a link to a document that does not exist. It went red.",

	"TestTheObservationsAreNumberedOnceEach": "checked 2026-08-02 two ways: one number used twice, and a number cut out of the middle leaving a hole. " +
		"Both went red. Written the same day the rule it guards was broken - two rows were appended reusing 25 and 26, and nothing noticed.",

	"TestTheWindowBinaryDoesNotImportOpenGLAtLoadTime": "broken by hand on 2026-09-17 and put back byte for byte: the one line of the patched " +
		"Windows lookup in third_party/go-gl-gl/v2.1/gl/procaddr.go turned back into a direct call of wglGetProcAddress. " +
		"Red, naming opengl32.dll among the fifteen libraries the binary imported - resolved through the -lopengl32 the windowing " +
		"library links on the same line, which is the exact way the import would come back in practice. " +
		"A probe rather than a mutation entry because of what the runner would score, not because the substitution cannot be written: " +
		"the guard itself took 3.1 s, and the run took 3 min 56 s, because a change to the binding recompiles the binding, " +
		"the toolkit above it and the guard binary above that. The runner's ceiling is 120 s at one worker and 240 s at six, so every " +
		"entry touching that package would be CAPPED, which is neither caught nor a failure. The companion guard that CAN be scored, " +
		"TestTheOpenGLBindingIsThePinnedModulePlusExactlyThePatch, holds the copy to the published module plus the patch and has entries " +
		"that touch nothing the compiler reads.",
}

var (
	testDeclaration = regexp.MustCompile(`(?m)^func (Test\w+)\(`)
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
