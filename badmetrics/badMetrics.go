package badmetrics

import (
	"sort"
	"time"
)

type BadMetrics struct {
	maxAge  time.Duration
	seen    map[string]Record
	In      chan Record
	getReq  chan time.Time
	getResp chan []Record
}

type Record struct {
	Metric   string // the key parsed, or "" if parse failure
	LastMsg  string // metric line read
	LastErr  string
	LastSeen time.Time
}

// ByMetric implements sort.Interface for []Record based on
// the Metric field.
type ByMetric []Record

func (a ByMetric) Len() int           { return len(a) }
func (a ByMetric) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByMetric) Less(i, j int) bool { return a[i].Metric < a[j].Metric }

// maxAge is the age after which we expire old records (in practice a bit later)
func New(maxAge time.Duration) *BadMetrics {
	b := &BadMetrics{
		maxAge,
		make(map[string]Record),
		// needs to big enough so we don't start blocking when cleans or Get()'s happen
		// if this fills up, Add() starts blocking, which blocks the table.
		make(chan Record, 100000),
		make(chan time.Time),
		make(chan []Record),
	}
	go b.manage()
	return b
}

func (b *BadMetrics) Get(expiry time.Duration) []Record {
	b.getReq <- time.Now().Add(-expiry)
	filtered := <-b.getResp
	sort.Sort(ByMetric(filtered))
	return filtered
}

func (b *BadMetrics) Add(metric []byte, msg []byte, err error) {
	b.In <- Record{
		string(metric),
		string(msg),
		err.Error(),
		time.Now(),
	}
}

func (b *BadMetrics) manage() {
	clean := time.NewTicker(b.maxAge / 10)
	for {
		select {
		case in := <-b.In:
			b.seen[in.Metric] = in
		case <-clean.C:
			cutoff := time.Now().Add(-b.maxAge)
			for metric, record := range b.seen {
				if record.LastSeen.Before(cutoff) {
					delete(b.seen, metric)
				}
			}
		case oldest := <-b.getReq:
			filtered := make([]Record, 0, len(b.seen))
			for _, record := range b.seen {
				if record.LastSeen.After(oldest) {
					filtered = append(filtered, record)
				}
			}
			b.getResp <- filtered
		}
	}
}
