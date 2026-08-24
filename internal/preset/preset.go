// Package preset turns a named test question into a recipe.
//
// A preset is a function from parameters to a recipe and nothing more. PR5 in
// docs/PRESETS.md says there are no closed presets - "tfg preset eject" gives
// back an editable recipe - and the cleanest way to keep that true is for
// expansion to produce recipe source rather than a structure.
//
// So a preset returns YAML, and the same parser reads it that reads a file
// somebody wrote by hand. What eject prints and what a run consumes are then
// the same bytes, and they cannot drift apart, because there is only one of
// them.
package preset

import (
	"fmt"
	"sort"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// Args are the parameter values a caller supplied, by name. A parameter the
// caller left out is absent rather than empty, so a preset can tell "not
// stated" from "stated as nothing" - the same distinction Request draws for
// format properties.
type Args map[string]string

// Preset is everything a preset announces about itself.
//
// The shape follows the anatomy in docs/PRESETS.md section 3. Parameters reuse
// the format package's Property rather than declaring a second vocabulary for
// the same idea: a name, a kind, a default and a sentence, with one
// implementation deciding what values are allowed and one wording refusing the
// rest. A window drawing a preset field then works the same way as one drawing
// a format field.
type Preset struct {
	ID string
	// Title is for people and is translated in a window. The id is not.
	Title string
	// Question is the test question this preset closes, and it is the sentence
	// the "what are you testing" wizard shows.
	Question string

	Parameters []format.Property

	// Reads names the global flags this preset gives a value to without
	// declaring a parameter of its own.
	//
	// It exists because of a rule and a collision. CLI.md section 6 says a
	// preset parameter whose name clashes with an existing flag is a mistake
	// in the preset, and size-boundaries wanted a --format parameter while
	// --format is already the flag naming the format of a file. They mean the
	// same thing, so the preset supplies a value through the precedence chain
	// instead of declaring a second one - and this field is what lets "preset
	// show" still list it. A setting nobody knows they can change is a setting
	// that is not there.
	Reads []string

	// Requires are the modules this preset needs, from docs/BACKLOG.md.
	Requires []string
	// Catches is what this preset typically finds, for the explain mode.
	Catches []string

	// SaidWhenDefaulted is what to say out loud when a parameter was left out
	// and its declared default stood in, keyed by parameter name.
	//
	// Some defaults describe our own file and some describe somebody else's
	// system. The limit of an upload form is theirs, and a number we chose for
	// it produces a set that looks right, carries expectations that read as
	// certain, and says nothing about the system under test. Refusing to run
	// would be honest and useless. Running silently would be neither. So it
	// runs and says which number it invented.
	SaidWhenDefaulted map[string]string

	// Expand builds the recipe. It returns source rather than a structure, so
	// what a run consumes is what eject prints.
	Expand func(Args) ([]byte, error)
}

// Defaults are the declared defaults, for the values a caller left out.
func (p Preset) Defaults() Args {
	out := Args{}
	for _, param := range p.Parameters {
		if param.Default != "" {
			out[param.Name] = param.Default
		}
	}
	return out
}

// Check refuses a name this preset does not declare, and a value its
// declaration does not allow. Same shape and same wording as the format
// registry, because it is the same question asked of a different thing.
func (p Preset) Check(args Args) error {
	known := make(map[string]format.Property, len(p.Parameters))
	for _, param := range p.Parameters {
		known[param.Name] = param
	}
	names := make([]string, 0, len(args))
	for name := range args {
		names = append(names, name)
	}
	sort.Strings(names)

	reads := map[string]bool{}
	for _, name := range p.Reads {
		reads[name] = true
	}

	for _, name := range names {
		param, ok := known[name]
		if !ok {
			// A global flag this preset reads is a legitimate name here. Its
			// value is checked by whoever owns the flag - the format registry
			// answers for --format - so it is not checked twice with two
			// wordings.
			if reads[name] {
				continue
			}
			return &UnknownParameterError{Preset: p.ID, Name: name, Known: p.ParameterNames()}
		}
		if raw := args[name]; raw != "" {
			if bad := param.Allows(raw); bad != "" {
				return &format.PropertyValueError{
					Format: p.ID, Key: name, Value: raw, Reason: bad,
				}
			}
		}
	}
	return nil
}

// Global is what one of the flags in Reads looks like as a setting somebody
// fills in, or false for a name this build knows nothing about.
//
// It exists because Reads is a list of NAMES and a surface has to draw a
// control. The command line does not need this - the flag already exists there,
// with its own wording - and a window has nothing to go on, which is how the
// preset screen came to offer size-boundaries in one format when the command
// line offered it in all thirteen. Measured on 2026-08-12: "tfg generate
// --preset size-boundaries --format png" produces seven PNGs and the window
// could only ever produce PDFs.
//
// The choices are read here rather than declared, because which formats exist
// is a property of the build and registration order between two packages is not
// something to rely on.
//
// The default is the one the presets in this package use. That is true today
// with one preset and it is a coupling rather than a design: a second preset
// reading --format with a different default has to turn this into something the
// preset declares, and the constant it points at is the one place to notice.
func Global(name string) (format.Property, bool) {
	switch name {
	case "format":
		return format.Property{
			Name:    "format",
			Kind:    format.PropertyChoice,
			Choices: format.IDs(),
			Default: defaultFormat,
			Detail:  "What kind of file the whole set is made of.",
		}, true
	}
	return format.Property{}, false
}

// Globals is every flag this preset reads, as settings to put on a screen.
//
// Anything this build cannot describe is left out rather than drawn as an empty
// box. A name here that Global knows nothing about is a preset asking for a
// flag nobody has declared, and a control with no description is worse than the
// missing control - it looks like it works.
func (p Preset) Globals() []format.Property {
	out := make([]format.Property, 0, len(p.Reads))
	for _, name := range p.Reads {
		if declared, ok := Global(name); ok {
			out = append(out, declared)
		}
	}
	return out
}

// ParameterNames is the declared names, in the order the preset listed them.
func (p Preset) ParameterNames() []string {
	out := make([]string, 0, len(p.Parameters))
	for _, param := range p.Parameters {
		out = append(out, param.Name)
	}
	return out
}

// Settle merges what the caller gave over the declared defaults, after
// checking both. The result has a value for every parameter that has one.
func (p Preset) Settle(args Args) (Args, error) {
	if err := p.Check(args); err != nil {
		return nil, err
	}
	settled := p.Defaults()
	for name, value := range args {
		if value != "" {
			settled[name] = value
		}
	}
	return settled, nil
}

// UnknownParameterError is a parameter name no preset declares.
type UnknownParameterError struct {
	Preset string
	Name   string
	Known  []string
}

func (e *UnknownParameterError) Error() string {
	if len(e.Known) == 0 {
		return fmt.Sprintf("the preset %s takes no parameters, so %q is not one of them", e.Preset, e.Name)
	}
	return fmt.Sprintf("the preset %s does not have a parameter called %q. It takes: %s",
		e.Preset, e.Name, strings.Join(e.Known, ", "))
}

// UnknownPresetError is a request for a preset nobody registered.
type UnknownPresetError struct {
	ID    string
	Known []string
}

func (e *UnknownPresetError) Error() string {
	if len(e.Known) == 0 {
		return fmt.Sprintf("unknown preset %q, and this build registers none", e.ID)
	}
	return fmt.Sprintf("unknown preset %q. This build has: %s", e.ID, strings.Join(e.Known, ", "))
}

// ImpossibleError is parameters that describe a set this build cannot produce.
//
// PR7 asks a preset to say so rather than generating the part it can. A set
// missing three of its seven files still looks like a set, and the three that
// are missing are the ones the run was about.
type ImpossibleError struct {
	Preset string
	Detail string
	Hint   string

	// Setting is which parameter of the preset the refusal is about, where it
	// is about one. Named the way the parameter is declared, which is what
	// both surfaces call it - the flag without its dashes and the label on the
	// field. See AboutSetting.
	Setting string
}

func (e *ImpossibleError) Error() string {
	return e.InTheWordsOf(e.Setting)
}

// InTheWordsOf is this refusal with the parameter named the way one surface
// names it - see core.SettingSlot. Error is this with the declared name, which
// is what the command line has always printed.
func (e *ImpossibleError) InTheWordsOf(name string) string {
	if name == "" {
		name = e.Setting
	}
	return core.InTheWordsOf(
		fmt.Sprintf("the preset %s cannot build this set - %s. %s", e.Preset, e.Detail, e.Hint),
		name)
}

// AboutSetting lets a window put this message beside the box that caused it,
// without the window knowing this type. UX8 asks for a refusal near where the
// error came from - O73, where it sat at the foot of the form under every
// other field. Empty means the refusal is about the set rather than one value.
func (e *ImpossibleError) AboutSetting() string { return e.Setting }

// registry holds what this build knows, written at init and read after.
//
// No lock, and that is deliberate rather than forgotten. The format registry
// beside this one carries the single lock in the tree, and a guard names the
// two files allowed to be concurrent so that widening that surface has to be a
// decision. Here there is nothing to decide about: registration happens in
// init, every init finishes before anything reads, and no goroutine goes near
// it. A lock would have been habit rather than need.
//
// If a preset ever has to be registered while the tool is running, the lock
// comes back and this file goes through that gate.
var registry = map[string]Preset{}

// Register adds a preset. It panics on a mistake that a build should not
// survive, the same way the format registry does.
func Register(p Preset) {
	switch {
	case p.ID == "":
		panic("preset: a preset with no id")
	case p.Expand == nil:
		panic(fmt.Sprintf("preset: %s cannot expand into anything", p.ID))
	}
	if _, taken := registry[p.ID]; taken {
		panic(fmt.Sprintf("preset: %s is registered twice", p.ID))
	}
	// A parameter IS a format.Property, so a closed set of values is put in the
	// same order here as it is over there. One rule for both, in the place each
	// declaration passes through exactly once.
	for i := range p.Parameters {
		format.SortChoices(p.Parameters[i].Choices)
	}
	registry[p.ID] = p
}

// Get returns one preset by id.
func Get(id string) (Preset, error) {
	p, ok := registry[id]
	if !ok {
		return Preset{}, &UnknownPresetError{ID: id, Known: ids()}
	}
	return p, nil
}

// All returns every registered preset, by id.
func All() []Preset {
	out := make([]Preset, 0, len(registry))
	for _, id := range ids() {
		out = append(out, registry[id])
	}
	return out
}

// IDs is every registered id, sorted.
func IDs() []string {
	return ids()
}

// Declaring is the preset that declares a parameter of this name, or empty.
//
// Parameter names and global flag names share one namespace, which is what
// lets "--preset size-boundaries --limit 10mb" read as one sentence. The cost
// of that is a flag which exists in one invocation and not in the next, so
// typing --limit without its preset has to be answered with something better
// than "not defined". This is how the command line finds out whose it is.
func Declaring(name string) string {
	for _, id := range ids() {
		for _, param := range registry[id].Parameters {
			if param.Name == name {
				return id
			}
		}
	}
	return ""
}

func ids() []string {
	out := make([]string, 0, len(registry))
	for id := range registry {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
