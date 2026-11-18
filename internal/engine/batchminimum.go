package engine

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// A size refused for a batch is answered for the WHOLE batch.
//
// Reported by the owner from the running window on 2026-11-16
// (docs/SIZE-FIXES-2026-11-16.md): a refusal named the smallest size of the
// first file that refused, the button under it put that number in the box, and
// the next run was refused again. The files of one target are not one file.
// Each picture is drawn from its own seed and codes to its own size - JPG
// 640x480, 24 files, smallest 32 443 to 42 496 B (O353) - and a text format
// that states its size in its label refused the very minimum it had named one
// run before (pdf 3409, then 3415). So the number a refusal names is now the
// smallest size every file of the target accepts, found by asking the files
// rather than by guessing, and both surfaces get the same number (G1).
//
// Only when the run is refused. A run that plans is untouched, so no file of a
// run that worked moves by a byte.

// refusedForTheBatch is the refusal planning a target ends with: err as it is,
// unless err is a size below what the format makes on a target of one size -
// then the minimum it names is the smallest size every file of the target
// accepts.
//
// The format's own refusal goes out unchanged whenever the batch agrees with
// it, which is every target of one file whose format names a true minimum. Its
// wording is the format's, with the alternatives only the format knows - a
// smaller picture, a lower quality - and that is worth keeping wherever it is
// right. When the batch needs more, the hint is worded here, because every
// hint a format writes carries the numbers of the one file it was asked about.
//
// first is the place in the target of the file that refused, from nought.
func refusedForTheBatch(ctx context.Context, walk batchWalk, first int, err error) error {
	var below *format.BelowMinimumError
	if !errors.As(err, &below) || below.Minimum <= 0 || !oneSize(walk.t) {
		return err
	}
	got, walkErr := walk.sizeEveryFileTakes(ctx, first, below)
	if walkErr != nil {
		return walkErr
	}
	refusal := err
	if got.size != below.Minimum {
		refusal = got.refusal(below.Requested, len(walk.t.Sizes))
	}
	if len(walk.t.Sizes) > 1 && smallestFits(walk.t, walk.desc) {
		refusal = &smallestOffered{err: refusal}
	}
	return refusal
}

// oneSize says every file of t was asked for one stated number, the one shape
// whose refusal a single number can answer.
//
// The others are answered elsewhere or not at all. A range draws its sizes and
// snaps them to writable ones itself (drawSizes). A boundary set is three sizes
// around a limit, and the smallest LIMIT is a different question that a guard
// keeps without a button (formcolumns_test.go). smallest and a size from the
// contents name no number to refuse.
func oneSize(t *Target) bool {
	if t.Smallest || t.SizeFromContents || t.Range.Used || t.BoundaryLimit != 0 || len(t.Sizes) == 0 {
		return false
	}
	for _, s := range t.Sizes[1:] {
		if s != t.Sizes[0] {
			return false
		}
	}
	return true
}

// batchWalk is what the walk below asks every file with: the target, its
// format, and the two things every file's request is built from.
type batchWalk struct {
	t          *Target
	desc       format.Descriptor
	targetSeed uint64
	damaged    string
	// run is what the memory setting lets the run hold (planstretch.go).
	run int64
	// position is the target's place in the recipe, counted from one, and
	// ext the ending of its files - what planning.file gives each of them.
	position int
	ext      string
}

// planAt plans file idx of the target, from nought, at size bytes.
func (w batchWalk) planAt(ctx context.Context, idx int, size int64) answer {
	r := requestFor(w.t, idx, core.FileSeed(w.targetSeed, idx), w.damaged)
	r.Bytes = size
	p, err := planFile(ctx, w.t, w.desc, r)
	return answer{plan: p, err: err}
}

// everyFile is where the walk ended.
type everyFile struct {
	// size is the smallest size every file accepts.
	size int64
	// last is the refusal that raised the size last.
	last *format.BelowMinimumError
	// differ says a file other than the first to refuse raised it, so the
	// files are not alike and no one file's reason is the batch's.
	differ bool
}

// sizeEveryFileTakes walks the files of the target round and round, from the
// one that refused and the minimum it named, until all of them in a row accept
// one size.
//
// A refusal raises the size to the minimum it names, or one byte past where it
// stands when it names nothing higher - a band the format cannot reach, the
// same step format.Descriptor.PlanSmallest takes. Every file is asked again
// after the last raise, and that is not caution: PNG accepts a picture's exact
// size and refuses the eleven bytes above it, so a file that took a smaller
// size can refuse a larger one.
//
// The cost is planning the batch once and part of it again - every file after
// the last raise, and the ones before it once more. A picture with its width
// and height set encodes while it plans, so ten thousand of them is the time
// their plan takes, measured in docs/SIZE-FIXES-2026-11-16.md. It holds nothing
// per file, so a million files cost time and no memory, and the run's context
// is asked before every stretch, so a stop is a stop.
//
// The files are asked a stretch at a time (planstretch.go), one file a
// goroutine, and their answers are read in the order of the walk. A file read
// after a refusal was asked at the size before the raise, so its answer is
// thrown away and it is asked again - which makes every raise, every file
// that raised it and so the reason the refusal gives the same at any width.
//
// The ceiling is for a generator that never accepts, which would otherwise walk
// one byte at a time without end. A raise never goes down, so nothing cycles,
// and no file raises it more often than its format walks to its own smallest
// size - format.SmallestRounds - so n files may raise it n times that.
func (w batchWalk) sizeEveryFileTakes(ctx context.Context, first int, below *format.BelowMinimumError) (everyFile, error) {
	n := len(w.t.Sizes)
	s := &walking{n: n, first: first, idx: first, ceiling: format.SmallestRounds * n,
		got: everyFile{size: max(below.Minimum, below.Requested+1), last: below}}
	ask := func(i int) answer { return w.planAt(ctx, i%n, s.got.size) }
	pace := pace{run: w.run}
	got := make([]answer, widthFor(n))
	for s.inARow < n {
		if err := ctx.Err(); err != nil {
			return everyFile{}, err
		}
		k, width := pace.stretch(n-s.inARow, 1, s.got.size, math.MaxInt)
		planStretch(&pace, got[:k], s.idx, width, ask)
		if err := s.take(got[:k], &pace, w.desc.ID); err != nil {
			return everyFile{}, err
		}
	}
	return s.got, nil
}

// walking is where the walk of sizeEveryFileTakes stands: the file to ask
// next, how many in a row accepted the size, and how often it was raised.
type walking struct {
	n, first, idx, inARow int
	raises, ceiling       int
	got                   everyFile
}

// take reads the answers of one stretch in order, up to the first refusal,
// which raises the size for the next.
func (s *walking) take(answers []answer, pace *pace, id string) error {
	for _, a := range answers {
		if a.err == nil {
			pace.saw(a.plan)
			s.inARow++
			s.idx = (s.idx + 1) % s.n
			continue
		}
		var refused *format.BelowMinimumError
		if !errors.As(a.err, &refused) {
			// A refusal about something else, or a stop the format noticed
			// first. Either is the answer somebody needs, so it goes out as it
			// came.
			return a.err
		}
		if s.raises++; s.raises > s.ceiling {
			return core.Defect(fmt.Errorf("%s: the size every one of %d files accepts did not settle in %d raises, the last at %d B",
				id, s.n, s.ceiling, s.got.size))
		}
		s.got.size = max(refused.Minimum, s.got.size+1)
		s.got.last, s.got.differ, s.inARow = refused, s.got.differ || s.idx != s.first, 0
		return nil
	}
	return nil
}

// refusal is the refusal for a batch whose size moved past the one the first
// file named.
//
// The reason stays the format's while one file did all the raising - a format
// stating its size in its label walks to its own smallest size that way, and
// its reason is still true. Once another file raised it, the reason is that
// the files are not alike, which is what somebody needs in order not to ask for
// the first file's number again. The hint is always this package's, because
// every hint a format writes carries the numbers of the one request it saw.
func (e everyFile) refusal(requested int64, files int) *format.BelowMinimumError {
	reason := e.last.Reason
	if e.differ {
		reason = core.Says("engine.EveryFileIsDifferent", "each of the %d files is different, so each has a smallest size of its own",
			core.A("Files", files))
	}
	return &format.BelowMinimumError{
		Format:    e.last.Format,
		Requested: requested,
		Minimum:   e.size,
		Reason:    reason,
		Hint:      askForEveryFile(e.size, files),
	}
}

// askForEveryFile is the hint of a refusal whose number is the batch's rather
// than one file's.
//
// No "or more", which every format's own hint says. Above the size every file
// accepts there can still be a byte count one of them cannot reach, and only
// this one number was asked of every file.
func askForEveryFile(size int64, files int) core.Said {
	if files == 1 {
		return core.Says("engine.AskForTheSmallestSize", "Ask for %d B.", core.A("Size", size))
	}
	return core.Says("engine.AskForTheSmallestSizeOfAll", "Ask for %d B, the smallest size all %d files can have.",
		core.A("Size", size), core.A("Files", files))
}

// smallestFits says t could ask for smallest instead of a number: a picture
// with its width and height known, the shape checkSmallest accepts - and no
// damage.
//
// Not with a damage, because smallest then has to clear the damage's floor
// too, and each file answers that at its own smallest size. Promising smallest
// there would be promising something only planning every file could keep.
func smallestFits(t *Target, desc format.Descriptor) bool {
	if len(t.Damage) != 0 {
		return false
	}
	asked := *t
	asked.Smallest = true
	return checkSmallest(&asked, desc) == nil
}

// smallestOffered is a refusal whose target could ask for smallest instead,
// which the window offers as a second way out (OffersSmallest).
//
// Only for a target of more than one file. Then the size every file accepts
// is the largest of their smallest sizes, and smallest makes each file at its
// own. A single file at its smallest size is the file smallest makes - the
// same bytes under the same name, measured for png, jpg and webp at 640x480
// on 2026-11-18 - so a second button there does what the first does, which
// the owner reported from the window that day. Not "when the smallest sizes
// differ": knowing that would mean planning every file at its smallest, ten
// thousand pictures encoded for the sake of one button.
//
// A wrapper rather than a field on format.BelowMinimumError, because the
// format does not know the target - a picture's width and height can come from
// the format's defaults as well as from the target, and only the engine holds
// both. It answers nothing itself, so every reader that asks the refusal
// underneath through errors.As - the exit code, the sentence, the setting it is
// about - gets exactly what it got before.
type smallestOffered struct{ err error }

func (s *smallestOffered) Error() string { return s.err.Error() }
func (s *smallestOffered) Unwrap() error { return s.err }

// OffersSmallest says err refused a size for a target that could ask for
// smallest instead - each file the smallest its own picture makes.
func OffersSmallest(err error) bool {
	var s *smallestOffered
	return errors.As(err, &s)
}
