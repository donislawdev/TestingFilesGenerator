package core

import "os"

// Opening a file that is read rather than written. One way in for every
// reader, so a file named on its own, the thousandth file of a folder, an
// archive somebody was sent and a file a manifest lists are opened with the
// same care - the review of #157 found a pipe swapped in between two looks at a
// name, and the answer to that is here rather than in each reader
// (docs/NARZEDZIA-SUMY-2026-09-29.md §13.4). Moved out of the checksum tool on
// 2026-10-19, when a second tool came to read files, and out of internal/tool
// into core on 2026-10-26, when verify and cleanup came to need it: they sit
// a layer below the tools and could not reach it there
// (docs/STABILITY-REVIEW-2026-10-26.md, R04).

// NotRegularError is a path that names something other than a file. Each
// reader says it in its own words, wrapping this, so a check that every reader
// refuses a pipe asks for this one type.
type NotRegularError struct {
	Path      string
	Directory bool
}

func (e *NotRegularError) Error() string {
	if e.Directory {
		return e.Path + " is a directory"
	}
	return e.Path + " is not a file but a device, a pipe or a socket"
}

// OpenRegular opens a path for reading, and hands it back only when what was
// opened is a file.
//
// Followed through a link, the way a person naming a file means it - but what
// is at the end has to be a file. A directory cannot be read as one, and a
// pipe or a device can be read forever.
//
// Asked of the open file rather than of the name, since 2026-09-30. The name
// was asked first and opened second until then, and between the two looks it
// could come to stand for a pipe, which open then waited on for good - a
// review of #157 found it. Now there is one look, at what was opened, and the
// open itself does not wait (openForLooking). The price is that a device named
// on purpose is opened and closed again, never read.
func OpenRegular(path string) (*os.File, os.FileInfo, error) {
	f, err := openForLooking(path)
	if err != nil {
		return nil, nil, err
	}
	info, err := f.Stat()
	if err == nil && !info.Mode().IsRegular() {
		err = &NotRegularError{Path: path, Directory: info.IsDir()}
	}
	if err == nil {
		err = waitAgain(f)
	}
	if err != nil {
		_ = f.Close()
		return nil, nil, err
	}
	return f, info, nil
}
