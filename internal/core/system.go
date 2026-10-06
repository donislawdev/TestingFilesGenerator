package core

import (
	"errors"
	"io/fs"
	"syscall"
)

// SystemKind is what the system refused, by what the error means rather than
// by its words.
//
// The words are the system's, in the system's language - Windows answers in
// English when it can and in the language of the machine when it cannot - so
// both surfaces put a sentence of their own in their place, with the number
// beside it, which is the part that means the same everywhere. The command
// line has done that since 2026-09-25. The kinds are here so the window and
// the command line cannot come to tell them apart differently (O251).
type SystemKind int

// The kinds a refusal of the system is told apart by.
const (
	SystemRefusedIt SystemKind = iota
	SystemNothingThere
	SystemNoPermission
	SystemAlreadyThere
)

// SystemKindOf is the kind of one error of the system.
func SystemKindOf(errno syscall.Errno) SystemKind {
	switch {
	case errors.Is(errno, fs.ErrNotExist):
		return SystemNothingThere
	case errors.Is(errno, fs.ErrPermission):
		return SystemNoPermission
	case errors.Is(errno, fs.ErrExist):
		return SystemAlreadyThere
	}
	return SystemRefusedIt
}
