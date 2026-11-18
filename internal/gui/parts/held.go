package parts

import "fyne.io/fyne/v2"

// Held is the options of a switch that cannot be chosen now, each with why,
// and the why open on a sheet while the pointer is over one. A switch carries
// one (Segments.Held) and asks it before a press or an arrow chooses anything.
//
// An option held stays on the switch and out of reach: a press or an arrow
// does not choose it, it is drawn in the quietest ink (ColorNameHeld), and the
// pointer over it opens why on the screen's sheet of explanations - the sheet
// the button beside a field opens on.
//
// Kept on the switch rather than taken off it, which is the owner's choice of
// 2026-11-18 for smallest without a width and a height: an option that comes
// and goes moves the words beside it whenever another field is typed in, and
// one that is not there cannot be found by somebody looking for it - which is
// how smallest was reported two days before.
//
// Choosing it by name (Segments.SetSelected) still moves onto it. The switch is
// a view of a box, and the box can hold a value the switch cannot offer now -
// typed, or put there before the field it depends on was emptied. The switch
// then says what the form will send, and the run says why it is refused.
//
// A type of its own rather than methods on Segments, which was at twenty of
// the twenty one methods a type may have before it counts as crowded.
type Held struct {
	why map[string]Detail
	// telling is the why open on the sheet it was opened on, or nil.
	telling   fyne.CanvasObject
	tellingOn *Tips
	// redraw is the switch's own refresh, for a change in what is held.
	redraw func()
}

// Set holds an option back with why, or lets it be chosen again when why is
// NoDetail.
func (h *Held) Set(option string, why Detail) {
	was, isHeld := h.why[option]
	switch {
	case why.Text == "" && !isHeld, why.Text != "" && isHeld && was.Text == why.Text:
		return
	case why.Text == "":
		delete(h.why, option)
	default:
		if h.why == nil {
			h.why = map[string]Detail{}
		}
		h.why[option] = why
	}
	h.stop()
	if h.redraw != nil {
		h.redraw()
	}
}

// Has says an option cannot be chosen now.
func (h *Held) Has(option string) bool {
	_, held := h.why[option]
	return held
}

// Telling is the why open on the sheet, or nil - for a guard.
func (h *Held) Telling() fyne.CanvasObject { return h.telling }

// reachable is the nearest of options from at, stepping by dir, that can be
// chosen - or -1 when there is none that way.
func (h *Held) reachable(options []string, at, dir int) int {
	for i := at + dir; i >= 0 && i < len(options); i += dir {
		if !h.Has(options[i]) {
			return i
		}
	}
	return -1
}

// tell opens why an option is held, near the control, when it is - and takes
// away whatever was open before.
func (h *Held) tell(option string, near fyne.CanvasObject) {
	h.stop()
	why, held := h.why[option]
	if !held || why.on == nil {
		return
	}
	h.telling, h.tellingOn = why.on.open(near, why.Text), why.on
}

// stop takes the why off its sheet.
func (h *Held) stop() {
	if h.telling != nil && h.tellingOn != nil {
		h.tellingOn.close(h.telling)
	}
	h.telling, h.tellingOn = nil, nil
}

// SegmentMiddle is the point in the middle of an option's segment, or the
// origin for an option the switch does not hold - for a guard that presses and
// hovers the way a pointer does.
func SegmentMiddle(s *Segments, option string) fyne.Position {
	x := float32(0)
	for _, o := range s.Options {
		w := s.segmentWidth(o)
		if o == option {
			return fyne.NewPos(x+w/2, s.MinSize().Height/2)
		}
		x += w + Hairline
	}
	return fyne.Position{}
}
