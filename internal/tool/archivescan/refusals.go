package archivescan

import (
	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/tool"
)

// NotAFileError is a path naming something that is not a file.
type NotAFileError struct {
	Path      string
	Directory bool
}

// AboutSetting is the input this is about.
func (e *NotAFileError) AboutSetting() string { return InputArchive }

func (e *NotAFileError) what() core.Said {
	if e.Directory {
		return core.Says("archivescan.IsADirectory", "%s is a directory and this looks into one archive", core.A("Path", e.Path))
	}
	return core.Says("archivescan.IsNotAFile", "%s is not a file but a device, a pipe or a socket", core.A("Path", e.Path))
}

func (e *NotAFileError) why() core.Said {
	if e.Directory {
		return core.Says("archivescan.ADirectoryIsUnpacked", "a directory is what an archive unpacks into, not an archive")
	}
	return core.Says("archivescan.ReadingMayNeverEnd", "reading one may never end, so it is not started")
}

func (e *NotAFileError) instead() core.Said {
	return core.Says("archivescan.NameOneArchive", "name one archive file")
}

func (e *NotAFileError) Error() string { return e.Said().String() }

// Unwrap is the refusal every tool shares.
func (e *NotAFileError) Unwrap() error {
	return &tool.NotRegularError{Path: e.Path, Directory: e.Directory}
}

// Said is the whole refusal, for a window that says it in its own language.
func (e *NotAFileError) Said() core.Said { return tool.Said(e.what(), e.why(), e.instead()) }

// Class says this is a mistake in the request.
func (e *NotAFileError) Class() tool.Class { return tool.Asked }

// NotAnArchiveError is a file that begins neither as a ZIP nor as a TAR nor as
// gzip.
type NotAnArchiveError struct {
	Path string
}

// AboutSetting is the input this is about.
func (e *NotAnArchiveError) AboutSetting() string { return InputArchive }

func (e *NotAnArchiveError) what() core.Said {
	return core.Says("archivescan.IsNotAnArchive", "%s is not a ZIP, TAR, TAR.GZ or gzip file", core.A("Path", e.Path))
}

func (e *NotAnArchiveError) why() core.Said {
	return core.Says("archivescan.ItDoesNotBegin", "it does not begin the way any of them does, and the name of a file does not decide what it is")
}

func (e *NotAnArchiveError) instead() core.Said {
	return core.Says("archivescan.NameAZipOrTar", "name a ZIP, TAR, TAR.GZ or gzip file")
}

func (e *NotAnArchiveError) Error() string { return e.Said().String() }

// Said is the whole refusal, for a window that says it in its own language.
func (e *NotAnArchiveError) Said() core.Said { return tool.Said(e.what(), e.why(), e.instead()) }

// Class says this is a mistake in the request.
func (e *NotAnArchiveError) Class() tool.Class { return tool.Asked }

// NotReadYetError is an archive of a kind this tool recognises and does not
// read.
type NotReadYetError struct {
	Path string
	Kind string
}

// AboutSetting is the input this is about.
func (e *NotReadYetError) AboutSetting() string { return InputArchive }

func (e *NotReadYetError) what() core.Said {
	return core.Says("archivescan.NotReadYet", "%s is a %s archive, which this tool does not read yet", core.A("Path", e.Path), core.A("Kind", e.Kind))
}

func (e *NotReadYetError) why() core.Said {
	return core.Says("archivescan.ItsOwnFormat", "its list of contents is written in a format of its own")
}

func (e *NotReadYetError) instead() core.Said {
	return core.Says("archivescan.PackAsZip", "pack the same files as ZIP or TAR, and look into that")
}

func (e *NotReadYetError) Error() string { return e.Said().String() }

// Said is the whole refusal, for a window that says it in its own language.
func (e *NotReadYetError) Said() core.Said { return tool.Said(e.what(), e.why(), e.instead()) }

// Class says this is a mistake in the request.
func (e *NotReadYetError) Class() tool.Class { return tool.Asked }

// ChangedError is an archive that changed while it was being read.
type ChangedError struct {
	Path string
}

// AboutSetting is the input this is about.
func (e *ChangedError) AboutSetting() string { return InputArchive }

func (e *ChangedError) what() core.Said {
	return core.Says("archivescan.Changed", "%s changed while it was being read", core.A("Path", e.Path))
}

func (e *ChangedError) why() core.Said {
	return core.Says("archivescan.AnswerAboutNoVersion", "an answer about a file being written is an answer about no version of it")
}

func (e *ChangedError) instead() core.Said {
	return core.Says("archivescan.WaitAndRunAgain", "wait until whatever is writing it has finished, and run this again")
}

func (e *ChangedError) Error() string { return e.Said().String() }

// Said is the whole refusal, for a window that says it in its own language.
func (e *ChangedError) Said() core.Said { return tool.Said(e.what(), e.why(), e.instead()) }

// Class says the disk did not cooperate rather than the request being wrong.
func (e *ChangedError) Class() tool.Class { return tool.Reading }
