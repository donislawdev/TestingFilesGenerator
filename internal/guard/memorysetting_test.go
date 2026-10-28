package guard

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// The memory setting reads what somebody wrote and nothing else -
// docs/STABILITY-REVIEW-2026-10-26.md section 10.8.
//
// One parser for both surfaces, so this table is the answer for the flag and
// for the box on the Preferences screen alike. The refusals name the memory
// setting rather than the size, because the size parser underneath names its
// own and that would send somebody to a flag they did not type.
func TestTheMemorySettingReadsWhatItSays(t *testing.T) {
	type want struct {
		auto, off bool
		bytes     int64
	}
	for _, c := range []struct {
		text string
		want want
	}{
		{"auto", want{auto: true}},
		{" AUTO ", want{auto: true}},
		{"off", want{off: true}},
		{"Off", want{off: true}},
		{"8gb", want{bytes: 8 << 30}},
		{"1.5gib", want{bytes: 1536 << 20}},
		{"512MB", want{bytes: 512 << 20}},
		{"1", want{bytes: 1}},
		{"1048576", want{bytes: 1 << 20}},
	} {
		got, err := core.ParseMemory(c.text)
		if err != nil {
			t.Errorf("%q was refused: %v", c.text, err)
			continue
		}
		if got.IsAuto() != c.want.auto || got.IsOff() != c.want.off || got.Bytes() != c.want.bytes {
			t.Errorf("%q read as auto=%v off=%v bytes=%d, expected auto=%v off=%v bytes=%d",
				c.text, got.IsAuto(), got.IsOff(), got.Bytes(), c.want.auto, c.want.off, c.want.bytes)
		}
	}

	for _, c := range []struct {
		text string
		// says is a piece of the sentence that has to be there - the reason in
		// the person's terms, not the parser's.
		says string
	}{
		{"", "empty memory"},
		{"   ", "empty memory"},
		{"0", "leaves no room at all"},
		{"0gb", "leaves no room at all"},
		{"0.0mb", "leaves no room at all"},
		{"-1", "is negative"},
		{"-8gb", "is negative"},
		{"max", "not a setting this tool knows"},
		{"unlimited", "not a setting this tool knows"},
		{"8 gigs", "unknown unit"},
		{"1e5", "not a plain number"},
		{"1.3", "memory"},
	} {
		_, err := core.ParseMemory(c.text)
		if err == nil {
			t.Errorf("%q was accepted", c.text)
			continue
		}
		var about interface{ AboutSetting() string }
		if !errors.As(err, &about) || about.AboutSetting() != core.SettingMemory {
			t.Errorf("%q was refused about %q rather than about %q, so a window marks the wrong box: %v",
				c.text, settingOf(err), core.SettingMemory, err)
		}
		msg := err.Error()
		if !strings.Contains(msg, c.says) {
			t.Errorf("%q was refused with %q, which does not say %q", c.text, msg, c.says)
		}
		// The size parser's own words for its own setting must not come
		// through: "size" names a flag this person did not type, and "a file
		// cannot be smaller than zero bytes" is about a file.
		if strings.HasPrefix(msg, "size ") || strings.Contains(msg, " size \"") || strings.Contains(msg, "a file cannot") {
			t.Errorf("%q was refused in the size parser's words: %q", c.text, msg)
		}
	}
}

func settingOf(err error) string {
	var about interface{ AboutSetting() string }
	if errors.As(err, &about) {
		return about.AboutSetting()
	}
	return ""
}

// The text a setting is kept as reads back as the same setting, to the byte.
//
// The window keeps the setting in preferences.json as this text and reads it
// at the next start, so a size that came back rounded would be a setting
// changed without anybody touching it.
func TestTheMemorySettingReadsBackAsItself(t *testing.T) {
	for _, m := range []core.Memory{
		{}, core.MemoryOff(),
		core.MemoryOf(1), core.MemoryOf(1023), core.MemoryOf(1024), core.MemoryOf(1025),
		core.MemoryOf(1536 << 20), core.MemoryOf(8 << 30), core.MemoryOf(3 << 40),
		core.MemoryOf(math.MaxInt64),
	} {
		back, err := core.ParseMemory(m.String())
		if err != nil {
			t.Errorf("%v (%d B) is kept as %q, which reads back as a refusal: %v", m, m.Bytes(), m.String(), err)
			continue
		}
		if back != m {
			t.Errorf("%d B is kept as %q and reads back as %d B", m.Bytes(), m.String(), back.Bytes())
		}
	}
}

// What each setting comes to on a machine of a given size.
//
// The machine is a number here rather than the one the test runs on, so the
// arithmetic is checked on the machines the decisions were made about: half
// for auto, never under the old plan ceiling for the plan, a size used as given
// in both directions, and off as nothing at all. The decisions are the owner's
// of 2026-10-27, docs/STABILITY-REVIEW-2026-10-26.md section 10.10.
func TestTheMemorySettingIsWorkedOutAgainstTheMachine(t *testing.T) {
	const gib = int64(1) << 30
	machine := func(n int64, err error) (func() (int64, error), *int) {
		calls := new(int)
		return func() (int64, error) { *calls++; return n, err }, calls
	}

	for _, c := range []struct {
		name      string
		setting   core.Memory
		machine   int64
		fails     bool
		run, plan int64
		unknown   bool
		asks      bool
	}{
		{"auto on 32 GiB", core.Memory{}, 32 * gib, false, 16 * gib, 16 * gib, false, true},
		{"auto on 7 GiB, a CI runner", core.Memory{}, 7 * gib, false, 7 * gib / 2, 7 * gib / 2, false, true},
		{"auto on 2 GiB keeps the old plan ceiling", core.Memory{}, 2 * gib, false, gib, 2 * gib, false, true},
		{"auto on a machine that cannot be asked", core.Memory{}, 0, true, 2 * gib, 2 * gib, true, true},
		{"auto on an answer of nothing", core.Memory{}, 0, false, 2 * gib, 2 * gib, true, true},
		{"auto on an answer of one byte", core.Memory{}, 1, false, 2 * gib, 2 * gib, true, true},
		{"a size above the machine", core.MemoryOf(64 * gib), 8 * gib, false, 64 * gib, 64 * gib, false, false},
		{"a size below the old plan ceiling", core.MemoryOf(100 << 20), 32 * gib, false, 100 << 20, 100 << 20, false, false},
		{"off", core.MemoryOff(), 32 * gib, false, 0, 0, false, false},
	} {
		var fail error
		if c.fails {
			fail = errors.ErrUnsupported
		}
		ask, calls := machine(c.machine, fail)
		got := c.setting.On(ask)
		if got.Run != c.run || got.Plan != c.plan || (got.Unknown != nil) != c.unknown {
			t.Errorf("%s: run %d plan %d unknown %v, expected run %d plan %d unknown %v",
				c.name, got.Run, got.Plan, got.Unknown, c.run, c.plan, c.unknown)
		}
		if (*calls > 0) != c.asks {
			t.Errorf("%s: the machine was asked %d times - a setting that does not need it must not touch the system, and auto must", c.name, *calls)
		}
	}
}
