package main

import (
	"bufio"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Dieterbe/topic"
	"github.com/graphite-ng/carbon-relay-ng/encoding"
)

// TODO see if we can get simplify this type. do we need to track all bufs? can we do it in a more performant way?
type TestEndpoint struct {
	t              *testing.T
	ln             net.Listener
	seen           chan encoding.Datapoint
	seenBufs       []encoding.Datapoint
	shutdown       chan bool
	shutdownHandle chan bool // to shut down 1 handler. if you start more handlers they'll keep running
	WhatHaveISeen  chan bool
	IHaveSeen      chan []encoding.Datapoint
	addr           string
	accepts        *topic.Topic
	numSeen        *topic.Topic
}

func NewTestEndpoint(t *testing.T, addr string) *TestEndpoint {
	// shutdown chan size 1 so that Close() doesn't have to wait on the write
	// because the loops will typically be stuck in Accept and Readline
	return &TestEndpoint{
		t:              t,
		addr:           addr,
		seen:           make(chan encoding.Datapoint),
		seenBufs:       make([]encoding.Datapoint, 0),
		shutdown:       make(chan bool, 1),
		shutdownHandle: make(chan bool, 1),
		WhatHaveISeen:  make(chan bool),
		IHaveSeen:      make(chan []encoding.Datapoint),
		accepts:        topic.New(),
		numSeen:        topic.New(),
	}
}

func (tE *TestEndpoint) Start() {
	ln, err := net.Listen("tcp", tE.addr)
	if err != nil {
		panic(err)
	}
	tE.t.Logf("tE %s is now listening", tE.addr)
	tE.ln = ln
	go func() {
		numAccepts := 0
		for {
			select {
			case <-tE.shutdown:
				return
			default:
			}
			tE.t.Logf("tE %s waiting for accept", tE.addr)
			conn, err := ln.Accept()
			// when closing, this can happen: accept tcp [::]:2005: use of closed network connection
			if err != nil {
				tE.t.Logf("tE %s accept error: '%s' -> stopping tE", tE.addr, err)
				return
			}
			numAccepts += 1
			tE.accepts.Broadcast <- numAccepts
			tE.t.Logf("tE %s accepted new conn", tE.addr)
			go tE.handle(conn)
			defer func() { tE.t.Logf("tE %s closing conn.", tE.addr); conn.Close() }()
		}
	}()
	go func() {
		numSeen := 0
		for {
			select {
			case buf := <-tE.seen:
				tE.seenBufs = append(tE.seenBufs, buf)
				numSeen += 1
				tE.numSeen.Broadcast <- numSeen
			case <-tE.WhatHaveISeen:
				var c []encoding.Datapoint
				c = append(c, tE.seenBufs...)
				tE.IHaveSeen <- c
			}
		}
	}()
}

func (tE *TestEndpoint) String() string {
	return fmt.Sprintf("testEndpoint %s", tE.addr)
}

// note: per conditionwatcher, only call Allow() or Wait(), once.
// (because the result is in channel met and can only be consumed once)
// feel free to make as many conditionWatcher's as you want.
type conditionWatcher struct {
	t             *testing.T
	key           string
	desiredStatus string

	sync.Mutex
	lastStatus string

	met chan bool
}

func newConditionWatcher(tE *TestEndpoint, key, desiredStatus string) *conditionWatcher {
	c := conditionWatcher{
		t:             tE.t,
		key:           key,
		desiredStatus: desiredStatus,
		met:           make(chan bool, 1),
	}
	return &c
}

func (c *conditionWatcher) AllowBG(timeout time.Duration, wg *sync.WaitGroup) {
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		c.Allow(timeout)
		wg.Done()
	}(wg)
}

func (c *conditionWatcher) PreferBG(timeout time.Duration, wg *sync.WaitGroup) {
	wg.Add(1)
	go func(wg *sync.WaitGroup) {
		c.Prefer(timeout)
		wg.Done()
	}(wg)
}

func (c *conditionWatcher) Prefer(timeout time.Duration) {
	timeoutChan := time.After(timeout)
	select {
	case <-c.met:
		return
	case <-timeoutChan:
		return
	}
}

func (c *conditionWatcher) Allow(timeout time.Duration) {
	timeoutChan := time.After(timeout)
	select {
	case <-c.met:
		return
	case <-timeoutChan:
		c.Lock()
		// for some reason the c.t.Fatalf often gets somehow stuck. but the fmt.Printf works at least
		fmt.Printf("FATAL: %s timed out after %s of waiting for %s (%s)", c.key, timeout, c.desiredStatus, c.lastStatus)
		c.t.Fatalf("%s timed out after %s of waiting for %s (%s)", c.key, timeout, c.desiredStatus, c.lastStatus)
		c.Unlock()
	}
}

// no timeout
func (c *conditionWatcher) Wait() {
	<-c.met
}

func (tE *TestEndpoint) conditionNumAccepts(desired int) *conditionWatcher {
	c := newConditionWatcher(tE, fmt.Sprintf("tE %s", tE.addr), fmt.Sprintf("%d accepts", desired))
	// we want to make sure we can consume straight after registering
	// (otherwise topic can kick us out)
	// but also this function can only return when we're ready to catch appropriate
	// events
	ready := make(chan bool)

	go func(ready chan bool) {
		consumer := make(chan interface{})
		tE.accepts.Register(consumer)
		c.Lock()
		c.lastStatus = fmt.Sprintf("saw 0")
		c.Unlock()
		ready <- true
		for val := range consumer {
			seen := val.(int)
			if desired == seen {
				c.met <- true
				// the condition is no longer useful and can kill itself
				// it's safer and simpler to just create new conditions for new checks
				return
			}
			c.Lock()
			c.lastStatus = fmt.Sprintf("saw %d", seen)
			c.Unlock()
		}
	}(ready)
	<-ready
	return c
}

func (tE *TestEndpoint) conditionNumSeen(desired int) *conditionWatcher {
	c := newConditionWatcher(tE, fmt.Sprintf("tE %s", tE.addr), fmt.Sprintf("%d packets seen", desired))
	ready := make(chan bool)

	go func(ready chan bool) {
		consumer := make(chan interface{})
		tE.numSeen.Register(consumer)
		c.Lock()
		c.lastStatus = fmt.Sprintf("saw 0")
		c.Unlock()
		ready <- true
		for val := range consumer {
			seen := val.(int)
			if desired == seen {
				c.met <- true
				// the condition is no longer useful and can kill itself
				// it's safer and simpler to just create new conditions for new checks
				tE.numSeen.Unregister(consumer)
				return
			}
			c.Lock()
			c.lastStatus = fmt.Sprintf("saw %d", seen)
			c.Unlock()
		}
	}(ready)
	<-ready
	return c
}

// arr implements sort.Interface for [][]byte
type arr []encoding.Datapoint

func (data arr) Len() int {
	return len(data)
}
func (data arr) Swap(i, j int) {
	data[i], data[j] = data[j], data[i]
}
func (data arr) Less(i, j int) bool {
	return data[i].Name < data[j].Name
}

// assume that ref feeds in sorted order, because we rely on that!
func (tE *TestEndpoint) SeenThisOrFatal(ref chan encoding.Datapoint) {
	tE.WhatHaveISeen <- true
	seen := <-tE.IHaveSeen
	sort.Sort(arr(seen))
	i := 0
	empty := encoding.Datapoint{}
	getSeenBuf := func() encoding.Datapoint {
		if len(seen) <= i {
			return empty
		}
		i += 1
		return seen[i-1]
	}

	ok := true
	refBuf := <-ref
	seenBuf := getSeenBuf()
	for refBuf != empty || seenBuf != empty {
		cmp := strings.Compare(seenBuf.Name, refBuf.Name)
		if seenBuf == empty || refBuf == empty {
			// if one of them is nil, we want it to be counted as "very high", because there's no more input
			// so in that case, invert the rules
			cmp *= -1
		}
		switch cmp {
		case 0:
			refBuf = <-ref
			seenBuf = getSeenBuf()
		case 1: // seen bigger than refBuf, i.e. seen is missing a line
			if ok {
				tE.t.Error("diff <reference> <seen>")
			}
			ok = false
			tE.t.Errorf("tE %s - %s", tE.addr, refBuf)
			refBuf = <-ref
		case -1: // seen is smaller than refBuf, i.e. it has a line more than the ref has
			if ok {
				tE.t.Error("diff <reference> <seen>")
			}
			ok = false
			tE.t.Errorf("tE %s + %s", tE.addr, seenBuf)
			seenBuf = getSeenBuf()
		}
	}
	if !ok {
		tE.t.Fatal("bad data")
	}
}

func (tE *TestEndpoint) handle(c net.Conn) {
	defer func() {
		tE.t.Logf("tE %s closing conn %s", tE.addr, c)
		c.Close()
	}()
	r := bufio.NewReaderSize(c, 4096)
	h := encoding.NewPlain(false, true)
	for {
		select {
		case <-tE.shutdownHandle:
			return
		default:
		}
		buf, _, err := r.ReadLine()
		if err != nil {
			tE.t.Logf("tE %s read error: %s. closing handler", tE.addr, err)
			return
		}
		tE.t.Logf("tE %s %s read", tE.addr, buf)
		d, _ := h.Load(buf)
		tE.seen <- d
	}
}

func (tE *TestEndpoint) Close() {
	tE.t.Logf("tE %s shutting down accepter (after accept breaks)", tE.addr)
	tE.shutdown <- true
	tE.t.Logf("tE %s shutting down handler (after readLine breaks)", tE.addr)
	tE.shutdownHandle <- true
	tE.t.Logf("tE %s shutting down listener", tE.addr)
	tE.ln.Close()
	tE.t.Logf("tE %s listener down", tE.addr)
}

type TestEndpointCounter struct {
	t              testing.TB
	addr           string
	ln             net.Listener
	seen           chan []byte
	seenBufs       [][]byte
	shutdown       chan bool
	shutdownHandle chan bool // to shut down 1 handler. if you start more handlers they'll keep running
	accepts        chan struct{}
	metrics        chan struct{}
}

// 10 accept calls can accumulate without reading them out, before accept starts to block
// 1000 metrics can come in without reading them out, before the reader starts to block
func NewTestEndpointCounter(t testing.TB, addr string) *TestEndpointCounter {
	// shutdown chan size 1 so that Close() doesn't have to wait on the write
	// because the loops will typically be stuck in Accept and Readline
	return &TestEndpointCounter{
		t:              t,
		addr:           addr,
		seen:           make(chan []byte),
		seenBufs:       make([][]byte, 0),
		shutdown:       make(chan bool, 1),
		shutdownHandle: make(chan bool, 1),
		accepts:        make(chan struct{}, 10),
		metrics:        make(chan struct{}, 1000),
	}
}

func (tE *TestEndpointCounter) Start() {
	ln, err := net.Listen("tcp", tE.addr)
	if err != nil {
		panic(err)
	}
	tE.t.Logf("tE %s is now listening", tE.addr)
	tE.ln = ln
	go func() {
		for {
			select {
			case <-tE.shutdown:
				return
			default:
			}
			tE.t.Logf("tE %s waiting for accept", tE.addr)
			conn, err := ln.Accept()
			// when closing, this can happen: accept tcp [::]:2005: use of closed network connection
			if err != nil {
				tE.t.Logf("tE %s accept error: '%s' -> stopping tE", tE.addr, err)
				return
			}
			tE.accepts <- struct{}{}
			tE.t.Logf("tE %s accepted new conn", tE.addr)
			go tE.handle(conn)
			defer func() { tE.t.Logf("tE %s closing conn.", tE.addr); conn.Close() }()
		}
	}()
}

func (tE *TestEndpointCounter) handle(c net.Conn) {
	defer func() {
		tE.t.Logf("tE %s closing conn %s", tE.addr, c)
		c.Close()
	}()
	r := bufio.NewReaderSize(c, 4096)
	for {
		select {
		case <-tE.shutdownHandle:
			return
		default:
		}
		_, _, err := r.ReadLine()
		if err != nil {
			tE.t.Logf("tE %s read error: %s. closing handler", tE.addr, err)
			return
		}
		tE.metrics <- struct{}{}
	}
}

func (tE *TestEndpointCounter) WaitAccepts(exp int, max time.Duration) {
	tE.t.Logf("waiting for %d accepts", exp)
	timeout := time.Tick(max)
	val := 0
	for {
		select {
		case <-tE.accepts:
			val += 1
			if val == exp {
				tE.t.Logf("seen %d accepts", val)
				return
			}
		case <-timeout:
			tE.t.Errorf("timed out after %s waiting for %d accepts. only saw %d", max, exp, val)
		}
	}
}

func (tE *TestEndpointCounter) WaitMetrics(exp int, max time.Duration) {
	tE.t.Logf("waiting until all %d messages received", exp)
	timeout := time.Tick(max)
	val := 0
	for {
		select {
		case <-tE.metrics:
			val += 1
			if val == exp {
				tE.t.Logf("received all %d metrics", exp)
				return
			}
		case <-timeout:
			tE.t.Errorf("timed out after %s waiting for %d metrics. only saw %d", max, exp, val)
		}
	}
}

func (tE *TestEndpointCounter) Close() {
	tE.t.Logf("tE %s shutting down accepter (after accept breaks)", tE.addr)
	tE.shutdown <- true
	tE.t.Logf("tE %s shutting down handler (after readLine breaks)", tE.addr)
	tE.shutdownHandle <- true
	tE.t.Logf("tE %s shutting down listener", tE.addr)
	tE.ln.Close()
	tE.t.Logf("tE %s listener down", tE.addr)
}
