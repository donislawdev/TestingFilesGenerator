package tool

import (
	"fmt"
	"strings"
)

// Sentence is a refusal of a tool as one line of the command line: what
// happened, why, and what to do instead - the shape "PNG cannot be smaller
// than 74 B - ... Ask for 74 B or more" already has. One function, so every
// tool refuses in the same shape without each writing it.
func Sentence(what, why, instead string) string {
	return what + " - " + why + ". " + strings.ToUpper(instead[:1]) + instead[1:]
}

// Class is what kind of mistake a refusal is, which is what the command line
// turns into an exit code (AR6: the code is a property of the error). Declared
// by the error rather than listed in the command line, so a tool added
// tomorrow refuses with the right code without anybody editing a list.
type Class int

// The two kinds a tool refuses with.
const (
	// Asked is a request that cannot be run as written - a wrong setting, a
	// missing file name, a directory where a file goes.
	Asked Class = iota + 1
	// Reading is a request that was fine and a disk that did not cooperate.
	Reading
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
func (e *UnknownError) What() string {
	return fmt.Sprintf("there is no tool called %q", e.ID)
}

// Why this is refused rather than guessed at.
func (e *UnknownError) Why() string {
	return "a tool is named exactly, and the nearest name to a mistyped one may do something else"
}

// Instead names what there is, from the registry rather than from a list.
func (e *UnknownError) Instead() string {
	if len(e.Known) == 0 {
		return "this build has no tools"
	}
	return "use one of: " + strings.Join(e.Known, ", ")
}

func (e *UnknownError) Error() string { return Sentence(e.What(), e.Why(), e.Instead()) }

// Class says this is a mistake in the request.
func (e *UnknownError) Class() Class { return Asked }

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
func (e *MissingInputError) What() string {
	return fmt.Sprintf("no %s was given", e.Input.Kind)
}

// Why this is refused rather than started.
func (e *MissingInputError) Why() string {
	return "the tool has nothing to read without one"
}

// Instead is what to give it.
func (e *MissingInputError) Instead() string {
	return fmt.Sprintf("name the %s to work on", e.Input.Kind)
}

func (e *MissingInputError) Error() string { return Sentence(e.What(), e.Why(), e.Instead()) }

// Class says this is a mistake in the request.
func (e *MissingInputError) Class() Class { return Asked }
