package aggregator

import (
	"fmt"
	"strconv"
	"sync"
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

// we purposely keep the regex relatively simple because regex performance is up to the carbon-relay-ng user,
// so we want to focus on the carbon-relay-ng features, not regex performance
// b.N is how many points we generate (each based on 100 inputs)
func BenchmarkAggregator(b *testing.B) {
	reader := &sync.WaitGroup{}
	out := make(chan []byte, 100)
	reader.Add(1)
	go func() {
		count := 0
		for v := range out {
			count += 1
			//	if !ok {
			//		b.Fatalf("out channel closed at count %d ??", count)
			//	}
			//if v != 100 {
			//		b.Fatalf("point %d: expected 100, got %f", count, v)
			//	}
			b.Logf("got %q", v)
			if count == b.N {
				reader.Done()
				return
			}

		}
	}()
	var inputs [][]byte
	for i := 0; i < 5; i++ {
		key := "raw.abc." + RandString(10)
		for j := 0; j < 10; j++ {
			inputs = append(inputs, []byte(key))
		}
	}
	for i := 0; i < 50; i++ {
		inputs = append(inputs, []byte("nomatch.foo.bar"))
	}
	var timestamps [][]byte
	var wall []int64
	// time starts at 1000. increase by 5
	// we will do b.N aggregations (each 10s apart) and comprising 2 sets of points (5s apart)
	// so there is 2*b.N input timestamps
	// before adding the values, we set the clock to 22 after the ts of the points so it can flush prev point
	// e.g. :
	// pointTS - outputTs - wall
	// 1000    - 1000      1022
	// 1005    - 1000      1027
	// 1010    - 1010      1022 -> flush point with outputTs 1000
	// 1015    - 1010      1037
	// 1020    - 1020      1032 -> flush point with outputTs 1010
	// 1025    - 1020      1047
	// 1030    - 1030      1042 -> flush point with outputTs 1020
	// 1035    - 1030      1057
	// 1040    - 1040      1052 -> flush point with outputTs 1030
	var t uint64
	for t = 1000; t < uint64(1000+(10*b.N)); t += 5 {
		timestamps = append(timestamps, strconv.AppendUint(nil, t, 10))
		wall = append(wall, int64(t+22))
	}
	// one more so we can flush last data
	wall = append(wall, int64(t+22))

	val := strconv.AppendUint(nil, 1, 10)

	regex := `^raw\.(...)\.([A-Za-z0-9_-]+)$`
	outFmt := "aggregated.totals.$1.$2"

	clock := NewMockClock(0)
	tick := NewMockTick(10)
	clock.AddTick(tick)

	agg, err := NewMocked("sum", regex, outFmt, 10, 30, out, clock.Now, tick.C)
	if err != nil {
		b.Fatalf("couldn't create aggregation: %q", err)
	}

	fmt.Println("prep done", len(inputs))

	b.ResetTimer()
	for i, ts := range timestamps {
		clock.Set(wall[i])
		fmt.Println("sending all vals for ts", wall[i])
		for _, input := range inputs {
			buf := [][]byte{
				input,
				val,
				ts,
			}
			agg.AddMaybe(buf)
		}
	}
	// flush last point
	clock.Set(wall[len(wall)-1])
	reader.Wait()
}
