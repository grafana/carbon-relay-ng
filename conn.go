package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

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
		buffered:    bufio.NewWriter(conn),
		shutdown:    make(chan bool, 1), // when we write here, HandleData() may not be running anymore to read from the chan
		In:          make(chan []byte),
		dest:        dest,
		up:          true,
		checkUp:     make(chan bool),
		updateUp:    make(chan bool),
		flush:       make(chan bool),
		flushErr:    make(chan error),
		periodFlush: periodFlush,
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
			log.Printf("%s conn c.conn.Read returned EOF -> conn is closed", c.dest.Addr)
			c.Close()
			return
		}
		// just in case i misunderstand something or the remote behaves badly
		if num != 0 {
			log.Printf("ERROR %s conn c.conn.Read data? did not expect that.  data: %s\n", c.dest.Addr, b[:num])
		}
		if err != io.EOF {
			log.Printf("ERROR %s conn c.conn.Read returned but without err=EOF.  did not expect that.  error: %s\n", c.dest.Addr, err)
		}
	}
}

func (c *Conn) HandleStatus() {
	for {
		select {
		case c.up = <-c.updateUp:
			log.Printf("%s conn marking alive as %v\n", c.dest.Addr, c.up)
			//log.Println("conn.up is now", c.up)
		case c.checkUp <- c.up:
			log.Println("query for conn.up, responded with", c.up)
		}
	}
}

func (c *Conn) HandleData() {
	periodFlush := c.periodFlush
	tickerFlush := time.NewTicker(periodFlush)

	for {
		start := time.Now()
		select {
		case buf := <-c.In:
			log.Printf("%s conn HandleData writing %s\n", c.dest.Addr, string(buf))
			buf = append(buf, '\n')
			n, err := c.Write(buf)
			errBecauseTruncated := false
			if err == nil && len(buf) != n {
				errBecauseTruncated = true
				c.dest.numErrTruncated.Add(1)
				err = errors.New(fmt.Sprintf("truncated write: %s", string(buf)))
			}
			if err != nil {
				if !errBecauseTruncated {
					c.dest.numErrWrite.Add(1)
				}
				log.Printf("%s conn write error: %s\n", c.dest.Addr, err)
				log.Printf("%s conn setting up=false\n", c.dest.Addr)
				c.updateUp <- false // assure In won't receive more data because every loop that writes to In reads this out
				log.Printf("%s conn Closing\n", c.dest.Addr)
				go c.Close() // this can take a while but that's ok. this conn won't be used anymore
				// TODO: should add function that returns unflushed data, for dest to query so it can spool it
				return
			} else {
				c.dest.numOut.Add(1)
			}
		case <-tickerFlush.C:
			err := c.buffered.Flush()
			log.Printf("%s conn HandleData c.buffered auto-flush done. error maybe: %s\n", c.dest.Addr, err)
		case <-c.flush:
			err := c.buffered.Flush()
			log.Printf("%s conn HandleData c.buffered manual flush done. error maybe: %s\n", c.dest.Addr, err)
			c.flushErr <- err
		case <-c.shutdown:
			return
		}
		log.Printf("%s conn HandleData iteration took %s (this needs to be super responsive!)\n", c.dest.Addr, time.Now().Sub(start))
	}
}

func (c *Conn) Write(buf []byte) (int, error) {
	return c.buffered.Write(buf)
}

func (c *Conn) Flush() error {
	log.Printf("%s conn going to flush my buffer\n", c.dest.Addr)
	c.flush <- true
	log.Printf("%s conn waiting for flush, getting error.\n", c.dest.Addr)
	return <-c.flushErr
}

func (c *Conn) Close() error {
	c.updateUp <- false // redundant in case HandleData() called us, but not if the dest called us
	log.Printf("%s conn Close() called. sending shutdown\n", c.dest.Addr)
	c.shutdown <- true
	log.Printf("%s conn c.conn.Close()\n", c.dest.Addr)
	a := c.conn.Close()
	log.Printf("%s conn c.conn is closed\n", c.dest.Addr)
	return a
}
