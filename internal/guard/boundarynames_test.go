package guard

import (
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
)

// Every file of a boundary set says which limit it was built around.
//
// Reported from use on 2026-08-11. The set names its files after the DISTANCE
// from the limit - under_1kb, over_1mb - and the limit itself appeared nowhere,
// so "at_limit.pdf" from a 10mb run and "at_limit.pdf" from a 5mb run are the
// same name for two different claims. A directory holding both is a directory
// of guesses, and the size on disk is the only way to tell them apart.
//
// The full suite stayed green when the names changed, which is the reason this
// exists: nothing was asking.
//
// The id is deliberately NOT checked for the limit. It derives the seed, so
// putting the limit there would move the bytes of every file in the set for a
// change that is only about telling two directories apart.
func TestEveryBoundaryFileNamesTheLimitItWasBuiltAround(t *testing.T) {
	// A plain byte count is in here on purpose beside the written sizes,
	// because it is the other spelling the limit accepts and it lands in the
	// name just the same. It has to clear the smallest set this preset can
	// build - 512 does not, and the preset refuses the whole set rather than
	// most of it, which is PR7 working.
	for _, limit := range []string{"5mb", "10mb", "20971520"} {
		src, err := preset.Expand("size-boundaries", preset.Args{"limit": limit})
		if err != nil {
			t.Fatalf("expanding at limit %s: %v", limit, err)
		}
		names := 0
		for _, line := range strings.Split(string(src.Source), "\n") {
			name, found := strings.CutPrefix(strings.TrimSpace(line), "name: ")
			if !found {
				continue
			}
			names++
			if !strings.HasPrefix(name, limit+"_") {
				t.Errorf("limit %s produced the file %q, which does not say what it is measured from.\n"+
					"Two sets built around different limits then share a name and differ only in size on disk.",
					limit, name)
			}
		}
		if names < 7 {
			t.Fatalf("limit %s produced %d name(s), so this guard read the wrong thing", limit, names)
		}
	}
}
