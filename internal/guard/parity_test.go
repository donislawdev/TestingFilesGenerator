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
// Filled on 2026-08-05 with the first generate window. It had been empty since
// the guard was armed on 2026-08-03, which was the point: the distance started
// as the whole list rather than as whatever was left after nobody was watching.
//
// This is where the section keys used to be ruled out, and the paragraph is
// replaced rather than deleted because it explained a real thing. It said
// recipe:targets and recipe:output name a part of a document rather than a
// setting, that no control corresponds to either, and that the window produced
// one target rather than a list - building an engine target directly and writing
// no recipe. It ended "they arrive with the screen that edits recipes", and on
// 2026-08-18 that screen arrived. It composes a document and the run is made
// from what comes back out of it, so the sections are produced rather than
// merely named.
var reachableFromTheWindow = []string{
	// A named question, its parameters drawn by the same code that draws a
	// format's settings - because a preset parameter IS a format.Property.
	// TestTheWindowOffersEveryPresetThereIs and the two beside it, one of which
	// runs the same preset from both surfaces and compares the bytes.
	"preset:size-boundaries",
	"preset:size-boundaries.limit",
	"preset:size-boundaries.spread",
	// The global flag this preset supplies a value for, drawn from
	// preset.Global and pressed by TestThePresetScreenCanBuildTheSetInAnyFormat,
	// which runs the set and reads the bytes back rather than looking at the
	// menu.
	"preset:size-boundaries.format",

	// Every format the registry holds, taken from the registry itself rather
	// than listed in the window. TestTheWindowOffersEveryFormatTheRegistryHas.
	"format:bmp",
	"format:docx",
	"format:csv",
	"format:gif",
	"format:html",
	"format:ico",
	"format:jpg",
	"format:pptx",
	"format:json",
	"format:log",
	"format:avif",
	"format:jxl",
	"format:md",
	"format:pdf",
	"format:png",
	"format:xlsx",
	"format:svg",
	"format:targz",
	"format:tiff",
	"format:webp",
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
	"property:bmp.height",
	"property:csv.columns",
	"property:csv.delimiter",
	"property:csv.header",
	"property:csv.line_ending",
	"property:csv.quote_style",
	"property:docx.paragraphs",
	"property:pptx.slides",
	"property:xlsx.columns",
	"property:xlsx.rows",
	"property:bmp.width",
	"property:gif.frames",
	"property:gif.height",
	"property:gif.width",
	"property:ico.embed",
	"property:ico.height",
	"property:ico.width",
	"property:jpg.height",
	"property:jpg.quality",
	"property:jpg.width",
	"property:log.entry_format",
	"property:log.ip_version",
	"property:log.level_mix",
	"property:log.line_ending",
	"property:log.methods",
	"property:log.rate",
	"property:log.status_mix",
	"property:log.timestamps",
	"property:pdf.page_size",
	"property:pdf.pages",
	"property:png.height",
	"property:png.width",
	"property:targz.compression",
	"property:targz.depth",
	"property:targz.directory_entries",
	"property:targz.entries",
	"property:targz.entry_mode",
	"property:targz.entry_owner",
	"property:targz.entry_format",
	"property:targz.entry_size",
	"property:tiff.height",
	"property:tiff.width",
	"property:webp.height",
	"property:webp.width",
	"property:avif.height",
	"property:avif.quality",
	"property:avif.width",
	"property:jxl.height",
	"property:jxl.quality",
	"property:jxl.width",
	"property:wav.bit_depth",
	"property:wav.channels",
	"property:wav.content",
	"property:wav.sample_rate",
	"property:zip.compression",
	"property:zip.depth",
	"property:zip.directory_entries",
	"property:zip.encryption",
	"property:zip.entries",
	"property:zip.entry_format",
	"property:zip.entry_size",
	"property:zip.password",

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

	// The recipe screen, 2026-08-18. Several batches in one run, with the ways
	// of asking that one target's worth of form had no room for.
	//
	// Every one of these is pressed by TestWhatIsTypedOnTheRecipeScreenIsWhatGetsWritten,
	// which runs the screen and reads the manifest off the disk - the narrow bar
	// this list is meant to be held to, rather than "there is a box for it".
	//
	// The three section keys arrive here because the screen composes a document
	// with those sections in it and the run is made from what comes back out.
	// They were absent before for the reason written above: no control
	// corresponds to a section, and the screens that produced one target wrote
	// no recipe at all.
	"recipe:targets",
	"recipe:targets.group",
	"recipe:targets.boundary",
	"recipe:targets.contains",
	"recipe:targets.expected",
	"recipe:targets.size-range",
	"recipe:defaults",
	"recipe:defaults.label",
	"recipe:output",
	"recipe:output.manifest",
}

// notYetReachable is everything the engine can do that the window cannot.
//
// This list may only shrink. It is long on purpose: it is the distance to
// parity, written down rather than estimated.
//
// Some entries are here for a second reason - the engine refuses them too, so
// neither surface has them. extends, with, policy, engine, targets.mutations,
// targets.fill, defaults.fill and output.split_threshold are all answered today
// with "not in this build yet".
//
// That is eight, and this sentence said seven until 2026-08-18: it had left out
// output.split_threshold, which recipe.go has refused all along. Counted from
// the code that does the refusing rather than from this list, which is the only
// way it could have been found - a comment has no guard, and the number here
// looked as settled as the ones a test prints.
//
// They stay on this list because a key nobody has built is still a
// key the window cannot produce, and separating the two reasons would be a
// second list to keep in step for no gain.
var notYetReachable = []string{
	"recipe:allow_nondeterministic",
	"recipe:defaults.fill",
	"recipe:engine",
	"recipe:extends",
	"recipe:locale",
	"recipe:output.split_threshold",
	"recipe:policy",
	"recipe:targets.fill",
	"recipe:targets.mutations",
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
// Reads is counted, and it was not until 2026-08-12. The reasoning for leaving
// it out was that a preset supplying a default for --format adds no capability,
// because the format is already on this list - which is true of the format and
// false of the thing that went missing. "size-boundaries in a format somebody
// chose" is its own capability and it belongs to a SCREEN: the command line had
// it from the day the preset was registered, the window could only ever produce
// PDFs, and this guard was green throughout. Reported by the owner from the
// screen rather than caught here.
//
// So the unit is the pair. A preset reading a flag no surface draws a control
// for is now a capability nobody has classified, which is what this guard does
// something about.
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
		// A name here cannot collide with a parameter above it: CLI.md section
		// 6 calls that a mistake in the definition of the preset, and there is
		// a guard that refuses to let one ship.
		for _, name := range p.Reads {
			out = append(out, "preset:"+p.ID+"."+name)
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
