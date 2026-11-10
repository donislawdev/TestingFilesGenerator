package recipe

// Notes is what a target says ABOUT its files rather than how to make them:
// none of it takes part in the seed, the size or the name on disk, so any of
// it can be reworded without moving a byte of any file (D11).
//
// A type of its own, with the same fields in the same order as the engine's
// type of that name, so that a surface turns one into the other in a single
// conversion. See engine.Notes for what that closes.
//
// Every one of them is a plain recipe key rather than something a preset
// passes around the recipe. PR5 says an ejected preset is an ordinary recipe,
// and a field only a preset could set would make the ejected copy produce
// something different from the preset it came from.
type Notes struct {
	// Group names the class of case these files belong to, and it reaches the
	// manifest so a test can assert about a whole class at once rather than
	// file by file - "every file in extension-content-mismatch was rejected".
	Group string
	// Purpose says in words what these files are and why they are in the
	// set, for the person who opens the directory afterwards. It reaches the
	// manifest and the instructions written beside it.
	Purpose string
	// SendAs is the name to hand these files to the system under test under,
	// when that is not the name they have on disk, and empty when the recipe
	// does not say. It is taken exactly as written - no placeholder is read in
	// it, so every file of the target carries the same one - and it never
	// names anything on the disk of the machine the recipe runs on.
	SendAs string
	// Expected is what the system under test should do with these files, and
	// ExpectedReason is why, from the closed list in docs/MANIFEST.md.
	Expected       string
	ExpectedReason string
}
