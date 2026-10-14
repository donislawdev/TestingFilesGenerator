package guard

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// A format that holds other files stops planning them when the run is stopped.
//
// The engine asks for a stop between one file and the next, and a container
// plans every file inside it from inside its own Plan - ten thousand small
// pngs in a page took 72 s on 2026-10-14, and as long again in a mail message,
// a zip and a tar.gz, with Ctrl+C waiting for the last of them (review of
// #178, docs/HTML-3-2026-10-14.md section 11).

// countdown is a context that is live for a number of questions and stopped
// from then on, so a stop lands part of the way through planning without a
// clock: a container that asks between its files sees it, and one that asks
// only before it starts does not.
type countdown struct {
	context.Context
	mu   sync.Mutex
	left int
	done chan struct{}
}

func newCountdown(left int) *countdown {
	return &countdown{Context: context.Background(), left: left, done: make(chan struct{})}
}

func (c *countdown) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.left > 0 {
		c.left--
		return nil
	}
	select {
	case <-c.done:
	default:
		close(c.done)
	}
	return context.Canceled
}

func (c *countdown) Done() <-chan struct{} { return c.done }

// containerRequests are the four formats that plan files inside them, each
// asked for forty, which a live context plans in well under a second.
func containerRequests() map[string]format.Request {
	forty := []format.Content{{Format: "txt", Count: 40, Bytes: 1024}}
	pictures := map[string]string{"inline": "40", "inline_size": "1kb"}
	return map[string]format.Request{
		"html":  {Bytes: 1 << 20, Seed: 7741, Label: true, Properties: pictures},
		"eml":   {Bytes: 1 << 20, Seed: 7741, Label: true, Properties: pictures},
		"zip":   {Bytes: 1 << 20, Seed: 7741, Label: true, Contains: forty},
		"targz": {Bytes: 1 << 20, Seed: 7741, Label: true, Contains: forty},
	}
}

// TestAContainerStopsBetweenTheFilesItPlans asks each container twice. With a
// live context it has to plan - otherwise the stop below could be a refusal of
// something else and pass for the thing guarded. With a context that stops
// after three questions it has to end with the stop itself, and NOT as a
// refusal: a window marks the box a refusal is about, and a stop is the answer
// to what was asked for rather than a fault in any box.
func TestAContainerStopsBetweenTheFilesItPlans(t *testing.T) {
	for id, r := range containerRequests() {
		d, err := format.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := d.Generator.(format.ContextPlanner); !ok {
			t.Errorf("%s plans files inside it and cannot be stopped while it does", id)
			continue
		}
		if _, err := format.PlanWith(context.Background(), d.Generator, r); err != nil {
			t.Errorf("%s: forty files inside are not planned with a live context, so the stop is not asked: %v", id, err)
			continue
		}
		_, err = format.PlanWith(newCountdown(3), d.Generator, r)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("%s: stopped after three questions and planning ended with %v, not the stop", id, err)
		}
		var refusal *core.Refusal
		if errors.As(err, &refusal) {
			t.Errorf("%s: a stop came back as a refusal, %q, so a window would mark a box for it", id, err)
		}
	}
}

// TestTheEngineHandsTheStopToAContainerItIsPlanning is the other half: the
// containers listen, and the engine has to be the one talking to them. A page
// of ten thousand small pictures takes over a minute to plan, so a run stopped
// a moment in has to end long before that, with the stop.
func TestTheEngineHandsTheStopToAContainerItIsPlanning(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	_, err := engine.PlanContext(ctx, []engine.Target{{
		ID: "g", Format: "html", Sizes: engine.Uniform(1, 1<<30), Label: true,
		Properties: map[string]string{"inline": "10000", "inline_size": "8kb"},
	}}, engine.Options{OutDir: t.TempDir(), Seed: 7741})
	took := time.Since(start)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("planning a page of ten thousand pictures, stopped after 200 ms, ended with %v after %v, not the stop", err, took)
	}
	if took > 20*time.Second {
		t.Errorf("planning stopped after 200 ms took %v to notice", took)
	}
}
