package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

var bufio_buffer_size = 2000000 // in bytes. 4096 is go default

// to make sure writes to In are fast until we really can't keep up
var conn_in_buffer = 30000 // in metrics. (each metric line is typically about 70 bytes)

var keepsafe_initial_cap = 100000 // not very important

// this interval should be long enough to capture all failure modes
// (endpoint down, delayed timeout, etc), so it should be at least as long as the flush interval
var keepsafe_keep_duration = time.Duration(10 * time.Second)

type Conn struct {
	conn        *net.TCPConn
	buffered    *bufio.Writer
	shutdown    chan bool
	In          chan []byte
	dest        *Destination // which dest do we correspond to
	up          bool
	checkUp     chan bool
	updateUp    chan bool
	flush       chan bool
	flushErr    chan error
	periodFlush time.Duration
	unFlushed   []byte
	keepSafe    *keepSafe
}

func NewConn(addr string, dest *Destination, periodFlush time.Duration) (*Conn, error) {
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
		conn:        conn,
		buffered:    bufio.NewWriterSize(conn, bufio_buffer_size),
		shutdown:    make(chan bool, 1), // when we write here, HandleData() may not be running anymore to read from the chan
		In:          make(chan []byte, conn_in_buffer),
		dest:        dest,
		up:          true,
		checkUp:     make(chan bool),
		updateUp:    make(chan bool),
		flush:       make(chan bool),
		flushErr:    make(chan error),
		periodFlush: periodFlush,
		keepSafe:    NewKeepSafe(keepsafe_initial_cap, keepsafe_keep_duration),
	}

	go connObj.checkEOF()

	go connObj.HandleData()
	go connObj.HandleStatus()
	return connObj, nil
}

func (c *Conn) isAlive() bool {
	return <-c.checkUp
}

// normally the remote end should never write anything back
// but we know when we get EOF that the other end closed the conn
// if not for this, we can happily write and flush without getting errors (in Go) but getting RST tcp packets back (!)
// props to Tv` for this trick.
func (c *Conn) checkEOF() {
	b := make([]byte, 1024)
	for {
		num, err := c.conn.Read(b)
		if err == io.EOF {
			log.Notice("conn %s .conn.Read returned EOF -> conn is closed. closing conn explicitly", c.dest.Addr)
			c.Close()
			return
		}
		// just in case i misunderstand something or the remote behaves badly
		if num != 0 {
			log.Error("conn %s .conn.Read data? did not expect that.  data: %s\n", c.dest.Addr, b[:num])
		}
		if err != io.EOF {
			log.Error("conn %s checkEOF .conn.Read returned err != EOF, which is unexpected.  closing conn. error: %s\n", c.dest.Addr, err)
			c.Close()
			return
		}
	}
}

// all these messages should potentially be resubmitted, because we're not confident about their delivery
// note: getting this data means resetting it! so handle it wisely.
func (c *Conn) getRedo() [][]byte {
	return c.keepSafe.GetAll()
}

func (c *Conn) HandleStatus() {
	for {
		select {
		// note: when we mark as down here, it is expected that conn doesn't absorb any more data,
		// so that you can call getRedo() and get the full picture
		// this is actually not true yet.
		case c.up = <-c.updateUp:
			log.Debug("conn %s .up set to %v\n", c.dest.Addr, c.up)
		case c.checkUp <- c.up:
			log.Debug("conn %s .up query responded with %t", c.dest.Addr, c.up)
		}
	}
}

func (c *Conn) HandleData() {
	periodFlush := c.periodFlush
	tickerFlush := time.NewTicker(periodFlush)
	newLine := []byte{'\n'}
	var now time.Time
	var durationActive time.Duration
	flushSize := int64(0)

	for {
		start := time.Now()
		var active time.Time
		var action string
		select {
		// note that bufio.Writer.Write() can potentially cause a flush and hence block
		// choose the size of In based on how long these loop iterations take
		case buf := <-c.In:
			// seems to take about 30 micros when writing log to disk, 10 micros otherwise (100k messages/second)
			active = time.Now()
			c.dest.numBuffered.Dec(1)
			action = "write"
			log.Info("conn %s HandleData: writing %s\n", c.dest.Addr, string(buf))
			c.keepSafe.Add(buf)
			size := len(buf)
			n, err := c.Write(buf)
			if err == nil && size == n {
				n, err = c.Write(newLine)
				size = 1
			}
			errBecauseTruncated := false
			if err == nil && size != n {
				errBecauseTruncated = true
				c.dest.numErrTruncated.Inc(1)
				err = errors.New(fmt.Sprintf("truncated write: %s", string(buf)))
			}
			if err != nil {
				if !errBecauseTruncated {
					c.dest.numErrWrite.Inc(1)
				}
				log.Warning("conn %s write error: %s\n", c.dest.Addr, err)
				log.Debug("conn %s setting up=false\n", c.dest.Addr)
				c.updateUp <- false // assure In won't receive more data because every loop that writes to In reads this out
				log.Debug("conn %s Closing\n", c.dest.Addr)
				go c.Close() // this can take a while but that's ok. this conn won't be used anymore
				return
			}
			c.dest.numOut.Inc(1)
			flushSize += int64(len(buf) + 1)
			now = time.Now()
			durationActive = now.Sub(active)
			c.dest.durationWrite.Update(durationActive)
		case <-tickerFlush.C:
			active = time.Now()
			action = "auto-flush"
			log.Debug("conn %s HandleData: c.buffered auto-flushing...\n", c.dest.Addr)
			err := c.buffered.Flush()
			if err != nil {
				log.Warning("conn %s HandleData c.buffered auto-flush done but with error: %s, closing\n", c.dest.Addr, err)
				// TODO instrument
				c.updateUp <- false
				go c.Close()
				return
			}
			log.Debug("conn %s HandleData c.buffered auto-flush done without error\n", c.dest.Addr)
			now = time.Now()
			durationActive = now.Sub(active)
			c.dest.durationAutoFlush.Update(durationActive)
			c.dest.autoFlushSize.Update(flushSize)
			flushSize = 0
		case <-c.flush:
			active = time.Now()
			action = "manual-flush"
			log.Debug("conn %s HandleData: c.buffered manual flushing...\n", c.dest.Addr)
			err := c.buffered.Flush()
			c.flushErr <- err
			if err != nil {
				log.Warning("conn %s HandleData c.buffered manual flush done but witth error: %s, closing\n", c.dest.Addr, err)
				// TODO instrument
				c.updateUp <- false
				go c.Close()
				return
			}
			log.Notice("conn %s HandleData c.buffered manual flush done without error\n", c.dest.Addr)
			now = time.Now()
			durationActive = now.Sub(active)
			c.dest.durationManuFlush.Update(durationActive)
			c.dest.manuFlushSize.Update(flushSize)
			flushSize = 0
		case <-c.shutdown:
			active = time.Now()
			log.Debug("conn %s HandleData: shutdown received. returning.\n", c.dest.Addr)
			return
		}
		log.Debug("conn %s HandleData %s %s (total iter %s) (use this to tune your In buffering)\n", c.dest.Addr, action, durationActive, now.Sub(start))
	}
}

func (c *Conn) Write(buf []byte) (int, error) {
	return c.buffered.Write(buf)
}

func (c *Conn) Flush() error {
	log.Debug("conn %s going to flush my buffer\n", c.dest.Addr)
	c.flush <- true
	log.Debug("conn %s waiting for flush, getting error.\n", c.dest.Addr)
	return <-c.flushErr
}

func (c *Conn) Close() error {
	c.updateUp <- false // redundant in case HandleData() called us, but not if the dest called us
	log.Debug("conn %s Close() called. sending shutdown\n", c.dest.Addr)
	c.shutdown <- true
	log.Debug("conn %s c.conn.Close()\n", c.dest.Addr)
	a := c.conn.Close()
	log.Debug("conn %s c.conn is closed\n", c.dest.Addr)
	return a
}
