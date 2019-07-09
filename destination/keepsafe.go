package destination

import (
	"sync"
	"time"

	"github.com/graphite-ng/carbon-relay-ng/encoding"
)

// keepSafe is a buffer which retains
// at least the last periodKeep's worth of data
// typically you get between periodKeep and 2*periodKeep
// but don't rely on that
type keepSafe struct {
	initialCap int
	safeOld    []encoding.Datapoint
	safeRecent []encoding.Datapoint
	periodKeep time.Duration
	closed     chan struct{}
	wg         sync.WaitGroup
	sync.Mutex
}

func NewKeepSafe(initialCap int, periodKeep time.Duration) *keepSafe {
	k := &keepSafe{
		initialCap: initialCap,
		safeOld:    make([]encoding.Datapoint, 0, initialCap),
		safeRecent: make([]encoding.Datapoint, 0, initialCap),
		periodKeep: periodKeep,
		closed:     make(chan struct{}),
	}
	k.wg.Add(1)
	go k.keepClean()
	return k
}

func (k *keepSafe) keepClean() {
	tick := time.NewTicker(k.periodKeep)
	defer k.wg.Done()
	for {
		select {
		case <-k.closed:
			return
		case <-tick.C:
			k.Lock()
			k.expire()
			k.Unlock()
		}
	}
}

func (k *keepSafe) expire() {
	swapSlice := k.safeOld[:0]
	if cap(swapSlice) > 2*k.initialCap {
		// if the capacity of the former slice is too big then
		// allocate a smaller one in order to save memory
		swapSlice = make([]encoding.Datapoint, 0, k.initialCap)
	}
	k.safeOld = k.safeRecent
	k.safeRecent = swapSlice
}

func (k *keepSafe) Add(dp encoding.Datapoint) {
	k.Lock()
	k.safeRecent = append(k.safeRecent, dp)
	k.Unlock()
}

func (k *keepSafe) GetAll() []encoding.Datapoint {
	k.Lock()
	ret := make([]encoding.Datapoint, 0, len(k.safeOld)+len(k.safeRecent))
	ret = append(ret, k.safeOld...)
	ret = append(ret, k.safeRecent...)
	k.safeOld = k.safeOld[:0]
	k.safeRecent = k.safeRecent[:0]
	k.Unlock()
	return ret
}

func (k *keepSafe) Stop() {
	k.Lock()
	close(k.closed)
	k.wg.Wait()
	k.Unlock()
}
