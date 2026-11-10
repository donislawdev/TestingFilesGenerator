// Package damage is what a file can be broken with, and the registry holding
// the kinds this build knows.
//
// The tool answers questions about SIZE and about NAME. It does not answer the
// question about bad CONTENT, and an upload validator asks three things: the
// size, the type, and whether the file opens. The third is unreachable here,
// because every file this tool writes is well formed by definition - the
// oracles see to that. Damage is the axis that makes the third question
// askable.
//
// Why it is an axis of the TARGET rather than a property of a format: a
// property is declared per format, so "corrupt" as a property would be a
// hundred declarations of one thing, a hundred registry tests, and a hundred
// places to forget. As an axis it works for a format added tomorrow without
// that format being touched. Measured on 2026-09-08 across 24 formats: five
// format agnostic damages produce 86 testable pairs with zero lines of code
// per format. docs/CORRUPTION-ARCHITECTURE-2026-09-08.md carries the numbers.
//
// Every damage is applied to every format. What the witness rule decides is
// what the manifest may SAY about the result, not whether the file is written:
// a file that breaks its format's own rules is one a reader must refuse, and
// the manifest says reject with certainty. A file whose fate depends on the
// reader - a text cut short is still text, bytes after a PNG's end are skipped
// by most viewers - is a fair question to put to a system under test, and the
// manifest says so instead of guessing. Owner's call on 2026-11-10, after
// measuring both: refusing such a file took away exactly the test somebody
// came here to run (docs/DAMAGE-ETAP-2-2026-11-09.md section 12.9).
//
// Which pairs are which is declared here, per damage (Descriptor.Policy), and
// measured by the guard TestEveryDamageIsRefusedByTheStructuralCheckInEveryFormat
// for every format and every damage registered here: a pair declared certain
// that the structural check accepts is red, and so is a pair declared
// policy-dependent that the check refuses for a reason nobody has looked at.
package damage

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// Values are the parameter values of one damage, as a recipe or a flag wrote
// them.
//
// Strings rather than a typed struct, and the same shape a format's properties
// arrive in, because it is the same question asked of a different thing: the
// declaration says what a value may be, and one piece of code answers for all
// of them.
type Values map[string]string

// Stream is one damage sitting in the write path, between the generator and
// the file.
type Stream interface {
	io.Writer

	// Touched says whether at least one byte leaving here differs from the
	// byte that came in. Read after the last write.
	//
	// A damage that changed nothing is a failed run rather than a file: the
	// manifest would say expected: reject about a file every reader accepts,
	// which is the tool lying exactly where its whole value is. So somebody
	// has to answer this question, and the cheap answer is not the obvious
	// one.
	//
	// Measured in internal/engine/parallel.go: the write path computes ONE
	// checksum, of the bytes going to the file. Comparing "before" against
	// "after" would therefore cost a SECOND sha256 of every file. A damage
	// knows for free instead - zero-head has to read the bytes before it
	// overwrites them, so the answer is a handful of byte compares. That is
	// why this sits on the damage rather than on the engine.
	Touched() bool

	// Finish is called once, after the generator's last write, in the order of
	// the chain. A damage that writes after the end of the file writes there,
	// and every other one returns nil.
	//
	// After the last write rather than on a close, because the bytes it adds
	// have to pass through the damages that come after it in the chain - and
	// the first of the list is outermost, so finishing in list order is what
	// sends them through the rest.
	Finish() error
}

// ErrKeptEnough is what a damage that cuts the file short returns once it has
// passed on every byte it keeps.
//
// The generator stops writing on it, which is the whole point: cutting 90% of
// a file would otherwise cost generating all of it while the progress bar
// stood at the end. The engine reads it as the file being complete. The bytes
// kept do not depend on whether the rest was ever generated, because a
// generator writes from the front, so stopping early changes no byte that
// reaches the disk - the guard TestATruncatedFileIsTheStartOfTheWholeOne holds
// that byte for byte.
var ErrKeptEnough = errors.New("damage: the file has every byte it keeps")

// File is what one damage is told about the file it is breaking.
type File struct {
	// Seed is this damage's own seed for this file - derived from the file's
	// seed and the damage's place in the chain, never drawn from a stream, so
	// a damage that writes bytes of its own writes the same bytes for the same
	// recipe (RC4, D11).
	Seed uint64
	// Bytes is how many bytes leave this damage.
	Bytes int64
}

// PolicyCase is a format where this damage leaves a file whose fate depends on
// the reader, so the manifest says unspecified rather than reject.
//
// Three shapes of it were measured, and each case says which in Why: the
// damage breaks nothing the format's rules forbid (a text cut short is still
// text), the only thing broken is the shape this tool writes (a CSV with no
// line break after the last row, which RFC 4180 allows), or the bytes sit
// after the format's own end, where the rules forbid them and readers skip
// them (data after a PNG's IEND).
type PolicyCase struct {
	// Format is the id of the format.
	Format string
	// When narrows the case to files whose setting holds one of the values,
	// by setting name. Empty means every file of the format.
	When map[string][]string
	// Why is the sentence the manifest carries as the detail of its
	// expectation, and tfg damage --json as the reason.
	Why string
}

// Descriptor is one kind of damage.
type Descriptor struct {
	// ID is the name a recipe writes. A public name under untouchable rule 10.
	ID string

	// Detail is the one sentence a person reads, beside the field in a window
	// and under the name on the command line.
	//
	// It says what the damage DOES to the bytes, not what it is for. A name
	// and a sentence about intent lie about half the formats already at
	// twenty four: zeroing the first eight bytes has a witness in txt, md and
	// log, which have no signature at all - the witness is the structural
	// check refusing eight zero bytes in text, not a killed magic number.
	Detail string

	// Parameters are declared the way a format declares its settings.
	//
	// The type is format.Property on purpose rather than a second mechanism.
	// It already carries the name, the kind, a range or a closed set, a
	// default and a sentence, and with it come validation, one voice of
	// refusal, exit code 4 and a field drawn by the window with nothing added
	// here. preset.Preset.Parameters is the same type for the same reason.
	Parameters []format.Property

	// Floor is the fewest bytes this damage can leave behind, for the values
	// stated.
	//
	// It exists because a file can be smaller than its own damage. Measured on
	// 2026-09-09: txt declares a minimum of 0 B and a zero byte file really is
	// produced, exit 0 - so zero-head on it would change nothing at all. That
	// has to be a refusal in the PLAN rather than a failure part way through,
	// because an invalid recipe writes no files at all.
	//
	// Counted on what LEAVES the damage, because that is the one length every
	// damage can state about itself: zero-head needs as many bytes as it
	// zeroes, truncate needs a file long enough that its cut is at least four
	// bytes, and append-junk needs room for what it appends.
	//
	// It is also what the witness matrix is measured at, which is the whole
	// reason one number does two jobs: a witness measured at 20 kB does not
	// answer for a file of 300 B, and that was measured too.
	Floor func(Values) int64

	// Length is how many bytes have to come into this damage for out bytes to
	// leave it. Nil for a damage that keeps the length, which is what makes a
	// chain of those exactly what it was before lengths could change.
	//
	// Saturating rather than failing: a result past what an int64 holds comes
	// back as math.MaxInt64, and the format it is asked of refuses that size
	// in its own words.
	Length func(v Values, out int64) int64

	// CutsTail says this damage keeps only the first bytes of what reaches it,
	// so it takes ANY input at least Length(out) long. That is what lets a
	// size the generator cannot write exactly be moved up to one it can.
	CutsTail bool

	// AddsTail says this damage writes bytes after everything that reached it.
	// A damage that cuts the tail must not come after one, or the cut takes
	// away what was added and the manifest lists a damage the file does not
	// have.
	AddsTail bool

	// Policy is where this damage leaves a file whose fate depends on the
	// reader. Everywhere else the result breaks its format's rules and the
	// manifest says reject with certainty.
	Policy []PolicyCase

	// Open builds the transformation for one file, writing on to out.
	Open func(Values, File, io.Writer) (Stream, error)
}

// Spec is one entry of a damage list: which damage, and what was said to it.
type Spec struct {
	ID     string
	Values Values
}

var (
	mu       sync.RWMutex
	registry = map[string]Descriptor{}
)

// Register adds a kind of damage. It panics on a duplicate or an incomplete
// declaration, because both are programming mistakes rather than conditions to
// handle at runtime - the same bargain the format registry makes.
func Register(d Descriptor) {
	mu.Lock()
	defer mu.Unlock()

	if d.ID == "" {
		panic("damage: registered a descriptor with no id")
	}
	if _, exists := registry[d.ID]; exists {
		panic(fmt.Sprintf("damage: %q is registered twice", d.ID))
	}
	if d.Open == nil {
		panic(fmt.Sprintf("damage: %q has nothing to apply", d.ID))
	}
	if d.Floor == nil {
		panic(fmt.Sprintf("damage: %q does not say how small a file it can break", d.ID))
	}
	if d.Detail == "" {
		panic(fmt.Sprintf("damage: %q has no sentence describing it", d.ID))
	}
	if d.CutsTail && d.Length == nil {
		panic(fmt.Sprintf("damage: %q cuts the tail and does not say how long a file it needs", d.ID))
	}
	for _, c := range d.Policy {
		if c.Format == "" || c.Why == "" {
			panic(fmt.Sprintf("damage: %q declares a policy case with no format or no reason", d.ID))
		}
	}
	for i := range d.Parameters {
		format.SortChoices(d.Parameters[i].Choices)
	}
	registry[d.ID] = d
}

// Get is one kind of damage by the name a recipe writes.
func Get(id string) (Descriptor, error) {
	mu.RLock()
	defer mu.RUnlock()

	d, ok := registry[id]
	if !ok {
		return Descriptor{}, &UnknownError{ID: id, Known: names()}
	}
	return d, nil
}

// All is every kind of damage this build knows, in a stable order.
func All() []Descriptor {
	mu.RLock()
	defer mu.RUnlock()

	out := make([]Descriptor, 0, len(registry))
	for _, id := range names() {
		out = append(out, registry[id])
	}
	return out
}

// Names is every kind of damage by name, in a stable order.
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	return names()
}

// names is the sorted ids. The caller holds the lock.
func names() []string {
	out := make([]string, 0, len(registry))
	for id := range registry {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
