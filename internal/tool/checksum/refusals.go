package checksum

import (
	"fmt"
	"strings"

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
}

// AboutSetting is the input this is about.
func (e *NotAFileError) AboutSetting() string { return InputFile }

// What happened.
func (e *NotAFileError) What() string {
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
