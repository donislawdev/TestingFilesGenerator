package preset

import "sort"

// Expansion is one preset settled on its parameters and turned into the recipe
// it stands for.
//
// The source is what a run consumes and what eject prints, because it is the
// same bytes - PR5 in docs/PRESETS.md. Every caller goes through Expand, so
// none of them can be the one that expands differently.
//
// It lives here rather than in a surface because there are two surfaces now.
// The command line owned this until 2026-08-05 and the window cannot import it
// - they sit on the same layer - so leaving it there meant copying it, and a
// copy carrying Defaulted would be two answers to a question untouchable rule 5
// says has one: which of these numbers did we invent.
type Expansion struct {
	Preset  Preset
	Settled Args
	// Defaulted names the parameters nobody gave, whose declared default stood
	// in. Sorted, so two runs of one preset produce identical records.
	Defaulted []string
	Source    []byte
}

// Expand settles a preset on what the caller gave and builds its recipe.
func Expand(id string, given Args) (*Expansion, error) {
	p, err := Get(id)
	if err != nil {
		return nil, err
	}
	settled, err := p.Settle(given)
	if err != nil {
		return nil, err
	}

	var defaulted []string
	for name := range p.Defaults() {
		if given[name] == "" {
			defaulted = append(defaulted, name)
		}
	}
	sort.Strings(defaulted)

	src, err := p.Expand(settled)
	if err != nil {
		return nil, err
	}
	return &Expansion{Preset: p, Settled: settled, Defaulted: defaulted, Source: src}, nil
}

// Notes is what to say out loud about a value nobody gave us.
//
// Some defaults describe our own file and some describe somebody else's system.
// A set built around a limit we invented carries expectations that read exactly
// like a set built around the real one, so the run says which number it made up.
func (e *Expansion) Notes() []string {
	var out []string
	for _, name := range e.Defaulted {
		if said := e.Preset.SaidWhenDefaulted[name]; said != "" {
			out = append(out, said)
		}
	}
	return out
}
