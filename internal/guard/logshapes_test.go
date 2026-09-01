package guard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// logShapes is every shape the format offers, taken from the registry rather
// than written out here. A list copied into a guard stops describing the thing
// it guards the moment somebody adds to the registry, and this project has
// been caught by that before.
func logShapes(t *testing.T) []string {
	t.Helper()
	d, err := format.Get("log")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range d.Properties {
		if p.Name == "entry_format" {
			if len(p.Choices) == 0 {
				t.Fatal("entry_format declares no choices, so this guard would check nothing")
			}
			return p.Choices
		}
	}
	t.Fatal("the log format declares no entry_format, so there are no shapes to walk")
	return nil
}

func writeLog(t *testing.T, size int64, props map[string]string) []byte {
	t.Helper()
	d, err := format.Get("log")
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Generator.Plan(format.Request{Bytes: size, Seed: 7741, Label: true, Properties: props})
	if err != nil {
		t.Fatalf("planning %d B with %v: %v", size, props, err)
	}
	var buf bytes.Buffer
	if err := d.Generator.Write(context.Background(), &buf, p); err != nil {
		t.Fatalf("writing %d B with %v: %v", size, props, err)
	}
	if int64(buf.Len()) != size {
		t.Fatalf("%v: asked for %d B and got %d", props, size, buf.Len())
	}
	return buf.Bytes()
}

// Every shape writes whole lines, and hits the size to the byte.
//
// A log is read line by line, so a file that is the right length but ends in
// half an entry is a broken file. Each shape reaches its length by stretching
// one field of the last entry - a request path for the web shapes, a message
// for the rest - and each has its own arithmetic to get that wrong in.
//
// The sizes cross the awkward places: one byte above the minimum, an odd size,
// and sizes where the closing entry has to be much longer than a natural one.
func TestEveryLogShapeWritesWholeLinesAtTheRightSize(t *testing.T) {
	for _, shape := range logShapes(t) {
		t.Run(shape, func(t *testing.T) {
			props := map[string]string{"entry_format": shape}
			for _, size := range []int64{300, 301, 512, 1000, 4096, 4097, 65537} {
				body := writeLog(t, size, props)
				if !bytes.HasSuffix(body, []byte("\n")) {
					t.Errorf("%d B: the file does not end with a newline, so the last entry is unterminated", size)
					continue
				}
				for i, line := range strings.Split(strings.TrimRight(string(body), "\n"), "\n") {
					if strings.TrimSpace(line) == "" {
						t.Errorf("%d B: line %d is empty", size, i+1)
					}
				}
			}
		})
	}
}

// Every shape hits the size across seeds, not just on the sizes somebody tried.
//
// This is the guard that would have caught the defect it was written after, and
// the shape of that defect is the reason it sweeps seeds rather than sizes. The
// syslog line counted its process id as four digits always, when it runs from
// 100 to 9998 and so is three about one time in ten. Only the LAST entry of a
// file is built to a length, so the miss needed the last entry to draw a short
// pid - one file in ten, which a handful of hand picked sizes walks straight
// past. Measured before the repair: 35 files out of 360, syslog alone.
//
// The project's own size guard never saw it either, because it asks each format
// with its settings left alone, and left alone this one is Apache combined.
func TestEveryLogShapeHitsTheSizeAcrossSeeds(t *testing.T) {
	d, err := format.Get("log")
	if err != nil {
		t.Fatal(err)
	}
	sizes := []int64{400, 512, 777, 1000, 2048, 4097}

	for _, shape := range logShapes(t) {
		t.Run(shape, func(t *testing.T) {
			checked := 0
			for seed := uint64(0); seed < 40; seed++ {
				for _, size := range sizes {
					p, err := d.Generator.Plan(format.Request{Bytes: size, Seed: seed, Label: true,
						Properties: map[string]string{"entry_format": shape}})
					if err != nil {
						continue // below this shape's own minimum, which it names
					}
					var buf bytes.Buffer
					if err := d.Generator.Write(context.Background(), &buf, p); err != nil {
						t.Fatalf("seed %d, %d B: %v", seed, size, err)
					}
					if int64(buf.Len()) != size {
						t.Fatalf("seed %d: asked for %d B and got %d.\n"+
							"One field of the closing entry is stretched to reach the length, so its arithmetic is out by %d for this draw.",
							seed, size, buf.Len(), int64(buf.Len())-size)
					}
					checked++
				}
			}
			if checked < 100 {
				t.Fatalf("only %d files were produced, too few for a sweep to mean anything", checked)
			}
		})
	}
}

// Every shape can be reached with EVERY setting sent, which is how a window
// asks.
//
// This is the guard the reported defect needed and did not have. A menu cannot
// be empty: it opens on its declared default, so the window sends a value for
// every setting the format declares, touched or not. The first version of the
// log settings refused a setting that could do nothing for the chosen shape
// whenever the KEY arrived - so from the window, where methods always arrives,
// syslog and JSON lines could not be produced at all. Every attempt came back
// refused for a setting nobody had touched.
//
// The command line never showed it, because there an unset flag is an absent
// key, and every test written before the report used the command line's shape
// of a request. Reported from a screenshot on 2026-08-31.
//
// So the request here is built the way a window builds one: every declared
// setting, each at its declared default.
func TestEveryLogShapeIsReachableWithEverySettingSent(t *testing.T) {
	d, err := format.Get("log")
	if err != nil {
		t.Fatal(err)
	}

	for _, shape := range logShapes(t) {
		t.Run(shape, func(t *testing.T) {
			props := map[string]string{}
			for _, p := range d.Properties {
				if p.Default != "" {
					props[p.Name] = p.Default
				}
			}
			props["entry_format"] = shape
			if len(props) < 2 {
				t.Fatal("no declared defaults were sent, so this guard is not asking what a window asks")
			}

			plan, err := d.Generator.Plan(format.Request{Bytes: 20 << 10, Seed: 7741, Label: true, Properties: props})
			if err != nil {
				t.Fatalf("a window sending every setting at its default cannot produce %s: %v\n"+
					"A default that arrived is not a setting anybody asked for, so it cannot disagree with the shape.",
					shape, err)
			}
			var buf bytes.Buffer
			if err := d.Generator.Write(context.Background(), &buf, plan); err != nil {
				t.Fatalf("%s planned and then would not write: %v", shape, err)
			}
			if buf.Len() != 20<<10 {
				t.Fatalf("%s came out %d B rather than %d", shape, buf.Len(), 20<<10)
			}
		})
	}
}

// The clock advances, and every tick is there.
//
// Until 2026-08-31 every entry carried the same instant, which docs/BACKLOG.md
// recorded as a fidelity defect rather than a missing setting: a log where ten
// thousand requests happen at once cannot test a time window or a rate alert.
//
// Two things are asked, and they fail differently. That the instants MOVE is
// the defect itself coming back. That they move by exactly one step each time,
// with no gap, is the record builder putting the clock back for the entry it
// built only to measure and then threw away - miss that and the file skips a
// second before its last line, which nothing else here would see, because the
// size stays exact and every line still parses.
func TestALogClockAdvancesByOneTickPerEntry(t *testing.T) {
	body := string(writeLog(t, 4096, map[string]string{"rate": "1"}))

	stamp := regexp.MustCompile(`\[([^\]]+)\]`)
	var seen []time.Time
	for _, line := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
		if strings.HasPrefix(line, "# ") {
			continue
		}
		m := stamp.FindStringSubmatch(line)
		if m == nil {
			t.Fatalf("no timestamp in %q", line)
		}
		at, err := time.Parse("02/Jan/2006:15:04:05 -0700", m[1])
		if err != nil {
			t.Fatalf("unparseable timestamp %q: %v", m[1], err)
		}
		seen = append(seen, at)
	}
	if len(seen) < 3 {
		t.Fatalf("only %d entries, too few to say anything about the clock", len(seen))
	}

	if seen[0].Equal(seen[len(seen)-1]) {
		t.Errorf("every entry carries %s, so the clock is not advancing at all.\n"+
			"That is the defect docs/BACKLOG.md recorded: a log where everything happens at one instant.",
			seen[0].Format(time.RFC3339))
	}
	for i := 1; i < len(seen); i++ {
		if gap := seen[i].Sub(seen[i-1]); gap != time.Second {
			t.Errorf("entry %d is %s after the one before it, and at one entry a second every gap should be 1s.\n"+
				"A gap of two means a tick was spent on the entry that was built to be measured and thrown away, and not put back.",
				i+1, gap)
			break
		}
	}
}

// The way back is exact, and it is a property of the file rather than a promise.
//
// Advancing the clock moved the bytes of every log this tool writes, so
// timestamps=fixed exists to reproduce the old file. The pinned hash for that
// lives with the other golden values - this asks the cheaper question, which is
// whether the clock is held still at all.
func TestALogWithFixedTimestampsHoldsOneInstant(t *testing.T) {
	body := string(writeLog(t, 4096, map[string]string{"timestamps": "fixed"}))

	stamp := regexp.MustCompile(`\[([^\]]+)\]`)
	first, count := "", 0
	for _, line := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
		if strings.HasPrefix(line, "# ") {
			continue
		}
		m := stamp.FindStringSubmatch(line)
		if m == nil {
			t.Fatalf("no timestamp in %q", line)
		}
		if first == "" {
			first = m[1]
		} else if m[1] != first {
			t.Fatalf("timestamps=fixed produced %q and then %q, so it is not fixed", first, m[1])
		}
		count++
	}
	if count < 3 {
		t.Fatalf("only %d entries, too few to say anything", count)
	}
}

// A setting that would do nothing is refused, not ignored.
//
// Most of these only mean something for some shapes: a request method has no
// place in a syslog line, and a rate has none while the clock is held still.
// Taking them and quietly doing nothing is the silence rule 6 forbids - the
// recipe would say one thing and the file would be another.
//
// The refusal has to name BOTH settings. Naming one leaves the reader to guess
// which of the pair to change, and either could be the one they meant.
func TestALogRefusesASettingThatWouldChangeNothing(t *testing.T) {
	d, err := format.Get("log")
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct{ props map[string]string }{
		{map[string]string{"entry_format": "syslog", "methods": "mixed"}},
		{map[string]string{"entry_format": "syslog", "ip_version": "v6"}},
		{map[string]string{"entry_format": "plain", "status_mix": "success"}},
		{map[string]string{"timestamps": "fixed", "rate": "5"}},
		{map[string]string{"entry_format": "nginx", "level_mix": "quiet"}},
		{map[string]string{"entry_format": "syslog", "level_mix": "errors"}},
	}
	for _, c := range cases {
		t.Run(fmt.Sprint(c.props), func(t *testing.T) {
			_, err := d.Generator.Plan(format.Request{Bytes: 4096, Seed: 7741, Label: true, Properties: c.props})
			if err == nil {
				t.Fatalf("%v was accepted, and one of those two settings then does nothing at all", c.props)
			}
			for key := range c.props {
				if !strings.Contains(err.Error(), key) {
					t.Errorf("the refusal does not name %q, so it says which half of the pair is wrong only by luck: %v", key, err)
				}
			}
		})
	}

	// And the pair that is NOT a conflict, so the guard cannot pass by
	// refusing everything: JSON lines carry a response code of their own.
	if _, err := d.Generator.Plan(format.Request{Bytes: 4096, Seed: 7741, Label: true,
		Properties: map[string]string{"entry_format": "json-lines", "status_mix": "success"}}); err != nil {
		t.Errorf("json-lines does carry a status, so status_mix belongs there: %v", err)
	}
}

// The label is a line the shape's own reader accepts.
//
// Every line has to be a whole record, and a hash comment is not one in JSON
// lines - it is the single line in the file that no reader would load. So the
// carrier changes with the shape, and this asks the readers rather than the
// code: every line of a JSON log parses as an object, the label included.
func TestTheLogLabelIsALineItsOwnReaderAccepts(t *testing.T) {
	body := writeLog(t, 4096, map[string]string{"entry_format": "json-lines"})

	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if len(lines) < 3 {
		t.Fatalf("only %d lines, too few to say anything", len(lines))
	}
	for i, line := range lines {
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Fatalf("line %d is not a JSON object, so a reader stops there: %v\n  %s", i+1, err, line)
		}
	}
	if !strings.Contains(lines[0], `"label"`) {
		t.Errorf("the first line carries no label field, so nothing in the file says what it is: %s", lines[0])
	}

	// The shapes whose readers do treat a hash as a comment keep it.
	hashed := writeLog(t, 4096, map[string]string{"entry_format": "syslog"})
	if !bytes.HasPrefix(hashed, []byte("# ")) {
		t.Errorf("the syslog label is not a hash comment, so the one line that is not an entry does not say so")
	}
}

// The severity mix is the set that gets drawn, and it moves the minimum.
//
// Two things could be wrong here and they fail differently.
//
// The first is the setting doing nothing: level_mix is read, stored, and then
// the draw reaches for the whole vocabulary anyway. Nothing else would see it -
// the size stays exact, every line still parses, and the manifest still says
// which mix was asked for. So this asks the FILE which severities are in it.
//
// The second is subtler and is why this guard exists at all. The shortest entry
// a shape can write has to leave room for the LONGEST severity it might draw,
// because the closing entry is built to an exact length and the unluckiest draw
// has to fit. quiet draws only INFO, four bytes, where realistic can draw ERROR
// or DEBUG at five. If the arithmetic reads the whole vocabulary instead of the
// set in force, the minimum is one byte too high for quiet - a floor the format
// can actually beat, announced as if it could not. That is checked by asking
// for the announced minimum and for one byte below it, rather than by repeating
// the arithmetic here, which would only prove the guard agrees with itself.
func TestTheLogSeverityMixIsDrawnAndSetsTheMinimum(t *testing.T) {
	d, err := format.Get("log")
	if err != nil {
		t.Fatal(err)
	}

	// What each declared mix is allowed to contain. Stated here on purpose:
	// this guard is outside the package, so it says what the contract is rather
	// than reading it back out of the thing under test. A mix added to the
	// registry without a line here stops the guard instead of slipping past it.
	allowed := map[string]map[string]bool{
		"realistic": {"DEBUG": true, "INFO": true, "WARN": true, "ERROR": true},
		"quiet":     {"INFO": true},
		"errors":    {"INFO": true, "WARN": true, "ERROR": true},
		"debug":     {"DEBUG": true, "INFO": true, "WARN": true},
	}

	var mixes []string
	for _, p := range d.Properties {
		if p.Name == "level_mix" {
			mixes = p.Choices
		}
	}
	if len(mixes) == 0 {
		t.Fatal("the log format declares no level_mix, so this guard would check nothing")
	}
	for _, m := range mixes {
		if allowed[m] == nil {
			t.Fatalf("level_mix offers %q and this guard does not say what may be in it. "+
				"Add the line rather than deleting this check - a mix nobody described is a mix nobody verified.", m)
		}
	}

	// Which shapes carry a severity is asked of the product, not listed here.
	// A shape reports level_mix in its plan exactly when it has one.
	var levelled, flat []string
	for _, shape := range logShapes(t) {
		p, err := d.Generator.Plan(format.Request{Bytes: 20 << 10, Seed: 7741, Label: true,
			Properties: map[string]string{"entry_format": shape}})
		if err != nil {
			t.Fatalf("%s would not plan at its defaults: %v", shape, err)
		}
		if _, ok := p.Properties["level_mix"]; ok {
			levelled = append(levelled, shape)
		} else {
			flat = append(flat, shape)
		}
	}
	if len(levelled) == 0 || len(flat) == 0 {
		t.Fatalf("found %d shapes with a severity and %d without, so this guard is walking nothing",
			len(levelled), len(flat))
	}

	severity := regexp.MustCompile(`\b(DEBUG|INFO|WARN|ERROR)\b`)
	minimum := func(shape, mix string) int64 {
		t.Helper()
		props := map[string]string{"entry_format": shape, "level_mix": mix}
		_, err := d.Generator.Plan(format.Request{Bytes: 1, Seed: 7741, Label: true, Properties: props})
		var below *format.BelowMinimumError
		if !errors.As(err, &below) {
			t.Fatalf("%s/%s took a one byte log, or refused it without saying what the floor is: %v", shape, mix, err)
		}
		return below.Minimum
	}

	for _, shape := range levelled {
		for _, mix := range mixes {
			t.Run(shape+"/"+mix, func(t *testing.T) {
				props := map[string]string{"entry_format": shape, "level_mix": mix}

				// The file, not the manifest, says which severities are in it.
				body := writeLog(t, 64<<10, props)
				seen := map[string]bool{}
				for _, m := range severity.FindAllString(string(body), -1) {
					seen[m] = true
				}
				if len(seen) == 0 {
					t.Fatalf("no severity at all in 64 kB of %s, so nothing here is being read", shape)
				}
				for level := range seen {
					if !allowed[mix][level] {
						t.Errorf("%s is in a %s log and that mix does not offer it, "+
							"so the setting was stored and then the draw ignored it", level, mix)
					}
				}

				// The floor it announces is the floor it takes, and one below is
				// refused. This is where a minimum read off the whole vocabulary
				// rather than off this mix goes red.
				n := minimum(shape, mix)
				if _, err := d.Generator.Plan(format.Request{Bytes: n, Seed: 7741, Label: true,
					Properties: props}); err != nil {
					t.Errorf("%s/%s announces %d B as its minimum and then refuses it: %v", shape, mix, n, err)
				}
				if _, err := d.Generator.Plan(format.Request{Bytes: n - 1, Seed: 7741, Label: true,
					Properties: props}); err == nil {
					t.Errorf("%s/%s took %d B, one below the %d B it calls its minimum", shape, mix, n-1, n)
				}

				// Exact to the byte at every seed, because the closing entry is
				// built to a length and the severity is part of that arithmetic.
				for seed := int64(1); seed <= 8; seed++ {
					for _, size := range []int64{n, n + 1, 777, 4 << 10} {
						if size < n {
							continue
						}
						p, err := d.Generator.Plan(format.Request{Bytes: size, Seed: uint64(seed),
							Label: true, Properties: props})
						if err != nil {
							t.Fatalf("seed %d, %d B: %v", seed, size, err)
						}
						var buf bytes.Buffer
						if err := d.Generator.Write(context.Background(), &buf, p); err != nil {
							t.Fatalf("seed %d, %d B: %v", seed, size, err)
						}
						if int64(buf.Len()) != size {
							t.Fatalf("seed %d asked for %d B and got %d", seed, size, buf.Len())
						}
					}
				}
			})
		}

		// The mixes are not all the same thing under different names, and the
		// arithmetic reads the one in force. quiet draws only INFO, which is
		// shorter than the longest severity realistic can draw, so its floor
		// has to be lower. Without this the whole test above would still pass
		// on a build where every mix was realistic.
		q, r := minimum(shape, "quiet"), minimum(shape, "realistic")
		if q >= r {
			t.Errorf("%s: the quiet floor is %d B and the realistic one %d B. quiet draws only INFO, "+
				"which is shorter than ERROR, so a floor that did not move was measured against "+
				"every severity this format knows rather than against the mix that was chosen.", shape, q, r)
		}
	}

	// And the shapes with no severity keep refusing a mix that was really
	// chosen, so the guard cannot pass by accepting everything.
	for _, shape := range flat {
		if _, err := d.Generator.Plan(format.Request{Bytes: 4096, Seed: 7741, Label: true,
			Properties: map[string]string{"entry_format": shape, "level_mix": "quiet"}}); err == nil {
			t.Errorf("%s carries no severity and took level_mix anyway, so the setting does nothing there", shape)
		}
	}
}
