// The clock: what time each entry says it happened.
//
// Until 2026-08-31 every entry in every log this tool wrote carried the same
// instant, because the timestamp was a constant. That is a fidelity defect
// rather than a missing setting, and docs/BACKLOG.md said so: a log where ten
// thousand requests happen at one instant cannot be used to test a time window
// query, a rate alert, or anything that rotates.
//
// It advances now, and that moves the bytes of every log, so `timestamps=fixed`
// is kept as the way back and reproduces the old file exactly. Same shape as
// `frames=1` for the still GIF, and there is a pinned hash for it too.
package logfile

import "time"

// epoch is where every log starts. A constant, because D11 promises the same
// bytes from the same seed and time.Now() would promise the opposite.
//
// It is the instant the fixed timestamp used to carry, so a run with
// timestamps=fixed writes the bytes this tool wrote before this file existed.
var epoch = time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)

const (
	// apacheTime is the layout the Apache family and nginx write, and
	// isoTime is what rsyslog and application logs write. Both are fixed
	// width for every instant with a four digit year, which is what lets an
	// entry's length be known before it is built.
	//
	// Measured on a real nginx and a real rsyslog on 2026-08-31 rather than
	// recalled - see docs/MVP-FORMATS.md.
	apacheTime = "02/Jan/2006:15:04:05 -0700"
	isoTime    = "2006-01-02T15:04:05.000000-07:00"
)

// clock hands out the instant for each entry in turn.
//
// It counts, so it is one of the builders core.Record.Discard exists for: the
// filler builds one entry past the end to measure it and throws it away, and
// without a way back the file would skip a tick. csv counts rows the same way.
type clock struct {
	// step is how far the clock moves between entries. Zero holds it still,
	// which is what timestamps=fixed asks for.
	step time.Duration
	at   time.Time
}

func newClock(o options) clock {
	c := clock{at: epoch}
	if o.advancing {
		// Entries per second into a gap between entries. Integer division
		// rounds towards zero, so a rate above one second per entry still
		// moves - a step of nought would silently reproduce fixed.
		c.step = time.Second / time.Duration(o.rate)
		if c.step <= 0 {
			c.step = time.Nanosecond
		}
	}
	return c
}

// tick returns the instant for this entry and moves on.
func (c *clock) tick() time.Time {
	at := c.at
	c.at = c.at.Add(c.step)
	return at
}

// back undoes one tick, for the entry that was built only to measure it.
func (c *clock) back() { c.at = c.at.Add(-c.step) }
