package window

// The two values a work screen arrives with, in one place because two screens
// now have to arrive with the same ones.
//
// The single batch screen has opened with a name and a size typed in since
// there was a screen: press Generate and something happens. The batch screen
// opened with both of them empty and a red star on each, so the same button
// under the same mark did two different things depending on which tab somebody
// was on - Generate works on the first screen and refuses on the third. A
// tester learns the first screen and is turned down by the third.
//
// Only the two settings a run REFUSES when they are empty are filled in, and
// that line is where the fix stops. Everything else on the batch screen keeps
// its placeholder and stays unstated, which is O109 and untouchable rule 5: a
// value typed in is a value stated, and a form whose every box arrives
// carrying one can never say "I did not state this". How many files, the kind
// of case, the expectation and the reason are all settings a recipe is
// allowed to leave out, and they still are.
//
// A batch name of "files" is also why only the FIRST batch is filled. Two
// batches with one name is a refusal the recipe reader already words, so
// handing somebody a second batch carrying the first one's name would be a
// form that has to be repaired before it can be used - the same reason
// duplicateBatch leaves the copy's name empty.
const (
	// startingBatchName anchors the seeds of the batch and names the files, so
	// it has no default anywhere in the engine and a run without one is
	// refused. This is the window offering a first one, not a default.
	startingBatchName = "files"
	// startingSize is a size every format in the registry can produce - the
	// largest minimum is a few thousand bytes - so a screen that opens on any
	// format opens on a form that would run.
	startingSize = "10mb"
)
