package validate

import (
	"errors"
	"hash"
	"hash/fnv"
	"sync"
)

var lock sync.Mutex
var m map[uint64]uint32
var errNotNewer = errors.New("point is not newer than previous")
var h hash.Hash64

func init() {
	m = make(map[uint64]uint32)
	h = fnv.New64a()
}

func Ordered(key []byte, ts uint32) error {
	lock.Lock()
	h.Write(key)
	k := h.Sum64()
	h.Reset()
	defer lock.Unlock()
	tsOld := m[k]
	if ts > tsOld {
		m[k] = ts
		return nil
	}
	return errNotNewer
}
