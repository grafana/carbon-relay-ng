package destination

import (
	"time"

	"github.com/graphite-ng/carbon-relay-ng/metrics"
	"github.com/graphite-ng/carbon-relay-ng/nsqd"
	log "github.com/sirupsen/logrus"
)

// sits in front of nsqd diskqueue.
// provides buffering (to accept input while storage is slow / sync() runs -every 1000 items- etc)
// QoS (RT vs Bulk) and controllable i/o rates
type Spool struct {
	key          string
	InRT         chan []byte
	InBulk       chan []byte
	Out          chan []byte
	spoolSleep   time.Duration // how long to wait between stores to spool
	unspoolSleep time.Duration // how long to wait between loads from spool

	queue       *nsqd.DiskQueue
	queueBuffer chan []byte // buffer metrics into queue because it can block

	shutdownWriter chan bool
	shutdownBuffer chan bool
	sm             *metrics.SpoolMetrics
}

// parameters should be tuned so that:
// can buffer packets for the duration of 1 sync
// buffer no more then needed, esp if we know the queue is slower then the ingest rate
func NewSpool(key, spoolDir string, bufSize int, maxBytesPerFile, syncEvery int64, syncPeriod, spoolSleep, unspoolSleep time.Duration) *Spool {
	dqName := "spool_" + key
	// bufSize should be tuned to be able to hold the max amount of metrics that can be received
	// while the disk subsystem is doing a write/sync. Basically set it to the amount of metrics
	// you receive in a second.
	queue := nsqd.NewDiskQueue(dqName, spoolDir, maxBytesPerFile, syncEvery, syncPeriod).(*nsqd.DiskQueue)
	s := Spool{
		key:            key,
		InRT:           make(chan []byte, 10),
		InBulk:         make(chan []byte),
		Out:            NewSlowChan(queue.ReadChan(), unspoolSleep),
		spoolSleep:     spoolSleep,
		unspoolSleep:   unspoolSleep,
		queue:          queue,
		queueBuffer:    make(chan []byte, bufSize),
		shutdownWriter: make(chan bool),
		shutdownBuffer: make(chan bool),
		sm:             metrics.NewSpoolMetrics("destination", key, nil),
	}
	s.sm.Buffer.Size.Set(float64(maxBytesPerFile))

	go s.Writer()
	go s.Buffer()
	return &s
}

func (s *Spool) write(buf []byte, writeType string) {
	s.sm.IncomingMetrics.WithLabelValues(writeType).Inc()
	pre := time.Now()
	log.Debugf("spool %v satisfying spool `%s`", s.key, writeType)
	log.Tracef("spool %s %s Writer -> queue.Put", s.key, buf)
	s.queueBuffer <- buf
	s.sm.Buffer.WriteDuration.Observe(time.Since(pre).Seconds())
	s.sm.Buffer.BufferedMetrics.Inc()
}

// provides a channel based api to the queue
func (s *Spool) Writer() {
	// we always try to serve realtime traffic as much as we can
	// because that's an inputstream at a fixed rate, it won't slow down
	// but if no realtime traffic is coming in, then we can use spare capacity
	// to read from the Bulk input, which is used to offload a known (potentially large) set of data
	// that could easily exhaust the capacity of our disk queue.  But it doesn't require RT processing,
	// so just handle this to the extent we can
	// note that this still allows for channel ops to come in on InRT and to be starved, resulting
	// in some realtime traffic to be dropped, but that shouldn't be too much of an issue. experience will tell..
	for {
		select {
		case <-s.shutdownWriter:
			return
		case buf := <-s.InRT: // wish we could somehow prioritize this higher
			s.write(buf, "RT")
		case buf := <-s.InBulk:
			s.write(buf, "bulk")
		}
	}
}

func (s *Spool) Ingest(bulkData [][]byte) {
	for _, buf := range bulkData {
		s.InBulk <- buf
		time.Sleep(s.spoolSleep)
	}
}

func (s *Spool) Buffer() {
	for {
		select {
		case <-s.shutdownBuffer:
			return
		case buf := <-s.queueBuffer:
			s.sm.Buffer.BufferedMetrics.Dec()
			pre := time.Now()
			s.queue.Put(buf)
			s.sm.WriteDuration.Observe(time.Since(pre).Seconds())
		}
	}
}

func (s *Spool) Close() {
	s.shutdownWriter <- true
	s.shutdownBuffer <- true
	// we don't need to close Out, our user should just not read from it anymore. destination does this
	s.queue.Close()
}
