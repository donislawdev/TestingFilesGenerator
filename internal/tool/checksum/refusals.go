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
func (e *UnknownAlgorithmError) What() string {
	return fmt.Sprintf("there is no algorithm called %q", e.Name)
}

// Why it is refused rather than matched to the nearest name.
func (e *UnknownAlgorithmError) Why() string {
	return "an algorithm is named exactly, because two that sound alike give different checksums"
}

// Instead is the list, from the table the tool works from.
func (e *UnknownAlgorithmError) Instead() string {
	return "use " + strings.Join(Names(), ", ") + ", several separated by commas, or all on its own"
}

func (e *UnknownAlgorithmError) Error() string { return tool.Sentence(e.What(), e.Why(), e.Instead()) }

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
func (e *ExpectedError) What() string {
	if e.Digits == 0 {
		return fmt.Sprintf("%q is not a checksum written in hexadecimal", e.Given)
	}
	return fmt.Sprintf("a checksum %d digits long is not one this tool works out", e.Digits)
}

// Why the length matters.
func (e *ExpectedError) Why() string {
	return "the length of a checksum says which algorithm made it, and that is the one worked out to compare"
}

// Instead is the lengths there are.
func (e *ExpectedError) Instead() string {
	lengths := make([]string, 0, len(algorithms))
	for _, a := range algorithms {
		lengths = append(lengths, fmt.Sprintf("%s %d", a.name, a.digits))
	}
	return "paste the checksum alone, without the file name. The lengths are: " + strings.Join(lengths, ", ")
}

func (e *ExpectedError) Error() string { return tool.Sentence(e.What(), e.Why(), e.Instead()) }

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
func (e *NotAFileError) What() string {
	if e.Directory && e.Input == InputChecksumFile {
		return fmt.Sprintf("%s is a directory and this reads one checksum file", e.Path)
	}
	if e.Directory {
		return fmt.Sprintf("%s is a directory and this works out the checksum of one file", e.Path)
	}
	return fmt.Sprintf("%s is not a file but a device, a pipe or a socket", e.Path)
}

// Why it is not read anyway.
func (e *NotAFileError) Why() string {
	if e.Directory {
		return "a directory has no bytes of its own to work a checksum out of"
	}
	return "reading one may never end, so it is not started"
}

// Instead is what to name.
func (e *NotAFileError) Instead() string {
	return "name one file"
}

func (e *NotAFileError) Error() string { return tool.Sentence(e.What(), e.Why(), e.Instead()) }

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
func (e *ChangedError) What() string {
	return fmt.Sprintf("%s changed while it was being read", e.Path)
}

// Why no checksum is given.
func (e *ChangedError) Why() string {
	return "a checksum of a file being written is the checksum of no version of it"
}

// Instead is when to try again.
func (e *ChangedError) Instead() string {
	return "wait until whatever is writing it has finished, and run this again"
}

func (e *ChangedError) Error() string { return tool.Sentence(e.What(), e.Why(), e.Instead()) }

// Class says the request was fine and the file did not hold still.
func (e *ChangedError) Class() tool.Class { return tool.Reading }

// NotAFolderError is a folder tool given something that is not a folder.
type NotAFolderError struct {
	Path string
}

// AboutSetting is the input this is about.
func (e *NotAFolderError) AboutSetting() string { return InputFolder }

// What happened.
func (e *NotAFolderError) What() string {
	return fmt.Sprintf("%s is a file and this lists a folder", e.Path)
}

// Why a file will not do.
func (e *NotAFolderError) Why() string {
	return "the checksum file is written beside the files it lists, so it needs the folder they are in"
}

// Instead is what to name.
func (e *NotAFolderError) Instead() string {
	return "name the folder, or use tfg tool checksum for the one file"
}

func (e *NotAFolderError) Error() string { return tool.Sentence(e.What(), e.Why(), e.Instead()) }

// Class says this is a mistake in the request.
func (e *NotAFolderError) Class() tool.Class { return tool.Asked }

// SumsExistError is a checksum file already where one would be written.
type SumsExistError struct {
	Path string
}

// AboutSetting is the input this is about.
func (e *SumsExistError) AboutSetting() string { return InputFolder }

// What happened.
func (e *SumsExistError) What() string {
	return fmt.Sprintf("%s is already there, so nothing was written", e.Path)
}

// Why it is not written over.
func (e *SumsExistError) Why() string {
	return "a checksum file is never written over, since it may be the only record of what the folder held"
}

// Instead is what to do with it.
func (e *SumsExistError) Instead() string {
	return "check the folder against it with tfg tool checksum-check, or move it away and run this again"
}

func (e *SumsExistError) Error() string { return tool.Sentence(e.What(), e.Why(), e.Instead()) }

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
func (e *NothingToListError) What() string {
	if e.LeftOut > 0 {
		return fmt.Sprintf("%s holds no file to list, only %s that are not files", e.Folder, core.Count(e.LeftOut, "name", "names"))
	}
	return fmt.Sprintf("%s holds no file to list", e.Folder)
}

// Why an empty checksum file is not written.
func (e *NothingToListError) Why() string {
	return "a checksum file without a line in it is refused by sha256sum -c, so it would check nothing"
}

// Instead is what to name.
func (e *NothingToListError) Instead() string {
	return "name a folder with at least one file in it"
}

func (e *NothingToListError) Error() string { return tool.Sentence(e.What(), e.Why(), e.Instead()) }

// Class says this is a mistake in the request.
func (e *NothingToListError) Class() tool.Class { return tool.Asked }

// FolderUnreadableError is a folder with something in it that could not be
// read, which refuses the whole checksum file.
type FolderUnreadableError struct {
	Folder string
	// Problems is every one of them, each a path and what the system said.
	Problems []string
}

// AboutSetting is the input this is about.
func (e *FolderUnreadableError) AboutSetting() string { return InputFolder }

// What happened.
func (e *FolderUnreadableError) What() string {
	return fmt.Sprintf("not everything under %s could be read, so nothing was written", e.Folder)
}

// Why a checksum file with a gap is not written.
func (e *FolderUnreadableError) Why() string {
	return "a checksum file promises every file in the folder, and one with a gap in it would not say where the gap is"
}

// Instead is what to do. The list follows it.
func (e *FolderUnreadableError) Instead() string {
	return "make these readable or move them out of the folder, and run this again"
}

// Error is the sentence and then every problem on a line of its own, so none
// of them is left for the next try to find.
func (e *FolderUnreadableError) Error() string {
	return tool.Sentence(e.What(), e.Why(), e.Instead()) + ":\n  " + strings.Join(e.Problems, "\n  ")
}

// Class says the request was fine and the disk did not cooperate.
func (e *FolderUnreadableError) Class() tool.Class { return tool.Reading }

// NoRoomError is a checksum file the disk has no room for.
type NoRoomError struct {
	Path       string
	Need, Have int64
}

// What happened.
func (e *NoRoomError) What() string {
	return fmt.Sprintf("there is no room for %s, so nothing was written", e.Path)
}

// Why, with the two numbers.
func (e *NoRoomError) Why() string {
	return fmt.Sprintf("it takes %s and the disk has %s free", core.HumanBytes(e.Need), core.HumanBytes(e.Have))
}

// Instead is where to find the room.
func (e *NoRoomError) Instead() string {
	return "free some space on that disk and run this again"
}

func (e *NoRoomError) Error() string { return tool.Sentence(e.What(), e.Why(), e.Instead()) }

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
func (e *SumsTooLargeError) What() string {
	return fmt.Sprintf("%s is %s, larger than a checksum file this reads", e.Path, core.HumanBytes(e.Bytes))
}

// Why there is a limit, and what it is.
func (e *SumsTooLargeError) Why() string {
	return fmt.Sprintf("the most is %s, about half a million files, and a larger file is more likely not a checksum file at all",
		core.HumanBytes(sumsMostBytes))
}

// Instead is how to check a folder that large.
func (e *SumsTooLargeError) Instead() string {
	return "check the folder in parts, each with a checksum file of its own"
}

func (e *SumsTooLargeError) Error() string { return tool.Sentence(e.What(), e.Why(), e.Instead()) }

// Class says the file handed in is not one this takes.
func (e *SumsTooLargeError) Class() tool.Class { return tool.Reading }

// NoSumsError is a file with no checksum line in it.
type NoSumsError struct {
	Path string
}

// AboutSetting is the input this is about.
func (e *NoSumsError) AboutSetting() string { return InputChecksumFile }

// What happened.
func (e *NoSumsError) What() string {
	return fmt.Sprintf("%s holds no checksum line", e.Path)
}

// Why that is refused rather than passed.
func (e *NoSumsError) Why() string {
	return "a checksum line is a checksum and a path, written as hex  path or as SHA256 (path) = hex, and without one there is nothing to check"
}

// Instead is what to name.
func (e *NoSumsError) Instead() string {
	return "name the checksum file itself, such as SHA256SUMS, rather than a page or a signature beside it"
}

func (e *NoSumsError) Error() string { return tool.Sentence(e.What(), e.Why(), e.Instead()) }

// Class says the file handed in is not one this can use.
func (e *NoSumsError) Class() tool.Class { return tool.Reading }
