package recipe

import (
	"reflect"
	"sort"
	"strings"
)

// Keys is every key a recipe accepts, in dotted form.
//
// Derived from the structures that read a recipe rather than written out
// again. A hand written list would be a second place to keep in step, and this
// one exists to be compared against another surface - so a list that drifted
// would make the comparison agree with a mistake instead of catching it.
//
// Keys this build refuses are included, because they are keys a recipe may
// contain and the refusal is what makes them visible. What this build does with
// a key is a different question from whether the key exists.
func Keys() []string {
	// collect fills this map through the call below, which the rule does not
	// follow, so it reads the map as one nothing ever writes to. An empty one
	// would redden the two guards that read Keys.
	// nosemgrep: trailofbits.go.iterate-over-empty-map.iterate-over-empty-map
	seen := map[string]bool{}
	collect(reflect.TypeOf(rawRecipe{}), "", seen)
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// collect walks one structure, following the nested ones a recipe has.
//
// Depth is bounded by the shape of a recipe rather than by a counter: the
// nesting is defaults, output, targets and the entries of contains, and none
// of them contains a recipe.
func collect(t reflect.Type, prefix string, seen map[string]bool) {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := strings.Split(f.Tag.Get("yaml"), ",")[0]
		if tag == "" || tag == "-" {
			continue
		}
		name := prefix + tag
		seen[name] = true
		collect(f.Type, name+".", seen)
	}
}
