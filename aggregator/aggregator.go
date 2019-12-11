package aggregator

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/graphite-ng/carbon-relay-ng/encoding"

	"github.com/graphite-ng/carbon-relay-ng/clock"
	"github.com/graphite-ng/carbon-relay-ng/metrics"
)

type Aggregator struct {
	Fun          string `json:"fun"`
	procConstr   func(val float64, ts uint32) Processor
	in           chan encoding.Datapoint `json:"-"` // incoming metrics, already split in 3 fields
	out          chan encoding.Datapoint // outgoing metrics
	Regex        string                  `json:"regex,omitempty"`
	Prefix       string                  `json:"prefix,omitempty"`
	Sub          string                  `json:"substring,omitempty"`
	regex        *regexp.Regexp          // compiled version of Regex
	prefix       []byte                  // automatically generated based on Prefix or regex, for fast preMatch
	substring    []byte                  // based on Sub, for fast preMatch
	OutFmt       string
	outFmt       []byte
	Cache        bool
	reCache      map[string]CacheEntry
	reCacheMutex sync.Mutex
	Interval     uint                 // expected interval between values in seconds, we will quantize to make sure alginment to interval-spaced timestamps
	Wait         uint                 // seconds to wait after quantized time value before flushing final outcome and ignoring future values that are sent too late.
	DropRaw      bool                 // drop raw values "consumed" by this aggregator
	aggregations map[aggkey]Processor // aggregations in process: one for each quantized timestamp and output key, i.e. for each output metric.
	snapReq      chan bool            // chan to issue snapshot requests on
	snapResp     chan *Aggregator     // chan on which snapshot response gets sent
	shutdown     chan struct{}        // chan used internally to shut down
	wg           sync.WaitGroup       // tracks worker running state
	now          func() time.Time     // returns current time. wraps time.Now except in some unit tests
	tick         <-chan time.Time     // controls when to flush
	am           *metrics.AggregatorMetrics
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
func New(fun, regex, prefix, sub, outFmt string, cache bool, interval, wait uint, dropRaw bool, out chan encoding.Datapoint) (*Aggregator, error) {
	return NewMocked(fun, regex, prefix, sub, outFmt, cache, interval, wait, dropRaw, out, 2000, time.Now, clock.AlignedTick(time.Duration(interval)*time.Second))
}

func NewMocked(fun, regex, prefix, sub, outFmt string, cache bool, interval, wait uint, dropRaw bool, out chan encoding.Datapoint, inBuf int, now func() time.Time, tick <-chan time.Time) (*Aggregator, error) {
	regexObj, err := regexp.Compile(regex)
	if err != nil {
		return nil, err
	}
	procConstr, err := GetProcessorConstructor(fun)
	if err != nil {
		return nil, err
	}

	a := &Aggregator{
		Fun:          fun,
		procConstr:   procConstr,
		in:           make(chan encoding.Datapoint, inBuf),
		out:          out,
		Regex:        regex,
		Sub:          sub,
		regex:        regexObj,
		substring:    []byte(sub),
		OutFmt:       outFmt,
		outFmt:       []byte(outFmt),
		Cache:        cache,
		Interval:     interval,
		Wait:         wait,
		DropRaw:      dropRaw,
		aggregations: make(map[aggkey]Processor),
		snapReq:      make(chan bool),
		snapResp:     make(chan *Aggregator),
		shutdown:     make(chan struct{}),
		now:          now,
		tick:         tick,
		am:           metrics.NewAggregatorMetrics(prefix, nil),
	}
	if prefix != "" {
		a.prefix = []byte(prefix)
		a.Prefix = prefix
	} else {
		a.prefix = regexToPrefix(regex)
		a.Prefix = string(a.prefix)
	}
	if cache {
		a.reCache = make(map[string]CacheEntry)
	}
	a.wg.Add(1)
	go a.run()
	return a, nil
}

type aggkey struct {
	key string
	ts  uint
}

func (a *Aggregator) AddOrCreate(key string, ts uint32, quantized uint, value float64) {
	k := aggkey{
		key,
		quantized,
	}
	a.am.ObserveTimestamp(ts)
	proc, ok := a.aggregations[k]
	if ok {
		proc.Add(value, ts)
	} else {
		// note, we only flush where for a given value of now, quantized < now-wait
		// this means that as long as the clock doesn't go back in time
		// we never recreate a previously created bucket (and reflush with same key and ts)
		// a consequence of this is, that if your data stream runs consistently significantly behind
		// real time, it may never be included in aggregates, but it's up to you to configure your wait
		// parameter properly. You can use the rangeTracker and counterTooOldMetrics metrics to help with this
		if quantized > uint(a.now().Unix())-a.Wait {
			proc = a.procConstr(value, ts)
			a.aggregations[k] = proc
			return
		}
		a.am.Dropped.Inc()
	}
}

// Flush finalizes and removes aggregations that are due
func (a *Aggregator) Flush(ts uint) {
	for k, proc := range a.aggregations {
		if k.ts < ts {
			results, ok := proc.Flush()
			if ok {
				if len(results) == 1 {
					a.out <- encoding.Datapoint{Name: k.key, Value: results[0].val, Timestamp: uint64(k.ts)}
				} else {
					for _, result := range results {
						a.out <- encoding.Datapoint{Name: fmt.Sprintf("%s.%s", k.key, result.fcnName), Value: results[0].val, Timestamp: uint64(k.ts)}
					}
				}
			}
			delete(a.aggregations, k)
		}
	}
	//fmt.Println("flush done for ", a.now().Unix(), ". agg size now", len(a.aggregations), a.now())
}

func (a *Aggregator) Shutdown() {
	close(a.shutdown)
	a.wg.Wait()
}

func (a *Aggregator) AddMaybe(dp encoding.Datapoint) bool {
	if !a.PreMatchString(dp.Name) {
		return false
	}

	if a.DropRaw {
		_, ok := a.matchWithCacheString(dp.Name)
		if !ok {
			return false
		}
	}

	a.in <- dp

	return a.DropRaw
}

//PreMatch checks if the specified metric matches the specified prefix and/or substring
//If prefix isn't explicitly specified it will be derived from the regex where possible.
//If this returns false the metric will not be passed through to the main regex matching stage.
func (a *Aggregator) PreMatchString(s string) bool {
	if len(a.Prefix) > 0 && !strings.HasPrefix(s, a.Prefix) {
		return false
	}
	if len(a.Sub) > 0 && !strings.Contains(s, a.Sub) {
		return false
	}
	return true
}

func (a *Aggregator) PreMatch(buf []byte) bool {
	if len(a.prefix) > 0 && !bytes.HasPrefix(buf, a.prefix) {
		return false
	}
	if len(a.substring) > 0 && !bytes.Contains(buf, a.substring) {
		return false
	}
	return true
}

type CacheEntry struct {
	match bool
	key   string
	seen  uint32
}

//
func (a *Aggregator) match(key []byte) (string, bool) {
	var dst []byte
	matches := a.regex.FindSubmatchIndex(key)
	if matches == nil {
		return "", false
	}
	return string(a.regex.Expand(dst, a.outFmt, key, matches)), true
}

// matchWithCache returns whether there was a match, and under which key, if so.
func (a *Aggregator) matchWithCacheString(key string) (string, bool) {
	return a.matchWithCache([]byte(key))
}

func (a *Aggregator) matchWithCache(key []byte) (string, bool) {
	if a.reCache == nil {
		return a.match(key)
	}

	a.reCacheMutex.Lock()

	var outKey string
	var ok bool
	entry, ok := a.reCache[string(key)]
	if ok {
		entry.seen = uint32(a.now().Unix())
		a.reCache[string(key)] = entry
		a.reCacheMutex.Unlock()
		return entry.key, entry.match
	}

	outKey, ok = a.match(key)

	a.reCache[string(key)] = CacheEntry{
		ok,
		outKey,
		uint32(a.now().Unix()),
	}
	a.reCacheMutex.Unlock()

	return outKey, ok
}

func (a *Aggregator) run() {
	for {
		select {
		case msg := <-a.in:
			// note, we rely here on the fact that the packet has already been validated
			outKey, ok := a.matchWithCacheString(msg.Name)
			if !ok {
				continue
			}
			//TODO: m.conraux Remove int casting logic
			ts := uint(msg.Timestamp)
			quantized := ts - (ts % a.Interval)
			a.AddOrCreate(outKey, uint32(ts), quantized, msg.Value)
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
				a.reCacheMutex.Lock()
				for k, v := range a.reCache {
					if v.seen < cutoff {
						delete(a.reCache, k)
					}
					break // stop looking when we don't see old entries. we'll look again soon enough.
				}
				a.reCacheMutex.Unlock()
			}
		case <-a.snapReq:
			aggs := make(map[aggkey]Processor)
			for k := range a.aggregations {
				aggs[k] = nil
			}
			s := &Aggregator{
				Fun:          a.Fun,
				procConstr:   a.procConstr,
				Regex:        a.Regex,
				Prefix:       a.Prefix,
				Sub:          a.Sub,
				prefix:       a.prefix,
				substring:    a.substring,
				OutFmt:       a.OutFmt,
				Cache:        a.Cache,
				Interval:     a.Interval,
				Wait:         a.Wait,
				DropRaw:      a.DropRaw,
				aggregations: aggs,
				now:          time.Now,
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
