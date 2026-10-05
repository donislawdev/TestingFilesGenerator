package engine

import (
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

func (e *InstructionsError) Error() string { return e.Said().String() }

// Said is the refusal, for a window that says it in its own language.
func (e *InstructionsError) Said() core.Said {
	return core.Says("engine.InstructionsNotWritten", "cannot write the instructions to %s: %v",
		core.A("Path", core.Shown(e.Path)), core.A("Cause", e.Err))
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
	var instructions os.FileInfo
	if text := m.Instructions(filepath.Base(rec.Manifest)); text != nil {
		path := InstructionsPath(opt)
		own, err := manifest.SaveInstructions(path, text)
		if err != nil {
			rec.Missed = &InstructionsError{Path: path, Err: err}
		} else {
			instructions = own
			rec.Instructions = path
			m.Run.Instructions = filepath.Base(path)
		}
	}
	if err := saveManifest(res, m, rec.Manifest); err != nil {
		if rec.Instructions != "" {
			// The file this call wrote, and nothing that took its name since.
			_ = core.RemoveOwn(rec.Instructions, instructions)
			rec.Instructions, m.Run.Instructions = "", ""
		}
		return rec, err
	}
	return rec, nil
}

// saveManifest saves through the reservation the run took before its first
// file, or reserves and saves in one go for a result that has none.
func saveManifest(res *Result, m *manifest.Manifest, path string) error {
	if r := res.reservation; r != nil {
		res.reservation = nil
		return r.Save(m)
	}
	return m.Save(path)
}
