package otlp

// widest are the longest points each instrument of the settings can
// write, every series of it, with every count at the most a file can reach
// and every fraction at its longest. None when the file holds no metrics.
//
// Dated at the start of the file, for the reason widestSpans gives.
func (m *metricPlan) widest() []point {
	var out []point
	wide := make([]int64, bucketCount)
	for i := range wide {
		wide[i] = widestCount
	}
	for _, sl := range m.slots {
		out = append(out, m.widestPoint(sl, wide))
	}
	return out
}

// widestPoint is the longest point one series can write: every count at the
// most a file can reach, every fraction at its longest, the widest value its
// instrument draws.
func (m *metricPlan) widestPoint(sl slot, wide []int64) point {
	inst := sl.inst
	p := point{slot: sl, start: m.s.start, at: m.s.start, temporality: m.s.temporalityOf(inst),
		count: widestCount, sum: microCeiling - 1, least: longestDuration - 1, most: longestDuration - 1}
	switch inst.values {
	case cpuUtilization:
		p.number = 949_999
		if m.s.nonFinite {
			p.number = -negativeInfinity
		}
	case ordersPending:
		p.number = 250
	case cpuTime:
		p.number = microCeiling - 1
	case classesLoaded:
		p.number = widestCount
	case memoryUsed:
		p.number = memoryPools[sl.series].high
	case requestDuration, gcDuration, requestTimer:
		// Measured, so the shape below writes them rather than a number.
	}
	switch inst.shape {
	case shapeHistogram:
		p.buckets = wide[:len(requestBounds)+1]
		if m.s.exemplars {
			m.widestExemplars(&p)
		}
	case shapeExponential:
		p.buckets, p.offset = wide, firstBucket
	case shapeSummary:
		p.quantiles = [3]micros{longestDuration - 1, longestDuration - 1, longestDuration - 1}
	case shapeGauge, shapeSum:
		// A number, written above.
	}
	return p
}

// widestExemplars gives a point as many exemplars as a collection can keep,
// each with the longest value.
func (m *metricPlan) widestExemplars(p *point) {
	p.examples = mostMeasurements
	for i := range p.exemplars {
		p.exemplars[i].at, p.exemplars[i].value = m.s.start, longestDuration-1
	}
}

// pointWriter writes the points of metrics with the emitter of a file:
// apart from records so the writing of one shape stays beside the others
// and the builder of records keeps to building.
type pointWriter struct{ e *emitter }

// write writes one point, after the name of its metric when opens says so
// and before what closes the metric when closes does. padded adds the padding
// attribute. at is where the padding begins, nought without one.
func (w pointWriter) write(p *point, opens, padded, closes bool) (at int) {
	e, inst := w.e, p.slot.inst
	if opens {
		w.openMetric(inst)
	}
	e.next()
	e.open('{')
	at = w.writeSeries(inst.series[p.slot.series], padded)
	e.key("startTimeUnixNano")
	e.quoted(p.start)
	e.key("timeUnixNano")
	e.quoted(p.at)
	switch inst.shape {
	case shapeGauge, shapeSum:
		w.writeNumber(p)
	case shapeHistogram:
		w.writeHistogram(p)
	case shapeExponential:
		w.writeExponential(p)
	case shapeSummary:
		w.writeSummary(p)
	}
	e.close('}')
	if closes {
		w.closeMetric(p)
	}
	return at
}

// openMetric writes what a metric opens with, through the bracket of its
// points: the name, the description, the unit and the member of its shape.
func (w pointWriter) openMetric(inst *instrument) {
	e := w.e
	e.next()
	e.open('{')
	e.key("name")
	e.text(inst.name)
	e.key("description")
	e.text(inst.description)
	e.key("unit")
	e.text(inst.unit)
	e.key(member[inst.shape])
	e.open('{')
	e.key("dataPoints")
	e.open('[')
}

// closeMetric writes what follows the last point of a metric.
func (w pointWriter) closeMetric(p *point) {
	e := w.e
	e.close(']')
	if p.temporality != 0 {
		e.key("aggregationTemporality")
		e.enum(p.temporality, temporalityNames[p.temporality])
	}
	if p.slot.inst.monotonic {
		e.key("isMonotonic")
		e.truth()
	}
	e.close('}')
	e.close('}')
}

// writeSeries writes the attributes of a series, with the padding attribute
// last when padded says so, and returns where the padding begins. A series
// with none and no padding writes no member at all.
func (w pointWriter) writeSeries(attrs series, padded bool) (at int) {
	if len(attrs) == 0 && !padded {
		return 0
	}
	e := w.e
	e.key("attributes")
	e.open('[')
	for _, a := range attrs {
		if a.whole() {
			e.wholeAttribute(a.key, a.number)
		} else {
			e.textAttribute(a.key, a.text)
		}
	}
	if padded {
		at = e.padding()
	}
	e.close(']')
	return at
}

// writeNumber is the value of a gauge or a sum.
func (w pointWriter) writeNumber(p *point) {
	e := w.e
	switch {
	case p.slot.inst.whole:
		e.key("asInt")
		e.quoted(p.number)
	case p.number < 0:
		e.key("asDouble")
		e.text(nonFiniteNames[-p.number])
	default:
		e.key("asDouble")
		e.decimal(p.number)
	}
}

func (w pointWriter) writeHistogram(p *point) {
	e := w.e
	w.writeDistribution(p)
	e.key("explicitBounds")
	e.open('[')
	for _, b := range requestBounds {
		e.next()
		e.decimal(b)
	}
	e.close(']')
	w.writeExemplars(p)
	w.writeRange(p)
}

func (w pointWriter) writeExponential(p *point) {
	e := w.e
	e.key("count")
	e.quoted(p.count)
	e.key("sum")
	e.decimal(p.sum)
	e.key("scale")
	e.whole(exponentialScale)
	e.key("positive")
	e.open('{')
	if p.offset != 0 {
		e.key("offset")
		e.whole(int64(p.offset))
	}
	w.writeCounts(p)
	e.close('}')
	// Always there, also with nothing in it: that is how the Collector writes
	// a histogram that measured nothing below nought.
	e.key("negative")
	e.nothing()
	w.writeRange(p)
}

func (w pointWriter) writeSummary(p *point) {
	e := w.e
	e.key("count")
	e.quoted(p.count)
	e.key("sum")
	e.decimal(p.sum)
	e.key("quantileValues")
	e.open('[')
	for i, q := range timerQuantiles {
		e.next()
		e.open('{')
		e.key("quantile")
		e.decimal(q)
		e.key("value")
		e.decimal(p.quantiles[i])
		e.close('}')
	}
	e.close(']')
}

// writeDistribution is the count, the sum and the counts of the buckets of an
// explicit histogram.
func (w pointWriter) writeDistribution(p *point) {
	e := w.e
	e.key("count")
	e.quoted(p.count)
	e.key("sum")
	e.decimal(p.sum)
	w.writeCounts(p)
}

func (w pointWriter) writeCounts(p *point) {
	e := w.e
	e.key("bucketCounts")
	e.open('[')
	for _, c := range p.buckets {
		e.next()
		e.quoted(c)
	}
	e.close(']')
}

// writeExemplars writes the measurements a point names, when it names any.
// The trace id stands before the span id: measured on 2026-11-11, that is
// where the Collector writes it, against the order metrics.proto declares.
func (w pointWriter) writeExemplars(p *point) {
	if p.examples == 0 {
		return
	}
	e := w.e
	e.key("exemplars")
	e.open('[')
	for _, ex := range p.exemplars[:p.examples] {
		e.next()
		e.open('{')
		e.key("timeUnixNano")
		e.quoted(ex.at)
		e.key("asDouble")
		e.decimal(ex.value)
		e.key("traceId")
		e.id(ex.trace[:])
		e.key("spanId")
		e.id(ex.span[:])
		e.close('}')
	}
	e.close(']')
}

func (w pointWriter) writeRange(p *point) {
	e := w.e
	e.key("min")
	e.decimal(p.least)
	e.key("max")
	e.decimal(p.most)
}
