package guard

import (
	"context"
	"errors"
	"io"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// The files written at once hold no more than the memory setting leaves them,
// and the queue that keeps them there never refuses, never stalls a file that
// holds nothing and never starts a file after a stop -
// docs/STABILITY-REVIEW-2026-10-26.md sections 10.1, 10.4 and 10.8.
//
// Nothing here allocates the memory it talks about. What a file holds is its
// declaration (format.Plan.Holds), so the plan of four kilobyte text files is
// given declarations of a gigabyte or four and the queue is watched through a
// generator that counts the writers inside it. The room is set from the heap
// as it stands, so the test process's own few megabytes do not move the
// answer: two files of a gigabyte fit in two and a half, three do not.
//
// Four writers whatever the machine, so the answer does not depend on how many
// processors the runner has.
func TestTheFilesWrittenAtOnceHoldNoMoreThanTheMemorySettingLeaves(t *testing.T) {
	was := runtime.GOMAXPROCS(4)
	t.Cleanup(func() { runtime.GOMAXPROCS(was) })
	const gib = int64(1) << 30

	t.Run("two fit, and two are written at once", func(t *testing.T) {
		c := &crowd{waitForSecond: true}
		res, err := runCrowded(t, context.Background(), c, []int64{gib, gib, gib, gib, gib, gib}, 5*gib/2)
		if err != nil {
			t.Fatal(err)
		}
		if c.most > 2 {
			t.Errorf("%d files of a gigabyte were written at once in two and a half gigabytes of room", c.most)
		}
		if c.most < 2 {
			t.Errorf("only one file was written at a time with room for two - the queue holds the writers back further than the setting asks")
		}
		if len(res.RunNotes) != 0 {
			t.Errorf("a run that never went past the setting says it did: %v", res.RunNotes)
		}
	})

	t.Run("a file larger than all the room goes on its own and says so", func(t *testing.T) {
		c := &crowd{}
		res, err := runCrowded(t, context.Background(), c, []int64{4 * gib, 4 * gib, 4 * gib}, 5*gib/2)
		if err != nil {
			t.Fatal(err)
		}
		if c.calls != 3 {
			t.Fatalf("%d of three files were written - a file larger than the room has to be written, not refused and not left waiting", c.calls)
		}
		if c.most != 1 {
			t.Errorf("%d files larger than the room were written at once", c.most)
		}
		if len(res.RunNotes) != 1 || !strings.Contains(res.RunNotes[0].String(), "files_0001.txt") {
			t.Errorf("the run went past the setting and did not say so once, naming the first file: %v", res.RunNotes)
		}
	})

	t.Run("a file that holds nothing does not wait", func(t *testing.T) {
		c := &crowd{waitForSecond: true}
		if _, err := runCrowded(t, context.Background(), c, []int64{4 * gib, 0, 0, 0}, 5*gib/2); err != nil {
			t.Fatal(err)
		}
		if c.most < 2 {
			t.Errorf("text files waited behind a picture written on its own, though they hold no memory at all")
		}
	})

	t.Run("a stop while files wait starts none of them", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		c := &crowd{stopFirst: cancel}
		_, err := runCrowded(t, ctx, c, []int64{4 * gib, 4 * gib, 4 * gib, 4 * gib}, 5*gib/2)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("the run ended with %v rather than the stop", err)
		}
		if c.calls != 1 {
			t.Errorf("%d files were started - the ones waiting for room began writing after the run was stopped", c.calls)
		}
	})
}

// runCrowded plans one text file per declaration, gives each its declaration
// and the counting generator, and runs it with room bytes left for the writers.
func runCrowded(t *testing.T, ctx context.Context, c *crowd, holds []int64, room int64) (*engine.Result, error) {
	t.Helper()
	opt := engine.Options{OutDir: t.TempDir(), Seed: 7, Command: "test"}
	planned, err := engine.Plan([]engine.Target{{ID: "files", Format: "txt", Sizes: engine.Uniform(len(holds), 4096)}}, opt)
	if err != nil {
		t.Fatal(err)
	}
	for i := range planned {
		planned[i].Plan.Holds = holds[i]
		planned[i].Desc.Generator = crowdedGenerator{Generator: planned[i].Desc.Generator, c: c}
	}
	// The limit is the room on top of the heap as it is now, read the way the
	// engine reads it when it has to be exact.
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	opt.Memory = core.MemoryOf(int64(m.HeapAlloc) + room)
	return engine.Run(ctx, planned, opt)
}

// crowd counts the writers inside the generator: how many now, the most at
// once, and how many files were started at all.
type crowd struct {
	mu               sync.Mutex
	now, most, calls int
	waitForSecond    bool
	stopFirst        func()
}

// crowdedGenerator is the real generator with a count around it.
type crowdedGenerator struct {
	format.Generator
	c *crowd
}

func (g crowdedGenerator) Write(ctx context.Context, w io.Writer, p format.Plan) error {
	first := g.c.enter()
	defer g.c.leave()
	if first && g.c.stopFirst != nil {
		g.c.stopFirst()
		return ctx.Err()
	}
	// The first file waits for a second writer to join it, for at most a few
	// seconds. A queue that lets two in is answered in milliseconds. One that
	// lets only one in is answered by the deadline - the clock bounds a wait
	// here, it does not measure anything.
	if first && g.c.waitForSecond {
		g.c.waitForAnother(5 * time.Second)
	}
	return g.Generator.Write(ctx, w, p)
}

func (c *crowd) enter() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now++
	c.calls++
	c.most = max(c.most, c.now)
	return c.calls == 1
}

func (c *crowd) leave() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now--
}

func (c *crowd) waitForAnother(limit time.Duration) {
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		// The most at once rather than the number now: a text file is in
		// and out in microseconds, between two looks a millisecond apart.
		joined := c.most >= 2
		c.mu.Unlock()
		if joined {
			return
		}
		time.Sleep(time.Millisecond)
	}
}
