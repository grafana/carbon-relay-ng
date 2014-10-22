package main

import (
	"bufio"
	"log"
	"net"
	"testing"
)

type TestEndpoint struct {
	t              *testing.T
	ln             net.Listener
	seen           chan []byte
	seenBufs       [][]byte
	shutdown       chan bool
	shutdownHandle chan bool // to shut down 1 handler. if you start more handlers they'll keep running
	WhatHaveISeen  chan bool
	IHaveSeen      chan [][]byte
	addr           string
}

func NewTestEndpoint(t *testing.T, addr string) *TestEndpoint {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}
	log.Printf("tE %s is now listening\n", addr)
	// shutdown chan size 1 so that Close() doesn't have to wait on the write
	// because the loops will typically be stuck in Accept ad Readline
	tE := &TestEndpoint{
		addr:           addr,
		t:              t,
		ln:             ln,
		seen:           make(chan []byte),
		seenBufs:       make([][]byte, 0),
		shutdown:       make(chan bool, 1),
		shutdownHandle: make(chan bool, 1),
		WhatHaveISeen:  make(chan bool),
		IHaveSeen:      make(chan [][]byte),
	}
	go func() {
		for {
			select {
			case <-tE.shutdown:
				return
			default:
			}
			log.Printf("tE %s waiting for accept\n", tE.addr)
			conn, err := ln.Accept()
			// when closing, this can happen: accept tcp [::]:2005: use of closed network connection
			if err != nil {
				log.Printf("tE %s accept error: '%s' -> stopping tE\n", tE.addr, err)
				return
			}
			log.Printf("tE %s accepted new conn\n", tE.addr)
			go tE.handle(conn)
			defer func() { log.Printf("tE %s closing conn.\n", tE.addr); conn.Close() }()
		}
	}()
	go func() {
		for {
			select {
			case buf := <-tE.seen:
				tE.seenBufs = append(tE.seenBufs, buf)
			case <-tE.WhatHaveISeen:
				var c [][]byte
				c = append(c, tE.seenBufs...)
				tE.IHaveSeen <- c
			}
		}
	}()
	return tE
}

func (tE *TestEndpoint) handle(c net.Conn) {
	defer func() {
		log.Printf("tE %s closing conn %s\n", tE.addr, c)
		c.Close()
	}()
	r := bufio.NewReaderSize(c, 4096)
	for {
		select {
		case <-tE.shutdownHandle:
			return
		default:
		}
		buf, _, err := r.ReadLine()
		if err != nil {
			log.Printf("tE %s read error: %s. closing handler\n", tE.addr, err)
			return
		}
		log.Printf("tE %s read %s\n", tE.addr, string(buf))
		buf_copy := make([]byte, len(buf), len(buf))
		copy(buf_copy, buf)
		tE.seen <- buf_copy
	}
}

func (tE *TestEndpoint) Close() {
	log.Printf("tE %s shutting down accepter (after accept breaks)", tE.addr)
	tE.shutdown <- true
	log.Printf("tE %s shutting down handler (after readLine breaks)", tE.addr)
	tE.shutdownHandle <- true
	log.Printf("tE %s shutting down listener", tE.addr)
	tE.ln.Close()
	log.Printf("tE %s listener down", tE.addr)
}
