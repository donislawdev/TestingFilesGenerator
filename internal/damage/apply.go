package damage

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
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
	if why.IsZero() {
		return nil
	}
	return &format.PropertyValueError{
		Format: d.ID, Key: p.Name, Value: raw, Reason: why, Remedy: p.Instead(),
	}
}

// Check is the first problem with what this chain states: a damage this build
// does not know, or a setting its declaration does not take or does not allow.
//
// The engine asks it, for the command line and the window, which hand it a
// chain without reading a recipe. The recipe reader asks CheckEach of every
// entry and reports each problem with its address, so this is a second caller
// of the same rule rather than a copy of it - the shape O199 gave the
// expectation. The first problem only, as format.Descriptor.CheckProperties
// gives it beside this: a window draws one damage at a time, and the engine
// addresses one refusal per target.
//
// Asked before a byte is written, because a value past the declaration does
// not fail later. The settings are read with a fallback to the default
// (missingShare, junkBytes, headBytes), so without this the file would be cut
// by the default and the manifest would record the value as written.
func (c Chain) Check() error {
	for i, s := range c {
		d, err := Get(s.ID)
		if err != nil {
			return err
		}
		if bad := d.CheckEach(s.Values); len(bad) > 0 {
			return &SettingError{Position: i + 1, Err: bad[0]}
		}
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

// step is one entry of a chain with its declaration and its settings resolved,
// which every question below needs and none should ask the registry twice for.
type step struct {
	d      Descriptor
	values Values
}

func (c Chain) steps() ([]step, error) {
	out := make([]step, 0, len(c))
	for _, s := range c {
		d, err := Get(s.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, step{d: d, values: withDefaults(d, s.Values)})
	}
	return out, nil
}

// lengths is how many bytes leave each damage for a file of size bytes on
// disk, in the order of the chain, and how many the generator writes. ok is
// false when some damage would be left fewer bytes than its floor, or handed
// fewer than none.
//
// Worked from the END of the chain, because the size somebody asked for is
// what leaves the last damage, and each damage says how much it needs to be
// given for what it hands on.
func lengths(steps []step, size int64) (outs []int64, generator int64, ok bool) {
	outs = make([]int64, len(steps))
	at := size
	ok = true
	for i := len(steps) - 1; i >= 0; i-- {
		outs[i] = at
		if at < steps[i].d.Floor(steps[i].values) {
			ok = false
		}
		if steps[i].d.Length != nil {
			at = steps[i].d.Length(steps[i].values, at)
		}
		if at < 0 {
			ok = false
			at = 0
		}
	}
	return outs, at, ok
}

// lengthCeiling is where the search for a floor stops. Far past any size a
// run accepts, and far enough below math.MaxInt64 that doubling towards it
// cannot wrap.
const lengthCeiling = int64(1) << 60

// Floor is the smallest file on disk every damage in the chain can be applied
// to, and the damage that sets it.
//
// For a chain whose damages keep the length this is the largest of their
// floors, which is what it was before lengths could change. A damage that
// changes the length makes the ones before it see a different file, so the
// answer is searched for rather than added up: what leaves each damage grows
// with the size asked for, so the smallest size that leaves every damage its
// floor is one number, found by halving.
func (c Chain) Floor() (int64, string, error) {
	steps, err := c.steps()
	if err != nil || len(steps) == 0 {
		return 0, "", err
	}
	if _, _, ok := lengths(steps, 0); ok {
		return 0, "", nil
	}
	lo, hi := int64(0), int64(1)
	for {
		if _, _, ok := lengths(steps, hi); ok {
			break
		}
		if hi >= lengthCeiling {
			return lengthCeiling, steps[len(steps)-1].d.ID, nil
		}
		lo, hi = hi, hi*2
	}
	// lo fails and hi passes, so the floor is in (lo, hi].
	for hi-lo > 1 {
		mid := lo + (hi-lo)/2
		if _, _, ok := lengths(steps, mid); ok {
			hi = mid
		} else {
			lo = mid
		}
	}
	return hi, ownerOfFloor(steps, lo), nil
}

// ownerOfFloor is the damage that refuses a file of size bytes - the one a
// refusal names. The last one that does, because a refusal speaks about the
// size somebody wrote, and the last damage is the one that sees it.
func ownerOfFloor(steps []step, size int64) string {
	outs, _, _ := lengths(steps, size)
	owner := steps[len(steps)-1].d.ID
	for i := len(steps) - 1; i >= 0; i-- {
		if outs[i] < steps[i].d.Floor(steps[i].values) {
			return steps[i].d.ID
		}
	}
	return owner
}

// Before is how many bytes the generator writes for a file of size bytes on
// disk. It refuses a size below the chain's floor, in the same words the plan
// does.
func (c Chain) Before(size int64) (int64, error) {
	steps, err := c.steps()
	if err != nil {
		return 0, err
	}
	_, generator, ok := lengths(steps, size)
	if !ok {
		floor, owner, err := c.Floor()
		if err != nil {
			return 0, err
		}
		return 0, &TooSmallError{Damage: owner, Requested: size, Floor: floor}
	}
	return generator, nil
}

// ChangesLength says whether the file the generator writes is a different
// length from the one on disk.
func (c Chain) ChangesLength() bool {
	steps, err := c.steps()
	if err != nil {
		return false
	}
	for _, s := range steps {
		if s.d.Length != nil {
			return true
		}
	}
	return false
}

// Stretches says the generator may write MORE than Before asks for and the
// file on disk still comes out the size asked for.
//
// True when a damage cuts the tail and nothing before it changes the length:
// whatever the generator writes past the cut is dropped, so a size the format
// cannot write exactly - a band a PNG cannot reach, an odd number in UTF-16 -
// can be moved up to one it can. That is what makes truncate's missing an
// "at least" (owner's call, 2026-11-10).
func (c Chain) Stretches() bool {
	steps, err := c.steps()
	if err != nil {
		return false
	}
	for _, s := range steps {
		if s.d.CutsTail {
			return true
		}
		if s.d.Length != nil {
			return false
		}
	}
	return false
}

// Open builds the chain as one writer for a file of size bytes on disk, with
// the first damage of the list outermost so that the bytes meet them in the
// order written. seed is the file's own seed, and each damage is handed one
// derived from it and its place in the list.
//
// Returns the streams as well, because whether each one changed anything is a
// question asked per damage rather than once at the end - two damages can
// cancel out, and a chain whose second member was idle would otherwise pass.
func (c Chain) Open(out io.Writer, size int64, seed uint64) (io.Writer, []Stream, error) {
	steps, err := c.steps()
	if err != nil {
		return nil, nil, err
	}
	outs, _, _ := lengths(steps, size)
	w := out
	streams := make([]Stream, len(steps))
	// Built back to front so that the first entry ends up outermost.
	for i := len(steps) - 1; i >= 0; i-- {
		s, err := steps[i].d.Open(steps[i].values, File{Seed: core.DamageSeed(seed, i), Bytes: outs[i]}, w)
		if err != nil {
			return nil, nil, err
		}
		streams[i] = s
		w = s
	}
	return w, streams, nil
}

// Finish ends every damage of the chain in the order of the list, after the
// generator's last write.
func (c Chain) Finish(streams []Stream) error {
	for _, s := range streams {
		if err := s.Finish(); err != nil {
			return err
		}
	}
	return nil
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

// LabelLimit is the most bytes the damage takes up in a label.
//
// The label lands in fields with limits of their own, and the tightest one
// measured is a gzip comment, which Go's reader refuses from 512 bytes
// (compress/gzip, a 512 byte buffer). A chain has no length limit of its own,
// so without this a long one would break a format before any damage was
// applied - an archive Go cannot open, a ZIP comment past its padding. Bounded
// here, once, so a format has one length to be ready for rather than every
// length a recipe can write (TestALongDamageLabelLeavesEveryFormatWhole).
const LabelLimit = 160

// Label is the chain as the label of a damaged file states it: every entry in
// order, its settings resolved the way the manifest records them, joined by
// " then " - "truncate:missing=50 then append-junk:bytes=1024". Empty for an
// empty chain, which is a file whose label does not change.
//
// Resolved rather than as written, for the reason Resolved gives: the label
// answers what was done to this file, and its reader should not need the
// defaults of the build that wrote it. ASCII whatever a setting holds, because
// the label goes into encodings where nothing else is sure to survive
// (format/label.go): a value with anything else in it, or with a space or one
// of the separators, is written quoted with escapes.
//
// At most LabelLimit bytes. A chain longer than that names the entries that
// fit and how many more there are - "... and 12 more" - and the manifest lists
// every one. An entry too long to fit on its own is named by its damage alone.
func (c Chain) Label() string {
	out := make([]string, 0, len(c))
	for _, s := range c {
		values := Values{}
		for name, value := range s.Resolved() {
			values[name] = plainOrQuoted(value)
		}
		out = append(out, Spec{ID: s.ID, Values: values}.String())
	}
	if whole := strings.Join(out, " then "); len(whole) <= LabelLimit {
		return whole
	}
	return shortened(c, out)
}

// shortened is the entries of a chain that fit in LabelLimit with room left
// for saying how many did not.
func shortened(c Chain, entries []string) string {
	kept := ""
	for i, entry := range entries {
		if i == 0 && len(entry) > LabelLimit/2 {
			entry = c[0].ID
		}
		next := entry
		if kept != "" {
			next = kept + " then " + entry
		}
		if len(next)+len(moreThan(len(entries)-i-1)) > LabelLimit {
			return kept + moreThan(len(entries)-i)
		}
		kept = next
	}
	return kept
}

// moreThan is what a shortened label says about the entries it leaves out.
func moreThan(n int) string {
	if n == 0 {
		return ""
	}
	return " and " + strconv.Itoa(n) + " more"
}

// plainOrQuoted is v as it is when every byte of it is printable ASCII other
// than the separators a label entry is written with, and quoted otherwise.
func plainOrQuoted(v string) string {
	for i := 0; i < len(v); i++ {
		if b := v[i]; b <= ' ' || b > '~' || b == ',' || b == '=' || b == ':' || b == '"' {
			return strconv.QuoteToASCII(v)
		}
	}
	return v
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
