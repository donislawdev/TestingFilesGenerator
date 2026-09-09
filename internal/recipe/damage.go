package recipe

import (
	"errors"
	"fmt"
	"sort"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// KeyDamageType is the key inside a damage entry that names which damage it is.
const KeyDamageType = "type"

// damages reads what a target said it wanted broken.
//
// A list rather than one value, from the first day. Composition is a
// requirement rather than an extension, and the shape has to carry it before
// anything is written against the contract - a single value promoted to a list
// later would be a change to a public name under untouchable rule 10.
//
// An entry is a word or a mapping, which is how expected already reads. The
// word is the common case and stays short, and the mapping is there the moment
// a damage takes settings.
func damages(p *problems, where spot, raw []any) damage.Chain {
	if raw == nil {
		return nil
	}
	if len(raw) == 0 {
		// Not silently nothing. Somebody wrote the key, so they are expecting
		// broken files, and a run that quietly produced whole ones would be
		// the silence untouchable rule 6 forbids.
		p.add(where.of(KeyDamage), fmt.Sprintf("%s says damage and names none", where),
			"a target that damages nothing produces the same files as one that does not mention it",
			"name a damage, or remove the line")
		return nil
	}

	chain := make(damage.Chain, 0, len(raw))
	for i, entry := range raw {
		if spec, ok := oneDamage(p, where.entry(KeyDamage, i), entry); ok {
			chain = append(chain, spec)
		}
	}
	return chain
}

// oneDamage reads a single entry of the list.
func oneDamage(p *problems, at spot, entry any) (damage.Spec, bool) {
	switch x := entry.(type) {
	case string:
		return checkedDamage(p, at, x, damage.Values{})
	case map[string]any:
		return mappedDamage(p, at, x)
	default:
		p.add(at.of(KeyDamageType), fmt.Sprintf("%s is neither a name nor a set of settings", at),
			"a damage is written as its name, or as a mapping with type and the settings it takes",
			"write the name on its own, or a mapping starting with type")
		return damage.Spec{}, false
	}
}

// mappedDamage reads the long form: a type and the settings for it.
func mappedDamage(p *problems, at spot, x map[string]any) (damage.Spec, bool) {
	rawType, stated := x[KeyDamageType]
	if !stated {
		p.add(at.of(KeyDamageType), fmt.Sprintf("%s does not say which damage it is", at),
			"a damage written as a mapping names itself with type",
			"add type, with the name of the damage")
		return damage.Spec{}, false
	}
	id, isScalar := scalarText(rawType)
	if !isScalar {
		p.add(at.of(KeyDamageType), fmt.Sprintf("%s names a damage that is not a word", at),
			"the type of a damage is the name this build knows it by",
			"write the name as a word")
		return damage.Spec{}, false
	}

	values := damage.Values{}
	for _, key := range sortedKeysOf(x) {
		if key == KeyDamageType {
			continue
		}
		text, single := scalarText(x[key])
		if !single {
			p.add(at.of(key), fmt.Sprintf("%s: %s is a list or a block", at, key),
				"a damage setting takes one value, the way a format property does",
				"give it one value, for example bytes: 16")
			continue
		}
		values[key] = text
	}
	return checkedDamage(p, at, id, values)
}

// checkedDamage refuses a damage this build does not know, and a setting its
// declaration does not allow.
//
// The wording comes from the declaration rather than from here, which is the
// whole reason a damage parameter is a format.Property: a mistyped setting on
// a damage and a mistyped setting on a format are refused in one voice, and
// neither copies the other's sentences. Same shape as askTheFormat, one axis
// over.
func checkedDamage(p *problems, at spot, id string, values damage.Values) (damage.Spec, bool) {
	d, err := damage.Get(id)
	if err != nil {
		var unknown *damage.UnknownError
		if errors.As(err, &unknown) {
			p.add(at.of(KeyDamageType), fmt.Sprintf("%s: %s", at, unknown.What()),
				unknown.Why(), unknown.Instead())
		} else {
			p.add(at.of(KeyDamageType), fmt.Sprintf("%s: %s", at, err.Error()), "", "")
		}
		return damage.Spec{}, false
	}

	ok := true
	for _, bad := range d.CheckEach(values) {
		reportDamageSetting(p, at, bad)
		ok = false
	}
	return damage.Spec{ID: id, Values: values}, ok
}

// reportDamageSetting puts one refusal on the box it belongs to.
//
// The same three branches askTheFormat keeps, and for the reason written there:
// every problem the registry returns names its key today, and a fourth kind
// added without one has to arrive unaddressed rather than vanish.
func reportDamageSetting(p *problems, at spot, bad error) {
	var about interface{ AboutSetting() string }
	if !errors.As(bad, &about) {
		p.add(at.of(KeyDamage), bad.Error(), "", "")
		return
	}
	where := at.of(about.AboutSetting())

	var value *format.PropertyValueError
	if errors.As(bad, &value) {
		p.add(where, fmt.Sprintf("%s: %s cannot be %q", at, value.Key, value.Value),
			core.InTheWordsOf(value.Reason, value.Key), value.Remedy)
		return
	}
	var unknown *format.UnknownPropertyError
	if errors.As(bad, &unknown) {
		p.add(where, fmt.Sprintf("%s: %s", at, unknown.What()), unknown.Why(), unknown.Instead())
		return
	}
	p.add(where, fmt.Sprintf("%s: %s", at, bad.Error()), "", "")
}

// refuseImpossibleExpectation stops a target that breaks a file and expects it
// to be accepted.
//
// Only accept, and the narrowness is the decision rather than caution. reject
// is what damage implies and is what a target gets when it says nothing.
// sanitize and unspecified are both sensible questions about a broken file - a
// system under test may be expected to repair it, or that may be exactly what
// the test is asking - so neither is touched.
//
// The alternative was letting one side win quietly, and both ways of doing
// that are worse. The recipe winning puts accept in the manifest about a file
// a judge was measured to refuse, which is the tool lying in the one place its
// value lives. Damage winning overwrites what somebody wrote, and the
// regression surface says an expectation stated in a recipe reaches the
// manifest unchanged. Owner's call on 2026-09-09.
//
// The condition itself is damage's rather than this file's, since 2026-09-09.
// Asking it here as well as in the engine is what keeps this reported with the
// address of the target and beside every other problem of the same recipe -
// while the engine asking it is what covers the command line, which never
// reads a recipe at all. One rule, two callers. See O199.
func refuseImpossibleExpectation(p *problems, where spot, t Target) {
	refusal := t.Damage.ExpectationConflict(t.Expected)
	if refusal == nil {
		return
	}
	p.add(where.of(refusal.AboutSetting()),
		fmt.Sprintf("%s: %s", where, refusal.What()),
		refusal.Why(), refusal.Instead())
}

// sortedKeysOf puts the settings of one entry in a stable order, so the same
// recipe always reports the same one first.
func sortedKeysOf(x map[string]any) []string {
	out := make([]string, 0, len(x))
	for key := range x {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
