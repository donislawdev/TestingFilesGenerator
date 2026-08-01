package format

import (
	"fmt"
	"sort"
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
	registry[d.ID] = d
}

// Get returns one format by id.
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
