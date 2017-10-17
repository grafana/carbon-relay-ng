package aggregator

import (
	"bytes"
	"strconv"
	"testing"
	"time"
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

// an operation here is an aggregation, comprising of 2*aggregates*pointsPerAggregate points,
// (with just as many points ignored each time)
func BenchmarkAggregator1Aggregates2PointsPerAggregate(b *testing.B) {
	benchmarkAggregator(1, 2, "4.0000", b)
}
func BenchmarkAggregator5Aggregates10PointsPerAggregate(b *testing.B) {
	benchmarkAggregator(5, 10, "20.000", b)
}
func BenchmarkAggregator5Aggregates100PointsPerAggregate(b *testing.B) {
	benchmarkAggregator(5, 100, "200.00", b)
}

// we purposely keep the regex relatively simple because regex performance is up to the carbon-relay-ng user,
// so we want to focus on the carbon-relay-ng features, not regex performance
// b.N is how many points we generate (each based on 100 inputs)

func benchmarkAggregator(aggregates, pointsPerAggregate int, match string, b *testing.B) {
	//fmt.Println("BenchmarkAggregator", aggregates, pointsPerAggregate, "with b.N", b.N)
	out := make(chan []byte)
	done := make(chan struct{})
	go func(match string) {
		count := 0
		for v := range out {
			count += 1
			if bytes.HasPrefix(v, []byte("aggregated.totals.abc.ignoreme")) {
				continue
			}
			if string(v[33:39]) != match {
				b.Fatalf("expected 'aggregated.totals.abc.<10 random chars> %s... <ts>'. got: %q", match, v)
			}
			//	if count%100 == 0 {
			//fmt.Println("got", string(v), "count is now", count)
			//	}
			if count == aggregates*b.N {
				close(done)
				return
			}
		}
	}(match)

	// at each timestamp we will send all the inputs we predeclare here, with as many matching as non-matching.
	// (2*aggregates*pointsPerAggregate in total are sent at each timestamp)
	// (2*aggregates*pointsPerAggregate matching points go into each aggregation, since each input is used twice see below)
	var inputs [][]byte
	for i := 0; i < aggregates; i++ {
		key := "raw.abc." + RandString(10)
		for j := 0; j < pointsPerAggregate; j++ {
			inputs = append(inputs, []byte(key))
		}
	}
	for i := 0; i < aggregates*pointsPerAggregate; i++ {
		inputs = append(inputs, []byte("nomatch.foo.bar"))
	}

	// time starts at 1000. increase by 5
	// we will do b.N aggregations (each 10s apart) and comprising 2 sets of points (5s apart)
	// so there is 2*b.N input timestamps
	// before adding the values, we set the clock to 12 after the ts of the points so it can flush prev point
	// this means we can allow the aggregator to buffer only 2*aggregates*pointsPerAggregate points, otherwise buffering would be unsafe
	// at the speed that our benchmark runs (as it could trigger flush before it takes in the points)
	// e.g. :
	// pointTS - outputTs - wall
	// 1000    - 1000      1012
	// 1005    - 1000      1017
	// 1010    - 1010      1022 -> flush point with outputTs 990
	// 1015    - 1010      1027
	// 1020    - 1020      1032 -> flush point with outputTs 1000
	// 1025    - 1020      1037
	// 1030    - 1030      1042 -> flush point with outputTs 1010
	// 1035    - 1030      1047
	// 1040    - 1040      1052 -> flush point with outputTs 1020

	// tinfo holds all data to represent a point in time during the bench run
	type tinfo struct {
		ts    uint32 // the timestamp for the data at each point
		tsBuf []byte // ascii representation for the timestamp
		wall  int64  // the fake wall clock time at each point
	}
	var tinfos []tinfo
	for t := uint32(1000); t < uint32(1000+(10*b.N)); t += 5 {
		tinfos = append(tinfos, tinfo{
			ts:    t,
			tsBuf: strconv.AppendUint(nil, uint64(t), 10),
			wall:  int64(t + 12),
		})
	}
	val := strconv.AppendUint(nil, 1, 10)

	regex := `^raw\.(...)\.([A-Za-z0-9_-]+)$`
	outFmt := "aggregated.totals.$1.$2"

	clock := NewMockClock(0)
	tick := NewMockTick(10)
	clock.AddTick(tick)
	bufSize := 2 * aggregates * pointsPerAggregate

	agg, err := NewMocked("sum", regex, outFmt, 10, 30, out, bufSize, clock.Now, tick.C)
	if err != nil {
		b.Fatalf("couldn't create aggregation: %q", err)
	}

	b.ResetTimer()
	for _, t := range tinfos {
		//	fmt.Println("setting clock to", wall[i], "and sending", len(inputs)/2, "will go through. for ts", string(ts))
		clock.Set(t.wall)
		for _, input := range inputs {
			buf := [][]byte{
				input,
				val,
				t.tsBuf,
			}
			agg.AddMaybe(buf, 1, t.ts)
		}
	}
	// we must make sure to get the final aggregated point.
	// first of all, fill up the aggregator input buffer with metrics that will match and will go into the queue
	// but can't affect any results we care about.
	// making sure that all values we care about are pushed out of the buffer and processed.
	//fmt.Println("adding", bufSize, "more to push through buffer")
	for i := 0; i < bufSize; i++ {
		buf := [][]byte{
			[]byte("raw.abc.ignoreme"),
			val,
			tinfos[0].tsBuf,
		}
		agg.AddMaybe(buf, 1, tinfos[0].ts)
	}
	// then, we keep updating the clock, triggering ticks, with high enough timestamps to surely flush the last aggregates.
	// this so that if the aggregator is busy and doesn't read ticks, we keep trying until it isn't and does.
	lastFlushTs := tinfos[len(tinfos)-1].wall + 1000
	almostFinished := time.Now()
	for {
		select {
		case <-done:
			return
		default:
			if time.Since(almostFinished) > 2*time.Second {
				b.Fatalf("waited 2 seconds for all results to come in. giving up")
			}
			clock.Set(lastFlushTs)
			lastFlushTs += 10
			time.Sleep(10 * time.Microsecond)
		}
	}
}
