package main

import (
	"container/list"
	"sync"
	"time"
)

type keepSafe struct {
	safeOld    *list.List
	safeRecent *list.List
	periodKeep time.Duration // period to keep at least.  you get this period + a bit more
	sync.Mutex
}

func NewKeepSafe(periodKeep time.Duration) *keepSafe {
	k := keepSafe{
		safeOld:    list.New(),
		safeRecent: list.New(),
		periodKeep: periodKeep,
	}
	go k.keepClean()
	return &k
}

func (k *keepSafe) keepClean() {
	tick := time.NewTicker(k.periodKeep)
	for _ = range tick.C {
		k.Lock()
		k.safeOld = k.safeRecent
		k.safeRecent = list.New()
		k.Unlock()
	}
}

func (k *keepSafe) Add(buf []byte) {
	k.Lock()
	k.safeRecent.PushBack(buf)
	k.Unlock()
}

func (k *keepSafe) GetAll() chan []byte {
	ret := make(chan []byte)
	k.Lock()
	go func(ret chan []byte, old, recent *list.List) {
		for e := old.Front(); e != nil; e = e.Next() {
			ret <- e.Value.([]byte)
		}
		for e := recent.Front(); e != nil; e = e.Next() {
			ret <- e.Value.([]byte)
		}
	}(ret, k.safeOld, k.safeRecent)
	k.Unlock()
	return ret
}
