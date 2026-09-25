package engine

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// Record is what a run saved beside its files to say what they are.
type Record struct {
	// Manifest is where the manifest was saved.
	Manifest string
	// Instructions is where the instructions were saved, and empty when the
	// run wrote none: no file was given a purpose, or writing them failed.
	Instructions string
	// Missed is set when instructions were due and are not there. The run is
	// not failed by it - the manifest was saved and holds the same facts - and
	// the caller says it out loud rather than letting it pass.
	Missed *InstructionsError
}

// InstructionsError is instructions that were due and could not be written.
type InstructionsError struct {
	Path string
	Err  error
}

func (e *InstructionsError) Error() string {
	return fmt.Sprintf("cannot write the instructions to %s: %v", core.Shown(e.Path), e.Err)
}

func (e *InstructionsError) Unwrap() error { return e.Err }

// SaveRecord writes the instructions, when any file of the run was given a
// purpose, and then the manifest, which names them.
//
// One function for the command line and the window, and that is the reason it
// exists. Each surface saved the manifest its own way, and a second file
// written twice would be two chances for the two surfaces to leave different
// directories behind (D1).
//
// The instructions first, so that the manifest only names a file that is on
// the disk. A manifest that cannot be saved takes the instructions with it:
// they describe files nothing records, and cleanup would have no way to them.
func SaveRecord(res *Result, opt Options) (Record, error) {
	rec := Record{Manifest: ManifestPath(opt)}
	m := res.Manifest
	if text := m.Instructions(filepath.Base(rec.Manifest)); text != nil {
		path := InstructionsPath(opt)
		if err := manifest.SaveInstructions(path, text); err != nil {
			rec.Missed = &InstructionsError{Path: path, Err: err}
		} else {
			rec.Instructions = path
			m.Run.Instructions = filepath.Base(path)
		}
	}
	if err := m.Save(rec.Manifest); err != nil {
		if rec.Instructions != "" {
			_ = os.Remove(rec.Instructions)
			rec.Instructions, m.Run.Instructions = "", ""
		}
		return rec, err
	}
	return rec, nil
}
