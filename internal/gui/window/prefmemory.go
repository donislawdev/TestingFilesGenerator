package window

import (
	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// memoryPreference is the Memory section of the Preferences screen: how much
// memory a run may take - docs/STABILITY-REVIEW-2026-10-26.md section 10.8,
// the owner's decisions of 2026-10-27.
//
// The window's half of --memory. Three ways on one switch - Automatic, Custom
// and No limit - and a box for the size that is on the screen only while
// Custom is chosen, the way the form shows one of its three boxes for a size.
// What is chosen is kept at once, as the language is, and read at the start of
// every run (runner.kept), so it applies from the next run and never to one
// already going.
//
// A section of its own type from the start (GUI rule 9): it has a switch, a
// box, a line that says what the choice comes to on this machine, and the rule
// that opening the screen writes nothing.
type memoryPreference struct {
	host   Host
	fields *parts.Fields
	way    *parts.Segments
	size   *parts.Entry
	box    fyne.CanvasObject
	// state is what the chosen way comes to on this machine, or why a kept
	// value was not used. Never empty, so the section does not move when it
	// changes - GUI rule 3.
	state *parts.QuietText
	// opening is true while the screen puts the kept value on the controls,
	// when their callbacks must not write it back.
	opening bool
	object  fyne.CanvasObject
}

func newMemoryPreference(h Host) *memoryPreference {
	m := &memoryPreference{host: h, fields: parts.NewFields(), size: parts.NewEntry()}
	// The callbacks go on before the controls are registered, because the
	// fields chain what a control already does rather than replace it.
	m.way = parts.NewSegments(memoryWayNames(), m.chose)
	m.size.OnChanged = m.typed
	m.box = m.fields.Add(core.SettingMemory, text.FieldMemorySize(), text.HintMemorySize(), parts.NoDetail, parts.Numeric(m.size))
	m.state = parts.NewQuietText("")
	m.state.Wrapping = fyne.TextWrapWord
	m.open(h.Remembered().Memory())

	m.object = parts.Section(text.SectionMemory(),
		parts.Grid(m.fields.Named(text.FieldMemoryWay(), parts.NoDetail, m.way), m.box),
		parts.Prose(text.PreferencesMemoryScope()),
		m.state,
	)
	return m
}

// The three ways, in one order, named once.
func memoryWayNames() []string {
	return []string{text.MemoryWayAuto(), text.MemoryWayCustom(), text.MemoryWayOff()}
}

// open puts a kept setting on the controls without saving anything. A kept
// value that does not read is shown as Automatic, which is what a run does with
// it, and the line under the switch says what it was - saving Automatic over it
// would erase the person's choice without a press, untouchable rule 7.
//
// The box is emptied first. Forget opens the screen again on nothing kept, and
// a size left standing in the box from before would come back the moment
// Custom is chosen - kept again by the choice, after the person asked for it
// to be forgotten.
func (m *memoryPreference) open(kept string) {
	m.opening = true
	defer func() { m.opening = false }()
	m.size.SetText("")
	m.fields.Clear(core.SettingMemory)
	parsed, err := core.ParseMemory(kept)
	unreadable := ""
	switch {
	case kept == "":
		m.way.SetSelected(text.MemoryWayAuto())
	case err != nil:
		m.way.SetSelected(text.MemoryWayAuto())
		unreadable = kept
	case parsed.IsOff():
		m.way.SetSelected(text.MemoryWayOff())
	default:
		m.way.SetSelected(text.MemoryWayCustom())
		m.size.SetText(parsed.String())
	}
	m.show(unreadable)
}

// chose keeps the way picked on the switch. Custom keeps the size in the box
// when there is one, and Automatic until there is - which the line says.
func (m *memoryPreference) chose(string) {
	if m.opening {
		return
	}
	m.fields.Clear(core.SettingMemory)
	m.host.Remembered().RememberMemory(keptAs(m.chosen()))
	m.show("")
}

// typed keeps a size as it is typed, and marks the box while what is in it is
// not one - the run then uses Automatic, and the line says so.
func (m *memoryPreference) typed(string) {
	if m.opening || m.way.Selected != text.MemoryWayCustom() {
		return
	}
	if _, err := core.ParseMemorySize(m.size.Text); err != nil {
		m.fields.Mark(core.SettingMemory, err)
	} else {
		m.fields.Clear(core.SettingMemory)
	}
	m.host.Remembered().RememberMemory(keptAs(m.chosen()))
	m.show("")
}

// chosen is the setting the controls say, as a run will read it.
func (m *memoryPreference) chosen() core.Memory {
	switch m.way.Selected {
	case text.MemoryWayOff():
		return core.MemoryOff()
	case text.MemoryWayCustom():
		if size, err := core.ParseMemorySize(m.size.Text); err == nil {
			return size
		}
	}
	return core.Memory{}
}

// show leaves the box on the screen for Custom alone and says what the choice
// comes to.
func (m *memoryPreference) show(unreadable string) {
	if m.way.Selected == text.MemoryWayCustom() {
		m.box.Show()
	} else {
		m.box.Hide()
	}
	m.state.SetText(m.sentence(unreadable))
}

// sentence is the line under the switch.
func (m *memoryPreference) sentence(unreadable string) string {
	switch {
	case unreadable != "":
		return text.MemoryUnreadable(unreadable)
	case m.way.Selected == text.MemoryWayOff():
		return text.MemoryOffIs()
	case m.way.Selected == text.MemoryWayCustom() && m.chosen().IsAuto():
		return text.MemoryCustomEmpty()
	case m.way.Selected == text.MemoryWayCustom():
		return text.MemoryCustomIs()
	}
	limits := core.Memory{}.On(m.host.MachineMemory)
	if limits.Unknown != nil {
		return text.MemoryAutoUnknown(text.HumanBytes(limits.Run))
	}
	return text.MemoryAutoIs(text.HumanBytes(limits.Run))
}

// keptAs is the text a setting is kept as: nothing for Automatic, so the value
// is removed rather than written, the way a language left to the system is.
func keptAs(m core.Memory) string {
	if m.IsAuto() {
		return ""
	}
	return m.String()
}

// memoryOf is the kept setting as a run reads it. A value that does not read is
// Automatic, the same answer the screen shows for it.
func memoryOf(kept Remembered) core.Memory {
	m, err := core.ParseMemory(kept.Memory())
	if err != nil {
		return core.Memory{}
	}
	return m
}
