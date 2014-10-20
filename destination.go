package main

import (
	"errors"
	"fmt"
	statsD "github.com/Dieterbe/statsd-go"
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
	Addr         string         // tcp dest
	spoolDir     string         // where to store spool files (if enabled)
	Spool        bool           // spool metrics to disk while dest down?
	Pickle       bool           // send in pickle format?
	Online       bool           // state of connection online/offline.
	SlowNow      bool           // did we have to drop packets in current loop
	SlowLastLoop bool           // "" last loop
	statsd       *statsD.Client // to submit stats to

	// set automatically in init, passed on in copy
	Reg *regexp.Regexp // compiled version of patt

	// set in/via Run()
	In           chan []byte     // incoming metrics
	shutdown     chan bool       // signals shutdown internally
	queue        *nsqd.DiskQueue // queue used if spooling enabled
	queueIn      chan []byte     // send data to queue
	connUpdates  chan *Conn      // when the dest changes (possibly nil)
	inConnUpdate chan bool       // to signal when we start a new conn and when we finish
}

// after creating, run Run()!
func NewDestination(prefix, sub, regex, addr, spoolDir string, spool, pickle bool, statsd *statsD.Client) (*Destination, error) {
	m, err := NewMatcher(prefix, sub, regex)
	if err != nil {
		return nil, err
	}
	dest := &Destination{
		Matcher:  *m,
		Addr:     addr,
		spoolDir: spoolDir,
		Spool:    spool,
		statsd:   statsd,
		Pickle:   pickle,
	}
	return dest, nil
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
		Matcher:  dest.Matcher.Snapshot(),
		Addr:     dest.Addr,
		spoolDir: dest.spoolDir,
		Spool:    dest.Spool,
		statsd:   dest.statsd,
		Reg:      dest.Reg,
		Pickle:   dest.Pickle,
		Online:   dest.Online,
	}
}

func (dest *Destination) Run() (err error) {
	dest.In = make(chan []byte)
	dest.shutdown = make(chan bool)
	dest.connUpdates = make(chan *Conn)
	dest.inConnUpdate = make(chan bool)
	if dest.Spool {
		dqName := "spool_" + dest.Addr
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
	conn, err := NewConn(addr, dest)
	if err != nil {
		log.Printf("%v: %v\n", dest.Addr, err.Error())
		return
	}
	log.Printf("%v connected to %v\n", dest.Addr, addr)
	if addr != dest.Addr {
		log.Printf("%v update address to %v)\n", dest.Addr, addr)
		dest.Addr = addr
	}
	dest.connUpdates <- conn
	return
}

// TODO func (l *TCPListener) SetDeadline(t time.Time)
// TODO Decide when to drop this buffer and move on.
func (dest *Destination) relay() {
	periodAssureConn := time.Duration(60) * time.Second
	ticker := time.NewTicker(periodAssureConn)
	var toUnspool chan []byte
	var conn *Conn

	// try to send the data on the buffered tcp conn
	// if that's slow or down, discard the data
	nonBlockingSend := func(buf []byte) {
		if dest.Pickle {
			dp, err := parseDataPoint(buf)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				dest.statsd.Increment("dest=" + addrToPath(dest.Addr) + ".target_type=count.unit=Metric.action=drop.reason=bad_pickle")
				return
			}
			buf = pickle(dp)
		}
		select {
		// this op won't succeed as long as the conn is busy processing/flushing
		case conn.In <- buf:
		default:
			// we don't want to just buffer everything in memory,
			// it would probably keep piling up until OOM.  let's just drop the traffic.
			go func() {
				dest.statsd.Increment("dest=" + addrToPath(dest.Addr) + ".target_type=count.unit=Metric.action=drop.reason=slow_conn")
			}()
			dest.SlowNow = true // racey, channelify this
		}
	}

	// try to send the data to the spool
	// if slow or down, drop and move on
	nonBlockingSpool := func(buf []byte) {
		select {
		case dest.queueIn <- buf:
			dest.statsd.Increment("dest=" + addrToPath(dest.Addr) + ".target_type=count.unit=Metric.direction=spool")
		default:
			go func() {
				dest.statsd.Increment("dest=" + addrToPath(dest.Addr) + ".target_type=count.unit=Metric.action=drop.reason=slow_spool")
			}()
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

		select {
		case inConnUpdate := <-dest.inConnUpdate:
			if inConnUpdate {
				numConnUpdates += 1
			} else {
				numConnUpdates -= 1
			}
		// note: new conn can be nil and that's ok (it means we had to [re]connect but couldn't)
		case conn := <-dest.connUpdates:
			dest.Online = conn != nil
		case <-ticker.C: // periodically try to bring connection (back) up, if we have to, and no other connect is happening
			if conn == nil && numConnUpdates == 0 {
				go dest.updateConn(dest.Addr)
			}
			dest.SlowLastLoop = dest.SlowNow
			dest.SlowNow = false
		case <-dest.shutdown:
			//fmt.Println(dest.Addr + " dest relay -> requested shutdown. quitting")
			return
		case buf := <-toUnspool:
			// we know that conn != nil here
			nonBlockingSend(buf)
		case buf := <-dest.In:
			if conn != nil {
				nonBlockingSend(buf)
			} else if dest.Spool {
				nonBlockingSpool(buf)
			} else {
				go func() {
					dest.statsd.Increment("dest=" + addrToPath(dest.Addr) + ".target_type=count.unit=Metric.action=drop.reason=conn_down_no_spool")
				}()
			}
		}
	}
}
