package commandline

import (
	"sort"
	"strings"
)

// The verbs and flags a screen of the window has an equivalent for.
//
// Here rather than typed in the window, so that the names live in one place,
// and that they are the names the command line really takes is held by the
// guard that pastes a copied line into the command line and compares the files
// it makes with the files the window makes - copycommand_test.go. A name that
// drifted would be a flag the command line refuses, and that guard goes red.
const (
	Generate = "generate"
	Tool     = "tool"

	Clean         = "--clean"
	Count         = "--count"
	Damage        = "--damage"
	Format        = "--format"
	ID            = "--id"
	Memory        = "--memory"
	NameTemplate  = "--name"
	NumberIfTaken = "--number-if-taken"
	Out           = "--out"
	Preset        = "--preset"
	Seed          = "--seed"
	Set           = "--set"
	Size          = "--size"
)

// Named is the flag for a parameter of a preset or a setting of a tool, which
// are flags named after themselves.
func Named(name string) string { return "--" + name }

// DamageValue writes one damage as --damage takes it: the name, then a colon
// and its settings as name=value separated by commas.
//
// The settings go in the order of their names, because the flag reads them into
// a map and the order means nothing to it, and a line that kept the order of a
// map would differ on every press. A comma inside a value cannot be written,
// because the flag splits on every comma and has no way to escape one -
// internal/cli, damageSettings - so that is refused under the setting it is in.
func DamageValue(id string, values map[string]string) (string, error) {
	names := make([]string, 0, len(values))
	for name, value := range values {
		if strings.Contains(value, ",") {
			return "", &Unwritable{Setting: name, Reason: CommaInDamage}
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return id, nil
	}
	sort.Strings(names)
	pairs := make([]string, 0, len(names))
	for _, name := range names {
		pairs = append(pairs, name+"="+values[name])
	}
	return id + ":" + strings.Join(pairs, ","), nil
}
