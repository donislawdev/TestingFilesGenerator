package engine

// Notes is what a target says ABOUT its files rather than how to make them.
//
// Every field reaches the manifest, some reach the instructions beside it, and
// none reaches a file: none takes part in the seed, the size or the name on
// disk, so any of them can be changed without moving a byte (D11).
//
// One type rather than five fields of Target, and the reason is the two places
// that turn a recipe target into this one - the command line and the window,
// on the same layer, sharing nothing. Each copied these fields one by one, and
// a field added here and to one of the two lists was a setting the other
// surface dropped without a word. The recipe package holds a type of the same
// shape, and both surfaces now convert the whole of it, so a field added to
// one type and not the other stops the build.
type Notes struct {
	// Group names the class of case these files belong to, so a test can
	// assert about a whole class at once.
	Group string
	// Purpose is what these files are and why they are in the set, in words.
	// It reaches the manifest and the instructions beside it.
	Purpose string
	// SendAs is the name to hand these files to the system under test under,
	// when that is not the name they have on disk. Empty when nobody asked.
	//
	// It exists for the names no file system stores - one that climbs out of
	// a directory, an absolute path, a name with a character Windows refuses.
	// Such a name is still a test case: it travels in the request that uploads
	// the file, and the file itself keeps an ordinary name here.
	//
	// It is never joined to a directory, never opened and never compared with
	// a name on disk.
	SendAs string
	// Expected is what the system under test should do with these files, and
	// ExpectedReason is why, from the closed list in docs/MANIFEST.md.
	Expected       string
	ExpectedReason string
}
