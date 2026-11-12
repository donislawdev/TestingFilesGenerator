package engine

import (
	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// The notes a manifest entry carries, each sentence remembered once for the
// window to say in its own language.

// note is one note for the manifest, remembered as a sentence once for each
// different thing it says.
func (res *Result) note(code string, said core.Said) manifest.Note {
	detail := said.String()
	if res.NotesSaid == nil {
		res.NotesSaid = map[string]core.Said{}
	}
	if _, have := res.NotesSaid[detail]; !have {
		res.NotesSaid[detail] = said
	}
	return manifest.Note{Code: code, Detail: detail}
}
