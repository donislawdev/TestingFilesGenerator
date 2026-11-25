package text

import "strings"

// The words of the Copy command button, which puts on the clipboard the command
// that does what the screen would do. See internal/commandline for how the
// command is written and docs, the analysis of 2026-11-25, for why.

// ButtonCopyCommand puts the command for the screen on the clipboard.
func ButtonCopyCommand() string { return say("ButtonCopyCommand", "Copy command") }

// CommandCopiedPowerShell says the command is on the clipboard and which shell
// it is written for, because pressing the button changes nothing that can be
// seen and a line pasted into the wrong shell fails.
func CommandCopiedPowerShell() string {
	return say("CommandCopiedPowerShell", "The command is on the clipboard, written for PowerShell.")
}

// CommandCopiedTerminal is CommandCopiedPowerShell on macOS and Linux.
func CommandCopiedTerminal() string {
	return say("CommandCopiedTerminal", "The command is on the clipboard, written for a terminal such as bash or zsh.")
}

// CommandNeedsPowerShell7 follows the line above when a value is one Windows
// PowerShell 5.1 would change before tfg sees it - a double quote, or a last
// backslash after a space - and names the boxes it is in. PowerShell 7 before
// 7.3 hands such a value over the way 5.1 does, so the version is named whole.
func CommandNeedsPowerShell7(labels []string) string {
	return sayf("CommandNeedsPowerShell7",
		"Run it in PowerShell 7.3 or later. Windows PowerShell 5.1 and earlier versions of PowerShell 7 would change the double quotes or the last backslash in {{.Settings}} before tfg reads them.",
		map[string]any{"Settings": strings.Join(labels, ", ")})
}

// CommandWithoutTfg follows the first line when tfg is neither where a
// terminal looks for programs nor next to this window, so the line cannot
// run here as it stands.
func CommandWithoutTfg() string {
	return say("CommandWithoutTfg",
		"tfg was not found on this computer. Install the command line, or put the folder it is in on PATH, before you run this.")
}

// CommandControlCharacter is a refusal under a box whose value holds a line
// break or another character nobody can see.
func CommandControlCharacter() string {
	return say("CommandControlCharacter",
		"A command line cannot carry a line break or another invisible character. Remove it to copy the command.")
}

// CommandNotText is a refusal under a box whose value is not readable text.
func CommandNotText() string {
	return say("CommandNotText",
		"This value is not readable text, so it cannot be written into a command. Type it again to copy the command.")
}

// CommandCommaInDamage is a refusal under a setting of a damage that holds a
// comma, which the command line uses to separate the settings of a damage.
func CommandCommaInDamage() string {
	return say("CommandCommaInDamage",
		"A command line cannot carry a comma here, because --damage separates its settings with commas. Remove the comma to copy the command.")
}

// CommandLeadingDash is a refusal under a box whose value starts with a dash,
// which the command would read as an option.
func CommandLeadingDash() string {
	return say("CommandLeadingDash",
		"A command would read a value starting with a dash as an option. Give the full path instead.")
}
