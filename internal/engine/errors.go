package engine

import (
	"errors"
	"fmt"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// Errors carry what went wrong in a form the surface above can turn into an
// exit code, without the engine knowing anything about exit codes.
//
// The alternative is a switch on message text at the top of the program,
// where every failure mode nobody remembered to list quietly becomes a
// generic error - and CI, which can only tell situations apart by the code,
// stops being able to react.

// RecipeError is a request that is well formed but asks for something that
// makes no sense.
type RecipeError struct {
	// Detail is what is wrong. Because is why the rule exists and Remedy is
	// what to do instead - the other two of the three parts every refusal in
	// this tool has, kept apart so a report can carry them apart and a script
	// reading it does not have to take a sentence written for a person back to
	// pieces. Both may be empty, and then the whole refusal is in Detail and
	// reads exactly as it did before either field existed.
	Detail  string
	Because string
	Remedy  string

	// Setting is which setting the refusal is about, where it is about one.
	//
	// It exists so a window can put the message beside the box that caused it.
	// UX8 asks for that in as many words - near where the error came from, and
	// in a window that means beside the field rather than at the foot of the
	// screen. Measured 2026-08-11: the refusal about "how many" sat 748 px
	// below the field it named, under every other field, and that distance
	// grows with each field added rather than with the size of the window.
	//
	// The command line ignores it and prints as it always has, so this is
	// additive. Empty is normal and means the refusal is about the run rather
	// than about one box - those keep landing where they land today.
	//
	// It is spelled as the recipe key rather than as either surface's label,
	// because the recipe keys are the vocabulary both surfaces already share.
	// A third naming here is a third thing to keep in step.
	Setting string
}

func (e *RecipeError) Error() string {
	return e.InTheWordsOf(core.LastSettingSegment(e.Setting))
}

// InTheWordsOf is this refusal with the setting named the way one surface names
// it - see core.SettingSlot. Error is this with the recipe key.
//
// One sentence assembled from the parts, joined the way these refusals have
// always read: what is wrong, a dash, why the rule is there, a full stop, what
// to do instead. A refusal that has not been cut into parts yet carries all of
// it in Detail and comes out exactly as it did before, so the two states are
// not a half finished middle - they are the same sentence written in one piece
// or in three.
func (e *RecipeError) InTheWordsOf(name string) string {
	if name == "" {
		name = core.LastSettingSegment(e.Setting)
	}
	message := e.Detail
	if e.Because != "" {
		message += " - " + e.Because
	}
	if e.Remedy != "" {
		message += ". " + e.Remedy
	}
	return core.InTheWordsOf(message, name)
}

// The three parts a report keeps apart, answering the same names the format and
// preset refusals answer so that nothing above has to know this type.
//
// The fields are Because and Remedy rather than Why and Fix because a field
// cannot share a name with a method, and the method names are the ones already
// spoken here - UnknownPropertyError has answered to them since the recipe
// reader started reporting four parts.
func (e *RecipeError) What() string {
	return core.InTheWordsOf(e.Detail, core.LastSettingSegment(e.Setting))
}

func (e *RecipeError) Why() string {
	return core.InTheWordsOf(e.Because, core.LastSettingSegment(e.Setting))
}

func (e *RecipeError) Instead() string {
	return core.InTheWordsOf(e.Remedy, core.LastSettingSegment(e.Setting))
}

// AboutSetting lets a window place this message without knowing this type.
// The preset package answers the same question about its own refusals, and a
// window that type switched on both would grow a case for every error a screen
// can be shown.
func (e *RecipeError) AboutSetting() string { return e.Setting }

// The settings a refusal can point at. Recipe keys, and deliberately not the
// window's labels or the command line's flags - see RecipeError.Setting.
const (
	SettingID     = "id"
	SettingCount  = "count"
	SettingName   = "name"
	SettingOutDir = "output.dir"
	// The manifest is a name too, and it is checked by the same function as a
	// target file name - so without this both came back as "name". Measured on
	// the batch screen 2026-08-25: a manifest name the host cannot store marked
	// nothing, though output.manifest is a box on that screen.
	SettingOutputManifest = "output.manifest"
	// Two settings nothing refuses today, named here because a surface has to
	// name every box it draws. A screen that could only name the settings a
	// refusal already points at is a screen where marking a field is a
	// property of what somebody remembered to wire, which is the state this
	// window was in on 2026-08-12 - five fields could carry a refusal and
	// three could not. docs/UX.md section 7.0.
	SettingFormat = "format"
	SettingSeed   = "seed"
	SettingLabel  = "label"
)

// atTarget gives a refusal from below the position of the target it happened
// in, so a screen showing twenty of them can mark the right box.
//
// Measured on 2026-08-25, on the batch screen: a batch asking for a 10 B PDF
// and a batch with a name the host cannot store both refused the run and
// marked nothing at all, because the refusal knew it was about "size" or
// "name" and the screen registers its boxes as targets[2].size. The same two
// refusals on the single batch screen marked their boxes, because that screen
// registers the bare key - so this was not a broken refusal but two
// vocabularies meeting.
//
// It does not switch on types, because everything that knows the setting it is
// about answers the same interface a window asks.
//
// A branch putting a format's own setting under properties, the way
// internal/recipe addresses the ones it can see, was written here and taken
// out the same day. The mutation runner said NOT CAUGHT, and it was right:
// nothing can see the difference. On the batch screen a property is refused by
// the recipe reader, which addresses it before this is reached, so the branch
// never ran there. On the single batch screen the address has its position
// dropped again and the last segment is the same either way. Thirty nine
// refusals came out identical with the branch and without it. A rule that is
// correct for a path nobody walks is the shape this project has taken out four
// times already - if such a path arrives, its guard will ask for the branch
// back.
//
// A refusal that does not know its setting is passed through untouched.
// Inventing a position for it would put a message about the whole run under
// one batch of twenty, which is worse than leaving it at the foot of the form
// where a message about the run belongs.
func atTarget(position int, err error) error {
	var about interface{ AboutSetting() string }
	if !errors.As(err, &about) || about.AboutSetting() == "" {
		return err
	}
	setting := about.AboutSetting()
	// Already placed - worded here with a position of its own, or carried up
	// through more than one of these.
	if core.AddressNamesATarget(setting) {
		return err
	}
	return &addressedError{err: err, at: core.TargetAddress(position, setting)}
}

// addressedError is a refusal with the position of the target added.
//
// It answers AboutSetting itself and hands everything else on, which is what
// makes it invisible to every other reader: the exit code still comes from the
// error underneath through errors.As, and so does the wording a field shows in
// its own words. The message is untouched, so the command line prints what it
// always printed.
type addressedError struct {
	err error
	at  string
}

func (a *addressedError) Error() string        { return a.err.Error() }
func (a *addressedError) Unwrap() error        { return a.err }
func (a *addressedError) AboutSetting() string { return a.at }

// SpaceError is refusing to start because the disk cannot hold the result.
//
// Free space is checked before the first byte rather than discovered at file
// five thousand of ten thousand. Filling up a working disk is the most
// expensive thing this tool can do to someone.
type SpaceError struct {
	Needed    int64
	Available int64
	Path      string
}

func (e *SpaceError) Error() string {
	return fmt.Sprintf(
		"this run needs %d B and %s has %d B free - nothing was written. Ask for fewer files or a smaller size, or write to another disk by changing the output directory",
		e.Needed, e.Path, e.Available)
}

// CollisionError is refusing to write over something that is already there.
//
// This tool runs in directories that belong to the user. Overwriting without
// a word is the one mistake that destroys work rather than wasting time.
type CollisionError struct {
	Path string

	// Manifest tells the two apart. The remedy differs, and so does what is
	// lost by writing anyway: a data file costs the bytes, and the manifest
	// costs the record of every file an earlier run wrote - after which
	// cleanup cannot see them and nothing can.
	Manifest bool
}

func (e *CollisionError) Error() string {
	if e.Manifest {
		return fmt.Sprintf(
			"%s already exists and this run will not write over it. It is the only record of what an earlier run wrote, so replacing it would leave those files with nothing to remove them by. Generate into an empty directory, or move the old manifest aside",
			e.Path)
	}
	return fmt.Sprintf(
		"%s already exists and this run will not write over it. Generate into an empty directory, or remove the file first",
		e.Path)
}
