package guard

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/version"
)

// What this defends. Every version README.md writes out by hand is the version
// this build is.
//
// Why it needed a guard. README names the version in three places a person
// copies from: the Docker image tag, the version the GitHub Actions install
// step downloads, and the example manifest. All three are typed, and nothing
// read them - the example manifest stayed at the old version through the whole
// of 0.3.0-rc1 (O198). The first two are worse when they go stale, because they
// are commands: an old tag still pulls, and the person gets last release's
// bytes without being told.
//
// When this goes red. On the pull request that raises the version, which is
// where README has to follow. The image for the new version exists only once
// the release is published, so for that short while the README names an image
// that is still being made.

var readmeVersions = []struct {
	where string
	match *regexp.Regexp
}{
	{"the Docker image tag", regexp.MustCompile(`ghcr\.io/donislawdev/tfg:([0-9][0-9A-Za-z.-]*)`)},
	{"the version the GitHub Actions install step downloads", regexp.MustCompile(`(?m)^\s*version=([0-9][0-9A-Za-z.-]*)\s*$`)},
	{"the example manifest", regexp.MustCompile(`"version": "([0-9][0-9A-Za-z.-]*)"`)},
}

func TestEveryVersionTheReadmeWritesOutIsThisOne(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "README.md"))
	if err != nil {
		t.Fatalf("README.md is missing: %v", err)
	}

	for _, place := range readmeVersions {
		found := place.match.FindAllSubmatch(body, -1)
		// Asserted, not assumed: a reworded README that no longer matches
		// would otherwise pass here by finding nothing to compare.
		if len(found) == 0 {
			t.Errorf("README.md no longer writes out %s in a shape this guard reads (%s). "+
				"Teach it the new shape rather than deleting it.", place.where, place.match)
			continue
		}
		for _, m := range found {
			if string(m[1]) != version.Version {
				t.Errorf("README.md writes out %s as %s, and this build is %s.\n"+
					"A person copying it gets another release than the one the page describes.",
					place.where, m[1], version.Version)
			}
		}
	}
}
