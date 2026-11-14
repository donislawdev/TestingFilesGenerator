// Part of package engine. See engine.go.
package engine

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// A run into a directory that already holds one - Options.NumberIfTaken.
//
// Without it a second run into a directory is refused, because the manifest of
// the first is the only record able to remove its files (see preflight). That
// refusal is right and it stays the answer by default. What it cost was the
// window: somebody making twenty five files in twenty five settings had to
// choose a new directory twenty five times (docs/RUN-FOLDERS-2026-11-14.md).
//
// So a run told to may take a number instead. One number for the whole run,
// its manifest included, and the smallest at which every name it would write
// is free - the owner's decision of 2026-11-14, section 11.5 of that document.
//
// Counting from the manifest, rather than numbering only the names that
// collide, is the point of the rule and not a shortcut. A name can be free on
// the disk and still be in an earlier run's record: somebody deleted that run's
// files and kept its manifest. A run writing the same bytes under that name
// would hand its file to the earlier run's cleanup, which removes whatever
// matches its record (audit.look). A manifest lying in the directory holds its
// number, so its names cannot come back while it is there - and nothing has to
// read it to know that. Reading every record instead was weighed and turned
// down in section 11.1: this tool writes JSON files, and a JSON document that
// is not a manifest is read to its end before it is refused.
//
// The bytes do not move. A file's seed comes from the run's seed, its target's
// id and its place in the target (core.FileSeed), its label carries no name
// (format.Request.LabelSized), and a generator is never told the name. A
// numbered file is the file an empty directory would have got, under another
// name.

// numberIfTaken gives this run its number when it was asked to, renames the
// plan's files to carry it, and answers with the options the rest of the run
// uses - the manifest under its numbered name.
//
// The files are renamed where they stand rather than copied. A plan of a
// million files is hundreds of megabytes, and every caller plans afresh for
// each run - the command line, and the window's preview and run - so nothing
// runs a plan this has renamed a second time.
//
// names is the listing of the output directory, shared with preflight so a
// directory of a million is read once (mayStart).
func (res *Result) numberIfTaken(ctx context.Context, files []PlannedFile, opt Options, names func() *listing) (Options, error) {
	if !opt.NumberIfTaken || len(files) == 0 {
		return opt, nil
	}
	n, err := runNumber(ctx, files, opt, names())
	if err != nil || n < 2 {
		return opt, err
	}
	numbered := opt
	numbered.ManifestName = numberedRecord(manifestNameOf(opt), n)
	if err := numberFits(opt.OutDir, n, recordNames(manifestNameOf(numbered), explainedFiles(files))...); err != nil {
		return opt, err
	}
	exts := map[*Target]string{}
	for i := range files {
		name := numberedFile(files[i], n, exts)
		if err := numberFits(opt.OutDir, n, name); err != nil {
			return opt, err
		}
		files[i].Name = name
	}
	res.Number = n
	res.noteNumber(n, files[0].Name)
	return numbered, nil
}

// numberFits refuses a run whose number makes one of its names longer than a
// system stores.
//
// Length is the only question a number can change. Planning asked every name
// the rest (Plan, checkFileName), and "-2" brings no separator, colon, reserved
// word or trailing dot with it - it only adds bytes. A name that was at the
// limit before is a name somebody asked for at the limit: the preset that asks
// how a system treats the longest name it stores has one (filename-handling,
// measured 2026-11-14: 257 bytes with -5). Writing it shortened would answer a
// different question under the same entry, so the run is refused and told
// where the name fits as asked.
func numberFits(dir string, n int, names ...string) error {
	for _, name := range names {
		if len(name) > core.MaxNameBytes {
			return &NumberTooLongError{Dir: dir, Name: name, Number: n}
		}
	}
	return nil
}

// NumberTooLongError is a run into a directory holding an earlier one, refused
// because the number it would take there makes a name too long.
//
// Its own type rather than the refusal of a long name, because the name was
// fine as asked: what has to change is the directory, and the window puts the
// refusal under that box and offers to open it (the window's inTheWay).
type NumberTooLongError struct {
	// Dir is the output directory the number came from.
	Dir string
	// Name is the first name the number made too long.
	Name string
	// Number is the number the run would have taken.
	Number int
}

func (e *NumberTooLongError) Error() string { return e.Said().String() }

// Said is the refusal, for a window that says it in its own language.
func (e *NumberTooLongError) Said() core.Said {
	return core.Says("engine.NumberTooLong",
		"%s already holds names this run would write, so its names would carry the number %d, and that makes %s %d bytes long. "+
			"A file name holds at most %d bytes on some systems, and this one was close to that as it was asked for. "+
			"Generate into an empty directory, where every name is written as asked",
		core.A("Dir", core.Shown(e.Dir)), core.A("Number", e.Number), core.A("Name", core.Shown(e.Name)),
		core.A("Length", len(e.Name)), core.A("Most", core.MaxNameBytes))
}

// numbered is opt with the manifest under the name this run gave it, which is
// what claiming it, saving it and saying where it went all have to use.
func (res *Result) numbered(opt Options) Options {
	if res == nil || res.Number < 2 {
		return opt
	}
	opt.ManifestName = numberedRecord(manifestNameOf(opt), res.Number)
	return opt
}

// noteNumber says the run took a number, once, with a name to show what that
// looks like. Silence is banned (untouchable rule 6), and a name somebody did
// not ask for is the first thing they would otherwise go looking for.
func (res *Result) noteNumber(n int, example string) {
	res.RunNotes = append(res.RunNotes, core.Says("engine.Numbered",
		"the output directory already holds names this run would write, so every name of this run carries the number %d, as in %s. Nothing that was there has changed",
		core.A("Number", n), core.A("Example", core.Shown(example))))
}

// runNumber is the smallest number at which every name this run would write is
// free in its directory, one meaning no number at all.
//
// The loop has no ceiling of its own and needs none. A number is passed over
// only when something in the directory holds one of its names, and a directory
// holds finitely many, so the first number past all of them is free. A ceiling
// would be a refusal nothing could reach (trap 2 in CLAUDE.md). It can be
// stopped, which is what a long one needs.
//
// The temporary names are not asked about here. They carry this process's id,
// so only a run of an earlier process with the same id could have left one,
// and preflight still asks - a leftover of that kind is refused as it always
// was.
func runNumber(ctx context.Context, files []PlannedFile, opt Options, taken *listing) (int, error) {
	// An empty or missing directory is the ordinary first run, and it is
	// answered without walking the plan.
	if taken != nil && len(taken.exact) == 0 {
		return 1, nil
	}
	explains := explainedFiles(files)
	exts := map[*Target]string{}
	for n := 1; ; n++ {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		if recordFree(taken, opt, n, explains) && filesFree(ctx, taken, files, opt.OutDir, n, exts) {
			return n, nil
		}
	}
}

// recordFree says whether the record of this run is free under number n: the
// manifest, the reservation a run holds it by, and the instructions in a run
// that writes them.
func recordFree(taken *listing, opt Options, n int, explains bool) bool {
	name := numberedRecord(manifestNameOf(opt), n)
	held := append(recordNames(name, explains), filepath.Base(manifest.ReservationPath(name)))
	for _, h := range held {
		if hit, _ := taken.holds(filepath.Join(opt.OutDir, h), h); hit {
			return false
		}
	}
	return true
}

// filesFree says whether every file of the plan is free under number n. It
// stops at the first that is not, so a number some earlier run holds costs one
// question rather than a pass over the plan.
func filesFree(ctx context.Context, taken *listing, files []PlannedFile, dir string, n int, exts map[*Target]string) bool {
	for i := range files {
		// Asked now and then rather than per file, and a cancelled walk
		// answers "not free" - the caller asks the context next and says so.
		if i%4096 == 0 && ctx.Err() != nil {
			return false
		}
		name := numberedFile(files[i], n, exts)
		if hit, _ := taken.holds(filepath.Join(dir, name), name); hit {
			return false
		}
	}
	return true
}

// recordNames is the record a run writes under the manifest name given: the
// manifest, and the instructions beside it in a run that writes them.
func recordNames(manifestName string, explains bool) []string {
	if !explains {
		return []string{manifestName}
	}
	return []string{manifestName, manifest.InstructionsName(manifestName)}
}

// numberedFile is the name of f under number n.
//
// Where the number goes is the owner's choice of 2026-11-14: after the batch in
// a name this tool put together, so the files of one run stand together in a
// listing (files-2_0001.pdf), and before the ending in a name somebody wrote
// whole, a template or a preset's (invoice_0001-2.pdf, 10mb_under_1kb-2.bin).
func numberedFile(f PlannedFile, n int, exts map[*Target]string) string {
	if n < 2 {
		return f.Name
	}
	return withNumber(f.Name, numberSpot(f, exts), n)
}

// numberSpot is where in f's name the number goes. The ending is the one the
// format gave the file, asked once a target, because ".tar.gz" is one ending
// and the last dot would split it.
func numberSpot(f PlannedFile, exts map[*Target]string) int {
	t := f.Target
	if t == nil {
		return beforeEnding(f.Name, f.Desc.Extension)
	}
	if t.NameTmpl == "" && strings.HasPrefix(f.Name, t.ID) {
		return len(t.ID)
	}
	ext, ok := exts[t]
	if !ok {
		ext = f.Desc.ExtensionFor(t.Properties)
		exts[t] = ext
	}
	return beforeEnding(f.Name, ext)
}

// numberedRecord is the manifest name under number n - before its ".json".
func numberedRecord(name string, n int) string {
	if n < 2 {
		return name
	}
	return withNumber(name, beforeEnding(name, ".json"), n)
}

// beforeEnding is where ext starts in name, compared without case. A name that
// does not end in it - a template somebody gave an ending of their own - takes
// the number before its last dot, and a name with no dot at its end.
func beforeEnding(name, ext string) int {
	if ext != "" && len(name) > len(ext) && strings.EqualFold(name[len(name)-len(ext):], ext) {
		return len(name) - len(ext)
	}
	if dot := strings.LastIndexByte(name, '.'); dot > 0 {
		return dot
	}
	return len(name)
}

// withNumber puts "-n" into name at.
func withNumber(name string, at, n int) string {
	return name[:at] + "-" + strconv.Itoa(n) + name[at:]
}
