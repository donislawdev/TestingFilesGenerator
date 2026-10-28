package core

import (
	"errors"
	"fmt"
	"strings"
)

// The one number that says how much memory a run may take.
//
// docs/STABILITY-REVIEW-2026-10-26.md section 10.8, the owner's decisions of
// 2026-10-27 in 10.10. Before it there were two ceilings and neither could be
// moved: MaxPlanBytes refused a plan past two gigabytes on a machine with
// sixty four, and nothing at all held the writers back, so sixteen pictures of
// forty megapixels written at once took 2.6 GB on a machine where one took 358
// MB (section 3.2). The owner's principle is that somebody may want files of
// two hundred gigabytes or a great many of them, from either surface, and this
// tool must not be what stops them - so the number is theirs to set, and what
// it does when nobody sets it is worked out from the machine rather than
// written into the build.
//
// Here rather than in the engine because both surfaces read it: the command
// line from a flag and the window from what it remembers, and a value checked
// on one side only is one somebody reaches through the other - the argument
// that put ParseSize in this package.

// SettingMemory is the name a refusal about the memory setting is about. It is
// the flag's name, because the command line is where the setting is typed as
// text, and a window puts the label of its own box in the slot instead.
const SettingMemory = "memory"

// Memory is how much memory one run may take, the plan and the writing
// together.
//
// The zero value is auto: half of what this machine gives the program
// (MachineMemory), so a caller that never heard of the setting gets the safe
// answer rather than no limit at all. A size is what somebody asked for, and
// off is no limit - the writers are not held back and the plan has no ceiling,
// at the risk of the system ending the process, which is the person's choice.
type Memory struct {
	bytes int64
	off   bool
}

// MemoryOff is no limit at all.
func MemoryOff() Memory { return Memory{off: true} }

// MemoryOf is a size somebody chose. Nought or less is not a size and is
// read as auto, which is what ParseMemory refuses it for being.
func MemoryOf(bytes int64) Memory {
	if bytes <= 0 {
		return Memory{}
	}
	return Memory{bytes: bytes}
}

// IsAuto, IsOff and Bytes say which of the three a setting is, and the size
// when it is one.
func (m Memory) IsAuto() bool { return !m.off && m.bytes == 0 }
func (m Memory) IsOff() bool  { return m.off }
func (m Memory) Bytes() int64 { return m.bytes }

// The two words, spelled once for the parser, String and the help.
const (
	memoryAuto = "auto"
	memoryOff  = "off"
)

// String is the setting the way ParseMemory reads it back: auto, off, or the
// size in the largest unit that holds it whole. A window keeps this text and
// a later start reads it through ParseMemory, so the two must agree to the
// byte - 1.5 GiB comes back as 1536mb rather than as a rounded 1.5gb.
func (m Memory) String() string {
	switch {
	case m.off:
		return memoryOff
	case m.bytes == 0:
		return memoryAuto
	}
	for _, u := range []struct {
		name string
		size int64
	}{{"tb", 1 << 40}, {"gb", 1 << 30}, {"mb", 1 << 20}, {"kb", 1 << 10}} {
		if m.bytes%u.size == 0 {
			return fmt.Sprintf("%d%s", m.bytes/u.size, u.name)
		}
	}
	return fmt.Sprintf("%d", m.bytes)
}

// ParseMemory reads the setting as somebody wrote it: auto, off, or a size in
// the units every size in this tool takes.
//
// Nought is refused rather than read as no limit. Off is the word for that, the
// same as --far-over off, and a nought typed by mistake would otherwise take
// every limit away without a word. A size of one byte is accepted: it is a
// strange thing to ask for and it does what it says, a refusal for every plan
// and every picture written alone.
func ParseMemory(s string) (Memory, error) {
	raw := strings.TrimSpace(s)
	switch strings.ToLower(raw) {
	case memoryAuto:
		return Memory{}, nil
	case memoryOff:
		return MemoryOff(), nil
	case "":
		return Memory{}, RefuseAbout(SettingMemory, Says("core.MemoryEmpty", "empty {setting}: write auto, off or a size such as 8gb"))
	}
	if !strings.ContainsAny(raw, "0123456789") {
		return Memory{}, RefuseAbout(SettingMemory, Says("core.MemoryUnknown",
			"{setting} %q is not a setting this tool knows: write auto for half of this machine's memory, off for no limit, or a size such as 8gb",
			A("Value", s)))
	}
	if strings.HasPrefix(raw, "-") {
		return Memory{}, RefuseAbout(SettingMemory, Says("core.MemoryNegative",
			"{setting} %q is negative: write auto, off or a size such as 8gb", A("Value", s)))
	}
	n, err := ParseSize(raw)
	if err != nil {
		return Memory{}, aboutMemory(err)
	}
	if n == 0 {
		return Memory{}, RefuseAbout(SettingMemory, Says("core.MemoryZero",
			"{setting} %q leaves no room at all: write off for no limit, or a size such as 8gb", A("Value", s)))
	}
	return Memory{bytes: n}, nil
}

// ParseMemorySize is ParseMemory for a box that holds a size and nothing else
// - the window's Custom, where auto and off are chosen on the switch above it
// and a word typed into the box would be a second way to the same choice.
//
// Anything with no digit in it is refused in those words, the two words of the
// switch among them - ParseMemory would answer "lots" by offering auto and off,
// which is advice for a different box.
func ParseMemorySize(s string) (Memory, error) {
	if !strings.ContainsAny(s, "0123456789") {
		return Memory{}, RefuseAbout(SettingMemory, Says("core.MemoryNotASize",
			"{setting} %q is not a size: write one such as 8gb", A("Value", s)))
	}
	return ParseMemory(s)
}

// aboutMemory is a refusal of ParseSize, put in the memory slot. The size
// parser says what is wrong with the number better than a second sentence
// could, and it names its own setting, which here is the wrong one: the
// person typed --memory, and "size" would send them to a flag they did not
// write.
func aboutMemory(err error) error {
	var r *Refusal
	if errors.As(err, &r) {
		return &Refusal{said: r.said, setting: SettingMemory}
	}
	return err
}

// MemoryLimits is a setting worked out on one machine.
type MemoryLimits struct {
	// Run is what the run may hold while it writes, the plan among it. Zero
	// is no limit.
	Run int64
	// Plan is the ceiling on the plan. Zero is no ceiling.
	Plan int64
	// Unknown is why the machine could not be asked, when it could not. Auto
	// then works to MaxPlanBytes, the ceiling every run had before the
	// setting existed, and a run says so once rather than pretending the
	// number came from the machine.
	Unknown error
}

// On works the setting out against the memory of one machine, asked through
// machine - MachineMemory for a real run, a number for a guard.
//
// Auto is half, because a run is not alone on the machine and the plan and
// the writers share the one number. The plan's ceiling under auto never drops
// below MaxPlanBytes, so no run that was planned before this setting existed
// is refused now - on a machine of four gigabytes or less, auto gives the
// writers less than that and leaves the plan where it always was. A size is
// the person's own choice and is used as given, downwards too.
//
// machine is not called for a setting that does not need it, so a size and
// off never touch the system.
func (m Memory) On(machine func() (int64, error)) MemoryLimits {
	switch {
	case m.off:
		return MemoryLimits{}
	case m.bytes > 0:
		return MemoryLimits{Run: m.bytes, Plan: m.bytes}
	}
	total, err := machine()
	if err == nil && total <= 1 {
		// An answer, but not one about a machine that runs anything, so it is
		// treated as no answer rather than halved into a limit of nothing.
		err = Refuse(Says("core.MachineMemoryNone", "the system said this machine has %d B of memory", A("Bytes", total)))
	}
	if err != nil {
		return MemoryLimits{Run: MaxPlanBytes, Plan: MaxPlanBytes, Unknown: err}
	}
	half := total / 2
	return MemoryLimits{Run: half, Plan: max(half, MaxPlanBytes)}
}
