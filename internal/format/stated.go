package format

import "sort"

// CheckStated is every problem with the values stated for a list of declared
// settings that belongs to something other than a format - a tool, today.
//
// CheckEachProperty, a damage's CheckEach and a preset's own check are three
// copies of this already (O261), and a tool would have been the fourth. The
// refusals are the same two types, so a form puts them under the box they are
// about and the command line ends with the same code, whoever declared the
// setting. owner fills the field those types call Format.
//
// In a file of its own since 2026-09-30, when format.go went past the ceiling
// on the length of a file. It is the one part of the package that is not
// about a format.
func CheckStated(owner string, declared []Property, stated map[string]string) []error {
	known := make(map[string]Property, len(declared))
	names := make([]string, 0, len(declared))
	for _, p := range declared {
		known[p.Name] = p
		names = append(names, p.Name)
	}
	keys := make([]string, 0, len(stated))
	for k := range stated {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var bad []error
	for _, k := range keys {
		if err := statedProblem(owner, known, names, k, stated[k]); err != nil {
			bad = append(bad, err)
		}
	}
	return bad
}

// statedProblem is what is wrong with one stated value, or nil.
func statedProblem(owner string, known map[string]Property, names []string, key, value string) error {
	p, ok := known[key]
	if !ok {
		return &UnknownPropertyError{Format: owner, Key: key, Known: names}
	}
	// Not stated, the same as left out - see CheckEachProperty.
	if value == "" {
		return nil
	}
	if why := p.Allows(value); !why.IsZero() {
		return &PropertyValueError{Format: owner, Key: key, Value: value, Reason: why, Remedy: p.Instead()}
	}
	return nil
}
