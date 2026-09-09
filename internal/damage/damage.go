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
// What this package does NOT do is decide whether a damaged file is worth
// writing. That is the witness rule - a pair with no judge able to refuse the
// result is a pair this tool does not offer - and it belongs with the oracles,
// because it is a measurement rather than a declaration.
package damage

import (
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

	// Floor is the smallest file this damage can be given, for the values
	// stated.
	//
	// It exists because a file can be smaller than its own damage. Measured on
	// 2026-09-09: txt declares a minimum of 0 B and a zero byte file really is
	// produced, exit 0 - so zero-head on it would change nothing at all. That
	// has to be a refusal in the PLAN rather than a failure part way through,
	// because an invalid recipe writes no files at all.
	//
	// It is also what the witness matrix is measured at, which is the whole
	// reason one number does two jobs: a witness measured at 20 kB does not
	// answer for a file of 300 B, and that was measured too.
	Floor func(Values) int64

	// Open builds the transformation for one file, writing on to out.
	Open func(Values, io.Writer) (Stream, error)
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
