package main

import (
	"errors"
	"expvar"
	"fmt"
	"github.com/graphite-ng/carbon-relay-ng/nsqd"
	"os"
	"strings"
	"sync"
	"time"
)

func addrToPath(s string) string {
	return strings.Replace(s, ":", "_", -1)
}

type Destination struct {
	// basic properties in init and copy
	Matcher      Matcher `json:"matcher"`
	Addr         string  `json:"address"` // tcp dest
	spoolDir     string  // where to store spool files (if enabled)
	Spool        bool    `json:"spool"`        // spool metrics to disk while dest down?
	Pickle       bool    `json:"pickle"`       // send in pickle format?
	Online       bool    `json:"online"`       // state of connection online/offline.
	SlowNow      bool    `json:"slowNow"`      // did we have to drop packets in current loop
	SlowLastLoop bool    `json:"slowLastLoop"` // "" last loop
	cleanAddr    string
	periodFlush  time.Duration
	periodReConn time.Duration

	// set in/via Run()
	in           chan []byte     // incoming metrics
	shutdown     chan bool       // signals shutdown internally
	queue        *nsqd.DiskQueue // queue used if spooling enabled
	queueInRT    chan []byte     // send data to queue
	queueInBulk  chan []byte     // send data to queue
	connUpdates  chan *Conn      // when the dest changes (possibly nil)
	inConnUpdate chan bool       // to signal when we start a new conn and when we finish
	flush        chan bool
	flushErr     chan error
	tasks        sync.WaitGroup

	numDropNoConnNoSpool *expvar.Int
	numSpool             *expvar.Int
	numDropSlowSpool     *expvar.Int
	numDropSlowConn      *expvar.Int
	numDropBadPickle     *expvar.Int
	numErrTruncated      *expvar.Int
	numErrWrite          *expvar.Int
	numOut               *expvar.Int
}

// after creating, run Run()!
func NewDestination(prefix, sub, regex, addr, spoolDir string, spool, pickle bool, periodFlush, periodReConn time.Duration) (*Destination, error) {
	m, err := NewMatcher(prefix, sub, regex)
	if err != nil {
		return nil, err
	}
	cleanAddr := addrToPath(addr)
	dest := &Destination{
		Matcher:      *m,
		Addr:         addr,
		spoolDir:     spoolDir,
		Spool:        spool,
		Pickle:       pickle,
		cleanAddr:    cleanAddr,
		periodFlush:  periodFlush,
		periodReConn: periodReConn,
	}
	dest.setExpvars()
	return dest, nil
}

func (dest *Destination) setExpvars() {

	dest.numDropNoConnNoSpool = Int("dest=" + dest.cleanAddr + ".target_type=count.unit=Metric.action=drop.reason=conn_down_no_spool")
	dest.numSpool = Int("dest=" + dest.cleanAddr + ".target_type=count.unit=Metric.direction=spool")
	dest.numDropSlowSpool = Int("dest=" + dest.cleanAddr + ".target_type=count.unit=Metric.action=drop.reason=slow_spool")
	dest.numDropSlowConn = Int("dest=" + dest.cleanAddr + ".target_type=count.unit=Metric.action=drop.reason=slow_conn")
	dest.numDropBadPickle = Int("dest=" + dest.cleanAddr + ".target_type=count.unit=Metric.action=drop.reason=bad_pickle")
	dest.numErrTruncated = Int("dest=" + dest.cleanAddr + ".target_type=count.unit=Err.type=truncated")
	dest.numErrWrite = Int("dest=" + dest.cleanAddr + ".target_type=count.unit=Err.type=write")
	dest.numOut = Int("dest=" + dest.cleanAddr + ".target_type=count.unit=Metric.direction=out")
}

func (dest *Destination) Match(s []byte) bool {
	return dest.Matcher.Match(s)
}

func (dest *Destination) UpdateMatcher(matcher Matcher) {
	// TODO: looks like we need lock here, not sure yet how to organize this
	//dest.Lock()
	//defer dest.Unlock()
	dest.Matcher = matcher
}

// a "basic" static copy of the dest, not actually running
func (dest *Destination) Snapshot() Destination {
	return Destination{
		Matcher:   dest.Matcher,
		Addr:      dest.Addr,
		spoolDir:  dest.spoolDir,
		Spool:     dest.Spool,
		Pickle:    dest.Pickle,
		Online:    dest.Online,
		cleanAddr: dest.cleanAddr,
	}
}

func (dest *Destination) Run() (err error) {
	dest.in = make(chan []byte)
	dest.shutdown = make(chan bool)
	dest.connUpdates = make(chan *Conn)
	dest.inConnUpdate = make(chan bool)
	dest.flush = make(chan bool)
	dest.flushErr = make(chan error)
	if dest.Spool {
		dqName := "spool_" + dest.cleanAddr
		dest.queue = nsqd.NewDiskQueue(dqName, dest.spoolDir, 200*1024*1024, 1000, 2*time.Second).(*nsqd.DiskQueue)
		dest.queueInRT = make(chan []byte)
		dest.queueInBulk = make(chan []byte)
	}
	dest.tasks = sync.WaitGroup{}
	go dest.QueueWriter()
	go dest.relay()
	return err
}

// provides a channel based api to the queue
func (dest *Destination) QueueWriter() {
	// we always try to serve realtime traffic as much as we can
	// because that's an inputstream at a fixed rate, it won't slow down
	// but if no realtime traffic is coming in, then we can use spare capacity
	// to read from the Bulk input, which is used to offload a known (potentially large) set of data
	// that could easily exhaust the capacity of our disk queue.  But it doesn't require RT processing,
	// so just handle this to the extent we can
	// note that this still allows for channel ops to come in on queueInRT and to be starved, resulting
	// in some realtime traffic to be dropped, but that shouldn't be too much of an issue. experience will tell..
	for {
		select {
		case buf := <-dest.queueInRT:
			log.Debug("dest %v satisfying spool RT 1")
			dest.queue.Put(buf)
		default:
			select {
			case buf := <-dest.queueInRT:
				log.Debug("dest %v satisfying spool RT 2")
				dest.queue.Put(buf)
			case buf := <-dest.queueInBulk:
				log.Debug("dest %v satisfying spool BULK")
				dest.queue.Put(buf)
			default:
			}
		}
	}
}

func (dest *Destination) Flush() error {
	dest.flush <- true
	return <-dest.flushErr
}

func (dest *Destination) Shutdown() error {
	if dest.shutdown == nil {
		return errors.New("not running yet")
	}
	dest.shutdown <- true
	dest.tasks.Wait()
	return nil
}

func (dest *Destination) updateConn(addr string) {
	log.Debug("dest %v (re)connecting to %v\n", dest.Addr, addr)
	dest.inConnUpdate <- true
	defer func() { dest.inConnUpdate <- false }()
	conn, err := NewConn(addr, dest, dest.periodFlush)
	if err != nil {
		log.Debug("dest %v: %v\n", dest.Addr, err.Error())
		return
	}
	log.Debug("dest %v connected to %v\n", dest.Addr, addr)
	if addr != dest.Addr {
		log.Notice("dest %v update address to %v)\n", dest.Addr, addr)
		dest.Addr = addr
		dest.cleanAddr = addrToPath(addr)
		dest.setExpvars()
	}
	dest.connUpdates <- conn
	return
}

func (dest *Destination) collectRedo(conn *Conn) {
	dest.tasks.Add(1)
	bulkData := conn.getRedo()
	for _, buf := range bulkData {
		dest.queueInBulk <- buf
	}
	dest.tasks.Done()
}

// TODO func (l *TCPListener) SetDeadline(t time.Time)
// TODO Decide when to drop this buffer and move on.
func (dest *Destination) relay() {
	ticker := time.NewTicker(dest.periodReConn)
	var toUnspool chan []byte
	var conn *Conn

	// try to send the data on the buffered tcp conn
	// if that's slow or down, discard the data
	nonBlockingSend := func(buf []byte) {
		if dest.Pickle {
			dp, err := parseDataPoint(buf)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				dest.numDropBadPickle.Add(1)
				return
			}
			buf = pickle(dp)
		}
		select {
		// this op won't succeed as long as the conn is busy processing/flushing
		case conn.In <- buf:
		default:
			log.Warning("dest %s %s nonBlockingSend -> dropping due to slow conn\n", dest.Addr)
			// TODO check if it was because conn closed
			// we don't want to just buffer everything in memory,
			// it would probably keep piling up until OOM.  let's just drop the traffic.
			dest.numDropSlowConn.Add(1)
			dest.SlowNow = true // racey, channelify this
		}
	}

	// try to send the data to the spool
	// if slow or down, drop and move on
	nonBlockingSpool := func(buf []byte) {
		select {
		case dest.queueInRT <- buf:
			dest.numSpool.Add(1)
			log.Info("dest %s %s nonBlockingSpool -> added to spool\n", dest.Addr, string(buf))
		default:
			log.Warning("dest %s nonBlockingSpool -> dropping due to slow spool\n", dest.Addr)
			dest.numDropSlowSpool.Add(1)
		}
	}

	numConnUpdates := 0
	go dest.updateConn(dest.Addr)

	// this loop/select should never block, we can't hang dest.In or the route & table locks up
	for {
		if conn != nil {
			if !conn.isAlive() {
				if dest.Spool {
					go dest.collectRedo(conn)
				}
				conn = nil
			}
		}
		// only process spool queue if we have an outbound connection and we haven't needed to drop packets in a while
		if conn != nil && dest.Spool && !dest.SlowLastLoop && !dest.SlowNow {
			toUnspool = dest.queue.ReadChan()
		} else {
			toUnspool = nil
		}
		log.Notice("dest %v entering select. conn: %v spooling: %v slowLastloop: %v, slowNow: %v spoolQueue: %v", dest.Addr, conn != nil, dest.Spool, dest.SlowLastLoop, dest.SlowNow, toUnspool != nil)
		select {
		case inConnUpdate := <-dest.inConnUpdate:
			if inConnUpdate {
				numConnUpdates += 1
			} else {
				numConnUpdates -= 1
			}
		// note: new conn can be nil and that's ok (it means we had to [re]connect but couldn't)
		case conn = <-dest.connUpdates:
			dest.Online = conn != nil
			log.Notice("dest %s updating conn. online: %v\n", dest.Addr, dest.Online)
			if dest.Online {
				// new conn? start with a clean slate!
				dest.SlowLastLoop = false
				dest.SlowNow = false
			}
		case <-ticker.C: // periodically try to bring connection (back) up, if we have to, and no other connect is happening
			if conn == nil && numConnUpdates == 0 {
				go dest.updateConn(dest.Addr)
			}
			dest.SlowLastLoop = dest.SlowNow
			dest.SlowNow = false
		case <-dest.flush:
			if conn != nil {
				dest.flushErr <- conn.Flush()
			} else {
				dest.flushErr <- nil
			}
		case <-dest.shutdown:
			log.Notice("dest %v shutting down. flushing and closing conn\n", dest.Addr)
			if conn != nil {
				conn.Flush()
				conn.Close()
			}
			return
		case buf := <-toUnspool:
			// we know that conn != nil here because toUnspool is set above
			log.Info("dest %v %s received from spool -> nonBlockingSend\n", dest.Addr, string(buf))
			nonBlockingSend(buf)
		case buf := <-dest.in:
			if conn != nil {
				log.Info("dest %v %s received from In -> nonBlockingSend\n", dest.Addr, string(buf))
				nonBlockingSend(buf)
			} else if dest.Spool {
				log.Info("dest %v %s received from In -> nonBlockingSpool\n", dest.Addr, string(buf))
				nonBlockingSpool(buf)
			} else {
				log.Info("dest %v %s received from In -> no conn no spool -> drop\n", dest.Addr, string(buf))
				dest.numDropNoConnNoSpool.Add(1)
			}
		}
	}
}
