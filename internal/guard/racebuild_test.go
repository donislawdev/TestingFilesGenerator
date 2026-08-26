//go:build !race

package guard

// underTheRaceDetector says whether this build is the one CI runs weekly. See
// the file beside this one.
const underTheRaceDetector = false
