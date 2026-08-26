//go:build race

package guard

// underTheRaceDetector says whether this build is the one CI runs weekly.
//
// Two files rather than a runtime check, because the answer is a build tag and
// nothing else can ask it.
const underTheRaceDetector = true
