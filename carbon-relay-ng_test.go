package main

import (
	"fmt"
	"github.com/bmizerany/assert"
	"os"
	"testing"
	"time"
)

var singleMetric = []byte("test-metric 123 1234567890")

// for now the tests use the one with 10 vals, once everything works better and is tweaked, we can use the larger ones
var tenMetricsA [10][]byte
var tenMetricsB [10][]byte
var tenMetricsC [10][]byte

var kMetricsA [1000][]byte
var kMetricsB [1000][]byte
var kMetricsC [1000][]byte

var MMetricsA [1000000][]byte
var MMetricsB [1000000][]byte
var MMetricsC [1000000][]byte

func init() {
	for i := 0; i < 10; i++ {
		tenMetricsA[i] = []byte(fmt.Sprintf("test-metricA 123 %d", i))
		tenMetricsB[i] = []byte(fmt.Sprintf("test-metricB 123 %d", i))
		tenMetricsC[i] = []byte(fmt.Sprintf("test-metricC 123 %d", i))
	}
	for i := 0; i < 1000; i++ {
		kMetricsA[i] = []byte(fmt.Sprintf("test-metricA 123 %d", i))
		kMetricsB[i] = []byte(fmt.Sprintf("test-metricB 123 %d", i))
		kMetricsC[i] = []byte(fmt.Sprintf("test-metricC 123 %d", i))
	}
	for i := 0; i < 1000000; i++ {
		MMetricsA[i] = []byte(fmt.Sprintf("test-metricA 123 %d", i))
		MMetricsB[i] = []byte(fmt.Sprintf("test-metricB 123 %d", i))
		MMetricsC[i] = []byte(fmt.Sprintf("test-metricC 123 %d", i))
	}
}

func TestSinglePointSingleRoute(t *testing.T) {
	tE := NewTestEndpoint(t, ":2005")
	defer tE.Close()
	table = NewTable("")
	err := applyCommand(table, "addRoute sendAllMatch test1  127.0.0.1:2005")
	if err != nil {
		t.Fatal(err)
	}
	err = table.Run()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond) // give time to establish conn
	table.Dispatch(singleMetric)
	time.Sleep(time.Millisecond) // give time to traverse the routing pipeline into conn
	err = table.Shutdown()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond) // give time to read data
	tE.WhatHaveISeen <- true
	seen := <-tE.IHaveSeen
	assert.Equal(t, len(seen), 1)
	assert.Equal(t, seen[0], singleMetric)
}

func Test3RangesWith2EndpointAndSpoolInMiddle(t *testing.T) {
	os.RemoveAll("test_spool")
	os.Mkdir("test_spool", os.ModePerm)

	log.Notice("##### START STEP 1 #####")
	// UUU -> up-up-up
	// UDU -> up-down-up
	tUUU := NewTestEndpoint(t, ":2005")
	tUDU := NewTestEndpoint(t, ":2006")
	table = NewTable("test_spool")
	// reconnect retry should be quick now, so we can proceed quicker
	err := applyCommand(table, "addRoute sendAllMatch test1  127.0.0.1:2005  127.0.0.1:2006 spool=true reconn=200")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(table.Print())

	err = table.Run()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond) // give time to establish conn
	for _, m := range kMetricsA {
		table.Dispatch(m)
		// give time to write to conn without triggering slow conn (i.e. no faster than 100k/s)
		// note i'm afraid this sleep masks another issue: data can get reordered.
		// if you take this sleep away, and run like so:
		// go test 2>&1 | egrep '(table sending to route|route.*receiving)' | grep -v 2006
		// you should see that data goes through the table in the right order, but the route receives
		// the points in a different order.
		time.Sleep(20 * time.Microsecond)
	}
	time.Sleep(10 * time.Millisecond) // give time to traverse the routing pipeline
	err = table.Flush()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond) // give time to read data
	tUUU.WhatHaveISeen <- true
	seenUUU := <-tUUU.IHaveSeen
	tUDU.WhatHaveISeen <- true
	seenUDU := <-tUDU.IHaveSeen
	assert.Equal(t, len(seenUUU), 10)
	assert.Equal(t, len(seenUDU), 10)
	assert.Equal(t, seenUUU, kMetricsA[:])
	assert.Equal(t, seenUDU, kMetricsA[:])

	// STEP 2: tUDU goes down! simulate outage
	log.Notice("##### START STEP 2 #####")
	tUDU.Close()

	for _, m := range kMetricsB {
		table.Dispatch(m)
		time.Sleep(10 * time.Microsecond)
	}

	time.Sleep(10 * time.Millisecond) // give time to traverse the routing pipeline
	err = table.Flush()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond) // give time to read data
	tUUU.WhatHaveISeen <- true
	seenUUU = <-tUUU.IHaveSeen
	assert.Equal(t, len(seenUUU), 20)
	allSent := make([][]byte, 20)
	copy(allSent[0:10], kMetricsA[:])
	copy(allSent[10:20], kMetricsB[:])
	// human friendly:
	for i, m := range seenUUU {
		if string(m) != string(allSent[i]) {
			t.Errorf("error in UUU at pos %d: expected '%s', received: '%s'", i, allSent[i], m)
		}
	}
	// equivalent, but for deeper debugging
	assert.Equal(t, seenUUU, allSent)

	// STEP 3: bring the one that was down back up, it should receive all data it missed thanks to the spooling (+ new data)
	log.Notice("##### START STEP 3 #####")
	tUDU = NewTestEndpoint(t, ":2006")

	time.Sleep(250 * time.Millisecond) // make sure conn can detect the endpoint is back up

	for _, m := range kMetricsC {
		table.Dispatch(m)
		time.Sleep(10 * time.Microsecond)
	}

	time.Sleep(time.Millisecond) // give time to traverse the routing pipeline
	err = table.Flush()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5000 * time.Millisecond) // give time to read data
	allSent = make([][]byte, 30)
	copy(allSent[0:10], kMetricsA[:])
	copy(allSent[10:20], kMetricsB[:])
	copy(allSent[20:30], kMetricsC[:])
	tUUU.WhatHaveISeen <- true
	seenUUU = <-tUUU.IHaveSeen
	tUDU.WhatHaveISeen <- true
	seenUDU = <-tUDU.IHaveSeen

	//check UUU
	assert.Equal(t, len(seenUUU), 30)
	// human friendly:
	for i, m := range seenUUU {
		if string(m) != string(allSent[i]) {
			t.Errorf("error in UUU at pos %d: expected '%s', received: '%s'", i, allSent[i], m)
		}
	}
	// equivalent, but for deeper debugging
	assert.Equal(t, seenUUU, allSent)

	//check UDU
	// in theory we only need 20 points here, but because of the redo buffer it should have sent the first 10 points as well
	assert.Equal(t, len(seenUDU), 30)
	fmt.Println(seenUDU)

	err = table.Shutdown()
	if err != nil {
		t.Fatal(err)
	}
}

//TODO the length of some of those sleeps are not satisfactory, we should maybe look into some perf issue or something

// TODO benchmark the pipeline/matching
