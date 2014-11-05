package main

// for now the tests use 10 vals,
// once everything works better and is tweaked, we can use larger amounts

import (
	"fmt"
	logging "github.com/op/go-logging"
	"os"
	"sync"
	"testing"
	"time"
)

var packets1A *dummyPackets
var packets1B *dummyPackets
var packets1C *dummyPackets

var packets3A *dummyPackets
var packets3B *dummyPackets
var packets3C *dummyPackets

var packets4A *dummyPackets
var packets5A *dummyPackets

var packets6A *dummyPackets
var packets6B *dummyPackets
var packets6C *dummyPackets

func init() {
	packets1A = NewDummyPackets("1A", 10)
	packets1B = NewDummyPackets("1B", 10)
	packets1C = NewDummyPackets("1C", 10)
	packets3A = NewDummyPackets("3A", 1000)
	packets3B = NewDummyPackets("3B", 1000)
	packets3C = NewDummyPackets("3C", 1000)
	packets4A = NewDummyPackets("4A", 10000)
	packets5A = NewDummyPackets("5A", 100000)
	packets6A = NewDummyPackets("6A", 1000000)
	//packets6B = NewDummyPackets("6B", 1000000)
	//packets6C = NewDummyPackets("6C", 1000000)
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
	table.Dispatch(packets1A.Get(0))
	tE.WaitNumSeenOrFatal(1, 500*time.Millisecond, nil)
	//c := make(chan []byte, 1)
	//c <- metricBuf.Bytes()
	//close(c)
	//tE.SeenThisOrFatal(c)
	table.ShutdownOrFatal(t)
	time.Sleep(100 * time.Millisecond) // not sure yet why, but for some reason there's annoying/confusing conn Close() logs still showing up
	// we don't want to mess up the view of the next test
}

func Test3RangesWith2EndpointAndSpoolInMiddle(t *testing.T) {
	logging.SetLevel(logging.DEBUG, "carbon-relay-ng")
	os.RemoveAll("test_spool")
	os.Mkdir("test_spool", os.ModePerm)
	//var checkerUUU chan []byte
	//var checkerUDU chan []byte
	tEWaits := sync.WaitGroup{} // for when we want to wait on both tE's simultaneously

	log.Notice("##### START STEP 1 #####")
	// UUU -> up-up-up
	// UDU -> up-down-up
	tUUU := NewTestEndpoint(t, ":2005")
	tUDU := NewTestEndpoint(t, ":2006")
	// reconnect retry should be quick now, so we can proceed quicker
	// also flushing freq is increased so we don't have to wait as long
	table := NewTableOrFatal(t, "test_spool", "addRoute sendAllMatch test1  127.0.0.1:2005 flush=10  127.0.0.1:2006 spool=true reconn=20 flush=10")
	fmt.Println(table.Print())
	tEWaits.Add(2)
	go tUUU.WaitNumAcceptsOrFatal(1, 50*time.Millisecond, &tEWaits)
	go tUDU.WaitNumAcceptsOrFatal(1, 50*time.Millisecond, &tEWaits)
	tEWaits.Wait()
	//checkerUUU = make(chan []byte, 1000)
	//checkerUDU = make(chan []byte, 1000)
	for i := 0; i < 1000; i++ {
		table.Dispatch(packets3A.Get(i))
		//checkerUUU <- metricBuf.Bytes()
		//checkerUDU <- metricBuf.Bytes()
		// give time to write to conn without triggering slow conn (i.e. no faster than 100k/s)
		// note i'm afraid this sleep masks another issue: data can get reordered.
		// if you take this sleep away, and run like so:
		// go test 2>&1 | egrep '(table sending to route|route.*receiving)' | grep -v 2006
		// you should see that data goes through the table in the right order, but the route receives
		// the points in a different order.
		time.Sleep(20 * time.Microsecond)
	}
	//close(checkerUUU)
	//close(checkerUDU)
	tEWaits.Add(2)
	go tUUU.WaitNumSeenOrFatal(1000, 2*time.Second, &tEWaits)
	go tUDU.WaitNumSeenOrFatal(1000, 2*time.Second, &tEWaits)
	tEWaits.Wait()
	//tUUU.SeenThisOrFatal(checkerUUU)
	//tUDU.SeenThisOrFatal(checkerUDU)

	// STEP 2: tUDU goes down! simulate outage
	log.Notice("##### START STEP 2 #####")
	tUDU.Close()

	//checkerUUU = make(chan []byte, 1000)
	for i := 0; i < 1000; i++ {
		table.Dispatch(packets3B.Get(i))
		//checkerUUU <- metricBuf.Bytes()
		time.Sleep(10 * time.Microsecond) // see above
	}

	tUUU.WaitNumSeenOrFatal(2000, 2*time.Second, nil)
	//tUUU.SeenThisOrFatal(checkerUUU)

	// STEP 3: bring the one that was down back up, it should receive all data it missed thanks to the spooling (+ new data)
	log.Notice("##### START STEP 3 #####")
	tUDU = NewTestEndpoint(t, ":2006")
	//checkerUUU = make(chan []byte, 1000)
	//checkerUDU = make(chan []byte, 1000)

	tUDU.WaitNumAcceptsOrFatal(1, 50*time.Millisecond, nil)

	for i := 0; i < 1000; i++ {
		table.Dispatch(packets3C.Get(i))
		time.Sleep(10 * time.Microsecond) // see above
	}

	tEWaits.Add(2)
	go tUUU.WaitNumSeenOrFatal(3000, 1*time.Second, &tEWaits)
	// in theory we only need 2000 points here, but because of the redo buffer it should have sent the first points as well
	go tUDU.WaitNumSeenOrFatal(3000, 6*time.Second, &tEWaits)
	tEWaits.Wait()
	//allSent = make([][]byte, 3000)

	//tUUU.SeenThisOrFatal(allSent)
	//tUDU.SeenThisOrFatal(allSent)

	table.ShutdownOrFatal(t)
}

func benchmarkSendAndReceive(b *testing.B, dp *dummyPackets) {
	logging.SetLevel(logging.WARNING, "carbon-relay-ng")
	tE := NewTestEndpoint(nil, ":2005")
	table = NewTableOrFatal(b, "", "addRoute sendAllMatch test1  127.0.0.1:2005")
	tE.WaitUntilNumAccepts(1)
	// reminder: go benchmark will invoke this with N = 0, then maybe N = 20, then maybe more
	// and the time it prints is function run divided by N, which
	// should be of a more or less stable time, which gets printed
	fmt.Println()
	for i := 0; i < b.N; i++ {
		log.Notice("iteration %d: sending %d metrics", i, dp.amount)
		for m := range dp.All() {
			//fmt.Println("dispatching", m)
			//fmt.Printf("dispatching '%s'\n", string(m))
			table.Dispatch(m)
			time.Sleep(10 * time.Microsecond) // see above
		}
		log.Notice("waiting until all %d messages received", dp.amount*(i+1))
		tE.WaitUntilNumMsg(dp.amount * (i + 1))
		log.Notice("iteration %d done. received %d metrics (%d total)", i, dp.amount, dp.amount*(i+1))
	}
	log.Notice("received all %d messages. wrapping up benchmark run", string(dp.amount*b.N))
	err := table.Shutdown()
	if err != nil {
		b.Fatal(err)
	}
	tE.Close()
}

func BenchmarkSendAndReceiveThousand(b *testing.B) {
	benchmarkSendAndReceive(b, packets3A)
}
func BenchmarkSendAndReceiveTenThousand(b *testing.B) {
	benchmarkSendAndReceive(b, packets4A)
}
func BenchmarkSendAndReceiveHundredThousand(b *testing.B) {
	benchmarkSendAndReceive(b, packets5A)
}
func BenchmarkSendAndReceiveMillion(b *testing.B) {
	benchmarkSendAndReceive(b, packets6A)
}
