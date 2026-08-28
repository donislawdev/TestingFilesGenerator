// What a running binary can say about itself, without a file beside it and
// without a network.
//
// The notice both surfaces already print points at THIRD-PARTY-NOTICES.md,
// which is honest and useless to somebody holding only the binary - the window
// is downloaded on its own, and the file is one directory away at best. So the
// binary answers from what it carries: Go records its own module list inside
// the executable, and this joins that record with the reviewed licences.
//
// The join is the whole point. The build knows the versions and knows nothing
// about licences. The registry knows the licences and deliberately holds no
// versions. Neither half can drift from the other, because neither half is a
// copy of the other.

package legal

import (
	"runtime/debug"
	"sort"
)

// An Item is one thing a binary carries that somebody else wrote.
//
// Deliberately not a rendered line. The command line writes English and the
// window writes whatever language it was asked for, so the words around these
// values belong to each surface - and the values themselves are names,
// numbers and licence identifiers, which are the same in every language.
type Item struct {
	// Name is the module path, or the name of a thing that is not a module.
	Name string

	// Version as the build recorded it. Empty for embedded files, which have
	// no version of their own - they are dated by the module that brings them.
	Version string

	// SPDX and Copyright come from the registry.
	SPDX      string
	Copyright string

	// Embedded separates code from bytes that are not code. A person reading
	// the list wants to know which of these is a library and which is a font.
	Embedded bool
}

// Line is the item written out: name, version, licence and copyright, joined
// by two spaces.
//
// Values only, and no words at all. Both surfaces print this - the command
// line in English by rule, the window in whatever language it was asked for -
// and a sentence here would be English arriving on a translated screen.
//
// A version can be absent, because a font has none of its own. A copyright can
// be absent too, and one is: the licence file of one module is the unmodified
// Apache template with the placeholder left in, so there is no line to quote.
// A licence cannot be absent - Carried lists only what the registry knows, and
// the registry cannot hold an entry without one.
func (i Item) Line() string {
	line := i.Name
	if i.Version != "" {
		line += "  " + i.Version
	}
	line += "  " + i.SPDX
	if i.Copyright != "" {
		line += "  " + i.Copyright
	}
	return line
}

// Carried reports what this build contains, from the build's own record.
//
// The main module is left out on purpose: it is this program, its licence is
// the one printed above this list, and repeating it here would read as though
// we were somebody else's dependency.
//
// A module with no entry in the registry is left out, and that is a measured
// decision rather than a shrug. In a shipped binary the case cannot happen: a
// guard fails when anything either binary links is missing from the registry,
// in both directions. What it can happen in is a TEST binary, which links its
// own machinery - measured on 2026-08-28, the guard package's binary carries
// four modules the window does not - and the window's about screen is rendered
// and compared pixel by pixel from inside that binary. Listing whatever the
// running build happens to contain would put a testing library on the stored
// picture of a product screen, and would move it again the next time a test
// gained a dependency.
//
// So the list is what this build carries AND somebody has reviewed. The other
// half of that sentence is a separate question, asked by a guard rather than
// answered by a silent gap in a list.
func Carried(info *debug.BuildInfo) []Item {
	if info == nil {
		return nil
	}
	reviewed := map[string]Module{}
	for _, m := range modules {
		reviewed[m.Path] = m
	}

	// The runtime first, and it is the entry no build reports: every Go binary
	// links it, it is not a module, so it appears in no dependency list and
	// would be the one thing missing from a list claiming to be complete.
	items := []Item{moduleItem(runtimeName, info.GoVersion, reviewed)}
	linked := map[string]bool{}
	for _, dep := range info.Deps {
		if dep == nil {
			continue
		}
		linked[dep.Path] = true
		if _, known := reviewed[dep.Path]; !known {
			continue
		}
		items = append(items, moduleItem(dep.Path, dep.Version, reviewed))
	}
	items = append(items, embeddedItems(linked)...)

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Embedded != items[j].Embedded {
			return !items[i].Embedded
		}
		return items[i].Name < items[j].Name
	})
	return items
}

// Reviewed is the whole registry as items, with no versions.
//
// This is what the window shows, and both halves of that are measured rather
// than chosen. The window links every module in this list - a guard compares
// the two in both directions - so for that binary "what it carries" and "the
// registry" are the same set. And a version cannot be read there anyway when
// it matters most: measured on 2026-08-28, debug.ReadBuildInfo in a binary
// built by go test comes back with no dependencies at all, so a screen asking
// its own build would draw one line under test and thirty in the product, and
// the stored picture of that screen would depict neither.
//
// The command line has no such problem and does show versions, because it is
// asked in a binary that has them. The versions also stand in
// THIRD-PARTY-NOTICES.md, held to the build by a guard of their own.
func Reviewed() []Item {
	items := make([]Item, 0, len(modules)+len(assets))
	for _, m := range modules {
		items = append(items, Item{Name: displayName(m.Path), SPDX: m.SPDX, Copyright: m.Copyright})
	}
	for _, a := range assets {
		items = append(items, Item{Name: a.Name, SPDX: a.SPDX, Copyright: a.Copyright, Embedded: true})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Embedded != items[j].Embedded {
			return !items[i].Embedded
		}
		return items[i].Name < items[j].Name
	})
	return items
}

// CarriedHere is Carried asked of the binary that is running.
func CarriedHere() []Item {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return nil
	}
	return Carried(info)
}

// displayName is what a person is shown instead of a module path. Only the
// runtime needs one: "std" is what the toolchain calls it and means nothing
// to somebody reading a licence screen. Written once so the window and the
// command cannot come to call the same thing two names.
func displayName(path string) string {
	if path == runtimeName {
		return "Go runtime and standard library"
	}
	return path
}

// runtimeName is how the registry spells the runtime entry.
const runtimeName = "std"

func moduleItem(path, version string, reviewed map[string]Module) Item {
	m := reviewed[path]
	return Item{Name: displayName(path), Version: version, SPDX: m.SPDX, Copyright: m.Copyright}
}

// embeddedItems are the registry entries whose module is in this build. A font
// travels with the package that embeds it, so the question of whether it ships
// is the question of whether its module was linked.
func embeddedItems(linked map[string]bool) []Item {
	var items []Item
	for _, asset := range assets {
		if !linked[asset.Module] {
			continue
		}
		items = append(items, Item{
			Name:      asset.Name,
			SPDX:      asset.SPDX,
			Copyright: asset.Copyright,
			Embedded:  true,
		})
	}
	return items
}
