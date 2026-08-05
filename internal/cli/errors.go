// Part of package cli. See cli.go.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/donislawdev/TestingFilesGenerator/internal/audit"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

func describeError(err error) string {
	if err == nil {
		return ""
	}

	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return err.Error()
	}

	// Replace it wherever it sits rather than only at the end. A message built
	// by wrapping prints the innermost error last, but nothing guarantees that,
	// and a leak of one sentence is the whole defect.
	full := err.Error()
	if osText := errno.Error(); osText != "" && strings.Contains(full, osText) {
		return strings.ReplaceAll(full, osText, systemReason(errno))
	}
	return full
}

// systemReason is our own English sentence for a system error, with the number
// beside it. The number is the part that survives translation.
func systemReason(errno syscall.Errno) string {
	reason := "the system refused it"
	switch {
	case errors.Is(errno, fs.ErrNotExist):
		reason = "there is nothing at that path"
	case errors.Is(errno, fs.ErrPermission):
		reason = "the system refused permission"
	case errors.Is(errno, fs.ErrExist):
		reason = "something is already there"
	}
	return fmt.Sprintf("%s (system error %d)", reason, uintptr(errno))
}

// mustBeFile turns the likeliest mistake into a sentence of ours.
//
// Every command below takes a file and the neighbouring ones take directories,
// so pointing one at a directory is the mistake somebody actually makes. Left
// alone it surfaces as whatever the system says about reading a directory,
// which on Windows is "Incorrect function." and says nothing to anybody.
func mustBeFile(path, kind, command string) error {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		// Anything else is left to the read that follows, so one fault gives
		// one message.
		return nil
	}
	return fmt.Errorf("%s is a directory and %s reads a file. Name the one inside it, for example: tfg %s %s",
		path, command, command, filepath.Join(path, kind))
}

// Split into three by what the error is about rather than left as one chain.
// The chain reached the ceiling on function length, and cutting it where the
// subject changes is the cut that costs nothing to read: a caller still asks
// one question and gets one number.
func classify(err error) int {
	if code, ok := classifyRequest(err); ok {
		return code
	}
	if code, ok := classifyReading(err); ok {
		return code
	}
	if errors.Is(err, context.Canceled) {
		return ExitInterrupted
	}
	// A deadline running out is the same ending as a signal that says time is
	// up, and the frozen table tells that apart from somebody cancelling.
	// Nothing in the command line sets a deadline today - it is reachable only
	// through a caller that does, which the window will be - so this is here
	// before it is needed rather than after it has been reported as the tool
	// crashing. Without it the ending falls through to RUNTIME.
	if errors.Is(err, context.DeadlineExceeded) {
		return ExitTerminated
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return ExitIO
	}
	return ExitRuntime
}

// classifyRequest covers what was asked for: the recipe, the preset and what
// the format can deliver.
func classifyRequest(err error) (int, bool) {
	var recipeErr *engine.RecipeError
	if errors.As(err, &recipeErr) {
		return ExitRecipe, true
	}
	var invalid *recipe.ValidationError
	if errors.As(err, &invalid) {
		return ExitRecipe, true
	}
	var syntax *recipe.SyntaxError
	if errors.As(err, &syntax) {
		return ExitRecipe, true
	}
	// Two parts of one recipe saying different things about the same archive
	// is a recipe problem, like a boundary stated beside a size.
	var conflict *format.ContentsConflictError
	if errors.As(err, &conflict) {
		return ExitRecipe, true
	}
	var badProp *format.UnknownPropertyError
	if errors.As(err, &badProp) {
		return ExitUsage, true
	}
	// A preset id nobody registered is a typo in the invocation, which is what
	// USAGE means. Deliberately not the code an unknown format gets: a format
	// can arrive from inside a recipe, where it describes a request rather than
	// something somebody just typed, and a preset id only ever comes from the
	// command line.
	var unknownPreset *preset.UnknownPresetError
	if errors.As(err, &unknownPreset) {
		return ExitUsage, true
	}
	if code, ok := classifyFormat(err); ok {
		return code, true
	}
	return 0, false
}

// classifyFormat is the one class the frozen table gives its own code: the
// request is well formed and no format here can deliver it.
func classifyFormat(err error) (int, bool) {
	var belowMin *format.BelowMinimumError
	if errors.As(err, &belowMin) {
		return ExitFormat, true
	}
	var unknown *format.UnknownFormatError
	if errors.As(err, &unknown) {
		return ExitFormat, true
	}
	// A format that holds nothing being asked to hold something, and a
	// container asked to nest, are both "this format cannot do that" - the
	// same class as a size below the minimum.
	var notContainer *format.NotAContainerError
	if errors.As(err, &notContainer) {
		return ExitFormat, true
	}
	var nesting *format.NestingUnsupportedError
	if errors.As(err, &nesting) {
		return ExitFormat, true
	}
	// A value outside what the format declares is a request the format cannot
	// deliver, which is what FORMAT means - the same class as a size below the
	// minimum. It used to fall through to RUNTIME, so "--set width=abc" told
	// CI this program had a bug rather than that the value was wrong.
	var badValue *format.PropertyValueError
	if errors.As(err, &badValue) {
		return ExitFormat, true
	}
	// A set the build cannot complete is a request no format here can deliver -
	// the same class as a size below the minimum, and usually literally that,
	// since it is the floor of a format that puts the smallest step out of
	// reach. Refusing the whole set rather than the part that fits is PR7: the
	// files nearest the limit are the ones the run was about.
	var impossible *preset.ImpossibleError
	if errors.As(err, &impossible) {
		return ExitFormat, true
	}
	return 0, false
}

// classifyReading covers the disk and what came off it.
func classifyReading(err error) (int, bool) {
	var spaceErr *engine.SpaceError
	if errors.As(err, &spaceErr) {
		return ExitSpace, true
	}
	var collision *engine.CollisionError
	if errors.As(err, &collision) {
		return ExitIO, true
	}
	// A manifest we cannot read is a reading failure, not a bug in the tool.
	// Falling through to RUNTIME would tell CI to file a report against us for
	// a file somebody handed in.
	var schema *manifest.SchemaError
	if errors.As(err, &schema) {
		return ExitIO, true
	}
	// Same class, one step earlier: a manifest too large to read is a file we
	// will not take, not a fault of ours.
	var manifestTooLarge *manifest.TooLargeError
	if errors.As(err, &manifestTooLarge) {
		return ExitIO, true
	}
	// And the same class one step later: a manifest whose entries leave the
	// directory once the links are followed. The text of the path passed, the
	// filesystem did not.
	var escape *audit.EscapeError
	if errors.As(err, &escape) {
		return ExitIO, true
	}
	return 0, false
}

// propertyFlag collects repeated --set key=value pairs.
