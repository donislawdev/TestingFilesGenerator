package core

import (
	"errors"
	"io/fs"
	"os"
)

// Publish puts a finished file under its final name, and refuses when anything
// already holds that name.
//
// Every file this tool finishes goes through here: each generated file, the
// manifest, the instructions beside it and a recipe written by "preset eject
// -o". They are all written whole under a temporary name first, so a run cut
// short never leaves half a file under a real one, and this is the step that
// gives the finished bytes their name.
//
// It replaced os.Rename, and the reason is O252, measured on 2026-09-29 with a
// second writer spinning on the same name (docs/O252-ANALIZA-2026-09-29.md).
// A rename REPLACES whatever is at the destination. So a file somebody else put
// under that name while we were writing - after our look, before our rename -
// was destroyed without a word. The measured loss was 2544 of 3000 names on
// NTFS, around 1900 of 1950 on ext4, tmpfs and overlay, and 999 of 1000 on an
// exFAT pendrive. The same writer through this function lost none on any of
// them, because the system settles "is the name free" and "take it" as one
// operation and says fs.ErrExist otherwise.
//
// The call differs by system, and the files beside this one each name theirs:
// MoveFileEx without MOVEFILE_REPLACE_EXISTING on Windows, renameat2 with
// RENAME_NOREPLACE on Linux, renamex_np with RENAME_EXCL on macOS. A hard link
// was the first idea and it is the fallback rather than the rule, because FAT
// and exFAT have none - measured on Linux, where link answers EPERM, and on
// Windows, where it answers "Incorrect function" even for a name that IS taken.
//
// Two fallbacks stand behind the call, for a filesystem that does not know it -
// some network shares, macOS on a FAT stick. Neither was reached on any disk
// measured, which is why each is taken only for an answer that means "this
// filesystem cannot do that" and never for any other failure:
//
//  1. a hard link to the final name, then the temporary name removed. It
//     refuses a taken name the same way, where links exist at all.
//  2. the rename this replaced, after a look at the final name. That is the
//     window O252 was about, and it is kept rather than refused because the
//     alternative is a tool that cannot write to a stick at all. It is
//     narrower than it was: it is only ever the last resort.
//
// The temporary name is ours in every case: on success it is gone, and on a
// refusal it is left for the caller, which created it and removes it.
func Publish(tmp, final string) error {
	return PublishThrough(tmp, final, renameNoReplace, os.Link)
}

// PublishThrough is Publish with its two system calls passed in.
//
// It exists for one reason: the fallbacks are reached only on a filesystem this
// project's runners do not have, and a fallback nothing can reach is a defence
// nothing can turn red. So a guard walks the chain here on an ordinary disk,
// handing it calls that answer "unsupported". Publish is the only caller in the
// program, and it passes the real ones.
func PublishThrough(tmp, final string, noReplace, link func(string, string) error) error {
	err := noReplace(tmp, final)
	if err == nil {
		return nil
	}
	if !errors.Is(err, errors.ErrUnsupported) {
		return &os.LinkError{Op: "publish", Old: tmp, New: final, Err: err}
	}

	err = link(tmp, final)
	if err == nil {
		// The finished file has both names for a moment. The final one is
		// what counts, so a failure to drop the other is not a failure of
		// the publish - it leaves one of our temporary names behind, which
		// verify already names as ours.
		_ = os.Remove(tmp)
		return nil
	}
	if !linkUnsupported(err) {
		return &os.LinkError{Op: "publish", Old: tmp, New: final, Err: err}
	}

	// "I could not look" is not "nothing is there", and this is the one step
	// that replaces what it lands on - so only a look that found nothing lets
	// it through (the rule of review 2026-08-23, 3.7c, asked again on #150).
	switch _, lookErr := os.Lstat(final); {
	case lookErr == nil:
		return &os.LinkError{Op: "publish", Old: tmp, New: final, Err: fs.ErrExist}
	case !errors.Is(lookErr, fs.ErrNotExist):
		return &os.LinkError{Op: "publish", Old: tmp, New: final, Err: lookErr}
	}
	return os.Rename(tmp, final)
}
