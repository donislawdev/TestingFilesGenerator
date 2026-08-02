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

	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
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

func classify(err error) int {
	var recipeErr *engine.RecipeError
	if errors.As(err, &recipeErr) {
		return ExitRecipe
	}
	var invalid *recipe.ValidationError
	if errors.As(err, &invalid) {
		return ExitRecipe
	}
	var syntax *recipe.SyntaxError
	if errors.As(err, &syntax) {
		return ExitRecipe
	}
	var spaceErr *engine.SpaceError
	if errors.As(err, &spaceErr) {
		return ExitSpace
	}
	var collision *engine.CollisionError
	if errors.As(err, &collision) {
		return ExitIO
	}
	var belowMin *format.BelowMinimumError
	if errors.As(err, &belowMin) {
		return ExitFormat
	}
	var unknown *format.UnknownFormatError
	if errors.As(err, &unknown) {
		return ExitFormat
	}
	// A format that holds nothing being asked to hold something, and a
	// container asked to nest, are both "this format cannot do that" - the
	// same class as a size below the minimum.
	var notContainer *format.NotAContainerError
	if errors.As(err, &notContainer) {
		return ExitFormat
	}
	var nesting *format.NestingUnsupportedError
	if errors.As(err, &nesting) {
		return ExitFormat
	}
	// Two parts of one recipe saying different things about the same archive
	// is a recipe problem, like a boundary stated beside a size.
	var conflict *format.ContentsConflictError
	if errors.As(err, &conflict) {
		return ExitRecipe
	}
	var badProp *format.UnknownPropertyError
	if errors.As(err, &badProp) {
		return ExitUsage
	}
	// A manifest we cannot read is a reading failure, not a bug in the tool.
	// Falling through to RUNTIME would tell CI to file a report against us for
	// a file somebody handed in.
	var schema *manifest.SchemaError
	if errors.As(err, &schema) {
		return ExitIO
	}
	if errors.Is(err, context.Canceled) {
		return ExitInterrupted
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return ExitIO
	}
	return ExitRuntime
}

// propertyFlag collects repeated --set key=value pairs.
