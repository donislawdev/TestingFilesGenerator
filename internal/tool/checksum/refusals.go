package checksum

import (
	"fmt"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/tool"
)

// UnknownAlgorithmError is an algorithm this tool does not work out.
type UnknownAlgorithmError struct {
	Name string
}

// AboutSetting is the setting this is about, for the box it goes under.
func (e *UnknownAlgorithmError) AboutSetting() string { return SettingAlgorithm }

// What happened.
func (e *UnknownAlgorithmError) what() core.Said {
	return core.Says("checksum.ThereIsNoAlgorithmCalled", "there is no algorithm called %q", core.A("Name", e.Name))
}

// Why it is refused rather than matched to the nearest name.
func (e *UnknownAlgorithmError) why() core.Said {
	return core.Says("checksum.AnAlgorithmIsNamedExactlyBecause", "an algorithm is named exactly, because two that sound alike give different checksums")
}

// Instead is the list, from the table the tool works from.
func (e *UnknownAlgorithmError) instead() core.Said {
	return core.Says("checksum.UseAlgorithms", "use %s, several separated by commas, or all on its own", core.A("Algorithms", strings.Join(Names(), ", ")))
}

func (e *UnknownAlgorithmError) Error() string { return e.Said().String() }

// Said is the whole refusal, for a window that says it in its own language.
func (e *UnknownAlgorithmError) Said() core.Said { return tool.Said(e.what(), e.why(), e.instead()) }

// Class says this is a mistake in the request.
func (e *UnknownAlgorithmError) Class() tool.Class { return tool.Asked }

// ExpectedError is a checksum to compare with that is not one.
type ExpectedError struct {
	Given string
	// Digits is how long it was, when it was hexadecimal of a length no
	// algorithm here has. Zero when it was not hexadecimal at all.
	Digits int
}

// AboutSetting is the setting this is about.
func (e *ExpectedError) AboutSetting() string { return SettingExpect }

// What happened.
func (e *ExpectedError) what() core.Said {
	if e.Digits == 0 {
		return core.Says("checksum.IsNotAChecksumWrittenIn", "%q is not a checksum written in hexadecimal", core.A("Given", e.Given))
	}
	return core.Says("checksum.AChecksumDigitsLongIsNot", "a checksum %d digits long is not one this tool works out", core.A("Digits", e.Digits))
}

// Why the length matters.
func (e *ExpectedError) why() core.Said {
	return core.Says("checksum.TheLengthOfAChecksumSays", "the length of a checksum says which algorithm made it, and that is the one worked out to compare")
}

// Instead is the lengths there are.
func (e *ExpectedError) instead() core.Said {
	lengths := make([]string, 0, len(algorithms))
	for _, a := range algorithms {
		lengths = append(lengths, fmt.Sprintf("%s %d", a.name, a.digits))
	}
	return core.Says("checksum.PasteAlone", "paste the checksum alone, without the file name. The lengths are: %s", core.A("Lengths", strings.Join(lengths, ", ")))
}

func (e *ExpectedError) Error() string { return e.Said().String() }

// Said is the whole refusal, for a window that says it in its own language.
func (e *ExpectedError) Said() core.Said { return tool.Said(e.what(), e.why(), e.instead()) }

// Class says this is a mistake in the request.
func (e *ExpectedError) Class() tool.Class { return tool.Asked }

// NotAFileError is a path naming something that is not a file.
type NotAFileError struct {
	Path      string
	Directory bool
	// Input is the input the path was given as. Empty is the file of the
	// checksum tool, which is where this error was first raised.
	Input string
}

// AboutSetting is the input this is about.
func (e *NotAFileError) AboutSetting() string {
	if e.Input == "" {
		return InputFile
	}
	return e.Input
}

// What happened.
func (e *NotAFileError) what() core.Said {
	if e.Directory && e.Input == InputChecksumFile {
		return core.Says("checksum.IsADirectoryAndThisReads", "%s is a directory and this reads one checksum file", core.A("Path", e.Path))
	}
	if e.Directory {
		return core.Says("checksum.IsADirectoryAndThisWorks", "%s is a directory and this works out the checksum of one file", core.A("Path", e.Path))
	}
	return core.Says("checksum.IsNotAFileButA", "%s is not a file but a device, a pipe or a socket", core.A("Path", e.Path))
}

// Why it is not read anyway.
func (e *NotAFileError) why() core.Said {
	if e.Directory {
		return core.Says("checksum.ADirectoryHasNoBytesOf", "a directory has no bytes of its own to work a checksum out of")
	}
	return core.Says("checksum.ReadingOneMayNeverEndSo", "reading one may never end, so it is not started")
}

// Instead is what to name.
func (e *NotAFileError) instead() core.Said {
	return core.Says("checksum.NameOneFile", "name one file")
}

func (e *NotAFileError) Error() string { return e.Said().String() }

// Said is the whole refusal, for a window that says it in its own language.
func (e *NotAFileError) Said() core.Said { return tool.Said(e.what(), e.why(), e.instead()) }

// Class says this is a mistake in the request: the path names something
// that is there, and it is the wrong kind of thing.
func (e *NotAFileError) Class() tool.Class { return tool.Asked }

// ChangedError is a file that changed while it was being read.
type ChangedError struct {
	Path string
}

// AboutSetting is the input this is about.
func (e *ChangedError) AboutSetting() string { return InputFile }

// What happened.
func (e *ChangedError) what() core.Said {
	return core.Says("checksum.ChangedWhileItWasBeingRead", "%s changed while it was being read", core.A("Path", e.Path))
}

// Why no checksum is given.
func (e *ChangedError) why() core.Said {
	return core.Says("checksum.AChecksumOfAFileBeing", "a checksum of a file being written is the checksum of no version of it")
}

// Instead is when to try again.
func (e *ChangedError) instead() core.Said {
	return core.Says("checksum.WaitUntilWhateverIsWritingIt", "wait until whatever is writing it has finished, and run this again")
}

func (e *ChangedError) Error() string { return e.Said().String() }

// Said is the whole refusal, for a window that says it in its own language.
func (e *ChangedError) Said() core.Said { return tool.Said(e.what(), e.why(), e.instead()) }

// Class says the request was fine and the file did not hold still.
func (e *ChangedError) Class() tool.Class { return tool.Reading }

// NotAFolderError is a folder tool given something that is not a folder.
type NotAFolderError struct {
	Path string
}

// AboutSetting is the input this is about.
func (e *NotAFolderError) AboutSetting() string { return InputFolder }

// What happened.
func (e *NotAFolderError) what() core.Said {
	return core.Says("checksum.IsAFileAndThisLists", "%s is a file and this lists a folder", core.A("Path", e.Path))
}

// Why a file will not do.
func (e *NotAFolderError) why() core.Said {
	return core.Says("checksum.TheChecksumFileIsWrittenBeside", "the checksum file is written beside the files it lists, so it needs the folder they are in")
}

// Instead is what to name.
func (e *NotAFolderError) instead() core.Said {
	return core.Says("checksum.NameTheFolderOrUseTfg", "name the folder, or use tfg tool checksum for the one file")
}

func (e *NotAFolderError) Error() string { return e.Said().String() }

// Said is the whole refusal, for a window that says it in its own language.
func (e *NotAFolderError) Said() core.Said { return tool.Said(e.what(), e.why(), e.instead()) }

// Class says this is a mistake in the request.
func (e *NotAFolderError) Class() tool.Class { return tool.Asked }

// SumsExistError is a checksum file already where one would be written.
type SumsExistError struct {
	Path string
}

// AboutSetting is the input this is about.
func (e *SumsExistError) AboutSetting() string { return InputFolder }

// What happened.
func (e *SumsExistError) what() core.Said {
	return core.Says("checksum.IsAlreadyThereSoNothingWas", "%s is already there, so nothing was written", core.A("Path", e.Path))
}

// Why it is not written over.
func (e *SumsExistError) why() core.Said {
	return core.Says("checksum.AChecksumFileIsNeverWritten", "a checksum file is never written over, since it may be the only record of what the folder held")
}

// Instead is what to do with it.
func (e *SumsExistError) instead() core.Said {
	return core.Says("checksum.CheckTheFolderAgainstItWith", "check the folder against it with tfg tool checksum-check, or move it away and run this again")
}

func (e *SumsExistError) Error() string { return e.Said().String() }

// Said is the whole refusal, for a window that says it in its own language.
func (e *SumsExistError) Said() core.Said { return tool.Said(e.what(), e.why(), e.instead()) }

// Class says the request was fine and the folder is not in the state it needs.
func (e *SumsExistError) Class() tool.Class { return tool.Reading }

// NothingToListError is a folder with no file a checksum file could list.
type NothingToListError struct {
	Folder string
	// LeftOut is how many names under it are not files - links, pipes, and
	// what a stopped run left half written.
	LeftOut int
}

// AboutSetting is the input this is about.
func (e *NothingToListError) AboutSetting() string { return InputFolder }

// What happened.
func (e *NothingToListError) what() core.Said {
	if e.LeftOut > 0 {
		return core.Says("checksum.HoldsNoFileToListOnly", "%s holds no file to list, only %s that are not files", core.A("Folder", e.Folder), core.A("Names", core.SaysN("checksum.Names", "%d name", "%d names", core.A("Count", e.LeftOut))))
	}
	return core.Says("checksum.HoldsNoFileToList", "%s holds no file to list", core.A("Folder", e.Folder))
}

// Why an empty checksum file is not written.
func (e *NothingToListError) why() core.Said {
	return core.Says("checksum.AChecksumFileWithoutALine", "a checksum file without a line in it is refused by sha256sum -c, so it would check nothing")
}

// Instead is what to name.
func (e *NothingToListError) instead() core.Said {
	return core.Says("checksum.NameAFolderWithAtLeast", "name a folder with at least one file in it")
}

func (e *NothingToListError) Error() string { return e.Said().String() }

// Said is the whole refusal, for a window that says it in its own language.
func (e *NothingToListError) Said() core.Said { return tool.Said(e.what(), e.why(), e.instead()) }

// Class says this is a mistake in the request.
func (e *NothingToListError) Class() tool.Class { return tool.Asked }

// FolderUnreadableError is a folder with something in it that could not be
// read, which refuses the whole checksum file.
type FolderUnreadableError struct {
	Folder string
	// Problems is every one of them, each a path and what the system said.
	Problems []core.Said
}

// AboutSetting is the input this is about.
func (e *FolderUnreadableError) AboutSetting() string { return InputFolder }

// What happened.
func (e *FolderUnreadableError) what() core.Said {
	return core.Says("checksum.NotEverythingUnderCouldBeRead", "not everything under %s could be read, so nothing was written", core.A("Folder", e.Folder))
}

// Why a checksum file with a gap is not written.
func (e *FolderUnreadableError) why() core.Said {
	return core.Says("checksum.AChecksumFilePromisesEveryFile", "a checksum file promises every file in the folder, and one with a gap in it would not say where the gap is")
}

// Instead is what to do. The list follows it.
func (e *FolderUnreadableError) instead() core.Said {
	return core.Says("checksum.MakeTheseReadableOrMoveThem", "make these readable or move them out of the folder, and run this again")
}

// Error is the sentence and then every problem on a line of its own, so none
// of them is left for the next try to find.
func (e *FolderUnreadableError) Error() string { return e.Said().String() }

// Said is the whole refusal with its list, for a window that says it in its
// own language.
func (e *FolderUnreadableError) Said() core.Said {
	return core.Says("checksum.FolderUnreadable", "%s:\n  %s",
		core.A("Sentence", tool.Said(e.what(), e.why(), e.instead())), core.A("Problems", core.Lines(e.Problems)))
}

// Class says the request was fine and the disk did not cooperate.
func (e *FolderUnreadableError) Class() tool.Class { return tool.Reading }

// NoRoomError is a checksum file the disk has no room for.
type NoRoomError struct {
	Path       string
	Need, Have int64
}

// What happened.
func (e *NoRoomError) what() core.Said {
	return core.Says("checksum.ThereIsNoRoomForSo", "there is no room for %s, so nothing was written", core.A("Path", e.Path))
}

// Why, with the two numbers.
func (e *NoRoomError) why() core.Said {
	return core.Says("checksum.ItTakesAndTheDiskHas", "it takes %s and the disk has %s free", core.A("Need", core.HumanBytes(e.Need)), core.A("Have", core.HumanBytes(e.Have)))
}

// Instead is where to find the room.
func (e *NoRoomError) instead() core.Said {
	return core.Says("checksum.FreeSomeSpaceOnThatDisk", "free some space on that disk and run this again")
}

func (e *NoRoomError) Error() string { return e.Said().String() }

// Said is the whole refusal, for a window that says it in its own language.
func (e *NoRoomError) Said() core.Said { return tool.Said(e.what(), e.why(), e.instead()) }

// Class says the disk is full, which has a code of its own.
func (e *NoRoomError) Class() tool.Class { return tool.Room }

// SumsTooLargeError is a checksum file larger than one this reads.
type SumsTooLargeError struct {
	Path  string
	Bytes int64
}

// AboutSetting is the input this is about.
func (e *SumsTooLargeError) AboutSetting() string { return InputChecksumFile }

// What happened.
func (e *SumsTooLargeError) what() core.Said {
	return core.Says("checksum.IsLargerThanAChecksumFile", "%s is %s, larger than a checksum file this reads", core.A("Path", e.Path), core.A("Bytes", core.HumanBytes(e.Bytes)))
}

// Why there is a limit, and what it is.
func (e *SumsTooLargeError) why() core.Said {
	return core.Says("checksum.TheMostIsAboutHalfA", "the most is %s, about half a million files, and a larger file is more likely not a checksum file at all", core.A("SumsMostBytes", core.HumanBytes(sumsMostBytes)))
}

// Instead is how to check a folder that large.
func (e *SumsTooLargeError) instead() core.Said {
	return core.Says("checksum.CheckTheFolderInPartsEach", "check the folder in parts, each with a checksum file of its own")
}

func (e *SumsTooLargeError) Error() string { return e.Said().String() }

// Said is the whole refusal, for a window that says it in its own language.
func (e *SumsTooLargeError) Said() core.Said { return tool.Said(e.what(), e.why(), e.instead()) }

// Class says the file handed in is not one this takes.
func (e *SumsTooLargeError) Class() tool.Class { return tool.Reading }

// NoSumsError is a file with no checksum line in it.
type NoSumsError struct {
	Path string
}

// AboutSetting is the input this is about.
func (e *NoSumsError) AboutSetting() string { return InputChecksumFile }

// What happened.
func (e *NoSumsError) what() core.Said {
	return core.Says("checksum.HoldsNoChecksumLine", "%s holds no checksum line", core.A("Path", e.Path))
}

// Why that is refused rather than passed.
func (e *NoSumsError) why() core.Said {
	return core.Says("checksum.AChecksumLineIsAChecksum", "a checksum line is a checksum and a path, written as hex  path or as SHA256 (path) = hex, and without one there is nothing to check")
}

// Instead is what to name.
func (e *NoSumsError) instead() core.Said {
	return core.Says("checksum.NameTheChecksumFileItselfSuch", "name the checksum file itself, such as SHA256SUMS, rather than a page or a signature beside it")
}

func (e *NoSumsError) Error() string { return e.Said().String() }

// Said is the whole refusal, for a window that says it in its own language.
func (e *NoSumsError) Said() core.Said { return tool.Said(e.what(), e.why(), e.instead()) }

// Class says the file handed in is not one this can use.
func (e *NoSumsError) Class() tool.Class { return tool.Reading }

// What, Why and Instead are the parts in English, for a report that lays
// them out apart.
func (e *ChangedError) What() string    { return e.what().String() }
func (e *ChangedError) Why() string     { return e.why().String() }
func (e *ChangedError) Instead() string { return e.instead().String() }

// What, Why and Instead are the parts in English, for a report that lays
// them out apart.
func (e *ExpectedError) What() string    { return e.what().String() }
func (e *ExpectedError) Why() string     { return e.why().String() }
func (e *ExpectedError) Instead() string { return e.instead().String() }

// What, Why and Instead are the parts in English, for a report that lays
// them out apart.
func (e *FolderUnreadableError) What() string    { return e.what().String() }
func (e *FolderUnreadableError) Why() string     { return e.why().String() }
func (e *FolderUnreadableError) Instead() string { return e.instead().String() }

// What, Why and Instead are the parts in English, for a report that lays
// them out apart.
func (e *NoRoomError) What() string    { return e.what().String() }
func (e *NoRoomError) Why() string     { return e.why().String() }
func (e *NoRoomError) Instead() string { return e.instead().String() }

// What, Why and Instead are the parts in English, for a report that lays
// them out apart.
func (e *NoSumsError) What() string    { return e.what().String() }
func (e *NoSumsError) Why() string     { return e.why().String() }
func (e *NoSumsError) Instead() string { return e.instead().String() }

// What, Why and Instead are the parts in English, for a report that lays
// them out apart.
func (e *NotAFileError) What() string    { return e.what().String() }
func (e *NotAFileError) Why() string     { return e.why().String() }
func (e *NotAFileError) Instead() string { return e.instead().String() }

// What, Why and Instead are the parts in English, for a report that lays
// them out apart.
func (e *NotAFolderError) What() string    { return e.what().String() }
func (e *NotAFolderError) Why() string     { return e.why().String() }
func (e *NotAFolderError) Instead() string { return e.instead().String() }

// What, Why and Instead are the parts in English, for a report that lays
// them out apart.
func (e *NothingToListError) What() string    { return e.what().String() }
func (e *NothingToListError) Why() string     { return e.why().String() }
func (e *NothingToListError) Instead() string { return e.instead().String() }

// What, Why and Instead are the parts in English, for a report that lays
// them out apart.
func (e *SumsExistError) What() string    { return e.what().String() }
func (e *SumsExistError) Why() string     { return e.why().String() }
func (e *SumsExistError) Instead() string { return e.instead().String() }

// What, Why and Instead are the parts in English, for a report that lays
// them out apart.
func (e *SumsTooLargeError) What() string    { return e.what().String() }
func (e *SumsTooLargeError) Why() string     { return e.why().String() }
func (e *SumsTooLargeError) Instead() string { return e.instead().String() }

// What, Why and Instead are the parts in English, for a report that lays
// them out apart.
func (e *UnknownAlgorithmError) What() string    { return e.what().String() }
func (e *UnknownAlgorithmError) Why() string     { return e.why().String() }
func (e *UnknownAlgorithmError) Instead() string { return e.instead().String() }
