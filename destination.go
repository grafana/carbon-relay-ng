package main

import (
	"errors"
	"expvar"
	"fmt"
	"github.com/graphite-ng/carbon-relay-ng/nsqd"
	"log"
	"os"
	"regexp"
	"strings"
	"time"
)

func addrToPath(s string) string {
	return strings.Replace(s, ":", "_", -1)
}

type Destination struct {
	// basic properties in init and copy
	Matcher      Matcher
	Addr         string // tcp dest
	spoolDir     string // where to store spool files (if enabled)
	Spool        bool   // spool metrics to disk while dest down?
	Pickle       bool   // send in pickle format?
	Online       bool   // state of connection online/offline.
	SlowNow      bool   // did we have to drop packets in current loop
	SlowLastLoop bool   // "" last loop
	CleanAddr    string
	periodFlush  time.Duration
	periodReConn time.Duration

	// set automatically in init, passed on in copy
	Reg *regexp.Regexp // compiled version of patt

	// set in/via Run()
	In           chan []byte     // incoming metrics
	shutdown     chan bool       // signals shutdown internally
	queue        *nsqd.DiskQueue // queue used if spooling enabled
	queueIn      chan []byte     // send data to queue
	connUpdates  chan *Conn      // when the dest changes (possibly nil)
	inConnUpdate chan bool       // to signal when we start a new conn and when we finish
	flush        chan bool
	flushErr     chan error

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
		CleanAddr:    cleanAddr,
		periodFlush:  periodFlush,
		periodReConn: periodReConn,
	}
	dest.setExpvars()
	return dest, nil
}

func (dest *Destination) setExpvars() {

	dest.numDropNoConnNoSpool = Int("dest=" + dest.CleanAddr + ".target_type=count.unit=Metric.action=drop.reason=conn_down_no_spool")
	dest.numSpool = Int("dest=" + dest.CleanAddr + ".target_type=count.unit=Metric.direction=spool")
	dest.numDropSlowSpool = Int("dest=" + dest.CleanAddr + ".target_type=count.unit=Metric.action=drop.reason=slow_spool")
	dest.numDropSlowConn = Int("dest=" + dest.CleanAddr + ".target_type=count.unit=Metric.action=drop.reason=slow_conn")
	dest.numDropBadPickle = Int("dest=" + dest.CleanAddr + ".target_type=count.unit=Metric.action=drop.reason=bad_pickle")
	dest.numErrTruncated = Int("dest=" + dest.CleanAddr + ".target_type=count.unit=Err.type=truncated")
	dest.numErrWrite = Int("dest=" + dest.CleanAddr + ".target_type=count.unit=Err.type=write")
	dest.numOut = Int("dest=" + dest.CleanAddr + ".target_type=count.unit=Metric.direction=out")
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
		Matcher:   dest.Matcher.Snapshot(),
		Addr:      dest.Addr,
		spoolDir:  dest.spoolDir,
		Spool:     dest.Spool,
		Reg:       dest.Reg,
		Pickle:    dest.Pickle,
		Online:    dest.Online,
		CleanAddr: dest.CleanAddr,
	}
}

func (dest *Destination) Run() (err error) {
	dest.In = make(chan []byte)
	dest.shutdown = make(chan bool)
	dest.connUpdates = make(chan *Conn)
	dest.inConnUpdate = make(chan bool)
	dest.flush = make(chan bool)
	dest.flushErr = make(chan error)
	if dest.Spool {
		dqName := "spool_" + dest.CleanAddr
		dest.queue = nsqd.NewDiskQueue(dqName, dest.spoolDir, 200*1024*1024, 1000, 2*time.Second).(*nsqd.DiskQueue)
		dest.queueIn = make(chan []byte)
	}
	go dest.QueueWriter()
	go dest.relay()
	return err
}

// provides a channel based api to the queue
func (dest *Destination) QueueWriter() {
	for buf := range dest.queueIn {
		dest.queue.Put(buf)
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
	return nil
}

func (dest *Destination) updateConn(addr string) {
	log.Printf("%v (re)connecting to %v\n", dest.Addr, addr)
	dest.inConnUpdate <- true
	defer func() { dest.inConnUpdate <- false }()
	conn, err := NewConn(addr, dest, dest.periodFlush)
	if err != nil {
		log.Printf("%v: %v\n", dest.Addr, err.Error())
		return
	}
	log.Printf("%v connected to %v\n", dest.Addr, addr)
	if addr != dest.Addr {
		log.Printf("%v update address to %v)\n", dest.Addr, addr)
		dest.Addr = addr
		dest.CleanAddr = addrToPath(addr)
		dest.setExpvars()
	}
	dest.connUpdates <- conn
	return
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
			log.Printf("%s DROPPING DUE TO SLOW CONN (just to be safe, conn.In is %v)\n", dest.Addr, conn.In)
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
		case dest.queueIn <- buf:
			dest.numSpool.Add(1)
		default:
			dest.numDropSlowSpool.Add(1)
		}
	}

	numConnUpdates := 0
	go dest.updateConn(dest.Addr)

	// this loop/select should never block, we can't hang dest.In or the route & table locks up
	for {
		if conn != nil {
			if !conn.isAlive() {
				conn = nil
			}
		}
		// only process spool queue if we have an outbound connection and we haven't needed to drop packets in a while
		if conn != nil && dest.Spool && !dest.SlowLastLoop && !dest.SlowNow {
			toUnspool = dest.queue.ReadChan()
		} else {
			toUnspool = nil
		}

		log.Printf("#### %s entering the main select ######\n", dest.Addr)
		//if conn == nil {
		//    log.Printf("#### %s has conn nil ######\n", dest.Addr)
		// }
		select {
		case inConnUpdate := <-dest.inConnUpdate:
			if inConnUpdate {
				numConnUpdates += 1
			} else {
				numConnUpdates -= 1
			}
		// note: new conn can be nil and that's ok (it means we had to [re]connect but couldn't)
		case conn = <-dest.connUpdates:
			log.Printf("%s updating conn to %v\n", dest.Addr, conn)
			dest.Online = conn != nil
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
			log.Printf("%v shutting down\n", dest.Addr)
			if conn != nil {
				conn.Flush()
				conn.Close()
			}
			return
		case buf := <-toUnspool:
			// we know that conn != nil here
			log.Printf("%v received from spool: %s\n", dest.Addr, string(buf))
			nonBlockingSend(buf)
		case buf := <-dest.In:
			log.Printf("%v received from In: %s\n", dest.Addr, string(buf))
			if conn != nil {
				nonBlockingSend(buf)
			} else if dest.Spool {
				nonBlockingSpool(buf)
			} else {
				dest.numDropNoConnNoSpool.Add(1)
			}
		}
	}
}
