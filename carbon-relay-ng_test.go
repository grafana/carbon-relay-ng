package main

import (
	"fmt"
	"github.com/bmizerany/assert"
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
	err := applyCommand(table, "addRoute sendAllMatch test1  127.0.0.1:2005", config)
	if err != nil {
		t.Fatal(err)
	}
	err = table.Run()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond) // give time to establish conn
	table.Dispatch(singleMetric)
	time.Sleep(10 * time.Millisecond) // give time to traverse the routing pipeline into conn
	err = table.Shutdown()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond) // give time to read data
	tE.WhatHaveISeen <- true
	seen := <-tE.IHaveSeen
	assert.Equal(t, len(seen), 1)
	assert.Equal(t, seen[0], singleMetric)
}

func Test3RangesWith2EndpointAndSpoolInMiddle(t *testing.T) {
	// UUU -> up-up-up
	// UDU -> up-down-up
	tUUU := NewTestEndpoint(t, ":2005")
	tUDU := NewTestEndpoint(t, ":2006")
	table = NewTable("")
	err := applyCommand(table, "addRoute sendAllMatch test1  127.0.0.1:2005  127.0.0.1:2006 spool=true", config)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(table.Print())

	err = table.Run()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond) // give time to establish conn
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
	time.Sleep(200 * time.Millisecond) // give time to traverse the routing pipeline
	err = table.Flush()
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(800 * time.Millisecond) // give time to read data
	tUUU.WhatHaveISeen <- true
	seenUUU := <-tUUU.IHaveSeen
	tUDU.WhatHaveISeen <- true
	seenUDU := <-tUDU.IHaveSeen
	assert.Equal(t, len(seenUUU), 10)
	assert.Equal(t, len(seenUDU), 10)
	assert.Equal(t, seenUUU, kMetricsA[:])
	assert.Equal(t, seenUDU, kMetricsA[:])

	time.Sleep(10 * time.Millisecond) // give time to traverse the routing pipeline into conn
	err = table.Shutdown()
	if err != nil {
		t.Fatal(err)
	}
}
