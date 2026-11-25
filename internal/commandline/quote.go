package commandline

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Shell is the syntax a line is written in.
type Shell int

const (
	// PowerShell is what Windows opens a terminal with, both Windows
	// PowerShell 5.1 and PowerShell 7.
	PowerShell Shell = iota + 1
	// POSIX is sh and the shells that read its quoting the same way: bash and
	// zsh, which is what macOS and Linux open a terminal with.
	POSIX
)

// ShellFor is the shell a terminal on this system opens with, by the name Go
// gives the system.
//
// One per system rather than a choice on the screen, the decision the owner
// made for the same button in QA Data Generator. cmd.exe is not one of them: it
// expands %NAME% even inside quotes, and a line pasted there has no way to keep
// a value from being read as a variable.
func ShellFor(goos string) Shell {
	if goos == "windows" {
		return PowerShell
	}
	return POSIX
}

// quote writes a value so that the shell reads it as one argument and as
// nothing else.
//
// Every value outside a short list is quoted, not only one with a space. A value
// is often the name of a folder or a file, and a name may carry what a shell
// reads as code: in PowerShell a semicolon starts a second command and $( runs
// one inside double quotes. Single quotes in both shells take the text as it
// stands, and the one character that has to be written differently inside them
// is the single quote itself - so that is the only part that follows the system.
func quote(value string, s Shell) string {
	if bare(value, s) {
		return value
	}
	if s == PowerShell {
		return "'" + powerShellQuotes.Replace(value) + "'"
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// powerShellQuotes doubles every character PowerShell reads as a single quote:
// the plain one and four typographic forms of it. Measured with the parser of
// PowerShell 7 in QA Data Generator on 2026-10-26: each of the four, left single
// inside quotes, closed them and gave the line a second command. Written as
// numbers so that this file stays plain ASCII.
var powerShellQuotes = strings.NewReplacer(
	"'", "''",
	string(rune(0x2018)), strings.Repeat(string(rune(0x2018)), 2),
	string(rune(0x2019)), strings.Repeat(string(rune(0x2019)), 2),
	string(rune(0x201A)), strings.Repeat(string(rune(0x201A)), 2),
	string(rune(0x201B)), strings.Repeat(string(rune(0x201B)), 2),
)

// bare reports whether a value means only itself in the shell as it stands.
//
// ASCII letters and digits and eight marks, with the backslash as well in
// PowerShell, where it separates the parts of a path and means nothing else.
// Everything outside ASCII is quoted, because PowerShell reads some of those
// characters as quotes and dashes. The comma is quoted although sh would leave
// it alone, because PowerShell reads an unquoted comma as a list.
//
// Two first characters are quoted wherever they stand: a dash, which a shell
// may read as the start of a flag of its own, and an equals sign, which zsh
// expands into the path of the program named after it.
func bare(value string, s Shell) bool {
	if value == "" || value[0] == '-' || value[0] == '=' {
		return false
	}
	for _, c := range value {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case strings.ContainsRune("_-./:=+%", c):
		case c == '\\' && s == PowerShell:
		default:
			return false
		}
	}
	return true
}

// olderPowerShellDiffers reports whether Windows PowerShell 5.1 hands this value
// to a program differently from PowerShell 7.3 and later.
//
// Measured on 2026-11-25 on this project's own machine, 5.1.26100 against
// 7.7.0, with the same line given to a program that prints what it received.
// 5.1 does not escape a double quote when it builds the command line, so 'a"b'
// arrives as ab, and it wraps a value holding a space in quotes without
// doubling a backslash at its end, so 'C:\My Dir\' swallows the closing quote
// and the next argument with it. 7.7 gets both right, and no one line is right
// in both, so the window says which PowerShell the line needs instead. 7.3 is
// the first that builds the line this way by default - the documentation of
// $PSNativeCommandArgumentPassing - and 7.0 to 7.2 hand values over as 5.1 does.
func olderPowerShellDiffers(value string) bool {
	if strings.ContainsRune(value, '"') {
		return true
	}
	return strings.ContainsAny(value, " \t") && strings.HasSuffix(value, `\`)
}

// writable refuses a value no line can carry.
//
// A line break would end the command in the middle of a value, and a terminal
// pasting it would run the first half. Every other control character is refused
// with it, because it cannot be seen in the line it would be pasted as.
func writable(value, setting string) error {
	if !utf8.ValidString(value) {
		return &Unwritable{Setting: setting, Reason: NotText}
	}
	for _, c := range value {
		if unicode.IsControl(c) || c == 0x2028 || c == 0x2029 {
			return &Unwritable{Setting: setting, Reason: ControlCharacter}
		}
	}
	return nil
}

// Reason is why a value cannot be written on a command line.
type Reason int

const (
	// ControlCharacter is a line break, a tab or another character that
	// cannot be seen.
	ControlCharacter Reason = iota + 1
	// NotText is a value that is not valid UTF-8.
	NotText
	// CommaInDamage is a comma in a setting of a damage, which --damage uses
	// to separate its settings and has no way to escape.
	CommaInDamage
	// LeadingDash is an argument starting with a dash, which the command
	// would read as a flag.
	LeadingDash
	// EndsText is a line of a file a Script saves that the shell would read
	// as the end of the text - see Script.Write.
	EndsText
)

// Unwritable is a value the line cannot carry, and the setting it came from.
type Unwritable struct {
	Setting string
	Reason  Reason
}

func (u *Unwritable) Error() string {
	switch u.Reason {
	case ControlCharacter:
		return u.Setting + " holds a line break or another control character, which a command line cannot carry"
	case NotText:
		return u.Setting + " is not valid text"
	case CommaInDamage:
		return u.Setting + " holds a comma, and --damage separates its settings with commas"
	case LeadingDash:
		return u.Setting + " starts with a dash, which the command would read as a flag"
	case EndsText:
		return "a line of the file would end the text the shell saves before the file is whole"
	}
	return u.Setting + " cannot be written on a command line"
}
