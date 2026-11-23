package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/version"
)

// What this defends. Every version README.md writes out by hand is the version
// this build is, or, while this build is a release candidate, the last stable
// release in the commands.
//
// Why it needed a guard. README names the version in three places a person
// copies from: the Docker image tag, the version the GitHub Actions install
// step downloads, and the example manifest. All three are typed, and nothing
// read them - the example manifest stayed at the old version through the whole
// of 0.3.0-rc1 (O198). The first two are worse when they go stale, because they
// are commands: an old tag still pulls, and the person gets last release's
// bytes without being told.
//
// When this goes red. On the pull request that raises the version to a stable
// one, which is where README has to follow. The image for the new version
// exists only once the release is published, so for that short while the
// README names an image that is still being made.
//
// A release candidate is never made into an image or sent to npm or PyPI
// (stablechannels_test.go), so while this build is one, the two commands keep
// naming the last stable release - the newest heading of CHANGELOG.md without
// a hyphen - and only the example manifest names the candidate, because that
// is what this build writes.

var readmeVersions = []struct {
	where   string
	match   *regexp.Regexp
	command bool // a command that fetches a release, so it names a stable one
}{
	{"the Docker image tag", regexp.MustCompile(`ghcr\.io/donislawdev/tfg:([0-9][0-9A-Za-z.-]*)`), true},
	{"the version the GitHub Actions install step downloads", regexp.MustCompile(`(?m)^\s*version=([0-9][0-9A-Za-z.-]*)\s*$`), true},
	{"the example manifest", regexp.MustCompile(`"version": "([0-9][0-9A-Za-z.-]*)"`), false},
}

func TestEveryVersionTheReadmeWritesOutIsThisOne(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "README.md"))
	if err != nil {
		t.Fatalf("README.md is missing: %v", err)
	}
	// Canary: the last stable release is read past a candidate, and a stable
	// build is its own.
	planted := "## [Unreleased]\n\n## [0.7.0-rc1] - 2027-01-02\n\n## [0.6.0] - 2026-12-01\n\n## [0.5.0] - 2026-10-20\n"
	for build, want := range map[string]string{"0.7.0-rc1": "0.6.0", "0.7.0": "0.7.0"} {
		if got, err := releasedVersion(planted, build); got != want || err != nil {
			t.Fatalf("canary: for build %s the planted changelog gave %q (%v), and the last stable release is %s",
				build, got, err, want)
		}
	}
	released := releasedVersionOfThisBuild(t)

	for _, place := range readmeVersions {
		found := place.match.FindAllSubmatch(body, -1)
		// Asserted, not assumed: a reworded README that no longer matches
		// would otherwise pass here by finding nothing to compare.
		if len(found) == 0 {
			t.Errorf("README.md no longer writes out %s in a shape this guard reads (%s). "+
				"Teach it the new shape rather than deleting it.", place.where, place.match)
			continue
		}
		want, why := version.Version, "this build is "+version.Version
		if place.command && released != version.Version {
			want, why = released, "this build is the release candidate "+version.Version+
				", which is not made into an image or sent anywhere but GitHub, and the last stable release is "+released
		}
		for _, m := range found {
			if string(m[1]) != want {
				t.Errorf("README.md writes out %s as %s, and %s.\n"+
					"A person copying it gets another release than the one the page describes.",
					place.where, m[1], why)
			}
		}
	}
}

// releasedVersionOfThisBuild is releasedVersion for this build and this
// repository's CHANGELOG.md.
func releasedVersionOfThisBuild(t *testing.T) string {
	t.Helper()
	changelog, err := os.ReadFile(filepath.Join(repoRoot(t), "CHANGELOG.md"))
	if err != nil {
		t.Fatalf("CHANGELOG.md is missing: %v", err)
	}
	released, err := releasedVersion(string(changelog), version.Version)
	if err != nil {
		t.Fatal(err)
	}
	return released
}

// releasedVersion is the release a person can install while the build is
// build: the build itself when it is stable, and the newest stable heading of
// the changelog while it is a release candidate.
func releasedVersion(changelog, build string) (string, error) {
	if !strings.Contains(build, "-") {
		return build, nil
	}
	m := regexp.MustCompile(`(?m)^## \[([0-9]+\.[0-9]+\.[0-9]+)\]`).FindStringSubmatch(changelog)
	if m == nil {
		return "", fmt.Errorf("this build is the release candidate %s, and CHANGELOG.md has no heading of a "+
			"stable release to name in the README's commands", build)
	}
	return m[1], nil
}
