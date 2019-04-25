package aggregator

import (
	"fmt"
	"math"
	"sort"
)

type processorResult struct {
	fcnName string
	val     float64
}

// Avg aggregates to average
type Avg struct {
	sum float64
	cnt int
}

func NewAvg(val float64, ts uint32) Processor {
	return &Avg{
		sum: val,
		cnt: 1,
	}
}

func (a *Avg) Add(val float64, ts uint32) {
	a.sum += val
	a.cnt += 1
}

func (a *Avg) Flush() ([]processorResult, bool) {
	return []processorResult{
		{fcnName: "avg", val: a.sum / float64(a.cnt)},
	}, true
}

// Count aggregates to the number of values seen
type Count struct {
	cnt int
}

func NewCount(val float64, ts uint32) Processor {
	return &Count{
		cnt: 1,
	}
}

func (c *Count) Add(val float64, ts uint32) {
	c.cnt += 1
}

func (c *Count) Flush() ([]processorResult, bool) {
	return []processorResult{
		{fcnName: "count", val: float64(c.cnt)},
	}, true
}

// Delta aggregates to the difference between highest and lowest value seen
type Delta struct {
	max float64
	min float64
}

func NewDelta(val float64, ts uint32) Processor {
	return &Delta{
		max: val,
		min: val,
	}
}

func (d *Delta) Add(val float64, ts uint32) {
	if val > d.max {
		d.max = val
	}
	if val < d.min {
		d.min = val
	}
}

func (d *Delta) Flush() ([]processorResult, bool) {
	return []processorResult{
		{fcnName: "delta", val: d.max - d.min},
	}, true
}

// Derive aggregates to the derivative of the largest timeframe we get
type Derive struct {
	oldestTs  uint32
	newestTs  uint32
	oldestVal float64
	newestVal float64
}

func NewDerive(val float64, ts uint32) Processor {
	return &Derive{
		oldestTs:  ts,
		newestTs:  ts,
		oldestVal: val,
		newestVal: val,
	}
}

func (d *Derive) Add(val float64, ts uint32) {
	if ts > d.newestTs {
		d.newestTs = ts
		d.newestVal = val
	}
	if ts < d.oldestTs {
		d.oldestTs = ts
		d.oldestVal = val
	}
}

func (d *Derive) Flush() ([]processorResult, bool) {
	if d.newestTs == d.oldestTs {
		return nil, false
	}
	return []processorResult{
		{fcnName: "derive", val: (d.newestVal - d.oldestVal) / float64(d.newestTs-d.oldestTs)},
	}, true
}

// Last aggregates to the last value seen
type Last struct {
	val float64
}

func NewLast(val float64, ts uint32) Processor {
	return &Last{
		val: val,
	}
}

func (l *Last) Add(val float64, ts uint32) {
	l.val = val
}

func (l *Last) Flush() ([]processorResult, bool) {
	return []processorResult{
		{fcnName: "last", val: l.val},
	}, true
}

// Max aggregates to the highest value seen
type Max struct {
	val float64
}

func NewMax(val float64, ts uint32) Processor {
	return &Max{
		val: val,
	}
}

func (m *Max) Add(val float64, ts uint32) {
	if val > m.val {
		m.val = val
	}
}

func (m *Max) Flush() ([]processorResult, bool) {
	return []processorResult{
		{fcnName: "max", val: m.val},
	}, true
}

// Min aggregates to the lowest value seen
type Min struct {
	val float64
}

func NewMin(val float64, ts uint32) Processor {
	return &Min{
		val: val,
	}
}

func (m *Min) Add(val float64, ts uint32) {
	if val < m.val {
		m.val = val
	}
}

func (m *Min) Flush() ([]processorResult, bool) {
	return []processorResult{
		{fcnName: "min", val: m.val},
	}, true
}

// Stdev aggregates to standard deviation
type Stdev struct {
	sum    float64
	values []float64
}

func NewStdev(val float64, ts uint32) Processor {
	return &Stdev{
		sum:    val,
		values: []float64{val},
	}
}

func (s *Stdev) Add(val float64, ts uint32) {
	s.sum += val
	s.values = append(s.values, val)
}

func (s *Stdev) Flush() ([]processorResult, bool) {
	mean := s.sum / float64(len(s.values))

	// Calculate the variance
	variance := float64(0)
	for _, term := range s.values {
		variance += math.Pow((term - mean), float64(2))
	}
	variance /= float64(len(s.values))

	// Calculate the standard deviation
	return []processorResult{
		{fcnName: "stdev", val: math.Sqrt(variance)},
	}, true
}

// Percentiles aggregates to different percentiles
type Percentiles struct {
	percents map[string]float64
	values   []float64
}

func NewPercentiles(val float64, ts uint32) Processor {
	return &Percentiles{
		values: []float64{val},
		percents: map[string]float64{
			"p25": 25,
			"p50": 50,
			"p75": 75,
			"p90": 90,
			"p95": 95,
			"p99": 99,
		},
	}
}

func (p *Percentiles) Add(val float64, ts uint32) {
	p.values = append(p.values, val)
}

// Using the latest recommendation from NIST
// See https://www.itl.nist.gov/div898/handbook/prc/section2/prc262.htm
// The method implemented corresponds to method R6 of Hyndman and Fan.
// See https://en.wikipedia.org/wiki/Percentile, Third variant
func (p *Percentiles) Flush() ([]processorResult, bool) {

	size := len(p.values)
	if size == 0 {
		return nil, false
	}

	var results []processorResult
	sort.Float64s(p.values)

	for fcnName, percent := range p.percents {
		rank := (percent / 100) * (float64(size) + 1)
		floor := int(rank)

		if rank < 1 {
			results = append(results, processorResult{fcnName, p.values[0]})
		} else if floor >= size {
			results = append(results, processorResult{fcnName, p.values[size-1]})
		} else {
			frac := rank - float64(floor)
			upper := floor + 1
			percentile := p.values[floor-1] + frac*(p.values[upper-1]-p.values[floor-1])
			results = append(results, processorResult{fcnName, percentile})
		}
	}

	return results, true
}

// Sum aggregates to sum
type Sum struct {
	sum float64
}

func NewSum(val float64, ts uint32) Processor {
	return &Sum{
		sum: val,
	}
}

func (s *Sum) Add(val float64, ts uint32) {
	s.sum += val
}

func (s *Sum) Flush() ([]processorResult, bool) {
	return []processorResult{
		{fcnName: "sum", val: s.sum},
	}, true
}

type Processor interface {
	// Add adds a point to aggregate
	Add(val float64, ts uint32)
	// Flush returns the aggregated value(s) and true if it is valid
	// the only reason why it would be non-valid is for aggregators that need
	// more than 1 value but they didn't have enough to produce a useful result.
	Flush() ([]processorResult, bool)
}

func GetProcessorConstructor(fun string) (func(val float64, ts uint32) Processor, error) {
	switch fun {
	case "avg":
		return NewAvg, nil
	case "count":
		return NewCount, nil
	case "delta":
		return NewDelta, nil
	case "last":
		return NewLast, nil
	case "max":
		return NewMax, nil
	case "min":
		return NewMin, nil
	case "stdev":
		return NewStdev, nil
	case "sum":
		return NewSum, nil
	case "derive":
		return NewDerive, nil
	case "percentiles":
		return NewPercentiles, nil
	}
	return nil, fmt.Errorf("no such aggregation function '%s'", fun)
}
