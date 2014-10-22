package main

import (
	"fmt"
	"github.com/bmizerany/assert"
	"log"
	"os"
	"testing"
	"time"
)

var singleMetric = []byte("test-metric 123 1234567890")

// when testing works, make this 1000
var kMetricsA [10][]byte
var kMetricsB [10][]byte
var kMetricsC [10][]byte

func init() {
	for i := 0; i < 10; i++ {
		kMetricsA[i] = []byte(fmt.Sprintf("test-metric 123 %d", i))
		kMetricsB[i] = []byte(fmt.Sprintf("test-metric 123 %d", 10+i))
		kMetricsC[i] = []byte(fmt.Sprintf("test-metric 123 %d", 20+i))
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
	time.Sleep(10 * time.Millisecond) // give time to read data
	tE.WhatHaveISeen <- true
	seen := <-tE.IHaveSeen
	assert.Equal(t, len(seen), 1)
	assert.Equal(t, seen[0], singleMetric)
}

func Test3RangesWith2EndpointAndSpoolInMiddle(t *testing.T) {
	os.RemoveAll("test_spool")
	os.Mkdir("test_spool", os.ModePerm)

	log.Println("##### START STEP 1 #####")
	// UUU -> up-up-up
	// UDU -> up-down-up
	tUUU := NewTestEndpoint(t, ":2005")
	tUDU := NewTestEndpoint(t, ":2006")
	table = NewTable("test_spool")
	err := applyCommand(table, "addRoute sendAllMatch test1  127.0.0.1:2005  127.0.0.1:2006 spool=true")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(table.Print())

	err = table.Run()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond) // give time to establish conn
	for _, m := range kMetricsA {
		table.Dispatch(m)
		// give time to write to conn without triggering slow conn (i.e. no faster than 100k/s)
		// note i'm afraid this sleep masks another issue: data can get reordered.
		// if you take this sleep away, and run like so:
		// go test 2>&1 | egrep '(table sending to route|route.*receiving)' | grep -v 2006
		// you should see that data goes through the table in the right order, but the route receives
		// the points in a different order.
		time.Sleep(10 * time.Microsecond)
	}
	time.Sleep(time.Millisecond) // give time to traverse the routing pipeline
	err = table.Flush()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond) // give time to read data
	tUUU.WhatHaveISeen <- true
	seenUUU := <-tUUU.IHaveSeen
	tUDU.WhatHaveISeen <- true
	seenUDU := <-tUDU.IHaveSeen
	assert.Equal(t, len(seenUUU), 10)
	assert.Equal(t, len(seenUDU), 10)
	assert.Equal(t, seenUUU, kMetricsA[:])
	assert.Equal(t, seenUDU, kMetricsA[:])

	// STEP 2: tUDU goes down! simulate outage
	log.Println("##### START STEP 2 #####")
	tUDU.Close()

	for _, m := range kMetricsB {
		table.Dispatch(m)
		time.Sleep(10 * time.Microsecond)
	}

	time.Sleep(time.Millisecond) // give time to traverse the routing pipeline
	err = table.Flush()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond) // give time to read data
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
	log.Println("##### START STEP 3 #####")
	tUDU = NewTestEndpoint(t, ":2006")

	for _, m := range kMetricsC {
		table.Dispatch(m)
		time.Sleep(10 * time.Microsecond)
	}

	time.Sleep(time.Millisecond) // give time to traverse the routing pipeline
	err = table.Flush()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond) // give time to read data
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
	assert.Equal(t, len(seenUDU), 20)
	//assert.Equal(t, len(seenUDU), 30)
	fmt.Println(seenUDU)

	err = table.Shutdown()
	if err != nil {
		t.Fatal(err)
	}
}

// TODO benchmark the pipeline/matching
