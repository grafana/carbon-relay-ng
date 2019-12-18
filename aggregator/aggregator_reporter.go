package aggregator

import (
	"fmt"
	"sync"
	"time"

	"github.com/grafana/carbon-relay-ng/statsmt"
)

// AggregatorReporter reports the state of aggregation buckets when they are known. (after flushing)
// it reports on buckets by their logical timestamps, not wallclock time
type AggregatorReporter struct {
	sync.Mutex
	buf []aggregate
}

type aggregate struct {
	key   string
	ts    uint32
	count uint32
}

func NewAggregatorReporter() (*AggregatorReporter, error) {
	p := AggregatorReporter{}
	aggregatorReporter = &p
	return statsmt.Register.GetOrAdd("aggregator", &p).(*AggregatorReporter), nil
}

func (r *AggregatorReporter) add(key string, ts, count uint32) {
	r.Lock()
	r.buf = append(r.buf, aggregate{
		key:   key,
		ts:    ts,
		count: count,
	})
	r.Unlock()
}
func (r *AggregatorReporter) ReportGraphite(prefix, buf []byte, now time.Time) []byte {
	r.Lock()
	for _, a := range r.buf {
		buf = statsmt.WriteUint64(buf, prefix, []byte(fmt.Sprintf("aggregator.%s.in_logical", a.key)), uint64(a.count), time.Unix(int64(a.ts), 0))
	}
	r.buf = r.buf[:0]
	r.Unlock()

	return buf
}
