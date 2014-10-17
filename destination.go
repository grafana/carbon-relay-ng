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
	}
	go dest.relay()
	return err
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
	conn, err := NewConn(addr)
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

	sender := make(chan []byte)

	processPacket := func(buf []byte) {
		if conn == nil {
			if dest.Spool {
				dest.statsd.Increment("dest=" + addrToPath(dest.Addr) + ".target_type=count.unit=Metric.direction=spool")
				dest.queue.Put(buf)
			} else {
				// note, we drop packets while we set up connection
				dest.statsd.Increment("dest=" + addrToPath(dest.Addr) + ".target_type=count.unit=Metric.direction=drop")
			}
			return
		}
		dest.statsd.Increment("dest=" + addrToPath(dest.Addr) + ".target_type=count.unit=Metric.direction=out")

		var err error
		var n int
		if dest.Pickle {
			dp, err := parseDataPoint(buf)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return // no point in spooling these again, so just drop the packet
			}
			buf = pickle(dp)
		}

		n, err = conn.Write(buf)
		if err == nil && len(buf) != n {
			err = errors.New(fmt.Sprintf("truncated write: %s", string(buf)))
		}
		if err != nil {
			dest.statsd.Increment("dest=" + addrToPath(dest.Addr) + ".target_type=count.unit=Err")
			log.Println(dest.Addr + " " + err.Error())
			conn.Close()
			dest.connUpdates <- nil
			if dest.Spool {
				fmt.Println("writing to spool")
				dest.queue.Put(buf)
			}
			return
		}
	}

	// process the data, but without blocking!
	// -> try to send the data on the buffered tcp conn
	// -> if it's slow or down, try to write to spool
	// -> if that's slow or down, discard the data
	nonBlockingSend := func(buf []byte) {
		select {
		case sender <- buf:
			processPacket(buf)
		default:
			// sender is doing really bad. sending and spooling to disk queue both can't keep up.
			// we can't keep the routing pipeline blocking, and we also don't want to just buffer everything in memory,
			// it would probably keep piling up until OOM.  let's just drop the traffic.
			go func() {
				dest.statsd.Increment("dest=" + addrToPath(dest.Addr) + ".target_type=count.unit=Metric.direction=drop")
			}()
			dest.SlowNow = true // racey, channelify this
		}
	}

	numConnUpdates := 0
	go dest.updateConn(dest.Addr)

	// this loop/select should never block, we can't hang dest.In or the route & table locks up
	for {
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
			nonBlockingSend(buf)
		case buf := <-dest.In:
			nonBlockingSend(buf)
		}
	}
}
