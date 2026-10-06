package guard

import (
	"errors"
	"sort"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// The minimum stands whatever the seed is under every value a closed set
// offers, not only under the defaults.
//
// TestTheMinimumThisToolPrintsStandsWhateverTheSeedIs sweeps 256 seeds, and it
// sweeps them at the default settings only - a Request with no properties. So
// a value that made the length of a file depend on what the seed drew would
// sail past it: the default never reaches that value. Found while planning the
// PDF settings of 2026-09-29, where compressing the page text would have done
// exactly that, and the guard that exists to catch it could not have seen it.
//
// Every choice and every switch, each non-default value on its own. A value
// that is refused on its own - "mixed" in a document of one page - is given
// the setting it needs from companions, and a companion that stops being
// needed is itself a failure, so the list cannot rot into a list of reasons to
// look away.
func TestTheMinimumStandsWhateverTheSeedIsUnderEveryDeclaredChoice(t *testing.T) {
	const seeds = 256
	checked := 0
	usedCompanion := map[string]bool{}

	for _, d := range format.All() {
		for _, p := range d.Properties {
			for _, v := range nonDefaultValues(p) {
				key := d.ID + "." + p.Name + "=" + v
				props := map[string]string{p.Name: v}
				for k, cv := range choiceCompanions[key] {
					props[k] = cv
					usedCompanion[key] = true
				}
				r := format.Request{Label: true, Properties: props}
				floor := d.SmallestAccepted(r)
				for seed := uint64(0); seed < seeds; seed++ {
					r.Bytes, r.Seed = floor, seed
					_, err := d.Generator.Plan(r)
					if err == nil {
						continue
					}
					var below *format.BelowMinimumError
					if errors.As(err, &below) {
						t.Errorf("%s: the minimum is %d B and seed %d refuses it: %v\n"+
							"A floor that moves with the seed means somebody reads one number and gets "+
							"a refusal on the run after the one that worked.", key, floor, seed, err)
					} else {
						t.Errorf("%s is refused for a reason that is not its size, so the seeds were not "+
							"swept at all - give it what it needs in choiceCompanions: %v", key, err)
					}
					break
				}
				checked++
			}
		}
	}
	if checked == 0 {
		t.Fatal("no format declares a closed set, so this guard checked nothing")
	}

	// A companion nobody needs any more is a value this guard sets for no
	// reason, and it may be hiding the very refusal it was added for.
	keys := make([]string, 0, len(choiceCompanions))
	for k := range choiceCompanions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !usedCompanion[key] {
			t.Errorf("choiceCompanions names %s and no declared value matches it any more", key)
		}
	}
}

// nonDefaultValues is every value of a closed set or a switch but the default.
func nonDefaultValues(p format.Property) []string {
	var values []string
	switch p.Kind {
	case format.PropertyChoice:
		values = p.Choices
	case format.PropertyBool:
		values = []string{"true", "false"}
	case format.PropertyInt, format.PropertySize, format.PropertyDuration, format.PropertyText:
		// Open ranges and free text have no list to walk. A kind added
		// later reddens the linter here rather than being skipped unseen.
	}
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v != p.Default {
			out = append(out, v)
		}
	}
	return out
}

// choiceCompanions is what a value needs beside it before a file can be made
// at all, keyed by format.setting=value.
var choiceCompanions = map[string]map[string]string{
	// Mixed pages need more than one page.
	"pdf.page_size=mixed":   {"pages": "2"},
	"pdf.orientation=mixed": {"pages": "2"},
	// A severity is carried only by the shapes that have one.
	"log.level_mix=debug":  {"entry_format": "plain"},
	"log.level_mix=errors": {"entry_format": "plain"},
	"log.level_mix=quiet":  {"entry_format": "plain"},
	// XML in UTF-16 opens with a byte order mark.
	"xml.encoding=utf-16be": {"bom": "true"},
	"xml.encoding=utf-16le": {"bom": "true"},
	// A directory entry needs a directory.
	"zip.directory_entries=true":   {"depth": "1"},
	"targz.directory_entries=true": {"depth": "1"},
	// Locking needs something to lock with.
	"zip.encryption=aes-128":   {"password": "guard"},
	"zip.encryption=aes-192":   {"password": "guard"},
	"zip.encryption=aes-256":   {"password": "guard"},
	"zip.encryption=zipcrypto": {"password": "guard"},
}
