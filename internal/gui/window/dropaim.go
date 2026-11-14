package window

import "github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"

// Where a folder or a file dropped onto the window goes, decided apart from
// the window (GUI rule 15): what was dropped and which boxes the screen has go
// in, an answer comes out, and nothing here draws or reads the disk. Exported
// for the guards, which live in another package.
//
// The rules, in the order they are asked, and why each is where it is:
//
//   - A screen with no path box, or a drop of nothing, comes to nothing. The
//     owner's decision of 2026-11-14 for About and Preferences.
//   - A screen whose work has started takes nothing, the way a frozen box
//     takes no typing. Asked before anything else because the work read the
//     boxes when it began, so no answer about the drop is worth giving.
//   - More than one thing dropped is refused, and nothing is taken: which of
//     them comes first is the order a file manager listed them in. The owner's
//     decision of 2026-11-14.
//   - The one thing dropped has to be a folder or a file that is there, with a
//     name that is text.
//   - Then the box. Exactly one box taking that kind gets it WHEREVER the drop
//     landed, and that is what keeps this working on a Linux desktop that runs
//     Wayland natively: there the toolkit's position is where the pointer was
//     before the drag began, not where it ended (glfw wl_window.c,
//     dataDeviceHandleMotion is empty). Only between two boxes of the same
//     kind is the position asked, and a drop on neither is refused rather
//     than guessed. See docs/DRAG-DROP-2026-11-14.md.

// DroppedItem is one thing dropped onto the window, once the disk was asked
// what it is.
type DroppedItem struct {
	Path string
	// Takes is the kind of path this is, and nought for a thing that is there
	// and is neither a folder nor a file.
	Takes parts.Takes
	// Unread is why the disk could not say what this is, nil when it could.
	Unread error
	// NotText is a name that is not text a box can hold.
	NotText bool
}

// DropBox is one path box on the screen a drop landed on.
type DropBox struct {
	Takes parts.Takes
	Area  Area
}

// Area is a rectangle on the canvas. The zero area holds no point: a box that
// is not drawn is never under a drop.
type Area struct{ X, Y, W, H float32 }

func (a Area) holds(x, y float32) bool {
	return a.W > 0 && a.H > 0 && x >= a.X && x < a.X+a.W && y >= a.Y && y < a.Y+a.H
}

// cut is the part of this area that lies inside another, or the zero area
// when none of it does.
func (a Area) cut(by Area) Area {
	left, top := max(a.X, by.X), max(a.Y, by.Y)
	right, bottom := min(a.X+a.W, by.X+by.W), min(a.Y+a.H, by.Y+by.H)
	if right <= left || bottom <= top {
		return Area{}
	}
	return Area{X: left, Y: top, W: right - left, H: bottom - top}
}

// DropRefusal is why a drop put nothing into a box, and so which sentence
// goes under it.
type DropRefusal int

// The reasons, in the order AimDrop asks about them.
const (
	RefusedMany DropRefusal = iota + 1
	RefusedNotText
	RefusedUnread
	RefusedNeither
	RefusedKind
	RefusedAimless
)

// DropAnswer is what a drop comes to. Box is an index into the boxes given,
// or below nought when nothing happens at all. Put says the path goes into
// the box, and otherwise Why says what goes under it.
type DropAnswer struct {
	Box int
	Put bool
	Why DropRefusal
}

// AimDrop decides where a drop goes. x and y are where the toolkit says it
// landed, asked only between two boxes of the same kind.
func AimDrop(items []DroppedItem, boxes []DropBox, x, y float32, busy bool) DropAnswer {
	if len(items) == 0 || len(boxes) == 0 || busy {
		return DropAnswer{Box: -1}
	}
	// A refusal goes under the box the drop landed on, and under the first
	// box where it landed on none - a screen with one box has no other choice.
	hit := boxAt(boxes, x, y)
	under := max(hit, 0)
	if len(items) > 1 {
		return DropAnswer{Box: under, Why: RefusedMany}
	}
	item := items[0]
	if why := refusalOf(item); why != 0 {
		return DropAnswer{Box: under, Why: why}
	}
	return boxFor(item, boxes, hit)
}

// refusalOf is what is wrong with a dropped thing whatever box it was aimed at,
// or nought when nothing is.
func refusalOf(item DroppedItem) DropRefusal {
	switch {
	case item.NotText:
		return RefusedNotText
	case item.Unread != nil:
		return RefusedUnread
	case item.Takes == 0:
		return RefusedNeither
	}
	return 0
}

// boxFor is the box a folder or a file that is there goes into. hit is the box
// under the drop, below nought when it landed on none.
func boxFor(item DroppedItem, boxes []DropBox, hit int) DropAnswer {
	var fitting []int
	for i, b := range boxes {
		if b.Takes == item.Takes {
			fitting = append(fitting, i)
		}
	}
	switch {
	case len(fitting) == 1:
		return DropAnswer{Box: fitting[0], Put: true}
	case hit >= 0 && boxes[hit].Takes == item.Takes:
		return DropAnswer{Box: hit, Put: true}
	case hit >= 0:
		return DropAnswer{Box: hit, Why: RefusedKind}
	case len(fitting) == 0:
		return DropAnswer{Box: 0, Why: RefusedKind}
	}
	return DropAnswer{Box: fitting[0], Why: RefusedAimless}
}

// boxAt is the box under a point, or below nought when there is none.
func boxAt(boxes []DropBox, x, y float32) int {
	for i, b := range boxes {
		if b.Area.holds(x, y) {
			return i
		}
	}
	return -1
}
