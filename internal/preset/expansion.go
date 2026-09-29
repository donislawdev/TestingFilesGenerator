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
	spoken := e.Spoken()
	out := make([]string, 0, len(spoken))
	for _, n := range spoken {
		out = append(out, n.Said)
	}
	return out
}

// Note is one thing a run says about a set, and the parameter it is about when
// it is about a value nobody gave - which is what lets a window find the
// sentence in its own language. About is empty for what a preset says about
// the set as a whole: those sentences are put together from the values, so
// there is nothing fixed to translate them from.
type Note struct {
	About string
	Said  string
}

// Spoken is Notes with what each one is about, in the same order.
func (e *Expansion) Spoken() []Note {
	var out []Note
	for _, name := range e.Defaulted {
		if said := e.Preset.SaidWhenDefaulted[name]; said != "" {
			out = append(out, Note{About: name, Said: said})
		}
	}
	// What those values then laid out, after what we invented, because a
	// sentence about the set reads as the consequence of the numbers above it.
	if e.Preset.Says != nil {
		for _, said := range e.Preset.Says(e.Settled) {
			out = append(out, Note{Said: said})
		}
	}
	return out
}
