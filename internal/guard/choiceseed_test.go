package guard

import (
	"errors"
	"sort"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
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
//
// The seeds are 0 to 255 and then the sixteen whose label is one numeral eight
// times, 00000000 to ffffffff. The first 256 move only the last two numerals of
// the label, and a file's own seed is a hash with all eight moving: a drawing
// whose label was outlines needed 2.5 kB more for some seeds than for others
// (2026-11-14, docs/PLACEHOLDER-2026-11-14.md), and a length that grows with
// what a numeral looks like is not swept by moving two of eight. The sixteen
// are the extremes where the length of one numeral adds up eight times.
func TestTheMinimumStandsWhateverTheSeedIsUnderEveryDeclaredChoice(t *testing.T) {
	seeds := sweptSeeds()
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
				for _, seed := range seeds {
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

// sweptSeeds is 0 to 255 and the sixteen seeds whose label is one numeral at
// every place - and it says so, or the sweep would stop reaching the state it
// is for and stay green (O118).
func sweptSeeds() []uint64 {
	out := make([]uint64, 0, 256+16)
	for seed := uint64(0); seed < 256; seed++ {
		out = append(out, seed)
	}
	for d := uint64(0); d < 16; d++ {
		out = append(out, d*0x11111111)
	}
	return out
}

// TestTheSeedSweepMovesEveryNumeralOfTheLabel is the canary of the sweep
// above: across the seeds it takes, every place of the label has to show all
// sixteen numerals.
func TestTheSeedSweepMovesEveryNumeralOfTheLabel(t *testing.T) {
	seen := make([]map[byte]bool, 8)
	for i := range seen {
		seen[i] = map[byte]bool{}
	}
	for _, seed := range sweptSeeds() {
		label := core.SeedLabel(seed)
		for i := 0; i < len(label) && i < 8; i++ {
			seen[i][label[i]] = true
		}
	}
	for i, numerals := range seen {
		if len(numerals) != 16 {
			t.Errorf("place %d of the label shows %d of 16 numerals across the sweep, so a length that moves with that place is not swept", i+1, len(numerals))
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
	// A setting that shapes log records needs a file of them.
	"otlp.level_mix=debug":     {"signal": "logs"},
	"otlp.level_mix=errors":    {"signal": "logs"},
	"otlp.level_mix=quiet":     {"signal": "logs"},
	"otlp.trace_context=false": {"signal": "logs"},
	"otlp.body=any-value":      {"signal": "logs"},
	// And one that shapes metrics, a file of metrics.
	"otlp.metric_kind=exponential-histogram": {"signal": "metrics"},
	"otlp.metric_kind=gauge":                 {"signal": "metrics"},
	"otlp.metric_kind=histogram":             {"signal": "metrics"},
	"otlp.metric_kind=sum":                   {"signal": "metrics"},
	"otlp.metric_kind=summary":               {"signal": "metrics"},
	"otlp.temporality=delta":                 {"signal": "metrics"},
	"otlp.exemplars=true":                    {"signal": "metrics"},
	"otlp.non_finite=true":                   {"signal": "metrics"},
	// XML in UTF-16 opens with a byte order mark.
	"xml.encoding=utf-16be": {"bom": "true"},
	"xml.encoding=utf-16le": {"bom": "true"},
	// A directory entry needs a directory.
	"zip.directory_entries=true":   {"depth": "1"},
	"targz.directory_entries=true": {"depth": "1"},
	// Guides are drawn on a placeholder, and a test picture has none.
	"png.guides=true": {"content": "placeholder"},
	"jpg.guides=true": {"content": "placeholder"},
	"svg.guides=true": {"content": "placeholder"},
	// Locking needs something to lock with.
	"zip.encryption=aes-128":   {"password": "guard"},
	"zip.encryption=aes-192":   {"password": "guard"},
	"zip.encryption=aes-256":   {"password": "guard"},
	"zip.encryption=zipcrypto": {"password": "guard"},
}
