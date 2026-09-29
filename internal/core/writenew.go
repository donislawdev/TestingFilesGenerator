package core

import (
	"errors"
	"os"
)

// WriteNew writes a whole file under a name nobody holds, and hands back what
// the file is so the caller can later tell it from anything else put there.
//
// Written under a temporary name beside the final one, flushed to the device,
// and only then given its name by Publish - so a run cut short leaves at most a
// temporary name behind, never half a file under a real one, and a name
// somebody else took in the meantime is refused rather than written over.
//
// The flush is here because every caller is a file somebody keeps: the
// instructions beside a manifest and a recipe written by "preset eject -o".
// One of each per command, so the cost argument that keeps generated files
// unflushed does not reach them.
//
// The mode goes to the create, so the process umask applies to it the way it
// applies to every other file this tool writes.
func WriteNew(path string, content []byte, mode os.FileMode) (os.FileInfo, error) {
	tmp := SiblingPath(path, WritingMarker)
	f, err := CreateNew(tmp, mode)
	if err != nil {
		return nil, err
	}
	own, err := writeAndKeep(f, content)
	if err != nil {
		_ = RemoveOwn(tmp, own)
		return nil, err
	}
	if err := Publish(tmp, path); err != nil {
		_ = RemoveOwn(tmp, own)
		return nil, err
	}
	return own, nil
}

// writeAndKeep fills a file this call created, flushes it and closes it, and
// says what it is. The identity comes from the open handle rather than from the
// name, because the name is exactly what somebody else could have swapped - and
// it is asked after the write, because RemoveOwn compares the size and the time
// too, and those are what the write changes.
func writeAndKeep(f *os.File, content []byte) (os.FileInfo, error) {
	if _, err := f.Write(content); err != nil {
		return Finish(f, err)
	}
	if err := f.Sync(); err != nil {
		return Finish(f, err)
	}
	return Finish(f, nil)
}

// Finish asks an open file what it is, closes it, and reports the first
// failure - the one given, the question, or the close.
func Finish(f *os.File, err error) (os.FileInfo, error) {
	own, statErr := f.Stat()
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = statErr
	}
	return own, err
}

// RemoveOwn removes a name only while it still holds the file this tool made
// there.
//
// Every clean-up after a failure used to remove by name, and a name is not a
// file: between our write and our clean-up somebody could put their own file
// under it, and the clean-up then deleted theirs (O252, the third window).
// Asked by identity instead - the same file, however it got there. What stays
// open is the moment between this look and the removal, which is a different
// order of narrow from a whole write.
//
// Identity is the file id and more. On Linux and macOS os.SameFile compares the
// device and the inode number and nothing else, and a filesystem hands a freed
// inode number to the next file created. Measured on 2026-09-29: "ours
// removed, theirs created" got our inode number back 500 times in 500 on ext4
// and on overlay (0 on tmpfs and NTFS), so os.SameFile alone would have called
// their file ours every time. With the size and the time of the last write
// asked as well it was fooled 0 times in 500 on all four, empty files
// included. The identity taken from the open file before the close agreed with
// the one asked by name afterwards 500 times in 500 on all four.
//
// A name that holds something else is left alone and reported, because
// untouchable rule 7 is that this tool removes only what it wrote. A name
// already gone is not an error: there is nothing of ours left to remove.
func RemoveOwn(path string, own os.FileInfo) error {
	now, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !sameWrite(own, now) {
		return &NotOursError{Path: path}
	}
	return os.Remove(path)
}

// sameWrite is the identity RemoveOwn asks for: one file, as it was left.
func sameWrite(own, now os.FileInfo) bool {
	return own != nil && os.SameFile(own, now) &&
		own.Size() == now.Size() && own.ModTime().Equal(now.ModTime())
}

// NotOursError is a clean-up that found somebody else's file under a name it
// meant to remove, and left it.
type NotOursError struct {
	Path string
}

func (e *NotOursError) Error() string {
	return "the name " + e.Path + " no longer holds the file this tool wrote there, so it was left as it is. " +
		"Something else put a file under that name while this tool was working. " +
		"Look at it before removing it yourself"
}
