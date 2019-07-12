package destination

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/graphite-ng/carbon-relay-ng/encoding"

	"github.com/graphite-ng/carbon-relay-ng/metrics"
	"github.com/prometheus/client_golang/prometheus"
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
	buffered    *bufio.Writer
	shutdown    chan bool
	In          chan encoding.Datapoint
	key         string
	pickle      bool
	flush       chan bool
	flushErr    chan error
	periodFlush time.Duration
	keepSafe    *keepSafe
	upMutex     sync.RWMutex
	up          bool // true until the conn goes down

	wg sync.WaitGroup

	droppedMetricsCounter *prometheus.CounterVec
	bm                    *metrics.BufferMetrics
	logger                *zap.Logger
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
		buffered: bufio.NewWriterSize(conn, ioBufSize),
		// when we write to shutdown, HandleData() may not be running anymore to read from the chan
		// but, it may also be in an error scenario in which case it calls c.close() writing a second time to shutdown,
		// after checkEOF has called c.close(). so we need enough room
		shutdown:    make(chan bool, 2),
		In:          make(chan encoding.Datapoint, connBufSize),
		key:         key,
		up:          true,
		pickle:      pickle,
		flush:       make(chan bool),
		flushErr:    make(chan error),
		periodFlush: periodFlush,
		keepSafe:    NewKeepSafe(keepsafe_initial_cap, keepsafe_keep_duration),
		bm: metrics.NewBufferMetrics("destination_conn", key, prometheus.Labels{
			"address": addr,
		}, []float64{250, 500, 750, 1000, 1250, 1500}),
		logger: zap.L().With(zap.String("connectionKey", key)), // prefill key
	}

	connObj.bm.Size.Set(float64(connBufSize))

	connObj.wg.Add(2)
	go connObj.checkEOF()
	go connObj.HandleData()
	return connObj, nil
}

func (c *Conn) isAlive() bool {
	c.upMutex.RLock()
	up := c.up
	c.upMutex.RUnlock()
	c.logger.Debug("conn isAlive", zap.Bool("up", up))
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
			c.logger.Info("conn %s .conn.Read returned EOF -> conn is closed. closing conn explicitly")
			c.close()
			return
		}
		// just in case i misunderstand something or the remote behaves badly
		if num != 0 {
			c.logger.Debug("conn %s .conn.Read data? did not expect that", zap.ByteString("data", b[:num]))
			continue
		}
		if err != io.EOF {
			c.logger.Error("conn %s checkEOF .conn.Read returned err != EOF, which is unexpected.  closing conn", zap.Error(err))
			c.close()
			return
		}
	}
}

// all these messages should potentially be resubmitted, because we're not confident about their delivery
// note: getting this data means resetting it! so handle it wisely.
// we also read out the In channel until it blocks.  Don't send any more input after calling this.
func (c *Conn) getRedo() []encoding.Datapoint {
	// drain In queue in case we still had some data buffered.
	// normally this channel should already have been closed by the time we call this, but this is hard/complicated to enforce
	// so instead let's leverage a select. as soon as it blocks (due to chan close or no more input but not closed yet) we know we're
	// done reading and move on. it's easy to prove in the implementer that we don't send any more data to In after calling this
	defer c.clearRedo()
	for {
		select {
		case dp := <-c.In:
			c.bm.BufferedMetrics.Dec()
			c.keepSafe.Add(dp)
		default:
			return c.keepSafe.GetAll()
		}
	}
}

func (c *Conn) alive(alive bool) {
	c.upMutex.Lock()
	c.up = alive
	c.logger.Debug("conn alive", zap.Bool("up", c.up))
	c.upMutex.Unlock()
}

func (c *Conn) HandleData() {
	defer c.wg.Done()
	periodFlush := c.periodFlush
	tickerFlush := time.NewTicker(periodFlush)
	defer tickerFlush.Stop()
	flushSize := int64(0)
	start := time.Now()

	writeBuf := make([]byte, 0, 300)

	for {
		iterStart := time.Now()
		var active time.Time
		var action string
		select { // handle incoming data or flush/shutdown commands
		// note that Writer.Write() can potentially cause a flush and hence block
		// choose the size of In based on how long these loop iterations take
		case dp := <-c.In:
			// seems to take about 30 micros when writing log to disk, 10 micros otherwise (100k messages/second)
			active = time.Now()
			c.bm.BufferedMetrics.Dec()
			action = "write"
			c.logger.Debug("conn HandleData: writing datapoint", zap.Stringer("datapoint", dp))
			c.keepSafe.Add(dp)

			writeBuf = append(writeBuf[:0], dp.Name...)
			writeBuf = append(writeBuf, ' ')
			writeBuf = strconv.AppendFloat(writeBuf, dp.Value, 'f', -1, 64)
			writeBuf = append(writeBuf, ' ')
			writeBuf = strconv.AppendUint(writeBuf, dp.Timestamp, 10)

			n, err := c.Write(writeBuf)
			if err != nil {
				c.logger.Warn("conn write error. closing", zap.Error(err))
				c.close() // this can take a while but that's ok. this conn won't be used anymore
				return
			}
			c.bm.WriteDuration.Observe(float64(time.Since(active)))
			flushSize += int64(n)
		case <-tickerFlush.C:
			active = time.Now()
			action = "auto-flush"
			c.logger.Debug("conn HandleData: c.buffered auto-flushing...")
			err := c.buffered.Flush()
			if err != nil {
				c.logger.Warn("conn HandleData c.buffered auto-flush done but with error. closing", zap.Error(err))
				errCounter.WithLabelValues(c.key, "flush").Inc()
				c.close()
				return
			}
			c.logger.Debug("conn HandleData c.buffered auto-flush done without error")
			c.bm.ObserveFlush(time.Since(active), flushSize, metrics.FlushTypeTicker)
			flushSize = 0
		case <-c.flush:
			active = time.Now()
			action = "manual-flush"
			c.logger.Debug("conn HandleData: c.buffered manual flushing...")
			err := c.buffered.Flush()
			c.flushErr <- err
			if err != nil {
				c.logger.Warn("conn HandleData c.buffered manual flush done but witth error. closing", zap.Error(err))
				errCounter.WithLabelValues(c.key, "flush").Inc()
				c.close()
				return
			}
			c.logger.Info("conn HandleData c.buffered manual flush done without error")
			c.bm.ObserveFlush(time.Since(active), flushSize, metrics.FlushTypeManual)
			flushSize = 0
		case <-c.shutdown:
			c.logger.Debug("conn HandleData: shutdown received. returning.")
			return
		}
		c.logger.Debug("conn HandleData (use this to tune your In buffering)",
			zap.String("action", action),
			zap.Duration("since_iterStart", time.Since(iterStart)),
			zap.Duration("since_start", time.Since(start)))
	}
}

// returns a network/write error, so that it can be retried later
// deals with pickle errors internally because retrying wouldn't help anyway
func (c *Conn) Write(buf []byte) (int, error) {
	if c.pickle {
		dp, err := ParseDataPoint(buf)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			droppedMetricsCounter.WithLabelValues(c.key, "bad_pickle").Inc()
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
		errCounter.WithLabelValues(c.key, "write").Inc()
	}
	if err == nil && size != n {
		errCounter.WithLabelValues(c.key, "truncated").Inc()
		err = fmt.Errorf("truncated write: %s", buf)
	}
	return written, err
}

func (c *Conn) Flush() error {
	c.logger.Debug("conn going to flush my buffer")
	c.flush <- true
	c.logger.Debug("conn waiting for flush, getting error.")
	return <-c.flushErr
}

func (c *Conn) close() {
	c.alive(false)
	c.logger.Debug("conn close() called. sending shutdown")
	c.shutdown <- true
	c.logger.Debug("conn c.conn.Close()")
	c.conn.Close()
	c.logger.Debug("conn c.conn.Close() complete")
}

// Close closes the connection and releases all resources, with the exception of the
// keepSafe buffer. because the caller of conn needs a chance to collect that data
func (c *Conn) Close() {
	c.close()
	c.logger.Debug("conn Close() waiting")
	c.wg.Wait()
	c.logger.Debug("conn Close() complete")
}

// clearRedo releases the keepSafe resources
func (c *Conn) clearRedo() {
	c.logger.Debug("conn c.keepSafe.Stop()")
	c.keepSafe.Stop()
}
