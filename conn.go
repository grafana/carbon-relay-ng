package main

import (
	"bufio"
	"net"
	"time"
)

type Conn struct {
	conn     *net.TCPConn
	buffered *bufio.Writer
	shutdown chan bool
}

func NewConn(addr string) (*Conn, error) {
	raddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, err
	}
	laddr, _ := net.ResolveTCPAddr("tcp", "0.0.0.0")
	conn, err := net.DialTCP("tcp", laddr, raddr)
	if err != nil {
		return nil, err
	}
	connObj := &Conn{conn, bufio.NewWriter(conn), make(chan bool)}

	periodFlush := time.Duration(1) * time.Second
	tickerFlush := time.NewTicker(periodFlush)

	go func() {
		for {
			select {
			case <-tickerFlush.C:
				connObj.buffered.Flush()
			case <-connObj.shutdown:
				return
			}
		}
	}()
	return connObj, nil
}

func (c *Conn) Write(buf []byte) (int, error) {
	return c.buffered.Write(buf)
}

func (c *Conn) Close() error {
	c.shutdown <- true
	return c.conn.Close()
}
