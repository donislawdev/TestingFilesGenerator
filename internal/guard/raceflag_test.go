//go:build !race

package guard

// raceEnabled says whether this binary was built with the race detector.
//
// It exists for exactly one guard: the object count in resources_test.go. The
// detector allocates on its own account, so a count taken under it is not the
// same measurement as a count taken without, and comparing either against one
// ceiling asks the number to mean two things.
//
// Two files rather than a runtime lookup, because the build tag is the only
// thing that knows. Go offers no way to ask at run time.
const raceEnabled = false
