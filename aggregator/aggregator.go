package aggregator

import (
	"bytes"
	"fmt"
	"math"
	"regexp"
	"sync"
	"time"

	"github.com/graphite-ng/carbon-relay-ng/clock"
)

type Func func(in []float64) float64

func Avg(in []float64) float64 {
	if len(in) == 0 {
		panic("avg() called in aggregator with 0 terms")
	}
	return Sum(in) / float64(len(in))
}

func Delta(in []float64) float64 {
	if len(in) == 0 {
		panic("delta() called in aggregator with 0 terms")
	}
	min := in[0]
	max := in[0]
	for _, val := range in {
		if val > max {
			max = val
		} else if val < min {
			min = val
		}
	}
	return max - min
}

func Last(in []float64) float64 {
	if len(in) == 0 {
		panic("last() called in aggregator with 0 terms")
	}
	last := in[len(in)-1]
	return last
}

func Max(in []float64) float64 {
	if len(in) == 0 {
		panic("max() called in aggregator with 0 terms")
	}
	max := in[0]
	for _, val := range in {
		if val > max {
			max = val
		}
	}
	return max
}

func Min(in []float64) float64 {
	if len(in) == 0 {
		panic("min() called in aggregator with 0 terms")
	}
	min := in[0]
	for _, val := range in {
		if val < min {
			min = val
		}
	}
	return min
}

func Stdev(in []float64) float64 {
	if len(in) == 0 {
		panic("stdev() called in aggregator with 0 terms")
	}
	// Get the average (or mean) of the series
	mean := Avg(in)

	// Calculate the variance
	variance := float64(0)
	for _, term := range in {
		variance += math.Pow((float64(term) - mean), float64(2))
	}
	variance /= float64(len(in))

	// Calculate the standard deviation
	return math.Sqrt(variance)
}

func Sum(in []float64) float64 {
	sum := float64(0)
	for _, term := range in {
		sum += term
	}
	return sum
}

var Funcs = map[string]Func{
	"avg":   Avg,
	"delta": Delta,
	"last":  Last,
	"max":   Max,
	"min":   Min,
	"stdev": Stdev,
	"sum":   Sum,
}

type Aggregator struct {
	Fun          string `json:"fun"`
	fn           Func
	in           chan msg       `json:"-"` // incoming metrics, already split in 3 fields
	out          chan []byte    // outgoing metrics
	Regex        string         `json:"regex,omitempty"`
	regex        *regexp.Regexp // compiled version of Regex
	prefix       []byte         // automatically generated based on regex, for fast preMatch
	OutFmt       string
	outFmt       []byte
	Cache        bool
	reCache      map[string]CacheEntry
	Interval     uint                 // expected interval between values in seconds, we will quantize to make sure alginment to interval-spaced timestamps
	Wait         uint                 // seconds to wait after quantized time value before flushing final outcome and ignoring future values that are sent too late.
	aggregations map[aggkey][]float64 // aggregations in process: one for each quantized timestamp and output key, i.e. for each output metric.
	snapReq      chan bool            // chan to issue snapshot requests on
	snapResp     chan *Aggregator     // chan on which snapshot response gets sent
	shutdown     chan struct{}        // chan used internally to shut down
	wg           sync.WaitGroup       // tracks worker running state
	now          func() time.Time     // returns current time. wraps time.Now except in some unit tests
	tick         <-chan time.Time     // controls when to flush
}

type msg struct {
	buf [][]byte
	val float64
	ts  uint32
}

// regexToPrefix inspects the regex and returns the longest static prefix part of the regex
// all inputs for which the regex match, must have this prefix
func regexToPrefix(regex string) []byte {
	substr := ""
	for i := 0; i < len(regex); i++ {
		ch := regex[i]
		if i == 0 {
			if ch == '^' {
				continue // good we need this
			} else {
				break // can't deduce any substring here
			}
		}
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-' {
			substr += string(ch)
			// "\." means a dot character
		} else if ch == 92 && i+1 < len(regex) && regex[i+1] == '.' {
			substr += "."
			i += 1
		} else {
			//fmt.Println("don't know what to do with", string(ch))
			// anything more advanced should be regex syntax that is more permissive and hence not a static substring.
			break
		}
	}
	return []byte(substr)
}

// New creates an aggregator
func New(fun, regex, outFmt string, cache bool, interval, wait uint, out chan []byte) (*Aggregator, error) {
	return NewMocked(fun, regex, outFmt, cache, interval, wait, out, 2000, time.Now, clock.AlignedTick(time.Duration(interval)*time.Second))
}

func NewMocked(fun, regex, outFmt string, cache bool, interval, wait uint, out chan []byte, inBuf int, now func() time.Time, tick <-chan time.Time) (*Aggregator, error) {
	regexObj, err := regexp.Compile(regex)
	if err != nil {
		return nil, err
	}
	fn, ok := Funcs[fun]
	if !ok {
		return nil, fmt.Errorf("no such aggregation function '%s'", fun)
	}
	var reCache map[string]CacheEntry
	if cache {
		reCache = make(map[string]CacheEntry)
	}
	a := &Aggregator{
		fun,
		fn,
		make(chan msg, inBuf),
		out,
		regex,
		regexObj,
		regexToPrefix(regex),
		outFmt,
		[]byte(outFmt),
		cache,
		reCache,
		interval,
		wait,
		make(map[aggkey][]float64),
		make(chan bool),
		make(chan *Aggregator),
		make(chan struct{}),
		sync.WaitGroup{},
		now,
		tick,
	}
	a.wg.Add(1)
	go a.run()
	return a, nil
}

type aggkey struct {
	key string
	ts  uint
}

func (a *Aggregator) AddOrCreate(key string, ts uint, value float64) {
	k := aggkey{
		key,
		ts,
	}
	agg, ok := a.aggregations[k]
	if ok || ts > uint(a.now().Unix())-a.Wait {
		a.aggregations[k] = append(agg, value)
	}
}

// Flush finalizes and removes aggregations that are due
func (a *Aggregator) Flush(ts uint) {
	for k, agg := range a.aggregations {
		if k.ts < ts {
			result := a.fn(agg)
			metric := fmt.Sprintf("%s %f %d", string(k.key), result, k.ts)
			//		log.Debug("aggregator %s-%v-%v values %v -> result %q", a.Fun, a.Regex, a.OutFmt, agg, metric)
			a.out <- []byte(metric)
			delete(a.aggregations, k)
		}
	}
	//fmt.Println("flush done for ", a.now().Unix(), ". agg size now", len(a.aggregations), a.now())
}

func (a *Aggregator) Shutdown() {
	close(a.shutdown)
	a.wg.Wait()
}

func (a *Aggregator) AddMaybe(buf [][]byte, val float64, ts uint32) {
	if a.PreMatch(buf[0]) {
		a.in <- msg{
			buf,
			val,
			ts,
		}
	}
}

//PreMatch checks if the specified metric might match the regex
//by comparing it to the prefix derived from the regex
//if this returns false, the metric will definitely not match the regex and be ignored.
func (a *Aggregator) PreMatch(buf []byte) bool {
	return bytes.HasPrefix(buf, a.prefix)
}

type CacheEntry struct {
	match bool
	key   string
	seen  uint32
}

//
func (a *Aggregator) match(m msg) (string, bool) {
	key := m.buf[0]
	var dst []byte
	matches := a.regex.FindSubmatchIndex(key)
	if matches == nil {
		return "", false
	}
	return string(a.regex.Expand(dst, a.outFmt, key, matches)), true
}

func (a *Aggregator) run() {
	for {
		select {
		case msg := <-a.in:
			// note, we rely here on the fact that the packet has already been validated
			key := msg.buf[0]
			var outKey string
			var ok bool
			if a.reCache != nil {
				entry, ok := a.reCache[string(key)]
				if ok {
					entry.seen = uint32(a.now().Unix())
					a.reCache[string(key)] = entry
					if !entry.match {
						continue
					}
					outKey = entry.key
				} else {
					outKey, ok = a.match(msg)
					a.reCache[string(key)] = CacheEntry{
						ok,
						outKey,
						uint32(a.now().Unix()),
					}
					if !ok {
						continue
					}
				}
			} else {
				outKey, ok = a.match(msg)
				if !ok {
					continue
				}
			}
			ts := uint(msg.ts)
			quantized := ts - (ts % a.Interval)
			a.AddOrCreate(outKey, quantized, msg.val)
		case now := <-a.tick:
			thresh := now.Add(-time.Duration(a.Wait) * time.Second)
			a.Flush(uint(thresh.Unix()))

			// if cache is enabled, clean it out of stale entries
			// it's not ideal to block our channel while flushing AND cleaning up the cache
			// ideally, these operations are interleaved in time, but we can optimize that later
			// this is a simple heuristic but should make the cache always converge on only active data (without memory leaks)
			// even though some cruft may temporarily linger a bit longer.
			// WARNING: this relies on Go's map implementation detail which randomizes iteration order, in order for us to reach
			// the entire keyspace. This may stop working properly with future go releases.  Will need to come up with smth better.
			cutoff := uint32(now.Add(-100 * time.Duration(a.Wait) * time.Second).Unix())
			if a.reCache != nil {
				for k, v := range a.reCache {
					if v.seen < cutoff {
						delete(a.reCache, k)
					}
					break // stop looking when we don't see old entries. we'll look again soon enough.
				}
			}
		case <-a.snapReq:
			aggs := make(map[aggkey][]float64)
			for k, a := range a.aggregations {
				aggs[k] = a
			}
			s := &Aggregator{
				a.Fun,
				a.fn,
				nil,
				nil,
				a.Regex,
				nil,
				a.prefix,
				a.OutFmt,
				nil,
				a.Cache,
				nil,
				a.Interval,
				a.Wait,
				aggs,
				nil,
				nil,
				nil,
				sync.WaitGroup{},
				time.Now,
				nil,
			}

			a.snapResp <- s
		case <-a.shutdown:
			thresh := a.now().Add(-time.Duration(a.Wait) * time.Second)
			a.Flush(uint(thresh.Unix()))
			a.wg.Done()
			return

		}
	}
}

// to view the state of the aggregator at any point in time
func (a *Aggregator) Snapshot() *Aggregator {
	a.snapReq <- true
	return <-a.snapResp
}
