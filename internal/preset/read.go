package preset

import (
	"errors"
	"fmt"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// Read is a recipe file as a run sees it: the recipe, and the preset it was
// built on when it was built on one.
//
// This is the door every surface reads a file through. The recipe package
// cannot expand a preset - the layer rule lets this package import that one
// and not the other way round - so a file that says extends: preset:<id> is
// read in two steps with the expansion between them, and this is where the
// steps meet. A guard holds the command line and the window to reading files
// here rather than through recipe.Parse, because Parse on such a file refuses
// it, loudly, and the refusal is not one a person can act on.
type Read struct {
	Recipe *recipe.Recipe
	// Expansion is the preset the file built on, settled on what the file's
	// with section gave it. Nil when the file stands alone - and that is how
	// a caller tells the two apart, because the manifest records a preset
	// only when there was one.
	Expansion *Expansion
}

// Notes is what the run has to say out loud about the preset's parameters
// nobody gave - see Expansion.Notes. Nothing, for a file that stands alone.
func (r *Read) Notes() []string {
	if r.Expansion == nil {
		return nil
	}
	return r.Expansion.Notes()
}

// ReadRecipe reads a file, expanding the preset it builds on if it builds
// on one.
//
// A problem with the preset side - a preset this build does not have, a
// parameter it does not declare, a value it refuses, a set it cannot build -
// is reported the way a problem with the file is reported, with the address
// of the line it came from: the extends key, or with.<name>. It is the file
// that is wrong in every one of those cases, and a screen has a box for each.
func ReadRecipe(src []byte, name string) (*Read, error) {
	ext, err := recipe.ExtensionOf(src, name)
	if err != nil {
		return nil, err
	}
	if ext == nil {
		rec, err := recipe.Parse(src, name)
		if err != nil {
			return nil, err
		}
		return &Read{Recipe: rec}, nil
	}

	expanded, err := Expand(ext.Preset, Args(ext.With))
	if err != nil {
		return nil, aboutTheFile(name, ext.Preset, err)
	}
	rec, err := recipe.ParseExtending(src, name, expanded.Source)
	if err != nil {
		return nil, err
	}
	return &Read{Recipe: rec, Expansion: expanded}, nil
}

// aboutTheFile turns a refusal from the preset side into a refusal about the
// recipe, addressed to the line it came from.
//
// The preset package words its refusals for the command line's --preset
// flags, where "the preset size-boundaries does not have a parameter called
// spreed" stands on its own. In a file the same mistake is a line under with,
// and the refusal has to say so and carry that address - the window marks a
// box by it, and the command line's list of problems names it.
func aboutTheFile(name, id string, err error) error {
	problem := recipe.Problem{At: recipe.KeyExtends, What: err.Error()}

	var unknownPreset *UnknownPresetError
	var unknownParameter *UnknownParameterError
	var unknownFormat *format.UnknownFormatError
	var value *format.PropertyValueError
	var about interface{ AboutSetting() string }
	var three interface {
		What() string
		Why() string
		Instead() string
	}
	switch {
	case errors.As(err, &unknownPreset):
		problem.What = fmt.Sprintf("extends names preset:%s, which this build does not have", unknownPreset.ID)
		problem.Why = "a recipe can build only on a preset the tool it runs on knows"
		problem.Fix = "run \"tfg preset list\" for the ids this build has"
		if len(unknownPreset.Known) > 0 {
			problem.Fix = fmt.Sprintf("this build has: %s", strings.Join(unknownPreset.Known, ", "))
		}
	case errors.As(err, &unknownParameter):
		problem.At = recipe.KeyWith + "." + unknownParameter.Name
		problem.What = fmt.Sprintf("with names %s, which the preset %s does not take", unknownParameter.Name, unknownParameter.Preset)
		problem.Why = "with fills the parameters the preset declares, and this is not one of them"
		problem.Fix = "remove the line"
		if len(unknownParameter.Known) > 0 {
			problem.Fix = fmt.Sprintf("the preset takes: %s", strings.Join(unknownParameter.Known, ", "))
		}
	case errors.As(err, &value):
		// A value the parameter refuses. Named by its line in the file
		// rather than by the preset, which is how the command line's
		// --preset path words the same refusal - there the flag is the line.
		problem.At = recipe.KeyWith + "." + value.Key
		problem.What = fmt.Sprintf("with.%s cannot be %q", value.Key, value.Value)
		problem.Why = value.Why()
		problem.Fix = value.Instead()
	case errors.As(err, &unknownFormat):
		// The one global a preset reads. The format registry refuses it in
		// its own words, which name no line, so the line is named here.
		problem.At = recipe.KeyWith + ".format"
		problem.What = fmt.Sprintf("with.format names %q, which this build does not have", unknownFormat.ID)
		problem.Why = "the preset gives every file it builds this format, so it has to be one the build can write"
		problem.Fix = "run \"tfg formats\" for the list"
		if len(unknownFormat.Known) > 0 {
			problem.Fix = fmt.Sprintf("this build has: %s", strings.Join(unknownFormat.Known, ", "))
		}
	case errors.As(err, &three) && errors.As(err, &about) && about.AboutSetting() != "":
		// A set the preset cannot build from these values. It comes in three
		// parts already and names the parameter, so the file's address is
		// the parameter's line. The hint ends in a full stop of its own and
		// the problem adds one, so the stop comes off here.
		problem.At = recipe.KeyWith + "." + about.AboutSetting()
		problem.What = three.What()
		problem.Why = strings.TrimSuffix(three.Why(), ".")
		problem.Fix = strings.TrimSuffix(three.Instead(), ".")
	default:
		// A refusal of a shape this does not know still arrives whole, at
		// the extends line, with the two parts it lacks filled in rather than
		// printed as a bare dash and a bare stop.
		problem.Why = "the preset refused what the recipe gave it"
		problem.Fix = "run \"tfg preset show " + id + "\" for what it takes"
	}
	return &recipe.ValidationError{Name: name, Problems: []recipe.Problem{problem}}
}
