package destination

import (
	"path"
	"time"

	"github.com/graphite-ng/carbon-relay-ng/encoding"
	"github.com/graphite-ng/carbon-relay-ng/metrics"
	"github.com/graphite-ng/carbon-relay-ng/queue"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"go.uber.org/zap"
)

// sits in front of nsqd diskqueue.
// provides buffering (to accept input while storage is slow / sync() runs -every 1000 items- etc)
// QoS (RT vs Bulk) and controllable i/o rates
type Spool struct {
	key          string
	InRT         chan encoding.Datapoint
	InBulk       chan encoding.Datapoint
	Out          chan encoding.Datapoint
	spoolSleep   time.Duration // how long to wait between stores to spool
	unspoolSleep time.Duration // how long to wait between loads from spool

	queue       *queue.Queue
	queueBuffer chan encoding.Datapoint // buffer metrics into queue because it can block

	shutdownWriter chan bool
	shutdownBuffer chan bool
	shutdownReader chan bool
	sm             *metrics.SpoolMetrics
	logger         *zap.Logger

	chunkSize int
}

func NewSpool(key, spoolDir string, bufSize int, maxBytesPerFile, syncEvery int64, syncPeriod, spoolSleep, unspoolSleep time.Duration) *Spool {
	dqName := "spool_" + key
	spoolDir = path.Join(spoolDir, dqName)
	o := opt.Options{
		BlockCacheCapacity:            1024 * 1024 * 50,
		BlockRestartInterval:          64,
		CompactionL0Trigger:           32,
		CompactionTableSizeMultiplier: 5,
		WriteBuffer:                   128 * 1024 * 1024,
		BlockSize:                     512 * 1024,
	}
	queue, err := queue.OpenQueue(spoolDir, &o)
	if err != nil {
		panic(err)
	}
	logger := zap.L().With(zap.String("key", key), zap.String("spool_dir", spoolDir))
	s := Spool{
		key:            key,
		InRT:           make(chan encoding.Datapoint, 10),
		InBulk:         make(chan encoding.Datapoint),
		Out:            make(chan encoding.Datapoint, 100),
		spoolSleep:     spoolSleep,
		unspoolSleep:   unspoolSleep,
		queue:          queue,
		queueBuffer:    make(chan encoding.Datapoint, bufSize),
		shutdownWriter: make(chan bool),
		shutdownBuffer: make(chan bool),
		shutdownReader: make(chan bool),
		sm:             metrics.NewSpoolMetrics("destination", key, nil),
		logger:         logger,
		chunkSize:      1000,
	}
	s.sm.Buffer.Size.Set(float64(maxBytesPerFile))

	go s.Writer()
	go s.Buffer()
	go s.Reader()
	return &s
}

func (s *Spool) write(dp encoding.Datapoint, writeType string) {
	s.sm.IncomingMetrics.WithLabelValues(writeType).Inc()
	pre := time.Now()
	s.logger.Debug("spool satisfying", zap.String("writeType", writeType))
	s.logger.Debug("spool Writer -> queue.Put", zap.Stringer("datapoint", dp))
	s.queueBuffer <- dp
	s.sm.Buffer.WriteDuration.Observe(time.Since(pre).Seconds())
	s.sm.Buffer.BufferedMetrics.Inc()
}

func (s *Spool) Reader() {
	ch := s.Out
	queue := s.queue

	h := encoding.NewPlain(false)
	for {
		if queue.Length() == 0 {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		i, err := queue.Dequeue()
		if err != nil {
			s.logger.Error("failed to dequeue item", zap.Error(err))
			continue
		}
		dp, err := h.Load(i.Value, i.Tags)
		if err != nil {
			s.logger.Error("failed to deserialize datapoint", zap.Error(err))
			continue
		}
		select {
		case <-s.shutdownReader:
			close(ch)
			return
		case ch <- dp:
		}
	}
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
		case buf := <-s.InRT:
			s.write(buf, "RT")
		case buf := <-s.InBulk:
			s.write(buf, "bulk")
		}
	}
}

func (s *Spool) Ingest(bulkData []encoding.Datapoint) {
	for _, dp := range bulkData {
		s.InBulk <- dp
	}
}

func (s *Spool) Buffer() {
	chunk := make([]encoding.Datapoint, 0, s.chunkSize)
	buf := make([]byte, 0, 200)
	for {
		select {
		case <-s.shutdownBuffer:
			return
		case dp := <-s.queueBuffer:
			s.sm.Buffer.BufferedMetrics.Dec()

			pre := time.Now()
			s.queue.Enqueue(dp.AppendToBuf(buf), dp.Tags)
			chunk = chunk[:0]
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
