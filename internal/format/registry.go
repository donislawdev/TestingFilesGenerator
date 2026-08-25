package format

import (
	"fmt"
	"sort"
	"strconv"
	"sync"
)

// A format announces itself once, in one place, with the full set of
// declarations. Adding a format is one registration rather than an edit in
// six files - and the same mechanism opens the plugin system later.

var (
	mu       sync.RWMutex
	registry = map[string]Descriptor{}
)

// Register adds a format. It panics on a duplicate or an obviously incomplete
// declaration, because both are programming mistakes that should never reach
// a build rather than conditions to handle at runtime.
func Register(d Descriptor) {
	mu.Lock()
	defer mu.Unlock()

	if d.ID == "" {
		panic("format: registered a descriptor with no id")
	}
	if _, exists := registry[d.ID]; exists {
		panic(fmt.Sprintf("format: %q is registered twice", d.ID))
	}
	if d.Generator == nil {
		panic(fmt.Sprintf("format: %q has no generator", d.ID))
	}
	for i := range d.Properties {
		SortChoices(d.Properties[i].Choices)
	}
	registry[d.ID] = d
}

// SortChoices puts a closed set in the order somebody looks for a value in.
//
// Here rather than in the menu that draws them, and that is the whole point:
// the same set is a menu in the window, a line of "tfg formats png" and the
// wording of a refusal, so a sort applied where they are shown would be one
// order in one surface and the declaration's order in the others. D1 breaks in
// exactly that kind of place - two surfaces describing one format two ways,
// where nobody thinks to compare.
//
// Numbers sort as numbers. Asked for as alphabetical and written as this on
// purpose, because a plain string sort puts the WAV bit depths in the order 16,
// 24, 32, 8 - alphabetical by the letter of the law and wrong to every reader.
// Every other closed set in this build is words, where the two agree.
func SortChoices(choices []string) {
	if numeric(choices) {
		sort.Slice(choices, func(i, j int) bool {
			return number(choices[i]) < number(choices[j])
		})
		return
	}
	sort.Strings(choices)
}

// numeric says whether every value of a set is a whole number, which is what
// decides between the two orders. All or nothing: a mixed set has no numeric
// order to put it in, so it goes alphabetically like any other words.
func numeric(choices []string) bool {
	if len(choices) == 0 {
		return false
	}
	for _, c := range choices {
		if _, err := strconv.ParseInt(c, 10, 64); err != nil {
			return false
		}
	}
	return true
}

func number(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

// Get returns one format by id.
// A Descriptor handed out here is READ ONLY, and that is a contract rather
// than a guarantee. The struct is copied, its Properties, JointLimits and
// Choices are not - they point at the same arrays the registry holds, and
// Register already sorts Choices in place. A caller that sorted or overwrote
// one would change what every other caller sees, and the window builds its
// menus straight out of them.
//
// Not copied on the way out, because nobody does that and a copy per call is a
// defence no test could redden - this project takes those out rather than
// keeps them. Written down instead, which is what an outside review of the
// whole tree asked for on 2026-08-23.
func Get(id string) (Descriptor, error) {
	mu.RLock()
	defer mu.RUnlock()

	d, ok := registry[id]
	if !ok {
		return Descriptor{}, &UnknownFormatError{ID: id, Known: idsLocked()}
	}
	return d, nil
}

// All returns every registered format, ordered by id so that output and tests
// do not depend on map iteration order.
func All() []Descriptor {
	mu.RLock()
	defer mu.RUnlock()

	out := make([]Descriptor, 0, len(registry))
	for _, d := range registry {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// IDs returns the registered format ids, sorted.
func IDs() []string {
	mu.RLock()
	defer mu.RUnlock()
	return idsLocked()
}

func idsLocked() []string {
	out := make([]string, 0, len(registry))
	for id := range registry {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
