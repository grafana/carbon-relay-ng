package main

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Dieterbe/go-metrics"
)

// some.host:2003 -> some_host_2003
// http://some.host:8080 -> http_some_host_8080
func addrToPath(s string) string {
	s = strings.Replace(s, ".", "_", -1)
	s = strings.Replace(s, ":", "_", -1)
	return strings.Replace(s, "/", "", -1)
}

func addrInstanceSplit(addr string) (string, string) {
	var instance string
	// The address may be specified as server, server:port or server:port:instance.
	if strings.Count(addr, ":") == 2 {
		addrComponents := strings.Split(addr, ":")
		addr = strings.Join(addrComponents[0:2], ":")
		instance = addrComponents[2]
	}
	return addr, instance
}

type Destination struct {
	// basic properties in init and copy
	lockMatcher sync.Mutex
	Matcher     Matcher `json:"matcher"`

	Addr         string `json:"address"`  // tcp dest
	Instance     string `json:"instance"` // Optional carbon instance name, useful only with consistent hashing
	Prepend      string `json:"prepend"`  // a string to prepend to each metric name
	Append       string `json:"append"`   // a string to append to each metric name
	spoolDir     string // where to store spool files (if enabled)
	Spool        bool   `json:"spool"`        // spool metrics to disk while dest down?
	Pickle       bool   `json:"pickle"`       // send in pickle format?
	Online       bool   `json:"online"`       // state of connection online/offline.
	SlowNow      bool   `json:"slowNow"`      // did we have to drop packets in current loop
	SlowLastLoop bool   `json:"slowLastLoop"` // "" last loop
	cleanAddr    string
	periodFlush  time.Duration
	periodReConn time.Duration

	// set in/via Run()
	in           chan []byte // incoming metrics
	shutdown     chan bool   // signals shutdown internally
	spool        *Spool      // queue used if spooling enabled
	connUpdates  chan *Conn  // when the dest changes (possibly nil)
	inConnUpdate chan bool   // to signal when we start a new conn and when we finish
	flush        chan bool
	flushErr     chan error
	tasks        sync.WaitGroup

	numDropNoConnNoSpool metrics.Counter
	numDropSlowSpool     metrics.Counter
	numDropSlowConn      metrics.Counter
}

// NewDestination creates a destination object. Note that it still needs to be told to run via Run().
func NewDestination(prefix, sub, regex, addr, spoolDir, prepend, appendopt string, spool, pickle bool, periodFlush, periodReConn time.Duration) (*Destination, error) {
	m, err := NewMatcher(prefix, sub, regex)
	if err != nil {
		return nil, err
	}
	addr, instance := addrInstanceSplit(addr)
	cleanAddr := addrToPath(addr)
	dest := &Destination{
		Matcher:      *m,
		Addr:         addr,
		Instance:     instance,
		spoolDir:     spoolDir,
		Spool:        spool,
		Pickle:       pickle,
		Prepend:      prepend,
		Append:       appendopt,
		cleanAddr:    cleanAddr,
		periodFlush:  periodFlush,
		periodReConn: periodReConn,
	}
	dest.setMetrics()
	return dest, nil
}

func (dest *Destination) setMetrics() {
	dest.numDropNoConnNoSpool = Counter("dest=" + dest.cleanAddr + ".unit=Metric.action=drop.reason=conn_down_no_spool")
	dest.numDropSlowSpool = Counter("dest=" + dest.cleanAddr + ".unit=Metric.action=drop.reason=slow_spool")
	dest.numDropSlowConn = Counter("dest=" + dest.cleanAddr + ".unit=Metric.action=drop.reason=slow_conn")
}

func (dest *Destination) Match(s []byte) bool {
	dest.lockMatcher.Lock()
	defer dest.lockMatcher.Unlock()
	return dest.Matcher.Match(s)
}

// can't be changed yet: pickle, spool, flush, reconn
func (dest *Destination) Update(opts map[string]string) error {
	matcher := dest.GetMatcher()
	prefix := matcher.Prefix
	sub := matcher.Sub
	regex := matcher.Regex
	updateMatcher := false
	addr := ""

	for name, val := range opts {
		switch name {
		case "addr":
			addr = val
		case "prefix":
			prefix = val
			updateMatcher = true
		case "sub":
			sub = val
			updateMatcher = true
		case "regex":
			regex = val
			updateMatcher = true
		default:
			return errors.New("no such option: " + name)
		}
	}
	if addr != "" {
		dest.updateConn(addr)
	}
	if updateMatcher {
		matcher, err := NewMatcher(prefix, sub, regex)
		if err != nil {
			return err
		}
		dest.UpdateMatcher(*matcher)
	}
	return nil
}

func (dest *Destination) UpdateMatcher(matcher Matcher) {
	dest.lockMatcher.Lock()
	defer dest.lockMatcher.Unlock()
	dest.Matcher = matcher
}

func (dest *Destination) GetMatcher() Matcher {
	dest.lockMatcher.Lock()
	defer dest.lockMatcher.Unlock()
	return dest.Matcher
}

// a "basic" static copy of the dest, not actually running
func (dest *Destination) Snapshot() *Destination {
	return &Destination{
		Matcher:   dest.GetMatcher(),
		Addr:      dest.Addr,
		spoolDir:  dest.spoolDir,
		Spool:     dest.Spool,
		Pickle:    dest.Pickle,
		Online:    dest.Online,
		cleanAddr: dest.cleanAddr,
	}
}

func (dest *Destination) Run() {
	if dest.in != nil {
		panic(fmt.Sprintf("Run() called on already running dest '%s'", dest.Addr))
	}
	dest.in = make(chan []byte)
	dest.shutdown = make(chan bool)
	dest.connUpdates = make(chan *Conn)
	dest.inConnUpdate = make(chan bool)
	dest.flush = make(chan bool)
	dest.flushErr = make(chan error)
	if dest.Spool {
		dest.spool = NewSpool(dest.cleanAddr, dest.spoolDir) // TODO better naming for spool, because it won't update when addr changes
	}
	dest.tasks = sync.WaitGroup{}
	go dest.relay()
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
	addr, instance := addrInstanceSplit(addr)
	conn, err := NewConn(addr, dest, dest.periodFlush, dest.Pickle)
	if err != nil {
		log.Debug("dest %v: %v\n", dest.Addr, err.Error())
		return
	}
	log.Debug("dest %v connected to %v\n", dest.Addr, addr)
	if addr != dest.Addr {
		log.Notice("dest %v update address to %v)\n", dest.Addr, addr)
		dest.Addr = addr
		dest.Instance = instance
		dest.cleanAddr = addrToPath(addr)
		dest.setMetrics()
	}
	dest.connUpdates <- conn
	return
}

func (dest *Destination) collectRedo(conn *Conn) {
	dest.tasks.Add(1)
	bulkData := conn.getRedo()
	dest.spool.Ingest(bulkData)
	dest.tasks.Done()
}

// TODO func (l *TCPListener) SetDeadline(t time.Time)
// TODO Decide when to drop this buffer and move on.
func (dest *Destination) relay() {
	ticker := time.NewTicker(dest.periodReConn)
	var toUnspool chan []byte
	var conn *Conn
	var prependTmp = []byte{}
	var appendTmp = []byte{}
	if len(dest.Prepend) != 0 {
		prependTmp = append(prependTmp, []byte(dest.Prepend)...)
	}

	// try to send the data on the buffered tcp conn
	// if that's slow or down, discard the data
	nonBlockingSend := func(buf []byte) {
		select {
		// this op won't succeed as long as the conn is busy processing/flushing
		case conn.In <- buf:
			conn.numBuffered.Inc(1)
		default:
			log.Info("dest %s %s nonBlockingSend -> dropping due to slow conn\n", dest.Addr, buf)
			// TODO check if it was because conn closed
			// we don't want to just buffer everything in memory,
			// it would probably keep piling up until OOM.  let's just drop the traffic.
			dest.numDropSlowConn.Inc(1)
			dest.SlowNow = true
		}
	}

	// try to send the data to the spool
	// if slow or down, drop and move on
	nonBlockingSpool := func(buf []byte) {
		select {
		case dest.spool.InRT <- buf:
			log.Info("dest %s %s nonBlockingSpool -> added to spool\n", dest.Addr, buf)
		default:
			log.Info("dest %s %s nonBlockingSpool -> dropping due to slow spool\n", dest.Addr, buf)
			dest.numDropSlowSpool.Inc(1)
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
			toUnspool = dest.spool.Out
		} else {
			toUnspool = nil
		}
		log.Debug("dest %v entering select. conn: %v spooling: %v slowLastloop: %v, slowNow: %v spoolQueue: %v", dest.Addr, conn != nil, dest.Spool, dest.SlowLastLoop, dest.SlowNow, toUnspool != nil)
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
			if dest.spool != nil {
				dest.spool.Close()
			}
			return
		case buf := <-toUnspool:
			// we know that conn != nil here because toUnspool is set above
			log.Info("dest %v %s received from spool -> nonBlockingSend\n", dest.Addr, buf)
			nonBlockingSend(buf)
		case buf := <-dest.in:
			if dest.Prepend != "" {
				prependTmp = prependTmp[0:len(dest.Prepend)]
				prependTmp = append(prependTmp, buf...)
				buf = prependTmp
			}

			if dest.Append != "" {
				sp := bytes.IndexByte([]byte(buf), byte(' '))
				if sp < 0 {
					// Didn't find a space, should probably error
				} else {
					appendTmp = appendTmp[:0]
					appendTmp = append(appendTmp, buf[0:sp]...)
					appendTmp = append(appendTmp, []byte(dest.Append)...)
					appendTmp = append(appendTmp, buf[sp:len(buf)]...)
					buf = appendTmp
				}
			}

			if conn != nil {
				log.Info("dest %v %s received from In -> nonBlockingSend\n", dest.Addr, buf)
				nonBlockingSend(buf)
			} else if dest.Spool {
				log.Info("dest %v %s received from In -> nonBlockingSpool\n", dest.Addr, buf)
				nonBlockingSpool(buf)
			} else {
				log.Info("dest %v %s received from In -> no conn no spool -> drop\n", dest.Addr, buf)
				dest.numDropNoConnNoSpool.Inc(1)
			}
		}
	}
}
