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

func NewAvg(val float64) Processor {
	return &Avg{
		sum: val,
		cnt: 1,
	}
}

func (a *Avg) Add(val float64) {
	a.sum += val
	a.cnt += 1
}

func (a *Avg) Flush() float64 {
	return a.sum / float64(a.cnt)
}

// Delta aggregates to the difference between highest and lowest value seen
type Delta struct {
	max float64
	min float64
}

func NewDelta(val float64) Processor {
	return &Delta{
		max: val,
		min: val,
	}
}

func (d *Delta) Add(val float64) {
	if val > d.max {
		d.max = val
	}
	if val < d.min {
		d.min = val
	}
}

func (d *Delta) Flush() float64 {
	return d.max - d.min
}

// Last aggregates to the last value seen
type Last struct {
	val float64
}

func NewLast(val float64) Processor {
	return &Last{
		val: val,
	}
}

func (l *Last) Add(val float64) {
	l.val = val
}

func (l *Last) Flush() float64 {
	return l.val
}

// Max aggregates to the highest value seen
type Max struct {
	val float64
}

func NewMax(val float64) Processor {
	return &Max{
		val: val,
	}
}

func (m *Max) Add(val float64) {
	if val > m.val {
		m.val = val
	}
}

func (m *Max) Flush() float64 {
	return m.val
}

// Min aggregates to the lowest value seen
type Min struct {
	val float64
}

func NewMin(val float64) Processor {
	return &Min{
		val: val,
	}
}

func (m *Min) Add(val float64) {
	if val < m.val {
		m.val = val
	}
}

func (m *Min) Flush() float64 {
	return m.val
}

// Stdev aggregates to standard deviation
type Stdev struct {
	sum    float64
	values []float64
}

func NewStdev(val float64) Processor {
	return &Stdev{
		sum:    val,
		values: []float64{val},
	}
}

func (s *Stdev) Add(val float64) {
	s.sum += val
	s.values = append(s.values, val)
}

func (s *Stdev) Flush() float64 {
	mean := s.sum / float64(len(s.values))

	// Calculate the variance
	variance := float64(0)
	for _, term := range s.values {
		variance += math.Pow((term - mean), float64(2))
	}
	variance /= float64(len(s.values))

	// Calculate the standard deviation
	return math.Sqrt(variance)
}

// Sum aggregates to average
type Sum struct {
	sum float64
}

func NewSum(val float64) Processor {
	return &Sum{
		sum: val,
	}
}

func (s *Sum) Add(val float64) {
	s.sum += val
}

func (s *Sum) Flush() float64 {
	return s.sum
}

type Processor interface {
	Add(val float64)
	Flush() float64
}

func GetProcessorConstructor(fun string) (func(val float64) Processor, error) {
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
	}
	return nil, fmt.Errorf("no such aggregation function '%s'", fun)
}
