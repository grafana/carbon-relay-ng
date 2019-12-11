package queue

import (
	"encoding/binary"
	"github.com/graphite-ng/carbon-relay-ng/encoding"
)

// var keyPool = sync

// Item represents an entry in either a stack or queue.
type Item struct {
	ID    uint64
	Key   []byte
	Value []byte
	Tags  encoding.Tags
}

func (i *Item) ToString() string {
	return string(i.Value)
}

func encodeID(id uint64) []byte {
	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, id)
	return key
}

// keyToID converts and returns the given key to an ID.
func keyToID(key []byte) uint64 {
	return binary.BigEndian.Uint64(key)
}
