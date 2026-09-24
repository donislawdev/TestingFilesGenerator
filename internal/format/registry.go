package format

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
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

// SmallestWithLabel is d.SmallestAccepted(Request{Label: true}), worked out
// once per format per process and remembered.
//
// The request is the same every time, so the answer is too - and finding it
// means planning the format at growing sizes, which for a picture means
// encoding one. The minimal preset asks it of every format at every
// expansion, and the window expands that preset on every change while the
// batch screen builds on it: measured on 2026-09-23, 50.4 MB allocated per
// expansion afresh against 0.51 MB remembered, and a keystroke that cost
// 380 ms in the real window (docs/GUI-MEMORY-2026-09-23.md section 2.3).
//
// Keyed by id and request - see SmallestRemembered - which is safe because
// Register refuses a second descriptor under one id. Here rather than beside
// its caller because this file is already where the registry's reads meet its
// writes: the window settles from its worker as well as from its own
// goroutine. The size is worked out without the lock held, because planning an
// archive reads the registry itself.
func SmallestWithLabel(d Descriptor) int64 {
	return SmallestRemembered(d, Request{Label: true})
}

// SmallestRemembered is d.SmallestAccepted(r), worked out once per format and
// request and remembered.
//
// SmallestWithLabel's reasoning, for the questions that carry settings or a
// seed. The presets ask the floor of a file with its dialect, of a sheet with
// its rows and columns, of the boundary set with seed 1 - and asked it afresh
// at every expansion, which on a picture is encoding one. Measured 2026-09-23:
// 29% of expanding tabular-import (docs/GUI-MEMORY-2026-09-23.md section 4j).
//
// The size a request asks for and SizeFromContents are left out, because
// SmallestAccepted sets both itself. A request with contents is worked out
// every time - see RequestKey.
//
// At most smallestCeiling answers are kept. The key grows with values somebody
// types - the rows of a sheet - so a long session would otherwise keep one for
// every number ever typed. Past the ceiling the memory starts again, which
// costs one working out per question and nothing else.
func SmallestRemembered(d Descriptor, r Request) int64 {
	r.Bytes, r.SizeFromContents = 0, false
	key, ok := RequestKey(d.ID, r)
	if !ok {
		return d.SmallestAccepted(r)
	}
	smallestMu.Lock()
	known, found := smallestKnown[key]
	smallestMu.Unlock()
	if found {
		return known
	}
	size := d.SmallestAccepted(r)
	smallestMu.Lock()
	if len(smallestKnown) >= smallestCeiling {
		smallestKnown = map[string]int64{}
	}
	smallestKnown[key] = size
	smallestMu.Unlock()
	return size
}

// smallestCeiling is how many answers SmallestRemembered keeps - a few hundred
// bytes each, so the most it holds is about a megabyte.
const smallestCeiling = 4096

// smallestKnown is what SmallestRemembered has worked out, by RequestKey.
var (
	smallestMu    sync.Mutex
	smallestKnown = map[string]int64{}
)

// RequestKey is one request to one format written as text that no other
// request shares, so that something worked out for it can be kept under it.
//
// Every field of Request that can change a plan is in it, and a guard sets
// each field in turn to hold that true when Request grows. The values are
// quoted, because a setting's value is text somebody typed and may hold any
// separator - a CSV delimiter of "|" would otherwise read as the start of a
// second setting, and two requests sharing a key share an answer.
//
// A request with contents has no key. What an archive holds is a list of
// formats with sizes of their own, and nothing asks such a request twice.
func RequestKey(id string, r Request) (string, bool) {
	if len(r.Contains) > 0 {
		return "", false
	}
	names := make([]string, 0, len(r.Properties))
	for name := range r.Properties {
		names = append(names, name)
	}
	sort.Strings(names)
	var b strings.Builder
	fmt.Fprintf(&b, "%q %d %t %t %d", id, r.Bytes, r.SizeFromContents, r.Label, r.Seed)
	for _, name := range names {
		fmt.Fprintf(&b, " %q=%q", name, r.Properties[name])
	}
	return b.String(), true
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

// SecretProperties are the names of every property any registered format
// declares as a credential, sorted.
//
// A name rather than a format and a name, because the places that ask are
// looking at a value somebody typed - "--set password=..." on a command line
// carries no format with it, and a recipe can name several. One name being
// secret anywhere is enough for those places to treat it as secret, which errs
// in the direction that cannot leak.
func SecretProperties() []string {
	mu.RLock()
	defer mu.RUnlock()

	seen := map[string]struct{}{}
	for _, d := range registry {
		collectSecrets(d.Properties, seen)
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// collectSecrets adds the names of the secret properties in props to into.
//
// A function of its own rather than the inner loop of the one above, for the
// reason written beside the same split in internal/recipe: together they nest
// three deep - the loop over formats, the loop over properties, the test - and
// the shape guard counts how many functions sit that deep as well as how deep
// the deepest one is.
func collectSecrets(props []Property, into map[string]struct{}) {
	for _, p := range props {
		if p.Secret {
			into[p.Name] = struct{}{}
		}
	}
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
