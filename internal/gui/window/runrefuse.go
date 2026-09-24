package window

import (
	"errors"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// Where a refusal lands: under the box it is about, or at the foot of the
// form when it is about the run as a whole.
//
// Split out of run.go on 2026-09-14, when the strip at the foot of the form
// took that file past its ceiling. The ceiling is a ratchet, so the answer is
// a split and never a higher number. The split is by subject, and this
// subject was chosen over the preview because it holds no goroutine and no
// channel: the files that may be concurrent are declared by name, in the
// guards and in the workflow that runs the race detector, and moving a
// goroutine into a new file is a change to that list.

// refuse shows a refusal, under the field it is about where it names one.
//
// The choice is the engine's rather than the window's. It sets the setting on
// the error, and this looks it up - the alternative was matching the wording
// here, which is a second copy of rules the engine owns and the copy that
// drifts.
func (r *runner) refuse(err error) {
	// A refusal replaces what the status line said about work under way. Seen
	// in the real window on 2026-09-23: a preview refused for a manifest
	// already in the directory left "Working out what this would cost..."
	// standing over the refusal, as if it were still working. Both ways into
	// a refusal from planning - a preview and Generate - set that line first
	// and neither took it down.
	showOn(r.status, "")
	var loose []string
	// The first box a refusal lands on, so the form can be brought to it. A
	// refusal that marks a box the person cannot see reads as a button that did
	// nothing - see parts.Reveal and O107.
	first := ""
	for _, one := range spread(err) {
		if where := placed(r, one); where != "" {
			if first == "" {
				first = where
			}
			continue
		}
		// About the run rather than about one box, or about a setting this
		// screen does not draw. The foot of the form is where those belong.
		loose = append(loose, core.ShownText(one.Error()))
	}
	if len(loose) > 0 {
		r.problem.Say(strings.Join(loose, "\n\n"))
	}
	if first == "" {
		return
	}
	// Every problem went onto a box, so the foot of the form says nothing and
	// the only sign the press was even received is a red box that may be off
	// the screen. Both halves of the answer are here: a sentence where the
	// button is, and the form moved to the first box that needs attention.
	if len(loose) == 0 {
		r.say(text.RefusedBeforeWriting())
	}
	// Anything folded away that a refusal is about is opened before the form is
	// moved, because a box inside a fold cannot be shown by scrolling to it -
	// and a screen that refuses to run while marking nothing anybody can see
	// reads as a button that did nothing. This is what keeps the objection of
	// 2026-08-18 answered rather than dodged: refusals about a batch that is
	// not on the screen were the reason a list with one batch open at a time
	// was rejected.
	for _, marked := range r.fields.Marked() {
		if r.unfold != nil {
			r.unfold(marked)
		}
		if field := r.fields.Lookup(marked); field != nil {
			r.sections.openHolding(field.Control)
		}
	}
	if field := r.fields.Lookup(first); field != nil {
		parts.Reveal(r.scroll, field.Control)
	}
}

// placed puts one refusal under the box it is about and says which, or says
// nothing when it is about no box this screen draws. A function rather than a
// method, because the runner stands at its ceiling of methods, and out of
// refuse because the two ways of placing nested it past the ceiling of depth.
func placed(r *runner, one error) string {
	// About what is already in the output directory: put under that box,
	// with the way to the directory under the sentence - see inTheWay.
	if dir := inTheWay(one); dir != "" {
		if where := r.placeOf(engine.SettingOutDir); r.fields.Mark(where, one) {
			r.fields.Lookup(where).Offer(text.ButtonOpenFolder(), openIn(r.offer, dir))
			return where
		}
	}
	// An interface rather than a case per error type, so a screen shown a
	// kind of refusal nobody thought about here still gets it placed. The
	// engine, the format registry and the preset package all answer this
	// and none of them had to be imported for the question to be asked.
	var about interface{ AboutSetting() string }
	if !errors.As(one, &about) || about.AboutSetting() == "" {
		return ""
	}
	if where := r.placeOf(about.AboutSetting()); r.fields.Mark(where, one) {
		return where
	}
	return ""
}

// openIn is what the button under a refusal about a directory does: ask the
// desktop to show that directory.
func openIn(o *offers, dir string) func() {
	return func() {
		if o.openFolder != nil {
			o.openFolder(dir)
		}
	}
}

// spread opens a refusal that carries several into the ones it carries.
//
// The window used to mark ONE box however many were wrong, because everything
// between the screen and the field registry was singular by type: settle
// returned at the first bad box, refuse took one error, Mark marked one field.
// Reported from the screen on 2026-08-18, and it is the window narrowing what
// the layer below already does - RC7 has the engine refuse a recipe with every
// problem it has rather than the first, on the grounds that fixing a file one
// error per run is the cheapest way to make somebody stop using the tool. The
// same argument applies to a form.
//
// errors.Join is what carries them, so nothing here has to be a new error type
// and a single refusal still arrives as itself. Walked rather than flattened
// once, because a join can hold a join - the preset screen collects its own and
// hands on whatever the recipe parser gave it.
func spread(err error) []error {
	if err == nil {
		return nil
	}
	if joined, several := err.(interface{ Unwrap() []error }); several {
		var out []error
		for _, one := range joined.Unwrap() {
			out = append(out, spread(one)...)
		}
		return out
	}
	// A join under a single wrapper is still a join. The type assertion above
	// only sees the outermost layer, so one fmt.Errorf("%s: %w", ...) anywhere
	// on the way here turns five marked boxes back into one paragraph at the
	// foot of the form - which is the defect this function exists to prevent.
	//
	// Nothing wraps a join today: settle returns errors.Join straight out on
	// both screens. So this is not fixing anything that is broken, it is
	// removing the way it comes back - and it comes back silently, because the
	// guards for marking all build their errors with a bare join.
	if inner := errors.Unwrap(err); inner != nil {
		if _, several := inner.(interface{ Unwrap() []error }); several {
			return spread(inner)
		}
	}
	return []error{err}
}

// recheck says what is wrong with the screen while somebody is still typing.
//
// Asked for from the screen on 2026-08-18: a bad value should turn its box red
// and give the reason straight away, rather than waiting for a button. It runs
// the SAME settle the buttons run, which is the whole of the design - there is
// no second set of rules to write, nothing to keep in step, and a box that can
// be refused is refused here because it is refused there. A field nobody has
// added a rule for needs no rule added.
//
// Two things it does not do. It leaves the foot of the form alone, because a
// complaint about the run rather than about a box is not something to shout
// while somebody is mid-word. And it says nothing about an empty box - see
// Fields.Blank.
// placeOf is where a refusal about one setting belongs on this screen, which is
// the setting itself unless the screen said otherwise - see readdress.
func (r *runner) placeOf(setting string) string {
	if r.readdress == nil {
		return setting
	}
	return r.readdress(setting)
}

// withoutTheTarget is the readdress a screen showing exactly one target uses.
//
// The engine addresses a refusal about a target by its position, because a
// screen with twenty batches cannot place one otherwise. A screen with one
// batch draws its boxes under the bare key - measured 2026-08-25, the single
// batch screen registers size, name and width where the batch screen registers
// targets[1].size, targets[1].name and targets[1].properties.width. So the
// position is the part to drop, and the last segment is what is left.
//
// Settings of the run itself are handed back untouched. output.dir names no
// target, and taking its last segment would leave "dir", which is a box
// nothing draws.
func withoutTheTarget(address string) string {
	if !core.AddressNamesATarget(address) {
		return address
	}
	return core.LastSettingSegment(address)
}

func (r *runner) recheck(setting string) {
	// Nothing to check against yet, during the screen being built.
	if r.settle == nil {
		return
	}
	// A run owns the screen while it lasts. Its progress and its refusals are
	// not to be wiped by a keystroke.
	if r.busy.occupied {
		return
	}
	// The form is read ONCE, for the line and for the box below. It was read
	// twice until 2026-09-23 - through refreshLine here and again after the
	// early return - and on the preset screen each reading expanded the
	// preset: 271-295 ms of the window's thread for every key typed with
	// upload-validation chosen, half of it spent working out an answer the
	// line had just been given (docs/GUI-MEMORY-2026-09-23.md section 4h).
	// Nothing settle reads is changed between the two places it was called,
	// so the one reading is the answer both of them got.
	targets, opt, err := r.settle()
	// Whatever changed, the line says what the form comes to now - over an
	// outcome or a preview, which described a form that no longer exists.
	// Before the early return below, because a box emptied to be retyped
	// changes the count as surely as a box filled in.
	lineFrom(r, targets, opt, err)
	// Only this box, in both directions. What the other boxes were told is
	// about values nobody has just changed, and it is still true - including
	// the parts of it this cannot see, because a format minimum and a name
	// already taken are the engine's answers rather than settle's.
	r.fields.Clear(setting)
	if r.fields.Blank(setting) {
		return
	}
	for _, one := range spread(err) {
		var about interface{ AboutSetting() string }
		if errors.As(one, &about) && r.placeOf(about.AboutSetting()) == setting {
			r.fields.Mark(setting, one)
			return
		}
	}
}

// clearProblems empties every place a refusal can appear, not just the last one
// used. Clearing only the foot of the form would leave a message under a field
// after the value that caused it was fixed.
func (r *runner) clearProblems() {
	r.problem.Clear()
	r.fields.ClearAll()
}
