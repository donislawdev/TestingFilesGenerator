package otlp

import (
	// D11 promises the same bytes from the same seed, so a deliberate,
	// reproducible generator is the product rather than a weakness. Nothing
	// here ever makes a secret.
	// nosemgrep: go.lang.security.audit.crypto.math_random.math-random-used
	"math/rand/v2"
)

// A file of metrics is a run of collections, the way the periodic reader of an
// SDK exports them: every interval, one point for every series of every
// instrument, sent together. In the default layout a line is one collection.
//
// The record the filler counts is one point. The first point of a metric in a
// collection carries the name, the unit and the kind in front of it, and the
// last one what closes the metric, the same way a span carries what opens and
// closes its batch.

// pointDepth is how deep a point sits: inside the metric, the member that
// holds its data and the array of points.
const pointDepth = recordDepth + 3

// widestCount bounds every count a file holds. A series takes three
// measurements at most between two collections and a collection comes a
// second apart at the least, so from the first year a file may start in to
// the last moment it may name a count stays below 2.5e10 - eleven digits.
const widestCount = 99_999_999_999

// slot is one point of a collection: the instrument, which of its series,
// and whether it is the last series, the point that closes the metric.
type slot struct {
	inst   *instrument
	series int
	last   bool
}

// seriesState is what a series remembers from one collection to the next.
// Arrays rather than slices, so the state of the one series a thrown away
// point changed is put back by copying a value.
type seriesState struct {
	// running is the count a counter has reached: microseconds of CPU, or
	// classes loaded.
	running     int64
	count       int64
	sum         micros
	least, most micros
	buckets     [bucketCount]int64
}

// exemplar is one measurement a point of a histogram names, with the trace it
// was taken in.
type exemplar struct {
	at    int64
	value micros
	trace traceID
	span  spanID
}

// point is everything one data point says, drawn or chosen in one place so the
// smallest file is measured with the same code that writes every other one.
type point struct {
	slot      slot
	start, at int64
	// number is the value of a gauge or a sum: a whole number, or millionths
	// where the instrument writes asDouble.
	number      int64
	count       int64
	sum         micros
	least, most micros
	// buckets are the counts written, from the first bucket that holds any.
	buckets []int64
	// offset is the index of the first exponential bucket written.
	offset    int
	quantiles [3]micros
	exemplars [mostMeasurements]exemplar
	examples  int
	// temporality is the number written, nought for a shape without one.
	temporality int64
}

// metricPlan is how a file of metrics is laid out and what it remembers.
type metricPlan struct {
	s      settings
	slots  []slot
	states []seriesState
	// saved is the state the last point changed, as it was before, so the
	// point thrown away to measure it can be undone.
	saved   seriesState
	savedAt int
	// cycle holds the buckets of one collection, scratch reused for every
	// point.
	cycle [bucketCount]int64
	cur   point
}

func newMetricPlan(s settings) *metricPlan {
	m := &metricPlan{s: s}
	for _, inst := range s.instruments {
		for i := range inst.series {
			m.slots = append(m.slots, slot{inst: inst, series: i, last: i == len(inst.series)-1})
		}
	}
	m.states = make([]seriesState, len(m.slots))
	return m
}

// cumulative says whether a point of this instrument covers everything since
// the process started rather than the time since the collection before.
func (s settings) cumulative(inst *instrument) bool {
	return s.temporality == Cumulative || inst.updown || !inst.shape.temporal()
}

// temporalityOf is the number a metric of this instrument states, nought when
// its shape states none.
func (s settings) temporalityOf(inst *instrument) int64 {
	switch {
	case !inst.shape.temporal():
		return 0
	case s.cumulative(inst):
		return temporalityCumulative
	default:
		return temporalityDelta
	}
}

// draw draws the point a file holds after count others and moves its series on.
//
// The order of the draws is part of the format, for the reason drawSpan gives.
// The trace and the span of every measurement of a request are drawn whether
// or not the file writes exemplars, so turning them on changes nothing else.
func (m *metricPlan) draw(rng *rand.Rand, count int64) *point {
	s := m.s
	at := int(count % int64(len(m.slots)))
	collection := count / int64(len(m.slots))
	sl := m.slots[at]
	st := &m.states[at]
	m.saved, m.savedAt = *st, at

	p := &m.cur
	// Dated by its place on the clock, which time_order may swap with the
	// collection next to it. What it measured is drawn in the order written.
	tick := s.order(collection)
	*p = point{slot: sl, at: s.start + (tick+1)*s.step(), temporality: s.temporalityOf(sl.inst)}
	cumulative := s.cumulative(sl.inst)
	p.start = s.start
	if !cumulative {
		p.start = s.start + tick*s.step()
	}
	switch sl.inst.values {
	case cpuUtilization:
		p.number = utilization(rng)
		if s.nonFinite {
			p.number = nonFiniteAt(collection, p.number)
		}
	case ordersPending:
		p.number = rng.Int64N(251)
	case cpuTime:
		// A share of one processor over the interval, in microseconds.
		used := utilization(rng) * s.interval
		st.running += used
		p.number = pick(cumulative, st.running, used)
	case classesLoaded:
		loaded := rng.Int64N(4)
		if collection == 0 {
			loaded = 4000 + rng.Int64N(1000)
		}
		st.running += loaded
		p.number = pick(cumulative, st.running, loaded)
	case memoryUsed:
		pool := memoryPools[sl.series]
		p.number = pool.low + rng.Int64N(pool.high-pool.low+1)
	default:
		m.measure(rng, p, st, cumulative)
	}
	return p
}

func utilization(rng *rand.Rand) micros { return 1000 + rng.Int64N(949_000) }

func pick(cumulative bool, running, step int64) int64 {
	if cumulative {
		return running
	}
	return step
}

// The values that are not numbers, as the JSON mapping of protobuf writes a
// double that holds one: text, which the OTLP specification takes as it is.
// A point holds one as the negative of its place here, a number no
// utilization can be.
const (
	notANumber       = 1
	positiveInfinity = 2
	negativeInfinity = 3
)

var nonFiniteNames = [...]string{notANumber: "NaN", positiveInfinity: "Infinity", negativeInfinity: "-Infinity"}

// nonFiniteAt is the utilization a collection reports with non_finite on:
// NaN in the second of every four, Infinity and -Infinity in turn in the
// fourth, and what was measured in the other two.
func nonFiniteAt(collection int64, measured micros) micros {
	switch collection % 4 {
	case 1:
		return -notANumber
	case 3:
		if collection/4%2 == 0 {
			return -positiveInfinity
		}
		return -negativeInfinity
	}
	return measured
}
