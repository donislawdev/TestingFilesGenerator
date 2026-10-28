package guard

import (
	"errors"
	"strings"
	"testing"

	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// The Memory section of the Preferences screen - the window's --memory,
// docs/STABILITY-REVIEW-2026-10-26.md section 10.8.
//
// Chosen on a switch, kept at once the way the language is, and read by the
// next run. The machine is the stand in host's sixteen gigabytes, so Automatic
// reads eight on every runner.
func TestTheMemorySettingIsChosenOnPreferencesAndKept(t *testing.T) {
	host := newFakeHost(t)
	window.Open(host)
	prefs := selectTab(t, host.content, text.TabPreferences())
	way, size := memoryControls(t, prefs)

	// Nothing kept: Automatic, and what it comes to on this machine.
	if way.Selected != text.MemoryWayAuto() || host.Remembered().Memory() != "" {
		t.Fatalf("with nothing kept the switch shows %q and %q is kept, not Automatic and nothing", way.Selected, host.Remembered().Memory())
	}
	if said := textIn(prefs); !strings.Contains(said, text.MemoryAutoIs(text.HumanBytes(8<<30))) {
		t.Errorf("the screen does not say what Automatic comes to on a machine of sixteen gigabytes:\n%s", said)
	}
	// And the floor the plan keeps under it, from the constant that sets it -
	// review on #208: without it a machine of four gigabytes or less reads a
	// refusal the run would never give.
	if said := textIn(prefs); !strings.Contains(said, "a plan of up to "+text.HumanBytes(core.MaxPlanBytes)+" is never refused") {
		t.Errorf("the screen does not say the floor of the plan under Automatic:\n%s", said)
	}

	// Custom keeps the size as it is typed, and a size that is not one is
	// marked and kept as Automatic rather than as what was typed.
	way.SetSelected(text.MemoryWayCustom())
	size.SetText("512mb")
	if got := host.Remembered().Memory(); got != "512mb" {
		t.Errorf("512mb was typed under Custom and %q was kept", got)
	}
	size.SetText("lots")
	if got := host.Remembered().Memory(); got != "" {
		t.Errorf("%q is not a size and %q was kept for it", "lots", got)
	}
	// The refusal is asked for by its words, not by the value: "lots" is on
	// the screen anyway, in the box it was typed into.
	if said := textIn(prefs); !strings.Contains(said, text.MemoryCustomEmpty()) || !strings.Contains(said, "is not a size") {
		t.Errorf("a box holding something that is not a size is not marked, or the line does not say runs use Automatic:\n%s", said)
	}
	size.SetText("off")
	if got := host.Remembered().Memory(); got != "" {
		t.Errorf("off typed into the box for a size was kept as %q - No limit is chosen on the switch, not typed", got)
	}

	way.SetSelected(text.MemoryWayOff())
	if got := host.Remembered().Memory(); got != "off" {
		t.Errorf("No limit was chosen and %q was kept", got)
	}
	way.SetSelected(text.MemoryWayAuto())
	if got := host.Remembered().Memory(); got != "" {
		t.Errorf("Automatic was chosen and %q stayed kept", got)
	}

	// Forget takes it away with the rest, and the switch goes back with it.
	// Kept as a size, so the box holds one that reads - and the size goes with
	// the rest: a box left holding it would keep it again the moment Custom is
	// chosen after Forget.
	way.SetSelected(text.MemoryWayCustom())
	size.SetText("512mb")
	if got := host.Remembered().Memory(); got != "512mb" {
		t.Fatalf("512mb was typed under Custom and %q was kept", got)
	}
	buttonNamed(prefs, text.ButtonForget()).OnTapped()
	if got := host.Remembered().Memory(); got != "" || way.Selected != text.MemoryWayAuto() {
		t.Errorf("after Forget %q is kept and the switch shows %q", got, way.Selected)
	}
	way.SetSelected(text.MemoryWayCustom())
	if got := host.Remembered().Memory(); got != "" || size.Text != "" {
		t.Errorf("Custom chosen after Forget keeps %q with %q in the box - the size Forget took away came back", got, size.Text)
	}
}

// Opening the screen writes nothing, and a kept value that does not read is
// shown as Automatic and named rather than written over - untouchable rule 7,
// as for a language this build does not carry.
func TestOpeningPreferencesKeepsAnUnreadableMemorySetting(t *testing.T) {
	host := newFakeHost(t)
	host.Remembered().RememberMemory("lots")
	window.Open(host)
	prefs := selectTab(t, host.content, text.TabPreferences())
	way, _ := memoryControls(t, prefs)
	if got := host.Remembered().Memory(); got != "lots" {
		t.Errorf("a kept value of lots became %q by opening the window", got)
	}
	if way.Selected != text.MemoryWayAuto() {
		t.Errorf("an unreadable kept value shows as %q, and a run uses Automatic for it", way.Selected)
	}
	if said := textIn(prefs); !strings.Contains(said, text.MemoryUnreadable("lots")) {
		t.Errorf("the screen does not say the kept value could not be read:\n%s", said)
	}

	// And a size kept before opens on Custom with the size in the box.
	host = newFakeHost(t)
	host.Remembered().RememberMemory("3gb")
	window.Open(host)
	prefs = selectTab(t, host.content, text.TabPreferences())
	way, size := memoryControls(t, prefs)
	if way.Selected != text.MemoryWayCustom() || size.Text != "3gb" || host.Remembered().Memory() != "3gb" {
		t.Errorf("3gb kept opens as %q with %q in the box and %q kept", way.Selected, size.Text, host.Remembered().Memory())
	}
}

// The setting kept on Preferences is the one the next run plans under.
//
// A ceiling of one megabyte against twenty thousand text files, about 17 MB of
// plan, from the Generate screen's preview: the refusal has to be the memory
// setting's, which only a runner that read the kept value can give.
func TestARunPlansUnderTheMemorySettingKeptOnPreferences(t *testing.T) {
	host := newFakeHost(t)
	host.Remembered().RememberMemory("1mb")
	window.Open(host)
	t.Cleanup(func() { join(host) })
	content := tabNamed(t, host.content, text.TabOneTarget())
	fill(t, content, text.FieldOutputDir(), t.TempDir())
	fill(t, content, text.FieldSize(), "4kb")
	fill(t, content, text.FieldCount(), "20000")
	ceiling := "the memory setting of this run gives it " + text.HumanBytes(1<<20)
	press(t, content, text.ButtonPreview())
	join(host)
	if said := textIn(content); !strings.Contains(said, ceiling) {
		t.Errorf("a preview of 17 MB of plan under a kept setting of 1 MB was not refused by the setting:\n%s", said)
	}
	// The run reads it as well as the preview - they start apart, and each
	// reads the setting for itself.
	press(t, content, text.ButtonGenerate())
	join(host)
	if said := textIn(content); !strings.Contains(said, ceiling) {
		t.Errorf("a run of 17 MB of plan under a kept setting of 1 MB was not refused by the setting:\n%s", said)
	}
}

// What a run says about the memory it ran in reaches the window, from a
// preview and from a run - review on #208. The command line has said it since
// the setting existed, and the window said only what the run said about its
// files, so a run that could not ask the machine worked to a limit nobody on
// the screen was told the source of (untouchable rule 6).
//
// Asked on a host that cannot say how much memory the machine has, which is
// also the proof that a run asks the same host the Preferences screen asks.
func TestTheWindowSaysWhatARunSaidAboutItsMemory(t *testing.T) {
	host := newFakeHost(t)
	host.machineFails = true
	window.Open(host)
	t.Cleanup(func() { join(host) })
	content := tabNamed(t, host.content, text.TabOneTarget())
	fill(t, content, text.FieldOutputDir(), t.TempDir())
	fill(t, content, text.FieldSize(), "1kb")
	fill(t, content, text.FieldCount(), "2")
	const said = "the memory of this machine could not be read"
	for _, button := range []string{text.ButtonPreview(), text.ButtonGenerate()} {
		press(t, content, button)
		join(host)
		if on := textIn(content); !strings.Contains(strings.ToLower(on), said) {
			t.Errorf("%s on a machine that could not be asked does not say so:\n%s", button, on)
		}
	}
}

// memoryControls are the switch and the box of the Memory section. The screen
// holds no other switch and no other box, which is asserted rather than
// assumed, or the guard could be pressing a control of another section.
func memoryControls(t *testing.T, o fyne.CanvasObject) (*parts.Segments, *parts.Entry) {
	t.Helper()
	var ways []*parts.Segments
	var boxes []*parts.Entry
	walk(o, func(obj fyne.CanvasObject) {
		switch c := obj.(type) {
		case *parts.Segments:
			ways = append(ways, c)
		case *parts.Entry:
			boxes = append(boxes, c)
		}
	})
	if len(ways) != 1 || len(boxes) != 1 {
		t.Fatalf("the Preferences screen holds %d switches and %d boxes, expected the one of each the Memory section has", len(ways), len(boxes))
	}
	return ways[0], boxes[0]
}

// MachineMemory is a machine of sixteen gigabytes unless a guard said
// otherwise - never the memory of the runner the guard is on, or the stored
// picture of the Preferences screen would differ from one runner to the next.
func (h *fakeHost) MachineMemory() (int64, error) {
	switch {
	case h.machineFails:
		return 0, errors.ErrUnsupported
	case h.machine > 0:
		return h.machine, nil
	}
	return fakeMachineMemory, nil
}

const fakeMachineMemory = 16 << 30
