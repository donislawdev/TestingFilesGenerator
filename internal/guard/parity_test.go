package guard

import (
	"sort"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
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
// The bar for moving an entry here is deliberately narrow, because a capability
// listed as reachable that the window does not actually offer would make this
// guard agree with the thing it exists to catch. Two things are required, and
// the second is the one that is easy to skip: there is a control on the screen,
// AND a guard presses it and finds the value on the other side. Drawing a box
// that is dropped on the way to the engine looks exactly like drawing one that
// works.
//
// So the section keys are absent on purpose. recipe:targets and recipe:output
// name a part of a document rather than a setting, no control corresponds to
// either, and the window produces one target rather than a list - it builds an
// engine target directly and writes no recipe. They arrive with the screen that
// edits recipes.
//
// Filled on 2026-08-05 with the first generate window. It had been empty since
// the guard was armed on 2026-08-03, which was the point: the distance started
// as the whole list rather than as whatever was left after nobody was watching.
var reachableFromTheWindow = []string{
	// Every format the registry holds, taken from the registry itself rather
	// than listed in the window. TestTheWindowOffersEveryFormatTheRegistryHas.
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

	// Drawn from the declaration and nothing else, which is what declaring
	// properties rather than only naming them was for: the kind picks the
	// control, the range and the closed set become the sentence under it, and
	// the refusal comes from the registry in the registry's words. A format that
	// gains a property gains its field with no window code.
	// TestTheWindowDrawsAFieldForEveryDeclaredProperty.
	"property:pdf.page_size",
	"property:pdf.pages",
	"property:png.height",
	"property:png.width",
	"property:targz.entries",
	"property:targz.entry_format",
	"property:targz.entry_size",
	"property:wav.bit_depth",
	"property:wav.channels",
	"property:wav.content",
	"property:wav.sample_rate",
	"property:zip.entries",
	"property:zip.entry_format",
	"property:zip.entry_size",

	// One target's worth of settings, each with a field and each found again in
	// the manifest of a run started from the window.
	// TestWhatIsTypedOnTheScreenIsWhatGetsWritten.
	"recipe:output.dir",
	"recipe:seed",
	"recipe:targets.count",
	"recipe:targets.format",
	"recipe:targets.id",
	"recipe:targets.label",
	"recipe:targets.name",
	"recipe:targets.properties",
	"recipe:targets.size",
}

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
	"preset:size-boundaries",
	"preset:size-boundaries.limit",
	"preset:size-boundaries.spread",
	"recipe:allow_nondeterministic",
	"recipe:targets.group",
	"recipe:defaults",
	"recipe:defaults.fill",
	"recipe:defaults.label",
	"recipe:engine",
	"recipe:extends",
	"recipe:locale",
	"recipe:output",
	"recipe:output.manifest",
	"recipe:output.split_threshold",
	"recipe:policy",
	"recipe:targets",
	"recipe:targets.boundary",
	"recipe:targets.contains",
	"recipe:targets.expected",
	"recipe:targets.fill",
	"recipe:targets.mutations",
	"recipe:targets.size-range",
	"recipe:version",
	"recipe:with",
}

// capabilities is everything the engine can be asked for, gathered from the
// code that defines it rather than from a list somebody typed.
//
// Four sources, because four different things can drift apart: the keys a
// recipe accepts, the formats this build registered, the settings each format
// declares, and the presets. A window has to reach all four to be at parity.
//
// The presets arrived last and they arrived late, which is the whole lesson of
// O59 in docs/OBSERVATIONS.md. This guard was armed before the first widget so
// that it would never start from a full list of things already built - and then
// size-boundaries was registered and it stayed green, because a preset is not a
// recipe key and not a format and nothing here looked for one. Measured on
// 2026-08-04: adding the preset did not redden this, while adding the recipe key
// group reddened it at once.
//
// Reads is deliberately not counted. A preset that supplies a default for
// --format has not added a capability - the format is already on this list, and
// counting it twice would make the distance to parity read longer than it is.
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
	for _, p := range preset.All() {
		out = append(out, "preset:"+p.ID)
		for _, param := range p.Parameters {
			out = append(out, "preset:"+p.ID+"."+param.Name)
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
