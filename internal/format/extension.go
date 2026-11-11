package format

import (
	"maps"
	"slices"
)

// ExtensionFor is what the name of a file of this format ends in when it is
// made with props: the ending the chosen value of a setting names, and
// Extension when none does.
//
// A setting that is absent or empty counts at its default, the way every
// generator reads it, and a default never names an ending of its own - so that
// is Extension too. The value is compared as it is spelled, as Allows compares
// it, so the value the engine accepted is the value looked up here.
//
// The engine asks once for each target rather than once for each file: the
// settings belong to the target, and a target can be a million files.
func (d Descriptor) ExtensionFor(props map[string]string) string {
	for _, p := range d.Properties {
		if ext, named := p.Extensions[props[p.Name]]; named {
			return ext
		}
	}
	return d.Extension
}

// Extensions is every ending a file of this format can have: Extension first,
// then each one a setting can give instead, in order and once each. It is for
// a surface that lists formats, and has no settings in hand to ask ExtensionFor
// with.
func (d Descriptor) Extensions() []string {
	others := map[string]bool{}
	for _, p := range d.Properties {
		for _, ext := range p.Extensions {
			others[ext] = true
		}
	}
	delete(others, d.Extension)
	return append([]string{d.Extension}, slices.Sorted(maps.Keys(others))...)
}
