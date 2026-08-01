package engine

import "fmt"

// Errors carry what went wrong in a form the surface above can turn into an
// exit code, without the engine knowing anything about exit codes.
//
// The alternative is a switch on message text at the top of the program,
// where every failure mode nobody remembered to list quietly becomes a
// generic error - and CI, which can only tell situations apart by the code,
// stops being able to react.

// RecipeError is a request that is well formed but asks for something that
// makes no sense.
type RecipeError struct {
	Detail string
}

func (e *RecipeError) Error() string { return e.Detail }

// SpaceError is refusing to start because the disk cannot hold the result.
//
// Free space is checked before the first byte rather than discovered at file
// five thousand of ten thousand. Filling up a working disk is the most
// expensive thing this tool can do to someone.
type SpaceError struct {
	Needed    int64
	Available int64
	Path      string
}

func (e *SpaceError) Error() string {
	return fmt.Sprintf(
		"this run needs %d B and %s has %d B free - nothing was written. Ask for fewer files or a smaller size, or write to another disk with --out",
		e.Needed, e.Path, e.Available)
}

// CollisionError is refusing to write over something that is already there.
//
// This tool runs in directories that belong to the user. Overwriting without
// a word is the one mistake that destroys work rather than wasting time.
type CollisionError struct {
	Path string
}

func (e *CollisionError) Error() string {
	return fmt.Sprintf(
		"%s already exists and this run will not write over it. Generate into an empty directory with --out, or remove the file first",
		e.Path)
}
