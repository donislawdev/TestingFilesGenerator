package window

import (
	"os"
	"path/filepath"
	"sync"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// A folder or a file dropped onto the window goes into the path box that takes
// it, as if it had been picked with Choose... beside that box. Owner's decision
// of 2026-11-14, docs/DRAG-DROP-2026-11-14.md.
//
// The toolkit takes ONE callback for the whole window (fyne v2.8.2, window.go:
// SetOnDropped) and has no way for a control to be a drop target of its own.
// So the window holds the callback, asks which screen is being looked at, and
// the screen's path boxes say what they take - the box is the target, and a
// path box added to any screen tomorrow takes drops without a line written
// here. Where it goes is AimDrop's answer, see dropaim.go.

// dropScreen is what a screen hands the window so a drop can land on it: the
// fields that hold its path boxes, whether work owns it, and the three things
// that bring a box into view.
type dropScreen struct {
	fields   *parts.Fields
	busy     *busy
	sections *sections
	scroll   *container.Scroll
	// unfold opens whatever the screen put a box away inside. Nil on a screen
	// that folds nothing of its own.
	unfold func(string)
}

// takesDrops is a screen that has path boxes. The three screens that run the
// engine hand over their runner's, and the Tools tab its own.
type takesDrops interface{ dropScreen() dropScreen }

func (g *Generate) dropScreen() dropScreen { return dropOnto(g.runner) }
func (p *Preset) dropScreen() dropScreen   { return dropOnto(p.runner) }
func (r *Recipe) dropScreen() dropScreen   { return dropOnto(r.runner) }

// dropScreen is what the Tools tab hands the window. It folds nothing of its
// own beyond its sections, so there is no unfold.
func (t *Tools) dropScreen() dropScreen {
	return dropScreen{fields: t.fields, busy: t.busy, sections: t.sections, scroll: t.view.scroll}
}

// dropOnto is what a screen that runs the engine hands the window. A function
// rather than a method, because the runner stands at its ceiling of methods.
func dropOnto(r *runner) dropScreen {
	return dropScreen{fields: r.fields, busy: r.busy, sections: r.sections, scroll: r.scroll, unfold: r.unfold}
}

// dropping is the window's one answer to a drop.
type dropping struct {
	screens map[string]dropScreen
	looking *string

	// latest counts drops, so the answer about an older one that the disk
	// gave late does not land over a newer one.
	latest int
	// reading is the disk being asked about a drop, waited for by a guard.
	reading sync.WaitGroup
}

// wireDropping puts the window's drop callback on the host. looking is the tab
// on show, read when the drop arrives - the same pointer the keyboard reads.
//
// The screens are the ones the keyboard reaches, each asked whether it takes a
// drop, so the work screens are one list rather than a second one kept beside
// the first: a screen added to the keyboard's map with path boxes on it takes
// drops the day it is added.
func wireDropping(h Host, keyed map[string]keyboardScreen, looking *string) *dropping {
	d := &dropping{screens: map[string]dropScreen{}, looking: looking}
	for id, screen := range keyed {
		if takes, ok := screen.(takesDrops); ok {
			d.screens[id] = takes.dropScreen()
		}
	}
	h.SetOnDropped(d.dropped)
	return d
}

// Settled waits for the disk to answer about a drop, for a guard. See
// offerSettling.
func (d *dropping) Settled() { d.reading.Wait() }

// dropped is the toolkit's callback, on the interface thread.
//
// The disk is asked on a worker, and that is a measurement rather than care:
// os.Stat on a network path nobody answers for took 21.0 s on this machine on
// 2026-11-14, and the window would have stood still for all of it. More than
// one thing is refused without asking the disk at all.
func (d *dropping) dropped(at fyne.Position, uris []fyne.URI) {
	screen, takes := d.screens[*d.looking]
	if !takes || len(uris) == 0 {
		return
	}
	d.latest++
	if len(uris) > 1 {
		d.land(screen, make([]DroppedItem, len(uris)), at)
		return
	}
	d.read(dropWaiting{mine: d.latest, aimed: *d.looking, screen: screen, at: at}, PathOf(uris[0]))
}

// PathOf is the path a toolkit address names, written the way the system
// writes it.
//
// The toolkit holds a path with forward slashes on every system (fyne v2.8.2,
// storage/repository/parse.go), so on Windows both a picker and a drop hand
// back "C:/Users". One function for the pickers and for a drop, so a folder
// picked with Choose... and the same folder dropped put the same text in the
// box. The pickers handed the forward slashes straight on until 2026-11-14.
func PathOf(u fyne.URI) string { return filepath.FromSlash(u.Path()) }

// dropWaiting is one drop the disk is being asked about: which drop it was,
// the screen it was aimed at, and where it landed.
type dropWaiting struct {
	mine   int
	aimed  string
	screen dropScreen
	at     fyne.Position
}

// read asks the disk on a worker and answers on the interface thread.
func (d *dropping) read(drop dropWaiting, path string) {
	d.reading.Add(1)
	go func() {
		defer d.reading.Done()
		item := readDropped(path)
		fyne.Do(func() { d.arrived(drop, item) })
	}()
}

// arrived lands a drop the disk has answered about, unless a newer drop or
// another tab has made it an answer about a screen nobody is dropping onto
// any more.
func (d *dropping) arrived(drop dropWaiting, item DroppedItem) {
	if drop.mine != d.latest || *d.looking != drop.aimed {
		return
	}
	d.land(drop.screen, []DroppedItem{item}, drop.at)
}

// readDropped asks the disk what a dropped path is. Followed through a link,
// as a path typed into the box is.
func readDropped(path string) DroppedItem {
	item := DroppedItem{Path: path}
	if !utf8.ValidString(path) {
		item.NotText = true
		return item
	}
	info, err := os.Stat(path)
	switch {
	case err != nil:
		item.Unread = err
	case info.IsDir():
		item.Takes = parts.TakesFolder
	case info.Mode().IsRegular():
		item.Takes = parts.TakesFile
	}
	return item
}

// land puts a drop where AimDrop says, on the interface thread, and brings the
// box into view whatever happened to it - a change made off the screen is a
// change nobody saw.
func (d *dropping) land(screen dropScreen, items []DroppedItem, at fyne.Position) {
	targets := parts.PathTargets(screen.fields)
	answer := AimDrop(items, boxesOf(targets, screen.scroll), at.X, at.Y, screen.busy.occupied)
	if answer.Box < 0 {
		return
	}
	target := targets[answer.Box]
	if answer.Put {
		// What the box said about its last value is not about this one.
		target.Field.Clear()
		target.Box.SetText(items[0].Path)
	} else {
		target.Field.Say(dropSentence(answer.Why, target, items))
	}
	if screen.unfold != nil {
		screen.unfold(target.Field.Setting)
	}
	screen.sections.openHolding(target.Field.Control)
	parts.Reveal(screen.scroll, target.Field.Control)
}

// boxesOf is the path boxes of a screen as AimDrop takes them.
//
// Where each one stands is asked only when there are two or more, because only
// then is the position read at all - and asking the toolkit where something is
// walks the whole canvas.
func boxesOf(targets []parts.PathTarget, scroll *container.Scroll) []DropBox {
	boxes := make([]DropBox, len(targets))
	for i, t := range targets {
		boxes[i].Takes = t.Takes
		if len(targets) > 1 {
			boxes[i].Area = areaOf(t.Field.Control, scroll)
		}
	}
	return boxes
}

// areaOf is where a control stands on the canvas, cut to the part of the
// screen that scrolls, or the zero area when it is not drawn.
//
// The toolkit answers nought for a control it did not find on a visible
// canvas, which is a control folded away. A box at the very corner of the
// window would read the same, and none stands there: the tabs do.
func areaOf(o fyne.CanvasObject, scroll *container.Scroll) Area {
	driver := fyne.CurrentApp().Driver()
	at := driver.AbsolutePositionForObject(o)
	if at.IsZero() || !o.Visible() {
		return Area{}
	}
	size := o.Size()
	a := Area{X: at.X, Y: at.Y, W: size.Width, H: size.Height}
	if scroll == nil {
		return a
	}
	view := driver.AbsolutePositionForObject(scroll)
	return a.cut(Area{X: view.X, Y: view.Y, W: scroll.Size().Width, H: scroll.Size().Height})
}

// dropSentence is what goes under a box a drop did not go into.
func dropSentence(why DropRefusal, target parts.PathTarget, items []DroppedItem) string {
	switch why {
	case RefusedMany:
		return text.DropOneAtATime(len(items))
	case RefusedNotText:
		return text.DropNotText()
	case RefusedAimless:
		return text.DropAimless()
	case RefusedUnread:
		return text.Refusal(items[0].Unread, target.Field.Label)
	}
	name := droppedName(items[0].Path)
	switch {
	case why == RefusedNeither:
		return text.DropNeither(name)
	case target.Takes == parts.TakesFolder:
		return text.DropTakesFolder(target.Field.Label, name)
	}
	return text.DropTakesFile(target.Field.Label, name)
}

// droppedName is the name a sentence calls a dropped path by: the last part of
// it, or the whole of it for the root of a drive, whose last part is a slash.
func droppedName(path string) string {
	name := filepath.Base(path)
	if name == "." || os.IsPathSeparator(name[0]) {
		name = path
	}
	return core.Shown(name)
}
