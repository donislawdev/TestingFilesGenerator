//go:build !race

package guard

// raceEnabled says whether this binary was built with the race detector.
//
// It exists for guards whose measurement the detector changes. Most are about
// memory - the object count in resources_test.go, the bytes compared in
// manifestreadlimits_test.go. The detector allocates on its own account, so a
// count taken under it is not the same measurement as a count taken without,
// and comparing either against one ceiling asks the number to mean two things.
// One is about time: jxlladder_test.go sweeps fewer seeds under it.
//
// Two files rather than a runtime lookup, because the build tag is the only
// thing that knows. Go offers no way to ask at run time.
const raceEnabled = false
