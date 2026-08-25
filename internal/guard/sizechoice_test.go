package guard

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// The three ways of saying how big are a switch with one box under it, and
// these two guards are what O114 turned into.
//
// O114 was that the three stood side by side as equals, only one of them could
// be filled in, and nothing on the screen said so - somebody filled in two and
// learned it from a refusal. It was closed by writing the rule where all three
// could be read with it, and three guards held that arrangement in place: the
// rule is on the screen, it is there once, and it is above all three.
//
// On 2026-08-25 the arrangement was replaced by a switch, so the state the rule
// warned about cannot be reached. All three of those guards went with it, and
// they are replaced rather than dropped: what they protected was that nobody
// can state two sizes without knowing, and that is now a property of the screen
// instead of a sentence on it. These two ask for the property.

// sizeWaySwitches are the controls that choose how each batch says how big, in
// the order the batches are drawn in.
func sizeWaySwitches(o fyne.CanvasObject) []*widget.RadioGroup {
	var found []*widget.RadioGroup
	walk(o, func(obj fyne.CanvasObject) {
		if group, ok := obj.(*widget.RadioGroup); ok {
			found = append(found, group)
		}
	})
	return found
}

// sizeWaySwitch is the one belonging to the first batch.
func sizeWaySwitch(t *testing.T, o fyne.CanvasObject) *widget.RadioGroup {
	t.Helper()
	return sizeWaySwitchIn(t, o, 1)
}

// sizeWaySwitchIn is the one belonging to a batch counted from one, the way
// every address on this screen counts.
func sizeWaySwitchIn(t *testing.T, o fyne.CanvasObject, position int) *widget.RadioGroup {
	t.Helper()
	all := sizeWaySwitches(o)
	if position < 1 || position > len(all) {
		t.Fatalf("batch %d has no switch for the three ways of stating a size, and there are %d of them",
			position, len(all))
	}
	return all[position-1]
}

// chooseSizeWay presses one of its options on the first batch.
func chooseSizeWay(t *testing.T, o fyne.CanvasObject, way string) {
	t.Helper()
	chooseSizeWayIn(t, o, 1, way)
}

// chooseSizeWayIn presses one of its options, the way a person does.
func chooseSizeWayIn(t *testing.T, o fyne.CanvasObject, position int, way string) {
	t.Helper()
	group := sizeWaySwitchIn(t, o, position)
	for _, option := range group.Options {
		if option == way {
			group.SetSelected(way)
			return
		}
	}
	t.Fatalf("the switch offers %v and not %q", group.Options, way)
}

// TestEveryWayOfStatingASizeIsOfferedAndOnlyOneIsOnTheScreen is the first half:
// all three are reachable, and never two at once.
//
// Reachable matters as much as exclusive. A switch that offered one way and hid
// the other two behind nothing would make the invalid state unreachable by
// making two thirds of the engine unreachable with it, which is D1 broken to
// pass a guard.
func TestEveryWayOfStatingASizeIsOfferedAndOnlyOneIsOnTheScreen(t *testing.T) {
	ways := map[string]string{
		text.SizeWayExact():    text.FieldSize(),
		text.SizeWayRange():    text.FieldSizeRange(),
		text.SizeWayBoundary(): text.FieldBoundary(),
	}

	content, _ := laidOutWindow(t)
	batches := tabContent(t, content, text.TabRecipe())

	group := sizeWaySwitch(t, batches)
	if len(group.Options) != len(ways) {
		t.Fatalf("the switch offers %v, and there are %d ways of stating a size", group.Options, len(ways))
	}

	for way, box := range ways {
		chooseSizeWay(t, batches, way)
		// Whole words rather than anywhere in the text. "Size" lives inside
		// "Size range", so a guard asking whether the screen contains one of
		// them answers yes for the other - which it did, on its first run, and
		// it is the same substring mistake this screen's refusals were taken
		// apart by earlier the same day.
		shown := map[string]bool{}
		for _, said := range strings.Split(shownText(batches), "\n") {
			shown[said] = true
		}
		if !shown[box] {
			t.Errorf("the switch is on %q and the screen has no box called %q", way, box)
		}
		for other, hidden := range ways {
			if other == way {
				continue
			}
			if shown[hidden] {
				t.Errorf("the switch is on %q and %q is on the screen as well, so two sizes can be stated at once",
					way, hidden)
			}
		}
	}
}

// TestOnlyTheChosenWayOfStatingASizeReachesTheRun is the half that matters, and
// it asks the engine rather than the screen.
//
// A box that is hidden keeps what was typed into it, so that changing your mind
// twice does not cost the value. That is the whole risk of this design: if the
// hidden value were still sent, the run would refuse a batch stating both a
// size and a size range - the very refusal O114 was about, now arriving from a
// box nobody can see.
//
// So one way is filled in with something valid, the switch is moved, and the
// run is asked. Neither half of this is a declaration: the value is typed into
// a real box and the verdict comes from the engine.
func TestOnlyTheChosenWayOfStatingASizeReachesTheRun(t *testing.T) {
	batches, _, host := screenInAWindowWithHost(t, text.TabRecipe())
	entryUnder(t, batches, text.FieldTargetID()).SetText("invoices")

	// One size, stated and valid.
	chooseSizeWay(t, batches, text.SizeWayExact())
	entryUnder(t, batches, text.FieldSize()).SetText("2mb")
	press(t, batches, text.ButtonPreview())
	join(host)
	if refusal := anyRefusal(batches); refusal != "" {
		t.Fatalf("a batch stating one size was refused before anything was switched: %s", refusal)
	}

	// Now a range, stated and valid, with the size still in its box.
	chooseSizeWay(t, batches, text.SizeWayRange())
	entryUnder(t, batches, text.FieldSizeRange()).SetText("1kb-8kb")
	press(t, batches, text.ButtonPreview())
	join(host)
	if refusal := anyRefusal(batches); refusal != "" {
		t.Errorf("the switch is on %q and the run was refused, so the box behind it is still being sent:\n%s",
			text.SizeWayRange(), refusal)
	}

	// And the value left behind does not stand in for a way that is empty. The
	// run has to refuse here, or the size typed at the start is quietly acting
	// as the answer to a question nobody answered.
	chooseSizeWay(t, batches, text.SizeWayBoundary())
	press(t, batches, text.ButtonPreview())
	join(host)
	if anyRefusal(batches) == "" {
		t.Errorf("the switch is on %q with nothing in its box and the run was accepted, "+
			"so a value typed under another way is answering for it", text.SizeWayBoundary())
	}
}

// shownText is what a person can read, which is not the same as what the tree
// holds.
//
// allText walks every label there is, and a hidden box is still a box in the
// tree - so a guard asking whether two ways of stating a size are on the screen
// at once would answer yes however well the hiding worked. Measured on
// 2026-08-25: it reported all three while the render showed one.
//
// Built out of walk rather than beside it. A second walker is a second thing
// that has to know about ThemeOverride, about a Card being a widget rather than
// a container, and about the reflect fallback for a type nobody listed - and
// this project has four recorded cases of a walk meeting an unknown type and
// silently reporting an empty tree. So this one walks twice with the same walk:
// everything, then everything under each hidden thing, and takes the second
// away from the first.
func shownText(o fyne.CanvasObject) string {
	var all []*widget.Label
	var hidden []fyne.CanvasObject
	walk(o, func(obj fyne.CanvasObject) {
		if l, ok := obj.(*widget.Label); ok && l.Text != "" {
			all = append(all, l)
		}
		if obj != nil && !obj.Visible() {
			hidden = append(hidden, obj)
		}
	})

	out := map[*widget.Label]bool{}
	for _, l := range all {
		out[l] = true
	}
	for _, root := range hidden {
		walk(root, func(obj fyne.CanvasObject) {
			if l, ok := obj.(*widget.Label); ok {
				delete(out, l)
			}
		})
	}

	var said []string
	for _, l := range all {
		if out[l] {
			said = append(said, l.Text)
		}
	}
	return strings.Join(said, "\n")
}

// anyRefusal is whatever the screen is complaining about, or nothing.
func anyRefusal(o fyne.CanvasObject) string {
	var said []string
	walk(o, func(obj fyne.CanvasObject) {
		if l, ok := obj.(*widget.Label); ok && l.Importance == widget.DangerImportance && l.Text != "" {
			said = append(said, l.Text)
		}
	})
	return strings.Join(said, "\n")
}
