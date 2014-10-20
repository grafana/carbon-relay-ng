package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"time"
)

type Conn struct {
	conn     *net.TCPConn
	buffered *bufio.Writer
	shutdown chan bool
	In       chan []byte
	dest     *Destination // which dest do we correspond to
	up       bool
	checkUp  chan bool
	updateUp chan bool
}

func NewConn(addr string, dest *Destination) (*Conn, error) {
	raddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, err
	}
	laddr, _ := net.ResolveTCPAddr("tcp", "0.0.0.0")
	conn, err := net.DialTCP("tcp", laddr, raddr)
	if err != nil {
		return nil, err
	}
	connObj := &Conn{conn, bufio.NewWriter(conn), make(chan bool), make(chan []byte), dest, true, make(chan bool), make(chan bool)}

	go connObj.HandleData()
	go connObj.HandleStatus()
	return connObj, nil
}

func (c *Conn) isAlive() bool {
	return <-c.checkUp
}

func (c *Conn) HandleStatus() {
	for {
		select {
		case c.up = <-c.updateUp:
		case c.checkUp <- c.up:
		}
	}
}

func (c *Conn) HandleData() {
	periodFlush := time.Duration(1) * time.Second
	tickerFlush := time.NewTicker(periodFlush)

	for {
		select {
		case buf := <-c.In:
			n, err := c.Write(buf)
			errBecauseTruncated := false
			if err == nil && len(buf) != n {
				errBecauseTruncated = true
				go func() {
					c.dest.statsd.Increment("dest=" + addrToPath(c.dest.Addr) + ".target_type=count.unit=Err.type=truncated")
				}()
				err = errors.New(fmt.Sprintf("truncated write: %s", string(buf)))
			}
			if err != nil {
				if !errBecauseTruncated {
					go func() {
						c.dest.statsd.Increment("dest=" + addrToPath(c.dest.Addr) + ".target_type=count.unit=Err.type=write")
					}()
				}
				log.Println(c.dest.Addr + " " + err.Error())
				c.Close()
				c.updateUp <- false
				// TODO: should add function that returns unflushed data, for dest to query so it can spool it
				return
			} else {
				go func() {
					c.dest.statsd.Increment("dest=" + addrToPath(c.dest.Addr) + ".target_type=count.unit=Metric.direction=out")
				}()
			}
		case <-tickerFlush.C:
			c.buffered.Flush()
		case <-c.shutdown:
			return
		}
	}
}

func (c *Conn) Write(buf []byte) (int, error) {
	return c.buffered.Write(buf)
}

func (c *Conn) Close() error {
	c.shutdown <- true
	return c.conn.Close()
}
