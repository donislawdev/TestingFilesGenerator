package engine

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// The rules a file name has to pass before anything is written under it.
// Moved out of engine.go on 2026-09-24, when the length rule (O239) took
// that file past its ceiling: this is the part of planning that judges a
// name, and nothing else in it does.

// checkFileName keeps a name a name.
//
// A name carrying a path escapes the directory the run was pointed at. A
// recipe travels between teams by design, so "../../something" in a file
// somebody sent over would write outside the directory its reader chose - and
// the free space check, the collision check and cleanup all work on the
// directory, so none of them would be looking in the right place.
//
// Both separators are refused on every system, not just the local one. A name
// holding a backslash is legal on Linux and cannot exist on Windows, and a
// recipe that only works on the machine it was written on is not portable.
func checkFileName(setting, where, name string) error {
	switch {
	case name == "":
		return &RecipeError{Setting: setting, Detail: fmt.Sprintf("%s produces a file with no name", where)}

	case strings.ContainsAny(name, `/\`):
		return &RecipeError{Setting: setting,
			Detail:  fmt.Sprintf("%s produces the name %q, which is a path rather than a file name", where, name),
			Because: "names stay inside the output directory, and a separator is refused on every system so that a recipe works everywhere",
			Remedy:  "Choose the directory with the output setting instead"}

	// A colon, on every system, for the same reason as a separator.
	//
	// Windows reads it as the start of an alternate data stream, so
	// "AB:c.txt" names a stream called c.txt inside a file called AB.
	// Measured on 2026-08-04: the run reported the file as not produced and
	// ended with the partial code, and an empty file called AB was left in the
	// directory anyway - not in the manifest, reported by verify as something
	// nobody asked for, and beyond the reach of cleanup for good. A single
	// letter in front of it is read as a drive instead, which is how the same
	// recipe came to be accepted on Linux and refused on Windows.
	//
	// Legal in a name on Linux and macOS, and refused there too. A recipe
	// travels between machines by design, and one that quietly leaves debris on
	// somebody else's is worse than one refused on all of them.
	case strings.Contains(name, ":"):
		return &RecipeError{Setting: setting,
			Detail:  fmt.Sprintf("%s produces the name %q, which holds a colon", where, name),
			Because: "Windows reads that as a drive or as an alternate data stream rather than as part of the name, so the file arrives called something else or not at all. It is refused on every system so that a recipe means one thing everywhere",
			Remedy:  "Take the colon out, or ask for the file inside an archive where the name survives"}

	// Characters Windows will not put in a file name, refused on every system
	// for the same reason as the separator and the colon above.
	//
	// Measured on 2026-08-25, on each of <>"|?* and on a name holding a tab.
	// All seven planned cleanly, --dry-run answered "1 file in 1 target" and
	// exit 0, and the run then failed that one file with the system's own
	// words: "open a<b.txt.tfg-partial-53628: The filename, directory name, or
	// volume label syntax is incorrect". Three things wrong in one line. The
	// dry run answered for a run that could not happen, which is the fault
	// preflight exists to stop. The sentence is not ours and carries none of
	// the four parts a refusal owes a reader, while carrying the temporary
	// name, which is ours and nobody else's business. And the same recipe
	// writes the file on Linux, where all of these are legal - the reason the
	// separator has been refused everywhere since 2026-08-03.
	//
	// Producing such a name on purpose is a real test case and it belongs to
	// the name laboratory, which writes it into an archive rather than onto
	// the host filesystem. See D10.
	case firstForbidden(name) != 0:
		bad := firstForbidden(name)
		return &RecipeError{Setting: setting,
			Detail:  fmt.Sprintf("%s produces the name %q, which holds %s", where, name, describeForbidden(bad)),
			Because: "Windows refuses that character in a file name, so the file is not written there at all. It is refused on every system so that a recipe means one thing everywhere",
			Remedy:  "Take the character out, or ask for the file inside an archive where the name survives"}

	// Longer than every system this runs on will store, refused on every
	// system for the same reason as the characters above.
	//
	// The three file systems count three different things: ext4 bytes, NTFS
	// UTF-16 units, APFS characters. Measured on 2026-09-24 with one recipe on
	// each: a name of 200 CJK characters, 604 bytes, was written on Windows
	// and on macOS and cannot be on Linux, where the run ended with the
	// partial code and a sentence about a file not produced, with no reason
	// in it. Bytes are never fewer than the other two counts, so the limit in
	// bytes is the one that holds on all three (O239).
	case len(name) > core.MaxNameBytes:
		return &RecipeError{Setting: setting,
			Detail: fmt.Sprintf("%s produces the name %q, which is %d bytes long", where, name, len(name)),
			Because: fmt.Sprintf("Linux stores at most %d bytes in a file name, and a letter outside ASCII takes two to four of them. "+
				"It is refused on every system so that a recipe means one thing everywhere", core.MaxNameBytes),
			Remedy: fmt.Sprintf("Shorten it to %d bytes or fewer, or ask for the file inside an archive where the name survives", core.MaxNameBytes)}

	// One reserved device name, and one only.
	//
	// The folklore list has twenty two - CON, PRN, AUX, COM1 to COM9, LPT1 to
	// LPT9 - and every one of them was measured on 2026-08-25 rather than
	// remembered, on two editions: Windows 11 Pro 26200 and Windows Server
	// 2025 build 26100. con, con.txt, prn, aux, com1, com1.bin, lpt1 and
	// conin$ each came back an ordinary file that verify then passed, on both.
	// Refusing that list would refuse con.pdf, a name both systems store
	// perfectly well, which is a rule written from memory doing damage.
	//
	// NUL is the exception on both, and it is the one worth catching, because
	// it does not fail. The write succeeds and the bytes go nowhere, which is
	// the silence rule broken as completely as it can be - a manifest
	// describing a file that was never on the disk. The run is refused a step
	// later today, by the collision check finding something at the path, and
	// that is safe but tells the reader to remove a file nobody can remove.
	//
	// The bare name only. An extension saves it - nul.txt is an ordinary file
	// on both editions - so this is not the "any extension" rule the folklore
	// describes either.
	case strings.EqualFold(name, "nul"):
		return &RecipeError{Setting: setting,
			Detail:  fmt.Sprintf("%s produces the name %q, which names the null device on Windows rather than a file", where, name),
			Because: "writing there succeeds and the bytes go nowhere, so the run would record a file that is not on the disk. It is refused on every system so that a recipe means one thing everywhere",
			Remedy:  "Give it an extension, nul.txt is an ordinary name, or choose another one"}

	case name == "." || name == "..":
		return &RecipeError{Setting: setting, Detail: fmt.Sprintf(
			"%s produces the name %q, which names a directory rather than a file", where, name)}

	// A name Windows stores under a different name than the one it was given.
	// Refused on every system for the same reason a separator is: a recipe that
	// only works on the machine it was written on is not portable.
	//
	// Measured on 2026-08-03, and it is the silence rule broken rather than a
	// portability nicety. "--name trailing." finished with exit code 0, the
	// file landed as "trailing", and the manifest recorded "trailing." - so the
	// run described a file that was not there under that name, and "tfg verify"
	// on the tool's own output failed with exit code 7 a second later.
	//
	// Producing such a name deliberately is a real test case and it belongs to
	// the name laboratory, which writes it into an archive rather than onto the
	// host filesystem for exactly this reason. See D10.
	case strings.HasSuffix(name, ".") || strings.HasSuffix(name, " "):
		return &RecipeError{Setting: setting,
			Detail:  fmt.Sprintf("%s produces the name %q, which ends in a dot or a space", where, name),
			Because: "Windows stores such a name without it, so the file on disk would not be the file the manifest describes and verify would report both",
			Remedy:  "Take the last character off, or ask for the file inside an archive where the name survives"}

	// Judged the same way on every system, like the separator above. Using
	// filepath here asks the machine this build runs on, and the answer
	// differs: measured on 2026-08-04, "a:b.txt" was accepted on Linux and
	// refused on Windows from one recipe, so a fixture set written on one
	// machine failed on the next. That is the failure this rule exists to
	// prevent, arriving through the rule itself.
	case filepath.IsAbs(name) || core.HasVolumeName(name):
		return &RecipeError{Setting: setting,
			Detail:  fmt.Sprintf("%s produces the absolute path %q", where, name),
			Because: "a recipe carries no absolute paths, because then it only works on the machine it was written on",
			Remedy:  "Choose the directory with the output setting instead"}
	}
	return nil
}

// forbiddenChars are the printable characters Windows refuses in a file name.
//
// The separator and the colon are not here. They are refused above with their
// own sentences, because what goes wrong with them is not "the file is not
// written" but something worse and worth its own explanation - a name that
// leaves the output directory, and a name Windows reads as a drive or as a
// stream inside another file.
const forbiddenChars = `<>"|?*`

// firstForbidden is the first character of a name that Windows will not store,
// or zero when there is none.
//
// The first rather than all of them, because a refusal naming one character a
// reader can find beats a list they have to compare against their own name.
func firstForbidden(name string) rune {
	for _, r := range name {
		// Below the space, which is every control character. Windows refuses
		// the whole range, and a name holding one is unreadable on any system
		// - a tab in a file name is a name nobody can type back.
		if r < 0x20 || strings.ContainsRune(forbiddenChars, r) {
			return r
		}
	}
	return 0
}

// describeForbidden names a character in a way somebody can act on. A control
// character has nothing to show, so it is given as its number instead of being
// printed into the middle of a sentence where it would do what it says.
func describeForbidden(r rune) string {
	if r < 0x20 {
		return fmt.Sprintf("a control character, U+%04X", r)
	}
	return fmt.Sprintf("the character %q", string(r))
}
