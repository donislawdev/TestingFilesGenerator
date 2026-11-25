package window

import (
	"github.com/donislawdev/TestingFilesGenerator/internal/commandline"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// The Copy command of the Several batches screen, which copies two commands
// rather than one: the recipe the screen would run, saved as a file by the
// shell, and tfg generate on that file. The command line takes a recipe as a
// file and has no flags for most of what this screen holds - several batches,
// the files inside an archive, a group and a purpose - so the recipe is the
// one way to say it. The owner's choice of 2026-11-25, docs, the analysis of
// that day, section 5.

// offerRecipeCopy is the button of the Several batches screen, at the right
// end of its bar like on every screen run by a runner.
func offerRecipeCopy(r *Recipe, host Host) *parts.Button {
	return offerCopy(r.runner, host, func() (commandline.Command, error) { return recipeCommand(r) })
}

// recipeCommand is the recipe of the screen and the command that runs it.
//
// Settled first, so a form Generate refuses is refused here under the same
// boxes. The folder is written whole, because the recipe is read in whatever
// folder the terminal is in and a relative one would name another place than
// the window means. Seed, manifest name and the label switch travel in the
// recipe. Taking a free name and the memory to plan with are not recipe keys,
// so they are flags on the second command.
func recipeCommand(r *Recipe) (commandline.Command, error) {
	_, opt, err := r.settle()
	if err != nil {
		return nil, err
	}
	out, err := outWritten(opt.OutDir)
	if err != nil {
		return nil, err
	}
	doc := documentOf(r)
	doc.OutDir = out
	src, err := recipe.Compose(doc)
	if err != nil {
		return nil, err
	}
	hash, err := recipe.Hash(src)
	if err != nil {
		return nil, err
	}
	file := commandline.RecipeFile(hash)
	then := commandline.New(commandline.Generate).
		Argument(file, "").
		Switch(commandline.NumberIfTaken, opt.NumberIfTaken).
		Flag(commandline.Memory, memoryWritten(r.kept()), "")
	return commandline.FileThen(file, string(src), then), nil
}
