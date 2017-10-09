package aggregator

import (
	"testing"
)

type AggCase map[string]float64

type TestCase struct {
	in  []float64
	ts  []uint
	agg AggCase
}

var AllTestCases = []TestCase{
	{
		in: []float64{5, 4, 7, 4, 2, 5, 4, 9},
		ts: []uint{1, 2, 3, 4, 5, 6, 7, 8},
		agg: AggCase{
			"avg":   5,
			"delta": 7,
			"last":  9,
			"max":   9,
			"min":   2,
			"stdev": 2,
			"sum":   40,
			"rate":  0.5714285714285714,
		},
	},
	{
		in: []float64{6, 2, 3, 1},
		ts: []uint{1, 2, 3, 4},
		agg: AggCase{
			"avg":   3,
			"delta": 5,
			"last":  1,
			"max":   6,
			"min":   1,
			"stdev": 1.8708286933869707,
			"sum":   12,
			"rate":  -1.6666666666666667,
		},
	},
	/* FIXME(gautran): Out of order metrics don't get handled properly
	{ // Test receiving metric out of order
		in: []float64{5, 4, 7, 4, 2, 5, 9, 4},
		ts: []uint{1, 2, 3, 4, 5, 6, 8, 7},
		agg: AggCase{
			"avg":   5,
			"delta": 7,
			"last":  9,
			"max":   9,
			"min":   2,
			"stdev": 2,
			"sum":   40,
			"rate":  0.5714285714285714,
		},
	},
	*/
}

func TestScanner(t *testing.T) {

	log.Info("In the test scanner method")

	for i := range AllTestCases {
		test := AllTestCases[i]
		for agg, expected := range test.agg {
			actual := Funcs[agg](test.in, test.ts)
			if actual != expected {
				t.Fatalf("case %s - expected %v, actual %v", agg, expected, actual)
			}
		}
	}

}
