package main

import (
	"bufio"
	"io"
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
			log.Println("tE waiting for accept")
			conn, err := ln.Accept()
			// when closing, this can happen: accept tcp [::]:2005: use of closed network connection
			if err != nil {
				log.Println("tE accept error:", err, "stopping tE")
				//t.Fatal(err)
				return
			}
			log.Println("tE accepted new conn")
			go tE.handle(conn)
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
	defer c.Close()
	r := bufio.NewReaderSize(c, 4096)
	for {
		select {
		case <-tE.shutdownHandle:
			return
		default:
		}
		buf, _, err := r.ReadLine()
		log.Println(tE.addr, "read", string(buf))
		if err == io.EOF {
			break
		}
		buf_copy := make([]byte, len(buf), len(buf))
		copy(buf_copy, buf)
		tE.seen <- buf_copy
	}
}

func (tE *TestEndpoint) Close() {
	//log.Println("AAAAcalling shut")
	tE.shutdown <- true
	tE.shutdownHandle <- true
	//log.Println("AAAAcalled shut")
	tE.ln.Close()
	//log.Println("AAACLOSE")
}
