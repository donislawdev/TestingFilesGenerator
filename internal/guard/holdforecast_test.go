package guard

// A run that will hold more than its memory setting gives it says so before its
// first byte (O296, docs/FILE-CEILING-2026-11-02.md section 13).
//
// Measured before this existed: 600 000 text files in a container of 1 GB passed
// every check, wrote for fifteen minutes and were ended by the system, with no
// sentence and no manifest. The same run with Docker's default swap finished,
// which is why this is a note and not a refusal. A run the system ends prints
// nothing afterwards, so the command line has to say it before it writes.

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// holdsMore is the fixed start of the command line's sentence.
const holdsMore = "note: this run will hold about "

// The command line says it before the dry run's last line, and says nothing
// when the setting has room.
//
// One megabyte is less than this test process holds before the run starts, so
// the forecast is past it whatever else ran before, and twenty text files plan
// far under it, so the plan's own ceiling lets the run through to be warned.
// Sixty four gigabytes is more than any runner this suite meets.
func TestARunThatWillHoldMoreThanItsMemorySaysSoBeforeItsFirstByte(t *testing.T) {
	dry := func(memory string) string {
		return runCLI(t, "generate", "--format", "txt", "--size", "200b", "--count", "20",
			"--memory", memory, "--dry-run", "--out", t.TempDir())
	}
	const last = "dry run - nothing was written."

	said := dry("1mb")
	end := strings.Index(said, last)
	if end < 0 {
		t.Fatalf("the dry run under --memory 1mb was not accepted, so nothing here was asked about what it holds.\nIt said:\n%s", said)
	}
	at := strings.Index(said, holdsMore)
	if at < 0 {
		t.Fatalf("a run that will hold more than its 1 MiB said nothing about it.\n"+
			"A run past its memory setting is slower and may be ended by the system with no manifest, "+
			"and a run that is ended prints nothing afterwards.\nIt said:\n%s", said)
	}
	if at > end {
		t.Errorf("the note came after the run's last line rather than before its first byte:\n%s", said)
	}
	if !strings.Contains(said[at:], "1.0 MB the memory setting gives it") {
		t.Errorf("the note does not name the 1 MiB the setting gives the run:\n%s", said)
	}

	quiet := dry("64gb")
	if !strings.Contains(quiet, last) {
		t.Fatalf("the dry run under --memory 64gb was not accepted, so its silence proves nothing.\nIt said:\n%s", quiet)
	}
	if strings.Contains(quiet, holdsMore) {
		t.Errorf("a run with room under its setting was warned anyway, which teaches somebody to stop reading the line:\n%s", quiet)
	}
}

// The window says it after a preview and after a run, from the same forecast.
//
// The machine is the stand in host's, so auto is half of what the guard says
// it has: two megabytes, a megabyte for the run, which this process already
// holds more than. Left alone the host is a machine of sixteen gigabytes, and
// the same preview is quiet. The run is asked as well as the preview, because a
// preview is a button somebody may not press.
func TestTheWindowSaysWhenARunWillHoldMoreThanItsMemory(t *testing.T) {
	marker := text.RunHoldsMoreThanItsMemory("HOLDS", "ROOM")
	fixed := marker[:strings.Index(marker, "HOLDS")]
	written := text.Written(0)
	written = written[strings.LastIndex(written, " ")+1:]

	ask := func(machine int64, button, accepted string) string {
		content, w, host := screenInAWindowWithHost(t, text.TabOneTarget())
		host.machine = machine
		picker, ok := controlUnder(content, text.FieldFormat()).(*parts.Chooser)
		if !ok {
			t.Fatal("the format field is not a list to choose from, so this guard read the wrong tree")
		}
		picker.SetSelected("txt")
		fill(t, content, text.FieldSize(), "200b")
		fill(t, content, text.FieldCount(), "20")
		fill(t, content, text.FieldOutputDir(), t.TempDir())
		press(t, content, button)
		join(host)
		settle(content, w)
		_, status := runMessages(content)
		if status == nil {
			t.Fatal("the screen has no status line, so this guard read the wrong tree")
		}
		if !strings.Contains(status.Text, accepted) {
			t.Fatalf("%s was not accepted, so nothing here was asked about what the run holds.\nIt said:\n%s", button, status.Text)
		}
		return status.Text
	}

	if said := ask(2<<20, text.ButtonPreview(), text.AndNothingWrittenYet()); !strings.Contains(said, fixed) {
		t.Errorf("a preview on a machine of 2 MiB said nothing about the run holding more than its setting.\n"+
			"The command line says this before the first byte.\nIt said:\n%s", said)
	}
	if said := ask(2<<20, text.ButtonGenerate(), written); !strings.Contains(said, fixed) {
		t.Errorf("a run on a machine of 2 MiB, never previewed, said nothing about holding more than its setting:\n%s", said)
	}
	if said := ask(0, text.ButtonPreview(), text.AndNothingWrittenYet()); strings.Contains(said, fixed) {
		t.Errorf("a preview on a machine of sixteen gigabytes was warned anyway:\n%s", said)
	}
}

// What the forecast counts for a file is what a file adds to the run's memory.
//
// The forecast carries one number for every file, measured with the runsize
// probe. A number typed beside the code and compared with nothing goes stale in
// silence, so this measures it again the way the probe did - the live heap after
// the run against the live heap before it, the plan and the result both held -
// and asks the forecast what it adds for the same files.
//
// In a process of its own. The heap of this package's process is shared by
// every guard that ran before (O162), and in a batch of eighty eight of them the
// same 2000 files measured 199 B a file against 534 alone - something freed
// between the two readings. A small run first, so whatever a first run builds
// once is not counted against the files.
func TestAFileAddsWhatTheForecastSaysToTheRunsMemory(t *testing.T) {
	const name = "TestAFileAddsWhatTheForecastSaysToTheRunsMemory"
	if os.Getenv(holdChild) != "1" {
		run := exec.Command(os.Args[0], "-test.run=^"+name+"$", "-test.v")
		run.Env = append(os.Environ(), holdChild+"=1")
		out, err := run.CombinedOutput()
		if err != nil || !strings.Contains(string(out), "--- PASS: "+name) {
			t.Fatalf("%s did not pass in its own process (%v):\n%s", name, err, out)
		}
		return
	}

	const files = 2000
	opt := engine.Options{OutDir: t.TempDir(), Seed: 1, Command: "test", Memory: core.MemoryOff()}
	warm := opt
	warm.OutDir = t.TempDir()
	if _, err := engine.Run(context.Background(), planOf(t, 10, warm), warm); err != nil {
		t.Fatalf("the run before the measured one: %v", err)
	}
	planned := planOf(t, files, opt)

	before := liveHeap()
	res, err := engine.Run(context.Background(), planned, opt)
	if err != nil {
		t.Fatalf("the run: %v", err)
	}
	after := liveHeap()
	runtime.KeepAlive(planned)
	if len(res.Manifest.Files) != files {
		t.Fatalf("the run recorded %d files of %d, so this did not measure what a run of them holds", len(res.Manifest.Files), files)
	}
	measured := float64(int64(after)-int64(before)) / files

	// A setting far over this process, so the forecast is worked out rather
	// than left at nought, and a million files so that what it adds outweighs
	// anything the heap does between the two readings.
	room := engine.Options{Memory: core.MemoryOf(1 << 50)}
	runtime.GC()
	none := engine.PredictHold(0, room).Holds
	runtime.GC()
	million := engine.PredictHold(1_000_000, room).Holds
	forecast := float64(million-none) / 1_000_000

	if ratio := forecast / measured; ratio < 0.7 || ratio > 1.4 {
		t.Errorf("the forecast counts %.0f B for a file and a run of %d text files added %.0f B a file.\n"+
			"Measure it again with tools/probes/runsize and write the number in internal/engine/holdforecast.go.",
			forecast, files, measured)
	}

	// And past the cheap reading: a setting 256 MiB over this heap has room
	// for no files and none for a million, so the second reading, after a
	// collection, is the one that answers - and it has to count the files too.
	tight := engine.Options{Memory: core.MemoryOf(none + 256<<20)}
	if engine.PredictHold(0, tight).Over {
		t.Fatalf("a run of no files under a setting 256 MiB over this heap was judged over it, so the half below proves nothing")
	}
	if !engine.PredictHold(1_000_000, tight).Over {
		t.Errorf("a million files, about 500 MB of entries, under a setting 256 MiB over this heap were not judged over it")
	}
}

// holdChild puts the measurement above in a process of its own.
const holdChild = "TFG_HOLD_FORECAST_CHILD"

// planOf plans count text files of 200 B.
func planOf(t *testing.T, count int, opt engine.Options) []engine.PlannedFile {
	t.Helper()
	planned, err := engine.Plan([]engine.Target{txtTarget("t", count, 200)}, opt)
	if err != nil {
		t.Fatalf("planning %d files: %v", count, err)
	}
	return planned
}

// liveHeap is the heap after a collection.
func liveHeap() uint64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}
