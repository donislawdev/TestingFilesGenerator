package guard

import (
	"sort"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// D1 says every capability of the engine is reachable from both surfaces, and
// until 2026-08-03 nothing watched it.
//
// The regression surface ran to forty eight rows and none of them asked
// whether the window can do what the command line can. The layer test watches
// the import graph, which is a different question - it proves cli and gui
// cannot reach into each other, not that they offer the same things.
// PRODUCT.md section 8 calls the drift between them the largest risk in the
// project, and the defence on record was a sentence.
//
// This guard is armed before the first widget on purpose. Started later it
// would begin with a full list of things already built and no way to tell what
// was missed on the way. Started now it begins with everything on the "not
// yet" list and it is green - the value is not that it catches something
// today, it is that from today the distance to parity is a number the test
// prints rather than something anybody has to remember.

// reachableFromTheWindow is what the desktop window can do.
//
// Empty, because internal/gui is a seventeen line stub. Entries move here as
// widgets arrive, and each one has to be true - a capability listed here that
// the window does not actually offer would make this guard agree with the
// thing it exists to catch.
var reachableFromTheWindow = []string{}

// notYetReachable is everything the engine can do that the window cannot.
//
// This list may only shrink. It is long on purpose: it is the distance to
// parity, written down rather than estimated.
//
// Some entries are here for a second reason - the engine refuses them too, so
// neither surface has them. extends, with, policy, engine, targets.mutations,
// targets.fill and defaults.fill are all answered today with "not in this
// build yet". They stay on this list because a key nobody has built is still a
// key the window cannot produce, and separating the two reasons would be a
// second list to keep in step for no gain.
var notYetReachable = []string{
	"format:csv",
	"format:html",
	"format:json",
	"format:log",
	"format:md",
	"format:pdf",
	"format:png",
	"format:svg",
	"format:targz",
	"format:txt",
	"format:wav",
	"format:xml",
	"format:zip",
	"property:pdf.page_size",
	"property:pdf.pages",
	"property:png.height",
	"property:png.width",
	"property:wav.bit_depth",
	"property:wav.channels",
	"property:wav.content",
	"property:wav.sample_rate",
	"property:targz.entries",
	"property:targz.entry_format",
	"property:targz.entry_size",
	"property:zip.entries",
	"property:zip.entry_format",
	"property:zip.entry_size",
	"recipe:allow_nondeterministic",
	"recipe:defaults",
	"recipe:defaults.fill",
	"recipe:defaults.label",
	"recipe:engine",
	"recipe:extends",
	"recipe:locale",
	"recipe:output",
	"recipe:output.dir",
	"recipe:output.manifest",
	"recipe:output.split_threshold",
	"recipe:policy",
	"recipe:seed",
	"recipe:targets",
	"recipe:targets.boundary",
	"recipe:targets.contains",
	"recipe:targets.count",
	"recipe:targets.expected",
	"recipe:targets.fill",
	"recipe:targets.format",
	"recipe:targets.id",
	"recipe:targets.label",
	"recipe:targets.mutations",
	"recipe:targets.name",
	"recipe:targets.properties",
	"recipe:targets.size",
	"recipe:targets.size-range",
	"recipe:version",
	"recipe:with",
}

// capabilities is everything the engine can be asked for, gathered from the
// code that defines it rather than from a list somebody typed.
//
// Three sources, because three different things can drift apart: the keys a
// recipe accepts, the formats this build registered, and the settings each
// format declares. A window has to reach all three to be at parity, and the
// third is the one that arrived last - until format properties carried a type
// and a range there was nothing here worth comparing.
func capabilities() []string {
	var out []string
	for _, k := range recipe.Keys() {
		out = append(out, "recipe:"+k)
	}
	for _, d := range format.All() {
		out = append(out, "format:"+d.ID)
		for _, p := range d.Properties {
			out = append(out, "property:"+d.ID+"."+p.Name)
		}
	}
	sort.Strings(out)
	return out
}

func TestEveryEngineCapabilityIsClassifiedForBothSurfaces(t *testing.T) {
	reachable := map[string]bool{}
	for _, c := range reachableFromTheWindow {
		reachable[c] = true
	}
	pending := map[string]bool{}
	for _, c := range notYetReachable {
		pending[c] = true
	}

	all := capabilities()
	known := map[string]bool{}
	var unclassified []string
	for _, c := range all {
		known[c] = true
		switch {
		case reachable[c] && pending[c]:
			t.Errorf("%s is on both lists, so nobody can tell whether the window has it", c)
		case reachable[c] || pending[c]:
		default:
			unclassified = append(unclassified, c)
		}
	}

	if len(unclassified) > 0 {
		t.Errorf("the engine gained %d capabilit%s nobody has said anything about:\n  %s\n"+
			"Put each on reachableFromTheWindow if the window offers it, or on "+
			"notYetReachable to say out loud that it does not. Adding to the engine "+
			"without answering this is exactly the drift D1 forbids.",
			len(unclassified), plural(len(unclassified)), strings.Join(unclassified, "\n  "))
	}

	// A list that outlived what it described would quietly overstate how close
	// the two surfaces are.
	for _, c := range append(append([]string{}, reachableFromTheWindow...), notYetReachable...) {
		if !known[c] {
			t.Errorf("%s is classified and the engine no longer has it - remove the entry", c)
		}
	}

	t.Logf("D1 parity: %d of %d capabilities reachable from the window, %d still to go",
		len(reachableFromTheWindow), len(all), len(notYetReachable))
}

func plural(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}
