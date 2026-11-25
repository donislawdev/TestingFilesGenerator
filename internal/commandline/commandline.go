// Package commandline writes the command that does what a screen of the window
// would do, for a person to paste into a terminal.
//
// # Why this exists
//
// The window and the command line run the same engine, and a person who has
// found the settings they want on a screen should be able to take them to a
// script or a pipeline without retyping them, which is the copy button on
// every screen of the window. The window decides what the command says and this
// package decides how it is written, because writing is the part that differs
// between systems and is easy to get wrong in a way that runs something else.
//
// # What this package is NOT
//
// It runs nothing, and it never finds anything by starting a process. It writes
// a line for a person to read and to paste. A window that ran the command line
// would be a second way to do everything, and the two would answer differently
// the first time one of them changed. Finding where tfg is reads PATH and asks
// the file system, the way the shell itself will - see program.go.
//
// It does not judge the values either. Whether a size is too small for a format
// is a question for the engine, and the command answers it when it runs, in the
// same words the window uses.
package commandline

import "strings"

// Line is a command being written out.
//
// The order things are added is the order they appear, because a person reads
// this to understand what the window would do, and a line whose flags arrive in
// a different order on every press is one they have to read again each time.
// Go randomises the order of a map by decision, so nothing here is built from one.
type Line struct {
	verb  []string
	words []word
}

// word is one flag, one flag with its value, or one argument.
type word struct {
	flag    string
	value   string
	valued  bool
	setting string
}

// New starts a line for a verb, such as "generate" or "tool checksum".
func New(verb ...string) *Line { return &Line{verb: verb} }

// Flag adds a flag with a value, and adds NOTHING when the value is empty.
//
// Empty means the person left the setting alone, and a flag carrying an empty
// value is not the same request: --seed with nothing after it is an error,
// while no --seed at all is what the window would do. setting is the key of
// the box the value came from, so that a value the line cannot carry is
// reported under that box.
func (l *Line) Flag(name, value, setting string) *Line {
	if value == "" {
		return l
	}
	l.words = append(l.words, word{flag: name, value: value, valued: true, setting: setting})
	return l
}

// Switch adds a flag that carries no value, when it is on.
func (l *Line) Switch(name string, on bool) *Line {
	if on {
		l.words = append(l.words, word{flag: name})
	}
	return l
}

// Argument adds what the verb acts on, such as the file a tool reads. An empty
// one is left out, for the reason Flag gives.
func (l *Line) Argument(value, setting string) *Line {
	if value == "" {
		return l
	}
	l.words = append(l.words, word{value: value, valued: true, setting: setting})
	return l
}

// Written is a line as text, and what somebody pasting it should know.
type Written struct {
	// Text is the line itself, for the clipboard.
	Text string
	// Program is where the line found tfg on this computer.
	Program Where
	// OlderPowerShell lists the settings whose values Windows PowerShell 5.1,
	// and PowerShell 7 before 7.3, hand to a program differently from
	// PowerShell 7.3 and later, in the order they appear - see
	// olderPowerShellDiffers. Empty for sh.
	OlderPowerShell []string
}

// Write is the line for one shell, started the way this computer starts tfg.
//
// It fails only on a value no line can carry, and the error is an *Unwritable
// naming the setting, so the window can mark the box it came from.
func (l *Line) Write(p Program, s Shell) (Written, error) {
	parts := append([]string{p.word(s)}, l.verb...)
	out := Written{Program: p.Where}
	for _, w := range l.words {
		text, err := w.write(s)
		if err != nil {
			return Written{}, err
		}
		parts = append(parts, text)
		if s == PowerShell && w.valued && olderPowerShellDiffers(w.value) {
			out.OlderPowerShell = append(out.OlderPowerShell, w.setting)
		}
	}
	out.Text = strings.Join(parts, " ")
	return out, nil
}

// write is one word as the shell should read it.
//
// A value that starts with a dash is joined to its flag as one quoted word,
// --name=-a.txt, and this is measured rather than tidy (docs, the analysis of
// 2026-11-25, section 2). Both versions of PowerShell split a bare -a.txt into
// two arguments, and read --% as the end of everything they parse, quoted or
// not - after which the rest of the line goes to the program as it stands. The
// joined form is the one that came through whole in both, and the flag package
// takes it. The same form in sh keeps the line one shape on every system.
func (w word) write(s Shell) (string, error) {
	if !w.valued {
		return w.flag, nil
	}
	if err := writable(w.value, w.setting); err != nil {
		return "", err
	}
	switch {
	case strings.HasPrefix(w.value, "-") && w.flag == "":
		// An argument has no flag to be joined to. The window hands over
		// absolute paths, which never start with a dash, so this is a
		// refusal rather than a line that would be read as a flag.
		return "", &Unwritable{Setting: w.setting, Reason: LeadingDash}
	case strings.HasPrefix(w.value, "-"):
		return quote(w.flag+"="+w.value, s), nil
	case w.flag == "":
		return quote(w.value, s), nil
	}
	return w.flag + " " + quote(w.value, s), nil
}
