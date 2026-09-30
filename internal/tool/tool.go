// Package tool is the registry of the tools on the Tools tab and behind
// "tfg tool": small things a person does with files they already have, beside
// the generator rather than inside it.
//
// A tool declares what it works on and what it takes, and both surfaces are
// drawn from that declaration - the command line registers a flag per setting
// and the window a field per setting, through the same code that already draws
// the settings of a format, a preset and a damage. So a tool added tomorrow
// reaches both without a line of surface code, which is D1 kept by
// construction rather than by somebody remembering. Several dozen of these are
// planned (docs/NARZEDZIA-SUMY-2026-09-29.md), which is what makes the
// declaration worth having from the first one.
//
// This package sits above the engine and the audit on purpose. Tools are
// facades - a set of samples is a run of the engine, a test of a memory stick
// is generate, verify and cleanup - so they have to be able to reach both.
//
// The work of a tool is done without a window and without a console. It gets
// its values checked and defaulted, a context to stop on and a place to report
// progress, and it hands back data. Wording the answer is the surface's job.
package tool

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// InputKind is what an input names on the disk.
type InputKind string

// The kinds of input.
const (
	// File is one existing file, read and never written.
	File InputKind = "file"
	// Folder is one existing folder. Every file under it is read, and a tool
	// that writes adds one new file to it - never over one that is there.
	Folder InputKind = "folder"
)

// Input is what a tool works on, as opposed to how it works - a path, given on
// the command line without a flag in front of it and chosen in the window with
// a picker.
type Input struct {
	// Name is the key of the input, in a request and in the words of every
	// language. Not a flag: inputs are positional.
	Name string
	Kind InputKind
	// Detail is the one sentence beside the field.
	Detail string
}

// Outcome is what a comparison came to. A tool that compares nothing leaves it
// Unasked.
type Outcome string

// The three answers. Kept as data rather than a sentence, because the two
// surfaces word it themselves - the window in its own language - and a
// sentence composed here would be English in both.
const (
	Unasked  Outcome = "none"
	Match    Outcome = "match"
	Mismatch Outcome = "mismatch"
)

// Verdict is the comparison a tool made, if it made one.
type Verdict struct {
	Outcome Outcome
	// About is what was compared - the name of an algorithm, for checksums,
	// or the name of the list every file was compared with. Data rather than
	// a word, so it is never translated (G8).
	About string
	// Wanted is what the request said it should be, and Got what it is. Both
	// are data and shown as they are, so a mismatch can be read without the
	// table beside it.
	Wanted string
	Got    string
	// Listed is a comparison of many files with a list that names them - a
	// checksum file - rather than of one value with another. Its answer is
	// said about the list, and what did not match is in the notes.
	Listed bool
}

// Said is the verdict in the words of the command line, which are English
// (D9). Empty when nothing was compared. The window says the same in its own
// language, and a guard holds its English to this sentence.
//
// No count is said, on purpose: "3 of 1 files" is wrong in English and every
// count reads differently in Polish. The counts are notes, each a name and a
// number (Note), which read the same whatever the number is.
func (v Verdict) Said() string {
	switch {
	case v.Outcome == Match && v.Listed:
		return fmt.Sprintf("Matches: every file %s lists is what it says.", v.About)
	case v.Outcome == Mismatch && v.Listed:
		return fmt.Sprintf("Does not match: %s lists files that are not what it says.", v.About)
	case v.Outcome == Match:
		return fmt.Sprintf("Matches: the %s is %s, as expected.", v.About, v.Got)
	case v.Outcome == Mismatch:
		return fmt.Sprintf("Does not match: the %s is %s and %s was expected.", v.About, v.Got, v.Wanted)
	}
	return ""
}

// Note is one thing a tool may say besides its table, declared once: a file
// written, how many files a list held, the names left out and why.
//
// Declared rather than composed in the tool, for the reason the settings are:
// both surfaces word it from the declaration, and a window says it in its own
// language through the registry's catalogue - so a tool that learns to say
// something new is heard in both without a line of surface code. The sentence
// holds no number, since a number is an item of the note and reads the same
// in every language.
type Note struct {
	// ID is the key of the note, in a result and in the words of every
	// language. Never shown: the command line prints Says, and --json carries
	// the same facts under keys of its own.
	ID string
	// Says is the sentence, in English, ending in a colon: the items follow.
	Says string
}

// Noted is one note a run of a tool made, with what it is about.
type Noted struct {
	ID string
	// Items are data - paths, line numbers, a count - shown as they are in
	// every language.
	Items []string
}

// Request is one run of a tool: the paths it works on and the settings it was
// given. Values left out take the declared default before the tool sees them.
type Request struct {
	Inputs map[string]string
	Values map[string]string
}

// Result is what a run of a tool hands back.
type Result struct {
	// Rows are the table a person reads, one entry per declared column.
	// Values in it are data - digests, names, numbers - and are shown as they
	// are in every language.
	Rows [][]string
	// Verdict is the comparison, when the request asked for one.
	Verdict Verdict
	// Notes are what the run has to say besides the table, in the order it
	// says them. Each is one the tool declared.
	Notes []Noted
	// Data is the same answer typed, for --json. A script reading a digest
	// asks for checksums.sha256 rather than for the second cell of a row.
	Data any
}

// Progress is told how much of the work is done, in bytes. The total is what
// the tool expects to read and may be zero when it does not know.
//
// Called from one goroutine at a time, whatever the tool runs beside itself -
// a window's bar is not written for two callers at once, and neither should
// any listener have to be. A tool reading several files at once counts through
// audit.Tally, which keeps that promise.
type Progress func(done, total int64)

// Descriptor is one tool.
type Descriptor struct {
	// ID is the name after "tfg tool". A public name under untouchable rule 10.
	ID string
	// Question is the title of the tool, asked the way a person asks it.
	Question string
	// Detail is one sentence on what the tool does.
	Detail string
	// Inputs are what it works on, in the order they are given.
	Inputs []Input
	// Settings are how it works, as the same declaration a format's settings
	// are - so the window draws them with DeclaredFields and the command line
	// describes them with Allowed.
	Settings []format.Property
	// Columns are the headings of the table in a result. A tool whose answer
	// is a list of files too long for a table - every file of a folder - has
	// none, and says what it found in notes.
	Columns []string
	// Notes are everything the tool may say besides the table.
	Notes []Note
	// Run does the work, on a request that has been checked and defaulted.
	Run func(ctx context.Context, in Request, progress Progress) (Result, error)
}

// reserved are the words after "tfg tool" that are operations rather than
// tools, so no tool can be called one of them.
var reserved = map[string]bool{"list": true, "show": true}

var (
	mu       sync.RWMutex
	registry = map[string]Descriptor{}
)

// Register adds a tool. It panics on a declaration that could not work, the
// way format.Register and damage.Register do: these are faults in the build,
// found by the first test that imports the package.
func Register(d Descriptor) {
	mu.Lock()
	defer mu.Unlock()

	if problem := unusable(d); problem != "" {
		panic(fmt.Sprintf("tool: %q %s", d.ID, problem))
	}
	if _, exists := registry[d.ID]; exists {
		panic(fmt.Sprintf("tool: %q is registered twice", d.ID))
	}
	for i := range d.Settings {
		format.SortChoices(d.Settings[i].Choices)
	}
	registry[d.ID] = d
}

// unusable is what is wrong with a declaration, or nothing.
func unusable(d Descriptor) string {
	switch {
	case d.ID == "":
		return "has no id"
	case reserved[d.ID]:
		return "is an operation of tfg tool and cannot name a tool"
	case d.Run == nil:
		return "has nothing to run"
	case d.Question == "" || d.Detail == "":
		return "has no question or no sentence describing it"
	case len(d.Columns) == 0 && len(d.Notes) == 0:
		return "declares neither columns nor notes, so a result would have nothing to say"
	}
	if problem := unusableNote(d.Notes); problem != "" {
		return problem
	}
	return clashingName(d)
}

// unusableNote is what is wrong with the notes a tool declares, or nothing.
func unusableNote(notes []Note) string {
	seen := map[string]bool{}
	for _, n := range notes {
		switch {
		case n.ID == "" || n.Says == "":
			return "declares a note without a key or without a sentence"
		case seen[n.ID]:
			return "declares the note " + n.ID + " twice"
		}
		seen[n.ID] = true
	}
	return ""
}

// NoteSays is the English of a note this tool declares, or nothing.
func (d Descriptor) NoteSays(id string) string {
	for _, n := range d.Notes {
		if n.ID == id {
			return n.Says
		}
	}
	return ""
}

// clashingName is a name declared twice across inputs and settings, which would
// make a request ambiguous and two flags one.
func clashingName(d Descriptor) string {
	seen := map[string]bool{}
	for _, in := range d.Inputs {
		if seen[in.Name] {
			return "declares " + in.Name + " twice"
		}
		seen[in.Name] = true
	}
	for _, p := range d.Settings {
		if seen[p.Name] {
			return "declares " + p.Name + " twice"
		}
		seen[p.Name] = true
	}
	return ""
}

// Get is one tool by its id.
func Get(id string) (Descriptor, error) {
	mu.RLock()
	defer mu.RUnlock()

	d, ok := registry[id]
	if !ok {
		return Descriptor{}, &UnknownError{ID: id, Known: names()}
	}
	return d, nil
}

// All is every tool, by id.
func All() []Descriptor {
	mu.RLock()
	defer mu.RUnlock()

	out := make([]Descriptor, 0, len(registry))
	for _, id := range names() {
		out = append(out, registry[id])
	}
	return out
}

// Names is the id of every tool, sorted.
func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	return names()
}

func names() []string {
	out := make([]string, 0, len(registry))
	for id := range registry {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// SettingNames is what a tool takes, in the declared order.
func (d Descriptor) SettingNames() []string {
	out := make([]string, 0, len(d.Settings))
	for _, p := range d.Settings {
		out = append(out, p.Name)
	}
	return out
}

// CheckEach is every problem with a request, inputs first, in a stable order -
// all of them rather than the first, so a form can mark every box at once.
func (d Descriptor) CheckEach(in Request) []error {
	var bad []error
	for _, want := range d.Inputs {
		if in.Inputs[want.Name] == "" {
			bad = append(bad, &MissingInputError{Tool: d.ID, Input: want})
		}
	}
	for _, err := range format.CheckStated(d.ID, d.Settings, in.Values) {
		bad = append(bad, &SettingError{Err: err})
	}
	return bad
}

// Start checks a request, fills in the defaults and runs the tool.
//
// The only way in. A surface that called Run itself would hand a tool a value
// nobody checked, and each tool would then have to repeat what the declaration
// already says.
func (d Descriptor) Start(ctx context.Context, in Request, progress Progress) (Result, error) {
	if bad := d.CheckEach(in); len(bad) > 0 {
		return Result{}, bad[0]
	}
	values := make(map[string]string, len(d.Settings))
	for _, p := range d.Settings {
		values[p.Name] = p.Default
		if given := in.Values[p.Name]; given != "" {
			values[p.Name] = given
		}
	}
	if progress == nil {
		progress = func(int64, int64) {}
	}
	result, err := d.Run(ctx, Request{Inputs: in.Inputs, Values: values}, progress)
	if err != nil {
		return result, err
	}
	// A note nobody declared has no words in any language, so a window would
	// show a key. Refused loudly, as a fault in the build, rather than shown.
	for _, n := range result.Notes {
		if d.NoteSays(n.ID) == "" {
			return Result{}, fmt.Errorf("the tool %s said a note it never declared (%s). This is a fault in the build", d.ID, n.ID)
		}
	}
	return result, nil
}
