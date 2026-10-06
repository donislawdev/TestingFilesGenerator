// This file holds a run's reservation of its manifest's name - taken before
// the first file, written through at the end. Its own file since 2026-09-29,
// when the reservation moved from an empty file under the manifest's name to
// the temporary name the manifest is written under (O252), and manifest.go went
// past the size a person can follow.

package manifest

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// Reservation is a run's hold on the name its manifest will take.
//
// Taken before the first file, not after the last one. Claiming at save time
// already stopped two runs from both writing a manifest, but it happened at the
// end - so a second run wrote its whole set of files and only then found out it
// had nowhere to record them. Measured on 2026-08-03: two runs started together
// under different ids ended 0 and 5, with sixteen files on the disk and eight
// of them in nobody's manifest (O43).
//
// Until 2026-09-29 the hold was an empty file under the FINAL name, and the save
// renamed over it. A rename replaces whatever it lands on, so a file somebody
// else put there between the claim and the save was destroyed without a word -
// one of the three windows of O252, measured that day with a second writer
// spinning on the name. The hold is now the temporary name the manifest is
// written under, created exclusively when the run starts and written through
// at the save. A second run still cannot take it and is refused before its first
// file, and the final name is never written over.
//
// What lies on the disk while a run goes is therefore "<manifest>.tfg-writing"
// and no manifest at all. A run killed outright leaves that name behind rather
// than an empty manifest, and that is better as well as safer: the empty
// manifest made the next run say "the only record of what an earlier run
// wrote" about a file that recorded nothing, and sent somebody looking for a
// run whose files it could not name. The leftover name makes the next run say
// that a run is going or was killed, and name the file to remove.
type Reservation struct {
	final string
	tmp   string
	// own is what the reservation file is. It is closed as soon as it is
	// made - a file held open from the start of a run to its save keeps a
	// Windows directory from being removed by anybody who ran the engine and
	// never saved, measured on 2026-09-29 when a guard's own clean-up failed
	// on it. So the save opens it again and asks the OPENED file whether it
	// is still this one (core.OpenOwn), which is what stops a name swapped for
	// a link in the hours a large run takes. A clean-up removes it only while
	// it is still this one too.
	own os.FileInfo
	// used says the reservation was saved through or given back.
	used bool
}

// ReservationPath is the name a run holds while it goes, beside the manifest.
func ReservationPath(path string) string {
	return core.SiblingPath(path, core.WritingMarker)
}

// Claim reserves the name a manifest will take, before a run writes anything.
//
// A manifest already under that name is refused here rather than at the save,
// in the words that always named it, so a run does not write its files first.
// A reservation somebody already holds comes back as core.NameTakenError about
// the reservation's own name, which is how the engine tells a run in progress
// from a finished one.
func Claim(path string) (*Reservation, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	// "Nothing is there" and "I could not look" are two answers. Reading every
	// failure of the look as an empty slot sent a path nobody could examine on
	// to words about a manifest that already existed - a sentence about the
	// wrong thing, and the one somebody would act on (review 2026-08-23, 3.7c).
	switch _, err := os.Lstat(path); {
	case err == nil:
		return nil, &os.PathError{Op: "save", Path: path, Err: fs.ErrExist}
	case !errors.Is(err, fs.ErrNotExist):
		return nil, err
	}
	tmp := ReservationPath(path)
	// Created exclusively, and core.CreateNew says why: this name sits in a
	// directory the run does not own, and a create that is not exclusive
	// follows whatever is at the name. Measured on 2026-09-06 - a link here
	// put the manifest on a file outside the output directory and the run
	// still exited 0.
	f, err := core.CreateNew(tmp, 0o666)
	if err != nil {
		return nil, err
	}
	own, err := core.Finish(f, nil)
	if err != nil {
		_ = core.RemoveOwn(tmp, own)
		return nil, err
	}
	return &Reservation{final: path, tmp: tmp, own: own}, nil
}

// Save writes the manifest through the reservation and gives it its name.
//
// Whatever fails, the reservation goes with it - but only while its name still
// holds the file this run created. A save can be asked once.
func (r *Reservation) Save(m *Manifest) error {
	if r.used {
		return core.Defect(fmt.Errorf("the reservation of %s was already used or given back", core.Shown(r.final)))
	}
	r.used = true
	f, err := core.OpenOwn(r.tmp, r.own)
	if err != nil {
		// A reservation that could not be opened is still this run's, unless
		// the name holds something else now - and RemoveOwn asks exactly that.
		// Left behind, it would tell the next run a run is going.
		_ = core.RemoveOwn(r.tmp, r.own)
		return err
	}
	own, err := saveReserved(f, m, r.tmp, r.final)
	if own != nil {
		r.own = own
	}
	if err != nil {
		_ = core.RemoveOwn(r.tmp, r.own)
		return err
	}
	return nil
}

// Release gives the name back, for a run that reserved it and then had nothing
// to record. Without it a refused run would leave its reservation behind and the
// next run into that directory would be told a run is going.
func (r *Reservation) Release() error {
	if r == nil || r.used {
		return nil
	}
	r.used = true
	return core.RemoveOwn(r.tmp, r.own)
}

// saveReserved fills the reservation, flushes it, closes it and gives it the
// manifest's name, and says what the file is - asked after the write, because a
// clean-up compares the size and the time as well as the file
// (core.RemoveOwn).
//
// Written as one straight sequence on purpose. The guard that asks whether the
// bytes reach the disk before the name does reads the steps of this function
// in order, and a step hidden in the body of an if is a step it cannot see.
//
// The mode is set on the opened file, before anything is in it, because the
// publish moves the file and its mode with it. So this is where a manifest
// carrying a password stops being readable by every account on the machine.
// The ordinary mode is the one the create left, umask and all.
//
// On the device before the publish, because the publish is what turns this
// into the manifest and a rename can reach the disk before the bytes do. What
// survives that is an empty file under the name of the only record able to
// remove a run's files - the loss this whole function is shaped against,
// reached by pulling the plug rather than by killing the process. One call per
// run, so the cost argument that keeps generated files unsynced does not reach
// here. That one is written on engine.Run and it is about ten thousand
// flushes, not one. docs/CODE-REVIEW-2026-08-23.md section 3.4, owner's call on
// 2026-08-25.
func saveReserved(f *os.File, m *Manifest, tmp, final string) (os.FileInfo, error) {
	if mode := m.mode(); mode != 0o666 {
		if err := f.Chmod(mode); err != nil {
			return core.Finish(f, err)
		}
	}
	if err := m.Encode(f); err != nil {
		return core.Finish(f, err)
	}
	if err := f.Sync(); err != nil {
		return core.Finish(f, err)
	}
	own, err := core.Finish(f, nil)
	if err != nil {
		return own, err
	}
	return own, core.Publish(tmp, final)
}
