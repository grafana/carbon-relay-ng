package main

import (
	"fmt"
	logging "github.com/op/go-logging"
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
		tenMetricsA[i] = []byte(fmt.Sprintf("test.tenMetricsA 123 %d", i))
		tenMetricsB[i] = []byte(fmt.Sprintf("test.tenMetricsB 123 %d", i))
		tenMetricsC[i] = []byte(fmt.Sprintf("test.tenMetricsC 123 %d", i))
	}
	for i := 0; i < 1000; i++ {
		kMetricsA[i] = []byte(fmt.Sprintf("test.kMetricsA 123 %d", i))
		kMetricsB[i] = []byte(fmt.Sprintf("test.kMetricsB 123 %d", i))
		kMetricsC[i] = []byte(fmt.Sprintf("test.kMetricsC 123 %d", i))
	}
	for i := 0; i < 1000000; i++ {
		MMetricsA[i] = []byte(fmt.Sprintf("test.MMetricsA 123 %d", i))
		MMetricsB[i] = []byte(fmt.Sprintf("test.MMetricsB 123 %d", i))
		MMetricsC[i] = []byte(fmt.Sprintf("test.MMetricsC 123 %d", i))
	}
}

func NewTableOrFatal(tb interface{}, spool_dir, cmd string) *Table {
	table = NewTable(spool_dir)
	fatal := func(err error) {
		switch tb.(type) {
		case *testing.T:
			tb.(*testing.T).Fatal(err)
		case *testing.B:
			tb.(*testing.B).Fatal(err)
		}
	}
	err := applyCommand(table, cmd)
	if err != nil {
		fatal(err)
	}
	err = table.Run()
	if err != nil {
		fatal(err)
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
	go tUDU.WaitNumSeenOrFatal(1000, 2*time.Second, &tEWaits)
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

	tUUU.WaitNumSeenOrFatal(2000, 2*time.Second, nil)
	allSent := make([][]byte, 2000)
	copy(allSent[0:1000], kMetricsA[:])
	copy(allSent[1000:2000], kMetricsB[:])
	tUUU.SeenThisOrFatal(allSent)

	// STEP 3: bring the one that was down back up, it should receive all data it missed thanks to the spooling (+ new data)
	log.Notice("##### START STEP 3 #####")
	tUDU = NewTestEndpoint(t, ":2006")

	tUDU.WaitNumAcceptsOrFatal(1, 50*time.Millisecond, nil)

	for _, m := range kMetricsC {
		table.Dispatch(m)
		time.Sleep(10 * time.Microsecond) // see above
	}

	tEWaits.Add(2)
	go tUUU.WaitNumSeenOrFatal(3000, 1*time.Second, &tEWaits)
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

func BenchmarkSendAndReceive(b *testing.B) {
	logging.SetLevel(logging.WARNING, "carbon-relay-ng")
	tE := NewTestEndpoint(nil, ":2005")
	table = NewTableOrFatal(b, "", "addRoute sendAllMatch test1  127.0.0.1:2005")
	tE.WaitUntilNumAccepts(1)
	// reminder: go benchmark will invoke this with N = 0, then N = 20, then maybe more
	// and the time it prints is function run divided by N, which
	// should be of a more or less stable time, which gets printed
	for i := 0; i < b.N; i++ {
		log.Notice("iteration %d: sending %d metrics", i, 1000)
		for _, m := range kMetricsA {
			table.Dispatch(m)
			time.Sleep(10 * time.Microsecond) // see above
		}
		log.Notice("waiting until all %d messages received", 1000*(i+1))
		tE.WaitUntilNumMsg(1000 * (i + 1))
		log.Notice("iteration %d done. received 1000 metrics (%d total)", 1000*(i+1))
	}
	log.Notice("received all %d messages. wrapping up benchmark run", string(1000*b.N))
	err := table.Shutdown()
	if err != nil {
		b.Fatal(err)
	}
	tE.Close()
}
