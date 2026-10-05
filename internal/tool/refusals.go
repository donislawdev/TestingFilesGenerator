package tool

import (
	"errors"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// Said is a refusal of a tool in one sentence: what happened, why, and what to
// do instead - the shape "PNG cannot be smaller than 74 B - ... Ask for 74 B or
// more" already has. One function, so every tool refuses in the same shape
// without each writing it. What to do starts with a capital in every language,
// because it starts a sentence.
func Said(what, why, instead core.Said) core.Said {
	return core.Says("tool.Sentence", "%s - %s. %s", core.A("What", what), core.A("Why", why), core.A("Instead", instead.Capitalized()))
}

// Class is what kind of mistake a refusal is, which is what the command line
// turns into an exit code (AR6: the code is a property of the error). Declared
// by the error rather than listed in the command line, so a tool added
// tomorrow refuses with the right code without anybody editing a list.
type Class int

// The kinds a tool refuses with.
const (
	// Asked is a request that cannot be run as written - a wrong setting, a
	// missing file name, a directory where a file goes.
	Asked Class = iota + 1
	// Reading is a request that was fine and a disk that did not cooperate.
	Reading
	// Room is a file a tool would write and a disk without the space for it -
	// its own class because the frozen table gives it its own code, and CI
	// has to tell "give it a bigger disk" from "fix the permissions".
	Room
)

// Classified is a refusal that says what kind it is.
type Classified interface {
	error
	Class() Class
}

// UnknownError is a tool this build does not have.
type UnknownError struct {
	ID    string
	Known []string
}

// What happened, without the list of names.
func (e *UnknownError) what() core.Said {
	return core.Says("tool.ThereIsNoToolCalled", "there is no tool called %q", core.A("ID", e.ID))
}

// Why this is refused rather than guessed at.
func (e *UnknownError) why() core.Said {
	return core.Says("tool.AToolIsNamedExactlyAnd", "a tool is named exactly, and the nearest name to a mistyped one may do something else")
}

// Instead names what there is, from the registry rather than from a list.
func (e *UnknownError) instead() core.Said {
	if len(e.Known) == 0 {
		return core.Says("tool.ThisBuildHasNoTools", "this build has no tools")
	}
	return core.Says("tool.UseOneOf", "use one of: %s", core.A("Known", strings.Join(e.Known, ", ")))
}

func (e *UnknownError) Error() string { return e.Said().String() }

// Said is the whole refusal, for a window that says it in its own language.
func (e *UnknownError) Said() core.Said { return Said(e.what(), e.why(), e.instead()) }

// Class says this is a mistake in the request.
func (e *UnknownError) Class() Class { return Asked }

// SettingError is a setting of a tool its declaration refuses: the refusal the
// registry words for every declared setting, classed as a mistake in the
// request.
//
// Wrapped because on its own that refusal ends a run with FORMAT, the code for
// a file a format cannot make, and a tool makes no file of any format. A wrong
// algorithm chosen for checksum-write ended with 4 until 2026-09-30 - the day
// the first tool declared a closed set of values, which is when
// docs/NARZEDZIA-SUMY-2026-09-29.md §13.3 said this would become reachable.
type SettingError struct {
	Err error
}

func (e *SettingError) Error() string { return e.Err.Error() }

// Unwrap is the registry's own refusal, for a caller asking what it was.
func (e *SettingError) Unwrap() error { return e.Err }

// AboutSetting is the setting refused, so a form marks its box.
func (e *SettingError) AboutSetting() string {
	var about interface{ AboutSetting() string }
	if errors.As(e.Err, &about) {
		return about.AboutSetting()
	}
	return ""
}

// Class says this is a mistake in the request.
func (e *SettingError) Class() Class { return Asked }

// MissingInputError is a request without something the tool works on.
type MissingInputError struct {
	Tool  string
	Input Input
}

// AboutSetting is the input this is about, so a form puts the message under
// the box it belongs to - the same question the refusals of a setting answer.
func (e *MissingInputError) AboutSetting() string { return e.Input.Name }

// What happened. Worded for both surfaces - a window shows this under the
// box and the command line prints it - so it names neither a flag nor a
// button.
func (e *MissingInputError) what() core.Said {
	return core.Says("tool.NoWasGiven", "no %s was given", core.A("Kind", e.Input.Kind))
}

// Why this is refused rather than started.
func (e *MissingInputError) why() core.Said {
	return core.Says("tool.TheToolHasNothingToRead", "the tool has nothing to read without one")
}

// Instead is what to give it.
func (e *MissingInputError) instead() core.Said {
	return core.Says("tool.NameTheToWorkOn", "name the %s to work on", core.A("Kind", e.Input.Kind))
}

func (e *MissingInputError) Error() string { return e.Said().String() }

// Said is the whole refusal, for a window that says it in its own language.
func (e *MissingInputError) Said() core.Said { return Said(e.what(), e.why(), e.instead()) }

// Class says this is a mistake in the request.
func (e *MissingInputError) Class() Class { return Asked }

// What, Why and Instead are the parts in English, for a report that lays
// them out apart.
func (e *MissingInputError) What() string    { return e.what().String() }
func (e *MissingInputError) Why() string     { return e.why().String() }
func (e *MissingInputError) Instead() string { return e.instead().String() }

// What, Why and Instead are the parts in English, for a report that lays
// them out apart.
func (e *UnknownError) What() string    { return e.what().String() }
func (e *UnknownError) Why() string     { return e.why().String() }
func (e *UnknownError) Instead() string { return e.instead().String() }
