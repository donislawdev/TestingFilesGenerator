package otlp

import (
	// D11 promises the same bytes from the same seed, so a deliberate,
	// reproducible generator is the product rather than a weakness. Nothing
	// here ever makes a secret.
	// nosemgrep: go.lang.security.audit.crypto.math_random.math-random-used
	"math/rand/v2"
)

// measure draws the durations a series measured since the collection before
// and folds them into its point: the buckets, the count, the sum, the least
// and the most, and for a summary the quantiles of this collection.
func (m *metricPlan) measure(rng *rand.Rand, p *point, st *seriesState, cumulative bool) {
	inst := p.slot.inst
	bands := requestBands
	if inst.values == gcDuration {
		bands = gcBands[p.slot.series]
	}
	n := 1 + rng.IntN(mostMeasurements)
	var taken [mostMeasurements]micros
	var where [mostMeasurements]int
	m.cycle = [bucketCount]int64{}
	for i := range taken {
		if i == n {
			break
		}
		b := bands[rng.IntN(len(bands))]
		taken[i] = b.low + rng.Int64N(b.high-b.low+1)
		if inst.exemplary {
			ex := &p.exemplars[i]
			ex.at = p.at - rng.Int64N(m.s.step())
			ex.value = taken[i]
			drawID(ex.trace[:], rng)
			drawID(ex.span[:], rng)
		}
		where[i] = bucketOf(inst.shape, taken[i])
		m.cycle[where[i]]++
	}
	p.count, p.sum, p.least, p.most = tally(taken[:n])
	st.fold(&m.cycle, p.count, p.sum, p.least, p.most)
	held := m.cycle[:]
	if cumulative {
		held = st.buckets[:]
		p.count, p.sum, p.least, p.most = st.count, st.sum, st.least, st.most
	}
	m.shapePoint(p, st, held, where[:n], taken[:n])
}

// tally is how many durations there are, their sum, the least and the most.
func tally(taken []micros) (count int64, sum, least, most micros) {
	least = taken[0]
	for _, d := range taken {
		sum += d
		least = min(least, d)
		most = max(most, d)
	}
	return int64(len(taken)), sum, least, most
}

// fold adds what one collection measured to what the series has measured
// since the process started.
func (st *seriesState) fold(cycle *[bucketCount]int64, count int64, sum, least, most micros) {
	if st.count == 0 || least < st.least {
		st.least = least
	}
	st.most = max(st.most, most)
	st.count += count
	st.sum += sum
	for i := range cycle {
		st.buckets[i] += cycle[i]
	}
}

// shapePoint gives a point what its shape writes beside the count and the
// sum: the buckets an explicit histogram writes all of and an exponential one
// writes from its first count to its last, the exemplars, or the quantiles.
func (m *metricPlan) shapePoint(p *point, st *seriesState, held []int64, where []int, taken []micros) {
	switch p.slot.inst.shape {
	case shapeHistogram:
		p.buckets = held[:len(requestBounds)+1]
		if m.s.exemplars {
			p.examples = byBucket(p, where)
		}
	case shapeExponential:
		first, last := 0, len(held)-1
		for held[first] == 0 {
			first++
		}
		for held[last] == 0 {
			last--
		}
		p.buckets, p.offset = held[first:last+1], firstBucket+first
	case shapeSummary:
		// The summary is cumulative in its count and its sum and speaks of
		// this collection alone in its quantiles, the way Micrometer and the
		// Prometheus client write one.
		p.count, p.sum = st.count, st.sum
		quantiles(p, taken)
	case shapeGauge, shapeSum:
		// A number is not measured, so it has no buckets and no quantiles.
	}
}

// quantiles are the percentiles of what one collection measured, each the
// nearest rank: the smallest measurement at or above that share of them.
func quantiles(p *point, taken []micros) {
	for i := 1; i < len(taken); i++ {
		for j := i; j > 0 && taken[j] < taken[j-1]; j-- {
			taken[j], taken[j-1] = taken[j-1], taken[j]
		}
	}
	for i, q := range timerQuantiles {
		rank := (q*int64(len(taken)) + 999_999) / 1_000_000
		p.quantiles[i] = taken[max(rank, 1)-1]
	}
}

// bucketOf is the bucket a duration falls in: the first whose upper bound it
// does not pass, the way an explicit histogram and an exponential one both
// count a value equal to a bound.
func bucketOf(sh shape, d micros) int {
	if sh == shapeHistogram {
		return explicitBucket(d)
	}
	return exponentialBucket(d)
}

func explicitBucket(d micros) int {
	for i, top := range requestBounds {
		if d <= top {
			return i
		}
	}
	return len(requestBounds)
}

// exponentialBucket finds the bucket by halving the table, which is sorted.
func exponentialBucket(d micros) int {
	low, high := 0, bucketCount-1
	for low < high {
		mid := (low + high) / 2
		if d <= bucketTops[mid] {
			high = mid
		} else {
			low = mid + 1
		}
	}
	return low
}

// byBucket keeps one exemplar for every bucket the collection measured in -
// the last measurement that fell there, as the reservoir of an explicit
// histogram keeps them - in the order of the buckets, and says how many.
func byBucket(p *point, where []int) int {
	kept := 0
	var keptBucket [mostMeasurements]int
	for i := range where {
		at := indexOf(keptBucket[:kept], where[i])
		if at < 0 {
			at, kept = kept, kept+1
		}
		keptBucket[at] = where[i]
		p.exemplars[at] = p.exemplars[i]
	}
	for i := 1; i < kept; i++ {
		for j := i; j > 0 && keptBucket[j] < keptBucket[j-1]; j-- {
			keptBucket[j], keptBucket[j-1] = keptBucket[j-1], keptBucket[j]
			p.exemplars[j], p.exemplars[j-1] = p.exemplars[j-1], p.exemplars[j]
		}
	}
	return kept
}

// indexOf is where a bucket stands among those kept, -1 when it is not there.
func indexOf(kept []int, bucket int) int {
	for k, b := range kept {
		if b == bucket {
			return k
		}
	}
	return -1
}
