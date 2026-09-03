package engine

import (
	"context"
	"fmt"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// GeneratorCrashError is a generator that stopped the way a program stops when
// it has a defect in it, rather than the way this tool refuses something.
//
// Until 2026-09-03 there was one recover in this whole tree, around the yaml
// reader, and the argument that put it there applies to generators word for
// word: a crash in there is a crash on ordinary user input. Two formats hand
// their pixels to somebody else's encoder, and every format runs on numbers
// that came from a recipe.
//
// What the crash cost was worse than the crash. A panic while writing left the
// file under its temporary name, and untouchable rule 7 makes the manifest the
// whole authority over what cleanup may delete - so a file that never reached
// one is a file nothing in this tool can ever take away. verify then reports it
// for good.
//
// While says which half of the work it happened in, because the two end
// differently. Planning is the whole run, so the run does not start. Writing is
// one file, so the file is dropped and the rest of the run carries on, which is
// what every other failure of a single file does.
type GeneratorCrashError struct {
	Format string
	While  string
	Name   string
	Value  any
}

func (e *GeneratorCrashError) Error() string {
	where := e.While
	if e.Name != "" {
		where += " " + e.Name
	}
	return fmt.Sprintf(
		"the %s generator stopped with an internal error while %s: %v. "+
			"This is a defect in this tool rather than something a recipe can ask for. "+
			"Please report it with the settings that produced it",
		e.Format, where, e.Value)
}

// planWithoutCrashing asks a generator what it would produce and turns a panic
// into an ordinary error.
//
// This is the likelier of the two places, which is not where the report that
// asked for this was looking. The picture formats encode while PLANNING - jxl
// and avif walk a ladder of sizes and hand each rung to a borrowed encoder -
// so a defect in one of them lands here rather than in the write below. A
// container planning its children calls them from inside its own Plan, so a
// child crashing is caught here too - and named as the container, because the
// container is what this was asked to plan.
func planWithoutCrashing(desc format.Descriptor, r format.Request) (p format.Plan, err error) {
	defer func() {
		v := recover()
		if v == nil {
			return
		}
		err = &GeneratorCrashError{Format: desc.ID, While: "planning a file", Value: v}
	}()
	return desc.Generator.Plan(r)
}

// writeWithoutCrashing is the same around the write, so a crash costs one file
// instead of the process.
//
// Only the generator is wrapped. What follows it - the flush, the close, the
// rename - is this package's own code, and the close and the remove it already
// runs are what take the temporary file away once the panic has become an
// error. A defence around code that cannot crash is a defence nothing can turn
// red, and this project has taken seven of those back out.
//
// Two things this cannot catch, and both are worth naming rather than
// implying. A panic on another goroutine answers to that goroutine alone, so a
// generator that starts one takes the process with it. And a memory violation
// is not a panic at all - the AVX2 path in the AVIF encoder reads past its
// buffer, which is why .github/build-tags exists.
//
// One thing it catches and names wrongly: the progress callback runs inside the
// generator, through the counting writer, so a caller whose callback crashes is
// told its generator did. The window's callback posts to a channel and does
// nothing else, and a wrapper of its own to tell the two apart would be more
// machinery than the mistake is worth.
func writeWithoutCrashing(ctx context.Context, f PlannedFile, w io.Writer) (err error) {
	defer func() {
		v := recover()
		if v == nil {
			return
		}
		err = &GeneratorCrashError{Format: f.Desc.ID, While: "writing", Name: f.Name, Value: v}
	}()
	return f.Desc.Generator.Write(ctx, w, f.Plan)
}
