package aggregator

import (
	"fmt"
	"math"
)

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

func (a *Avg) Flush() (float64, bool) {
	return a.sum / float64(a.cnt), true
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

func (d *Delta) Flush() (float64, bool) {
	return d.max - d.min, true
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

func (d *Derive) Flush() (float64, bool) {
	if d.newestTs == d.oldestTs {
		return 0, false
	}
	return (d.newestVal - d.oldestVal) / float64(d.newestTs-d.oldestTs), true
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

func (l *Last) Flush() (float64, bool) {
	return l.val, true
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

func (m *Max) Flush() (float64, bool) {
	return m.val, true
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

func (m *Min) Flush() (float64, bool) {
	return m.val, true
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

func (s *Stdev) Flush() (float64, bool) {
	mean := s.sum / float64(len(s.values))

	// Calculate the variance
	variance := float64(0)
	for _, term := range s.values {
		variance += math.Pow((term - mean), float64(2))
	}
	variance /= float64(len(s.values))

	// Calculate the standard deviation
	return math.Sqrt(variance), true
}

// Sum aggregates to average
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

func (s *Sum) Flush() (float64, bool) {
	return s.sum, true
}

type Processor interface {
	// Add adds a point to aggregate
	Add(val float64, ts uint32)
	// Flush returns the aggregated value and true if it is valid
	// the only reason why it would be non-valid is for aggregators that need
	// more than 1 value but they didn't have enough to produce a useful result.
	Flush() (float64, bool)
}

func GetProcessorConstructor(fun string) (func(val float64, ts uint32) Processor, error) {
	switch fun {
	case "avg":
		return NewAvg, nil
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
	}
	return nil, fmt.Errorf("no such aggregation function '%s'", fun)
}
