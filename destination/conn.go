package destination

import (
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/Dieterbe/go-metrics"
	"github.com/grafana/carbon-relay-ng/stats"
	log "github.com/sirupsen/logrus"
)

var keepsafe_initial_cap = 100000 // not very important

// this interval should be long enough to capture all failure modes
// (endpoint down, delayed timeout, etc), so it should be at least as long as the flush interval
var keepsafe_keep_duration = time.Duration(10 * time.Second)

var newLine = []byte{'\n'}

// Conn represents a connection to a tcp endpoint.
// As long as conn.isAlive(), caller may write data to conn.In
// when no longer alive, caller must call either getRedo or clearRedo:
// * getRedo to get the last bunch of data which may have not made
// it to the tcp endpoint. After calling getRedo(), no data may be written to conn.In
// it also clears the keepSafe buffer
// * clearRedo: releases the keepSafe buffer
// NOTE: in the future, this design can be much simpler:
// keepSafe doesn't need a separate structure, we could just have an in-line buffer in between the dest and the conn
// since we write buffered chunks in the bufWriter, may as well "keep those safe". e.g. buffered writing and keepSafe
// can be the same buffer. but this requires significant refactoring.
type Conn struct {
	conn        *net.TCPConn
	buffered    *Writer
	shutdown    chan bool
	In          chan []byte
	key         string
	pickle      bool
	flush       chan bool
	flushErr    chan error
	periodFlush time.Duration
	keepSafe    *keepSafe

	numErrTruncated   metrics.Counter
	numErrWrite       metrics.Counter
	numErrFlush       metrics.Counter
	numOut            metrics.Counter // metrics successfully written to our buffered conn (no flushing yet)
	durationWrite     metrics.Timer
	durationTickFlush metrics.Timer     // only updated after successful flush
	durationManuFlush metrics.Timer     // only updated after successful flush
	tickFlushSize     metrics.Histogram // only updated after successful flush. in bytes
	manuFlushSize     metrics.Histogram // only updated after successful flush. in bytes
	numBuffered       metrics.Gauge
	bufferSize        metrics.Gauge
	numDropBadPickle  metrics.Counter

	upMutex sync.RWMutex
	up      bool // true until the conn goes down

	wg sync.WaitGroup
}

func NewConn(key, addr string, periodFlush time.Duration, pickle bool, connBufSize, ioBufSize int) (*Conn, error) {
	raddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, err
	}
	laddr, _ := net.ResolveTCPAddr("tcp", "0.0.0.0")
	conn, err := net.DialTCP("tcp", laddr, raddr)
	if err != nil {
		return nil, err
	}
	connObj := &Conn{
		conn:     conn,
		buffered: NewWriter(conn, ioBufSize, key),
		// when we write to shutdown, HandleData() may not be running anymore to read from the chan
		// but, it may also be in an error scenario in which case it calls c.close() writing a second time to shutdown,
		// after checkEOF has called c.close(). so we need enough room
		shutdown:          make(chan bool, 2),
		In:                make(chan []byte, connBufSize),
		key:               key,
		up:                true,
		pickle:            pickle,
		flush:             make(chan bool),
		flushErr:          make(chan error),
		periodFlush:       periodFlush,
		keepSafe:          NewKeepSafe(keepsafe_initial_cap, keepsafe_keep_duration),
		numErrTruncated:   stats.Counter("dest=" + key + ".unit=Err.type=truncated"),
		numErrWrite:       stats.Counter("dest=" + key + ".unit=Err.type=write"),
		numErrFlush:       stats.Counter("dest=" + key + ".unit=Err.type=flush"),
		numOut:            stats.Counter("dest=" + key + ".unit=Metric.direction=out"),
		durationWrite:     stats.Timer("dest=" + key + ".what=durationWrite"),
		durationTickFlush: stats.Timer("dest=" + key + ".what=durationFlush.type=ticker"),
		durationManuFlush: stats.Timer("dest=" + key + ".what=durationFlush.type=manual"),
		tickFlushSize:     stats.Histogram("dest=" + key + ".unit=B.what=FlushSize.type=ticker"),
		manuFlushSize:     stats.Histogram("dest=" + key + ".unit=B.what=FlushSize.type=manual"),
		numBuffered:       stats.Gauge("dest=" + key + ".unit=Metric.what=numBuffered"),
		bufferSize:        stats.Gauge("dest=" + key + ".unit=Metric.what=bufferSize"),
		numDropBadPickle:  stats.Counter("dest=" + key + ".unit=Metric.action=drop.reason=bad_pickle"),
	}
	connObj.bufferSize.Update(int64(connBufSize))

	connObj.wg.Add(2)
	go connObj.checkEOF()
	go connObj.HandleData()
	return connObj, nil
}

// isAlive returns whether the connection is alive.
// if it is not alive, it has been - or is being - closed.
func (c *Conn) isAlive() bool {
	c.upMutex.RLock()
	up := c.up
	c.upMutex.RUnlock()
	log.Debugf("conn %s .up query responded with %t", c.key, up)
	return up
}

// normally the remote end should never write anything back
// but we know when we get EOF that the other end closed the conn
// if not for this, we can happily write and flush without getting errors (in Go) but getting RST tcp packets back (!)
// props to Tv` for this trick.
func (c *Conn) checkEOF() {
	defer c.wg.Done()
	b := make([]byte, 1024)
	for {
		num, err := c.conn.Read(b)
		if err == io.EOF {
			log.Infof("conn %s .conn.Read returned EOF -> conn is closed. closing conn explicitly", c.key)
			c.close()
			return
		}
		// just in case i misunderstand something or the remote behaves badly
		if num != 0 {
			log.Debugf("conn %s .conn.Read data? did not expect that.  data: %s", c.key, b[:num])
			continue
		}
		if err != io.EOF {
			log.Errorf("conn %s checkEOF .conn.Read returned err != EOF, which is unexpected.  closing conn. error: %s", c.key, err)
			c.close()
			return
		}
	}
}

// all these messages should potentially be resubmitted, because we're not confident about their delivery
// note: getting this data means resetting it! so handle it wisely.
// we also read out the In channel until it blocks.  Don't send any more input after calling this.
func (c *Conn) getRedo() [][]byte {
	// drain In queue in case we still had some data buffered.
	// normally this channel should already have been closed by the time we call this, but this is hard/complicated to enforce
	// so instead let's leverage a select. as soon as it blocks (due to chan close or no more input but not closed yet) we know we're
	// done reading and move on. it's easy to prove in the implementer that we don't send any more data to In after calling this
	defer c.clearRedo()
	for {
		select {
		case buf := <-c.In:
			c.numBuffered.Dec(1)
			c.keepSafe.Add(buf)
		default:
			return c.keepSafe.GetAll()
		}
	}
}

func (c *Conn) alive(alive bool) {
	c.upMutex.Lock()
	c.up = alive
	log.Debugf("conn %s .up set to %v", c.key, alive)
	c.upMutex.Unlock()
}

func (c *Conn) HandleData() {
	defer c.wg.Done()
	periodFlush := c.periodFlush
	tickerFlush := time.NewTicker(periodFlush)
	var now time.Time
	var durationActive time.Duration
	flushSize := int64(0)

	for {
		start := time.Now()
		var active time.Time
		var action string
		select { // handle incoming data or flush/shutdown commands
		// note that Writer.Write() can potentially cause a flush and hence block
		// choose the size of In based on how long these loop iterations take
		case buf := <-c.In:
			// seems to take about 30 micros when writing log to disk, 10 micros otherwise (100k messages/second)
			active = time.Now()
			c.numBuffered.Dec(1)
			action = "write"
			log.Tracef("conn %s HandleData: writing %s", c.key, buf)
			c.keepSafe.Add(buf)
			n, err := c.Write(buf)
			if err != nil {
				log.Warnf("conn %s write error: %s. closing", c.key, err)
				c.close() // this can take a while but that's ok. this conn won't be used anymore
				return
			}
			c.numOut.Inc(1)
			flushSize += int64(n)
			now = time.Now()
			durationActive = now.Sub(active)
			c.durationWrite.Update(durationActive)
		case <-tickerFlush.C:
			active = time.Now()
			action = "auto-flush"
			log.Debugf("conn %s HandleData: c.buffered auto-flushing...", c.key)
			err := c.buffered.Flush()
			if err != nil {
				log.Warnf("conn %s HandleData c.buffered auto-flush done but with error: %s, closing", c.key, err)
				c.numErrFlush.Inc(1)
				c.close()
				return
			}
			log.Debugf("conn %s HandleData c.buffered auto-flush done without error", c.key)
			now = time.Now()
			durationActive = now.Sub(active)
			c.durationTickFlush.Update(durationActive)
			c.tickFlushSize.Update(flushSize)
			flushSize = 0
		case <-c.flush:
			active = time.Now()
			action = "manual-flush"
			log.Debugf("conn %s HandleData: c.buffered manual flushing...", c.key)
			err := c.buffered.Flush()
			c.flushErr <- err
			if err != nil {
				log.Warnf("conn %s HandleData c.buffered manual flush done but witth error: %s, closing", c.key, err)
				// TODO instrument
				c.close()
				return
			}
			log.Infof("conn %s HandleData c.buffered manual flush done without error", c.key)
			now = time.Now()
			durationActive = now.Sub(active)
			c.durationManuFlush.Update(durationActive)
			c.manuFlushSize.Update(flushSize)
			flushSize = 0
		case <-c.shutdown:
			log.Debugf("conn %s HandleData: shutdown received. returning.", c.key)
			return
		}
		log.Debugf("conn %s HandleData %s %s (total iter %s) (use this to tune your In buffering)", c.key, action, durationActive, now.Sub(start))
	}
}

// returns a network/write error, so that it can be retried later
// deals with pickle errors internally because retrying wouldn't help anyway
func (c *Conn) Write(buf []byte) (int, error) {
	if c.pickle {
		dp, err := ParseDataPoint(buf)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			c.numDropBadPickle.Inc(1)
			return 0, nil
		}
		buf = Pickle(dp)
	}
	written := 0
	size := len(buf)
	n, err := c.buffered.Write(buf)
	written += n
	if err == nil && size == n && !c.pickle {
		size = 1
		n, err = c.buffered.Write(newLine)
		written += n
	}
	if err != nil {
		c.numErrWrite.Inc(1)
	}
	if err == nil && size != n {
		c.numErrTruncated.Inc(1)
		err = fmt.Errorf("truncated write: %s", buf)
	}
	return written, err
}

func (c *Conn) Flush() error {
	log.Debugf("conn %s going to flush my buffer", c.key)
	c.flush <- true
	log.Debugf("conn %s waiting for flush, getting error.", c.key)
	return <-c.flushErr
}

func (c *Conn) close() {
	c.alive(false)
	log.Debugf("conn %s close() called. sending shutdown", c.key)
	c.shutdown <- true
	log.Debugf("conn %s c.conn.Close()", c.key)
	err := c.conn.Close()
	if err != nil {
		log.Warnf("conn %s error closing: %s", c.key, err)
		return
	}
	log.Debugf("conn %s c.conn.Close() complete", c.key)
}

// Close closes the connection and releases all resources, with the exception of the
// keepSafe buffer. because the caller of conn needs a chance to collect that data
func (c *Conn) Close() {
	c.close()
	log.Debugf("conn %s Close() waiting", c.key)
	c.wg.Wait()
	log.Debugf("conn %s Close() complete", c.key)
}

// clearRedo releases the keepSafe resources
func (c *Conn) clearRedo() {
	log.Debugf("conn %s c.keepSafe.Stop()", c.key)
	c.keepSafe.Stop()
}
