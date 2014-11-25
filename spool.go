package main

import (
	"github.com/Dieterbe/go-metrics"
	"github.com/graphite-ng/carbon-relay-ng/nsqd"
	"time"
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

	durationWrite  metrics.Timer
	durationBuffer metrics.Timer
	numBuffered    metrics.Counter // track watermark on read and write
	numWritten     metrics.Counter
	// metrics we could do but i don't think that useful: diskqueue depth, amount going in/out diskqueue
	numIncomingBulk metrics.Counter // sync channel, no need to track watermark, instead we track number seen on read
	numIncomingRT   metrics.Counter // more or less sync (small buff). we track number of drops in dest so no need for watermark, instead we track num seen on read
}

// parameters should be tuned so that:
// can buffer packets for the duration of 1 sync
// buffer no more then needed, esp if we know the queue is slower then the ingest rate
func NewSpool(key, spoolDir string, spoolSleep, unspoolSleep time.Duration) *Spool {
	dqName := "spool_" + key
	queue := nsqd.NewDiskQueue(dqName, spoolDir, 200*1024*1024, 500, 2*time.Second).(*nsqd.DiskQueue)

	s := Spool{
		key:             key,
		InRT:            make(chan []byte, 10),
		InBulk:          make(chan []byte),
		Out:             NewSlowChan(queue.ReadChan(), unspoolSleep),
		spoolSleep:      spoolSleep,
		unspoolSleep:    unspoolSleep,
		queue:           queue,
		queueBuffer:     make(chan []byte, 800), // TODO configurable
		durationWrite:   Timer("spool=" + key + ".unit=ns.operation=write"),
		durationBuffer:  Timer("spool=" + key + ".unit=ns.operation=buffer"),
		numBuffered:     Counter("spool=" + key + ".unit=Metric.status=buffered"),
		numIncomingRT:   Counter("spool=" + key + ".target_type=count.unit=Metric.status=incomingRT"),
		numIncomingBulk: Counter("spool=" + key + ".target_type=count.unit=Metric.status=incomingBulk"),
	}
	go s.Writer()
	go s.Buffer()
	return &s
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
		// it looks like this is never taken though, it's always RT 2 . oh well.
		// because: first time we're here, there's probably nothing here, so it hangs in the default
		// when it processed default:
		// it just read from InRT: it's highly unlikely there's another msg ready, so we go to default again
		// it just read from InBulk: *because* there was nothing on RT, so it's highly ulikely something will be there straight after, so default again
		case buf := <-s.InRT:
			s.numIncomingRT.Inc(1)
			//pre = time.Now()
			log.Debug("spool %v satisfying spool RT 1", s.key)
			log.Info("spool %s %s Writer -> queue.Put\n", s.key, string(buf))
			s.durationBuffer.Time(func() { s.queueBuffer <- buf })
			s.numBuffered.Inc(1)
			//post = time.Now()
			//fmt.Println("queueBuffer duration RT 1:", post.Sub(pre).Nanoseconds())
		default:
			select {
			case buf := <-s.InRT:
				s.numIncomingRT.Inc(1)
				//pre = time.Now()
				log.Debug("spool %v satisfying spool RT 2", s.key)
				log.Info("spool %s %s Writer -> queue.Put\n", s.key, string(buf))
				s.durationBuffer.Time(func() { s.queueBuffer <- buf })
				s.numBuffered.Inc(1)
				//post = time.Now()
				//fmt.Println("queueBuffer duration RT 2:", post.Sub(pre).Nanoseconds())
			case buf := <-s.InBulk:
				s.numIncomingBulk.Inc(1)
				//pre = time.Now()
				log.Debug("spool %v satisfying spool BULK", s.key)
				log.Info("spool %s %s Writer -> queue.Put\n", s.key, string(buf))
				s.durationBuffer.Time(func() { s.queueBuffer <- buf })
				s.numBuffered.Inc(1)
				//post = time.Now()
				//fmt.Println("queueBuffer duration BULK:", post.Sub(pre).Nanoseconds())
			}
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
	// TODO clean shutdown
	for buf := range s.queueBuffer {
		s.numBuffered.Dec(1)
		//pre := time.Now()
		s.durationWrite.Time(func() { s.queue.Put(buf) })
		//post := time.Now()
		//fmt.Println("PUT DURATION", post.Sub(pre).Nanoseconds())
	}
}
