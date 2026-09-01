// Package archive is what a container format takes and how it reads it.
//
// ZIP and TAR.GZ are different files with one vocabulary. Both hold real
// generated files of other formats, both take how many either as a contains
// list or as the entries, entry_format and entry_size properties, and both
// refuse the same things for the same reasons.
//
// Until 2026-09-01 they said all of that twice. Measured that day across the
// two packages: six constants with identical values, two identical reader
// helpers, an identical mustSize, the entry ceiling refusal written out twice
// down to its wording, and a groupsFor whose code was identical to the byte -
// every difference between the two copies was a comment, and one of those
// comments pointed at the other copy. Nothing but that comment held them
// together.
//
// A third container is already named and measured (7Z, docs/MVP-FORMATS.md
// section 2.5) and M1 lists more behind it, so the copy was going to be taken a
// third time and a fourth.
//
// This is the third family package, after imagelabel for the pictures and opc
// for the Office formats, and it draws its line where they draw theirs: what
// every container shares lives here, what one of them measured for itself stays
// with it. The padding channel is the example worth naming. Both formats cap it
// at 65 535 bytes and that agreement is a coincidence - in ZIP it is the width
// of the field carrying the length, in TAR.GZ it is where 7-Zip stops reading a
// gzip comment. Merging them would put one number where there are two facts.
package archive

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// The keys a recipe writes. Public names under rule 10, spelled once here
// rather than quoted at each place that reads one.
const (
	Entries     = "entries"
	EntryFormat = "entry_format"
	EntrySize   = "entry_size"
	Password    = "password"
	Encryption  = "encryption"
	EntryMode   = "entry_mode"
	EntryOwner  = "entry_owner"
)

// The owners an entry can be recorded as.
const (
	// OwnerUnset records no owner at all, which is what a tar written here has
	// always carried and therefore what the default has to stay - a different
	// default would move the bytes of every archive, which is untouchable rule
	// 3.
	OwnerUnset = "unset"
	OwnerRoot  = "root"
	OwnerUser  = "user"
)

// The encryption methods, spelled the way a recipe writes them.
const (
	NoEncryption = "none"
	ZipCrypto    = "zipcrypto"
	AES128       = "aes-128"
	AES192       = "aes-192"
	AES256       = "aes-256"
)

const (
	// DefaultFormat is what an archive holds when nothing says otherwise. It
	// is exported because a container works its own minimum size out by asking
	// this format for an empty file.
	DefaultFormat = "txt"

	defaultEntries = 1

	// maxEntries is the ceiling on how many files an archive holds. It bounds
	// the declaration below and the reading of both doors into it, so the two
	// cannot drift - which they did, in two packages, until 2026-08-26.
	maxEntries = 10000

	// defaultSizeText is the default size of a file inside, written the way
	// somebody writes it. The number is derived from it rather than spelled a
	// second time.
	//
	// They used to be two constants and they had drifted: the declaration said
	// 8kb and the generator used 4096, so tfg formats printed one answer and
	// generating without the setting gave the other. Nothing could see it,
	// because the declaration is only read for printing. The declaration is
	// the half consumers believe - AR9 makes the registry the place a consumer
	// asks - so the generator was moved to it rather than the other way round.
	defaultSizeText = "8kb"
)

// defaultSize is defaultSizeText in bytes. Package variables are initialised
// before any init runs, so a registration can rely on it.
var defaultSize = mustSize(defaultSizeText)

func mustSize(s string) int64 {
	n, err := core.ParseSize(s)
	if err != nil {
		panic(fmt.Sprintf("archive: the default entry size %q is not a size this build can parse: %v", s, err))
	}
	return n
}

// axes is the declaration of every setting a container may take, by key.
//
// One copy, so two containers cannot offer the same setting with different
// bounds, a different default or a different sentence beside it. Which is not
// hypothetical tidiness: the two that exist today were identical only because
// somebody kept them so by hand, and the comment saying so was the whole
// mechanism.
var axes = map[string]format.Property{
	Entries: {
		Name: Entries, Kind: format.PropertyInt,
		Min: 0, Max: maxEntries,
		Default: strconv.Itoa(defaultEntries),
		Detail:  "How many files the archive holds. Use contains instead when the files are not all alike.",
	},
	EntryFormat: {
		Name: EntryFormat, Kind: format.PropertyText,
		Shape: "the id of a format, as tfg formats lists them",
		// Not a choice, because the allowed values are whatever this build
		// registered, and a list frozen here would drift away from the
		// registry the moment a format is added.
		Default: DefaultFormat,
		Detail:  "The format of the files inside. Run tfg formats to see what this build supports.",
	},
	EntrySize: {
		Name: EntrySize, Kind: format.PropertySize,
		Default: defaultSizeText,
		Detail:  "How big each file inside is.",
	},
	Password: {
		Name: Password, Kind: format.PropertyText,
		Shape: "the password, in plain text",
		// No default, and that is the point. A box somebody types in arrives
		// empty from a window, so leaving it alone is how "no password" is
		// said - see the pair rule in readLock.
		Detail: "The password the archive is locked with. It is written into the manifest as you typed it, " +
			"because a test that cannot open the file cannot check anything.",
	},
	EntryMode: {
		Name: EntryMode, Kind: format.PropertyChoice,
		// Written the way chmod takes them, and sorted, because a closed set
		// has one order on every surface. The interesting ones for a test are
		// at the ends: 000 is a file nothing can read, 444 is read only, and
		// 666 and 777 are what a scanner should have something to say about.
		Choices: []string{"000", "400", "444", "600", "644", "664", "666", "700", "755", "777"},
		Default: "644",
		Detail: "The permissions recorded for each file inside. It is what the archive says, " +
			"not what the file gets - that depends on who unpacks it and how.",
	},
	EntryOwner: {
		Name: EntryOwner, Kind: format.PropertyChoice,
		Choices: []string{OwnerRoot, OwnerUnset, OwnerUser},
		Default: OwnerUnset,
		Detail: "Who each file inside belongs to. Leave it unset and the archive names nobody, " +
			"which is what most archives written by a build carry.",
	},
	Encryption: {
		Name: Encryption, Kind: format.PropertyChoice,
		// Sorted, because a closed set has one order on every surface.
		Choices: []string{AES128, AES192, AES256, NoEncryption, ZipCrypto},
		Default: NoEncryption,
		Detail: "How the archive is locked. ZipCrypto is the old scheme every reader opens and nothing modern trusts. " +
			"AES is the WinZip scheme, and some readers cannot open it at all.",
	},
}

// Names is every container setting this build declares, in a stable order.
//
// It exists for the guard that compares what a container offers against what
// this package says the setting is. Given the list, that guard covers an axis
// added tomorrow without a line being changed in it - which is the same reason
// the entries ceiling is read out of the registry rather than repeated in a
// test.
func Names() []string {
	out := make([]string, 0, len(axes))
	for n := range axes {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Axes is the declarations for the settings named, in the order given.
//
// A container lists what it takes rather than receiving all of it, because the
// settings a container understands are not the same set for every container -
// tar carries an owner and a mode where zip carries neither. What a format does
// not name here it does not declare, so a window never draws a field for it and
// a recipe naming it is a typo the registry refuses.
//
// An unknown name panics rather than being skipped. This is called from init,
// the caller is a programmer and not a user, and a silently dropped axis is a
// setting that vanishes from both surfaces with nothing said.
func Axes(names ...string) []format.Property {
	out := make([]format.Property, 0, len(names))
	for _, n := range names {
		p, ok := axes[n]
		if !ok {
			panic(fmt.Sprintf("archive: %q is not a container setting this build declares", n))
		}
		out = append(out, p)
	}
	return out
}

// Groups works out what the archive holds.
//
// There are two ways to say it. "contains" in a recipe is the general one and
// takes a list of groups of different formats. The entries, entry_format and
// entry_size properties are the flag sized one, reachable through --set, and
// they say the same thing for a single format.
//
// Both at once is refused rather than one of them being picked. Picking would
// produce an archive holding something other than what the recipe says, and the
// recipe is the thing somebody reads in a pull request. Same rule as a boundary
// declared beside a size.
//
// id is the container asking, and it is the whole of what used to differ
// between the two copies of this: it names the format in every refusal and it
// is what an archive may not hold, since an archive inside an archive needs a
// depth limit that does not exist yet.
func Groups(id string, r format.Request) ([]format.Content, error) {
	var stated []string
	for _, key := range []string{Entries, EntryFormat, EntrySize} {
		if _, ok := r.Properties[key]; ok {
			stated = append(stated, key)
		}
	}

	// Not len() > 0. An empty contains says "an archive holding nothing",
	// which is a legitimate thing to ask for, and it is a different statement
	// from saying nothing at all.
	if r.Contains != nil {
		return fromContains(id, r, stated)
	}

	if r.SizeFromContents {
		// Only reachable if a caller sets the flag without contents. Saying so
		// beats producing an empty archive and calling it the answer.
		return nil, fmt.Errorf("%s: the size was left to the contents and there are none", id)
	}

	entries, err := intProperty(id, r.Properties, Entries, defaultEntries, 0, maxEntries)
	if err != nil {
		return nil, err
	}
	// Sizes in properties use the same syntax as --size. Anything else would
	// mean entry_size=200kb failing while size=200kb works, which nobody would
	// predict.
	entrySize, err := sizeProperty(id, r.Properties, EntrySize, defaultSize)
	if err != nil {
		return nil, err
	}
	entryFmt := DefaultFormat
	if v, ok := r.Properties[EntryFormat]; ok && v != "" {
		entryFmt = v
	}
	if entryFmt == id {
		return nil, &format.NestingUnsupportedError{Format: id}
	}
	return []format.Content{{Format: entryFmt, Count: entries, Bytes: entrySize}}, nil
}

// fromContains reads the general door, the one a recipe writes.
//
// Split out of Groups so the branching stays under the ceiling the shape gates
// hold, and because it is one subject: everything here is about a contains list
// and nothing about it reads a property.
func fromContains(id string, r format.Request, stated []string) ([]format.Content, error) {
	if len(stated) > 0 {
		return nil, &format.ContentsConflictError{Format: id, Keys: stated}
	}
	asked := 0
	for _, g := range r.Contains {
		if g.Format == id {
			return nil, &format.NestingUnsupportedError{Format: id}
		}
		asked += g.Count
	}
	// The same ceiling the entries property has, on the other way of saying the
	// same thing. Until 2026-08-26 this path had none: a recipe asking for
	// fifty thousand entries through contains validated clean and dry ran
	// clean, while entries=50000 was refused with "must be between 0 and
	// 10000". One quantity, two doors, two answers - and the door with no
	// ceiling planned a format.Plan for every child.
	if asked > maxEntries {
		// Spelled as the refusal the entries property already produces, down
		// to the exit code: a plain error here would have landed on 1 while
		// the same request through entries lands on 4, which is the same
		// disagreement one level further down.
		return nil, tooMany(id, "contains", asked)
	}
	return r.Contains, nil
}

// tooMany is the refusal both doors give, written once.
//
// TestBothWaysOfAskingForEntriesShareOneCeiling compares the reason the two
// produce, and before this it compared two sentences somebody had typed twice.
func tooMany(id, key string, asked int) *format.PropertyValueError {
	return &format.PropertyValueError{
		Format: id,
		Key:    key,
		Value:  strconv.Itoa(asked),
		Reason: fmt.Sprintf("it takes a whole number from 0 to %d", maxEntries),
		Remedy: fmt.Sprintf("Ask for %d entries or fewer.", maxEntries),
	}
}

// sizeProperty reads a byte count written the way --size accepts it.
func sizeProperty(id string, props map[string]string, key string, fallback int64) (int64, error) {
	raw, ok := props[key]
	if !ok || raw == "" {
		return fallback, nil
	}
	n, err := core.ParseSize(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %s: %w", id, key, err)
	}
	return n, nil
}

func intProperty(id string, props map[string]string, key string, fallback, min, max int) (int, error) {
	raw, ok := props[key]
	if !ok || raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: %s must be a whole number, got %q", id, key, raw)
	}
	if n < min || n > max {
		return 0, fmt.Errorf("%s: %s must be between %d and %d, got %d", id, key, min, max, n)
	}
	return n, nil
}
