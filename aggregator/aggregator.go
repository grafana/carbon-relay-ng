package aggregator

import (
	"crypto/md5"
	"fmt"
	"sort"
	"sync"
	"time"

	metrics "github.com/Dieterbe/go-metrics"
	"github.com/grafana/carbon-relay-ng/clock"
	"github.com/grafana/carbon-relay-ng/matcher"
	"github.com/grafana/carbon-relay-ng/stats"
)

type Aggregator struct {
	Fun          string `json:"fun"`
	procConstr   func(val float64, ts uint32) Processor
	in           chan msg    `json:"-"` // incoming metrics, already split in 3 fields
	out          chan []byte // outgoing metrics
	Matcher      matcher.Matcher
	OutFmt       string
	outFmt       []byte
	Cache        bool
	reCache      map[string]CacheEntry
	reCacheMutex sync.Mutex
	Interval     uint                  // expected interval between values in seconds, we will quantize to make sure alginment to interval-spaced timestamps
	Wait         uint                  // seconds to wait after quantized time value before flushing final outcome and ignoring future values that are sent too late.
	DropRaw      bool                  // drop raw values "consumed" by this aggregator
	tsList       []uint                // ordered list of quantized timestamps, so we can flush in correct order
	aggregations map[uint]*aggregation // aggregations in process: one for each quantized timestamp and output key, i.e. for each output metric.
	snapReq      chan bool             // chan to issue snapshot requests on
	snapResp     chan *Aggregator      // chan on which snapshot response gets sent
	shutdown     chan struct{}         // chan used internally to shut down
	wg           sync.WaitGroup        // tracks worker running state
	now          func() time.Time      // returns current time. wraps time.Now except in some unit tests
	tick         <-chan time.Time      // controls when to flush

	Key        string
	numIn      metrics.Counter
	numFlushed metrics.Counter
}

type aggregation struct {
	count uint32
	state map[string]Processor
}

type msg struct {
	buf [][]byte
	val float64
	ts  uint32
}

// New creates an aggregator
func New(fun string, matcher matcher.Matcher, outFmt string, cache bool, interval, wait uint, dropRaw bool, out chan []byte) (*Aggregator, error) {
	ticker := clock.AlignedTick(time.Duration(interval)*time.Second, time.Duration(wait)*time.Second, 2)
	return NewMocked(fun, matcher, outFmt, cache, interval, wait, dropRaw, out, 2000, time.Now, ticker)
}

func NewMocked(fun string, matcher matcher.Matcher, outFmt string, cache bool, interval, wait uint, dropRaw bool, out chan []byte, inBuf int, now func() time.Time, tick <-chan time.Time) (*Aggregator, error) {
	procConstr, err := GetProcessorConstructor(fun)
	if err != nil {
		return nil, err
	}

	a := &Aggregator{
		Fun:          fun,
		procConstr:   procConstr,
		in:           make(chan msg, inBuf),
		out:          out,
		Matcher:      matcher,
		OutFmt:       outFmt,
		outFmt:       []byte(outFmt),
		Cache:        cache,
		Interval:     interval,
		Wait:         wait,
		DropRaw:      dropRaw,
		aggregations: make(map[uint]*aggregation),
		snapReq:      make(chan bool),
		snapResp:     make(chan *Aggregator),
		shutdown:     make(chan struct{}),
		now:          now,
		tick:         tick,
	}
	if cache {
		a.reCache = make(map[string]CacheEntry)
	}
	a.setKey()
	a.numIn = stats.Counter("unit=Metric.direction=in.aggregator=" + a.Key)
	a.numFlushed = stats.Counter("unit=Metric.direction=out.aggregator=" + a.Key)
	a.wg.Add(1)
	go a.run()
	return a, nil
}

type TsSlice []uint

func (p TsSlice) Len() int           { return len(p) }
func (p TsSlice) Less(i, j int) bool { return p[i] < p[j] }
func (p TsSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func (a *Aggregator) setKey() string {
	h := md5.New()
	h.Write([]byte(a.Fun))
	h.Write([]byte("\000"))
	h.Write([]byte(a.Matcher.Regex))
	h.Write([]byte("\000"))
	h.Write([]byte(a.Matcher.NotRegex))
	h.Write([]byte("\000"))
	h.Write([]byte(a.Matcher.Prefix))
	h.Write([]byte("\000"))
	h.Write([]byte(a.Matcher.NotPrefix))
	h.Write([]byte("\000"))
	h.Write([]byte(a.Matcher.Sub))
	h.Write([]byte("\000"))
	h.Write([]byte(a.Matcher.NotSub))
	h.Write([]byte("\000"))
	h.Write([]byte(a.OutFmt))

	key := fmt.Sprintf("%x", h.Sum(nil))
	a.Key = key[:7]
	return a.Key
}

func (a *Aggregator) AddOrCreate(key string, ts uint32, quantized uint, value float64) {
	rangeTracker.Sample(ts)
	agg, ok := a.aggregations[quantized]
	var proc Processor
	if ok {
		proc, ok = agg.state[key]
		if ok {
			// if both levels already exist, we only need to add the value
			agg.count++
			proc.Add(value, ts)
			return
		}
	} else {
		// first level doesn't exist. create it and add the ts to the list
		// (second level will be created below)
		a.tsList = append(a.tsList, quantized)
		if len(a.tsList) > 1 && a.tsList[len(a.tsList)-2] > quantized {
			sort.Sort(TsSlice(a.tsList))
		}
		agg = &aggregation{
			state: make(map[string]Processor),
		}
		a.aggregations[quantized] = agg
	}

	// first level exists but we need to create the 2nd level.

	// note, we only flush where for a given value of now, quantized < now-wait
	// this means that as long as the clock doesn't go back in time
	// we never recreate a previously created bucket (and reflush with same key and ts)
	// a consequence of this is, that if your data stream runs consistently significantly behind
	// real time, it may never be included in aggregates, but it's up to you to configure your wait
	// parameter properly. You can use the rangeTracker and numTooOld metrics to help with this
	if quantized > uint(a.now().Unix())-a.Wait {
		agg.count++
		proc = a.procConstr(value, ts)
		agg.state[key] = proc
		return
	}
	numTooOld.Inc(1)
}

// Flush finalizes and removes aggregations that are due
func (a *Aggregator) Flush(cutoff uint) {
	flushWaiting.Inc(1)
	flushes.Add()
	flushWaiting.Dec(1)
	defer flushes.Done()

	pos := -1 // will track the pos of the last ts position that was successfully processed
	for i, ts := range a.tsList {
		if ts > cutoff {
			break
		}
		agg := a.aggregations[ts]
		for key, proc := range agg.state {
			results, ok := proc.Flush()
			if ok {
				if len(results) == 1 {
					a.out <- []byte(fmt.Sprintf("%s %f %d", key, results[0].val, ts))
					a.numFlushed.Inc(1)
				} else {
					for _, result := range results {
						a.out <- []byte(fmt.Sprintf("%s.%s %f %d", key, result.fcnName, result.val, ts))
						a.numFlushed.Inc(1)
					}
				}
			}
		}
		if aggregatorReporter != nil {
			aggregatorReporter.add(a.Key, uint32(ts), agg.count)
		}
		delete(a.aggregations, ts)
		pos = i
	}
	// now we must delete all the timestamps from the ordered list
	if pos == -1 {
		// we didn't process anything, so no action needed
		return
	}
	if pos == len(a.tsList)-1 {
		// we went through all of them. can just reset the slice
		a.tsList = a.tsList[:0]
		return
	}

	// adjust the slice to only contain the timestamps that still need processing,
	// reusing the backing array
	copy(a.tsList[0:], a.tsList[pos+1:])
	a.tsList = a.tsList[:len(a.tsList)-pos-1]

	//fmt.Println("flush done for ", a.now().Unix(), ". agg size now", len(a.aggregations), a.now())
}

func (a *Aggregator) Shutdown() {
	close(a.shutdown)
	a.wg.Wait()
}

func (a *Aggregator) AddMaybe(buf [][]byte, val float64, ts uint32) bool {
	if !a.Matcher.PreMatch(buf[0]) {
		return false
	}

	if a.DropRaw {
		_, ok := a.matchWithCache(buf[0])
		if !ok {
			return false
		}
	}

	a.in <- msg{
		buf,
		val,
		ts,
	}

	return a.DropRaw
}

type CacheEntry struct {
	match bool
	key   string
	seen  uint32
}

// matchWithCache returns whether there was a match, and under which key, if so.
func (a *Aggregator) matchWithCache(key []byte) (string, bool) {
	if a.reCache == nil {
		return a.Matcher.MatchRegexAndExpand(key, a.outFmt)
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

	outKey, ok = a.Matcher.MatchRegexAndExpand(key, a.outFmt)
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
			outKey, ok := a.matchWithCache(msg.buf[0])
			if !ok {
				continue
			}
			a.numIn.Inc(1)
			ts := uint(msg.ts)
			quantized := ts - (ts % a.Interval)
			a.AddOrCreate(outKey, msg.ts, quantized, msg.val)
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
			if a.reCache != nil {
				cutoff := uint32(now.Add(-100 * time.Duration(a.Wait) * time.Second).Unix())
				a.reCacheMutex.Lock()
				for k, v := range a.reCache {
					if v.seen < cutoff {
						delete(a.reCache, k)
					} else {
						break // stop looking when we don't see old entries. we'll look again soon enough.
					}
				}
				a.reCacheMutex.Unlock()
			}
		case <-a.snapReq:
			aggsCopy := make(map[uint]*aggregation)
			for quant, aggReal := range a.aggregations {
				stateCopy := make(map[string]Processor)
				for key := range aggReal.state {
					stateCopy[key] = nil
				}
				aggsCopy[quant] = &aggregation{
					state: stateCopy,
					count: aggReal.count,
				}
			}
			s := &Aggregator{
				Fun:          a.Fun,
				procConstr:   a.procConstr,
				Matcher:      a.Matcher,
				OutFmt:       a.OutFmt,
				Cache:        a.Cache,
				Interval:     a.Interval,
				Wait:         a.Wait,
				DropRaw:      a.DropRaw,
				aggregations: aggsCopy,
				now:          time.Now,
				Key:          a.Key,
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
