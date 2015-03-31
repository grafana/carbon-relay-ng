package main

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"time"
)

type AggregatorFunc func(in []float64) float64

func Sum(in []float64) float64 {
	sum := float64(0)
	for _, term := range in {
		sum += term
	}
	return sum
}

func Avg(in []float64) float64 {
	if len(in) == 0 {
		panic("avg() called in aggregator with 0 terms")
	}
	return Sum(in) / float64(len(in))
}

type Aggregator struct {
	Fun          string `json:"fun"`
	fn           AggregatorFunc
	in           chan []byte    // incoming metrics
	out          chan []byte    // outgoing metrics
	Regex        string         `json:"regex,omitempty"`
	regex        *regexp.Regexp // compiled version of Regex
	OutFmt       string
	outFmt       []byte
	Interval     uint             // expected interval between values in seconds, we will quantize to make sure alginment to interval-spaced timestamps
	Wait         uint             // seconds to wait after quantized time value before flushing final outcome and ignoring future values that are sent too late.
	aggregations []aggregation    // aggregations in process: one for each quantized timestamp and output key, i.e. for each output metric.
	snapReq      chan bool        // chan to issue snapshot requests on
	snapResp     chan *Aggregator // chan on which snapshot response gets sent
	shutdown     chan bool        // chan used internally to shut down
}

// NewAggregator creates an aggregator
func NewAggregator(fun, regex, outFmt string, interval, wait uint, out chan []byte) (*Aggregator, error) {
	regexObj, err := regexp.Compile(regex)
	if err != nil {
		return nil, err
	}
	funcs := map[string]AggregatorFunc{
		"sum": Sum,
		"avg": Avg,
	}
	fn, ok := funcs[fun]
	if !ok {
		return nil, fmt.Errorf("no such aggregation function '%s'", fun)
	}
	agg := &Aggregator{
		fun,
		fn,
		// i think it's possible in theory if this chan fills up, a deadlock to occur between table and this, but seems unlikely
		make(chan []byte, 2000),
		out,
		regex,
		regexObj,
		outFmt,
		[]byte(outFmt),
		interval,
		wait,
		make([]aggregation, 0, 4),
		make(chan bool),
		make(chan *Aggregator),
		make(chan bool),
	}
	go agg.run()
	return agg, nil
}

type aggregation struct {
	key    string
	ts     uint
	values []float64
}

func (a *Aggregator) AddOrCreate(key string, ts uint, value float64) {
	for i, agg := range a.aggregations {
		if agg.key == key && agg.ts == ts {
			a.aggregations[i].values = append(agg.values, value)
			return
		}
	}
	if ts > uint(time.Now().Unix())-a.Wait {
		a.aggregations = append(a.aggregations, aggregation{key, ts, []float64{value}})
	}
}

// Flush finalizes and removes aggregations that are due
func (a *Aggregator) Flush(ts uint) {
	aggregations2 := make([]aggregation, 0, len(a.aggregations))
	for _, agg := range a.aggregations {
		if agg.ts < ts {
			result := a.fn(agg.values)
			metric := fmt.Sprintf("%s %f %d", string(agg.key), result, agg.ts)
			fmt.Println("submitting result", metric)
			a.out <- []byte(metric)
		} else {
			aggregations2 = append(aggregations2, agg)
		}
	}
	a.aggregations = aggregations2
}

func (agg *Aggregator) Shutdown() {
	agg.shutdown <- true
	return
}

func (agg *Aggregator) run() {
	interval := time.Duration(agg.Interval) * time.Second
	ticker := getAlignedTicker(interval)
	for {
		select {
		case buf := <-agg.in:
			log.Info("agg %s receiving %s", agg.Regex, buf)
			fields := bytes.Fields(buf)
			// note, we rely here on the fact that the packet has already been validated
			key := fields[0]

			matches := agg.regex.FindSubmatchIndex(key)
			if len(matches) == 0 {
				continue
			}
			value, _ := strconv.ParseFloat(string(fields[1]), 64)
			t, _ := strconv.ParseUint(string(fields[2]), 10, 0)
			ts := uint(t)

			var dst []byte
			outKey := string(agg.regex.Expand(dst, agg.outFmt, key, matches))
			quantized := ts - (ts % agg.Interval)
			agg.AddOrCreate(outKey, quantized, value)
		case now := <-ticker.C:
			thresh := now.Add(-time.Duration(agg.Wait) * time.Second)
			agg.Flush(uint(thresh.Unix()))
			ticker = getAlignedTicker(interval)
		case <-agg.snapReq:
			var aggs []aggregation
			copy(aggs, agg.aggregations)
			s := &Aggregator{
				agg.Fun,
				agg.fn,
				nil,
				nil,
				agg.Regex,
				nil,
				agg.OutFmt,
				nil,
				agg.Interval,
				agg.Wait,
				aggs,
				nil,
				nil,
				nil,
			}

			agg.snapResp <- s
		case <-agg.shutdown:
			return

		}
	}
}

// to view the state of the aggregator at any point in time
func (agg *Aggregator) Snapshot() *Aggregator {
	agg.snapReq <- true
	return <-agg.snapResp
}

// getAlignedTicker returns a ticker so that, let's say interval is a second
// then it will tick at every whole second, or if it's 60s than it's every whole
// minute. Note that in my testing this is about .0001 to 0.0002 seconds later due
// to scheduling etc.
func getAlignedTicker(period time.Duration) *time.Ticker {
	unix := time.Now().UnixNano()
	diff := time.Duration(period - (time.Duration(unix) % period))
	return time.NewTicker(diff)
}
