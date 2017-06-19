package aggregator

import (
	"testing"
)

func TestScanner(t *testing.T) {
	cases := []struct {
		in    []float64
		avg   float64
		delta float64
		last  float64
		max   float64
		min   float64
		stdev float64
		sum   float64
	}{
		{
			[]float64{5, 4, 7, 4, 2, 5, 4, 9},
			5,
			7,
			9,
			9,
			2,
			2,
			40,
		},
		{
			[]float64{6, 2, 3, 1},
			3,
			5,
			1,
			6,
			1,
			1.8708286933869707,
			12,
		},
	}
	for i, e := range cases {
		var actual float64

		actual = Avg(e.in)
		if actual != e.avg {
			t.Fatalf("case %d AVG - expected %v, actual %v", i, e.avg, actual)
		}

		actual = Delta(e.in)
		if actual != e.delta {
			t.Fatalf("case %d DELTA - expected %v, actual %v", i, e.delta, actual)
		}

		actual = Last(e.in)
		if actual != e.last {
			t.Fatalf("case %d LAST - expected %v, actual %v", i, e.last, actual)
		}

		actual = Max(e.in)
		if actual != e.max {
			t.Fatalf("case %d MAX - expected %v, actual %v", i, e.max, actual)
		}

		actual = Min(e.in)
		if actual != e.min {
			t.Fatalf("case %d MIN - expected %v, actual %v", i, e.min, actual)
		}

		actual = Stdev(e.in)
		if actual != e.stdev {
			t.Fatalf("case %d STDEV - expected %v, actual %v", i, e.stdev, actual)
		}

		actual = Sum(e.in)
		if actual != e.sum {
			t.Fatalf("case %d SUM - expected %v, actual %v", i, e.sum, actual)
		}

	}
}
