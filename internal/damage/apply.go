package damage

import (
	"fmt"
	"io"
	"sort"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// Defaults are the declared defaults, for the values a caller left out.
func (d Descriptor) Defaults() Values {
	out := Values{}
	for _, p := range d.Parameters {
		if p.Default != "" {
			out[p.Name] = p.Default
		}
	}
	return out
}

// ParameterNames is what this damage takes, in the declared order.
func (d Descriptor) ParameterNames() []string {
	out := make([]string, 0, len(d.Parameters))
	for _, p := range d.Parameters {
		out = append(out, p.Name)
	}
	return out
}

// CheckEach is every problem with the values stated, in a stable order.
//
// All of them rather than the first, because RC7 says a recipe comes back with
// everything wrong with it. The wording is not written here: Allows and Instead
// are methods on the declaration, so a damage parameter and a format setting
// refuse in the same voice without either copying the other's sentences.
//
// The refusal type is format.PropertyValueError, whose first field is called
// Format and here holds the id of a damage. The field name is narrow and the
// type is right - it is the one refusal in this tree that a form can take
// apart by name - so it is used as it is and the narrowness is written down
// rather than fixed by a rename that would touch every format.
func (d Descriptor) CheckEach(v Values) []error {
	declared := make(map[string]format.Property, len(d.Parameters))
	for _, p := range d.Parameters {
		declared[p.Name] = p
	}

	stated := make([]string, 0, len(v))
	for name := range v {
		stated = append(stated, name)
	}
	sort.Strings(stated)

	var bad []error
	for _, name := range stated {
		p, known := declared[name]
		if !known {
			bad = append(bad, &format.UnknownPropertyError{
				Format: d.ID, Key: name, Known: d.ParameterNames(),
			})
			continue
		}
		if wrong := d.checkOne(p, v[name]); wrong != nil {
			bad = append(bad, wrong)
		}
	}
	return bad
}

// checkOne is one stated value against one declaration.
//
// Lifted out of the loop above rather than nested inside it, because the shape
// gates count how deep a reader has to follow and three was the line. It is
// also the whole of what "is this value allowed" means here, which is a better
// reason to have it than the count.
//
// An empty value means "not stated", the same as leaving it out - that is what
// an unset flag and an empty recipe entry both look like by the time they
// arrive here.
func (d Descriptor) checkOne(p format.Property, raw string) error {
	if raw == "" {
		return nil
	}
	why := p.Allows(raw)
	if why == "" {
		return nil
	}
	return &format.PropertyValueError{
		Format: d.ID, Key: p.Name, Value: raw, Reason: why, Remedy: p.Instead(),
	}
}

// Check is the first problem with the values stated, for the one-target path
// from the command line flags where there is no form to lay four of them out
// on.
func (d Descriptor) Check(v Values) error {
	if bad := d.CheckEach(v); len(bad) > 0 {
		return bad[0]
	}
	return nil
}

// Chain is a list of damages in the order they are applied.
//
// A list from the first day rather than a single value, because composition is
// a requirement rather than an extension, and because the shape has to carry it
// before anything is written against the contract. Order is significant and
// recorded: truncating after appending a tail removes what was appended, so the
// same two entries in the other order are different bytes, and D11 promises
// those bytes do not move.
type Chain []Spec

// Floor is the smallest file every damage in the chain can be applied to.
//
// The largest of the floors rather than their sum: each damage is applied to
// the whole file that reaches it, so a chain is bounded by its most demanding
// member. That is true for length preserving damage, which is all this build
// has - a length CHANGING damage makes the later members see a different file,
// and that arithmetic arrives with truncate rather than being guessed at here.
func (c Chain) Floor() (int64, string, error) {
	var floor int64
	var owner string
	for _, s := range c {
		d, err := Get(s.ID)
		if err != nil {
			return 0, "", err
		}
		if f := d.Floor(withDefaults(d, s.Values)); f > floor {
			floor, owner = f, s.ID
		}
	}
	return floor, owner, nil
}

// Open builds the chain as one writer, with the first damage of the list
// outermost so that the bytes meet them in the order written.
//
// Returns the streams as well, because whether each one changed anything is a
// question asked per damage rather than once at the end - two damages can
// cancel out, and a chain whose second member was idle would otherwise pass.
func (c Chain) Open(out io.Writer) (io.Writer, []Stream, error) {
	w := out
	streams := make([]Stream, 0, len(c))
	// Built back to front so that the first entry ends up outermost.
	for i := len(c) - 1; i >= 0; i-- {
		d, err := Get(c[i].ID)
		if err != nil {
			return nil, nil, err
		}
		s, err := d.Open(withDefaults(d, c[i].Values), w)
		if err != nil {
			return nil, nil, err
		}
		streams = append(streams, s)
		w = s
	}
	// Back into the order of the list, so a caller reporting on stream i is
	// reporting on entry i.
	for l, r := 0, len(streams)-1; l < r; l, r = l+1, r-1 {
		streams[l], streams[r] = streams[r], streams[l]
	}
	return w, streams, nil
}

// Idle is the first damage of the chain that changed nothing, by name, or an
// empty string when every one of them moved a byte.
func (c Chain) Idle(streams []Stream) string {
	for i, s := range streams {
		if i < len(c) && !s.Touched() {
			return c[i].ID
		}
	}
	return ""
}

// withDefaults fills in what the caller left out, so a damage never has to ask
// twice whether a value was stated.
func withDefaults(d Descriptor, v Values) Values {
	out := d.Defaults()
	for name, value := range v {
		if value != "" {
			out[name] = value
		}
	}
	return out
}

// String is the chain as a person writes it, for a message and for the record
// of the command a run was made with.
func (c Chain) String() string {
	out := make([]string, 0, len(c))
	for _, s := range c {
		out = append(out, s.String())
	}
	return fmt.Sprint(out)
}

// Resolved is the settings this entry was applied with, defaults filled in.
//
// Filled in rather than left as written, because the manifest answers "what
// was done to this file" and a reader of it should not have to know what the
// default was in the build that wrote it. Nil when the damage takes no
// settings, so the key is absent rather than empty.
func (s Spec) Resolved() map[string]string {
	d, err := Get(s.ID)
	if err != nil {
		// A damage the registry does not know cannot reach here through either
		// surface - the recipe refuses it and the flags refuse it. Returning
		// what was stated beats returning nothing, because a record of an
		// impossible state should still say what it saw.
		if len(s.Values) == 0 {
			return nil
		}
		return s.Values
	}
	out := withDefaults(d, s.Values)
	if len(out) == 0 {
		return nil
	}
	return out
}

// String is one entry as a person writes it on the command line.
func (s Spec) String() string {
	if len(s.Values) == 0 {
		return s.ID
	}
	names := make([]string, 0, len(s.Values))
	for name := range s.Values {
		names = append(names, name)
	}
	sort.Strings(names)

	out := s.ID
	for i, name := range names {
		sep := ","
		if i == 0 {
			sep = ":"
		}
		out += sep + name + "=" + s.Values[name]
	}
	return out
}
