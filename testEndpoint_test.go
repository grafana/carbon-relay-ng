package main

import (
	"bufio"
	"github.com/bmizerany/assert"
	"net"
	"sync"
	"testing"
	"time"
)

type TestEndpoint struct {
	t                       *testing.T
	ln                      net.Listener
	seen                    chan []byte
	seenBufs                [][]byte
	shutdown                chan bool
	shutdownHandle          chan bool // to shut down 1 handler. if you start more handlers they'll keep running
	WhatHaveISeen           chan bool
	IHaveSeen               chan [][]byte
	addr                    string
	numSeen                 chan int
	NumSeenReq              chan int
	NumSeenResp             chan int
	NumAcceptsReq           chan int
	NumAcceptsResp          chan int
	WaitUntilNumSeenReq     chan int
	WaitUntilNumSeenResp    chan int
	WaitUntilNumAcceptsReq  chan int
	WaitUntilNumAcceptsResp chan int
	accepts                 chan int
}

func NewTestEndpoint(t *testing.T, addr string) *TestEndpoint {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}
	log.Notice("tE %s is now listening\n", addr)
	// shutdown chan size 1 so that Close() doesn't have to wait on the write
	// because the loops will typically be stuck in Accept ad Readline
	tE := &TestEndpoint{
		t:                       t,
		addr:                    addr,
		ln:                      ln,
		seen:                    make(chan []byte),
		seenBufs:                make([][]byte, 0),
		shutdown:                make(chan bool, 1),
		shutdownHandle:          make(chan bool, 1),
		WhatHaveISeen:           make(chan bool),
		IHaveSeen:               make(chan [][]byte),
		numSeen:                 make(chan int),
		NumSeenReq:              make(chan int),
		NumSeenResp:             make(chan int),
		NumAcceptsReq:           make(chan int),
		NumAcceptsResp:          make(chan int),
		WaitUntilNumSeenReq:     make(chan int),
		WaitUntilNumSeenResp:    make(chan int, 1), // don't block on other end reading.
		WaitUntilNumAcceptsReq:  make(chan int),
		WaitUntilNumAcceptsResp: make(chan int, 1), // don't block on other end reading
		accepts:                 make(chan int),
	}
	go func() {
		waitUntilNumAccepts := -1
		numAccepts := 0
		for {
			select {
			case diff := <-tE.accepts:
				numAccepts += diff
			case waitUntilNumAccepts = <-tE.WaitUntilNumAcceptsReq:
			case <-tE.NumAcceptsReq:
				tE.NumAcceptsResp <- numAccepts
			}
			if numAccepts == waitUntilNumAccepts {
				tE.WaitUntilNumAcceptsResp <- numAccepts
			}
		}
	}()
	go func() {
		for {
			select {
			case <-tE.shutdown:
				return
			default:
			}
			log.Debug("tE %s waiting for accept\n", tE.addr)
			conn, err := ln.Accept()
			// when closing, this can happen: accept tcp [::]:2005: use of closed network connection
			if err != nil {
				log.Debug("tE %s accept error: '%s' -> stopping tE\n", tE.addr, err)
				return
			}
			tE.accepts <- 1
			log.Notice("tE %s accepted new conn\n", tE.addr)
			go tE.handle(conn)
			defer func() { log.Debug("tE %s closing conn.\n", tE.addr); conn.Close() }()
		}
	}()
	go func() {
		waitUntilNumSeen := -1
		for {
			select {
			// always try this first to make sure it gets set when it can
			case waitUntilNumSeen = <-tE.WaitUntilNumSeenReq:
				if len(tE.seenBufs) == waitUntilNumSeen {
					tE.WaitUntilNumSeenResp <- len(tE.seenBufs)
				}
			default:
			}
			select {
			case buf := <-tE.seen:
				tE.seenBufs = append(tE.seenBufs, buf)
				if len(tE.seenBufs) == waitUntilNumSeen {
					tE.WaitUntilNumSeenResp <- len(tE.seenBufs)
				}
			case <-tE.WhatHaveISeen:
				var c [][]byte
				c = append(c, tE.seenBufs...)
				tE.IHaveSeen <- c
			case <-tE.NumSeenReq:
				tE.NumSeenResp <- len(tE.seenBufs)
			case waitUntilNumSeen = <-tE.WaitUntilNumSeenReq:
				if len(tE.seenBufs) == waitUntilNumSeen {
					tE.WaitUntilNumSeenResp <- len(tE.seenBufs)
				}
			}
		}
	}()
	return tE
}

func (tE *TestEndpoint) WaitNumSeenOrFatal(numSeen int, timeout time.Duration, wg *sync.WaitGroup) {
	defer func() {
		if wg != nil {
			wg.Done()
		}
	}()
	tE.WaitUntilNumSeenReq <- numSeen
	select {
	case <-tE.WaitUntilNumSeenResp:
	case <-time.After(timeout):
		tE.NumSeenReq <- 0
		currentNum := <-tE.NumSeenResp
		tE.t.Fatalf("tE %s timed out after %s waiting for %d msg (only saw %d)", tE.addr, timeout, numSeen, currentNum)
	}
}

func (tE *TestEndpoint) WaitNumAcceptsOrFatal(numAccepts int, timeout time.Duration, wg *sync.WaitGroup) {
	defer func() {
		if wg != nil {
			wg.Done()
		}
	}()

	tE.WaitUntilNumAcceptsReq <- numAccepts
	select {
	case <-tE.WaitUntilNumAcceptsResp:
	case <-time.After(timeout):
		tE.NumAcceptsReq <- 0
		currentNum := <-tE.NumAcceptsResp
		tE.t.Fatalf("tE %s timed out after %s waiting for %d accepts (only saw %d)", tE.addr, timeout, numAccepts, currentNum)
	}
}

// don't call this concurrently
func (tE *TestEndpoint) WaitUntilNumAccepts(numAccept int) {
	tE.WaitUntilNumAcceptsReq <- numAccept
	<-tE.WaitUntilNumAcceptsResp
	return
}

// don't call this concurrently
func (tE *TestEndpoint) WaitUntilNumMsg(numMsg int) {
	tE.WaitUntilNumSeenReq <- numMsg
	<-tE.WaitUntilNumSeenResp
	return
}

func (tE *TestEndpoint) SeenThisOrFatal(ref [][]byte) {
	tE.WhatHaveISeen <- true
	seen := <-tE.IHaveSeen
	assert.Equal(tE.t, len(seen), len(ref))
	// human friendly:
	for i, m := range seen {
		if string(m) != string(ref[i]) {
			tE.t.Errorf("tE %s error at pos %d: expected '%s', received: '%s'", tE.addr, i, ref[i], m)
		}
	}
	// equivalent, but for deeper debugging
	assert.Equal(tE.t, seen, ref)
}

func (tE *TestEndpoint) handle(c net.Conn) {
	defer func() {
		log.Debug("tE %s closing conn %s\n", tE.addr, c)
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
			log.Warning("tE %s read error: %s. closing handler\n", tE.addr, err)
			return
		}
		log.Info("tE %s %s read\n", tE.addr, string(buf))
		buf_copy := make([]byte, len(buf), len(buf))
		copy(buf_copy, buf)
		tE.seen <- buf_copy
	}
}

func (tE *TestEndpoint) Close() {
	log.Debug("tE %s shutting down accepter (after accept breaks)", tE.addr)
	tE.shutdown <- true
	log.Debug("tE %s shutting down handler (after readLine breaks)", tE.addr)
	tE.shutdownHandle <- true
	log.Debug("tE %s shutting down listener", tE.addr)
	tE.ln.Close()
	log.Debug("tE %s listener down", tE.addr)
}
