package recipe

import (
	"fmt"
	"strings"
)

// A recipe is refused with every problem it has, not the first one.
//
// Fixing a file one error per run is the cheapest way to make somebody stop
// using the tool, and a recipe written by hand usually has more than one thing
// wrong with it the first time.

// Problem is one thing wrong with a recipe.
//
// It carries three parts on purpose: what is wrong, why that rule exists, and
// what to do instead. An error that only states the first leaves the reader to
// guess the other two.
type Problem struct {
	What string
	Why  string
	Fix  string
	// At is the setting this problem is about, in the vocabulary the two
	// surfaces share: a recipe key, with a 1-based index wherever a list is
	// involved, as in targets[2].size or targets[2].contains[1].format.
	//
	// It exists because prose cannot be pointed at a box. A window draws a
	// field per setting and has to mark the one that was refused, and the
	// sentence a person reads names a target by its id - which is the right
	// thing to read and the wrong thing to look a widget up by, since the box
	// holding a missing id has no id in it yet.
	//
	// Empty for a problem about the document as a whole rather than one of its
	// settings. A reader of this field has to expect that and put such a
	// refusal where refusals about the whole run go.
	At string
}

func (p Problem) String() string {
	return fmt.Sprintf("%s - %s.\n  %s.", p.What, p.Why, p.Fix)
}

// Error makes one problem an error in its own right, so a refused recipe can be
// handed on as the problems it holds rather than as one block of text.
func (p Problem) Error() string { return p.String() }

// AboutSetting is the address, under the name the surfaces already ask for it
// by.
//
// The window places a refusal by asking whether it implements this - see
// runner.refuse in internal/gui/window - and the engine, the format registry
// and the preset package all answer it already. Answering it here means a
// refused recipe is placed the same way as everything else rather than by a
// branch that knows what a recipe problem is.
func (p Problem) AboutSetting() string { return p.At }

// ValidationError is a recipe that parsed but does not describe a run this
// build can carry out.
type ValidationError struct {
	Name     string
	Problems []Problem
}

// Unwrap hands out the problems as errors, so a caller that places refusals one
// by one can place all of them instead of the first.
//
// The window already opens a joined error into the ones it carries and looks
// each up by the setting it is about, which is how a form with three bad boxes
// marks three. A refused recipe was the one refusal that arrived as a single
// error, so all of it landed at the foot of the form however many settings it
// named. This closes that without the window learning what a recipe is.
//
// The message of the whole is unchanged: it still lists every problem, because
// the command line prints the error and a reader there wants the list.
func (e *ValidationError) Unwrap() []error {
	out := make([]error, 0, len(e.Problems))
	for _, p := range e.Problems {
		out = append(out, p)
	}
	return out
}

func (e *ValidationError) Error() string {
	var b strings.Builder
	word := "problems"
	if len(e.Problems) == 1 {
		word = "problem"
	}
	fmt.Fprintf(&b, "%s has %d %s and nothing was written:", e.Name, len(e.Problems), word)
	for _, p := range e.Problems {
		fmt.Fprintf(&b, "\n\n- %s", p.String())
	}
	return b.String()
}

// SyntaxError is a file that is not readable as YAML at all, or that carries a
// key no version of the recipe schema has.
//
// The message comes from the parser, which points at the line and the column.
// Rewriting it would lose that.
type SyntaxError struct {
	Name   string
	Detail string
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("%s cannot be read as a recipe:\n%s", e.Name, e.Detail)
}

// problems collects everything wrong with one recipe.
type problems struct {
	name string
	list []Problem
}

func (p *problems) add(at, what, why, fix string) {
	p.list = append(p.list, Problem{What: what, Why: why, Fix: fix, At: at})
}

// notYet is a key the recipe document describes and this build cannot honour.
//
// It is its own kind of message because the answer is different: the recipe is
// not wrong, the tool is not there yet. Saying "unknown key" would send the
// reader looking for a typo that does not exist.
func (p *problems) notYet(key, why, fix string) {
	p.add(key, fmt.Sprintf("%s is not in this build yet", key), why, fix)
}

// notYetIn is the same for a key inside a target, where the sentence names the
// target and the address does not - see spot.
func (p *problems) notYetIn(where spot, setting, why, fix string) {
	p.add(where.of(setting), fmt.Sprintf("%s: %s is not in this build yet", where, setting), why, fix)
}

// spot is one place in a recipe, in the two vocabularies this tool needs at
// once: the prose a person reads and the address a window marks a box by.
//
// One string cannot do both, and the reason is not tidiness. The prose names a
// target by its id as soon as it has one, because that is what somebody
// recognises in a list of twenty targets. The address has to stay positional,
// because the box that holds a missing id is drawn before anybody types one
// into it - so the refusal about it cannot be addressed by the id it lacks.
//
// It renders as the prose, so every message built with %s reads as it did
// before this type existed.
type spot struct {
	says string
	key  string
}

func (s spot) String() string { return s.says }

// of names a setting inside this spot: targets[2] and "size" make
// targets[2].size. A dotted setting is passed through, for the settings that
// have a part of their own such as expected.reason.
func (s spot) of(setting string) string {
	if s.key == "" {
		return setting
	}
	return s.key + "." + setting
}

// entry names one item of a list inside this spot, counted from one the way the
// prose counts, so the two halves agree about which entry is meant.
func (s spot) entry(list string, index int) spot {
	return spot{
		says: fmt.Sprintf("%s: %s entry %d", s.says, list, index+1),
		key:  fmt.Sprintf("%s.%s[%d]", s.key, list, index+1),
	}
}

// targetSpot is where one entry of the targets list is.
func targetSpot(index int, id string) spot {
	s := spot{
		says: fmt.Sprintf("target %d", index+1),
		key:  fmt.Sprintf("targets[%d]", index+1),
	}
	if id != "" {
		s.says = fmt.Sprintf("target %q", id)
	}
	return s
}

func (p *problems) err() error {
	if len(p.list) == 0 {
		return nil
	}
	return &ValidationError{Name: p.name, Problems: p.list}
}
