package format

import (
	"fmt"
	"slices"
	"strings"
)

// Condition is a setting that changes a file only while another setting of
// the same declaration has one of a few values: the colours of a placeholder
// change nothing in a picture whose content is the test pattern.
//
// Declared rather than known by the places that care, because three of them
// do. The format refuses such a setting while it would change nothing, since a
// value read and ignored is a colour somebody waits for in silence (rule 6).
// A window hides the field, since a field that cannot change the file is a
// field nobody should have to read past - the owner's decision of 2026-11-17
// (docs/SIZE-FIXES-2026-11-16.md section 6.5). And tfg formats says so beside
// the setting, so the command line learns it before the refusal does.
type Condition struct {
	// Setting is the one this depends on: a choice declared earlier in the
	// same list, so a form reads down from the switch to what it shows.
	Setting string
	// Values are the choices of Setting under which this one applies.
	Values []string
}

// IsZero is whether there is no condition, so the setting always applies.
func (c Condition) IsZero() bool { return c.Setting == "" }

// Describe is the condition in the words tfg formats prints after what a
// setting takes.
func (c Condition) Describe() string {
	return fmt.Sprintf("only when %s is %s", c.Setting, strings.Join(c.Values, " or "))
}

// Applies reports whether p changes anything while the settings declared
// beside it have the values value gives. A setting left out, which value
// answers with nothing, has its default - the value a format reads it as.
func (p Property) Applies(declared []Property, value func(name string) string) bool {
	c := p.OnlyWhen
	if c.IsZero() {
		return true
	}
	v := value(c.Setting)
	if v == "" {
		v = defaultOf(declared, c.Setting)
	}
	return slices.Contains(c.Values, v)
}

// defaultOf is the default of the setting declared under name, or nothing.
func defaultOf(declared []Property, name string) string {
	for _, q := range declared {
		if q.Name == name {
			return q.Default
		}
	}
	return ""
}

// AnyConditional is whether one of props declares a condition. A damage, a
// preset and a tool declare their settings as Property too, and none of their
// engines refuses a setting that changes nothing - so their registries refuse
// the declaration, rather than letting a window hide a field the command line
// still takes in silence.
func AnyConditional(props []Property) bool {
	return slices.ContainsFunc(props, func(p Property) bool { return !p.OnlyWhen.IsZero() })
}

// ConditionProblem is what is wrong with a condition in props that nothing
// could keep, or nothing when every one can be kept. A condition on a setting
// declared later, or on one that is not a closed set, is one a window could
// not show and hide by, and one naming a value the setting does not have can
// never hold. One covering every value always holds and says nothing, and one
// on a setting with a condition of its own would hide a field behind a field
// that is itself hidden.
func ConditionProblem(props []Property) string {
	for i, p := range props {
		c := p.OnlyWhen
		if c.IsZero() {
			continue
		}
		at := slices.IndexFunc(props[:i], func(q Property) bool { return q.Name == c.Setting })
		switch {
		case at < 0:
			return fmt.Sprintf("%s depends on %s, which is not declared before it", p.Name, c.Setting)
		case props[at].Kind != PropertyChoice:
			return fmt.Sprintf("%s depends on %s, which is not a choice", p.Name, c.Setting)
		case !props[at].OnlyWhen.IsZero():
			return fmt.Sprintf("%s depends on %s, which depends on another setting itself", p.Name, c.Setting)
		case len(c.Values) == 0 || len(c.Values) >= len(props[at].Choices):
			return fmt.Sprintf("%s depends on %s holding some of its values and not all of them", p.Name, c.Setting)
		}
		choices := props[at].Choices
		if bad := slices.IndexFunc(c.Values, func(v string) bool { return !slices.Contains(choices, v) }); bad >= 0 {
			return fmt.Sprintf("%s depends on %s being %s, which it cannot be", p.Name, c.Setting, c.Values[bad])
		}
	}
	return ""
}
