package guard

import (
	"math"
	"testing"
)

// Two axes the size ceilings are blind to, added 2026-08-27.
//
// Length and branching do not move together. A hundred lines of straight line
// setup reads from top to bottom. Forty lines with eight branches inside one
// another has to be held in the head all at once, and that is where a case
// gets missed. A nine argument signature can be four lines of plain assignment
// and passes every ceiling in codeshape_test.go without a murmur.
//
// Both numbers are OUR measurement, not the defaults the tools ship with. The
// usual default for arguments is five, which would have been red on five
// signatures the day it went in - and a ratchet that arrives red gets raised
// to make it pass, which is how a ratchet becomes a rubber band.
const (
	worstComplexity = 22 // internal/recipe/target.go rawTarget.resolveSize
	mostArguments   = 9  // internal/cli/cleanup.go applyCleanup

	// Where each axis counts as on its way to the ceiling. Three quarters for
	// the two that have room to run, and an ABSOLUTE number for depth.
	//
	// Depth is absolute because a percentage needs a range to be a percentage
	// OF, and depth here runs 0 to 4. Three quarters of 4 is 3, which happens
	// to be right today - but the moment the ceiling dropped to 3 the band
	// would become "2 or more" and the count would jump from 54 into the
	// hundreds. A band that reshapes itself under the thing it watches says
	// nothing.
	crowdingComplexity = 17
	crowdingArguments  = 7
	crowdingDepth      = 3

	// Lowered from 5 on 2026-09-05: splitting the verify loop into compare and
	// claimedPaths took one function out of the band. The ratchet only tightens.
	crowdedComplexity = 4
	crowdedArguments  = 5
	// 53 until 2026-08-29, when TIFF arrived. The function that took it to 54
	// is tiff.chooseSize, and it is the same shape as bmp.chooseSize because
	// the two formats do the same arithmetic - the picture is grown to fill
	// the request, so one branch handles both dimensions named, one handles
	// either, and one handles neither. Flattening it in TIFF alone would make
	// two functions that answer the same question look different, which costs
	// more than the depth does.
	crowdedDepthFunctions = 52

	// An axis this set does not watch. crowding() asks n >= band, so nothing
	// reaches it.
	noBand = math.MaxInt
)

func TestNothingBranchesOrTakesMoreThanWhatAPersonCanFollow(t *testing.T) {
	m := measureTree(t)
	if m.files == 0 {
		t.Fatal("the scan read no file - this guard would pass against any ceiling ever set")
	}

	if m.biggestCyclo > worstComplexity {
		t.Errorf("%s has %d decision points and the ceiling is %d - pull a branch out into its own function",
			m.biggestCycloName, m.biggestCyclo, worstComplexity)
	}
	if m.biggestArgs > mostArguments {
		t.Errorf("%s takes %d arguments and the ceiling is %d - split the call rather than raising the number",
			m.biggestArgsName, m.biggestArgs, mostArguments)
	}
	if m.crowdedCyclo > crowdedComplexity {
		t.Errorf("%d function(s) reach %d decision points and the cap is %d - simplify one before adding another",
			m.crowdedCyclo, crowdingComplexity, crowdedComplexity)
	}
	if m.crowdedArgs > crowdedArguments {
		t.Errorf("%d signature(s) take %d arguments or more and the cap is %d",
			m.crowdedArgs, crowdingArguments, crowdedArguments)
	}
	if m.crowdedDepth > crowdedDepthFunctions {
		t.Errorf("%d function(s) nest %d deep or more and the cap is %d - flatten one before adding another",
			m.crowdedDepth, crowdingDepth, crowdedDepthFunctions)
	}

	t.Logf("worst branching %d (%s), most arguments %d (%s), crowding: %d branching, %d signatures, %d deep",
		m.biggestCyclo, m.biggestCycloName, m.biggestArgs, m.biggestArgsName,
		m.crowdedCyclo, m.crowdedArgs, m.crowdedDepth)
}

// The same second half every other ceiling here has: the number has to BE the
// measurement. The depth crowd count is the one that earns this most - the
// ceiling on the single deepest function was already exact, and 53 functions
// sitting one level under it were invisible to every guard in the package.
func TestTheBranchingCeilingsAreTodaysMeasurementAndNotALooserNumber(t *testing.T) {
	m := measureTree(t)
	if m.files == 0 {
		t.Fatal("the scan read no file - this guard would pass against any number ever set")
	}

	for _, a := range []struct {
		what           string
		ceiling, found int
		where          string
	}{
		{"worstComplexity", worstComplexity, m.biggestCyclo, m.biggestCycloName},
		{"mostArguments", mostArguments, m.biggestArgs, m.biggestArgsName},
		{"crowdedComplexity", crowdedComplexity, m.crowdedCyclo, "in the band"},
		{"crowdedArguments", crowdedArguments, m.crowdedArgs, "in the band"},
		{"crowdedDepthFunctions", crowdedDepthFunctions, m.crowdedDepth, "in the band"},
	} {
		if a.ceiling == a.found {
			continue
		}
		t.Errorf("%s is %d and the measurement is %d (%s) - move it to %d.\n"+
			"  A number standing above what it measures lets the next arrival in without a word.",
			a.what, a.ceiling, a.found, a.where, a.found)
	}
}
