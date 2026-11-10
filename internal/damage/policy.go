package damage

import (
	"slices"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// Kind is the format of a file and every one of its settings, defaults filled
// in - what a chain is asked about when the manifest needs to know whether the
// result breaks its format's rules.
//
// Defaults filled in rather than the settings as stated, because a policy case
// narrowed to one value has to match a file that got that value by default as
// surely as one that asked for it.
type Kind struct {
	Format   string
	Settings map[string]string
}

// KindOf is a format with the settings stated for it, defaults filled in.
func KindOf(d format.Descriptor, stated map[string]string) Kind {
	settings := make(map[string]string, len(d.Properties))
	for _, p := range d.Properties {
		if p.Default != "" {
			settings[p.Name] = p.Default
		}
	}
	for name, value := range stated {
		if value != "" {
			settings[name] = value
		}
	}
	return Kind{Format: d.ID, Settings: settings}
}

// matches says whether this case covers a file of this kind.
func (c PolicyCase) matches(k Kind) bool {
	if c.Format != k.Format {
		return false
	}
	for name, values := range c.When {
		if !slices.Contains(values, k.Settings[name]) {
			return false
		}
	}
	return true
}

// PolicyFor is the case covering this damage on a file of this kind, or false
// when the damage breaks the format's rules there.
func (d Descriptor) PolicyFor(k Kind) (PolicyCase, bool) {
	for _, c := range d.Policy {
		if c.matches(k) {
			return c, true
		}
	}
	return PolicyCase{}, false
}

// Certain says whether the chain leaves a file of this kind that its format's
// rules forbid. When it does not, why is the sentence the manifest carries.
//
// Certain as soon as ONE damage is, because what it broke stays broken after
// the next: a file cut short after its head was zeroed still has no
// signature. Measured per pair rather than per chain - the witness guard runs
// each damage on its own - and that limit is written down in
// docs/DAMAGE-ETAP-2-2026-11-09.md section 12.9.
func (c Chain) Certain(k Kind) (certain bool, why string, err error) {
	steps, err := c.steps()
	if err != nil {
		return false, "", err
	}
	for _, s := range steps {
		pc, depends := s.d.PolicyFor(k)
		if !depends {
			return true, "", nil
		}
		if why == "" {
			why = pc.Why
		}
	}
	return false, why, nil
}
