package archive

import (
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// What an archive records about the files it holds, beyond their bytes.
//
// None of this changes the size of anything, and that is what makes it cheap:
// a USTAR header is 512 bytes whatever the mode says and whoever the owner is,
// because every field in it is fixed width. So the arithmetic the container
// design rests on is untouched, and these settings cost a person nothing to
// use.
//
// They are worth having because the interesting cases are the ones nobody
// creates by accident. A file recorded as 000 is one nothing can read after
// unpacking, 777 is one a scanner should have something to say about, and an
// archive claiming root owns everything is what a careless extractor turns
// into a privilege problem. Producing one of those by hand takes a machine
// with the right permissions. Producing one here takes a flag.

// Ownership is what an entry says about its permissions and its owner.
//
// The zero value is what this tool has always written, which is the default
// for the same reason the default of every other setting is what it is: a
// different one would move the bytes of every archive already out there.
type Ownership struct {
	// Mode is the permission bits, already parsed from the three digits a
	// person writes.
	Mode int64
	// Uid and Gid are the numeric owner, and Uname and Gname are the names.
	// All four are left at nought and empty for the unset owner.
	Uid, Gid     int
	Uname, Gname string
	// Stated is what the settings said, in the words they were written in, for
	// the manifest to record.
	//
	// Carried rather than worked out again at the far end. A mode of 000 turns
	// back into "0" rather than "000" through the obvious arithmetic, and an
	// owner would have to be recovered from Uname, which is empty for two
	// different reasons. A fact the writer already has is cheaper to pass along
	// than to reconstruct - ARCHITECTURE.md 5.
	Stated Stated
}

// Stated is the pair of settings as a person wrote them.
type Stated struct {
	// Mode is the three octal digits, so 644 stays 644.
	Mode string
	// Owner is the declared word: root, user or unset.
	Owner string
}

// ReadOwnership reads the two settings that say what an entry records about
// itself.
//
// The mode arrives as three octal digits because that is how a person writes
// one and how chmod takes one. Parsing it here rather than declaring an int
// keeps 644 meaning what it looks like: as a decimal it is 1204 in octal, and
// a setting where the number a person types is not the number the file gets is
// the kind of difference nobody predicts.
func ReadOwnership(id string, props map[string]string) (Ownership, error) {
	own := Ownership{Mode: 0o644, Stated: Stated{Mode: "644", Owner: OwnerUnset}}

	if raw := props[EntryMode]; raw != "" {
		mode, err := strconv.ParseInt(raw, 8, 32)
		if err != nil {
			// Unreachable through the registry, which checks the value against
			// the declared set before a generator sees it. Here because this
			// function is also callable directly, and a silent 0 would be a
			// file nothing can read.
			return Ownership{}, &format.PropertyValueError{
				Format: id, Key: EntryMode, Value: raw,
				Reason: "it is not a permission written as octal digits",
				Remedy: "Write it the way chmod takes it, so 644 or 755.",
			}
		}
		own.Mode = mode
		own.Stated.Mode = raw
	}

	switch raw := props[EntryOwner]; raw {
	case OwnerRoot:
		own.Stated.Owner = OwnerRoot
		own.Uname, own.Gname = "root", "root"
	case OwnerUser:
		// The first ordinary account on a Linux system, which is what somebody
		// unpacking a fixture on their own machine most likely is.
		own.Stated.Owner = OwnerUser
		own.Uid, own.Gid = 1000, 1000
		own.Uname, own.Gname = "user", "user"
	case "", OwnerUnset:
		// Nothing to record, which is what this format wrote before the setting
		// existed. An absent key and the declared word for absent mean the same
		// archive.
	default:
		// There was no default branch here, and the value fell through to an
		// archive owned by nobody - the same bytes as unset, reported as
		// success. Measured 2026-09-02: entry_owner=USER and entry_owner=ROOT
		// each produced a file byte for byte identical to the default one, with
		// exit 0 and not a word about it, which is rule 6 broken outright. The
		// registry now stops a misspelling before it arrives, so this branch is
		// for the other door: this function is callable directly.
		return Ownership{}, &format.PropertyValueError{
			Format: id, Key: EntryOwner, Value: raw,
			// From the declaration, for the reason the same branch in
			// compression.go gives: a hand written list is a copy of the
			// registry's sentence with nothing comparing the two - O171.
			// A word added to the declaration and not to this switch lands
			// here, and the guard that puts every declared word through this
			// function is what stops that arriving as an empty reason.
			Reason: axes[EntryOwner].Allows(raw),
			Remedy: "Write it the way the setting is declared, so root or user, " +
				"or leave the line out and the entries carry no owner.",
		}
	}
	return own, nil
}
