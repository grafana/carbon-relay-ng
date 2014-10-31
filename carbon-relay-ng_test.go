package main

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

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

func NewTableOrFatal(t *testing.T, spool_dir, cmd string) *Table {
	table = NewTable(spool_dir)
	err := applyCommand(table, cmd)
	if err != nil {
		t.Fatal(err)
	}
	err = table.Run()
	if err != nil {
		t.Fatal(err)
	}
	return table
}

func (table *Table) ShutdownOrFatal(t *testing.T) {
	err := table.Shutdown()
	if err != nil {
		t.Fatal(err)
	}
}

//TODO the length of some of those sleeps/timeouts are not satisfactory, we need to do more perf testing and tuning
//TODO get rid of all sleeps, we can do better sync wait constructs

func TestSinglePointSingleRoute(t *testing.T) {
	tE := NewTestEndpoint(t, ":2005")
	defer tE.Close()
	table := NewTableOrFatal(t, "", "addRoute sendAllMatch test1  127.0.0.1:2005 flush=10")
	tE.WaitNumAcceptsOrFatal(1, 50*time.Millisecond, nil)
	table.Dispatch(tenMetricsA[0])
	tE.WaitNumSeenOrFatal(1, 500*time.Millisecond, nil)
	tE.SeenThisOrFatal(tenMetricsA[:1])
	table.ShutdownOrFatal(t)
	time.Sleep(100 * time.Millisecond) // not sure yet why, but for some reason there's annoying/confusing conn Close() logs still showing up
	// we don't want to mess up the view of the next test
}

func Test3RangesWith2EndpointAndSpoolInMiddle(t *testing.T) {
	os.RemoveAll("test_spool")
	os.Mkdir("test_spool", os.ModePerm)
	tEWaits := sync.WaitGroup{} // for when we want to wait on both tE's simultaneously

	log.Notice("##### START STEP 1 #####")
	// UUU -> up-up-up
	// UDU -> up-down-up
	tUUU := NewTestEndpoint(t, ":2005")
	tUDU := NewTestEndpoint(t, ":2006")
	// reconnect retry should be quick now, so we can proceed quicker
	// also flushing freq is increased so we don't have to wait as long
	table := NewTableOrFatal(t, "test_spool", "addRoute sendAllMatch test1  127.0.0.1:2005 flush=10  127.0.0.1:2006 spool=true reconn=200 flush=10")
	fmt.Println(table.Print())
	tEWaits.Add(2)
	go tUUU.WaitNumAcceptsOrFatal(1, 50*time.Millisecond, &tEWaits)
	go tUDU.WaitNumAcceptsOrFatal(1, 50*time.Millisecond, &tEWaits)
	tEWaits.Wait()
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
	tEWaits.Add(2)
	go tUUU.WaitNumSeenOrFatal(1000, 2*time.Second, &tEWaits)
	go tUDU.WaitNumSeenOrFatal(1000, 1*time.Millisecond, &tEWaits)
	tEWaits.Wait()
	tUUU.SeenThisOrFatal(kMetricsA[:])
	tUDU.SeenThisOrFatal(kMetricsA[:])

	// STEP 2: tUDU goes down! simulate outage
	log.Notice("##### START STEP 2 #####")
	tUDU.Close()

	for _, m := range kMetricsB {
		table.Dispatch(m)
		time.Sleep(10 * time.Microsecond) // see above
	}

	tUUU.WaitNumSeenOrFatal(2000, 4*time.Second, nil)
	allSent := make([][]byte, 2000)
	copy(allSent[0:1000], kMetricsA[:])
	copy(allSent[1000:2000], kMetricsB[:])
	tUUU.SeenThisOrFatal(allSent)

	// STEP 3: bring the one that was down back up, it should receive all data it missed thanks to the spooling (+ new data)
	log.Notice("##### START STEP 3 #####")
	tUDU = NewTestEndpoint(t, ":2006")

	time.Sleep(250 * time.Millisecond) // make sure conn can detect the endpoint is back up

	for _, m := range kMetricsC {
		table.Dispatch(m)
		time.Sleep(10 * time.Microsecond) // see above
	}

	tEWaits.Add(2)
	go tUUU.WaitNumSeenOrFatal(3000, 6*time.Second, &tEWaits)
	// in theory we only need 2000 points here, but because of the redo buffer it should have sent the first points as well
	go tUDU.WaitNumSeenOrFatal(3000, 6*time.Second, &tEWaits)
	tEWaits.Wait()
	allSent = make([][]byte, 3000)
	copy(allSent[0:1000], kMetricsA[:])
	copy(allSent[1000:2000], kMetricsB[:])
	copy(allSent[2000:3000], kMetricsC[:])

	tUUU.SeenThisOrFatal(allSent)
	tUDU.SeenThisOrFatal(allSent)

	table.ShutdownOrFatal(t)
}

func BenchmarkSanity(b *testing.B) {
	for i := 0; i < b.N; i++ {
		fmt.Println("loop SANITY #", i, "START")
		time.Sleep(100 * time.Millisecond)
	}
}

// TODO benchmark the pipeline/matching
func BenchmarkSend(b *testing.B) {
	tE := NewTestEndpoint(nil, ":2005")
	table = NewTable("")
	err := applyCommand(table, "addRoute sendAllMatch test1  127.0.0.1:2005")
	if err != nil {
		b.Fatal(err)
	}
	err = table.Run()
	if err != nil {
		b.Fatal(err)
	}
	tE.WaitUntilNumAccepts(1)
	for i := 0; i < b.N; i++ {
		fmt.Println("loop #", i, "START")
		for _, m := range kMetricsA {
			table.Dispatch(m)
		}
		fmt.Println("loop #", i, "DONE")
	}
	tE.WaitUntilNumMsg(1000 * b.N)
	log.Notice("received all", string(1000*b.N), "messages. wrapping up benchmark run")
	err = table.Shutdown()
	if err != nil {
		b.Fatal(err)
	}
	tE.Close()
}
