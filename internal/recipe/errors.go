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
}

func (p Problem) String() string {
	return fmt.Sprintf("%s - %s.\n  %s.", p.What, p.Why, p.Fix)
}

// ValidationError is a recipe that parsed but does not describe a run this
// build can carry out.
type ValidationError struct {
	Name     string
	Problems []Problem
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

func (p *problems) add(what, why, fix string) {
	p.list = append(p.list, Problem{What: what, Why: why, Fix: fix})
}

// notYet is a key the recipe document describes and this build cannot honour.
//
// It is its own kind of message because the answer is different: the recipe is
// not wrong, the tool is not there yet. Saying "unknown key" would send the
// reader looking for a typo that does not exist.
func (p *problems) notYet(key, why, fix string) {
	p.add(fmt.Sprintf("%s is not in this build yet", key), why, fix)
}

func (p *problems) err() error {
	if len(p.list) == 0 {
		return nil
	}
	return &ValidationError{Name: p.name, Problems: p.list}
}
