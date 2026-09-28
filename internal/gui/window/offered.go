package window

import (
	"os"
	"path/filepath"
)

// OfferedDirectory is the folder the window offers to write into when nobody
// has said anything, from the three things that decide it: the working
// directory, the directory the program itself lives in and the home
// directory. It asks the file system nothing but whether two directories are
// the same one, so a guard can hand it any three paths without a window.
//
// A folder of our own under the working directory, as since O103 - except
// when the working directory is one the program was never meant to write
// into. There are two of those, both measured on 2026-09-28
// (docs/STARTING-DIRECTORY-2026-09-28.md):
//
//   - the program's own directory. A double click in a file manager starts
//     there, and so does the Start menu shortcut an installer makes. Under
//     Program Files that is a refusal from the system at the first run, and in
//     a package manager's folder it is ten thousand files in a directory the
//     manager may clear at the next upgrade (O254).
//   - the root of a disk. macOS starts an application from Finder in "/",
//     which is read only, so the window offered "/tfg-out" and the first run
//     ended in the system's refusal.
//
// Either one sends the offer to the home directory instead, however the
// program was started - the rule asks where, not how. Any other directory is
// untouched, which is what keeps a terminal as it was: whoever typed their way
// to a directory knows which one it is.
//
// Without a home directory the offer stays where it always was, because a
// path offered before is better than one made up.
func OfferedDirectory(working, program, home string) string {
	if home != "" && notMeantForWriting(working, program) {
		return filepath.Join(home, OutputFolderName)
	}
	return filepath.Join(working, OutputFolderName)
}

// LeftByTheOldOffer says whether a remembered directory is only what the window
// used to offer from a place it should not write into: the folder of our own
// under the program's directory or under the root of a disk.
//
// Closing the window writes down whatever the box held, chosen or not, so
// everybody who once closed a window started from Finder carries "/tfg-out"
// and would be offered it at every start after the offer itself was fixed.
// That one was never somebody's choice and can never work, so it counts as
// nothing remembered (decided by the owner on 2026-09-28).
//
// A remembered folder under a program that has since moved - an older zip
// unpacked somewhere else - is not under THIS program's directory, so it
// stays. The box shows it before anything runs.
func LeftByTheOldOffer(remembered, program string) bool {
	if filepath.Base(remembered) != OutputFolderName {
		return false
	}
	return notMeantForWriting(filepath.Dir(remembered), program)
}

// notMeantForWriting is the one rule both of the above ask.
func notMeantForWriting(dir, program string) bool {
	return isRoot(dir) || sameDirectory(dir, program)
}

// isRoot says whether a directory is the root of its disk: "/" or "C:\". The
// working directory always comes absolute from the system.
//
// A remembered bare "tfg-out" reaches here as ".", and counts as a root on
// purpose. The bare name is what startingDirectory offers when it cannot read
// the working directory, and it means "here" - which the offer works out
// afresh, spelled in full, or as the home directory when "here" is the root
// of a disk. Asking for an absolute path first (outside review of #146) would
// keep the bare name, and started from Finder it would mean "/tfg-out" again.
func isRoot(dir string) bool {
	clean := filepath.Clean(dir)
	return filepath.Dir(clean) == clean
}

// sameDirectory asks the file system rather than compares the text. One
// directory has more than one spelling - letter case on Windows, a short 8.3
// name, a link - and the texts differ while the directory is one. Anything it
// cannot look at is not the same.
func sameDirectory(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	ia, err := os.Stat(a)
	if err != nil {
		return false
	}
	ib, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(ia, ib)
}

// programDirectory is the directory the running program lives in, or nothing
// when the system will not say.
func programDirectory() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(exe)
}
