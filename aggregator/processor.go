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

func (a *Avg) Add(val float64) {
	a.sum += val
	a.cnt += 1
}

func (a *Avg) Flush() float64 {
	if a.cnt == 0 {
		panic("avg.Flush() called in aggregator with 0 terms")
	}
	return a.sum / float64(a.cnt)
}

// Delta aggregates to the difference between highest and lowest value seen
type Delta struct {
	valid bool
	max   float64
	min   float64
}

func (d *Delta) Add(val float64) {
	if !d.valid || val > d.max {
		d.max = val
	}
	if !d.valid || val < d.min {
		d.min = val
	}
	d.valid = true
}

func (d *Delta) Flush() float64 {
	if !d.valid {
		panic("delta.Flush() called in aggregator with 0 terms")
	}
	return d.max - d.min
}

// Last aggregates to the last value seen
type Last struct {
	valid bool
	val   float64
}

func (l *Last) Add(val float64) {
	l.valid = true
	l.val = val
}

func (l *Last) Flush() float64 {
	if !l.valid {
		panic("last.Flush() called in aggregator with 0 terms")
	}
	return l.val
}

// Max aggregates to the highest value seen
type Max struct {
	valid bool
	val   float64
}

func (m *Max) Add(val float64) {
	if !m.valid || val > m.val {
		m.val = val
	}
	m.valid = true
}

func (m *Max) Flush() float64 {
	if !m.valid {
		panic("max.Flush() called in aggregator with 0 terms")
	}
	return m.val
}

// Min aggregates to the lowest value seen
type Min struct {
	valid bool
	val   float64
}

func (m *Min) Add(val float64) {
	if !m.valid || val < m.val {
		m.val = val
	}
	m.valid = true
}

func (m *Min) Flush() float64 {
	if !m.valid {
		panic("min.Flush() called in aggregator with 0 terms")
	}
	return m.val
}

// Stdev aggregates to standard deviation
type Stdev struct {
	sum    float64
	values []float64
}

func (s *Stdev) Add(val float64) {
	s.sum += val
	s.values = append(s.values, val)
}

func (s *Stdev) Flush() float64 {
	if len(s.values) == 0 {
		panic("stdev.Flush() called in aggregator with 0 terms")
	}
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
	valid bool
	sum   float64
}

func (s *Sum) Add(val float64) {
	s.sum += val
	s.valid = true
}

func (s *Sum) Flush() float64 {
	if !s.valid {
		panic("sum.Flush() called in aggregator with 0 terms")
	}
	return s.sum
}

type Processor interface {
	Add(val float64)
	Flush() float64
}

func GetProcessorConstructor(fun string) (func() Processor, error) {
	switch fun {
	case "avg":
		return func() Processor { return &Avg{} }, nil
	case "delta":
		return func() Processor { return &Delta{} }, nil
	case "last":
		return func() Processor { return &Last{} }, nil
	case "max":
		return func() Processor { return &Max{} }, nil
	case "min":
		return func() Processor { return &Min{} }, nil
	case "stdev":
		return func() Processor { return &Stdev{} }, nil
	case "sum":
		return func() Processor { return &Sum{} }, nil
	}
	return nil, fmt.Errorf("no such aggregation function '%s'", fun)
}
