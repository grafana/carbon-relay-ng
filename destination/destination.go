package destination

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/graphite-ng/carbon-relay-ng/matcher"
	"github.com/graphite-ng/carbon-relay-ng/util"
	log "github.com/sirupsen/logrus"
)

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
	Matcher     matcher.Matcher `json:"matcher"`

	Addr         string `json:"address"`  // tcp dest
	Instance     string `json:"instance"` // Optional carbon instance name, useful only with consistent hashing
	SpoolDir     string // where to store spool files (if enabled)
	Key          string // unique key per destination, based on routeName and destination addr/port combination
	Spool        bool   `json:"spool"`        // spool metrics to disk while dest down?
	Pickle       bool   `json:"pickle"`       // send in pickle format?
	Online       bool   `json:"online"`       // state of connection online/offline.
	SlowNow      bool   `json:"slowNow"`      // did we have to drop packets in current loop
	SlowLastLoop bool   `json:"slowLastLoop"` // "" last loop
	periodFlush  time.Duration
	periodReConn time.Duration
	connBufSize  int // in metrics. (each metric line is typically about 70 bytes). default 30k. to make sure writes to In are fast until conn flushing can't keep up
	ioBufSize    int // conn io buffer in bytes. 4096 is go default. 2M is our default

	SpoolBufSize         int
	SpoolMaxBytesPerFile int64
	SpoolSyncEvery       int64
	SpoolSyncPeriod      time.Duration
	SpoolSleep           time.Duration // how long to wait between stores to spool
	UnspoolSleep         time.Duration // how long to wait between loads from spool
	RouteName            string

	// set in/via Run()
	In                  chan []byte        `json:"-"` // incoming metrics
	shutdown            chan bool          // signals shutdown internally
	spool               *Spool             // queue used if spooling enabled
	connUpdates         chan *Conn         // channel for newly created connection. It replaces any previous connection
	inConnUpdate        chan bool          // to signal when we start a new conn and when we finish
	setSignalConnOnline chan chan struct{} // the provided chan will be closed when the conn comes online (internal implementation detail)
	flush               chan bool
	flushErr            chan error
	tasks               sync.WaitGroup
}

// New creates a destination object. Note that it still needs to be told to run via Run().
func New(routeName, prefix, sub, regex, addr, spoolDir string, spool, pickle bool, periodFlush, periodReConn time.Duration, connBufSize, ioBufSize, spoolBufSize int, spoolMaxBytesPerFile, spoolSyncEvery int64, spoolSyncPeriod, spoolSleep, unspoolSleep time.Duration) (*Destination, error) {
	m, err := matcher.New(prefix, sub, regex)
	if err != nil {
		return nil, err
	}
	key := util.Key(routeName, addr)
	addr, instance := addrInstanceSplit(addr)
	dest := &Destination{
		Matcher:              *m,
		Addr:                 addr,
		Instance:             instance,
		SpoolDir:             spoolDir,
		Key:                  key,
		Spool:                spool,
		Pickle:               pickle,
		periodFlush:          periodFlush,
		periodReConn:         periodReConn,
		connBufSize:          connBufSize,
		ioBufSize:            ioBufSize,
		SpoolBufSize:         spoolBufSize,
		SpoolMaxBytesPerFile: spoolMaxBytesPerFile,
		SpoolSyncEvery:       spoolSyncEvery,
		SpoolSyncPeriod:      spoolSyncPeriod,
		SpoolSleep:           spoolSleep,
		UnspoolSleep:         unspoolSleep,
		RouteName:            routeName,
	}
	return dest, nil
}

func (dest *Destination) Match(s []byte) bool {
	dest.lockMatcher.Lock()
	defer dest.lockMatcher.Unlock()
	return dest.Matcher.Match(s)
}

// can't be changed yet: pickle, spool, flush, reconn
func (dest *Destination) Update(opts map[string]string) error {
	match := dest.GetMatcher()
	prefix := match.Prefix
	sub := match.Sub
	regex := match.Regex
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
		match, err := matcher.New(prefix, sub, regex)
		if err != nil {
			return err
		}
		dest.UpdateMatcher(*match)
	}
	return nil
}

func (dest *Destination) UpdateMatcher(matcher matcher.Matcher) {
	dest.lockMatcher.Lock()
	defer dest.lockMatcher.Unlock()
	dest.Matcher = matcher
}

func (dest *Destination) GetMatcher() matcher.Matcher {
	dest.lockMatcher.Lock()
	defer dest.lockMatcher.Unlock()
	return dest.Matcher
}

// a "basic" static copy of the dest, not actually running
func (dest *Destination) Snapshot() *Destination {
	return &Destination{
		Matcher:  dest.GetMatcher(),
		Addr:     dest.Addr,
		SpoolDir: dest.SpoolDir,
		Spool:    dest.Spool,
		Pickle:   dest.Pickle,
		Online:   dest.Online,
		Key:      dest.Key,
	}
}

func (dest *Destination) Run() {
	if dest.In != nil {
		panic(fmt.Sprintf("Run() called on already running dest %q", dest.Key))
	}
	dest.In = make(chan []byte)
	dest.shutdown = make(chan bool)
	dest.connUpdates = make(chan *Conn)
	dest.inConnUpdate = make(chan bool)
	dest.flush = make(chan bool)
	dest.flushErr = make(chan error)
	dest.setSignalConnOnline = make(chan chan struct{})
	if dest.Spool {
		// TODO better naming for spool, because it won't update when addr changes
		dest.spool = NewSpool(
			dest.Key,
			dest.SpoolDir,
			dest.SpoolBufSize,
			dest.SpoolMaxBytesPerFile,
			dest.SpoolSyncEvery,
			dest.SpoolSyncPeriod,
			dest.SpoolSleep,
			dest.UnspoolSleep,
		)
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

// TODO: Fix Instance key update
func (dest *Destination) updateConn(addr string) {
	log.Debugf("dest %v (re)connecting to %v", dest.Key, addr)
	dest.inConnUpdate <- true
	defer func() { dest.inConnUpdate <- false }()
	key := util.Key(dest.RouteName, addr)
	addr, instance := addrInstanceSplit(addr)
	conn, err := NewConn(dest.Key, addr, dest.periodFlush, dest.Pickle, dest.connBufSize, dest.ioBufSize)
	if err != nil {
		log.Debugf("dest %v: %v", dest.Key, err.Error())
		return
	}
	log.Debugf("dest %v connected to %v", dest.Key, addr)
	if addr != dest.Addr {
		log.Infof("dest %v update address to %v", dest.Key, addr)
		dest.Addr = addr
		dest.Instance = instance
		dest.Key = key
	}
	dest.connUpdates <- conn
	return
}

func (dest *Destination) collectRedo(conn *Conn) {
	bulkData := conn.getRedo()
	dest.spool.Ingest(bulkData)
	dest.tasks.Done()
}

func (dest *Destination) WaitOnline() chan struct{} {
	signalConnOnline := make(chan struct{})
	dest.setSignalConnOnline <- signalConnOnline
	return signalConnOnline
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
		select {
		// this op won't succeed as long as the conn is busy processing/flushing
		case conn.In <- buf:
			conn.bm.BufferedMetrics.Inc()
		default:
			log.Tracef("dest %s %s nonBlockingSend -> dropping due to slow conn", dest.Key, buf)
			// TODO check if it was because conn closed
			// we don't want to just buffer everything in memory,
			// it would probably keep piling up until OOM.  let's just drop the traffic.
			droppedMetricsCounter.WithLabelValues(dest.Key, "slow_conn").Inc()
			dest.SlowNow = true
		}
	}

	// try to send the data to the spool
	// if slow or down, drop and move on
	nonBlockingSpool := func(buf []byte) {
		select {
		case dest.spool.InRT <- buf:
			log.Tracef("dest %s %s nonBlockingSpool -> added to spool", dest.Key, buf)
		default:
			log.Tracef("dest %s %s nonBlockingSpool -> dropping due to slow spool", dest.Key, buf)
			droppedMetricsCounter.WithLabelValues(dest.Key, "slow_pool").Inc()
		}
	}

	numConnUpdates := 0
	go dest.updateConn(dest.Addr)
	var signalConnOnline chan struct{}

	// this loop/select should never block, we can't hang dest.In or the route & table locks up
	for {
		if conn != nil {
			if !conn.isAlive() {
				dest.Online = false
				if dest.Spool {
					dest.tasks.Add(1)
					go dest.collectRedo(conn)
				} else {
					conn.clearRedo()
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
		log.Debugf("dest %v entering select. conn: %v spooling: %v slowLastloop: %v, slowNow: %v spoolQueue: %v", dest.Key, conn != nil, dest.Spool, dest.SlowLastLoop, dest.SlowNow, toUnspool != nil)
		select {
		case sig := <-dest.setSignalConnOnline:
			signalConnOnline = sig
		case inConnUpdate := <-dest.inConnUpdate:
			if inConnUpdate {
				numConnUpdates += 1
			} else {
				numConnUpdates -= 1
			}
		case conn = <-dest.connUpdates:
			dest.Online = true
			log.Infof("dest %s new conn online", dest.Key)
			// new conn? start with a clean slate!
			dest.SlowLastLoop = false
			dest.SlowNow = false
			if signalConnOnline != nil {
				close(signalConnOnline)
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
			log.Infof("dest %v shutting down. flushing and closing conn", dest.Key)
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
			log.Tracef("dest %v %s received from spool -> nonBlockingSend", dest.Key, buf)
			nonBlockingSend(buf)
		case buf := <-dest.In:
			if conn != nil {
				log.Tracef("dest %v %s received from In -> nonBlockingSend", dest.Key, buf)
				nonBlockingSend(buf)
			} else if dest.Spool {
				log.Tracef("dest %v %s received from In -> nonBlockingSpool", dest.Key, buf)
				nonBlockingSpool(buf)
			} else {
				log.Tracef("dest %v %s received from In -> no conn no spool -> drop", dest.Key, buf)
				droppedMetricsCounter.WithLabelValues(dest.Key, "conn_down_no_spool").Inc()
			}
		}
	}
}
