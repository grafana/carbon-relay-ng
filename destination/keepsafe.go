package destination

import (
	"sync"
	"time"
)

type keepSafe struct {
	initialCap int
	safeOld    [][]byte
	safeRecent [][]byte
	periodKeep time.Duration // period to keep at least.  you get this period + a bit more
	sync.Mutex
}

func NewKeepSafe(initialCap int, periodKeep time.Duration) *keepSafe {
	k := keepSafe{
		initialCap: initialCap,
		safeOld:    make([][]byte, 0, initialCap),
		safeRecent: make([][]byte, 0, initialCap),
		periodKeep: periodKeep,
	}
	go k.keepClean()
	return &k
}

func (k *keepSafe) keepClean() {
	tick := time.NewTicker(k.periodKeep)
	for range tick.C {
		k.Lock()
		k.safeOld = k.safeRecent
		k.safeRecent = make([][]byte, 0, k.initialCap)
		k.Unlock()
	}
}

func (k *keepSafe) Add(buf []byte) {
	k.Lock()
	k.safeRecent = append(k.safeRecent, buf)
	k.Unlock()
}

func (k *keepSafe) GetAll() [][]byte {
	k.Lock()
	ret := append(k.safeOld, k.safeRecent...)
	k.safeOld = make([][]byte, 0, k.initialCap)
	k.safeRecent = make([][]byte, 0, k.initialCap)
	k.Unlock()
	return ret
}
